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

"""Trace data loaders for CloudAI history, Alibaba cluster-trace-v2018, and Google ClusterData2019.

Each loader reads CSV/JSON trace files and produces a list of ``TraceTask``
records sorted by arrival time, suitable for replay in ``TraceReplayEnv``.
"""

from __future__ import annotations

import csv
import json
import logging
from datetime import datetime, timezone
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Sequence, Tuple

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
    requeue_count: int = 0
    worker_snapshot: Dict = field(default_factory=dict)


@dataclass
class TraceCluster:
    """Cluster configuration inferred from a trace."""

    workers: List[Dict]  # list of {worker_id, total_cpu, total_memory, total_storage}
    tasks: List[TraceTask]
    source: str = ""
    description: str = ""
    trace_window: str = ""
    metadata: Dict = field(default_factory=dict)


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


def _classify_task_type(req_cpu: float, req_memory: float) -> str:
    cpu_mem_ratio = req_cpu / max(req_memory, 0.01)
    if cpu_mem_ratio > 3.0:
        return "cpu-heavy"
    if cpu_mem_ratio < 0.3:
        return "memory-heavy"
    if req_cpu < 1.0 and req_memory < 1.0:
        return "cpu-light"
    return "mixed"


def _normalize_alibaba_cpu(plan_cpu: float) -> float:
    """Convert Alibaba plan_cpu to CPU cores.

    Alibaba batch traces typically encode CPU in centi-core units where 100 means
    1 full core. Some preprocessed variants already store cores directly.
    """
    if plan_cpu <= 0:
        return 0.1
    if plan_cpu > 10.0:
        return max(plan_cpu / 100.0, 0.1)
    return max(plan_cpu, 0.1)


def _normalize_alibaba_memory(plan_mem: float) -> float:
    """Keep Alibaba plan_mem in the trace's native normalised scale.

    Both ``plan_mem`` (task requests) and ``mem_size`` (machine capacity) in
    the Alibaba cluster-trace-v2018 are expressed in the **same** normalised
    units where 100 equals the full capacity of a reference machine.  Typical
    task values range from 0.01 to ~17; machine values are uniformly 100 in
    the curated dataset.

    A previous version mistakenly multiplied values ≤ 1.0 by 64, treating
    them as fractions of 64 GB.  That mixed scales between tasks and machines
    and caused workers to saturate after only a handful of placements.
    """
    if plan_mem <= 0:
        return 0.1
    return max(plan_mem, 0.1)


def _normalise_tasks(tasks: List[TraceTask], max_tasks: int) -> Tuple[List[TraceTask], float, float]:
    tasks.sort(key=lambda t: t.arrival_time)
    if len(tasks) > max_tasks:
        tasks = tasks[:max_tasks]

    raw_start = tasks[0].arrival_time if tasks else 0.0
    raw_end = tasks[-1].arrival_time if tasks else 0.0
    if tasks:
        offset = tasks[0].arrival_time
        for task in tasks:
            task.arrival_time -= offset
    return tasks, raw_start, raw_end


def _resolve_trace_window_label(trace_window: str, start_ts: float, end_ts: float) -> str:
    label = (trace_window or "").strip()
    if label:
        return label
    if start_ts <= 0 and end_ts <= 0:
        return "full"
    start = datetime.fromtimestamp(max(start_ts, 0), tz=timezone.utc).isoformat()
    end = datetime.fromtimestamp(max(end_ts, 0), tz=timezone.utc).isoformat()
    return f"{start}..{end}"


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
            machine_fields = ["machine_id", "time_stamp", "failure_domain_1", "failure_domain_2", "cpu_num", "mem_size", "status"]
            reader = csv.DictReader(fh, fieldnames=machine_fields)
            next(reader)  # Skip header row
            seen_ids: set = set()
            for idx, row in enumerate(reader):
                mid = row.get("machine_id", f"machine-{idx}")
                if mid in seen_ids:
                    continue
                seen_ids.add(mid)
                workers.append({
                    "worker_id": mid,
                    "total_cpu": float(row.get("cpu_num", 64)),
                    "total_memory": float(row.get("mem_size", 128)),
                    "total_storage": float(row.get("disk_size", 1000)),
                })
        LOGGER.info("Loaded %d unique machines from %s", len(workers), machine_path)
    else:
        LOGGER.warning("Machine file %s not found; using default cluster", machine_path)
        workers = _default_workers()

    # --- Load tasks ---
    tasks: List[TraceTask] = []
    task_path = trace_dir / task_csv
    if not task_path.exists():
        raise FileNotFoundError(f"Task file not found: {task_path}")

    with open(task_path, newline="", encoding="utf-8") as fh:
        task_fields = ["task_name", "instance_num", "job_name", "task_type", "status", "start_time", "end_time", "plan_cpu", "plan_mem"]
        reader = csv.DictReader(fh, fieldnames=task_fields)
        next(reader)  # Skip header row
        for row in reader:
            start_time = _safe_float(row.get("start_time", "0"))
            end_time = _safe_float(row.get("end_time", "0"))
            plan_cpu = _safe_float(row.get("plan_cpu", "1"))
            plan_mem = _safe_float(row.get("plan_mem", "0.5"))

            # Skip tasks without valid timing
            if start_time <= 0:
                continue

            runtime = max(end_time - start_time, 1.0) if end_time > start_time else 30.0
            task_type_raw = str(row.get("task_type", "6"))
            task_type = ALIBABA_TASK_TYPE_MAP.get(task_type_raw, "mixed")

            req_cpu = _normalize_alibaba_cpu(plan_cpu)
            req_memory = _normalize_alibaba_memory(plan_mem)

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

    tasks, raw_start, raw_end = _normalise_tasks(tasks, max_tasks)

    LOGGER.info("Loaded %d Alibaba tasks (capped at %d)", len(tasks), max_tasks)
    resolved_workers = workers if workers else _default_workers()
    trace_window = _resolve_trace_window_label("", raw_start, raw_end)

    return TraceCluster(
        workers=resolved_workers,
        tasks=tasks,
        source="alibaba-v2018",
        description=f"Alibaba cluster-trace-v2018 ({len(tasks)} tasks, {len(resolved_workers)} machines)",
        trace_window=trace_window,
        metadata={"trace_window": trace_window},
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

            task_type = _classify_task_type(req_cpu, req_memory)

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

    tasks, raw_start, raw_end = _normalise_tasks(tasks, max_tasks)

    LOGGER.info("Loaded %d Google tasks (capped at %d)", len(tasks), max_tasks)
    resolved_workers = workers if workers else _default_workers()
    trace_window = _resolve_trace_window_label("", raw_start, raw_end)

    return TraceCluster(
        workers=resolved_workers,
        tasks=tasks,
        source="google-2019",
        description=f"Google ClusterData2019 ({len(tasks)} tasks, {len(resolved_workers)} machines)",
        trace_window=trace_window,
        metadata={"trace_window": trace_window},
    )


# ---------------------------------------------------------------------------
# CloudAI replay/history adapter
# ---------------------------------------------------------------------------

def load_cloudai_trace(
    trace_path: str | Path = "",
    max_tasks: int = 5000,
    mongo_uri: str = "",
    mongo_db: str = "cluster_db",
    trace_window: str = "",
    trace_window_start: str = "",
    trace_window_end: str = "",
) -> TraceCluster:
    records: List[Dict]
    workers: List[Dict]

    if trace_path:
        records, workers = _load_cloudai_records_from_path(Path(trace_path))
    elif mongo_uri:
        records, workers = _load_cloudai_records_from_mongo(
            mongo_uri=mongo_uri,
            mongo_db=mongo_db,
            max_tasks=max_tasks,
            trace_window=trace_window,
            trace_window_start=trace_window_start,
            trace_window_end=trace_window_end,
        )
    else:
        raise ValueError("CloudAI trace source requires either trace_path or mongo_uri")

    if not records:
        raise ValueError("No CloudAI trace records found")

    filtered_records = _filter_cloudai_records(
        records,
        trace_window=trace_window,
        trace_window_start=trace_window_start,
        trace_window_end=trace_window_end,
    )
    if not filtered_records:
        raise ValueError("CloudAI trace window selection produced zero records")

    resolved_workers = workers if workers else _default_workers()
    workers_by_id = {str(w.get("worker_id", "")): w for w in resolved_workers}

    tasks: List[TraceTask] = []
    for idx, record in enumerate(filtered_records):
        task = _build_cloudai_task(record, idx, workers_by_id)
        if task is not None:
            tasks.append(task)

    tasks, raw_start, raw_end = _normalise_tasks(tasks, max_tasks)
    if not tasks:
        raise ValueError("CloudAI adapter produced no valid trace tasks")

    observed_window = ""
    for record in filtered_records:
        raw = str(record.get("trace_window", "")).strip()
        if raw:
            observed_window = raw
            break
    window_label = _resolve_trace_window_label(trace_window or observed_window, raw_start, raw_end)

    return TraceCluster(
        workers=resolved_workers,
        tasks=tasks,
        source="cloudai-history",
        description=f"CloudAI replay trace ({len(tasks)} tasks, {len(resolved_workers)} workers)",
        trace_window=window_label,
        metadata={
            "trace_window": window_label,
            "window_start": raw_start,
            "window_end": raw_end,
            "record_count": len(filtered_records),
        },
    )


def _load_cloudai_records_from_path(trace_path: Path) -> Tuple[List[Dict], List[Dict]]:
    if not trace_path.exists():
        raise FileNotFoundError(f"CloudAI trace path not found: {trace_path}")

    if trace_path.is_file():
        return _load_structured_records(trace_path), []

    task_candidates = [
        trace_path / "cloudai_trace.json",
        trace_path / "cloudai_trace.jsonl",
        trace_path / "cloudai_trace.csv",
        trace_path / "history.json",
        trace_path / "history.jsonl",
        trace_path / "history.csv",
        trace_path / "tasks.json",
        trace_path / "tasks.jsonl",
        trace_path / "tasks.csv",
    ]
    worker_candidates = [
        trace_path / "workers.json",
        trace_path / "workers.jsonl",
        trace_path / "workers.csv",
        trace_path / "worker_registry.json",
        trace_path / "worker_registry.csv",
        trace_path / "machine_meta.csv",
    ]

    task_records: List[Dict] = []
    for candidate in task_candidates:
        if candidate.exists():
            task_records = _load_structured_records(candidate)
            if task_records:
                break

    worker_records: List[Dict] = []
    for candidate in worker_candidates:
        if candidate.exists():
            worker_records = _load_structured_records(candidate)
            if worker_records:
                break

    if not task_records:
        for candidate in sorted(trace_path.glob("*")):
            if candidate.suffix.lower() not in {".json", ".jsonl", ".csv"}:
                continue
            lower_name = candidate.name.lower()
            if "worker" in lower_name or "machine" in lower_name:
                continue
            task_records = _load_structured_records(candidate)
            if task_records:
                break

    workers = [_normalise_worker_record(row, idx) for idx, row in enumerate(worker_records)]
    LOGGER.info("Loaded %d CloudAI records from %s", len(task_records), trace_path)
    return task_records, workers


def _load_cloudai_records_from_mongo(
    mongo_uri: str,
    mongo_db: str,
    max_tasks: int,
    trace_window: str,
    trace_window_start: str,
    trace_window_end: str,
) -> Tuple[List[Dict], List[Dict]]:
    from pymongo import MongoClient

    query: Dict = {}
    start_ts = _parse_optional_timestamp(trace_window_start)
    end_ts = _parse_optional_timestamp(trace_window_end)
    if start_ts is not None or end_ts is not None:
        submitted_range: Dict = {}
        if start_ts is not None:
            submitted_range["$gte"] = start_ts
        if end_ts is not None:
            submitted_range["$lte"] = end_ts
        query["submitted_at"] = submitted_range

    client = MongoClient(mongo_uri, serverSelectionTimeoutMS=5000)
    try:
        database = client[mongo_db]
        limit = max(max_tasks * 4, max_tasks)
        task_docs = list(database["TASKS"].find(query).sort("submitted_at", 1).limit(limit))
        if trace_window:
            has_trace_window = any(str(doc.get("trace_window", "")).strip() for doc in task_docs)
            if has_trace_window:
                task_docs = [doc for doc in task_docs if str(doc.get("trace_window", "")).strip() == trace_window]

        task_ids = [str(doc.get("task_id", "")).strip() for doc in task_docs if str(doc.get("task_id", "")).strip()]
        result_by_task: Dict[str, Dict] = {}
        if task_ids:
            for result_doc in database["RESULTS"].find({"task_id": {"$in": task_ids}}):
                task_id = str(result_doc.get("task_id", "")).strip()
                if task_id:
                    result_by_task[task_id] = result_doc

        records: List[Dict] = []
        for task_doc in task_docs:
            merged = dict(task_doc)
            task_id = str(merged.get("task_id", "")).strip()
            if task_id and task_id in result_by_task:
                merged.setdefault("sla_success", result_by_task[task_id].get("sla_success"))
                merged.setdefault("status", result_by_task[task_id].get("status"))
                merged.setdefault("result_completed_at", result_by_task[task_id].get("completed_at"))
            records.append(merged)

        worker_docs = list(
            database["WORKER_REGISTRY"].find(
                {},
                {
                    "worker_id": 1,
                    "total_cpu": 1,
                    "total_memory": 1,
                    "total_storage": 1,
                    "is_active": 1,
                },
            ).limit(256)
        )
        workers = [_normalise_worker_record(doc, idx) for idx, doc in enumerate(worker_docs)]
        LOGGER.info("Loaded %d CloudAI records from Mongo (db=%s)", len(records), mongo_db)
        return records, workers
    finally:
        client.close()


def _filter_cloudai_records(
    records: Sequence[Dict],
    trace_window: str,
    trace_window_start: str,
    trace_window_end: str,
) -> List[Dict]:
    start_ts = _parse_optional_timestamp(trace_window_start)
    end_ts = _parse_optional_timestamp(trace_window_end)
    selected_window = (trace_window or "").strip()
    has_window_labels = any(str(record.get("trace_window", "")).strip() for record in records)

    filtered: List[Dict] = []
    for record in records:
        if selected_window and has_window_labels:
            if str(record.get("trace_window", "")).strip() != selected_window:
                continue

        arrival_ts = _safe_timestamp(
            record.get("submitted_at", record.get("arrival_time", record.get("created_at", record.get("time", 0)))),
            default=-1.0,
        )
        if start_ts is not None and arrival_ts >= 0 and arrival_ts < start_ts:
            continue
        if end_ts is not None and arrival_ts >= 0 and arrival_ts > end_ts:
            continue
        filtered.append(record)
    return filtered


def _build_cloudai_task(record: Dict, idx: int, workers_by_id: Dict[str, Dict]) -> Optional[TraceTask]:
    task_id = str(record.get("task_id", record.get("id", f"cloudai-{idx}"))).strip() or f"cloudai-{idx}"

    arrival_time = _safe_timestamp(
        record.get("submitted_at", record.get("arrival_time", record.get("created_at", record.get("time", idx)))),
        default=float(idx),
    )
    if arrival_time < 0:
        return None

    req_cpu = max(_safe_float(record.get("req_cpu", record.get("cpu_request", record.get("cpu", 1.0))), 1.0), 0.1)
    req_memory = max(
        _safe_float(record.get("req_memory", record.get("memory_request", record.get("memory", 2.0))), 2.0),
        0.1,
    )
    req_storage = max(
        _safe_float(record.get("req_storage", record.get("storage_request", record.get("storage", 1.0))), 1.0),
        0.1,
    )

    start_ts = _safe_timestamp(record.get("started_at", record.get("start_time", 0)), default=0.0)
    completed_ts = _safe_timestamp(
        record.get("completed_at", record.get("result_completed_at", record.get("end_time", 0))),
        default=0.0,
    )
    runtime_seconds = _safe_float(record.get("actual_runtime", record.get("runtime_seconds", 0.0)), 0.0)
    if runtime_seconds <= 0 and completed_ts > start_ts > 0:
        runtime_seconds = completed_ts - start_ts
    if runtime_seconds <= 0:
        runtime_seconds = 30.0

    queue_wait_seconds = _safe_float(record.get("queue_wait_seconds", record.get("queue_wait", 0.0)), 0.0)
    if queue_wait_seconds <= 0 and start_ts > arrival_time > 0:
        queue_wait_seconds = start_ts - arrival_time

    task_type = str(record.get("task_type", "")).strip() or _classify_task_type(req_cpu, req_memory)
    sla_multiplier = _safe_float(record.get("sla_multiplier", record.get("k_value", 2.0)), 2.0)
    sla_success = _coerce_optional_bool(record.get("sla_success"))
    failure_reason = str(record.get("failure_reason", record.get("last_failure_reason", ""))).strip()
    requeue_count = max(_safe_int(record.get("recovery_count", record.get("requeue_count", 0)), 0), 0)

    worker_id = str(record.get("last_worker_id", record.get("worker_id", ""))).strip()
    worker_snapshot = {}
    if worker_id and worker_id in workers_by_id:
        worker_snapshot = dict(workers_by_id[worker_id])

    return TraceTask(
        task_id=task_id,
        arrival_time=arrival_time,
        req_cpu=req_cpu,
        req_memory=req_memory,
        req_storage=req_storage,
        runtime_seconds=max(runtime_seconds, 1.0),
        task_type=task_type,
        sla_multiplier=sla_multiplier,
        queue_wait_seconds=max(queue_wait_seconds, 0.0),
        sla_success=sla_success,
        failure_reason=failure_reason,
        requeue_count=requeue_count,
        worker_snapshot=worker_snapshot,
    )


def _normalise_worker_record(row: Dict, idx: int) -> Dict:
    return {
        "worker_id": str(row.get("worker_id", row.get("machine_id", f"worker-{idx}"))),
        "total_cpu": max(
            _safe_float(row.get("total_cpu", row.get("cpu_num", row.get("capacity_cpus", row.get("cpus", 16.0)))), 16.0),
            0.1,
        ),
        "total_memory": max(
            _safe_float(
                row.get("total_memory", row.get("mem_size", row.get("capacity_memory", row.get("memory", 64.0)))),
                64.0,
            ),
            0.1,
        ),
        "total_storage": max(
            _safe_float(row.get("total_storage", row.get("disk_size", row.get("storage", 500.0))), 500.0),
            0.1,
        ),
    }


def _load_structured_records(path: Path, max_file_bytes: int = 500 * 1024 * 1024) -> List[Dict]:
    file_size = path.stat().st_size
    if file_size > max_file_bytes:
        raise ValueError(
            f"Trace file {path} is {file_size} bytes, exceeding limit of {max_file_bytes} bytes"
        )
    suffix = path.suffix.lower()
    if suffix == ".json":
        with open(path, encoding="utf-8") as fh:
            payload = json.load(fh)
        if isinstance(payload, list):
            return [row for row in payload if isinstance(row, dict)]
        if isinstance(payload, dict):
            for key in ("tasks", "history", "records", "data", "items", "events"):
                value = payload.get(key)
                if isinstance(value, list):
                    return [row for row in value if isinstance(row, dict)]
            return [payload]
        return []

    if suffix == ".jsonl":
        records: List[Dict] = []
        with open(path, encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    payload = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if isinstance(payload, dict):
                    records.append(payload)
        return records

    if suffix == ".csv":
        with open(path, newline="", encoding="utf-8") as fh:
            return [row for row in csv.DictReader(fh)]

    return []


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

            task_type = _classify_task_type(req_cpu, req_memory)

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


def _safe_int(value, default: int = 0) -> int:
    try:
        return int(value)
    except (ValueError, TypeError):
        return default


def _safe_timestamp(value, default: float = 0.0) -> float:
    if value is None:
        return default
    if isinstance(value, datetime):
        return value.replace(tzinfo=value.tzinfo or timezone.utc).timestamp()

    if isinstance(value, (int, float)):
        ts = float(value)
    else:
        raw = str(value).strip()
        if not raw:
            return default
        try:
            ts = float(raw)
        except ValueError:
            try:
                parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
                return parsed.timestamp()
            except ValueError:
                return default

    if ts > 1e15:
        return ts / 1e6
    if ts > 1e12:
        return ts / 1e3
    return ts


def _parse_optional_timestamp(value: str) -> Optional[float]:
    raw = str(value or "").strip()
    if not raw:
        return None
    parsed = _safe_timestamp(raw, default=float("nan"))
    if parsed != parsed:  # NaN
        return None
    return parsed


def _coerce_optional_bool(value) -> Optional[bool]:
    if value is None:
        return None
    if isinstance(value, bool):
        return value
    normalized = str(value).strip().lower()
    if normalized in {"1", "true", "t", "yes", "y"}:
        return True
    if normalized in {"0", "false", "f", "no", "n"}:
        return False
    return None


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

def load_trace(
    trace_path: str | Path,
    source: str,
    max_tasks: int = 5000,
    trace_window: str = "",
    trace_window_start: str = "",
    trace_window_end: str = "",
    mongo_uri: str = "",
    mongo_db: str = "cluster_db",
) -> TraceCluster:
    """Load a cluster trace from the given path.

    Parameters
    ----------
    trace_path:
        Directory containing trace files.
    source:
        One of ``"alibaba"``, ``"google"``, or ``"cloudai"``.
    max_tasks:
        Maximum number of tasks to keep.
    """
    source = source.lower().strip()
    if source in ("alibaba", "alibaba-v2018"):
        return load_alibaba_trace(trace_path, max_tasks=max_tasks)
    elif source in ("google", "google-2019"):
        return load_google_trace(trace_path, max_tasks=max_tasks)
    elif source in ("cloudai", "cloudai-history"):
        return load_cloudai_trace(
            trace_path=trace_path,
            max_tasks=max_tasks,
            mongo_uri=mongo_uri,
            mongo_db=mongo_db,
            trace_window=trace_window,
            trace_window_start=trace_window_start,
            trace_window_end=trace_window_end,
        )
    else:
        raise ValueError(f"Unknown trace source: {source!r}. Use 'alibaba', 'google', or 'cloudai'.")
