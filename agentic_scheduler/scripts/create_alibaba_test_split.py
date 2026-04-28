#!/usr/bin/env python3
"""Create a larger contiguous Alibaba test split from local trace data.

This script builds ``agentic_scheduler/data/alibaba_v2018/alibaba_test`` by
copying a contiguous task window from an existing trace directory
(``core/`` by default). It can also validate zero overlap against the existing
held-out ``test/`` split.
"""

from __future__ import annotations

import argparse
import csv
import json
from pathlib import Path
from typing import Dict, Iterable, Iterator, List, Tuple

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

MACHINE_HEADER = [
    "machine_id",
    "time_stamp",
    "failure_domain_1",
    "failure_domain_2",
    "cpu_num",
    "mem_size",
    "status",
]


def _normalize_header(value: str) -> str:
    return str(value).strip().lower()


def _file_has_header(path: Path, expected_header: List[str]) -> bool:
    with path.open("r", encoding="utf-8", newline="") as fh:
        reader = csv.reader(fh)
        first = next(reader, None)
    if not first:
        return False
    return [_normalize_header(col) for col in first] == [_normalize_header(col) for col in expected_header]


def _iter_rows(path: Path, header: List[str]) -> Iterator[Dict[str, str]]:
    has_header = _file_has_header(path, header)
    with path.open("r", encoding="utf-8", newline="") as fh:
        if has_header:
            reader = csv.DictReader(fh)
        else:
            reader = csv.DictReader(fh, fieldnames=header)
        for row in reader:
            yield {key: str(row.get(key, "")).strip() for key in header}


def _count_rows(path: Path, header: List[str]) -> int:
    has_header = _file_has_header(path, header)
    with path.open("r", encoding="utf-8", newline="") as fh:
        reader = csv.reader(fh)
        total = sum(1 for _ in reader)
    return max(total - (1 if has_header else 0), 0)


def _row_key(row: Dict[str, str]) -> Tuple[str, str, str, str, str, str]:
    return (
        row.get("task_name", ""),
        row.get("job_name", ""),
        row.get("start_time", ""),
        row.get("end_time", ""),
        row.get("plan_cpu", ""),
        row.get("plan_mem", ""),
    )


def _collect_reference_keys(path: Path) -> set[Tuple[str, str, str, str, str, str]]:
    if not path.exists():
        return set()
    return {_row_key(row) for row in _iter_rows(path, TASK_HEADER)}


def _iter_contiguous_rows(rows: Iterable[Dict[str, str]], start_index: int, count: int) -> Iterator[Dict[str, str]]:
    end_index = start_index + count
    for idx, row in enumerate(rows):
        if idx < start_index:
            continue
        if idx >= end_index:
            break
        yield row


def _write_csv(path: Path, header: List[str], rows: Iterable[Dict[str, str]]) -> int:
    path.parent.mkdir(parents=True, exist_ok=True)
    count = 0
    with path.open("w", encoding="utf-8", newline="") as fh:
        writer = csv.DictWriter(fh, fieldnames=header)
        writer.writeheader()
        for row in rows:
            writer.writerow({k: row.get(k, "") for k in header})
            count += 1
    return count


def create_split(
    source_trace_dir: Path,
    dest_trace_dir: Path,
    task_count: int,
    force: bool,
    start_row: int,
    reference_test_csv: Path | None,
    allow_overlap: bool,
) -> Tuple[int, int]:
    src_task = source_trace_dir / "batch_task.csv"
    src_machine = source_trace_dir / "machine_meta.csv"
    if not src_task.exists():
        raise FileNotFoundError(f"Task CSV not found: {src_task}")
    if not src_machine.exists():
        raise FileNotFoundError(f"Machine CSV not found: {src_machine}")

    dst_task = dest_trace_dir / "batch_task.csv"
    dst_machine = dest_trace_dir / "machine_meta.csv"
    metadata_path = dest_trace_dir / "metadata.json"

    if dest_trace_dir.exists() and not force:
        if dst_task.exists() and dst_machine.exists():
            raise FileExistsError(
                f"Destination split already exists at {dest_trace_dir}. "
                "Use --force to overwrite."
            )
    dest_trace_dir.mkdir(parents=True, exist_ok=True)

    total_tasks = _count_rows(src_task, TASK_HEADER)
    if total_tasks == 0:
        raise ValueError(f"No task rows found in {src_task}")
    target = min(task_count, total_tasks)

    start_index = max(start_row - 1, 0)
    if start_index + target > total_tasks:
        raise ValueError(
            f"Requested contiguous window start_row={start_row}, task_count={target} "
            f"exceeds source size ({total_tasks} rows)"
        )

    reference_keys = _collect_reference_keys(reference_test_csv) if reference_test_csv is not None else set()

    contiguous_rows = list(_iter_contiguous_rows(_iter_rows(src_task, TASK_HEADER), start_index=start_index, count=target))
    if len(contiguous_rows) != target:
        raise ValueError(f"Could not read expected contiguous window: wanted {target}, got {len(contiguous_rows)}")

    overlap_count = 0
    if reference_keys:
        overlap_count = sum(1 for row in contiguous_rows if _row_key(row) in reference_keys)
        if overlap_count > 0 and not allow_overlap:
            raise ValueError(
                f"Contiguous split overlaps reference test split by {overlap_count} rows. "
                "Choose a different --start-row or pass --allow-overlap."
            )

    written_tasks = _write_csv(dst_task, TASK_HEADER, contiguous_rows)
    written_machines = _write_csv(dst_machine, MACHINE_HEADER, _iter_rows(src_machine, MACHINE_HEADER))

    metadata = {
        "source_trace_dir": str(source_trace_dir),
        "dest_trace_dir": str(dest_trace_dir),
        "source_task_rows": total_tasks,
        "requested_task_rows": int(task_count),
        "start_row_1_based": int(start_row),
        "end_row_1_based": int(start_row + target - 1),
        "written_task_rows": int(written_tasks),
        "written_machine_rows": int(written_machines),
        "reference_test_csv": str(reference_test_csv) if reference_test_csv is not None else "",
        "overlap_with_reference_rows": int(overlap_count),
        "split_name": dest_trace_dir.name,
        "selection_mode": "contiguous",
    }
    metadata_path.write_text(json.dumps(metadata, indent=2), encoding="utf-8")
    return written_tasks, written_machines


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create a deterministic Alibaba test split")
    parser.add_argument(
        "--source-trace-dir",
        type=Path,
        default=Path("agentic_scheduler/data/alibaba_v2018/core"),
        help="Source trace directory containing batch_task.csv and machine_meta.csv",
    )
    parser.add_argument(
        "--dest-trace-dir",
        type=Path,
        default=Path("agentic_scheduler/data/alibaba_v2018/alibaba_test"),
        help="Output directory for generated split",
    )
    parser.add_argument(
        "--task-count",
        type=int,
        default=300000,
        help="Number of task rows to include in generated split",
    )
    parser.add_argument(
        "--start-row",
        type=int,
        default=0,
        help="1-based starting row in source batch_task.csv. If 0, uses trailing contiguous window.",
    )
    parser.add_argument(
        "--reference-test-csv",
        type=Path,
        default=Path("agentic_scheduler/data/alibaba_v2018/test/batch_task.csv"),
        help="Reference split for overlap checks (set empty string to disable)",
    )
    parser.add_argument(
        "--allow-overlap",
        action="store_true",
        help="Allow overlap with reference test split",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Overwrite destination split if it already exists",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.task_count <= 0:
        raise ValueError("--task-count must be > 0")

    source_task = args.source_trace_dir / "batch_task.csv"
    total_rows = _count_rows(source_task, TASK_HEADER)
    if total_rows <= 0:
        raise ValueError(f"No rows found in {source_task}")

    target = min(args.task_count, total_rows)
    if args.start_row > 0:
        start_row = args.start_row
    else:
        # Default to trailing chunk to avoid common overlap with early train/test slices.
        start_row = max(total_rows - target + 1, 1)

    reference_csv = args.reference_test_csv
    if str(reference_csv).strip() == "":
        reference_csv = None

    written_tasks, written_machines = create_split(
        source_trace_dir=args.source_trace_dir,
        dest_trace_dir=args.dest_trace_dir,
        task_count=args.task_count,
        force=args.force,
        start_row=start_row,
        reference_test_csv=reference_csv,
        allow_overlap=args.allow_overlap,
    )
    print(
        f"Created Alibaba test split at {args.dest_trace_dir} "
        f"({written_tasks} tasks, {written_machines} machines; start_row={start_row})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
