#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — Full Clean-Slate Cluster Reset
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

COMPOSE_HOST="${REPO_ROOT}/testbench/docker-compose.host-master.yml"
COMPOSE_FULL="${REPO_ROOT}/testbench/docker-compose.yml"
DATABASE_COMPOSE="${REPO_ROOT}/database/docker-compose.yml"

# ── Parse args ────────────────────────────────────────────────────────────────
KEEP_IMAGES=false
KEEP_DIND=false
SOFT=false
YES=false

for arg in "$@"; do
  case "${arg}" in
    --keep-images) KEEP_IMAGES=true ;;
    --keep-dind)   KEEP_DIND=true ;;
    --soft)        SOFT=true ;;
    --yes|-y)      YES=true ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --keep-images     Skip removing pulled Docker images"
      echo "  --keep-dind       Skip wiping Docker-in-Docker layers"
      echo "  --soft            Only stop containers; do NOT wipe volumes / DB"
      echo "  --yes             Skip confirmation prompt"
      echo "  -h, --help        Show this help"
      exit 0
      ;;
    *) echo "Unknown argument: ${arg}" >&2; exit 1 ;;
  esac
done

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; RESET='\033[0m'

step()  { echo -e "\n${BOLD}━━━  $* ${RESET}"; }
log()   { echo -e "${CYAN}  ▶${RESET} $*"; }
ok()    { echo -e "${GREEN}  ✓${RESET} $*"; }
warn()  { echo -e "${YELLOW}  ⚠${RESET} $*"; }
skip()  { echo -e "${DIM}  ─ $* (skipped)${RESET}"; }

# ── Banner ────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}║     Agentic Cloud Cluster — Full Clean-Slate Reset               ║${RESET}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════════╝${RESET}"
echo ""
echo -e "  Mode        : $(${SOFT} && echo 'SOFT (stop only, no data wipe)' || echo 'FULL WIPE')"
echo -e "  Keep DinD   : ${KEEP_DIND}"
echo -e "  Keep Images : ${KEEP_IMAGES}"
echo ""

# ── Confirmation ──────────────────────────────────────────────────────────────
if [[ "${SOFT}" == "false" && "${YES}" == "false" ]]; then
  echo -e "${YELLOW}  This will permanently destroy:${RESET}"
  echo "    • All Docker volumes (MongoDB data, worker state, Prometheus metrics)"
  echo "    • All task/worker/scheduler records in MongoDB"
  echo "    • Docker-in-Docker image layers (unless --keep-dind)"
  echo "    • Local PPO replay buffer / online-update state in MongoDB"
  echo ""
  read -rp "  Continue? [y/N] " CONFIRM
  case "${CONFIRM}" in
    [yY]|[yY][eE][sS]) ;;
    *) echo "  Aborted."; exit 0 ;;
  esac
fi
echo ""

# ═════════════════════════════════════════════════════════════════════════════
# PHASE 1 — Stop all running services
# ═════════════════════════════════════════════════════════════════════════════
step "Phase 1 — Stop all running services"

# Host-master compose stack (workers + Mongo + Prometheus + Grafana)
if [[ -f "${COMPOSE_HOST}" ]]; then
  log "Stopping host-master compose stack..."
  docker compose -f "${COMPOSE_HOST}" down --remove-orphans --timeout 20 2>/dev/null \
    && ok "Host-master stack stopped" || warn "Host-master stack already down"
fi

# Full compose stack (all-in-Docker, including containerised master)
if [[ -f "${COMPOSE_FULL}" ]]; then
  log "Stopping full compose stack..."
  docker compose -f "${COMPOSE_FULL}" down --remove-orphans --timeout 20 2>/dev/null \
    && ok "Full stack stopped" || warn "Full stack already down"
fi

# Standalone database compose (used by scripts/master/run.sh)
if [[ -f "${DATABASE_COMPOSE}" ]]; then
  log "Stopping standalone MongoDB..."
  docker compose -f "${DATABASE_COMPOSE}" down --remove-orphans --timeout 10 2>/dev/null \
    && ok "Standalone MongoDB stopped" || warn "Standalone MongoDB already down"
fi

# Kill any host-side master or PPO Python process still running
HOST_MASTER_PID=$(pgrep -f "masterNode\b" 2>/dev/null || true)
if [[ -n "${HOST_MASTER_PID}" ]]; then
  log "Killing host masterNode process (PID ${HOST_MASTER_PID})..."
  kill "${HOST_MASTER_PID}" 2>/dev/null && ok "masterNode stopped" || warn "Could not kill masterNode"
  sleep 1
else
  skip "No host masterNode process found"
fi

HOST_PPO_PIDS=$(pgrep -f "agentic_scheduler.server" 2>/dev/null || true)
if [[ -n "${HOST_PPO_PIDS}" ]]; then
  log "Killing PPO gRPC service (PID(s): ${HOST_PPO_PIDS})..."
  echo "${HOST_PPO_PIDS}" | while read -r pid; do
    kill "${pid}" 2>/dev/null && ok "PPO service PID ${pid} stopped" || warn "Could not kill ${pid}"
  done
  sleep 1
else
  skip "No PPO gRPC service process found"
fi

# Kill Vite / npm dev server
UI_PIDS=$(pgrep -f "vite\|npm.*dev\|node.*vite" 2>/dev/null || true)
if [[ -n "${UI_PIDS}" ]]; then
  log "Killing UI dev server (Vite/npm)..."
  echo "${UI_PIDS}" | while read -r pid; do
    kill "${pid}" 2>/dev/null || true
  done
  sleep 1
  STUBBORN=$(pgrep -f "vite\|npm.*dev\|node.*vite" 2>/dev/null || true)
  if [[ -n "${STUBBORN}" ]]; then
    echo "${STUBBORN}" | while read -r pid; do
      kill -9 "${pid}" 2>/dev/null || true
    done
  fi
  ok "UI dev server stopped"
else
  skip "No UI dev server running"
fi

# Kill anything still holding the key ports
for PORT in 8080 50050 50051 3001; do
  PORT_PID=$(lsof -ti :"${PORT}" -sTCP:LISTEN 2>/dev/null || true)
  if [[ -n "${PORT_PID}" ]]; then
    log "Freeing port ${PORT} (PID ${PORT_PID})..."
    kill -9 "${PORT_PID}" 2>/dev/null || true
  fi
done

sleep 2

# ═════════════════════════════════════════════════════════════════════════════
# PHASE 1.5 — Clear MongoDB runtime collections
# ═════════════════════════════════════════════════════════════════════════════
step "Phase 1.5 — Clear MongoDB runtime state"

_clear_mongo_runtime() {
  local URI="$1"
  local LABEL="$2"
  docker run --rm --network host mongo:7.0 mongosh --quiet "${URI}" \
    --eval '
const cols = ["WORKER_REGISTRY","TASKS","ATTEMPTS","RESULTS","FILE_METADATA",
               "ASSIGNMENTS","SCHEDULER_MODELS",
               "scheduler_models.files","scheduler_models.chunks"];
var cleared = 0;
cols.forEach(col => {
  try { const r = db[col].deleteMany({}); if (r.deletedCount > 0) { print("  cleared " + col + ": " + r.deletedCount); cleared++; } } catch(e) {}
});
if (cleared === 0) { print("  (already empty)"); }
' 2>/dev/null && ok "Runtime collections cleared (${LABEL})" || warn "Could not clear ${LABEL} (not running?)"
}

# Standalone database MongoDB (port 27017 — used by master runner)
if docker ps --format '{{.Names}}' 2>/dev/null | grep -q -E "agentic-mongo|cloudai-mongo"; then
  _clear_mongo_runtime \
    "mongodb://agentic:agentic-cluster-pass@localhost:27017/cluster_db?authSource=admin" \
    "standalone MongoDB :27017" || \
  _clear_mongo_runtime \
    "mongodb://cloudai:cloudai-stress-test@localhost:27017/cluster_db?authSource=admin" \
    "standalone MongoDB :27017 (legacy)"
else
  skip "Standalone MongoDB not running — skipping collection clear"
fi

# Testbench compose MongoDB (port 27018 — used by ppo testbench)
if docker ps --format '{{.Ports}}' 2>/dev/null | grep -q "27018"; then
  _clear_mongo_runtime \
    "mongodb://agentic:agentic-cluster-pass@localhost:27018/cluster_db?authSource=admin" \
    "testbench MongoDB :27018" || \
  _clear_mongo_runtime \
    "mongodb://cloudai:cloudai-stress-test@localhost:27018/cluster_db?authSource=admin" \
    "testbench MongoDB :27018 (legacy)"
else
  skip "Testbench MongoDB not running — skipping collection clear"
fi

# ═════════════════════════════════════════════════════════════════════════════
# PHASE 2 — Wipe persistent data (skipped in --soft mode)
# ═════════════════════════════════════════════════════════════════════════════
if [[ "${SOFT}" == "true" ]]; then
  step "Phase 2 — Data wipe (SKIPPED — soft mode)"
  warn "Volumes and MongoDB data retained (--soft)"
else
  step "Phase 2 — Wipe all persistent volumes"

  HOST_MASTER_VOLUMES=(
    "testbench_mongo-data"
    "testbench_prometheus-data"
    "testbench_grafana-data"
    "testbench_worker-small-state"
    "testbench_worker-medium-state"
    "testbench_worker-large-state"
    "testbench_worker-small-outputs"
    "testbench_worker-medium-outputs"
    "testbench_worker-large-outputs"
  )

  DIND_VOLUMES=(
    "testbench_worker-small-docker"
    "testbench_worker-medium-docker"
    "testbench_worker-large-docker"
  )

  FULL_VOLUMES=(
    "btep_mongo-data"
    "btep_prometheus-data"
    "btep_grafana-data"
    "btep_master-files"
    "btep_worker-small-state"
    "btep_worker-medium-state"
    "btep_worker-large-state"
    "btep_worker-small-outputs"
    "btep_worker-medium-outputs"
    "btep_worker-large-outputs"
    "btep_worker-small-docker"
    "btep_worker-medium-docker"
    "btep_worker-large-docker"
  )

  STANDALONE_VOLUMES=(
    "database_mongo_data"
    "agentic_mongo_data"
    "cloudai_mongo_data"
    "mongo-data"
  )

  ALL_VOLUMES=( "${HOST_MASTER_VOLUMES[@]}" "${FULL_VOLUMES[@]}" "${STANDALONE_VOLUMES[@]}" )

  if [[ "${KEEP_DIND}" == "false" ]]; then
    ALL_VOLUMES+=( "${DIND_VOLUMES[@]}" )
  fi

  WIPED=0
  for vol in "${ALL_VOLUMES[@]}"; do
    if docker volume inspect "${vol}" >/dev/null 2>&1; then
      docker volume rm "${vol}" >/dev/null 2>&1 \
        && { ok "Removed volume: ${vol}"; WIPED=$((WIPED + 1)); } \
        || warn "Could not remove volume: ${vol} (in use?)"
    fi
  done

  if [[ ${WIPED} -eq 0 ]]; then
    skip "No matching named volumes found (already clean)"
  else
    ok "${WIPED} volume(s) removed"
  fi

  DANGLING=$(docker volume ls -qf dangling=true 2>/dev/null || true)
  if [[ -n "${DANGLING}" ]]; then
    COUNT=$(echo "${DANGLING}" | wc -l | tr -d ' ')
    log "Removing ${COUNT} dangling volume(s)..."
    echo "${DANGLING}" | xargs docker volume rm 2>/dev/null \
      && ok "Dangling volumes removed" || warn "Some dangling volumes could not be removed"
  fi
fi

# ═════════════════════════════════════════════════════════════════════════════
# PHASE 3 — Clear local application state
# ═════════════════════════════════════════════════════════════════════════════
step "Phase 3 — Clear local application state"

LOGS_DIR="${REPO_ROOT}/agentic_scheduler/logs"
if [[ -d "${LOGS_DIR}" ]] && compgen -G "${LOGS_DIR}/*.log" >/dev/null 2>&1; then
  COUNT=$(ls -1 "${LOGS_DIR}"/*.log 2>/dev/null | wc -l | tr -d ' ')
  log "Removing ${COUNT} PPO log file(s)..."
  rm -f "${LOGS_DIR}"/*.log && ok "PPO logs cleared"
else
  skip "No PPO logs to clear"
fi

ROOT_LOGS=( training_output.log setup.log test-unit.log testbench-integration.log )
CLEARED=0
for f in "${ROOT_LOGS[@]}"; do
  FP="${REPO_ROOT}/${f}"
  if [[ -f "${FP}" ]]; then
    : > "${FP}"
    CLEARED=$((CLEARED + 1))
  fi
done
[[ ${CLEARED} -gt 0 ]] && ok "Truncated ${CLEARED} root-level log file(s)" || skip "No root log files to truncate"

CKPT_DIR="${REPO_ROOT}/agentic_scheduler/models/checkpoints"
if [[ -d "${CKPT_DIR}" ]] && compgen -G "${CKPT_DIR}/*.pt" >/dev/null 2>&1; then
  COUNT=$(ls -1 "${CKPT_DIR}"/*.pt 2>/dev/null | wc -l | tr -d ' ')
  log "Removing ${COUNT} stale training checkpoint(s) in ${CKPT_DIR}..."
  rm -f "${CKPT_DIR}"/*.pt && ok "Stale checkpoints cleared"
else
  skip "No stale checkpoints in ${CKPT_DIR}"
fi

# ═════════════════════════════════════════════════════════════════════════════
# PHASE 4 — Remove Docker images (optional)
# ═════════════════════════════════════════════════════════════════════════════
if [[ "${KEEP_IMAGES}" == "true" ]]; then
  step "Phase 4 — Docker image cleanup (SKIPPED — --keep-images)"
else
  step "Phase 4 — Remove stale project Docker images"
  IMAGES_REMOVED=0
  for pattern in "testbench-worker" "testbench-master" "testbench_worker" "testbench_master" "agentic-worker" "agentic-master"; do
    IMG_IDS=$(docker images --filter "reference=*${pattern}*" -q 2>/dev/null || true)
    if [[ -n "${IMG_IDS}" ]]; then
      echo "${IMG_IDS}" | xargs docker rmi -f 2>/dev/null \
        && IMAGES_REMOVED=$((IMAGES_REMOVED + 1)) || true
    fi
  done
  [[ ${IMAGES_REMOVED} -gt 0 ]] && ok "Project images removed" || skip "No project images found"
fi

# ═════════════════════════════════════════════════════════════════════════════
# PHASE 5 — Verify new PPO model is in place
# ═════════════════════════════════════════════════════════════════════════════
step "Phase 5 — Verify PPO model"

MODEL_DIR="${REPO_ROOT}/agentic_scheduler/models"
LATEST_MODEL="${MODEL_DIR}/ppo_latest.pt"

if [[ -f "${LATEST_MODEL}" ]]; then
  ok "ppo_latest.pt exists : ${LATEST_MODEL} ($(du -h "${LATEST_MODEL}" | cut -f1))"
else
  warn "No trained model found at ${LATEST_MODEL}"
fi

echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}║  ✅  Clean slate ready — cluster fully reset                     ║${RESET}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════════╝${RESET}"
echo ""
