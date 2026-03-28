"""Trace data loaders for Alibaba cluster-trace-v2018 and Google ClusterData2019.

Each loader reads CSV/JSON trace files and produces a list of ``TraceTask``
records sorted by arrival time, suitable for replay in ``TraceReplayEnv``.
"""

from __future__ import annotations

import csv
import json
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Sequence

LOGGER = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Canonical data model
# ---------------------------------------------------------------------------

@dataclass
class TraceTask:
    """A single task extracted from a cluster trace."""

    task_id: str
    arrival_time: float  # seconds from trace start
    req_cpu: float
    req_memory: float
    req_storage: float = 1.0
    runtime_seconds: float = 30.0  # actual duration (for reward)
    task_type: str = "mixed"
    sla_multiplier: float = 2.0


@dataclass
class TraceCluster:
    """Cluster configuration inferred from a trace."""

    workers: List[Dict]  # list of {worker_id, total_cpu, total_memory, total_storage}
    tasks: List[TraceTask]
    source: str = ""
    description: str = ""


# ---------------------------------------------------------------------------
# Alibaba cluster-trace-v2018
# ---------------------------------------------------------------------------

# Expected CSV columns (batch_task.csv):
#   task_name, instance_num, job_name, task_type, status,
#   start_time, end_time, plan_cpu, plan_mem
ALIBABA_TASK_TYPE_MAP = {
    "1": "cpu-light",
    "2": "cpu-heavy",
    "3": "memory-heavy",
    "4": "cpu-heavy",
    "5": "memory-heavy",
    "6": "mixed",
    "7": "mixed",
    "8": "cpu-light",
    "9": "mixed",
    "10": "cpu-heavy",
    "11": "mixed",
    "12": "mixed",
}


def load_alibaba_trace(
    trace_dir: str | Path,
    max_tasks: int = 5000,
    machine_csv: str = "machine_meta.csv",
    task_csv: str = "batch_task.csv",
) -> TraceCluster:
    """Load tasks and machines from Alibaba cluster-trace-v2018 CSVs.

    The trace directory should contain at minimum ``batch_task.csv`` (task
    definitions) and ``machine_meta.csv`` (machine specs).

    Parameters
    ----------
    trace_dir:
        Path to the directory containing the trace CSV files.
    max_tasks:
        Maximum number of tasks to load (sorted by start_time).
    machine_csv:
        Filename for machine metadata CSV.
    task_csv:
        Filename for batch task CSV.
    """
    trace_dir = Path(trace_dir)

    # --- Load machines ---
    workers: List[Dict] = []
    machine_path = trace_dir / machine_csv
    if machine_path.exists():
        with open(machine_path, newline="", encoding="utf-8") as fh:
            reader = csv.DictReader(fh)
            for idx, row in enumerate(reader):
                workers.append({
                    "worker_id": row.get("machine_id", f"machine-{idx}"),
                    "total_cpu": float(row.get("cpu_num", 64)),
                    "total_memory": float(row.get("mem_size", 128)),
                    "total_storage": float(row.get("disk_size", 1000)),
                })
        LOGGER.info("Loaded %d machines from %s", len(workers), machine_path)
    else:
        LOGGER.warning("Machine file %s not found; using default cluster", machine_path)
        workers = _default_workers()

    # --- Load tasks ---
    tasks: List[TraceTask] = []
    task_path = trace_dir / task_csv
    if not task_path.exists():
        raise FileNotFoundError(f"Task file not found: {task_path}")

    with open(task_path, newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            start_time = _safe_float(row.get("start_time", "0"))
            end_time = _safe_float(row.get("end_time", "0"))
            plan_cpu = _safe_float(row.get("plan_cpu", "1")) * 100  # Alibaba uses 0-100 scale
            plan_mem = _safe_float(row.get("plan_mem", "0.5"))
            status = row.get("status", "")

            # Skip tasks without valid timing
            if start_time <= 0:
                continue

            runtime = max(end_time - start_time, 1.0) if end_time > start_time else 30.0
            task_type_raw = str(row.get("task_type", "6"))
            task_type = ALIBABA_TASK_TYPE_MAP.get(task_type_raw, "mixed")

            # Normalize CPU to cores (Alibaba uses "100" = 1 core)
            req_cpu = max(plan_cpu / 100.0, 0.1)
            req_memory = max(plan_mem * 64.0, 0.1)  # plan_mem fraction of total -> GB approx

            tasks.append(TraceTask(
                task_id=row.get("task_name", f"alibaba-{len(tasks)}"),
                arrival_time=start_time,
                req_cpu=req_cpu,
                req_memory=req_memory,
                req_storage=1.0,
                runtime_seconds=runtime,
                task_type=task_type,
                sla_multiplier=2.0,
            ))

    # Sort by arrival time and limit
    tasks.sort(key=lambda t: t.arrival_time)
    if len(tasks) > max_tasks:
        tasks = tasks[:max_tasks]

    # Normalise arrival times so trace starts at t=0
    if tasks:
        offset = tasks[0].arrival_time
        for t in tasks:
            t.arrival_time -= offset

    LOGGER.info("Loaded %d Alibaba tasks (capped at %d)", len(tasks), max_tasks)

    return TraceCluster(
        workers=workers if workers else _default_workers(),
        tasks=tasks,
        source="alibaba-v2018",
        description=f"Alibaba cluster-trace-v2018 ({len(tasks)} tasks, {len(workers)} machines)",
    )


# ---------------------------------------------------------------------------
# Google ClusterData2019
# ---------------------------------------------------------------------------

def load_google_trace(
    trace_dir: str | Path,
    max_tasks: int = 5000,
    instance_events_file: str = "instance_events.json",
    machine_events_file: str = "machine_events.json",
) -> TraceCluster:
    """Load tasks from Google ClusterData2019 JSON exports.

    Parameters
    ----------
    trace_dir:
        Path containing ``instance_events.json`` and ``machine_events.json``.
    max_tasks:
        Maximum number of tasks to load.
    instance_events_file:
        Filename for instance events data.
    machine_events_file:
        Filename for machine events data.
    """
    trace_dir = Path(trace_dir)

    # --- Load machines ---
    workers: List[Dict] = []
    machine_path = trace_dir / machine_events_file
    if machine_path.exists():
        with open(machine_path, encoding="utf-8") as fh:
            events = json.load(fh)
        seen: set = set()
        for event in events:
            mid = event.get("machine_id", event.get("machineID", ""))
            if mid in seen:
                continue
            seen.add(mid)
            workers.append({
                "worker_id": str(mid),
                "total_cpu": float(event.get("capacity", {}).get("cpus", 32)),
                "total_memory": float(event.get("capacity", {}).get("memory", 64)),
                "total_storage": 500.0,
            })
        LOGGER.info("Loaded %d machines from %s", len(workers), machine_path)
    else:
        # Try CSV alternative
        csv_path = trace_dir / "machine_events.csv"
        if csv_path.exists():
            workers = _load_google_machines_csv(csv_path)
        else:
            LOGGER.warning("Machine file not found; using default cluster")
            workers = _default_workers()

    # --- Load tasks ---
    tasks: List[TraceTask] = []
    instance_path = trace_dir / instance_events_file

    if not instance_path.exists():
        # Try CSV alternative
        csv_path = trace_dir / "instance_events.csv"
        if csv_path.exists():
            tasks = _load_google_tasks_csv(csv_path, max_tasks)
        else:
            raise FileNotFoundError(f"Instance events file not found: {instance_path}")
    else:
        with open(instance_path, encoding="utf-8") as fh:
            events = json.load(fh)

        for event in events:
            timestamp = _safe_float(event.get("time", event.get("timestamp", "0")))
            req_cpu = _safe_float(event.get("resource_request", {}).get("cpus", "1"))
            req_memory = _safe_float(event.get("resource_request", {}).get("memory", "0.5"))

            if timestamp < 0 or req_cpu <= 0:
                continue

            # Classify by resource ratio
            cpu_mem_ratio = req_cpu / max(req_memory, 0.01)
            if cpu_mem_ratio > 3.0:
                task_type = "cpu-heavy"
            elif cpu_mem_ratio < 0.3:
                task_type = "memory-heavy"
            elif req_cpu < 1.0 and req_memory < 1.0:
                task_type = "cpu-light"
            else:
                task_type = "mixed"

            tasks.append(TraceTask(
                task_id=event.get("collection_id", f"google-{len(tasks)}"),
                arrival_time=timestamp / 1e6,  # Google uses microseconds
                req_cpu=max(req_cpu, 0.1),
                req_memory=max(req_memory * 32.0, 0.1),  # normalised -> GB approx
                req_storage=1.0,
                runtime_seconds=30.0,  # Google traces don't always have duration
                task_type=task_type,
                sla_multiplier=2.0,
            ))

    tasks.sort(key=lambda t: t.arrival_time)
    if len(tasks) > max_tasks:
        tasks = tasks[:max_tasks]

    if tasks:
        offset = tasks[0].arrival_time
        for t in tasks:
            t.arrival_time -= offset

    LOGGER.info("Loaded %d Google tasks (capped at %d)", len(tasks), max_tasks)

    return TraceCluster(
        workers=workers if workers else _default_workers(),
        tasks=tasks,
        source="google-2019",
        description=f"Google ClusterData2019 ({len(tasks)} tasks, {len(workers)} machines)",
    )


# ---------------------------------------------------------------------------
# CSV fallbacks for Google traces
# ---------------------------------------------------------------------------

def _load_google_machines_csv(path: Path) -> List[Dict]:
    workers: List[Dict] = []
    seen: set = set()
    with open(path, newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            mid = row.get("machine_id", row.get("machineID", ""))
            if mid in seen:
                continue
            seen.add(mid)
            workers.append({
                "worker_id": str(mid),
                "total_cpu": _safe_float(row.get("capacity_cpus", row.get("cpus", "32"))),
                "total_memory": _safe_float(row.get("capacity_memory", row.get("memory", "64"))),
                "total_storage": 500.0,
            })
    LOGGER.info("Loaded %d Google machines from CSV", len(workers))
    return workers


def _load_google_tasks_csv(path: Path, max_tasks: int) -> List[TraceTask]:
    tasks: List[TraceTask] = []
    with open(path, newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            timestamp = _safe_float(row.get("time", "0"))
            req_cpu = _safe_float(row.get("resource_request_cpus", row.get("cpus", "1")))
            req_memory = _safe_float(row.get("resource_request_memory", row.get("memory", "0.5")))

            if timestamp < 0 or req_cpu <= 0:
                continue

            cpu_mem_ratio = req_cpu / max(req_memory, 0.01)
            if cpu_mem_ratio > 3.0:
                task_type = "cpu-heavy"
            elif cpu_mem_ratio < 0.3:
                task_type = "memory-heavy"
            elif req_cpu < 1.0 and req_memory < 1.0:
                task_type = "cpu-light"
            else:
                task_type = "mixed"

            tasks.append(TraceTask(
                task_id=row.get("collection_id", f"google-{len(tasks)}"),
                arrival_time=timestamp / 1e6,
                req_cpu=max(req_cpu, 0.1),
                req_memory=max(req_memory * 32.0, 0.1),
                req_storage=1.0,
                runtime_seconds=30.0,
                task_type=task_type,
                sla_multiplier=2.0,
            ))

            if len(tasks) >= max_tasks:
                break
    LOGGER.info("Loaded %d Google tasks from CSV", len(tasks))
    return tasks


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _safe_float(value, default: float = 0.0) -> float:
    try:
        return float(value)
    except (ValueError, TypeError):
        return default


def _default_workers() -> List[Dict]:
    """Fallback cluster when no machine metadata is available."""
    return [
        {"worker_id": "default-0", "total_cpu": 20, "total_memory": 64, "total_storage": 500},
        {"worker_id": "default-1", "total_cpu": 16, "total_memory": 48, "total_storage": 500},
        {"worker_id": "default-2", "total_cpu": 24, "total_memory": 96, "total_storage": 1000},
        {"worker_id": "default-3", "total_cpu": 12, "total_memory": 32, "total_storage": 500},
    ]


# ---------------------------------------------------------------------------
# Unified loader
# ---------------------------------------------------------------------------

def load_trace(trace_path: str | Path, source: str, max_tasks: int = 5000) -> TraceCluster:
    """Load a cluster trace from the given path.

    Parameters
    ----------
    trace_path:
        Directory containing trace files.
    source:
        One of ``"alibaba"`` or ``"google"``.
    max_tasks:
        Maximum number of tasks to keep.
    """
    source = source.lower().strip()
    if source in ("alibaba", "alibaba-v2018"):
        return load_alibaba_trace(trace_path, max_tasks=max_tasks)
    elif source in ("google", "google-2019"):
        return load_google_trace(trace_path, max_tasks=max_tasks)
    else:
        raise ValueError(f"Unknown trace source: {source!r}. Use 'alibaba' or 'google'.")
