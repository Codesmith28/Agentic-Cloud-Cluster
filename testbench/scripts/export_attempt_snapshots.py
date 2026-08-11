#!/usr/bin/env python3


"""Export per-task attempt snapshots from master APIs."""

from __future__ import annotations

import argparse
import json
import pathlib
from typing import Any

from shared_polling import request_json


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Export task and attempt snapshots")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master API URL")
    parser.add_argument(
        "--summary",
        type=pathlib.Path,
        default=None,
        help="Optional workload summary JSON with a tasks[] list",
    )
    parser.add_argument(
        "--task-id",
        action="append",
        default=[],
        help="Task ID to export (repeatable). If omitted, summary or /api/tasks is used.",
    )
    parser.add_argument(
        "--no-master-discovery",
        action="store_true",
        help="Do not fall back to /api/tasks when no task IDs are provided or found in summary",
    )
    parser.add_argument("--output-dir", required=True, type=pathlib.Path, help="Output directory")
    return parser.parse_args()


def load_task_ids(
    master_url: str,
    summary_path: pathlib.Path | None,
    explicit_ids: list[str],
    allow_master_discovery: bool,
) -> list[str]:
    if explicit_ids:
        return list(dict.fromkeys(explicit_ids))

    if summary_path is not None:
        summary = json.loads(summary_path.read_text(encoding="utf-8"))
        ids: list[str] = []
        for task in summary.get("tasks", []):
            task_id = str(task.get("task_id", "")).strip()
            if task_id:
                ids.append(task_id)
        if ids:
            return list(dict.fromkeys(ids))

    if not allow_master_discovery:
        return []

    payload = request_json("GET", f"{master_url.rstrip('/')}/api/tasks")
    ids = []
    for task in payload.get("tasks", []):
        task_id = str(task.get("task_id", "")).strip()
        if task_id:
            ids.append(task_id)
    return list(dict.fromkeys(ids))


def export_snapshots(master_url: str, task_ids: list[str], output_dir: pathlib.Path) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    exported: list[str] = []
    failures: dict[str, str] = {}

    for task_id in task_ids:
        try:
            snapshot = request_json("GET", f"{master_url.rstrip('/')}/api/tasks/{task_id}", timeout=15.0)
        except Exception as exc:  # pylint: disable=broad-except
            failures[task_id] = f"{type(exc).__name__}: {exc}"
            continue

        task_file = output_dir / f"{task_id}.json"
        task_file.write_text(json.dumps(snapshot, indent=2), encoding="utf-8")
        exported.append(task_id)

    index = {
        "master_url": master_url.rstrip("/"),
        "task_ids": task_ids,
        "exported_count": len(exported),
        "exported_task_ids": exported,
        "failed_count": len(failures),
        "failures": failures,
    }
    (output_dir / "index.json").write_text(json.dumps(index, indent=2), encoding="utf-8")
    return index


def main() -> int:
    args = parse_args()
    master_url = args.master_url.rstrip("/")
    task_ids = load_task_ids(master_url, args.summary, args.task_id, not args.no_master_discovery)
    index = export_snapshots(master_url, task_ids, args.output_dir.resolve())
    print(
        f"Attempt snapshots exported: {index['exported_count']} tasks "
        f"(failed={index['failed_count']}) -> {args.output_dir.resolve()}"
    )
    return 0 if index["failed_count"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
