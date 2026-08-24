"""Performance and SLA metrics computation for Agentic Cloud Cluster benchmarks."""

from __future__ import annotations

import math
import statistics
from typing import Any, Dict, List

from .schema import SchedulerTrialResult, TaskExecutionRecord


def _percentile(values: List[float], p: float) -> float:
    """Compute empirical percentile value."""
    if not values:
        return 0.0
    sorted_vals = sorted(values)
    k = (len(sorted_vals) - 1) * (p / 100.0)
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return sorted_vals[int(k)]
    d0 = sorted_vals[int(f)] * (c - k)
    d1 = sorted_vals[int(c)] * (k - f)
    return float(d0 + d1)


def compute_trial_metrics(
    records: List[TaskExecutionRecord],
    scheduler: str,
    profile: str,
    total_duration_sec: float,
) -> SchedulerTrialResult:
    """Compute detailed SLA, latency, wait time, and throughput metrics for a trial."""
    submitted = len(records)
    completed_records = [r for r in records if r.status.upper() == "SUCCESS"]
    failed_records = [r for r in records if r.status.upper() != "SUCCESS"]

    completed = len(completed_records)
    failed = len(failed_records)
    success_rate = (completed / submitted * 100.0) if submitted > 0 else 0.0

    sla_met_count = sum(1 for r in completed_records if r.sla_met)
    sla_attainment_rate = (sla_met_count / completed * 100.0) if completed > 0 else 0.0

    turnarounds = [r.turnaround_sec for r in completed_records if r.turnaround_sec > 0]
    waits = [r.wait_duration_sec for r in completed_records if r.wait_duration_sec >= 0]

    avg_turnaround = statistics.mean(turnarounds) if turnarounds else 0.0
    p50_turnaround = _percentile(turnarounds, 50.0)
    p90_turnaround = _percentile(turnarounds, 90.0)
    p95_turnaround = _percentile(turnarounds, 95.0)
    p99_turnaround = _percentile(turnarounds, 99.0)

    avg_wait = statistics.mean(waits) if waits else 0.0
    p95_wait = _percentile(waits, 95.0)

    # Worker load distribution
    worker_counts: Dict[str, int] = {}
    for r in completed_records:
        w = r.worker_id or "unknown"
        worker_counts[w] = worker_counts.get(w, 0) + 1

    worker_variance = 0.0
    if len(worker_counts) > 1:
        counts = list(worker_counts.values())
        worker_variance = float(statistics.stdev(counts))

    return SchedulerTrialResult(
        scheduler=scheduler,
        profile=profile,
        tasks_submitted=submitted,
        tasks_completed=completed,
        tasks_failed=failed,
        success_rate=round(success_rate, 2),
        sla_attainment_rate=round(sla_attainment_rate, 2),
        total_duration_sec=round(total_duration_sec, 2),
        avg_turnaround_sec=round(avg_turnaround, 3),
        p50_turnaround_sec=round(p50_turnaround, 3),
        p90_turnaround_sec=round(p90_turnaround, 3),
        p95_turnaround_sec=round(p95_turnaround, 3),
        p99_turnaround_sec=round(p99_turnaround, 3),
        avg_wait_sec=round(avg_wait, 3),
        p95_wait_sec=round(p95_wait, 3),
        makespan_sec=round(total_duration_sec, 2),
        worker_load_variance=round(worker_variance, 3),
        task_records=records,
        metadata={
            "worker_distribution": worker_counts,
            "sla_met_count": sla_met_count,
        },
    )


def compute_comparative_summary(trials: List[SchedulerTrialResult]) -> Dict[str, Any]:
    """Generate side-by-side comparison matrix across evaluated schedulers."""
    matrix: Dict[str, Any] = {}

    for t in trials:
        key = f"{t.profile}::{t.scheduler}"
        matrix[key] = {
            "profile": t.profile,
            "scheduler": t.scheduler,
            "tasks": t.tasks_submitted,
            "sla_attainment_%": t.sla_attainment_rate,
            "success_rate_%": t.success_rate,
            "p50_turnaround_s": t.p50_turnaround_sec,
            "p95_turnaround_s": t.p95_turnaround_sec,
            "avg_wait_s": t.avg_wait_sec,
            "makespan_s": t.makespan_sec,
        }

    return matrix
