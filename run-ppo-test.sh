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

# =============================================================================
# run-ppo-test.sh — Clean-slate PPO benchmark with resource-contention workload
#
# Usage:
#   ./run-ppo-test.sh [--scenarios <list>] [--workloads <list>] [--fast]
#                     [--skip-cleanup] [--skip-images] [--master-url <url>]
#
# Examples:
#   ./run-ppo-test.sh                          # Full test (all scenarios)
#   ./run-ppo-test.sh --fast                   # Baseline only (faster)
#   ./run-ppo-test.sh --skip-cleanup           # Skip teardown/DB wipe
#   ./run-ppo-test.sh --workloads resource-contention-ppo,heterogeneous-smoke
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"

# ── Defaults ──────────────────────────────────────────────────────────────────
MASTER_URL="${MASTER_URL:-http://localhost:8080}"
COMPOSE_HOST="${REPO_ROOT}/testbench/docker-compose.host-master.yml"
PYTHON="${REPO_ROOT}/venv/bin/python3"
SCENARIOS="baseline,burst,overload"
WORKLOADS="resource-contention-ppo"
SCHEDULERS="RR,RTS,PPO"
OUTPUT_DIR="${REPO_ROOT}/results/campaign/ppo-new-model-test"
STARTUP_WAIT=45
SKIP_CLEANUP=false
SKIP_IMAGES=false

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

log()  { echo -e "${CYAN}[ppo-test]${RESET} $*"; }
ok()   { echo -e "${GREEN}[ppo-test] ✓${RESET} $*"; }
warn() { echo -e "${YELLOW}[ppo-test] ⚠${RESET} $*"; }
die()  { echo -e "${RED}[ppo-test] ✗ $*${RESET}" >&2; exit 1; }

# ── Arg parsing ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --scenarios)    SCENARIOS="$2";    shift 2 ;;
    --workloads)    WORKLOADS="$2";    shift 2 ;;
    --schedulers)   SCHEDULERS="$2";   shift 2 ;;
    --master-url)   MASTER_URL="$2";   shift 2 ;;
    --output-dir)   OUTPUT_DIR="$2";   shift 2 ;;
    --fast)         SCENARIOS="baseline"; shift ;;
    --skip-cleanup) SKIP_CLEANUP=true; shift ;;
    --skip-images)  SKIP_IMAGES=true;  shift ;;
    --wait)         STARTUP_WAIT="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,12p' "$0" | sed 's/^# //'
      exit 0
      ;;
    *) die "Unknown argument: $1" ;;
  esac
done

# Timestamp output dir so multiple runs don't collide
TS="$(date +%Y%m%d-%H%M%S)"
OUTPUT_DIR="${OUTPUT_DIR}/${TS}"

# ── Pre-flight ────────────────────────────────────────────────────────────────
[[ -f "${PYTHON}" ]] || PYTHON="$(command -v python3)" || die "Python3 not found"
[[ -f "${COMPOSE_HOST}" ]] || die "Compose file not found: ${COMPOSE_HOST}"

echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}║         PPO Clean-Slate Benchmark (New Model + Workload)      ║${RESET}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════╝${RESET}"
echo ""
log "Scenarios : ${SCENARIOS}"
log "Workloads : ${WORKLOADS}"
log "Schedulers: ${SCHEDULERS}"
log "Output    : ${OUTPUT_DIR}"
log "Python    : ${PYTHON}"
echo ""

# ── Cleanup trap ──────────────────────────────────────────────────────────────
MASTER_BG_PID=""

cleanup_test() {
  echo ""
  log "Cleaning up test run..."
  if [[ -n "${MASTER_BG_PID:-}" ]]; then
    kill "${MASTER_BG_PID}" 2>/dev/null || true
    wait "${MASTER_BG_PID}" 2>/dev/null || true
    ok "Headless master stopped"
  fi
  # Free the PPO gRPC port in case it outlived master
  _PPO_PID="$(lsof -ti :50050 -sTCP:LISTEN 2>/dev/null || true)"
  [[ -n "$_PPO_PID" ]] && kill -9 "$_PPO_PID" 2>/dev/null || true
}
trap cleanup_test EXIT INT TERM

# ── Phase 1: Clean Slate ──────────────────────────────────────────────────────
echo -e "${BOLD}━━━  Phase 1: Clean Slate  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"

if [[ "${SKIP_CLEANUP}" == "false" ]]; then
  log "Stopping any existing Docker stack..."
  docker compose -f "${COMPOSE_HOST}" down --remove-orphans 2>/dev/null || true

  log "Removing testbench MongoDB data volume for a fresh database..."
  docker volume rm testbench_mongo-data 2>/dev/null && ok "Volume removed" || warn "Volume not found (already clean)"

  # Kill any leftover host master / PPO process
  pgrep -f "masterNode" 2>/dev/null | xargs kill 2>/dev/null || true
  pgrep -f "agentic_scheduler.server" 2>/dev/null | xargs kill 2>/dev/null || true
  sleep 2
  ok "Clean slate ready"
else
  warn "--skip-cleanup: skipping teardown and DB wipe"
fi

# Verify new model exists
MODEL_PATH="${REPO_ROOT}/agentic_scheduler/models/ppo_latest.pt"
if [[ -f "${MODEL_PATH}" ]]; then
  MODEL_SIZE=$(du -h "${MODEL_PATH}" | cut -f1)
  ok "New PPO model found: ${MODEL_PATH} (${MODEL_SIZE})"
else
  die "ppo_latest.pt not found at ${MODEL_PATH} — run the retraining step first"
fi

# Verify new workload exists
for wl in $(echo "${WORKLOADS}" | tr ',' ' '); do
  WL_FILE="${REPO_ROOT}/testbench/workloads/${wl}.json"
  if [[ -f "${WL_FILE}" ]]; then
    TASK_COUNT=$(${PYTHON} -c "import json; d=json.load(open('${WL_FILE}')); print(len(d.get('tasks', [])))")
    ok "Workload: ${wl}.json (${TASK_COUNT} tasks)"
  else
    die "Workload file not found: ${WL_FILE}"
  fi
done

echo ""

# ── Phase 2: Start Infrastructure ─────────────────────────────────────────────
echo -e "${BOLD}━━━  Phase 2: Start Infrastructure  ━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
log "Bringing up worker Docker stack (MongoDB on :27018, workers on :55052-55054)..."
docker compose -f "${COMPOSE_HOST}" up -d

log "Waiting ${STARTUP_WAIT}s for containers to initialise..."
for i in $(seq 1 "${STARTUP_WAIT}"); do
  printf "\r  %d/%d..." "${i}" "${STARTUP_WAIT}"
  sleep 1
done
echo ""

# ── Start master in headless mode (owns no stdin, runs until killed) ──────────
log "Building master binary..."
make master -C "${REPO_ROOT}" >/dev/null 2>&1 || { make master -C "${REPO_ROOT}"; }
ok "Master built"

log "Starting master in headless mode (CLOUDAI_HEADLESS=true, PPO enabled)..."
CLOUDAI_HEADLESS=true \
SCHED_ALGO=PPO \
PPO_AUTOSTART=true \
PPO_MODEL_PATH=latest \
PPO_DEPLOYMENT_MODE=active \
PPO_ONLINE_UPDATES_ENABLED=true \
MONGODB_URI="mongodb://cloudai:cloudai-stress-test@localhost:27018/cluster_db?authSource=admin" \
  "${REPO_ROOT}/master/masterNode" --mode headless \
  >> "${REPO_ROOT}/run-ppo-test-master.log" 2>&1 &
MASTER_BG_PID=$!
ok "Master started (PID=${MASTER_BG_PID}), log: run-ppo-test-master.log"

# Wait until master health endpoint responds
log "Waiting for master HTTP :8080..."
HEALTH_ATTEMPTS=0
until curl -sf "${MASTER_URL}/health" >/dev/null 2>&1; do
  HEALTH_ATTEMPTS=$((HEALTH_ATTEMPTS + 1))
  if [[ ${HEALTH_ATTEMPTS} -ge 40 ]]; then
    echo ""
    warn "Master did not start in time. Last 20 log lines:"
    tail -20 "${REPO_ROOT}/run-ppo-test-master.log" 2>/dev/null || true
    die "Master not responding at ${MASTER_URL}/health after ${HEALTH_ATTEMPTS} attempts"
  fi
  printf "\r  attempt %d/40..." "${HEALTH_ATTEMPTS}"
  sleep 2
done
echo ""
ok "Master is healthy at ${MASTER_URL}"
echo ""

# ── Phase 3: Register Workers & Prepare Images ────────────────────────────────
echo -e "${BOLD}━━━  Phase 3: Register Workers & Prepare Images  ━━━━━━━━━━━━━━${RESET}"
log "Registering workers (using host-mapped ports)..."
# Master runs on HOST; Docker workers are exposed on host ports 55052/55053/55054.
# Using Docker-internal hostnames (worker-small:50052) fails because the host
# cannot resolve Docker service names. Use 127.0.0.1 + mapped ports instead.
MASTER_URL="${MASTER_URL}" \
WORKER_SPECS="worker-small=127.0.0.1:55052;worker-medium=127.0.0.1:55053;worker-large=127.0.0.1:55054" \
  bash "${REPO_ROOT}/testbench/scripts/register_workers.sh"
sleep 5

# Verify workers are active
ACTIVE=$(curl -s "${MASTER_URL}/api/workers" | \
  ${PYTHON} -c "import sys,json; d=json.load(sys.stdin); ws=d.get('workers',[]); print(sum(1 for w in ws if w.get('is_active')))" 2>/dev/null || echo 0)
if [[ "${ACTIVE}" -lt 1 ]]; then
  die "No active workers found. Check: curl -s ${MASTER_URL}/api/workers | python3 -m json.tool"
fi
ok "${ACTIVE} active worker(s) registered"

if [[ "${SKIP_IMAGES}" == "false" ]]; then
  log "Preparing workflow images in worker DinD environments..."
  bash "${REPO_ROOT}/testbench/scripts/prepare_workflow_images.sh" 2>/dev/null \
    || warn "prepare_workflow_images.sh had a non-zero exit — images may already be present"
  ok "Workflow images ready"
else
  warn "--skip-images: skipping image preparation"
fi
echo ""

# ── Phase 4: Run Campaign ─────────────────────────────────────────────────────
echo -e "${BOLD}━━━  Phase 4: Run PPO Benchmark Campaign  ━━━━━━━━━━━━━━━━━━━━━${RESET}"
log "Starting campaign..."
log "  Scenarios : ${SCENARIOS}"
log "  Workloads : ${WORKLOADS}"
log "  Schedulers: ${SCHEDULERS}"
echo ""

"${PYTHON}" "${REPO_ROOT}/testbench/scripts/run_campaign.py" \
  --master-url    "${MASTER_URL}" \
  --scenarios     "${SCENARIOS}" \
  --schedulers    "${SCHEDULERS}" \
  --workloads     "${WORKLOADS}" \
  --output-dir    "${OUTPUT_DIR}"

echo ""

# ── Phase 5: Results ──────────────────────────────────────────────────────────
echo -e "${BOLD}━━━  Phase 5: Results  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"

LATEST_DIR=$(ls -td "${OUTPUT_DIR}"/*/ 2>/dev/null | head -1 || echo "${OUTPUT_DIR}")
if [[ ! -f "${LATEST_DIR}/scheduler-summary.csv" ]]; then
  # Sometimes output_dir IS the timestamped dir
  LATEST_DIR="${OUTPUT_DIR}"
fi

if [[ -f "${LATEST_DIR}/scheduler-summary.csv" ]]; then
  echo ""
  ok "Results saved to: ${LATEST_DIR}"
  echo ""
  echo -e "${BOLD}📊 Scheduler Summary:${RESET}"
  echo "──────────────────────────────────────────────────────────────────────"
  ${PYTHON} - "${LATEST_DIR}/scheduler-summary.csv" << 'PY'
import csv, sys

path = sys.argv[1]
rows = list(csv.DictReader(open(path)))
if not rows:
    print("  (no data)")
    sys.exit(0)

cols = ["scheduler", "avg_duration_seconds", "avg_queue_wait_seconds",
        "avg_turnaround_seconds", "p95_turnaround_seconds", "avg_success_rate"]
labels = {
    "scheduler": "Scheduler",
    "avg_duration_seconds": "Duration(s)",
    "avg_queue_wait_seconds": "QueueWait(s)",
    "avg_turnaround_seconds": "Turnaround(s)",
    "p95_turnaround_seconds": "P95(s)",
    "avg_success_rate": "Success%",
}
widths = {c: max(len(labels[c]), max(len(str(r.get(c,""))) for r in rows)) for c in cols}
hdr = "  ".join(labels[c].ljust(widths[c]) for c in cols)
sep = "  ".join("-" * widths[c] for c in cols)
print(hdr)
print(sep)
for r in rows:
    print("  ".join(str(r.get(c, "")).ljust(widths[c]) for c in cols))
PY
  echo "──────────────────────────────────────────────────────────────────────"

  # ── Compare with old results ─────────────────────────────────────────────
  OLD_CSV="${REPO_ROOT}/results/campaign/20260502-185646/scheduler-summary.csv"
  if [[ -f "${OLD_CSV}" ]]; then
    echo ""
    echo -e "${BOLD}📈 Comparison vs. Old Results (20260502-185646):${RESET}"
    ${PYTHON} - "${OLD_CSV}" "${LATEST_DIR}/scheduler-summary.csv" << 'PY'
import csv, sys

def load(path):
    return {r["scheduler"]: r for r in csv.DictReader(open(path))}

old = load(sys.argv[1])
new = load(sys.argv[2])

def pct(new_v, old_v):
    try:
        n, o = float(new_v), float(old_v)
        if o == 0:
            return "n/a"
        d = ((n - o) / o) * 100
        arrow = "▲" if d > 0 else "▼"
        return f"{arrow}{abs(d):.1f}%"
    except Exception:
        return "n/a"

def better(sched, metric, new_val, old_val):
    # Lower is better for duration/wait/turnaround; higher is better for success rate
    try:
        n, o = float(new_val), float(old_val)
        if "success" in metric:
            return "✓" if n >= o else "✗"
        return "✓" if n < o else ("=" if n == o else "✗")
    except Exception:
        return " "

metrics = [
    ("avg_duration_seconds", "Avg Duration"),
    ("avg_queue_wait_seconds", "Queue Wait"),
    ("avg_turnaround_seconds", "Turnaround"),
    ("p95_turnaround_seconds", "P95 Turnaround"),
]

for sched in ["RR", "RTS", "PPO"]:
    if sched not in old or sched not in new:
        continue
    print(f"\n  {sched}:")
    for key, label in metrics:
        ov = old[sched].get(key, "0")
        nv = new[sched].get(key, "0")
        b = better(sched, key, nv, ov)
        delta = pct(nv, ov)
        print(f"    {label:18s}  {float(ov):8.3f}s  →  {float(nv):8.3f}s  ({delta})  {b}")
PY
  fi
else
  warn "scheduler-summary.csv not found — campaign may have failed. Check above output."
fi

echo ""
echo -e "${BOLD}📂 Full report:${RESET}"
echo "   cat ${LATEST_DIR}/REPORT.md"
echo ""

# ── Teardown ──────────────────────────────────────────────────────────────────
echo -e "${BOLD}━━━  Done  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
ok "Benchmark complete!"
echo ""
echo "Results : ${LATEST_DIR}/REPORT.md"
echo "Log     : ${REPO_ROOT}/run-ppo-test-master.log"
echo ""
echo "To stop the worker Docker stack:"
echo "  docker compose -f ${COMPOSE_HOST} down --remove-orphans"
echo ""
# cleanup_test (EXIT trap) will kill the headless master
