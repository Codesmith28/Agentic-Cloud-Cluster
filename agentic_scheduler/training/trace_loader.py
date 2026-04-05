"""Trace data loaders for CloudAI history, Alibaba cluster-trace-v2018, and Google ClusterData2019.

Each loader produces a ``TraceCluster`` suitable for replay in
``TraceReplayEnv``. Public traces come from filesystem exports. CloudAI history
is loaded directly from the project's MongoDB collections so offline training
can replay the same task distributions the live scheduler sees.
"""

from __future__ import annotations

import csv
import json
import logging
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional, Tuple

try:
    from pymongo import MongoClient
except ImportError:  # pragma: no cover - exercised in environments without pymongo
    MongoClient = None  # type: ignore[assignment]

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
    queue_wait_seconds: float = 0.0
    sla_success: Optional[bool] = None
    failure_reason: str = ""
    assigned_worker_id: str = ""
    recovery_count: int = 0


@dataclass
class TraceCluster:
    """Cluster configuration inferred from a trace."""

    workers: List[Dict]  # list of {worker_id, total_cpu, total_memory, total_storage}
    tasks: List[TraceTask]
    source: str = ""
    description: str = ""


SUPPORTED_TASK_TYPES = {"cpu-light", "cpu-heavy", "memory-heavy", "mixed"}


# ---------------------------------------------------------------------------
# CloudAI history replay
# ---------------------------------------------------------------------------

def load_cloudai_trace(
    mongo_uri: str,
    mongo_db: str,
    max_tasks: int = 5000,
    start_time: str = "",
    end_time: str = "",
) -> TraceCluster:
    """Load replayable task history from CloudAI MongoDB collections."""
    if MongoClient is None:
        raise ImportError("pymongo is required to load CloudAI traces")
    if not mongo_uri:
        raise ValueError("mongo_uri is required for CloudAI trace loading")
    if not mongo_db:
        raise ValueError("mongo_db is required for CloudAI trace loading")

    start_dt = _parse_iso_datetime(start_time)
    end_dt = _parse_iso_datetime(end_time)

    client = MongoClient(mongo_uri, serverSelectionTimeoutMS=2500)
    try:
        client.admin.command("ping")
        database = client[mongo_db]
        workers = _load_cloudai_workers(database)
        tasks, window_description = _load_cloudai_tasks(database, max_tasks, start_dt, end_dt)
    finally:
        client.close()

    if tasks:
        offset = tasks[0].arrival_time
        for task in tasks:
            task.arrival_time -= offset

    description = (
        f"CloudAI history replay ({len(tasks)} tasks, {len(workers)} workers"
        f"{', window=' + window_description if window_description else ''})"
    )
    LOGGER.info("Loaded %d CloudAI replay tasks (capped at %d)", len(tasks), max_tasks)

    return TraceCluster(
        workers=workers if workers else _default_workers(),
        tasks=tasks,
        source="cloudai-history",
        description=description,
    )


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
    """Load tasks and machines from Alibaba cluster-trace-v2018 CSVs."""
    trace_dir = Path(trace_dir)

    workers: List[Dict] = []
    machine_path = trace_dir / machine_csv
    if machine_path.exists():
        with open(machine_path, newline="", encoding="utf-8") as fh:
            reader = csv.DictReader(fh)
            for idx, row in enumerate(reader):
                workers.append(
                    {
                        "worker_id": row.get("machine_id", f"machine-{idx}"),
                        "total_cpu": float(row.get("cpu_num", 64)),
                        "total_memory": float(row.get("mem_size", 128)),
                        "total_storage": float(row.get("disk_size", 1000)),
                    }
                )
        LOGGER.info("Loaded %d machines from %s", len(workers), machine_path)
    else:
        LOGGER.warning("Machine file %s not found; using default cluster", machine_path)
        workers = _default_workers()

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

            if start_time <= 0:
                continue

            runtime = max(end_time - start_time, 1.0) if end_time > start_time else 30.0
            task_type_raw = str(row.get("task_type", "6"))
            task_type = ALIBABA_TASK_TYPE_MAP.get(task_type_raw, "mixed")

            tasks.append(
                TraceTask(
                    task_id=row.get("task_name", f"alibaba-{len(tasks)}"),
                    arrival_time=start_time,
                    req_cpu=max(plan_cpu / 100.0, 0.1),
                    req_memory=max(plan_mem * 64.0, 0.1),
                    req_storage=1.0,
                    runtime_seconds=runtime,
                    task_type=task_type,
                    sla_multiplier=2.0,
                )
            )

    tasks.sort(key=lambda task: task.arrival_time)
    if len(tasks) > max_tasks:
        tasks = tasks[:max_tasks]

    if tasks:
        offset = tasks[0].arrival_time
        for task in tasks:
            task.arrival_time -= offset

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
    """Load tasks from Google ClusterData2019 JSON or CSV exports."""
    trace_dir = Path(trace_dir)

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
            workers.append(
                {
                    "worker_id": str(mid),
                    "total_cpu": float(event.get("capacity", {}).get("cpus", 32)),
                    "total_memory": float(event.get("capacity", {}).get("memory", 64)),
                    "total_storage": 500.0,
                }
            )
        LOGGER.info("Loaded %d machines from %s", len(workers), machine_path)
    else:
        csv_path = trace_dir / "machine_events.csv"
        if csv_path.exists():
            workers = _load_google_machines_csv(csv_path)
        else:
            LOGGER.warning("Machine file not found; using default cluster")
            workers = _default_workers()

    tasks: List[TraceTask] = []
    instance_path = trace_dir / instance_events_file
    if not instance_path.exists():
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

            tasks.append(
                TraceTask(
                    task_id=event.get("collection_id", f"google-{len(tasks)}"),
                    arrival_time=timestamp / 1e6,
                    req_cpu=max(req_cpu, 0.1),
                    req_memory=max(req_memory * 32.0, 0.1),
                    req_storage=1.0,
                    runtime_seconds=30.0,
                    task_type=_classify_task_type(req_cpu, req_memory),
                    sla_multiplier=2.0,
                )
            )

    tasks.sort(key=lambda task: task.arrival_time)
    if len(tasks) > max_tasks:
        tasks = tasks[:max_tasks]

    if tasks:
        offset = tasks[0].arrival_time
        for task in tasks:
            task.arrival_time -= offset

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
            workers.append(
                {
                    "worker_id": str(mid),
                    "total_cpu": _safe_float(row.get("capacity_cpus", row.get("cpus", "32"))),
                    "total_memory": _safe_float(row.get("capacity_memory", row.get("memory", "64"))),
                    "total_storage": 500.0,
                }
            )
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

            tasks.append(
                TraceTask(
                    task_id=row.get("collection_id", f"google-{len(tasks)}"),
                    arrival_time=timestamp / 1e6,
                    req_cpu=max(req_cpu, 0.1),
                    req_memory=max(req_memory * 32.0, 0.1),
                    req_storage=1.0,
                    runtime_seconds=30.0,
                    task_type=_classify_task_type(req_cpu, req_memory),
                    sla_multiplier=2.0,
                )
            )

            if len(tasks) >= max_tasks:
                break
    LOGGER.info("Loaded %d Google tasks from CSV", len(tasks))
    return tasks


# ---------------------------------------------------------------------------
# Unified loaders
# ---------------------------------------------------------------------------

def load_trace_with_options(
    trace_path: str | Path,
    source: str,
    max_tasks: int = 5000,
    mongo_uri: str = "",
    mongo_db: str = "",
    start_time: str = "",
    end_time: str = "",
) -> TraceCluster:
    """Load a cluster trace from a filesystem export or CloudAI history."""
    source = source.lower().strip()
    if source in ("cloudai", "cloudai-history"):
        return load_cloudai_trace(
            mongo_uri=mongo_uri,
            mongo_db=mongo_db,
            max_tasks=max_tasks,
            start_time=start_time,
            end_time=end_time,
        )
    if source in ("alibaba", "alibaba-v2018"):
        return load_alibaba_trace(trace_path, max_tasks=max_tasks)
    if source in ("google", "google-2019"):
        return load_google_trace(trace_path, max_tasks=max_tasks)
    raise ValueError(f"Unknown trace source: {source!r}. Use 'cloudai', 'alibaba', or 'google'.")


def load_trace(trace_path: str | Path, source: str, max_tasks: int = 5000) -> TraceCluster:
    """Backward-compatible loader for file-backed public traces."""
    return load_trace_with_options(trace_path=trace_path, source=source, max_tasks=max_tasks)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _load_cloudai_workers(database) -> List[Dict]:
    workers: List[Dict] = []
    cursor = database["WORKER_REGISTRY"].find(
        {"total_cpu": {"$gt": 0}},
        {
            "_id": 0,
            "worker_id": 1,
            "total_cpu": 1,
            "total_memory": 1,
            "total_storage": 1,
        },
    )
    for doc in cursor:
        workers.append(
            {
                "worker_id": str(doc.get("worker_id", f"worker-{len(workers)}")),
                "total_cpu": max(_safe_float(doc.get("total_cpu"), 1.0), 0.1),
                "total_memory": max(_safe_float(doc.get("total_memory"), 1.0), 0.1),
                "total_storage": max(_safe_float(doc.get("total_storage"), 1.0), 0.1),
            }
        )

    if not workers:
        LOGGER.warning("CloudAI worker registry is empty; using default replay cluster")
        return _default_workers()

    LOGGER.info("Loaded %d workers from CloudAI worker registry", len(workers))
    return workers


def _load_cloudai_tasks(
    database,
    max_tasks: int,
    start_dt: Optional[datetime],
    end_dt: Optional[datetime],
) -> Tuple[List[TraceTask], str]:
    task_filter: Dict[str, Dict] = {"status": {"$in": ["completed", "failed"]}}
    if start_dt or end_dt:
        completed_filter: Dict[str, datetime] = {}
        if start_dt:
            completed_filter["$gte"] = start_dt
        if end_dt:
            completed_filter["$lte"] = end_dt
        task_filter["completed_at"] = completed_filter

    raw_tasks: List[Dict] = []
    task_ids: List[str] = []
    cursor = database["TASKS"].find(task_filter).sort([("submitted_at", 1), ("created_at", 1)])
    for doc in cursor:
        task_id = str(doc.get("task_id", "")).strip()
        arrival_dt = _task_arrival_datetime(doc)
        if not task_id or arrival_dt is None:
            continue
        raw_tasks.append(doc)
        task_ids.append(task_id)
        if len(raw_tasks) >= max_tasks:
            break

    results_by_task = {
        str(doc.get("task_id", "")): doc
        for doc in database["RESULTS"].find(
            {"task_id": {"$in": task_ids}},
            {"_id": 0, "task_id": 1, "worker_id": 1, "status": 1, "sla_success": 1, "completed_at": 1},
        )
    }

    tasks: List[TraceTask] = []
    for doc in raw_tasks:
        task_id = str(doc.get("task_id", "")).strip()
        arrival_dt = _task_arrival_datetime(doc)
        if not task_id or arrival_dt is None:
            continue

        started_at = _coerce_datetime(doc.get("started_at"))
        completed_at = _coerce_datetime(doc.get("completed_at"))
        runtime_seconds = _safe_runtime_seconds(started_at, completed_at, _safe_float(doc.get("tau"), 30.0))
        queue_wait_seconds = 0.0
        if started_at is not None:
            queue_wait_seconds = max((started_at - arrival_dt).total_seconds(), 0.0)

        result_doc = results_by_task.get(task_id, {})
        task_sla_success = result_doc.get("sla_success")
        if task_sla_success is None:
            deadline = _coerce_datetime(doc.get("deadline"))
            if deadline is not None and completed_at is not None:
                task_sla_success = completed_at <= deadline

        tasks.append(
            TraceTask(
                task_id=task_id,
                arrival_time=arrival_dt.timestamp(),
                req_cpu=max(_safe_float(doc.get("req_cpu"), 1.0), 0.1),
                req_memory=max(_safe_float(doc.get("req_memory"), 1.0), 0.1),
                req_storage=max(_safe_float(doc.get("req_storage"), 1.0), 0.1),
                runtime_seconds=runtime_seconds,
                task_type=_normalize_task_type(doc.get("task_type") or doc.get("tag")),
                sla_multiplier=max(
                    _safe_float(doc.get("sla_multiplier"), _safe_float(doc.get("k_value"), 2.0)),
                    1.0,
                ),
                queue_wait_seconds=queue_wait_seconds,
                sla_success=bool(task_sla_success) if task_sla_success is not None else None,
                failure_reason=str(doc.get("last_failure_reason", "") or ""),
                assigned_worker_id=str(result_doc.get("worker_id") or doc.get("last_worker_id") or ""),
                recovery_count=int(doc.get("recovery_count", 0) or 0),
            )
        )

    tasks.sort(key=lambda task: task.arrival_time)
    return tasks, _describe_window(tasks, start_dt, end_dt)


def _safe_float(value, default: float = 0.0) -> float:
    try:
        return float(value)
    except (ValueError, TypeError):
        return default


def _default_workers() -> List[Dict]:
    return [
        {"worker_id": "default-0", "total_cpu": 20, "total_memory": 64, "total_storage": 500},
        {"worker_id": "default-1", "total_cpu": 16, "total_memory": 48, "total_storage": 500},
        {"worker_id": "default-2", "total_cpu": 24, "total_memory": 96, "total_storage": 1000},
        {"worker_id": "default-3", "total_cpu": 12, "total_memory": 32, "total_storage": 500},
    ]


def _coerce_datetime(value) -> Optional[datetime]:
    if value is None:
        return None
    if isinstance(value, datetime):
        if value.tzinfo is None:
            return value.replace(tzinfo=timezone.utc)
        return value.astimezone(timezone.utc)
    if isinstance(value, (int, float)):
        try:
            return datetime.fromtimestamp(float(value), tz=timezone.utc)
        except (TypeError, ValueError, OSError):
            return None
    if isinstance(value, str):
        return _parse_iso_datetime(value)
    return None


def _parse_iso_datetime(value: str) -> Optional[datetime]:
    normalized = str(value or "").strip()
    if not normalized:
        return None
    if normalized.endswith("Z"):
        normalized = normalized[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def _task_arrival_datetime(doc: Dict) -> Optional[datetime]:
    submitted_at = _safe_float(doc.get("submitted_at"), 0.0)
    if submitted_at > 0:
        return datetime.fromtimestamp(submitted_at, tz=timezone.utc)
    return _coerce_datetime(doc.get("created_at"))


def _safe_runtime_seconds(started_at: Optional[datetime], completed_at: Optional[datetime], fallback: float) -> float:
    if started_at is not None and completed_at is not None and completed_at >= started_at:
        return max((completed_at - started_at).total_seconds(), 1.0)
    return max(fallback, 1.0)


def _normalize_task_type(value) -> str:
    normalized = str(value or "").strip().lower()
    if normalized in SUPPORTED_TASK_TYPES:
        return normalized
    alias_map = {
        "cpu_light": "cpu-light",
        "cpu_heavy": "cpu-heavy",
        "memory_heavy": "memory-heavy",
        "mem-heavy": "memory-heavy",
        "batch-job": "mixed",
    }
    return alias_map.get(normalized, "mixed")


def _classify_task_type(req_cpu: float, req_memory: float) -> str:
    cpu_mem_ratio = req_cpu / max(req_memory, 0.01)
    if cpu_mem_ratio > 3.0:
        return "cpu-heavy"
    if cpu_mem_ratio < 0.3:
        return "memory-heavy"
    if req_cpu < 1.0 and req_memory < 1.0:
        return "cpu-light"
    return "mixed"


def _describe_window(tasks: List[TraceTask], start_dt: Optional[datetime], end_dt: Optional[datetime]) -> str:
    if start_dt or end_dt:
        start_label = start_dt.isoformat() if start_dt else "beginning"
        end_label = end_dt.isoformat() if end_dt else "latest"
        return f"{start_label}..{end_label}"
    if not tasks:
        return ""
    start_label = datetime.fromtimestamp(tasks[0].arrival_time, tz=timezone.utc).isoformat()
    end_label = datetime.fromtimestamp(tasks[-1].arrival_time, tz=timezone.utc).isoformat()
    return f"{start_label}..{end_label}"
