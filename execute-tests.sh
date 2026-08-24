#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# execute-tests.sh — End-to-end deployment and benchmark runner
#
# Uses the HOST-MASTER topology: the master node runs locally on your machine
# (so the PPO service can read/write the local .pt model file directly), while
# workers, MongoDB, and observability run in Docker containers.
#
# Usage:
#   ./execute-tests.sh                  # Run with defaults (smoke workload)
#   ./execute-tests.sh --full           # Full campaign (all workloads)
#   ./execute-tests.sh --model <path>   # Use a specific model checkpoint
#   ./execute-tests.sh --skip-build     # Skip Go build step
#   ./execute-tests.sh --teardown       # Only tear down the stack
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

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

# Ensure GF_ADMIN_PASSWORD is always set (required by docker-compose, even for teardown)
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
    --help | -h)
        echo "Usage: $0 [OPTIONS]"
        echo ""
        echo "Options:"
        echo "  --full            Run full campaign (all workloads + scenarios)"
        echo "  --comprehensive   Run comprehensive benchmark (multiple workloads, all scenarios)"
        echo "  --isolated-workloads  Run each workload in isolation with model reset (online PPO specialization)"
        echo "  --model <path>    Path to .pt model checkpoint (default: $MODEL_SRC)"
        echo "  --skip-build      Skip building master/worker binaries"
        echo "  --teardown        Only tear down the Docker worker stack"
        echo "  -h, --help        Show this help"
        exit 0
        ;;
    *)
        echo "Unknown option: $1" >&2
        exit 1
        ;;
    esac
done

# ── Helpers ──────────────────────────────────────────────────────────────────
info() { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok() { echo -e "\033[1;32m[OK]\033[0m    $*"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m  $*"; }
fail() {
    echo -e "\033[1;31m[FAIL]\033[0m  $*" >&2
    exit 1
}

separator() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  $*"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

cleanup() {
    stop_local_master
    if [[ "${TEARDOWN_DONE:-false}" != "true" ]]; then
        info "Tearing down Docker workers..."
        docker compose -f "${COMPOSE_FILE}" down --volumes --remove-orphans 2>/dev/null || true
        TEARDOWN_DONE=true
    fi
}

stop_local_master() {
    if [[ -n "${MASTER_PID}" ]] && kill -0 "${MASTER_PID}" 2>/dev/null; then
        info "Stopping local master (PID ${MASTER_PID})..."
        kill "${MASTER_PID}" 2>/dev/null || true
        wait "${MASTER_PID}" 2>/dev/null || true
        ok "Master stopped"
    fi
    MASTER_PID=""
}

start_local_master() {
    if [[ ! -f "${MASTER_BIN}" ]]; then
        fail "Master binary not found at ${MASTER_BIN}. Run without --skip-build."
    fi

    info "Launching master node locally (PPO model updates write to local .pt)..."
    CLOUDAI_HEADLESS=true \
        MONGODB_HOST=localhost:27018 \
        MONGODB_USERNAME="${MONGO_USERNAME}" \
        MONGODB_PASSWORD="${MONGO_PASSWORD}" \
        MONGODB_DATABASE=cluster_db \
        SCHED_ALGO="${SCHED_ALGO}" \
        PPO_AUTOSTART="${PPO_AUTOSTART}" \
        PPO_MODEL_PATH="${PPO_MODEL_PATH}" \
        PPO_DEPLOYMENT_MODE="${PPO_DEPLOYMENT_MODE}" \
        PPO_ONLINE_UPDATES_ENABLED="${PPO_ONLINE_UPDATES_ENABLED}" \
        "${MASTER_BIN}" --mode cli &
    MASTER_PID=$!

    info "Waiting for master API to become reachable..."
    local max_wait=60
    for i in $(seq 1 "${max_wait}"); do
        if curl -fsS "${MASTER_URL}/health" >/dev/null 2>&1; then
            ok "Master API is up at ${MASTER_URL} (PID ${MASTER_PID})"
            return 0
        fi
        if ! kill -0 "${MASTER_PID}" 2>/dev/null; then
            fail "Master process exited unexpectedly"
        fi
        if [[ $i -eq ${max_wait} ]]; then
            fail "Master API did not become ready after ${max_wait}s"
        fi
        sleep 1
    done
}

register_workers() {
    MASTER_URL="${MASTER_URL}" \
        WORKER_SPECS="${WORKER_SPECS}" \
        testbench/scripts/register_workers.sh
}
trap cleanup EXIT

# ── Teardown mode ────────────────────────────────────────────────────────────
if [[ "${TEARDOWN_ONLY}" == "true" ]]; then
    separator "Tearing down testbench stack"
    docker compose -f "${COMPOSE_FILE}" down --volumes --remove-orphans
    TEARDOWN_DONE=true
    ok "Testbench stack stopped and removed."
    exit 0
fi

# ── Pre-flight checks ───────────────────────────────────────────────────────
separator "Pre-flight checks"

command -v docker >/dev/null 2>&1 || fail "docker is not installed"
command -v python3 >/dev/null 2>&1 || fail "python3 is not installed"
docker info >/dev/null 2>&1 || fail "Docker daemon is not running"

if [[ ! -f "${MODEL_SRC}" ]]; then
    fail "Model checkpoint not found: ${MODEL_SRC}"
fi

ok "All pre-flight checks passed"

# ── Step 0: Smart cleanup — reuse workers if healthy ────────────────────────
separator "Step 0: Preparing testbench environment"

# Always kill stale master — it holds ports and scheduler state
if pgrep -f "masterNode" >/dev/null 2>&1; then
    info "Stopping stale master process..."
    while read -r pid; do
        [[ -n "${pid}" ]] || continue
        kill "${pid}" 2>/dev/null || true
    done < <(pgrep -f "masterNode" || true)
    sleep 1
    ok "Stale master stopped"
fi

# Check if workers are already running and healthy.
# Use exact docker health status to avoid false positives:
# - "unhealthy" contains the substring "healthy"
# - dind sidecars are separate from scheduler workers
WORKERS_HEALTHY=false
HEALTHY_COUNT=0
for worker in testbench-worker-small-1 testbench-worker-medium-1 testbench-worker-large-1; do
    status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${worker}" 2>/dev/null || true)"
    if [[ "${status}" == "healthy" ]]; then
        HEALTHY_COUNT=$((HEALTHY_COUNT + 1))
    fi
done
if [[ "${HEALTHY_COUNT}" -eq 3 ]]; then
    WORKERS_HEALTHY=true
fi

if [[ "${WORKERS_HEALTHY}" == "true" ]]; then
    info "Workers already running — reusing existing containers (skipping teardown)"
    info "Only the master will be restarted fresh"
else
    # No healthy workers — full teardown and fresh start
    info "No healthy workers found — doing full teardown and fresh start..."
    docker compose -f "${COMPOSE_FILE}" down --volumes --remove-orphans 2>/dev/null || true
    ok "Previous state cleaned up"
fi

# ── Step 1: Deploy model ────────────────────────────────────────────────────
separator "Step 1: Promoting trained model (with version archival)"

"${SCRIPT_DIR}/scripts/model_promote.sh" "${MODEL_SRC}"

# ── Step 2: Build binaries ──────────────────────────────────────────────────
if [[ "${SKIP_BUILD}" == "true" ]]; then
    info "Skipping build (--skip-build)"
else
    separator "Step 2: Building master, worker, and Python deps"
    make master
    make worker
    make pip-install
    ok "Binaries built and Python deps installed"
fi

# ── Step 3: Set environment for PPO ─────────────────────────────────────────
separator "Step 3: Configuring PPO environment"

export SCHED_ALGO=PPO
export PPO_AUTOSTART=true
export PPO_MODEL_PATH="${MODEL_DST}"
export PPO_DEPLOYMENT_MODE=active
export PPO_ONLINE_UPDATES_ENABLED=true
# export PPO_ONLINE_UPDATES_ENABLED=false

ok "Environment configured:"
echo "    SCHED_ALGO              = ${SCHED_ALGO}"
echo "    PPO_AUTOSTART           = ${PPO_AUTOSTART}"
echo "    PPO_MODEL_PATH          = ${PPO_MODEL_PATH}"
echo "    PPO_DEPLOYMENT_MODE     = ${PPO_DEPLOYMENT_MODE}"
echo "    PPO_ONLINE_UPDATES      = ${PPO_ONLINE_UPDATES_ENABLED}"

# ── Step 4: Start Docker workers + observability ─────────────────────────────
separator "Step 4: Starting Docker workers (host-master topology)"

if [[ "${WORKERS_HEALTHY}" == "true" ]]; then
    ok "Reusing existing healthy workers (no restart needed)"
else
    # Check if port 27018 is already in use by something else
    if docker ps 2>/dev/null | grep -q "27018"; then
        info "MongoDB already running on :27018 — starting workers without testbench mongo"
        docker compose -f "${COMPOSE_FILE}" up -d --scale mongo=0
    else
        info "Starting stack (mongo, workers, prometheus, grafana)..."
        docker compose -f "${COMPOSE_FILE}" up -d
    fi
    ok "Docker workers started"
fi

# ── Step 5: Start local master node ──────────────────────────────────────────
separator "Step 5: Starting local master node"
start_local_master

# ── Step 6: Register workers ────────────────────────────────────────────────
separator "Step 6: Registering workers"
register_workers
ok "Workers registered and active"

# ── Step 7: Prepare workflow images ──────────────────────────────────────────
separator "Step 7: Preparing workflow images"
testbench/scripts/prepare_workflow_images.sh
ok "Workflow images ready"

# ── Step 8: Run benchmark campaign ───────────────────────────────────────────
separator "Step 8: Running benchmark campaign (mode: ${CAMPAIGN_MODE})"

CAMPAIGN_ARGS=("--scenarios" "all")
RESULTS_DIR="results/campaign-$(date +%Y%m%d-%H%M%S)"
ISOLATED_WORKLOAD_LIST=(heterogeneous-smoke steady-cpu bursty memory-pressure)

if [[ "${CAMPAIGN_MODE}" == "comprehensive" ]]; then
    CAMPAIGN_ARGS+=("--workloads" "heterogeneous-smoke,steady-cpu,bursty,memory-pressure")
    CAMPAIGN_ARGS+=("--timeout" "900")
elif [[ "${CAMPAIGN_MODE}" == "full" ]]; then
    CAMPAIGN_ARGS+=("--workloads" "heterogeneous-smoke,steady-cpu,steady-mixed,memory-pressure,bursty,long-tail")
else
    CAMPAIGN_ARGS+=("--workloads" "heterogeneous-smoke")
fi

CAMPAIGN_ARGS+=("--output-dir" "${RESULTS_DIR}")

VENV_PYTHON="${SCRIPT_DIR}/venv/bin/python3"
if [[ ! -f "${VENV_PYTHON}" ]]; then
    VENV_PYTHON="python3"
    info "Venv python not found, using system python3"
fi

info "Results will be saved to: ${RESULTS_DIR}"
if [[ "${CAMPAIGN_MODE}" == "isolated" ]]; then
    info "Isolated workload mode: restart master + reset model for each workload"
    for workload in "${ISOLATED_WORKLOAD_LIST[@]}"; do
        separator "Isolated run for workload: ${workload}"

        cp "${MODEL_SRC}" "${MODEL_DST}"
        ok "Reset active model from frozen checkpoint for ${workload}"

        stop_local_master
        start_local_master

        register_workers
        ok "Workers registered and active"

        WORKLOAD_RESULTS_DIR="${RESULTS_DIR}/${workload}"
        WORKLOAD_ARGS=(
            "--scenarios" "all"
            "--workloads" "${workload}"
            "--timeout" "900"
            "--output-dir" "${WORKLOAD_RESULTS_DIR}"
        )
        "${VENV_PYTHON}" testbench/scripts/run_campaign.py "${WORKLOAD_ARGS[@]}"
        ok "Campaign completed for ${workload}"
    done
else
    "${VENV_PYTHON}" testbench/scripts/run_campaign.py "${CAMPAIGN_ARGS[@]}"
    ok "Campaign completed"
fi

# ── Step 9: Generate benchmark report ────────────────────────────────────────
separator "Step 9: Generating benchmark report"

if [[ "${CAMPAIGN_MODE}" == "isolated" ]]; then
    for workload in "${ISOLATED_WORKLOAD_LIST[@]}"; do
        WORKLOAD_RESULTS_DIR="${RESULTS_DIR}/${workload}"
        CAMPAIGN_SUBDIR="$(find "${WORKLOAD_RESULTS_DIR}" -maxdepth 1 -type d ! -path "${WORKLOAD_RESULTS_DIR}" | sort | tail -1)"
        if [[ -z "${CAMPAIGN_SUBDIR}" ]]; then
            warn "No campaign subdirectory found for ${workload}, skipping report generation"
            continue
        fi
        "${VENV_PYTHON}" scripts/generate_benchmark_report.py \
            --campaign-dir "${CAMPAIGN_SUBDIR}" \
            --master-url "${MASTER_URL}" \
            --model-path "${MODEL_DST}"
        ok "Benchmark report generated for ${workload}"
    done
else
    # Find the timestamped subdirectory created by run_campaign.py
    CAMPAIGN_SUBDIR="$(find "${RESULTS_DIR}" -maxdepth 1 -type d ! -path "${RESULTS_DIR}" | sort | tail -1)"
    if [[ -z "${CAMPAIGN_SUBDIR}" ]]; then
        CAMPAIGN_SUBDIR="${RESULTS_DIR}"
    fi

    "${VENV_PYTHON}" scripts/generate_benchmark_report.py \
        --campaign-dir "${CAMPAIGN_SUBDIR}" \
        --master-url "${MASTER_URL}" \
        --model-path "${MODEL_DST}"
    ok "Benchmark report generated"
fi

# ── Step 10: Summary ─────────────────────────────────────────────────────────
separator "Done!"

echo "Results directory: ${RESULTS_DIR}"
echo ""
if [[ -d "${RESULTS_DIR}" ]]; then
    echo "Result files:"
    find "${RESULTS_DIR}" -type f -name "*.json" -o -name "*.md" 2>/dev/null | head -20 | sed 's/^/  /'
fi
echo ""
echo "Docker workers are still running. To tear down:"
echo "  ./execute-tests.sh --teardown"
echo ""
echo "To view Grafana dashboards: http://localhost:3300 (admin/${GF_ADMIN_PASSWORD})"
echo "To view Prometheus:         http://localhost:9090"

TEARDOWN_DONE=true
