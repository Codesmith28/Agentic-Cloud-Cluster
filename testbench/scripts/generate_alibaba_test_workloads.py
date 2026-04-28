#!/usr/bin/env python3
"""Generate benchmark workloads from an Alibaba test split.

Reads ``batch_task.csv`` from ``agentic_scheduler/data/alibaba_v2018/alibaba_test``
and materializes testbench workload JSONs:

- alibaba-test-cpu
- alibaba-test-memory
- alibaba-test-mixed
- alibaba-test-bursty
"""

from __future__ import annotations

import argparse
import csv
import json
import math
from pathlib import Path
from typing import Dict, Iterable, Iterator, List

TASK_HEADER = [
    "task_name",
    "instance_num",
    "job_name",
    "task_type",
    "status",
    "start_time",
    "end_time",
    "plan_cpu",
    "plan_mem",
]

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


def _safe_float(raw: str, default: float = 0.0) -> float:
    try:
        return float(raw)
    except (TypeError, ValueError):
        return default


def _safe_int(raw: str, default: int = 0) -> int:
    try:
        return int(raw)
    except (TypeError, ValueError):
        return default


def _iter_task_rows(path: Path) -> Iterator[Dict[str, str]]:
    with path.open("r", encoding="utf-8", newline="") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            yield {k: str(row.get(k, "")).strip() for k in TASK_HEADER}


def _normalize_cpu(plan_cpu: float) -> float:
    if plan_cpu <= 0:
        return 0.25
    if plan_cpu > 10.0:
        plan_cpu = plan_cpu / 100.0
    return max(min(plan_cpu, 2.75), 0.25)


def _normalize_memory(plan_mem: float, task_type: str) -> float:
    if plan_mem <= 0:
        base = 0.5
    elif plan_mem <= 0.2:
        base = 0.5
    elif plan_mem <= 0.5:
        base = 0.75
    elif plan_mem <= 1.0:
        base = 1.0
    elif plan_mem <= 2.0:
        base = 1.4
    elif plan_mem <= 4.0:
        base = 1.8
    else:
        base = 2.4

    if task_type == "memory-heavy":
        base = max(base, 1.6)
    elif task_type == "cpu-light":
        base = min(base, 0.8)
    return max(min(base, 2.75), 0.35)


def _runtime_to_seconds(runtime: float, task_type: str) -> float:
    if runtime <= 0:
        runtime = 30.0
    seconds = 2.0 + math.log1p(runtime) / 1.4
    if task_type == "cpu-heavy":
        seconds += 1.0
    elif task_type == "cpu-light":
        seconds -= 0.5
    return round(max(min(seconds, 8.0), 2.0), 1)


def _workflow_profile_for(task_type: str) -> str:
    if task_type in {"cpu-heavy", "cpu-light", "memory-heavy", "mixed"}:
        return task_type
    return "mixed"


def _workflow_args(profile: str, seconds: float, memory_required: float, label: str) -> List[str]:
    args = ["--seconds", str(seconds)]
    if profile in {"memory-heavy", "mixed"}:
        mem_mb = int(max(128, min(640, round(memory_required * 180))))
        args.extend(["--memory-mb", str(mem_mb)])
    args.extend(["--label", label])
    return args


def _build_task(raw: Dict[str, str], prefix: str, idx: int, arrival_delay: float) -> Dict:
    task_type = ALIBABA_TASK_TYPE_MAP.get(raw.get("task_type", ""), "mixed")
    profile = _workflow_profile_for(task_type)

    start_time = _safe_int(raw.get("start_time", "0"), 0)
    end_time = _safe_int(raw.get("end_time", "0"), 0)
    runtime = float(max(end_time - start_time, 1))

    cpu_required = _normalize_cpu(_safe_float(raw.get("plan_cpu", "0"), 0.0))
    memory_required = _normalize_memory(_safe_float(raw.get("plan_mem", "0"), 0.0), task_type)
    seconds = _runtime_to_seconds(runtime, task_type)

    # Keep cpu-heavy workloads actually CPU dominant.
    if profile == "cpu-heavy":
        cpu_required = max(cpu_required, 1.2)
        memory_required = min(memory_required, 1.0)
    elif profile == "cpu-light":
        cpu_required = min(cpu_required, 0.75)
        memory_required = min(memory_required, 0.8)
    elif profile == "memory-heavy":
        cpu_required = min(cpu_required, 1.1)
        memory_required = max(memory_required, 1.6)

    name = f"{prefix}-{idx:03d}"
    return {
        "task_name": name,
        "workflow_profile": profile,
        "workflow_args": _workflow_args(profile, seconds, memory_required, name),
        "cpu_required": round(cpu_required, 2),
        "memory_required": round(memory_required, 2),
        "storage_required": 1,
        "tag": task_type,
        "k_value": 2.0,
        "arrival_delay_sec": round(max(arrival_delay, 0.0), 2),
    }


def _pick_tasks(
    buckets: Dict[str, List[Dict[str, str]]],
    count: int,
    primary: List[str],
    fallback: List[str],
) -> List[Dict[str, str]]:
    selected: List[Dict[str, str]] = []
    seen_keys: set[str] = set()
    for key in primary + fallback:
        for row in buckets.get(key, []):
            task_key = f"{row.get('job_name', '')}/{row.get('task_name', '')}/{row.get('start_time', '')}"
            if task_key in seen_keys:
                continue
            selected.append(row)
            seen_keys.add(task_key)
            if len(selected) >= count:
                return selected
    return selected


def _write_workload(path: Path, name: str, description: str, tasks: List[Dict]) -> None:
    payload = {"name": name, "description": description, "tasks": tasks}
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def generate_workloads(trace_dir: Path, output_dir: Path, tasks_per_workload: int) -> Dict[str, int]:
    task_path = trace_dir / "batch_task.csv"
    if not task_path.exists():
        raise FileNotFoundError(f"Task CSV not found: {task_path}")

    bucket_limit = max(tasks_per_workload * 8, 200)
    buckets: Dict[str, List[Dict[str, str]]] = {
        "cpu-heavy": [],
        "memory-heavy": [],
        "mixed": [],
        "cpu-light": [],
    }

    for row in _iter_task_rows(task_path):
        task_type = ALIBABA_TASK_TYPE_MAP.get(row.get("task_type", ""), "mixed")
        if task_type not in buckets:
            task_type = "mixed"
        if len(buckets[task_type]) < bucket_limit:
            buckets[task_type].append(row)

        if all(len(values) >= tasks_per_workload * 3 for values in buckets.values()):
            break

    cpu_rows = _pick_tasks(buckets, tasks_per_workload, ["cpu-heavy"], ["mixed", "cpu-light"])
    memory_rows = _pick_tasks(buckets, tasks_per_workload, ["memory-heavy"], ["mixed"])
    mixed_rows = _pick_tasks(buckets, tasks_per_workload, ["mixed"], ["cpu-heavy", "memory-heavy", "cpu-light"])
    bursty_rows = _pick_tasks(
        buckets,
        tasks_per_workload,
        ["cpu-heavy", "memory-heavy", "mixed", "cpu-light"],
        [],
    )

    if min(len(cpu_rows), len(memory_rows), len(mixed_rows), len(bursty_rows)) < tasks_per_workload:
        raise ValueError(
            "Insufficient trace diversity to build all Alibaba test workloads "
            f"at {tasks_per_workload} tasks each"
        )

    cpu_tasks = [_build_task(row, "alibaba-cpu", i + 1, arrival_delay=0.4) for i, row in enumerate(cpu_rows)]
    memory_tasks = [_build_task(row, "alibaba-mem", i + 1, arrival_delay=0.5) for i, row in enumerate(memory_rows)]
    mixed_tasks = [_build_task(row, "alibaba-mix", i + 1, arrival_delay=0.3) for i, row in enumerate(mixed_rows)]

    bursty_tasks: List[Dict] = []
    for i, row in enumerate(bursty_rows):
        delay = 0.0
        if i > 0 and i % 8 == 0:
            delay = 5.0
        elif i > 0 and i % 4 == 0:
            delay = 1.5
        bursty_tasks.append(_build_task(row, "alibaba-burst", i + 1, arrival_delay=delay))

    _write_workload(
        output_dir / "alibaba-test-cpu.json",
        "alibaba-test-cpu",
        "Alibaba-derived CPU-heavy workload sampled from alibaba_test split.",
        cpu_tasks,
    )
    _write_workload(
        output_dir / "alibaba-test-memory.json",
        "alibaba-test-memory",
        "Alibaba-derived memory-heavy workload sampled from alibaba_test split.",
        memory_tasks,
    )
    _write_workload(
        output_dir / "alibaba-test-mixed.json",
        "alibaba-test-mixed",
        "Alibaba-derived mixed workload sampled from alibaba_test split.",
        mixed_tasks,
    )
    _write_workload(
        output_dir / "alibaba-test-bursty.json",
        "alibaba-test-bursty",
        "Alibaba-derived bursty workload sampled from alibaba_test split.",
        bursty_tasks,
    )

    manifest = {
        "trace_dir": str(trace_dir),
        "tasks_per_workload": tasks_per_workload,
        "generated_workloads": [
            "alibaba-test-cpu",
            "alibaba-test-memory",
            "alibaba-test-mixed",
            "alibaba-test-bursty",
        ],
    }
    (output_dir / "alibaba-test-manifest.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")

    return {
        "alibaba-test-cpu": len(cpu_tasks),
        "alibaba-test-memory": len(memory_tasks),
        "alibaba-test-mixed": len(mixed_tasks),
        "alibaba-test-bursty": len(bursty_tasks),
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate Alibaba-derived benchmark workloads")
    parser.add_argument(
        "--trace-dir",
        type=Path,
        default=Path("agentic_scheduler/data/alibaba_v2018/alibaba_test"),
        help="Alibaba split directory containing batch_task.csv",
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path("testbench/workloads"),
        help="Directory where workload JSON files are written",
    )
    parser.add_argument(
        "--tasks-per-workload",
        type=int,
        default=40,
        help="Number of tasks per generated workload",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.tasks_per_workload <= 0:
        raise ValueError("--tasks-per-workload must be > 0")
    counts = generate_workloads(
        trace_dir=args.trace_dir,
        output_dir=args.output_dir,
        tasks_per_workload=args.tasks_per_workload,
    )
    print(f"Generated Alibaba test workloads in {args.output_dir}: {counts}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
