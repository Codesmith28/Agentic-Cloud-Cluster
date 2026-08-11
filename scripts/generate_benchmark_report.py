#!/usr/bin/env python3

# Copyright 2025-2026 Sarthak Siddhpura
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Post-campaign report generator.

Queries the master node API after a benchmark campaign completes to produce a
comprehensive execution report that includes:
  - Cluster topology (workers, capacities, health)
  - Per-worker task distribution and utilisation
  - Scheduler comparison (PPO vs RTS vs RR)
  - Model version info
  - Overall pass/fail verdict

Usage:
    python3 scripts/generate_benchmark_report.py \\
        --campaign-dir results/campaign-20260426-170000/20260426-170000 \\
        --master-url http://localhost:8080 \\
        --model-path agentic_scheduler/models/ppo_latest.pt
"""

from __future__ import annotations

import argparse
import json
import hashlib
import pathlib
import sys
import time
import urllib.request
import urllib.error
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional


def api_get(url: str, timeout: int = 10) -> Optional[Dict]:
    try:
        req = urllib.request.Request(url, headers={"Accept": "application/json"})
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except Exception:
        return None


def get_workers(master_url: str) -> List[Dict]:
    data = api_get(f"{master_url}/api/workers")
    if data and "workers" in data:
        return data["workers"]
    return []


def get_telemetry(master_url: str) -> Dict:
    data = api_get(f"{master_url}/telemetry")
    return data if data else {}


def get_tasks(master_url: str) -> List[Dict]:
    data = api_get(f"{master_url}/api/tasks")
    if data and "tasks" in data:
        return data["tasks"]
    return []


def model_info(model_path: Optional[str]) -> Dict[str, str]:
    info: Dict[str, str] = {}
    if not model_path:
        return info
    p = pathlib.Path(model_path)
    if not p.exists():
        info["path"] = str(p)
        info["status"] = "not found"
        return info
    info["path"] = str(p)
    info["size_bytes"] = str(p.stat().st_size)
    sha = hashlib.sha256(p.read_bytes()).hexdigest()
    info["sha256"] = sha[:16] + "..."

    archive_dir = p.parent / "archive"
    version_file = archive_dir / "VERSION"
    if version_file.exists():
        info["version"] = f"v{int(version_file.read_text().strip()):03d}"
    return info


def load_campaign_report(campaign_dir: pathlib.Path) -> Optional[Dict]:
    json_path = campaign_dir / "campaign-report.json"
    if json_path.exists():
        return json.loads(json_path.read_text())
    # Try finding the subdirectory
    for child in sorted(campaign_dir.iterdir()):
        if child.is_dir():
            nested = child / "campaign-report.json"
            if nested.exists():
                return json.loads(nested.read_text())
    return None


def build_worker_task_map(tasks: List[Dict]) -> Dict[str, Dict]:
    """Map worker_id → {assigned, completed, failed, task_types}."""
    worker_map: Dict[str, Dict] = {}
    for task in tasks:
        wid = task.get("assigned_worker", task.get("worker_id", "unassigned"))
        if wid not in worker_map:
            worker_map[wid] = {
                "assigned": 0,
                "completed": 0,
                "failed": 0,
                "task_types": {},
            }
        entry = worker_map[wid]
        entry["assigned"] += 1
        status = task.get("status", "unknown")
        if status == "completed":
            entry["completed"] += 1
        elif status in ("failed", "error"):
            entry["failed"] += 1
        tag = task.get("tag", task.get("docker_image", "unknown"))
        entry["task_types"][tag] = entry["task_types"].get(tag, 0) + 1
    return worker_map


def generate_report(
    campaign_dir: pathlib.Path,
    master_url: str,
    model_path: Optional[str],
) -> str:
    lines: List[str] = []
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

    lines.append("# Benchmark Execution Report")
    lines.append("")
    lines.append(f"**Generated**: {now}")
    lines.append(f"**Master URL**: {master_url}")
    lines.append("")

    # ── Model info ───────────────────────────────────────────────────────
    m_info = model_info(model_path)
    if m_info:
        lines.append("## Model")
        lines.append("")
        for k, v in m_info.items():
            lines.append(f"- **{k}**: `{v}`")
        lines.append("")

    # ── Cluster topology ─────────────────────────────────────────────────
    workers = get_workers(master_url)
    telemetry = get_telemetry(master_url)
    tasks = get_tasks(master_url)

    lines.append("## Cluster Topology")
    lines.append("")
    lines.append(f"- **Workers registered**: {len(workers)}")

    if workers:
        lines.append("")
        lines.append("| Worker ID | CPU | Memory (GB) | Storage (GB) | Active | Status |")
        lines.append("|-----------|-----|-------------|-------------|--------|--------|")
        for w in workers:
            wid = w.get("worker_id", "?")
            cpu = w.get("total_cpu", w.get("cpu", "?"))
            mem = w.get("total_memory_gb", w.get("memory_gb", "?"))
            stor = w.get("total_storage_gb", w.get("storage_gb", "?"))
            active = "✓" if w.get("is_active", False) else "✗"
            status = w.get("status", "unknown")
            lines.append(f"| {wid} | {cpu} | {mem} | {stor} | {active} | {status} |")
        lines.append("")

    # ── Per-worker task distribution ─────────────────────────────────────
    worker_task_map = build_worker_task_map(tasks)
    if worker_task_map:
        lines.append("## Worker Utilisation")
        lines.append("")
        lines.append("| Worker | Tasks Assigned | Completed | Failed | Task Types |")
        lines.append("|--------|---------------|-----------|--------|------------|")
        total_assigned = 0
        total_completed = 0
        total_failed = 0
        for wid in sorted(worker_task_map.keys()):
            stats = worker_task_map[wid]
            total_assigned += stats["assigned"]
            total_completed += stats["completed"]
            total_failed += stats["failed"]
            types_str = ", ".join(
                f"{tag}({cnt})" for tag, cnt in sorted(stats["task_types"].items())
            )
            lines.append(
                f"| {wid} | {stats['assigned']} | {stats['completed']} | "
                f"{stats['failed']} | {types_str} |"
            )
        lines.append(
            f"| **Total** | **{total_assigned}** | **{total_completed}** | "
            f"**{total_failed}** | |"
        )
        lines.append("")

        # Distribution balance
        counts = [s["assigned"] for s in worker_task_map.values() if s["assigned"] > 0]
        if len(counts) > 1:
            mean_load = sum(counts) / len(counts)
            max_load = max(counts)
            min_load = min(counts)
            imbalance = round((max_load - min_load) / max(mean_load, 1) * 100, 1)
            lines.append(f"- **Load imbalance**: {imbalance}% (min={min_load}, max={max_load}, mean={mean_load:.1f})")
            lines.append(f"- **Workers utilised**: {len(counts)} / {len(workers)}")
            lines.append("")

    # ── Telemetry snapshot ───────────────────────────────────────────────
    if telemetry and isinstance(telemetry, dict):
        worker_tel = telemetry.get("workers", {})
        if worker_tel:
            lines.append("## Resource Utilisation (at report time)")
            lines.append("")
            lines.append("| Worker | CPU % | Memory % | Disk % |")
            lines.append("|--------|-------|----------|--------|")
            for wid, wdata in sorted(worker_tel.items()):
                if isinstance(wdata, dict):
                    cpu_pct = wdata.get("cpu_usage_percent", wdata.get("cpu_percent", "N/A"))
                    mem_pct = wdata.get("memory_usage_percent", wdata.get("mem_percent", "N/A"))
                    disk_pct = wdata.get("disk_usage_percent", wdata.get("disk_percent", "N/A"))
                    if isinstance(cpu_pct, (int, float)):
                        cpu_pct = f"{cpu_pct:.1f}"
                    if isinstance(mem_pct, (int, float)):
                        mem_pct = f"{mem_pct:.1f}"
                    if isinstance(disk_pct, (int, float)):
                        disk_pct = f"{disk_pct:.1f}"
                    lines.append(f"| {wid} | {cpu_pct} | {mem_pct} | {disk_pct} |")
            lines.append("")

    # ── Campaign results ─────────────────────────────────────────────────
    campaign = load_campaign_report(campaign_dir)
    if campaign:
        lines.append("## Campaign Summary")
        lines.append("")
        lines.append(f"- **Started**: {campaign.get('started_at', '?')}")
        lines.append(f"- **Finished**: {campaign.get('finished_at', '?')}")
        lines.append(f"- **Duration**: {campaign.get('total_duration_seconds', '?')}s")
        lines.append(f"- **Scenarios run**: {campaign.get('scenarios_run', '?')}")
        lines.append("")

        summary = campaign.get("summary", {})
        by_scheduler = summary.get("by_scheduler", {})
        if by_scheduler:
            lines.append("### Scheduler Comparison")
            lines.append("")
            lines.append("| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |")
            lines.append("|-----------|-------|-----------|--------|--------------|-------------|")
            for sched, stats in by_scheduler.items():
                lines.append(
                    f"| {sched} | {stats['total_tasks']} | {stats['total_completed']} | "
                    f"{stats['total_failed']} | {stats['avg_success_rate']}% | "
                    f"{stats['avg_duration_seconds']}s |"
                )
            lines.append("")

            best = summary.get("best_scheduler", "")
            if best:
                lines.append(f"**Best scheduler**: **{best}**")
                lines.append("")

        # Per-scenario breakdown
        results = campaign.get("results", [])
        if results:
            lines.append("### Scenario Details")
            lines.append("")
            for r in results:
                scenario = r.get("scenario", "?")
                sched = r.get("scheduler", "?")
                workload = r.get("workload", "?")
                lines.append(f"#### {scenario} / {sched} / {workload}")
                lines.append("")
                lines.append(f"- Submitted: {r.get('tasks_submitted', 0)}")
                lines.append(f"- Completed: {r.get('tasks_completed', 0)}")
                lines.append(f"- Failed: {r.get('tasks_failed', 0)}")
                lines.append(f"- Success Rate: {r.get('success_rate', 0)}%")
                lines.append(f"- Duration: {r.get('duration_seconds', 0)}s")
                err = r.get("error", "")
                if err:
                    lines.append(f"- Error: {err}")
                lines.append("")

    # ── Verdict ──────────────────────────────────────────────────────────
    lines.append("## Verdict")
    lines.append("")

    total_tasks = len(tasks)
    completed_tasks = sum(1 for t in tasks if t.get("status") == "completed")
    failed_tasks = sum(1 for t in tasks if t.get("status") in ("failed", "error"))

    if total_tasks == 0:
        verdict = "⚠️  NO TASKS EXECUTED"
    elif failed_tasks == 0 and completed_tasks == total_tasks:
        verdict = "✅ ALL TASKS PASSED"
    elif failed_tasks == 0:
        verdict = f"✅ PASSED ({completed_tasks}/{total_tasks} completed)"
    else:
        fail_rate = round(failed_tasks / total_tasks * 100, 1)
        verdict = f"⚠️  {failed_tasks}/{total_tasks} tasks failed ({fail_rate}%)"

    lines.append(f"**{verdict}**")
    lines.append("")
    lines.append(f"- Total tasks seen by cluster: {total_tasks}")
    lines.append(f"- Completed: {completed_tasks}")
    lines.append(f"- Failed: {failed_tasks}")
    lines.append(f"- Workers used: {len(worker_task_map)}")
    lines.append("")

    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate post-campaign benchmark report")
    parser.add_argument(
        "--campaign-dir",
        type=pathlib.Path,
        required=True,
        help="Path to campaign results directory",
    )
    parser.add_argument(
        "--master-url",
        default="http://localhost:8080",
        help="Master node HTTP API URL",
    )
    parser.add_argument(
        "--model-path",
        default="agentic_scheduler/models/ppo_latest.pt",
        help="Path to active .pt model file",
    )
    parser.add_argument(
        "--output",
        type=pathlib.Path,
        default=None,
        help="Output path (default: campaign-dir/BENCHMARK_REPORT.md)",
    )
    args = parser.parse_args()

    if not args.campaign_dir.exists():
        print(f"Campaign directory not found: {args.campaign_dir}", file=sys.stderr)
        return 1

    report = generate_report(args.campaign_dir, args.master_url, args.model_path)

    output_path = args.output or args.campaign_dir / "BENCHMARK_REPORT.md"
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(report, encoding="utf-8")
    print(f"Report written to: {output_path}")

    # Also print a summary to stdout
    print("\n" + "=" * 60)
    for line in report.split("\n"):
        if line.startswith("## Verdict") or line.startswith("**"):
            print(line)
    print("=" * 60)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
