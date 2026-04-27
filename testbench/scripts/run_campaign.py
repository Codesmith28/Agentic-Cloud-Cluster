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
import os
import pathlib
import shlex
import subprocess
import sys
import time
from dataclasses import dataclass, field, asdict
from typing import Any, Dict, List

from shared_polling import poll_task_completion, request_json, TERMINAL_STATUSES

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

SCHEDULERS = ["RR", "RTS", "PPO"]
WORKLOADS = ["heterogeneous-smoke", "steady-cpu", "steady-mixed", "memory-pressure", "bursty", "long-tail"]
DEFAULT_WORKFLOW_IMAGE = "cloudai/workflow-deterministic:v1"


# ---------------------------------------------------------------------------
# Data models
# ---------------------------------------------------------------------------

@dataclass
class ScenarioResult:
    scenario: str
    scheduler: str
    scheduler_algorithm: str
    workload: str
    suite: str = "normal"
    tasks_submitted: int = 0
    tasks_completed: int = 0
    tasks_failed: int = 0
    duration_seconds: float = 0.0
    success_rate: float = 0.0
    avg_wait_seconds: float = 0.0
    avg_turnaround_seconds: float = 0.0
    p95_turnaround_seconds: float = 0.0
    task_ids: List[str] = field(default_factory=list)
    error: str = ""


@dataclass
class CampaignReport:
    started_at: str = ""
    finished_at: str = ""
    total_duration_seconds: float = 0.0
    scenarios_run: int = 0
    results: List[Dict] = field(default_factory=list)
    summary: Dict = field(default_factory=dict)


def set_scheduler(master_url: str, algo: str) -> bool:
    """Switch the master's active scheduler algorithm via API."""
    try:
        resp = request_json("POST", f"{master_url}/api/config/scheduler", {"algorithm": algo})
        ok = resp.get("success", False)
        if ok:
            print(f"[campaign]   scheduler switched to {algo}")
        else:
            print(f"[campaign]   WARNING: scheduler switch to {algo} failed: {resp.get('error', 'unknown')}")
        return ok
    except Exception as e:
        print(f"[campaign]   WARNING: scheduler switch to {algo} failed: {e}")
        return False


def verify_scheduler(master_url: str, expected: str) -> bool:
    """Verify the active scheduler matches expectations."""
    try:
        resp = request_json("GET", f"{master_url}/api/config/scheduler", timeout=5.0)
        # GET returns {"algorithm": "Round-Robin"} — normalise to short code
        raw = resp.get("algorithm", resp.get("current", ""))
        current = resolve_scheduler_algorithm(raw)
        if current == resolve_scheduler_algorithm(expected):
            return True
        print(f"[campaign]   WARNING: scheduler mismatch: expected {expected}, got {raw!r} (resolved: {current})")
        return False
    except Exception as e:
        print(f"[campaign]   WARNING: could not verify scheduler: {e}")
        return False


def switch_scheduler(master_url: str, algo: str) -> bool:
    """Switch scheduler and verify. Returns False on failure."""
    if not set_scheduler(master_url, algo):
        return False
    return verify_scheduler(master_url, algo)


def drain_cluster(master_url: str, timeout_seconds: int = 120) -> bool:
    """Wait until all tasks are terminal and workers are idle."""
    deadline = time.time() + timeout_seconds
    consecutive_idle = 0
    while time.time() < deadline:
        try:
            tasks_resp = request_json("GET", f"{master_url}/api/tasks", timeout=10.0)
            tasks = tasks_resp.get("tasks", [])
            active_tasks = [
                t for t in tasks
                if str(t.get("status", "")).lower() not in TERMINAL_STATUSES
            ]
            if not active_tasks:
                consecutive_idle += 1
                if consecutive_idle >= 2:
                    return True
            else:
                consecutive_idle = 0
                remaining = len(active_tasks)
                print(f"[campaign]   draining: {remaining} task(s) still active...")
        except Exception:
            consecutive_idle = 0

        time.sleep(3.0)
    print(f"[campaign]   WARNING: drain timeout after {timeout_seconds}s")
    return False


def check_clean_slate(master_url: str) -> bool:
    """Verify no non-terminal tasks exist before starting campaign."""
    try:
        resp = request_json("GET", f"{master_url}/api/tasks", timeout=10.0)
        tasks = resp.get("tasks", [])
        active = [t for t in tasks if str(t.get("status", "")).lower() not in TERMINAL_STATUSES]
        if active:
            print(f"[campaign] WARNING: {len(active)} non-terminal task(s) found — cluster not clean")
            print(f"[campaign]   Draining existing tasks before starting...")
            return drain_cluster(master_url, timeout_seconds=60)
        return True
    except Exception:
        return True


def compute_run_metrics(master_url: str, task_ids: List[str], result: ScenarioResult) -> None:
    """Compute per-run queue wait and turnaround times from task timestamps."""
    wait_times: List[float] = []
    turnaround_times: List[float] = []

    for task_id in task_ids:
        try:
            info = request_json("GET", f"{master_url}/api/tasks/{task_id}", timeout=10.0)
        except Exception:
            continue

        created = info.get("created_at", "")
        assigned = info.get("assigned_at", "")
        completed = info.get("completed_at", "")

        try:
            if created and assigned:
                c_ts = _parse_ts(created)
                a_ts = _parse_ts(assigned)
                if c_ts and a_ts:
                    wait_times.append(a_ts - c_ts)
            if created and completed:
                c_ts = _parse_ts(created)
                d_ts = _parse_ts(completed)
                if c_ts and d_ts:
                    turnaround_times.append(d_ts - c_ts)
        except Exception:
            continue

    if wait_times:
        result.avg_wait_seconds = round(sum(wait_times) / len(wait_times), 3)
    if turnaround_times:
        turnaround_times.sort()
        result.avg_turnaround_seconds = round(sum(turnaround_times) / len(turnaround_times), 3)
        p95_idx = int(len(turnaround_times) * 0.95)
        result.p95_turnaround_seconds = round(turnaround_times[min(p95_idx, len(turnaround_times) - 1)], 3)


def _parse_ts(ts_str: str) -> float | None:
    """Parse an ISO-8601 or RFC-3339 timestamp to Unix seconds."""
    if not ts_str or ts_str == "0001-01-01T00:00:00Z":
        return None
    # Go's time.Time zero value
    for fmt in ("%Y-%m-%dT%H:%M:%S.%fZ", "%Y-%m-%dT%H:%M:%SZ", "%Y-%m-%dT%H:%M:%S%z"):
        try:
            from datetime import datetime, timezone
            dt = datetime.strptime(ts_str, fmt)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt.timestamp()
        except ValueError:
            continue
    return None


def resolve_scheduler_algorithm(scheduler_label: str) -> str:
    normalized = scheduler_label.strip().upper()
    if normalized.startswith("PPO"):
        return "PPO"
    if normalized.startswith("RR"):
        return "RR"
    if normalized.startswith("RTS"):
        return "RTS"
    return normalized



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


def resolve_workflow_image(task: Dict[str, Any]) -> str:
    if "docker_image" in task and str(task["docker_image"]).strip():
        return str(task["docker_image"]).strip()

    if task.get("workflow") or task.get("workflow_params") or task.get("workflow_profile"):
        return os.environ.get("CLOUDAI_WORKFLOW_IMAGE_TAG", DEFAULT_WORKFLOW_IMAGE)

    raise ValueError("task missing docker_image")


def resolve_command(task: Dict[str, Any]) -> str:
    workflow = task.get("workflow") or task.get("workflow_params")
    legacy_profile = task.get("workflow_profile")
    legacy_args = task.get("workflow_args")

    if workflow is None and legacy_profile:
        cmd_parts: List[str] = ["/usr/local/bin/workflow", str(legacy_profile)]
        if legacy_args is None:
            return shlex.join(cmd_parts)
        if isinstance(legacy_args, list):
            cmd_parts.extend(str(item) for item in legacy_args if item is not None)
            return shlex.join(cmd_parts)
        if isinstance(legacy_args, str):
            cmd_parts.extend(shlex.split(legacy_args))
            return shlex.join(cmd_parts)
        raise ValueError("workflow_args must be a list or string")

    if workflow is None:
        return str(task.get("command", "")).strip()

    if not isinstance(workflow, dict):
        raise ValueError(f"workflow block must be an object, got {type(workflow).__name__}")

    profile = workflow.get("profile") or workflow.get("subcommand")
    if not profile:
        raise ValueError("workflow block requires profile")

    args = workflow.get("args", {})
    if not isinstance(args, dict):
        raise ValueError("workflow args must be an object")

    cmd_parts: List[str] = ["/usr/local/bin/workflow", str(profile)]
    for key in sorted(args.keys()):
        flag = f"--{key}"
        value = args[key]
        if value is None:
            continue
        if isinstance(value, bool):
            if value:
                cmd_parts.append(flag)
            continue
        if isinstance(value, list):
            for item in value:
                cmd_parts.extend([flag, str(item)])
            continue
        cmd_parts.extend([flag, str(value)])

    return shlex.join(cmd_parts)


def submit_tasks(master_url: str, tasks: List[Dict]) -> List[str]:
    """Submit tasks and return list of task IDs."""
    task_ids = []
    for task in tasks:
        docker_image = resolve_workflow_image(task)
        command = resolve_command(task)
        payload = {
            "docker_image": docker_image,
            "command": command,
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


# ---------------------------------------------------------------------------
# Scenario runners
# ---------------------------------------------------------------------------

def run_baseline(master_url: str, workload: str, scheduler: str, timeout_seconds: int = 600) -> ScenarioResult:
    """Run a clean baseline: submit workload under a specific scheduler."""
    scheduler_algo = resolve_scheduler_algorithm(scheduler)
    result = ScenarioResult(
        scenario="baseline",
        scheduler=scheduler,
        scheduler_algorithm=scheduler_algo,
        workload=workload,
        suite="normal",
    )
    try:
        if not switch_scheduler(master_url, scheduler_algo):
            result.error = f"Failed to switch scheduler to {scheduler_algo}"
            return result
        tasks = load_workload(workload)
        result.tasks_submitted = len(tasks)

        start = time.time()
        task_ids = submit_tasks(master_url, tasks)
        result.task_ids = task_ids
        statuses = poll_task_completion(master_url, task_ids, timeout_seconds=timeout_seconds)
        elapsed = time.time() - start

        result.duration_seconds = round(elapsed, 2)
        result.tasks_completed = sum(1 for s in statuses.values() if s == "completed")
        result.tasks_failed = sum(1 for s in statuses.values() if s in ("failed", "cancelled"))
        result.success_rate = round(result.tasks_completed / max(result.tasks_submitted, 1) * 100, 1)
        compute_run_metrics(master_url, task_ids, result)
    except Exception as e:
        result.error = str(e)
    return result


def run_burst(master_url: str, workload: str, scheduler: str, timeout_seconds: int = 600) -> ScenarioResult:
    """Run burst scenario: submit all tasks simultaneously (no delay)."""
    scheduler_algo = resolve_scheduler_algorithm(scheduler)
    result = ScenarioResult(
        scenario="burst",
        scheduler=scheduler,
        scheduler_algorithm=scheduler_algo,
        workload=workload,
        suite="normal",
    )
    try:
        if not switch_scheduler(master_url, scheduler_algo):
            result.error = f"Failed to switch scheduler to {scheduler_algo}"
            return result
        tasks = load_workload(workload)
        # Strip any arrival delays to create bursty submission
        for t in tasks:
            t.pop("arrival_delay_sec", None)
        result.tasks_submitted = len(tasks)

        start = time.time()
        task_ids = submit_tasks(master_url, tasks)
        result.task_ids = task_ids
        statuses = poll_task_completion(master_url, task_ids, timeout_seconds=timeout_seconds)
        elapsed = time.time() - start

        result.duration_seconds = round(elapsed, 2)
        result.tasks_completed = sum(1 for s in statuses.values() if s == "completed")
        result.tasks_failed = sum(1 for s in statuses.values() if s in ("failed", "cancelled"))
        result.success_rate = round(result.tasks_completed / max(result.tasks_submitted, 1) * 100, 1)
        compute_run_metrics(master_url, task_ids, result)
    except Exception as e:
        result.error = str(e)
    return result


def run_overload(master_url: str, workload: str, scheduler: str, timeout_seconds: int = 1200) -> ScenarioResult:
    """Run overload scenario: submit workload 3x to saturate cluster."""
    scheduler_algo = resolve_scheduler_algorithm(scheduler)
    result = ScenarioResult(
        scenario="overload",
        scheduler=scheduler,
        scheduler_algorithm=scheduler_algo,
        workload=workload,
        suite="normal",
    )
    try:
        if not switch_scheduler(master_url, scheduler_algo):
            result.error = f"Failed to switch scheduler to {scheduler_algo}"
            return result
        tasks = load_workload(workload)
        tasks_3x = tasks * 3  # Triple the workload
        result.tasks_submitted = len(tasks_3x)

        start = time.time()
        task_ids = submit_tasks(master_url, tasks_3x)
        result.task_ids = task_ids
        statuses = poll_task_completion(master_url, task_ids, timeout_seconds=timeout_seconds)
        elapsed = time.time() - start

        result.duration_seconds = round(elapsed, 2)
        result.tasks_completed = sum(1 for s in statuses.values() if s == "completed")
        result.tasks_failed = sum(1 for s in statuses.values() if s in ("failed", "cancelled"))
        result.success_rate = round(result.tasks_completed / max(result.tasks_submitted, 1) * 100, 1)
        compute_run_metrics(master_url, task_ids, result)
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
        wait_vals = [r.avg_wait_seconds for r in sched_results if r.avg_wait_seconds > 0]
        avg_wait = round(sum(wait_vals) / max(len(wait_vals), 1), 3) if wait_vals else 0.0
        ta_vals = [r.avg_turnaround_seconds for r in sched_results if r.avg_turnaround_seconds > 0]
        avg_turnaround = round(sum(ta_vals) / max(len(ta_vals), 1), 3) if ta_vals else 0.0
        p95_vals = [r.p95_turnaround_seconds for r in sched_results if r.p95_turnaround_seconds > 0]
        avg_p95 = round(sum(p95_vals) / max(len(p95_vals), 1), 3) if p95_vals else 0.0
        scheduler_summary[sched] = {
            "total_tasks": total_tasks,
            "total_completed": total_completed,
            "total_failed": total_failed,
            "avg_success_rate": avg_success,
            "avg_duration_seconds": avg_duration,
            "avg_queue_wait_seconds": avg_wait,
            "avg_turnaround_seconds": avg_turnaround,
            "p95_turnaround_seconds": avg_p95,
            "scenarios_run": len(sched_results),
        }

    report.summary = {
        "by_scheduler": scheduler_summary,
        "best_scheduler": max(scheduler_summary, key=lambda s: scheduler_summary[s]["avg_success_rate"]) if scheduler_summary else "",
    }
    return report


def write_markdown_report(report: CampaignReport, output_path: pathlib.Path) -> None:
    """Write a Markdown evidence report with paper comparison."""
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
        "| Scheduler | Tasks | Completed | Failed | Success Rate | Avg Duration | Avg Queue Wait | Avg Turnaround | P95 Turnaround |",
        "|-----------|-------|-----------|--------|-------------|-------------|---------------|---------------|----------------|",
    ]

    for sched, stats in report.summary.get("by_scheduler", {}).items():
        lines.append(
            f"| {sched} | {stats['total_tasks']} | {stats['total_completed']} | "
            f"{stats['total_failed']} | {stats['avg_success_rate']}% | {stats['avg_duration_seconds']}s | "
            f"{stats.get('avg_queue_wait_seconds', 'N/A')}s | "
            f"{stats.get('avg_turnaround_seconds', 'N/A')}s | "
            f"{stats.get('p95_turnaround_seconds', 'N/A')}s |"
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
        lines.append(f"- Avg Queue Wait: {r.get('avg_wait_seconds', 0)}s")
        lines.append(f"- Avg Turnaround: {r.get('avg_turnaround_seconds', 0)}s")
        lines.append(f"- P95 Turnaround: {r.get('p95_turnaround_seconds', 0)}s")
        if r.get("error"):
            lines.append(f"- Error: {r['error']}")
        lines.append("")

    # Paper comparison section
    lines.extend(_paper_comparison_section(report))

    output_path.write_text("\n".join(lines), encoding="utf-8")


def _paper_comparison_section(report: CampaignReport) -> List[str]:
    """Generate a qualitative comparison with published scheduling literature."""
    lines = [
        "## Comparison with Published Results",
        "",
        "### Reference: SAC-CS (Soft Actor-Critic for Container Scheduling)",
        "",
        "> Taha, A.; Maher, S.; Manimurugan, S.; Taha, M.; Amin, E. \"Optimized Container",
        "> Scheduling: A Soft Actor-Critic Deep Reinforcement Learning Approach.\"",
        "> *Computers* 2025, 14, 560. https://doi.org/10.3390/computers14120560",
        "",
        "**SAC-CS Key Claims:**",
        "",
        "| Metric | SAC-CS (Paper) | Method |",
        "|--------|---------------|--------|",
        "| Optimization target | Min execution time + energy | Multi-objective reward |",
        "| State space | 6 features/host (affinity, speed, idle, diff-CPU/mem/GPU) | Flattened N×6 vector |",
        "| Action space | Discrete (host index) | Stochastic policy sampling |",
        "| Discount factor (γ) | 0.01 | Short-horizon, immediate-reward focus |",
        "| Batch size | 128 | Replay buffer training |",
        "| Algorithm | SAC with twin critics + entropy regularization | Maximum entropy RL |",
        "",
        "**Our Approach (PPO-based) vs SAC-CS:**",
        "",
        "| Aspect | SAC-CS (Paper) | Our PPO Scheduler |",
        "|--------|---------------|-------------------|",
        "| RL Algorithm | Soft Actor-Critic | Proximal Policy Optimization |",
        "| Policy type | Stochastic (entropy-regularized) | Stochastic (clipped surrogate) |",
        "| Exploration | Entropy bonus (automatic temperature α) | GAE + entropy coefficient |",
        "| Training data | Simulated datacenter tasks | Alibaba cluster-trace-v2018 (200K real tasks) |",
        "| Online adaptation | Not described | Online PPO updates from live cluster feedback |",
        "| Baselines | Random, Round-Robin, First-Fit | Round-Robin (RR), Risk-aware (RTS) |",
        "| Evaluation | Simulated environment only | Live Docker cluster with real task execution |",
        "",
    ]

    # Add our results summary
    by_sched = report.summary.get("by_scheduler", {})
    if by_sched:
        lines.extend([
            "**Our Benchmark Results Summary:**",
            "",
            "| Scheduler | Success Rate | Avg Duration | Avg Queue Wait | Avg Turnaround |",
            "|-----------|-------------|-------------|---------------|---------------|",
        ])
        for sched in ["RR", "RTS", "PPO"]:
            stats = by_sched.get(sched, {})
            if stats:
                lines.append(
                    f"| {sched} | {stats.get('avg_success_rate', 'N/A')}% | "
                    f"{stats.get('avg_duration_seconds', 'N/A')}s | "
                    f"{stats.get('avg_queue_wait_seconds', 'N/A')}s | "
                    f"{stats.get('avg_turnaround_seconds', 'N/A')}s |"
                )
        lines.append("")

        ppo = by_sched.get("PPO", {})
        rr = by_sched.get("RR", {})
        if ppo and rr:
            rr_dur = rr.get("avg_duration_seconds", 0)
            ppo_dur = ppo.get("avg_duration_seconds", 0)
            if rr_dur > 0:
                improvement = round((rr_dur - ppo_dur) / rr_dur * 100, 1)
                lines.append(f"**PPO duration improvement over RR**: {improvement}%")
            rr_ta = rr.get("avg_turnaround_seconds", 0)
            ppo_ta = ppo.get("avg_turnaround_seconds", 0)
            if rr_ta > 0:
                ta_improvement = round((rr_ta - ppo_ta) / rr_ta * 100, 1)
                lines.append(f"**PPO turnaround improvement over RR**: {ta_improvement}%")
            lines.append("")

    lines.extend([
        "### Methodology Notes",
        "",
        "1. **Different evaluation harnesses**: SAC-CS evaluates in simulation; our results",
        "   come from a live Docker cluster with real container execution. Direct numeric",
        "   comparison is not appropriate — the environments measure different things.",
        "2. **Training data**: Our PPO model is pre-trained on the Alibaba cluster-trace-v2018",
        "   dataset (199,614 real production tasks from 17,592 machines) [1], providing a more",
        "   realistic training signal than synthetic workloads.",
        "3. **Online learning**: Unlike SAC-CS, our PPO continues learning from live cluster",
        "   feedback during deployment, adapting to the actual workload distribution.",
        "4. **Cluster topology**: 3-node heterogeneous cluster (small: 1 CPU/1.5 GB,",
        "   medium: 2 CPU/3 GB, large: 3 CPU/5 GB) with Docker-in-Docker task execution.",
        "",
        "### References",
        "",
        "[1] Alibaba Cluster Trace Program. \"cluster-trace-v2018.\"",
        "    https://github.com/alibaba/clusterdata, 2018.",
        "",
        "[2] Schulman, J.; Wolski, F.; Dhariwal, P.; Radford, A.; Klimov, O.",
        "    \"Proximal Policy Optimization Algorithms.\" *arXiv preprint arXiv:1707.06347*, 2017.",
        "",
        "[3] Taha, A. et al. \"Optimized Container Scheduling: A Soft Actor-Critic Deep",
        "    Reinforcement Learning Approach.\" *Computers* 2025, 14, 560.",
        "    https://doi.org/10.3390/computers14120560",
        "",
    ])

    return lines


def write_html_report(report: CampaignReport, output_path: pathlib.Path) -> None:
    summary_rows = []
    for sched, stats in report.summary.get("by_scheduler", {}).items():
        summary_rows.append(
            "<tr>"
            f"<td>{sched}</td>"
            f"<td>{stats['total_tasks']}</td>"
            f"<td>{stats['total_completed']}</td>"
            f"<td>{stats['total_failed']}</td>"
            f"<td>{stats['avg_success_rate']}%</td>"
            f"<td>{stats['avg_duration_seconds']}s</td>"
            f"<td>{stats.get('avg_queue_wait_seconds', 'N/A')}s</td>"
            f"<td>{stats.get('avg_turnaround_seconds', 'N/A')}s</td>"
            f"<td>{stats.get('p95_turnaround_seconds', 'N/A')}s</td>"
            "</tr>"
        )

    detail_rows = []
    for result in report.results:
        detail_rows.append(
            "<tr>"
            f"<td>{result.get('scenario', '')}</td>"
            f"<td>{result.get('scheduler', '')}</td>"
            f"<td>{result.get('scheduler_algorithm', '')}</td>"
            f"<td>{result.get('workload', '')}</td>"
            f"<td>{result.get('tasks_submitted', 0)}</td>"
            f"<td>{result.get('tasks_completed', 0)}</td>"
            f"<td>{result.get('tasks_failed', 0)}</td>"
            f"<td>{result.get('success_rate', 0)}%</td>"
            f"<td>{result.get('duration_seconds', 0)}s</td>"
            "</tr>"
        )

    html = f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>CloudAI Campaign Report</title>
  <style>
    body {{ font-family: Arial, sans-serif; margin: 24px; }}
    table {{ border-collapse: collapse; width: 100%; margin-bottom: 24px; }}
    th, td {{ border: 1px solid #ddd; padding: 8px; text-align: left; }}
    th {{ background: #f5f5f5; }}
  </style>
</head>
<body>
  <h1>CloudAI Evidence Benchmark Campaign Report</h1>
  <p><strong>Started:</strong> {report.started_at}</p>
  <p><strong>Finished:</strong> {report.finished_at}</p>
  <p><strong>Duration:</strong> {report.total_duration_seconds}s</p>
  <p><strong>Scenarios executed:</strong> {report.scenarios_run}</p>

  <h2>Scheduler Comparison</h2>
  <table>
    <thead>
      <tr><th>Scheduler</th><th>Tasks</th><th>Completed</th><th>Failed</th><th>Success Rate</th><th>Avg Duration</th><th>Queue Wait</th><th>Turnaround</th><th>P95 Turnaround</th></tr>
    </thead>
    <tbody>
      {''.join(summary_rows)}
    </tbody>
  </table>

  <h2>Scenario Details</h2>
  <table>
    <thead>
      <tr><th>Scenario</th><th>Scheduler Label</th><th>Algorithm</th><th>Workload</th><th>Submitted</th><th>Completed</th><th>Failed</th><th>Success</th><th>Duration</th></tr>
    </thead>
    <tbody>
      {''.join(detail_rows)}
    </tbody>
  </table>
</body>
</html>
"""
    output_path.write_text(html, encoding="utf-8")


def write_scheduler_summary_csv(report: CampaignReport, output_path: pathlib.Path) -> None:
    lines = ["scheduler,total_tasks,total_completed,total_failed,avg_success_rate,avg_duration_seconds,avg_queue_wait_seconds,avg_turnaround_seconds,p95_turnaround_seconds,scenarios_run"]
    for sched, stats in report.summary.get("by_scheduler", {}).items():
        lines.append(
            ",".join(
                [
                    sched,
                    str(stats.get("total_tasks", 0)),
                    str(stats.get("total_completed", 0)),
                    str(stats.get("total_failed", 0)),
                    str(stats.get("avg_success_rate", 0)),
                    str(stats.get("avg_duration_seconds", 0)),
                    str(stats.get("avg_queue_wait_seconds", 0)),
                    str(stats.get("avg_turnaround_seconds", 0)),
                    str(stats.get("p95_turnaround_seconds", 0)),
                    str(stats.get("scenarios_run", 0)),
                ]
            )
        )
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_task_attempt_timeline(master_url: str, results: List[ScenarioResult], output_path: pathlib.Path) -> None:
    header = [
        "scenario",
        "scheduler",
        "scheduler_algorithm",
        "workload",
        "task_id",
        "task_status",
        "current_attempt_id",
        "current_attempt_no",
        "recovery_count",
        "last_failure_reason",
        "attempt_id",
        "attempt_no",
        "attempt_status",
        "attempt_worker_id",
        "attempt_failure_reason",
        "assigned_at",
        "last_heartbeat",
        "completed_at",
    ]
    rows = [",".join(header)]

    for result in results:
        for task_id in result.task_ids:
            task_info: Dict[str, Any]
            try:
                task_info = request_json("GET", f"{master_url}/api/tasks/{task_id}", timeout=10.0)
            except Exception:
                task_info = {}

            task_status = str(task_info.get("status", "unknown"))
            current_attempt_id = str(task_info.get("current_attempt_id", ""))
            current_attempt_no = str(task_info.get("current_attempt_no", ""))
            recovery_count = str(task_info.get("recovery_count", ""))
            last_failure_reason = str(task_info.get("last_failure_reason", ""))

            attempts = task_info.get("attempts", []) if isinstance(task_info.get("attempts"), list) else []
            if not attempts:
                rows.append(
                    ",".join(
                        [
                            result.scenario,
                            result.scheduler,
                            result.scheduler_algorithm,
                            result.workload,
                            task_id,
                            task_status,
                            current_attempt_id,
                            current_attempt_no,
                            recovery_count,
                            last_failure_reason,
                            "",
                            "",
                            "",
                            "",
                            "",
                            "",
                            "",
                            "",
                        ]
                    )
                )
                continue

            for attempt in attempts:
                rows.append(
                    ",".join(
                        [
                            result.scenario,
                            result.scheduler,
                            result.scheduler_algorithm,
                            result.workload,
                            task_id,
                            task_status,
                            current_attempt_id,
                            current_attempt_no,
                            recovery_count,
                            last_failure_reason,
                            str(attempt.get("attempt_id", "")),
                            str(attempt.get("attempt_no", "")),
                            str(attempt.get("status", "")),
                            str(attempt.get("worker_id", "")),
                            str(attempt.get("failure_reason", "")),
                            str(attempt.get("assigned_at", "")),
                            str(attempt.get("last_heartbeat", "")),
                            str(attempt.get("completed_at", "")),
                        ]
                    )
                )

    output_path.write_text("\n".join(rows) + "\n", encoding="utf-8")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run CloudAI evidence benchmark campaign")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master API URL")
    parser.add_argument("--prometheus-url", default="http://localhost:9090", help="Prometheus API URL")
    parser.add_argument(
        "--scenarios", default="baseline,burst,overload",
        help="Comma-separated scenarios: baseline,burst,overload,all",
    )
    parser.add_argument(
        "--schedulers", default="RR,RTS,PPO",
        help="Comma-separated schedulers to test",
    )
    parser.add_argument(
        "--workloads", default="heterogeneous-smoke,steady-cpu,steady-mixed,memory-pressure,bursty,long-tail",
        help="Comma-separated workload names",
    )
    parser.add_argument(
        "--output-dir", type=pathlib.Path,
        default=pathlib.Path(__file__).resolve().parents[2] / "results" / "campaign",
        help="Output directory for reports",
    )
    parser.add_argument("--timeout", type=int, default=600, help="Per-scenario timeout in seconds")
    parser.add_argument(
        "--skip-observability-export",
        action="store_true",
        help="Skip exporting Prometheus/master observability artifacts at campaign end",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    started_at = time.time()

    # Pre-flight: verify master is reachable
    try:
        request_json("GET", f"{args.master_url}/health", timeout=5.0)
    except Exception:
        print(f"ERROR: Master API not reachable at {args.master_url}")
        print("  Make sure the master node is running before starting a campaign.")
        print("  Start it with: make run-master-ppo   (or: ./execute-tests.sh)")
        return 1

    # Pre-campaign: verify clean slate
    if not check_clean_slate(args.master_url):
        print("ERROR: Could not drain existing tasks. Cluster is not in a clean state.")
        return 1

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
    print(f"Timeout: {args.timeout}s per scenario")
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
                result = runner(args.master_url, workload, scheduler, timeout_seconds=args.timeout)
                results.append(result)
                if result.error:
                    print(f"[campaign]   ERROR: {result.error}")
                else:
                    print(f"[campaign]   done: {result.tasks_completed}/{result.tasks_submitted} "
                          f"({result.success_rate}%) in {result.duration_seconds}s "
                          f"[wait={result.avg_wait_seconds}s turnaround={result.avg_turnaround_seconds}s]")

                # Drain cluster between runs to prevent cross-contamination
                print(f"[campaign]   draining cluster before next run...")
                drain_cluster(args.master_url, timeout_seconds=120)

    report = generate_report(results, started_at)

    # Write outputs
    timestamp = time.strftime("%Y%m%d-%H%M%S")
    output_dir = args.output_dir / timestamp
    output_dir.mkdir(parents=True, exist_ok=True)

    json_path = output_dir / "campaign-report.json"
    json_path.write_text(json.dumps(asdict(report), indent=2), encoding="utf-8")

    md_path = output_dir / "REPORT.md"
    write_markdown_report(report, md_path)
    html_path = output_dir / "REPORT.html"
    write_html_report(report, html_path)
    write_scheduler_summary_csv(report, output_dir / "scheduler-summary.csv")
    write_task_attempt_timeline(args.master_url, results, output_dir / "task-attempt-timeline.csv")

    if not args.skip_observability_export:
        window_summary_path = output_dir / "campaign-window-summary.json"
        window_summary = {
            "started_at_unix": started_at,
            "finished_at_unix": time.time(),
        }
        window_summary_path.write_text(json.dumps(window_summary, indent=2), encoding="utf-8")

        export_script = pathlib.Path(__file__).resolve().parent / "export_metrics.py"
        observability_dir = output_dir / "observability"
        subprocess.run(
            [
                sys.executable,
                str(export_script),
                "--prometheus-url",
                args.prometheus_url,
                "--master-url",
                args.master_url,
                "--summary",
                str(window_summary_path),
                "--output-dir",
                str(observability_dir),
            ],
            check=False,
        )

    print(f"\nCampaign complete: {report.scenarios_run} scenario(s) in {report.total_duration_seconds}s")
    print(f"Reports: {output_dir}")

    best = report.summary.get("best_scheduler", "")
    if best:
        print(f"Best scheduler: {best}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
