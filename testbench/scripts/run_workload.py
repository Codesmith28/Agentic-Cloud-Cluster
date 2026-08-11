#!/usr/bin/env python3

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

"""Submit a workload to CloudAI and wait for completion."""

from __future__ import annotations

import argparse
import json
import pathlib
import shlex
import sys
import time
import urllib.error
import urllib.request
from typing import Any


TERMINAL_STATUSES = {"completed", "failed", "cancelled"}
DEFAULT_WORKFLOW_IMAGE = "cloudai-benchmark:1"


def default_workload_path() -> pathlib.Path:
    return pathlib.Path(__file__).resolve().parents[1] / "workloads" / "heterogeneous-smoke.json"


def default_output_dir() -> pathlib.Path:
    return pathlib.Path(__file__).resolve().parents[2] / "results" / "testbench"


def request_json(method: str, url: str, payload: dict[str, Any] | None = None, timeout: float = 10.0) -> dict[str, Any]:
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(url=url, method=method, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
        if not body:
            return {}
        return json.loads(body)


def build_workflow_command(task: dict[str, Any]) -> tuple[str, str]:
    workflow_profile = str(task.get("workflow_profile", "")).strip()
    workflow_args = task.get("workflow_args", [])
    if not workflow_profile:
        return str(task["docker_image"]), str(task.get("command", ""))

    if not isinstance(workflow_args, list):
        raise ValueError(f"workflow_args must be a list for workflow profile {workflow_profile}")

    image = str(task.get("docker_image") or DEFAULT_WORKFLOW_IMAGE)
    command_parts = ["cloudai-benchmark", workflow_profile, *[str(item) for item in workflow_args]]
    return image, shlex.join(command_parts)


def submit_tasks(master_url: str, tasks: list[dict[str, Any]]) -> list[dict[str, Any]]:
    submitted: list[dict[str, Any]] = []

    for idx, task in enumerate(tasks, start=1):
        image, command = build_workflow_command(task)
        payload = {
            "docker_image": image,
            "command": command,
            "cpu_required": task["cpu_required"],
            "memory_required": task["memory_required"],
            "storage_required": task.get("storage_required", 1),
            "tag": task.get("tag") or task.get("workflow_profile", ""),
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
                "task_name": task.get("task_name", f"task-{idx:02d}"),
                "tag": payload["tag"],
                "cpu_required": payload["cpu_required"],
                "memory_required": payload["memory_required"],
                "storage_required": payload["storage_required"],
                "docker_image": payload["docker_image"],
                "command": payload["command"],
                "workflow_profile": task.get("workflow_profile", ""),
                "expected_status": task.get("expected_status", "completed"),
            }
        )
        print(f"[submit {idx:02d}/{len(tasks):02d}] {task_id} queued ({payload['docker_image']} {payload['command']})")

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
    deadline = time.time() + timeout_seconds
    task_ids = [item["task_id"] for item in submitted]
    statuses: dict[str, dict[str, Any]] = {
        task_id: {"status": "queued", "last_update": time.time()} for task_id in task_ids
    }

    while time.time() < deadline:
        completed = 0
        failed = 0

        for task_id in task_ids:
            try:
                task_info = request_json("GET", f"{master_url}/api/tasks/{task_id}", timeout=10.0)
            except urllib.error.HTTPError as exc:
                statuses[task_id]["status"] = f"http-{exc.code}"
                continue
            except Exception as exc:  # pylint: disable=broad-except
                statuses[task_id]["status"] = f"error:{type(exc).__name__}"
                continue

            status = str(task_info.get("status", "unknown")).lower()
            statuses[task_id]["status"] = status
            statuses[task_id]["last_update"] = time.time()
            statuses[task_id]["task"] = task_info

            if status in TERMINAL_STATUSES:
                completed += 1
                if status != "completed":
                    failed += 1

        print(f"[poll] completed={completed}/{len(task_ids)} failed={failed}")
        if completed == len(task_ids):
            return statuses

        time.sleep(poll_interval)

    raise TimeoutError(f"timeout waiting for workload completion ({timeout_seconds}s)")


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
    expected_mismatches = 0

    for task in submitted:
        status = statuses.get(task["task_id"], {}).get("status", "unknown")
        by_status[status] = by_status.get(status, 0) + 1
        enriched = dict(task)
        enriched["status"] = status
        enriched["task_details"] = statuses.get(task["task_id"], {}).get("task", {})
        if task.get("expected_status") and task["expected_status"] != status:
            expected_mismatches += 1
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
            "expected_status_mismatches": expected_mismatches,
        },
        "status_breakdown": by_status,
        "tasks": tasks_out,
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")
    return summary


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run CloudAI workload and wait for completion")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Base URL for master HTTP API")
    parser.add_argument("--workload", type=pathlib.Path, default=default_workload_path(), help="Path to workload JSON file")
    parser.add_argument("--timeout-seconds", type=int, default=900, help="Max wait time for all tasks to reach terminal status")
    parser.add_argument("--poll-interval", type=float, default=2.0, help="Polling interval in seconds")
    parser.add_argument("--output", type=pathlib.Path, default=None, help="Path to write summary JSON")
    parser.add_argument("--fail-on-task-failure", action="store_true", help="Exit non-zero when one or more tasks are failed/cancelled")
    parser.add_argument("--fail-on-expected-mismatch", action="store_true", help="Exit non-zero when actual task status differs from expected_status in workload")
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

    if args.fail_on_task_failure and (summary["totals"]["failed"] > 0 or summary["totals"]["cancelled"] > 0):
        return 1
    if args.fail_on_expected_mismatch and summary["totals"]["expected_status_mismatches"] > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
