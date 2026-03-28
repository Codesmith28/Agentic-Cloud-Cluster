#!/usr/bin/env python3
"""Evidence benchmark campaign runner.

Orchestrates a multi-scenario benchmark campaign that exercises the CloudAI
cluster across different scheduler algorithms, workload profiles, and failure
injection scenarios.  Produces a consolidated evidence report suitable for
academic submissions and project evaluation.

Usage:
    python3 testbench/scripts/run_campaign.py --master-url http://localhost:8080
    python3 testbench/scripts/run_campaign.py --scenarios all --output-dir results/campaign
"""

from __future__ import annotations

import argparse
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field, asdict
from typing import Any, Dict, List, Optional

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

SCHEDULERS = ["RR", "RTS"]  # Round-Robin, Risk-Time-Size
WORKLOADS = ["heterogeneous-smoke", "deterministic-full"]
TERMINAL_STATUSES = {"completed", "failed", "cancelled"}


# ---------------------------------------------------------------------------
# Data models
# ---------------------------------------------------------------------------

@dataclass
class ScenarioResult:
    scenario: str
    scheduler: str
    workload: str
    tasks_submitted: int = 0
    tasks_completed: int = 0
    tasks_failed: int = 0
    duration_seconds: float = 0.0
    success_rate: float = 0.0
    avg_wait_seconds: float = 0.0
    error: str = ""


@dataclass
class CampaignReport:
    started_at: str = ""
    finished_at: str = ""
    total_duration_seconds: float = 0.0
    scenarios_run: int = 0
    results: List[Dict] = field(default_factory=list)
    summary: Dict = field(default_factory=dict)


# ---------------------------------------------------------------------------
# HTTP helpers
# ---------------------------------------------------------------------------

def request_json(method: str, url: str, payload: Optional[Dict] = None, timeout: float = 15.0) -> Dict:
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url=url, method=method, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
        return json.loads(body) if body else {}


def set_scheduler(master_url: str, algo: str) -> bool:
    """Switch the master's active scheduler algorithm."""
    try:
        resp = request_json("POST", f"{master_url}/api/config/scheduler", {"algorithm": algo})
        return resp.get("success", False)
    except Exception:
        # Some deployments may not support dynamic switching;
        # the scheduler is set via SCHED_ALGO env var instead.
        return False


# ---------------------------------------------------------------------------
# Workload submission + polling
# ---------------------------------------------------------------------------

def load_workload(workload_name: str) -> List[Dict]:
    """Load a workload JSON from testbench/workloads/."""
    workload_dir = pathlib.Path(__file__).resolve().parents[1] / "workloads"
    path = workload_dir / f"{workload_name}.json"
    if not path.exists():
        raise FileNotFoundError(f"Workload not found: {path}")
    data = json.loads(path.read_text(encoding="utf-8"))
    return data.get("tasks", [])


def submit_tasks(master_url: str, tasks: List[Dict]) -> List[str]:
    """Submit tasks and return list of task IDs."""
    task_ids = []
    for task in tasks:
        payload = {
            "docker_image": task["docker_image"],
            "command": task.get("command", ""),
            "cpu_required": task["cpu_required"],
            "memory_required": task["memory_required"],
            "storage_required": task.get("storage_required", 1),
            "tag": task.get("tag", ""),
            "k_value": task.get("k_value", 2.0),
            "user_id": "campaign",
        }
        resp = request_json("POST", f"{master_url}/api/tasks", payload=payload, timeout=20.0)
        tid = resp.get("task_id", "")
        if tid:
            task_ids.append(tid)

        delay = float(task.get("arrival_delay_sec", 0.0))
        if delay > 0:
            time.sleep(delay)
    return task_ids


def poll_completion(master_url: str, task_ids: List[str], timeout: int = 600, interval: float = 3.0) -> Dict[str, str]:
    """Wait for all tasks to reach terminal status. Returns {task_id: status}."""
    deadline = time.time() + timeout
    statuses: Dict[str, str] = {tid: "queued" for tid in task_ids}

    while time.time() < deadline:
        done = 0
        for tid in task_ids:
            if statuses[tid] in TERMINAL_STATUSES:
                done += 1
                continue
            try:
                info = request_json("GET", f"{master_url}/api/tasks/{tid}", timeout=10.0)
                statuses[tid] = str(info.get("status", "unknown")).lower()
                if statuses[tid] in TERMINAL_STATUSES:
                    done += 1
            except Exception:
                pass

        if done == len(task_ids):
            return statuses
        time.sleep(interval)

    return statuses


# ---------------------------------------------------------------------------
# Scenario runners
# ---------------------------------------------------------------------------

def run_baseline(master_url: str, workload: str, scheduler: str) -> ScenarioResult:
    """Run a clean baseline: submit workload under a specific scheduler."""
    result = ScenarioResult(scenario="baseline", scheduler=scheduler, workload=workload)
    try:
        set_scheduler(master_url, scheduler)
        tasks = load_workload(workload)
        result.tasks_submitted = len(tasks)

        start = time.time()
        task_ids = submit_tasks(master_url, tasks)
        statuses = poll_completion(master_url, task_ids)
        elapsed = time.time() - start

        result.duration_seconds = round(elapsed, 2)
        result.tasks_completed = sum(1 for s in statuses.values() if s == "completed")
        result.tasks_failed = sum(1 for s in statuses.values() if s in ("failed", "cancelled"))
        result.success_rate = round(result.tasks_completed / max(result.tasks_submitted, 1) * 100, 1)
    except Exception as e:
        result.error = str(e)
    return result


def run_burst(master_url: str, workload: str, scheduler: str) -> ScenarioResult:
    """Run burst scenario: submit all tasks simultaneously (no delay)."""
    result = ScenarioResult(scenario="burst", scheduler=scheduler, workload=workload)
    try:
        set_scheduler(master_url, scheduler)
        tasks = load_workload(workload)
        # Strip any arrival delays to create bursty submission
        for t in tasks:
            t.pop("arrival_delay_sec", None)
        result.tasks_submitted = len(tasks)

        start = time.time()
        task_ids = submit_tasks(master_url, tasks)
        statuses = poll_completion(master_url, task_ids)
        elapsed = time.time() - start

        result.duration_seconds = round(elapsed, 2)
        result.tasks_completed = sum(1 for s in statuses.values() if s == "completed")
        result.tasks_failed = sum(1 for s in statuses.values() if s in ("failed", "cancelled"))
        result.success_rate = round(result.tasks_completed / max(result.tasks_submitted, 1) * 100, 1)
    except Exception as e:
        result.error = str(e)
    return result


def run_overload(master_url: str, workload: str, scheduler: str) -> ScenarioResult:
    """Run overload scenario: submit workload 3x to saturate cluster."""
    result = ScenarioResult(scenario="overload", scheduler=scheduler, workload=workload)
    try:
        set_scheduler(master_url, scheduler)
        tasks = load_workload(workload)
        tasks_3x = tasks * 3  # Triple the workload
        result.tasks_submitted = len(tasks_3x)

        start = time.time()
        task_ids = submit_tasks(master_url, tasks_3x)
        statuses = poll_completion(master_url, task_ids, timeout=1200)
        elapsed = time.time() - start

        result.duration_seconds = round(elapsed, 2)
        result.tasks_completed = sum(1 for s in statuses.values() if s == "completed")
        result.tasks_failed = sum(1 for s in statuses.values() if s in ("failed", "cancelled"))
        result.success_rate = round(result.tasks_completed / max(result.tasks_submitted, 1) * 100, 1)
    except Exception as e:
        result.error = str(e)
    return result


SCENARIO_RUNNERS = {
    "baseline": run_baseline,
    "burst": run_burst,
    "overload": run_overload,
}


# ---------------------------------------------------------------------------
# Report generation
# ---------------------------------------------------------------------------

def generate_report(results: List[ScenarioResult], started_at: float) -> CampaignReport:
    finished_at = time.time()
    report = CampaignReport(
        started_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(started_at)),
        finished_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(finished_at)),
        total_duration_seconds=round(finished_at - started_at, 2),
        scenarios_run=len(results),
        results=[asdict(r) for r in results],
    )

    # Build summary: compare schedulers across scenarios
    by_scheduler: Dict[str, List[ScenarioResult]] = {}
    for r in results:
        by_scheduler.setdefault(r.scheduler, []).append(r)

    scheduler_summary = {}
    for sched, sched_results in by_scheduler.items():
        total_tasks = sum(r.tasks_submitted for r in sched_results)
        total_completed = sum(r.tasks_completed for r in sched_results)
        total_failed = sum(r.tasks_failed for r in sched_results)
        avg_success = round(total_completed / max(total_tasks, 1) * 100, 1)
        avg_duration = round(sum(r.duration_seconds for r in sched_results) / max(len(sched_results), 1), 2)
        scheduler_summary[sched] = {
            "total_tasks": total_tasks,
            "total_completed": total_completed,
            "total_failed": total_failed,
            "avg_success_rate": avg_success,
            "avg_duration_seconds": avg_duration,
            "scenarios_run": len(sched_results),
        }

    report.summary = {
        "by_scheduler": scheduler_summary,
        "best_scheduler": max(scheduler_summary, key=lambda s: scheduler_summary[s]["avg_success_rate"]) if scheduler_summary else "",
    }
    return report


def write_markdown_report(report: CampaignReport, output_path: pathlib.Path) -> None:
    """Write a Markdown evidence report."""
    lines = [
        "# CloudAI Evidence Benchmark Campaign Report",
        "",
        f"**Started**: {report.started_at}",
        f"**Finished**: {report.finished_at}",
        f"**Duration**: {report.total_duration_seconds}s",
        f"**Scenarios executed**: {report.scenarios_run}",
        "",
        "## Scheduler Comparison",
        "",
        "| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration |",
        "|-----------|-------|-----------|--------|-------------|-------------|",
    ]

    for sched, stats in report.summary.get("by_scheduler", {}).items():
        lines.append(
            f"| {sched} | {stats['total_tasks']} | {stats['total_completed']} | "
            f"{stats['total_failed']} | {stats['avg_success_rate']}% | {stats['avg_duration_seconds']}s |"
        )

    best = report.summary.get("best_scheduler", "")
    if best:
        lines.extend(["", f"**Best scheduler**: {best}", ""])

    lines.extend(["", "## Scenario Details", ""])

    for r in report.results:
        lines.append(f"### {r['scenario']} / {r['scheduler']} / {r['workload']}")
        lines.append("")
        lines.append(f"- Submitted: {r['tasks_submitted']}")
        lines.append(f"- Completed: {r['tasks_completed']}")
        lines.append(f"- Failed: {r['tasks_failed']}")
        lines.append(f"- Success Rate: {r['success_rate']}%")
        lines.append(f"- Duration: {r['duration_seconds']}s")
        if r.get("error"):
            lines.append(f"- Error: {r['error']}")
        lines.append("")

    output_path.write_text("\n".join(lines), encoding="utf-8")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run CloudAI evidence benchmark campaign")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master API URL")
    parser.add_argument(
        "--scenarios", default="baseline",
        help="Comma-separated scenarios: baseline,burst,overload,all (default: baseline)",
    )
    parser.add_argument(
        "--schedulers", default="RR,RTS",
        help="Comma-separated schedulers to test (default: RR,RTS)",
    )
    parser.add_argument(
        "--workloads", default="heterogeneous-smoke",
        help="Comma-separated workload names (default: heterogeneous-smoke)",
    )
    parser.add_argument(
        "--output-dir", type=pathlib.Path,
        default=pathlib.Path(__file__).resolve().parents[2] / "results" / "campaign",
        help="Output directory for reports",
    )
    parser.add_argument("--timeout", type=int, default=600, help="Per-scenario timeout in seconds")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    started_at = time.time()

    # Parse scenario list
    if args.scenarios.strip().lower() == "all":
        scenarios = list(SCENARIO_RUNNERS.keys())
    else:
        scenarios = [s.strip() for s in args.scenarios.split(",") if s.strip()]

    schedulers = [s.strip() for s in args.schedulers.split(",") if s.strip()]
    workloads = [w.strip() for w in args.workloads.split(",") if w.strip()]

    print(f"Campaign: {len(scenarios)} scenario(s) x {len(schedulers)} scheduler(s) x {len(workloads)} workload(s)")
    print(f"Scenarios: {scenarios}")
    print(f"Schedulers: {schedulers}")
    print(f"Workloads: {workloads}")
    print()

    results: List[ScenarioResult] = []

    for scenario_name in scenarios:
        runner = SCENARIO_RUNNERS.get(scenario_name)
        if runner is None:
            print(f"Unknown scenario: {scenario_name}, skipping")
            continue

        for scheduler in schedulers:
            for workload in workloads:
                label = f"{scenario_name}/{scheduler}/{workload}"
                print(f"[campaign] Running {label}...")
                result = runner(args.master_url, workload, scheduler)
                results.append(result)
                if result.error:
                    print(f"[campaign]   ERROR: {result.error}")
                else:
                    print(f"[campaign]   done: {result.tasks_completed}/{result.tasks_submitted} "
                          f"({result.success_rate}%) in {result.duration_seconds}s")

    report = generate_report(results, started_at)

    # Write outputs
    timestamp = time.strftime("%Y%m%d-%H%M%S")
    output_dir = args.output_dir / timestamp
    output_dir.mkdir(parents=True, exist_ok=True)

    json_path = output_dir / "campaign-report.json"
    json_path.write_text(json.dumps(asdict(report), indent=2), encoding="utf-8")

    md_path = output_dir / "REPORT.md"
    write_markdown_report(report, md_path)

    print(f"\nCampaign complete: {report.scenarios_run} scenario(s) in {report.total_duration_seconds}s")
    print(f"Reports: {output_dir}")

    best = report.summary.get("best_scheduler", "")
    if best:
        print(f"Best scheduler: {best}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
