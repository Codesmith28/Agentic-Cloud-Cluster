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
COMPOSE_FILE="${REPO_ROOT}/testbench/docker-compose.yml"

MASTER_URL="${MASTER_URL:-http://localhost:8080}"
RUN_COMPOSE_UP="${RUN_COMPOSE_UP:-true}"
AUTO_DOWN="${AUTO_DOWN:-false}"

cleanup() {
  if [[ "${AUTO_DOWN}" == "true" ]]; then
    echo "Tearing down testbench stack..."
    docker compose -f "${COMPOSE_FILE}" down --remove-orphans >/dev/null
  fi
}
trap cleanup EXIT

if [[ "${RUN_COMPOSE_UP}" == "true" ]]; then
  echo "Starting testbench stack..."
  docker compose -f "${COMPOSE_FILE}" up -d --build
fi

echo "Registering workers..."
MASTER_URL="${MASTER_URL}" "${SCRIPT_DIR}/register_workers.sh"

echo "Running workload..."
python3 "${SCRIPT_DIR}/run_workload.py" --master-url "${MASTER_URL}" "$@"
