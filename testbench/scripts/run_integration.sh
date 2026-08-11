#!/usr/bin/env bash

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

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MASTER_BIN="${REPO_ROOT}/master/masterNode"

repo_env_value() {
  local key="$1"
  local env_file="${REPO_ROOT}/.env"
  if [[ ! -f "${env_file}" ]]; then
    return 0
  fi
  python3 - "${env_file}" "${key}" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
target = sys.argv[2]
for raw in path.read_text(encoding="utf-8").splitlines():
    line = raw.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    key, value = line.split("=", 1)
    if key.strip() == target:
        print(value.strip().strip('"').strip("'"))
        break
PY
}

RUN_TIMESTAMP="${RUN_TIMESTAMP:-$(date +%Y%m%d-%H%M%S)}"
RUN_ROOT="${RUN_ROOT:-${REPO_ROOT}/results/testbench/${RUN_TIMESTAMP}-integration}"
PROFILE="${PROFILE:-hetero-small}"
BASE_SCHEDULER="${BASE_SCHEDULER:-current}"
EVIDENCE_SCHEDULER="${EVIDENCE_SCHEDULER:-current}"
RUN_UNIT_TESTS="${RUN_UNIT_TESTS:-true}"
BUILD_MASTER="${BUILD_MASTER:-true}"
KEEP_ENV="${KEEP_ENV:-false}"
CLEANUP_ON_EXIT="${CLEANUP_ON_EXIT:-true}"

TESTBENCH_COMPOSE_FILE="${TESTBENCH_COMPOSE_FILE:-${REPO_ROOT}/testbench/docker-compose.host-master.yml}"
DEFAULT_WORKER_SPECS="worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054"
WORKER_SPECS="${WORKER_SPECS:-${DEFAULT_WORKER_SPECS}}"
GF_ADMIN_USER="${GF_ADMIN_USER:-$(repo_env_value GF_ADMIN_USER)}"
GF_ADMIN_PASSWORD="${GF_ADMIN_PASSWORD:-$(repo_env_value GF_ADMIN_PASSWORD)}"
GF_ADMIN_USER="${GF_ADMIN_USER:-admin}"
GF_ADMIN_PASSWORD="${GF_ADMIN_PASSWORD:-password}"
MASTER_BIND_ADDR="${MASTER_BIND_ADDR:-0.0.0.0:50051}"
MASTER_ADVERTISE_ADDR="${MASTER_ADVERTISE_ADDR:-localhost:50051}"

require_command() {
  local command_name="$1"
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "Missing required command: ${command_name}" >&2
    exit 1
  fi
}

ensure_docker_ready() {
  if ! docker info >/dev/null 2>&1; then
    echo "Docker daemon is not running. Start Docker Desktop/Engine and retry." >&2
    exit 1
  fi
  if ! docker compose version >/dev/null 2>&1; then
    echo "docker compose is unavailable. Install/enable Docker Compose and retry." >&2
    exit 1
  fi
}

ensure_master_binary() {
  if [[ "${BUILD_MASTER}" == "true" ]]; then
    echo "Building master binary..."
    (cd "${REPO_ROOT}" && make master)
    return 0
  fi
  if [[ -x "${MASTER_BIN}" ]]; then
    return 0
  fi
  echo "master binary missing at ${MASTER_BIN} (set BUILD_MASTER=true or run make master)." >&2
  exit 1
}

run_preflight_tests() {
  if [[ "${RUN_UNIT_TESTS}" != "true" ]]; then
    echo "Skipping Go unit-test preflight (RUN_UNIT_TESTS=${RUN_UNIT_TESTS})"
    return 0
  fi
  echo "Running Go unit-test preflight..."
  (cd "${REPO_ROOT}" && make test-unit)
}

run_suite() {
  local suite="$1"
  local scheduler="$2"
  local output_dir="$3"
  local -a cmd=("${MASTER_BIN}" test run "${suite}" -profile "${PROFILE}" -out "${output_dir}" -scheduler "${scheduler}")

  if [[ "${KEEP_ENV}" == "true" ]]; then
    cmd+=(-keep-env)
  fi

  echo "Running suite '${suite}' (scheduler=${scheduler})..."
  "${cmd[@]}"
}

write_integration_summary() {
  RUN_ROOT="${RUN_ROOT}" \
  PROFILE="${PROFILE}" \
  BASE_SCHEDULER="${BASE_SCHEDULER}" \
  EVIDENCE_SCHEDULER="${EVIDENCE_SCHEDULER}" \
  COMPOSE_FILE="${TESTBENCH_COMPOSE_FILE}" \
  python3 - <<'PY'
import json
import os
from pathlib import Path

run_root = Path(os.environ["RUN_ROOT"])
summary_path = run_root / "integration-summary.json"

summary = {
    "profile": os.environ["PROFILE"],
    "base_scheduler": os.environ["BASE_SCHEDULER"],
    "evidence_scheduler": os.environ["EVIDENCE_SCHEDULER"],
    "compose_file": os.environ["COMPOSE_FILE"],
    "run_root": str(run_root),
    "suite_outputs": {
        "smoke": str(run_root / "smoke"),
        "reliability": str(run_root / "reliability"),
        "ui-smoke": str(run_root / "ui-smoke"),
        "evidence": str(run_root / "evidence"),
    },
    "success": True,
}

summary_path.parent.mkdir(parents=True, exist_ok=True)
summary_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")
print(f"Wrote integration summary: {summary_path}")
PY
}

cleanup() {
  if [[ "${CLEANUP_ON_EXIT}" != "true" || "${KEEP_ENV}" == "true" ]]; then
    return
  fi
  if [[ ! -x "${MASTER_BIN}" ]]; then
    return
  fi

  TESTBENCH_COMPOSE_FILE="${TESTBENCH_COMPOSE_FILE}" \
  WORKER_SPECS="${WORKER_SPECS}" \
  GF_ADMIN_USER="${GF_ADMIN_USER}" \
  GF_ADMIN_PASSWORD="${GF_ADMIN_PASSWORD}" \
  "${MASTER_BIN}" test cleanup >/dev/null 2>&1 || true
}

main() {
  require_command docker
  require_command make
  require_command go
  require_command python3

  ensure_docker_ready
  ensure_master_binary

  export TESTBENCH_COMPOSE_FILE
  export WORKER_SPECS
  export GF_ADMIN_USER
  export GF_ADMIN_PASSWORD
  export MASTER_BIND_ADDR
  export MASTER_ADVERTISE_ADDR

  mkdir -p "${RUN_ROOT}"
  trap cleanup EXIT

  run_preflight_tests
  run_suite smoke "${BASE_SCHEDULER}" "${RUN_ROOT}/smoke"
  run_suite reliability "${BASE_SCHEDULER}" "${RUN_ROOT}/reliability"
  run_suite ui-smoke "${BASE_SCHEDULER}" "${RUN_ROOT}/ui-smoke"
  run_suite evidence "${EVIDENCE_SCHEDULER}" "${RUN_ROOT}/evidence"
  write_integration_summary

  echo "Integration automation complete. Artifacts: ${RUN_ROOT}"
}

main "$@"
