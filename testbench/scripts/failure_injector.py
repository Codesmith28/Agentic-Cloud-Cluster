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

"""Inject controlled failures into the CloudAI testbench stack."""

from __future__ import annotations

import argparse
import pathlib
import subprocess
import sys
import time


def run_command(cmd: list[str]) -> int:
    proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if proc.returncode != 0:
        detail = proc.stderr.strip() or proc.stdout.strip() or "unknown-error"
        print(f"[failure-injector] command failed ({proc.returncode}): {' '.join(cmd)} :: {detail}", file=sys.stderr)
    return proc.returncode


def resolve_container_id(compose_file: pathlib.Path, service: str) -> str:
    proc = subprocess.run(
        ["docker", "compose", "-f", str(compose_file), "ps", "-q", service],
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        return ""
    return proc.stdout.strip()


def main() -> int:
    parser = argparse.ArgumentParser(description="Inject failure modes for campaign recovery suites")
    parser.add_argument(
        "--compose-file",
        type=pathlib.Path,
        default=pathlib.Path(__file__).resolve().parents[1] / "docker-compose.yml",
        help="Path to docker compose file for the testbench stack",
    )
    parser.add_argument(
        "--action",
        required=True,
        choices=[
            "kill-worker",
            "pause-worker-dind",
            "resume-worker-dind",
            "kill-dind",
            "restart-master",
            "bad-image-tag",
            "replay-stale-result",
        ],
        help="Failure action to inject",
    )
    parser.add_argument("--worker", default="worker-small", help="Worker service basename (worker-small/worker-medium/worker-large)")
    parser.add_argument("--delay-seconds", type=float, default=0.0, help="Delay before action execution")
    args = parser.parse_args()

    if args.delay_seconds > 0:
        time.sleep(args.delay_seconds)

    compose_file = args.compose_file.resolve()
    worker_service = args.worker
    dind_service = f"{worker_service}-dind"

    if args.action == "kill-worker":
        return run_command(["docker", "compose", "-f", str(compose_file), "kill", worker_service])

    if args.action == "kill-dind":
        return run_command(["docker", "compose", "-f", str(compose_file), "kill", dind_service])

    if args.action == "restart-master":
        return run_command(["docker", "compose", "-f", str(compose_file), "restart", "master"])

    if args.action == "pause-worker-dind":
        container_id = resolve_container_id(compose_file, dind_service)
        if not container_id:
            print(f"[failure-injector] no container found for service {dind_service}", file=sys.stderr)
            return 1
        return run_command(["docker", "pause", container_id])

    if args.action == "resume-worker-dind":
        container_id = resolve_container_id(compose_file, dind_service)
        if not container_id:
            print(f"[failure-injector] no container found for service {dind_service}", file=sys.stderr)
            return 1
        return run_command(["docker", "unpause", container_id])

    if args.action == "bad-image-tag":
        # This fault mode is represented by explicit bad-image tasks in failure-stressed workload manifests.
        print("[failure-injector] bad-image-tag is encoded by workload task definitions; no compose mutation required")
        return 0

    if args.action == "replay-stale-result":
        # Stale-result replay is exercised by recovery tests/workloads and server-side suppression.
        print("[failure-injector] stale-result replay requires scenario-level task replay orchestration; no-op here")
        return 0

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
