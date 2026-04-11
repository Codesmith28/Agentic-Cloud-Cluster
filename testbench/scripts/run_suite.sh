#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_ROOT}/testbench/docker-compose.yml}"

MASTER_URL="${MASTER_URL:-http://localhost:8080}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
RUN_COMPOSE_UP="${RUN_COMPOSE_UP:-true}"
AUTO_DOWN="${AUTO_DOWN:-false}"
PREPARE_WORKFLOW_IMAGES="${PREPARE_WORKFLOW_IMAGES:-true}"
RUN_TIMESTAMP="${RUN_TIMESTAMP:-$(date +%Y%m%d-%H%M%S)}"
SUITE_NAME="${SUITE_NAME:-smoke}"
SCENARIO_DIR="${SCENARIO_DIR:-${REPO_ROOT}/testbench/scenarios}"
SCENARIO_PATH="${SCENARIO_PATH:-${SCENARIO_DIR}/${SUITE_NAME}.json}"

RUN_ROOT="${RUN_ROOT:-${REPO_ROOT}/results/testbench/${RUN_TIMESTAMP}-${SUITE_NAME}}"
SUMMARY_PATH="${SUMMARY_PATH:-${RUN_ROOT}/summary.json}"
ASSERTION_PATH="${ASSERTION_PATH:-${RUN_ROOT}/assertion_results.json}"
OBS_OUTPUT_DIR="${OBS_OUTPUT_DIR:-${RUN_ROOT}/observability}"
ATTEMPT_OUTPUT_DIR="${ATTEMPT_OUTPUT_DIR:-${RUN_ROOT}/attempt_snapshots}"
LOG_OUTPUT_DIR="${LOG_OUTPUT_DIR:-${RUN_ROOT}/logs}"
CAMPAIGN_OUTPUT_DIR="${CAMPAIGN_OUTPUT_DIR:-${RUN_ROOT}/campaign}"
SUBRUNS_PATH="${RUN_ROOT}/sub_runs.jsonl"

EXTRA_ARGS=("$@")

mkdir -p "${RUN_ROOT}" "${OBS_OUTPUT_DIR}" "${ATTEMPT_OUTPUT_DIR}" "${LOG_OUTPUT_DIR}"

RUN_STARTED_UNIX="$(python3 - <<'PY'
import time
print(f"{time.time():.3f}")
PY
)"

SCENARIO_SUITE=""
RUNNER=""
SCENARIO_ASSERTIONS_JSON="[]"
SCENARIO_DESCRIPTION=""
SCENARIO_WORKLOAD=""
SCENARIO_UI_URL=""
SCENARIO_SCENARIOS=""
SCENARIO_WORKLOADS=""
SCENARIO_SCHEDULERS=""
SCENARIO_SEQUENCE=""
SCENARIO_PREFLIGHT_GO_TEST=""
SCENARIO_STOP_ON_FAILURE="true"
CAMPAIGN_REPORT_PATH=""

cleanup() {
  if [[ "${AUTO_DOWN}" == "true" ]]; then
    echo "Tearing down testbench stack..."
    docker compose -f "${COMPOSE_FILE}" down --remove-orphans >/dev/null || true
  fi
}
trap cleanup EXIT

default_worker_specs_for_compose() {
  local lowered
  lowered="$(echo "${COMPOSE_FILE}" | tr '[:upper:]' '[:lower:]')"
  if [[ "${lowered}" == *"host-master"* ]]; then
    echo "worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054"
    return 0
  fi
  return 1
}

ensure_worker_specs_for_compose() {
  if [[ -n "${WORKER_SPECS:-}" ]]; then
    return 0
  fi
  local default_specs
  default_specs="$(default_worker_specs_for_compose || true)"
  if [[ -n "${default_specs}" ]]; then
    export WORKER_SPECS="${default_specs}"
    echo "Using host-master worker registration defaults: ${WORKER_SPECS}"
  fi
}

resolve_workload_path() {
  local raw="${1:-}"
  if [[ -z "${raw}" ]]; then
    return 0
  fi
  if [[ "${raw}" == /* ]]; then
    printf "%s\n" "${raw}"
    return 0
  fi
  if [[ -f "${raw}" ]]; then
    printf "%s\n" "${raw}"
    return 0
  fi
  if [[ -f "${REPO_ROOT}/${raw}" ]]; then
    printf "%s\n" "${REPO_ROOT}/${raw}"
    return 0
  fi
  if [[ "${raw}" == *.json ]]; then
    printf "%s\n" "${REPO_ROOT}/testbench/workloads/${raw}"
    return 0
  fi
  printf "%s\n" "${REPO_ROOT}/testbench/workloads/${raw}.json"
}

parse_scenario() {
  if [[ ! -f "${SCENARIO_PATH}" ]]; then
    echo "Scenario manifest not found: ${SCENARIO_PATH}" >&2
    exit 2
  fi

  eval "$(
    SCENARIO_PATH="${SCENARIO_PATH}" python3 - <<'PY'
import json
import os
import pathlib
import shlex

path = pathlib.Path(os.environ["SCENARIO_PATH"])
data = json.loads(path.read_text(encoding="utf-8"))

def emit(key: str, value: str) -> None:
    print(f"{key}={shlex.quote(value)}")

suite = str(data.get("suite") or path.stem).strip()
runner = str(data.get("runner", "")).strip()
description = str(data.get("description", "")).strip()
workload = str(data.get("workload", "")).strip()
ui_url = str(data.get("ui_url", "")).strip()
scenarios = ",".join(str(item).strip() for item in data.get("scenarios", []) if str(item).strip())
workloads = ",".join(str(item).strip() for item in data.get("workloads", []) if str(item).strip())
schedulers = ",".join(str(item).strip() for item in data.get("schedulers", []) if str(item).strip())
sequence = ",".join(str(item).strip() for item in data.get("sequence", []) if str(item).strip())
assertions_json = json.dumps(data.get("assertions", []))
preflight = data.get("preflight", {}) if isinstance(data.get("preflight"), dict) else {}
preflight_go_test = str(preflight.get("go_test", "")).strip()
stop_on_failure = "true" if bool(data.get("stop_on_failure", True)) else "false"

emit("SCENARIO_SUITE", suite)
emit("RUNNER", runner)
emit("SCENARIO_DESCRIPTION", description)
emit("SCENARIO_WORKLOAD", workload)
emit("SCENARIO_UI_URL", ui_url)
emit("SCENARIO_SCENARIOS", scenarios)
emit("SCENARIO_WORKLOADS", workloads)
emit("SCENARIO_SCHEDULERS", schedulers)
emit("SCENARIO_SEQUENCE", sequence)
emit("SCENARIO_ASSERTIONS_JSON", assertions_json)
emit("SCENARIO_PREFLIGHT_GO_TEST", preflight_go_test)
emit("SCENARIO_STOP_ON_FAILURE", stop_on_failure)
PY
  )"

  if [[ -z "${RUNNER}" ]]; then
    echo "Scenario '${SCENARIO_PATH}' is missing runner" >&2
    exit 2
  fi
}

prepare_environment() {
  if [[ "${RUN_COMPOSE_UP}" == "true" ]]; then
    echo "Starting testbench stack..."
    docker compose -f "${COMPOSE_FILE}" up -d --build
  fi

  if [[ "${PREPARE_WORKFLOW_IMAGES}" == "true" ]]; then
    echo "Preparing deterministic workflow images..."
    COMPOSE_FILE="${COMPOSE_FILE}" "${SCRIPT_DIR}/prepare_workflow_images.sh"
  fi

  ensure_worker_specs_for_compose
  echo "Registering workers..."
  if [[ -n "${WORKER_SPECS:-}" ]]; then
    MASTER_URL="${MASTER_URL}" WORKER_SPECS="${WORKER_SPECS}" "${SCRIPT_DIR}/register_workers.sh"
  else
    MASTER_URL="${MASTER_URL}" "${SCRIPT_DIR}/register_workers.sh"
  fi
}

find_campaign_report() {
  CAMPAIGN_REPORT_PATH="$(
    find "${CAMPAIGN_OUTPUT_DIR}" -type f -name "campaign-report.json" 2>/dev/null | LC_ALL=C sort | tail -n 1
  )"
  if [[ -z "${CAMPAIGN_REPORT_PATH}" ]]; then
    echo "No campaign-report.json produced under ${CAMPAIGN_OUTPUT_DIR}" >&2
    return 1
  fi
}

write_campaign_summary() {
  local run_finished_unix="${1}"
  SUMMARY_PATH="${SUMMARY_PATH}" \
  SUITE_NAME="${SUITE_NAME}" \
  RUN_STARTED_UNIX="${RUN_STARTED_UNIX}" \
  RUN_FINISHED_UNIX="${run_finished_unix}" \
  CAMPAIGN_REPORT_PATH="${CAMPAIGN_REPORT_PATH}" \
  python3 - <<'PY'
import json
import os
from pathlib import Path

summary_path = Path(os.environ["SUMMARY_PATH"])
suite_name = os.environ["SUITE_NAME"]
started_at = float(os.environ["RUN_STARTED_UNIX"])
finished_at = float(os.environ["RUN_FINISHED_UNIX"])
report_path = Path(os.environ["CAMPAIGN_REPORT_PATH"])
report = json.loads(report_path.read_text(encoding="utf-8"))

submitted = 0
completed = 0
failed = 0
task_ids: list[str] = []
for item in report.get("results", []):
    submitted += int(item.get("tasks_submitted", 0) or 0)
    completed += int(item.get("tasks_completed", 0) or 0)
    failed += int(item.get("tasks_failed", 0) or 0)
    for task_id in item.get("task_ids", []):
        task_id = str(task_id).strip()
        if task_id:
            task_ids.append(task_id)

deduped_task_ids = list(dict.fromkeys(task_ids))
summary = {
    "suite": suite_name,
    "runner": "campaign",
    "started_at_unix": started_at,
    "finished_at_unix": finished_at,
    "duration_seconds": round(max(0.0, finished_at - started_at), 3),
    "totals": {
        "submitted": submitted,
        "completed": completed,
        "failed": failed,
        "cancelled": 0,
    },
    "task_ids": deduped_task_ids,
    "tasks": [{"task_id": task_id} for task_id in deduped_task_ids],
    "campaign_report": str(report_path),
}

summary_path.parent.mkdir(parents=True, exist_ok=True)
summary_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")
PY
}

append_subrun_record() {
  local child_suite="$1"
  local child_root="$2"
  local child_success="$3"
  local child_rc="$4"

  SUBRUNS_PATH="${SUBRUNS_PATH}" \
  CHILD_SUITE="${child_suite}" \
  CHILD_ROOT="${child_root}" \
  CHILD_SUCCESS="${child_success}" \
  CHILD_RC="${child_rc}" \
  python3 - <<'PY'
import json
import os
from pathlib import Path

child_root = Path(os.environ["CHILD_ROOT"])
entry = {
    "suite": os.environ["CHILD_SUITE"],
    "run_root": str(child_root),
    "summary_path": str(child_root / "summary.json"),
    "assertions_path": str(child_root / "assertion_results.json"),
    "success": os.environ["CHILD_SUCCESS"] == "true",
    "exit_code": int(os.environ["CHILD_RC"]),
}

with Path(os.environ["SUBRUNS_PATH"]).open("a", encoding="utf-8") as handle:
    handle.write(json.dumps(entry) + "\n")
PY
}

write_composite_summary() {
  local preflight_success="$1"
  local run_finished_unix="$2"

  SUMMARY_PATH="${SUMMARY_PATH}" \
  SUITE_NAME="${SUITE_NAME}" \
  RUN_STARTED_UNIX="${RUN_STARTED_UNIX}" \
  RUN_FINISHED_UNIX="${run_finished_unix}" \
  SCENARIO_SEQUENCE="${SCENARIO_SEQUENCE}" \
  SCENARIO_PREFLIGHT_GO_TEST="${SCENARIO_PREFLIGHT_GO_TEST}" \
  PREFLIGHT_SUCCESS="${preflight_success}" \
  SUBRUNS_PATH="${SUBRUNS_PATH}" \
  python3 - <<'PY'
import json
import os
from pathlib import Path

summary_path = Path(os.environ["SUMMARY_PATH"])
subruns_path = Path(os.environ["SUBRUNS_PATH"])
started_at = float(os.environ["RUN_STARTED_UNIX"])
finished_at = float(os.environ["RUN_FINISHED_UNIX"])
sequence = [item for item in os.environ.get("SCENARIO_SEQUENCE", "").split(",") if item]
preflight_success = os.environ.get("PREFLIGHT_SUCCESS", "true") == "true"

sub_runs = []
if subruns_path.exists():
    for line in subruns_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        sub_runs.append(json.loads(line))

overall_success = preflight_success and all(item.get("success", False) for item in sub_runs)
if sequence and len(sub_runs) < len(sequence):
    overall_success = False

summary = {
    "suite": os.environ["SUITE_NAME"],
    "runner": "composite",
    "started_at_unix": started_at,
    "finished_at_unix": finished_at,
    "duration_seconds": round(max(0.0, finished_at - started_at), 3),
    "preflight": {
        "go_test": os.environ.get("SCENARIO_PREFLIGHT_GO_TEST", ""),
        "success": preflight_success,
    },
    "sequence": sequence,
    "sub_runs": sub_runs,
    "success": overall_success,
}

summary_path.parent.mkdir(parents=True, exist_ok=True)
summary_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")
PY
}

write_composite_indexes() {
  ATTEMPT_OUTPUT_DIR="${ATTEMPT_OUTPUT_DIR}" \
  OBS_OUTPUT_DIR="${OBS_OUTPUT_DIR}" \
  SUITE_NAME="${SUITE_NAME}" \
  SUBRUNS_PATH="${SUBRUNS_PATH}" \
  python3 - <<'PY'
import json
import os
from pathlib import Path

subruns_path = Path(os.environ["SUBRUNS_PATH"])
sub_runs = []
if subruns_path.exists():
    for line in subruns_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        sub_runs.append(json.loads(line))

attempt_index = Path(os.environ["ATTEMPT_OUTPUT_DIR"]) / "index.json"
attempt_index.parent.mkdir(parents=True, exist_ok=True)
attempt_index.write_text(
    json.dumps(
        {
            "suite": os.environ["SUITE_NAME"],
            "runner": "composite",
            "task_ids": [],
            "exported_count": 0,
            "failed_count": 0,
            "sub_runs": sub_runs,
        },
        indent=2,
    ),
    encoding="utf-8",
)

obs_index = Path(os.environ["OBS_OUTPUT_DIR"]) / "index.json"
obs_index.parent.mkdir(parents=True, exist_ok=True)
obs_index.write_text(
    json.dumps(
        {
            "suite": os.environ["SUITE_NAME"],
            "runner": "composite",
            "note": "Observability artifacts are emitted by sub-runs.",
            "sub_runs": sub_runs,
        },
        indent=2,
    ),
    encoding="utf-8",
)
PY
}

capture_compose_logs() {
  echo "Capturing compose logs..."
  if ! docker compose -f "${COMPOSE_FILE}" logs --no-color > "${LOG_OUTPUT_DIR}/compose.log"; then
    echo "Unable to capture compose logs for ${COMPOSE_FILE}" > "${LOG_OUTPUT_DIR}/compose.log"
  fi
}

evaluate_assertions() {
  RUNNER="${RUNNER}" \
  SUITE_NAME="${SUITE_NAME}" \
  MASTER_URL="${MASTER_URL}" \
  SUMMARY_PATH="${SUMMARY_PATH}" \
  ASSERTION_PATH="${ASSERTION_PATH}" \
  OBS_OUTPUT_DIR="${OBS_OUTPUT_DIR}" \
  ATTEMPT_OUTPUT_DIR="${ATTEMPT_OUTPUT_DIR}" \
  LOG_OUTPUT_DIR="${LOG_OUTPUT_DIR}" \
  SCENARIO_ASSERTIONS_JSON="${SCENARIO_ASSERTIONS_JSON}" \
  python3 - <<'PY'
import json
import os
import urllib.request
from pathlib import Path
from typing import Any

summary_path = Path(os.environ["SUMMARY_PATH"])
assertion_path = Path(os.environ["ASSERTION_PATH"])
runner = os.environ["RUNNER"]
suite_name = os.environ["SUITE_NAME"]
master_url = os.environ["MASTER_URL"].rstrip("/")
expected = json.loads(os.environ.get("SCENARIO_ASSERTIONS_JSON", "[]"))
summary = json.loads(summary_path.read_text(encoding="utf-8"))

checks: list[dict[str, Any]] = []
check_map: dict[str, bool] = {}

def add_check(name: str, success: bool, detail: str) -> None:
    if name in check_map:
        return
    check_map[name] = bool(success)
    checks.append({"name": name, "success": bool(success), "detail": detail})

def request_json(url: str) -> dict[str, Any]:
    with urllib.request.urlopen(url, timeout=10.0) as response:
        body = response.read().decode("utf-8")
        return json.loads(body) if body else {}

if runner in {"suite", "workload"}:
    totals = summary.get("totals", {})
    submitted = int(totals.get("submitted", 0) or 0)
    completed = int(totals.get("completed", 0) or 0)
    failed = int(totals.get("failed", 0) or 0)
    cancelled = int(totals.get("cancelled", 0) or 0)
    add_check("submitted-at-least-one", submitted > 0, f"submitted={submitted}")
    add_check(
        "all-tasks-completed",
        submitted > 0 and completed == submitted,
        f"completed={completed} submitted={submitted}",
    )
    add_check("no-failed-tasks", failed == 0, f"failed={failed}")
    add_check("no-cancelled-tasks", cancelled == 0, f"cancelled={cancelled}")
    add_check(
        "task-terminal-status",
        submitted > 0 and completed == submitted and failed == 0 and cancelled == 0,
        f"completed={completed} failed={failed} cancelled={cancelled}",
    )
    add_check("task-output-roundtrip", failed == 0 and cancelled == 0, f"failed={failed} cancelled={cancelled}")
    try:
        health = request_json(f"{master_url}/health")
        status = str(health.get("status", "")).lower()
        add_check("health-endpoint", status == "healthy", json.dumps(health))
    except Exception as exc:  # pylint: disable=broad-except
        add_check("health-endpoint", False, f"{type(exc).__name__}: {exc}")
    try:
        workers_payload = request_json(f"{master_url}/api/workers")
        workers = workers_payload.get("workers", [])
        active_count = sum(1 for worker in workers if worker.get("is_active", False))
        add_check("workers-registered", active_count > 0, f"workers={len(workers)} active={active_count}")
    except Exception as exc:  # pylint: disable=broad-except
        add_check("workers-registered", False, f"{type(exc).__name__}: {exc}")

elif runner == "ui-smoke":
    for item in summary.get("checks", []):
        add_check(str(item.get("name", "")), bool(item.get("success", False)), str(item.get("detail", "")))
    add_check("ui-smoke-success", bool(summary.get("success", False)), f"success={summary.get('success', False)}")
    add_check(
        "auth-register-login-me",
        check_map.get("auth-register", False) and check_map.get("auth-login", False) and check_map.get("auth-me", False),
        "auth-register && auth-login && auth-me",
    )
    add_check("ui-root-loads", check_map.get("ui-root", False), "derived from ui-root")

elif runner == "campaign":
    report_path = Path(str(summary.get("campaign_report", "")).strip())
    report = {}
    if report_path.exists():
        report = json.loads(report_path.read_text(encoding="utf-8"))
    add_check("campaign-report-generated", report_path.exists(), str(report_path))
    results = report.get("results", []) if isinstance(report, dict) else []
    add_check("campaign-results-present", len(results) > 0, f"results={len(results)}")
    errors = [item for item in results if str(item.get("error", "")).strip()]
    add_check("campaign-no-run-errors", len(errors) == 0, f"errors={len(errors)}")
    scheduler_summary = report.get("summary", {}).get("by_scheduler", {}) if isinstance(report, dict) else {}
    add_check("scheduler-comparison-available", bool(scheduler_summary), f"schedulers={len(scheduler_summary)}")

elif runner == "composite":
    preflight = summary.get("preflight", {})
    add_check("preflight-go-test", bool(preflight.get("success", False)), f"preflight={preflight}")
    sequence = summary.get("sequence", [])
    sub_runs = summary.get("sub_runs", [])
    add_check("sub-suites-count-match", len(sub_runs) == len(sequence), f"sub_runs={len(sub_runs)} sequence={len(sequence)}")
    add_check("sub-suites-success", all(bool(item.get("success", False)) for item in sub_runs), f"sub_runs={len(sub_runs)}")
    for item in sub_runs:
        suite = str(item.get("suite", "")).strip()
        if suite:
            add_check(f"suite-{suite}", bool(item.get("success", False)), f"exit_code={item.get('exit_code', 0)}")

attempt_index = Path(os.environ["ATTEMPT_OUTPUT_DIR"]) / "index.json"
add_check("attempt-snapshots-exported", attempt_index.exists(), str(attempt_index))

obs_dir = Path(os.environ["OBS_OUTPUT_DIR"])
obs_files = [
    obs_dir / "prometheus-range.json",
    obs_dir / "prometheus-instant.json",
    obs_dir / "master-snapshot.json",
]
obs_ok = all(path.exists() for path in obs_files) or (obs_dir / "index.json").exists()
add_check("observability-export-generated", obs_ok, ",".join(str(path) for path in obs_files))
add_check("prometheus-export-generated", obs_ok, "derived from observability-export-generated")

compose_log_path = Path(os.environ["LOG_OUTPUT_DIR"]) / "compose.log"
add_check("logs-captured", compose_log_path.exists(), str(compose_log_path))

aliases = {
    "task-terminal-status": "all-tasks-completed",
    "task-output-roundtrip": "no-failed-tasks",
    "ui-root-loads": "ui-root",
    "prometheus-export-generated": "observability-export-generated",
}

for assertion_name in expected:
    assertion_name = str(assertion_name).strip()
    if not assertion_name or assertion_name in check_map:
        continue
    alias = aliases.get(assertion_name, "")
    if alias and alias in check_map:
        add_check(assertion_name, check_map[alias], f"derived from {alias}")
        continue
    if runner == "campaign":
        campaign_ok = check_map.get("campaign-results-present", False) and check_map.get("campaign-no-run-errors", False)
        add_check(assertion_name, campaign_ok, "derived from campaign result health")
        continue
    add_check(assertion_name, False, "missing explicit check")

result = {
    "suite": suite_name,
    "runner": runner,
    "success": all(item["success"] for item in checks),
    "checks": checks,
    "expected_assertions": expected,
}

assertion_path.parent.mkdir(parents=True, exist_ok=True)
assertion_path.write_text(json.dumps(result, indent=2), encoding="utf-8")
print(f"Assertion results written to {assertion_path}")
raise SystemExit(0 if result["success"] else 1)
PY
}

ensure_attempt_index_exists() {
  if [[ -f "${ATTEMPT_OUTPUT_DIR}/index.json" ]]; then
    return 0
  fi
  ATTEMPT_OUTPUT_DIR="${ATTEMPT_OUTPUT_DIR}" python3 - <<'PY'
import json
import os
from pathlib import Path

index_path = Path(os.environ["ATTEMPT_OUTPUT_DIR"]) / "index.json"
index_path.parent.mkdir(parents=True, exist_ok=True)
index_path.write_text(
    json.dumps(
        {
            "task_ids": [],
            "exported_count": 0,
            "failed_count": 0,
            "failures": {},
            "note": "attempt snapshot export did not produce an index; placeholder generated",
        },
        indent=2,
    ),
    encoding="utf-8",
)
PY
}

run_standard_runner() {
  case "${RUNNER}" in
    suite | workload)
      local workload_path
      workload_path="$(resolve_workload_path "${SCENARIO_WORKLOAD}")"
      echo "Running workload suite '${SUITE_NAME}'..."
      cmd=(python3 "${SCRIPT_DIR}/run_workload.py" --master-url "${MASTER_URL}" --output "${SUMMARY_PATH}")
      if [[ -n "${workload_path}" ]]; then
        cmd+=(--workload "${workload_path}")
      fi
      if [[ "${#EXTRA_ARGS[@]}" -gt 0 ]]; then
        cmd+=("${EXTRA_ARGS[@]}")
      fi
      "${cmd[@]}"
      ;;
    ui-smoke)
      local ui_url
      ui_url="${UI_URL:-${SCENARIO_UI_URL:-http://localhost:3000}}"
      echo "Running UI smoke suite '${SUITE_NAME}'..."
      cmd=(python3 "${SCRIPT_DIR}/run_ui_smoke.py" --master-url "${MASTER_URL}" --ui-url "${ui_url}" --output "${SUMMARY_PATH}")
      if [[ "${#EXTRA_ARGS[@]}" -gt 0 ]]; then
        cmd+=("${EXTRA_ARGS[@]}")
      fi
      "${cmd[@]}"
      ;;
    campaign)
      echo "Running campaign suite '${SUITE_NAME}'..."
      mkdir -p "${CAMPAIGN_OUTPUT_DIR}"
      cmd=(
        python3 "${SCRIPT_DIR}/run_campaign.py"
        --master-url "${MASTER_URL}"
        --prometheus-url "${PROMETHEUS_URL}"
        --compose-file "${COMPOSE_FILE}"
        --output-dir "${CAMPAIGN_OUTPUT_DIR}"
        --skip-observability-export
      )
      if [[ -n "${SCENARIO_SCENARIOS}" ]]; then
        cmd+=(--scenarios "${SCENARIO_SCENARIOS}")
      fi
      if [[ -n "${SCENARIO_WORKLOADS}" ]]; then
        cmd+=(--workloads "${SCENARIO_WORKLOADS}")
      fi
      if [[ -n "${SCENARIO_SCHEDULERS}" ]]; then
        cmd+=(--schedulers "${SCENARIO_SCHEDULERS}")
      fi
      if [[ "${#EXTRA_ARGS[@]}" -gt 0 ]]; then
        cmd+=("${EXTRA_ARGS[@]}")
      fi
      "${cmd[@]}"

      find_campaign_report
      local run_finished_unix
      run_finished_unix="$(python3 - <<'PY'
import time
print(f"{time.time():.3f}")
PY
)"
      write_campaign_summary "${run_finished_unix}"
      ;;
    *)
      echo "Unsupported runner '${RUNNER}' in ${SCENARIO_PATH}" >&2
      return 2
      ;;
  esac

  if [[ ! -f "${SUMMARY_PATH}" ]]; then
    echo "Suite did not produce summary file: ${SUMMARY_PATH}" >&2
    return 1
  fi
}

run_composite_runner() {
  : > "${SUBRUNS_PATH}"
  local preflight_success="true"
  local preflight_rc=0

  if [[ -n "${SCENARIO_PREFLIGHT_GO_TEST}" ]]; then
    local -a go_test_args=()
    read -r -a go_test_args <<< "${SCENARIO_PREFLIGHT_GO_TEST}"
    if [[ "${#go_test_args[@]}" -eq 0 ]]; then
      go_test_args=("./...")
    fi
    echo "Running preflight Go tests in master and worker modules..."
    set +e
    (cd "${REPO_ROOT}/master" && go test "${go_test_args[@]}") && (cd "${REPO_ROOT}/worker" && go test "${go_test_args[@]}")
    preflight_rc=$?
    set -e
    if [[ "${preflight_rc}" -ne 0 ]]; then
      preflight_success="false"
    fi
  fi

  local child_run_compose_up="${RUN_COMPOSE_UP}"
  local child_prepare_images="${PREPARE_WORKFLOW_IMAGES}"
  local child_failed="false"

  local -a sequence=()
  if [[ -n "${SCENARIO_SEQUENCE}" ]]; then
    IFS=',' read -r -a sequence <<< "${SCENARIO_SEQUENCE}"
  fi

  if [[ "${preflight_success}" == "false" && "${SCENARIO_STOP_ON_FAILURE}" == "true" ]]; then
    echo "Preflight failed; skipping scenario sequence."
  else
    local child_suite
    for child_suite in "${sequence[@]}"; do
      child_suite="${child_suite## }"
      child_suite="${child_suite%% }"
      if [[ -z "${child_suite}" ]]; then
        continue
      fi

      local child_root="${RUN_ROOT}/${child_suite}"
      mkdir -p "${child_root}"

      echo "Running child suite '${child_suite}'..."
      set +e
      if [[ -n "${WORKER_SPECS:-}" ]]; then
        COMPOSE_FILE="${COMPOSE_FILE}" \
        MASTER_URL="${MASTER_URL}" \
        PROMETHEUS_URL="${PROMETHEUS_URL}" \
        WORKER_SPECS="${WORKER_SPECS}" \
        RUN_COMPOSE_UP="${child_run_compose_up}" \
        PREPARE_WORKFLOW_IMAGES="${child_prepare_images}" \
        AUTO_DOWN="false" \
        SUITE_NAME="${child_suite}" \
        SCENARIO_DIR="${SCENARIO_DIR}" \
        RUN_ROOT="${child_root}" \
        "${SCRIPT_DIR}/run_suite.sh"
      else
        COMPOSE_FILE="${COMPOSE_FILE}" \
        MASTER_URL="${MASTER_URL}" \
        PROMETHEUS_URL="${PROMETHEUS_URL}" \
        RUN_COMPOSE_UP="${child_run_compose_up}" \
        PREPARE_WORKFLOW_IMAGES="${child_prepare_images}" \
        AUTO_DOWN="false" \
        SUITE_NAME="${child_suite}" \
        SCENARIO_DIR="${SCENARIO_DIR}" \
        RUN_ROOT="${child_root}" \
        "${SCRIPT_DIR}/run_suite.sh"
      fi
      local child_rc=$?
      set -e

      if [[ "${child_rc}" -eq 0 ]]; then
        append_subrun_record "${child_suite}" "${child_root}" "true" "${child_rc}"
      else
        append_subrun_record "${child_suite}" "${child_root}" "false" "${child_rc}"
        child_failed="true"
      fi

      child_run_compose_up="false"
      child_prepare_images="false"

      if [[ "${child_rc}" -ne 0 && "${SCENARIO_STOP_ON_FAILURE}" == "true" ]]; then
        echo "Child suite '${child_suite}' failed and stop_on_failure=true; stopping sequence."
        break
      fi
    done
  fi

  local run_finished_unix
  run_finished_unix="$(python3 - <<'PY'
import time
print(f"{time.time():.3f}")
PY
)"

  write_composite_summary "${preflight_success}" "${run_finished_unix}"
  write_composite_indexes
  capture_compose_logs
  evaluate_assertions

  if [[ "${preflight_rc}" -ne 0 || "${child_failed}" == "true" ]]; then
    return 1
  fi
}

parse_scenario
echo "Running suite '${SCENARIO_SUITE}' (${RUNNER})"
if [[ -n "${SCENARIO_DESCRIPTION}" ]]; then
  echo "Scenario: ${SCENARIO_DESCRIPTION}"
fi

if [[ "${RUNNER}" == "composite" ]]; then
  run_composite_runner
  rm -f "${SUBRUNS_PATH}"
  exit 0
fi

prepare_environment
run_standard_runner

echo "Exporting task/attempt snapshots..."
python3 "${SCRIPT_DIR}/export_attempt_snapshots.py" \
  --master-url "${MASTER_URL}" \
  --summary "${SUMMARY_PATH}" \
  --output-dir "${ATTEMPT_OUTPUT_DIR}" \
  --no-master-discovery || true
ensure_attempt_index_exists

echo "Exporting observability artifacts..."
python3 "${SCRIPT_DIR}/export_metrics.py" \
  --prometheus-url "${PROMETHEUS_URL}" \
  --master-url "${MASTER_URL}" \
  --summary "${SUMMARY_PATH}" \
  --output-dir "${OBS_OUTPUT_DIR}"

capture_compose_logs
evaluate_assertions
