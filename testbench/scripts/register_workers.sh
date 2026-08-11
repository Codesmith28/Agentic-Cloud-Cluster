#!/usr/bin/env bash


set -euo pipefail

MASTER_URL="${MASTER_URL:-http://localhost:8080}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-40}"
SLEEP_SECONDS="${SLEEP_SECONDS:-2}"
WORKER_SPECS="${WORKER_SPECS:-}"

DEFAULT_WORKERS=(
  "worker-small worker-small:50052"
  "worker-medium worker-medium:50052"
  "worker-large worker-large:50052"
)
WORKERS=()

trim_whitespace() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf "%s" "${value}"
}

load_workers() {
  if [[ -z "${WORKER_SPECS}" ]]; then
    WORKERS=("${DEFAULT_WORKERS[@]}")
    return 0
  fi

  local normalized
  normalized="${WORKER_SPECS//$'\r'/}"
  normalized="${normalized//;/$'\n'}"
  normalized="${normalized//,/$'\n'}"

  WORKERS=()
  local raw_spec worker_id worker_addr extra
  while IFS= read -r raw_spec; do
    raw_spec="$(trim_whitespace "${raw_spec}")"
    if [[ -z "${raw_spec}" ]]; then
      continue
    fi

    worker_id=""
    worker_addr=""
    extra=""

    if [[ "${raw_spec}" == *"="* ]]; then
      worker_id="$(trim_whitespace "${raw_spec%%=*}")"
      worker_addr="$(trim_whitespace "${raw_spec#*=}")"
    else
      IFS=$' \t' read -r worker_id worker_addr extra <<< "${raw_spec}"
      worker_id="$(trim_whitespace "${worker_id}")"
      worker_addr="$(trim_whitespace "${worker_addr}")"
      extra="$(trim_whitespace "${extra}")"
      if [[ -n "${extra}" ]]; then
        echo "Invalid WORKER_SPECS entry '${raw_spec}'. Use worker_id=host:port." >&2
        return 1
      fi
    fi

    if [[ -z "${worker_id}" || -z "${worker_addr}" ]]; then
      echo "Invalid WORKER_SPECS entry '${raw_spec}'. Use worker_id=host:port." >&2
      return 1
    fi

    WORKERS+=("${worker_id} ${worker_addr}")
  done <<< "${normalized}"

  if [[ "${#WORKERS[@]}" -eq 0 ]]; then
    echo "WORKER_SPECS was provided but no valid worker specs were found." >&2
    return 1
  fi
}

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

    local status
    status=$(
      curl -sS -o /dev/null -w "%{http_code}" \
        -X POST "${MASTER_URL}/api/workers" \
        -H "Content-Type: application/json" \
        -d "${payload}" || true
    )

    if [[ "${status}" == "201" ]]; then
      echo "Registered ${worker_id} (${worker_addr})"
      return 0
    fi

    sleep "${SLEEP_SECONDS}"
  done

  echo "Failed to register ${worker_id} after ${MAX_ATTEMPTS} attempts" >&2
  return 1
}

wait_for_workers_active() {
  local target_ids_csv
  local ids=()
  local spec worker_id worker_addr
  for spec in "${WORKERS[@]}"; do
    IFS=$' \t' read -r worker_id worker_addr <<< "${spec}"
    ids+=("${worker_id}")
  done
  target_ids_csv="$(IFS=,; printf "%s" "${ids[*]}")"

  local attempt
  for attempt in $(seq 1 "${MAX_ATTEMPTS}"); do
    local body
    body="$(curl -fsS "${MASTER_URL}/api/workers" || true)"
    if [[ -z "${body}" ]]; then
      sleep "${SLEEP_SECONDS}"
      continue
    fi

    if TARGET_IDS="${target_ids_csv}" WORKERS_JSON="${body}" python3 - <<'PY'
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
  load_workers
  wait_for_master

  local spec worker_id worker_addr
  for spec in "${WORKERS[@]}"; do
    IFS=$' \t' read -r worker_id worker_addr <<< "${spec}"
    register_worker "${worker_id}" "${worker_addr}"
  done

  wait_for_workers_active
}

main "$@"
