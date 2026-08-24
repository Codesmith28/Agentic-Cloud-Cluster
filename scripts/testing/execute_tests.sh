#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — End-to-End Testbench Campaign Runner
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}"

# Source shared helpers
# shellcheck source=../_common.sh
source "${SCRIPT_DIR}/../_common.sh"

# ── Defaults ─────────────────────────────────────────────────────────────────
MODEL_SRC="agentic_scheduler/results/ppo_trained_final.pt"
MODEL_DST="agentic_scheduler/models/ppo_latest.pt"
CAMPAIGN_MODE="smoke" # "smoke", "full", "comprehensive", or "isolated"
SKIP_BUILD=false
TEARDOWN_ONLY=false
ISOLATED_WORKLOADS=false
MASTER_URL="http://localhost:8080"
COMPOSE_FILE="testbench/docker-compose.host-master.yml"
WORKER_SPECS="worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054"
MASTER_PID=""
MASTER_BIN="master/masterNode"

# Ensure GF_ADMIN_PASSWORD is always set
export GF_ADMIN_PASSWORD="${GF_ADMIN_PASSWORD:-password}"
export MONGO_PASSWORD="${MONGO_PASSWORD:-agentic-cluster-pass}"
export MONGO_USERNAME="${MONGO_USERNAME:-agentic}"

# ── Parse arguments ─────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
    --full)
        CAMPAIGN_MODE="full"
        shift
        ;;
    --comprehensive)
        CAMPAIGN_MODE="comprehensive"
        shift
        ;;
    --isolated-workloads)
        CAMPAIGN_MODE="isolated"
        ISOLATED_WORKLOADS=true
        shift
        ;;
    --model)
        MODEL_SRC="$2"
        shift 2
        ;;
    --skip-build)
        SKIP_BUILD=true
        shift
        ;;
    --teardown)
        TEARDOWN_ONLY=true
        shift
        ;;
    -h | --help)
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --full                Run all 11 workloads across baseline, burst, overload"
        echo "  --comprehensive       Run comprehensive 5-workload benchmark with metrics export"
        echo "  --isolated-workloads  Run each workload in a freshly cleaned cluster"
        echo "  --model <path>        Path to model checkpoint to deploy before running"
        echo "  --skip-build          Skip rebuilding master binary"
        echo "  --teardown            Tear down running containers and exit"
        echo "  -h, --help            Show this help message"
        exit 0
        ;;
    *)
        err "Unknown option: $1"
        exit 1
        ;;
    esac
done

# ── Teardown Helper ──────────────────────────────────────────────────────────
teardown() {
    log "Tearing down Docker environment..."
    docker compose -f "${COMPOSE_FILE}" down -v --remove-orphans 2>/dev/null || true

    if [[ -n "${MASTER_PID}" ]] && kill -0 "${MASTER_PID}" 2>/dev/null; then
        log "Stopping master node (PID: ${MASTER_PID})..."
        kill "${MASTER_PID}" 2>/dev/null || true
        wait "${MASTER_PID}" 2>/dev/null || true
    fi

    local orphan_pids
    orphan_pids=$(pgrep -f "masterNode\b" 2>/dev/null || true)
    if [[ -n "${orphan_pids}" ]]; then
        log "Cleaning up orphan masterNode processes: ${orphan_pids}"
        kill -9 "${orphan_pids}" 2>/dev/null || true
    fi

    local ppo_pids
    ppo_pids=$(pgrep -f "agentic_scheduler.server" 2>/dev/null || true)
    if [[ -n "${ppo_pids}" ]]; then
        log "Cleaning up orphan PPO services: ${ppo_pids}"
        kill -9 "${ppo_pids}" 2>/dev/null || true
    fi

    ok "Teardown complete."
}

if [[ "${TEARDOWN_ONLY}" == "true" ]]; then
    teardown
    exit 0
fi

trap teardown EXIT

# ── Step 1: Deploy / Promote Model Checkpoint ────────────────────────────────
log "Deploying model checkpoint..."
if [[ -f "${MODEL_SRC}" ]]; then
    mkdir -p "$(dirname "${MODEL_DST}")"
    cp "${MODEL_SRC}" "${MODEL_DST}"
    ok "Model copied: ${MODEL_SRC} -> ${MODEL_DST}"
elif [[ -f "${MODEL_DST}" ]]; then
    ok "Using existing model at ${MODEL_DST}"
else
    warn "No model at ${MODEL_SRC} or ${MODEL_DST}. PPO will train from scratch or use cold-start."
fi

# ── Step 2: Build Master Binary ──────────────────────────────────────────────
if [[ "${SKIP_BUILD}" == "false" ]]; then
    log "Building master binary..."
    make master
    if [[ ! -f "${MASTER_BIN}" ]]; then
        die "Build failed: ${MASTER_BIN} not found"
    fi
    ok "Master binary built successfully."
else
    ok "Skipping master build (--skip-build set)."
fi

# ── Step 3: Clean Start Docker Stack ─────────────────────────────────────────
log "Starting worker and observability stack (Docker)..."
docker compose -f "${COMPOSE_FILE}" down -v --remove-orphans 2>/dev/null || true
docker compose -f "${COMPOSE_FILE}" up -d --build

log "Waiting for core services (MongoDB, Prometheus, Grafana)..."
docker compose -f "${COMPOSE_FILE}" exec -T mongo mongosh --eval "db.adminCommand('ping')" --quiet >/dev/null 2>&1 || {
    log "Waiting for MongoDB to become ready..."
    for i in $(seq 1 30); do
        if docker compose -f "${COMPOSE_FILE}" exec -T mongo mongosh --eval "db.adminCommand('ping')" --quiet >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
}
ok "Infrastructure stack is up and healthy."

# ── Step 4: Prepare Deterministic Workflow Image ─────────────────────────────
log "Building and loading workflow image into worker DinD daemons..."
bash testbench/scripts/prepare_workflow_images.sh
ok "Deterministic workflow image ready."

# ── Step 5: Launch Host-Master Node ──────────────────────────────────────────
log "Starting host-master node with PPO scheduler..."
AGENTIC_HEADLESS=true \
CLOUDAI_HEADLESS=true \
SCHED_ALGO=PPO \
PPO_AUTOSTART=true \
PPO_MODEL_PATH=latest \
PPO_DEPLOYMENT_MODE=active \
PPO_ONLINE_UPDATES_ENABLED=true \
MONGODB_URI="mongodb://${MONGO_USERNAME}:${MONGO_PASSWORD}@localhost:27018/cluster_db?authSource=admin" \
    ./"${MASTER_BIN}" --mode headless >master.log 2>&1 &
MASTER_PID=$!
log "Host master running with PID ${MASTER_PID}. Logs redirected to master.log."

# Wait for master HTTP API
log "Waiting for Master HTTP API on :8080..."
for i in $(seq 1 30); do
    if curl -s "${MASTER_URL}/health" >/dev/null 2>&1 || curl -s "${MASTER_URL}/api/workers" >/dev/null 2>&1; then
        break
    fi
    if ! kill -0 "${MASTER_PID}" 2>/dev/null; then
        err "Master node process died unexpectedly. Check master.log for details:"
        cat master.log
        exit 1
    fi
    sleep 1
done
ok "Master node is responding."

# ── Step 6: Register Workers ─────────────────────────────────────────────────
log "Registering worker nodes with master..."
export MASTER_URL
export WORKER_SPECS
bash testbench/scripts/register_workers.sh
ok "All workers registered."

# ── Step 7: Execute Campaign ─────────────────────────────────────────────────
PYTHON="$(resolve_python)"

case "${CAMPAIGN_MODE}" in
smoke)
    log "Running smoke campaign (heterogeneous-smoke workload)..."
    "${PYTHON}" testbench/scripts/run_campaign.py \
        --master-url "${MASTER_URL}" \
        --scenarios baseline \
        --schedulers PPO \
        --workloads heterogeneous-smoke
    ;;
full)
    log "Running FULL testbench campaign (11 workloads)..."
    "${PYTHON}" testbench/scripts/run_campaign.py \
        --master-url "${MASTER_URL}" \
        --scenarios baseline,burst,overload \
        --schedulers RR,RTS,PPO \
        --workloads deterministic-full,heterogeneous-smoke,stress-heavy,resource-contention-ppo,failure-stressed,bursty,steady-cpu,steady-mixed,memory-pressure,long-tail,failure-helpers
    ;;
comprehensive)
    log "Running COMPREHENSIVE campaign with metrics & snapshots..."
    bash scripts/tools/run_comprehensive.sh 2>/dev/null || \
    "${PYTHON}" testbench/scripts/run_campaign.py \
        --master-url "${MASTER_URL}" \
        --scenarios baseline,burst,overload \
        --schedulers RR,RTS,PPO \
        --workloads deterministic-full,heterogeneous-smoke,resource-contention-ppo,stress-heavy,failure-stressed
    ;;
isolated)
    log "Running ISOLATED workload campaign..."
    for wl in deterministic-full heterogeneous-smoke resource-contention-ppo stress-heavy failure-stressed; do
        log "Running isolated workload: ${wl}"
        "${PYTHON}" testbench/scripts/run_campaign.py \
            --master-url "${MASTER_URL}" \
            --scenarios baseline \
            --schedulers RR,RTS,PPO \
            --workloads "${wl}"
    done
    ;;
esac

ok "Campaign run completed."

# ── Step 8: Post-Execution Summary ───────────────────────────────────────────
echo ""
echo "============================================================================="
echo "  Testbench Campaign Completed Successfully"
echo "============================================================================="
echo "  Grafana Dashboard : http://localhost:3300"
echo "  Prometheus UI     : http://localhost:9090"
echo "  Master Node Logs  : master.log"
echo "  Campaign Results  : results/"
echo "============================================================================="
