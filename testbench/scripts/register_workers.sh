#!/usr/bin/env bash
set -euo pipefail

MASTER_URL="${MASTER_URL:-http://localhost:8080}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-40}"
SLEEP_SECONDS="${SLEEP_SECONDS:-2}"
WORKER_SPECS="${TESTBENCH_WORKERS:-worker-small=worker-small:50052,worker-medium=worker-medium:50052,worker-large=worker-large:50052}"

readarray -t WORKERS < <(printf '%s' "${WORKER_SPECS}" | tr ',' '\n')

wait_for_master() {
  local attempt
  for attempt in $(seq 1 "${MAX_ATTEMPTS}"); do
    if curl -fsS "${MASTER_URL}/telemetry" >/dev/null 2>&1; then
      echo "Master API reachable at ${MASTER_URL}"
      return 0
    fi
    sleep "${SLEEP_SECONDS}"
  done
  echo "Master API did not become ready after ${MAX_ATTEMPTS} attempts" >&2
  return 1
}

register_worker() {
  local worker_id="$1"
  local worker_addr="$2"
  local attempt

  for attempt in $(seq 1 "${MAX_ATTEMPTS}"); do
    local payload
    payload=$(printf '{"worker_id":"%s","worker_ip":"%s"}' "${worker_id}" "${worker_addr}")

    local response_file
    response_file="$(mktemp)"
    local status
    status=$(curl -sS -o "${response_file}" -w "%{http_code}" -X POST "${MASTER_URL}/api/workers" -H "Content-Type: application/json" -d "${payload}" || true)

    if [[ "${status}" == "201" || "${status}" == "409" ]]; then
      rm -f "${response_file}"
      echo "Registered ${worker_id} (${worker_addr})"
      return 0
    fi

    rm -f "${response_file}"
    sleep "${SLEEP_SECONDS}"
  done

  echo "Failed to register ${worker_id} after ${MAX_ATTEMPTS} attempts" >&2
  return 1
}

wait_for_workers_active() {
  local target_ids
  target_ids="$(printf '%s\n' "${WORKERS[@]}" | awk -F= '{print $1}' | paste -sd, -)"

  local attempt
  for attempt in $(seq 1 "${MAX_ATTEMPTS}"); do
    local body
    body="$(curl -fsS "${MASTER_URL}/api/workers" || true)"
    if [[ -z "${body}" ]]; then
      sleep "${SLEEP_SECONDS}"
      continue
    fi

    if TARGET_IDS="${target_ids}" WORKERS_JSON="${body}" python3 - <<'PY'
import json
import os

target_ids = [x for x in os.environ["TARGET_IDS"].split(",") if x]
payload = json.loads(os.environ["WORKERS_JSON"])
workers = {item.get("worker_id"): item for item in payload.get("workers", [])}

missing = []
inactive = []
for worker_id in target_ids:
    info = workers.get(worker_id)
    if info is None:
        missing.append(worker_id)
        continue
    if not info.get("is_active", False):
        inactive.append(worker_id)

if missing or inactive:
    if missing:
        print("waiting for workers to appear:", ",".join(missing))
    if inactive:
        print("waiting for workers to become active:", ",".join(inactive))
    raise SystemExit(1)

print("all workers active")
PY
    then
      echo "All workers are active"
      return 0
    fi

    sleep "${SLEEP_SECONDS}"
  done

  echo "Workers did not become active after ${MAX_ATTEMPTS} attempts" >&2
  return 1
}

main() {
  wait_for_master

  local spec worker_id worker_addr
  for spec in "${WORKERS[@]}"; do
    worker_id="${spec%%=*}"
    worker_addr="${spec#*=}"
    register_worker "${worker_id}" "${worker_addr}"
  done

  wait_for_workers_active
}

main "$@"
