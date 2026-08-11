#!/usr/bin/env python3


"""Submit a workload to CloudAI and wait for completion."""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import shlex
import sys
import time
from typing import Any

from shared_polling import TERMINAL_STATUSES, poll_task_completion, request_json
DEFAULT_WORKFLOW_IMAGE = "cloudai/workflow-deterministic:v1"


def default_workload_path() -> pathlib.Path:
    return pathlib.Path(__file__).resolve().parents[1] / "workloads" / "heterogeneous-smoke.json"


def default_output_dir() -> pathlib.Path:
    return pathlib.Path(__file__).resolve().parents[2] / "results" / "testbench"


def resolve_workflow_image(task: dict[str, Any]) -> str:
    if "docker_image" in task and str(task["docker_image"]).strip():
        return str(task["docker_image"]).strip()

    if task.get("workflow") or task.get("workflow_params") or task.get("workflow_profile"):
        return os.environ.get("CLOUDAI_WORKFLOW_IMAGE_TAG", DEFAULT_WORKFLOW_IMAGE)

    raise ValueError("task missing docker_image")


def resolve_command(task: dict[str, Any]) -> str:
    workflow = task.get("workflow") or task.get("workflow_params")
    legacy_profile = task.get("workflow_profile")
    legacy_args = task.get("workflow_args")

    if workflow is None and legacy_profile:
        cmd_parts: list[str] = ["/usr/local/bin/workflow", str(legacy_profile)]
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

    cmd_parts: list[str] = ["/usr/local/bin/workflow", str(profile)]
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


def submit_tasks(master_url: str, tasks: list[dict[str, Any]]) -> list[dict[str, Any]]:
    submitted: list[dict[str, Any]] = []

    for idx, task in enumerate(tasks, start=1):
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
            "user_id": task.get("user_id", "testbench"),
        }

        response = request_json("POST", f"{master_url}/api/tasks", payload=payload, timeout=20.0)
        task_id = response.get("task_id")
        if not task_id:
            raise RuntimeError(f"task submission {idx} did not return task_id: {response}")

        submitted.append(
            {
                "index": idx,
                "task_id": task_id,
                "tag": payload["tag"],
                "cpu_required": payload["cpu_required"],
                "memory_required": payload["memory_required"],
                "docker_image": payload["docker_image"],
                "command": payload["command"],
            }
        )
        print(f"[submit {idx:02d}/{len(tasks):02d}] {task_id} queued")

        delay = float(task.get("arrival_delay_sec", 0.0))
        if delay > 0:
            time.sleep(delay)

    return submitted


def wait_for_completion(
    master_url: str,
    submitted: list[dict[str, Any]],
    timeout_seconds: int,
    poll_interval: float,
) -> dict[str, dict[str, Any]]:
    task_ids = [item["task_id"] for item in submitted]
    raw_statuses = poll_task_completion(master_url, task_ids, timeout_seconds, poll_interval)
    statuses: dict[str, dict[str, Any]] = {}
    completed = 0
    failed = 0
    for task_id in task_ids:
        status = raw_statuses.get(task_id, "unknown")
        statuses[task_id] = {"status": status, "last_update": time.time()}
        if status in TERMINAL_STATUSES:
            completed += 1
            if status != "completed":
                failed += 1
    print(f"[poll] completed={completed}/{len(task_ids)} failed={failed}")
    if completed != len(task_ids):
        raise TimeoutError(f"timeout waiting for workload completion ({timeout_seconds}s)")
    return statuses


def write_summary(
    output_path: pathlib.Path,
    workload_path: pathlib.Path,
    started_at: float,
    submitted: list[dict[str, Any]],
    statuses: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    finished_at = time.time()
    by_status: dict[str, int] = {}
    tasks_out: list[dict[str, Any]] = []

    for task in submitted:
        status = statuses.get(task["task_id"], {}).get("status", "unknown")
        by_status[status] = by_status.get(status, 0) + 1
        enriched = dict(task)
        enriched["status"] = status
        tasks_out.append(enriched)

    summary = {
        "workload_file": str(workload_path),
        "started_at_unix": started_at,
        "finished_at_unix": finished_at,
        "duration_seconds": round(finished_at - started_at, 3),
        "totals": {
            "submitted": len(submitted),
            "completed": by_status.get("completed", 0),
            "failed": by_status.get("failed", 0),
            "cancelled": by_status.get("cancelled", 0),
        },
        "status_breakdown": by_status,
        "tasks": tasks_out,
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")
    return summary


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run CloudAI workload and wait for completion")
    parser.add_argument(
        "--master-url",
        default="http://localhost:8080",
        help="Base URL for master HTTP API",
    )
    parser.add_argument(
        "--workload",
        type=pathlib.Path,
        default=default_workload_path(),
        help="Path to workload JSON file",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=int,
        default=900,
        help="Max wait time for all tasks to reach terminal status",
    )
    parser.add_argument(
        "--poll-interval",
        type=float,
        default=2.0,
        help="Polling interval in seconds",
    )
    parser.add_argument(
        "--output",
        type=pathlib.Path,
        default=None,
        help="Path to write summary JSON (defaults to results/testbench/<timestamp>.json)",
    )
    parser.add_argument(
        "--fail-on-task-failure",
        action="store_true",
        help="Exit non-zero when one or more tasks are failed/cancelled",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    workload_path = args.workload.resolve()
    master_url = args.master_url.rstrip("/")

    workload = json.loads(workload_path.read_text(encoding="utf-8"))
    tasks = workload.get("tasks", [])
    if not tasks:
        print(f"No tasks found in workload: {workload_path}", file=sys.stderr)
        return 2

    if args.output is None:
        timestamp = time.strftime("%Y%m%d-%H%M%S", time.localtime())
        output_path = default_output_dir() / f"{timestamp}-summary.json"
    else:
        output_path = args.output.resolve()

    print(f"Running workload: {workload.get('name', workload_path.name)}")
    print(f"Master API: {master_url}")
    print(f"Task count: {len(tasks)}")

    started_at = time.time()
    submitted = submit_tasks(master_url, tasks)
    statuses = wait_for_completion(master_url, submitted, args.timeout_seconds, args.poll_interval)
    summary = write_summary(output_path, workload_path, started_at, submitted, statuses)

    print("Workload finished")
    print(f"Summary file: {output_path}")
    print(f"Totals: {summary['totals']}")

    if args.fail_on_task_failure:
        if summary["totals"]["failed"] > 0 or summary["totals"]["cancelled"] > 0:
            return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
