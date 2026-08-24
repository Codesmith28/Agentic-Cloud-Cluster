#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — Clean-Slate PPO Benchmark Runner
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Source shared helpers
# shellcheck source=../_common.sh
source "${SCRIPT_DIR}/../_common.sh"

# ── Defaults ──────────────────────────────────────────────────────────────────
MASTER_URL="${MASTER_URL:-http://localhost:8080}"
COMPOSE_HOST="${REPO_ROOT}/testbench/docker-compose.host-master.yml"
PYTHON="$(resolve_python)"
SCENARIOS="baseline,burst,overload"
WORKLOADS="resource-contention-ppo"
SCHEDULERS="RR,RTS,PPO"
OUTPUT_DIR="${REPO_ROOT}/results/campaign/ppo-new-model-test"
STARTUP_WAIT=45
SKIP_CLEANUP=false
SKIP_IMAGES=false

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
      echo "Usage: $0 [--scenarios <list>] [--workloads <list>] [--fast] [--skip-cleanup] [--skip-images]"
      exit 0
      ;;
    *) die "Unknown argument: $1" ;;
  esac
done

TS="$(date +%Y%m%d-%H%M%S)"
OUTPUT_DIR="${OUTPUT_DIR}/${TS}"

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

MASTER_BG_PID=""

cleanup_test() {
  echo ""
  log "Cleaning up test run..."
  if [[ -n "${MASTER_BG_PID:-}" ]]; then
    kill "${MASTER_BG_PID}" 2>/dev/null || true
    wait "${MASTER_BG_PID}" 2>/dev/null || true
    ok "Headless master stopped"
  fi
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

  pgrep -f "masterNode" 2>/dev/null | xargs kill 2>/dev/null || true
  pgrep -f "agentic_scheduler.server" 2>/dev/null | xargs kill 2>/dev/null || true
  sleep 2
  ok "Clean slate ready"
else
  warn "--skip-cleanup: skipping teardown and DB wipe"
fi

MODEL_PATH="${REPO_ROOT}/agentic_scheduler/models/ppo_latest.pt"
if [[ -f "${MODEL_PATH}" ]]; then
  MODEL_SIZE=$(du -h "${MODEL_PATH}" | cut -f1)
  ok "PPO model: ${MODEL_PATH} (${MODEL_SIZE})"
else
  die "Model file not found: ${MODEL_PATH}"
fi

IFS=',' read -ra WL_ARR <<< "${WORKLOADS}"
for wl in "${WL_ARR[@]}"; do
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

log "Building master binary..."
make master -C "${REPO_ROOT}" >/dev/null 2>&1 || { make master -C "${REPO_ROOT}"; }
ok "Master built"

log "Starting master in headless mode (AGENTIC_HEADLESS=true, PPO enabled)..."
AGENTIC_HEADLESS=true \
CLOUDAI_HEADLESS=true \
SCHED_ALGO=PPO \
PPO_AUTOSTART=true \
PPO_MODEL_PATH=latest \
PPO_DEPLOYMENT_MODE=active \
PPO_ONLINE_UPDATES_ENABLED=true \
MONGODB_URI="mongodb://${MONGO_USERNAME:-agentic}:${MONGO_PASSWORD:-agentic-cluster-pass}@localhost:27018/cluster_db?authSource=admin" \
  "${REPO_ROOT}/master/masterNode" --mode headless \
  >> "${REPO_ROOT}/run-ppo-test-master.log" 2>&1 &
MASTER_BG_PID=$!
ok "Master started (PID=${MASTER_BG_PID})"

log "Waiting for master HTTP :8080..."
for i in $(seq 1 40); do
  if curl -s -f "${MASTER_URL}/health" >/dev/null 2>&1; then
    ok "Master is healthy"
    break
  fi
  if [[ $i -eq 40 ]]; then
    die "Master did not become healthy within 40s"
  fi
  sleep 1
done

# ── Phase 3: Register Workers ─────────────────────────────────────────────────
echo -e "${BOLD}━━━  Phase 3: Register Workers  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
export MASTER_URL
export WORKER_SPECS="worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054"
bash "${REPO_ROOT}/testbench/scripts/register_workers.sh"
ok "Workers registered with master"

# ── Phase 4: Prepare Deterministic Images ──────────────────────────────────────
echo -e "${BOLD}━━━  Phase 4: Prepare Workflow Images  ━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
if [[ "${SKIP_IMAGES}" == "false" ]]; then
  bash "${REPO_ROOT}/testbench/scripts/prepare_workflow_images.sh"
  ok "Workflow images loaded into all DinD workers"
else
  warn "--skip-images: skipping workflow image load into workers"
fi

# ── Phase 5: Run Campaign ─────────────────────────────────────────────────────
echo -e "${BOLD}━━━  Phase 5: Run Benchmark Campaign  ━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
mkdir -p "${OUTPUT_DIR}"

log "Executing run_campaign.py..."
"${PYTHON}" "${REPO_ROOT}/testbench/scripts/run_campaign.py" \
  --master-url "${MASTER_URL}" \
  --scenarios "${SCENARIOS}" \
  --workloads "${WORKLOADS}" \
  --schedulers "${SCHEDULERS}" \
  --output-dir "${OUTPUT_DIR}" \
  --export-metrics \
  --export-snapshots

ok "Benchmark complete! Results written to: ${OUTPUT_DIR}"
