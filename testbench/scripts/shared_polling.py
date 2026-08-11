#!/usr/bin/env python3


"""Shared HTTP helpers and polling utilities for testbench scripts."""

from __future__ import annotations

import json
import time
import urllib.request
from typing import Any

TERMINAL_STATUSES = {"completed", "failed", "cancelled"}


def request_json(
    method: str,
    url: str,
    payload: dict[str, Any] | None = None,
    timeout: float = 10.0,
) -> dict[str, Any]:
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(url=url, method=method, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
        return json.loads(body) if body else {}


def poll_task_completion(
    master_url: str,
    task_ids: list[str],
    timeout_seconds: int = 600,
    poll_interval: float = 3.0,
) -> dict[str, str]:
    """Poll task statuses until all become terminal or timeout is reached."""
    if not task_ids:
        return {}

    deadline = time.time() + timeout_seconds
    statuses: dict[str, str] = {task_id: "queued" for task_id in task_ids}
    while time.time() < deadline:
        done = 0
        for task_id in task_ids:
            if statuses[task_id] in TERMINAL_STATUSES:
                done += 1
                continue
            try:
                task_info = request_json("GET", f"{master_url}/api/tasks/{task_id}", timeout=10.0)
            except Exception:  # pylint: disable=broad-except
                continue
            status = str(task_info.get("status", "unknown")).lower()
            statuses[task_id] = status
            if status in TERMINAL_STATUSES:
                done += 1

        if done == len(task_ids):
            return statuses
        time.sleep(poll_interval)

    return statuses
