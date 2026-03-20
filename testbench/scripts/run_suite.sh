#!/usr/bin/env bash
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
