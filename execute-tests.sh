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
CAMPAIGN_MODE="smoke" # "smoke", "full", "comprehensive", "isolated", or "alibaba-test"
SKIP_BUILD=false
TEARDOWN_ONLY=false
MASTER_URL="http://localhost:8080"
COMPOSE_FILE="testbench/docker-compose.host-master.yml"
WORKER_SPECS="worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054"
MASTER_PID=""
MASTER_BIN="master/masterNode"
ALIBABA_TEST_SOURCE_DIR="${ALIBABA_TEST_SOURCE_DIR:-agentic_scheduler/data/alibaba_v2018/core}"
ALIBABA_TEST_TRACE_DIR="${ALIBABA_TEST_TRACE_DIR:-agentic_scheduler/data/alibaba_v2018/alibaba_test}"
ALIBABA_TEST_TASKS="${ALIBABA_TEST_TASKS:-300000}"
ALIBABA_TEST_START_ROW="${ALIBABA_TEST_START_ROW:-0}"
ALIBABA_TEST_TASKS_PER_WORKLOAD="${ALIBABA_TEST_TASKS_PER_WORKLOAD:-40}"

# Ensure GF_ADMIN_PASSWORD is always set (required by docker-compose, even for teardown)
export GF_ADMIN_PASSWORD="${GF_ADMIN_PASSWORD:-password}"
export MONGO_PASSWORD="${MONGO_PASSWORD:-cloudai-stress-test}"
export MONGO_USERNAME="${MONGO_USERNAME:-cloudai}"

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
    --alibaba-test)
        CAMPAIGN_MODE="alibaba-test"
        shift
        ;;
    --alibaba-test-tasks)
        ALIBABA_TEST_TASKS="$2"
        shift 2
        ;;
    --alibaba-test-start-row)
        ALIBABA_TEST_START_ROW="$2"
        shift 2
        ;;
    --alibaba-test-tasks-per-workload)
        ALIBABA_TEST_TASKS_PER_WORKLOAD="$2"
        shift 2
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
        echo "  --alibaba-test    Generate/use large Alibaba test split and run derived campaign"
        echo "  --alibaba-test-tasks <N>   Number of tasks in Alibaba test split (default: ${ALIBABA_TEST_TASKS})"
        echo "  --alibaba-test-start-row <N>  1-based start row for contiguous split (default: auto-tail)"
        echo "  --alibaba-test-tasks-per-workload <N>  Tasks per generated workload (default: ${ALIBABA_TEST_TASKS_PER_WORKLOAD})"
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
info()  { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()    { echo -e "\033[1;32m[OK]\033[0m    $*"; }
warn()  { echo -e "\033[1;33m[WARN]\033[0m  $*"; }
fail()  { echo -e "\033[1;31m[FAIL]\033[0m  $*" >&2; exit 1; }

separator() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  $*"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

cleanup() {
    if [[ -n "${MASTER_PID}" ]]; then
        info "Stopping local master (PID ${MASTER_PID})..."
        kill "${MASTER_PID}" 2>/dev/null || true
        wait "${MASTER_PID}" 2>/dev/null || true
        ok "Master stopped"
    fi
    if [[ "${TEARDOWN_DONE:-false}" != "true" ]]; then
        warn "Script interrupted — Docker workers may still be running."
        warn "Run './execute-tests.sh --teardown' to stop them."
    fi
}
trap cleanup EXIT

# ── Teardown mode ────────────────────────────────────────────────────────────
if [[ "${TEARDOWN_ONLY}" == "true" ]]; then
    separator "Tearing down testbench stack"
    docker compose -f "${COMPOSE_FILE}" down --remove-orphans
    TEARDOWN_DONE=true
    ok "Testbench stack stopped and removed."
    exit 0
fi

# ── Pre-flight checks ───────────────────────────────────────────────────────
separator "Pre-flight checks"

command -v docker >/dev/null 2>&1  || fail "docker is not installed"
command -v python3 >/dev/null 2>&1 || fail "python3 is not installed"
docker info >/dev/null 2>&1       || fail "Docker daemon is not running"

if [[ ! -f "${MODEL_SRC}" ]]; then
    fail "Model checkpoint not found: ${MODEL_SRC}"
fi

ok "All pre-flight checks passed"

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
export PPO_ONLINE_UPDATES_ENABLED=false

ok "Environment configured:"
echo "    SCHED_ALGO              = ${SCHED_ALGO}"
echo "    PPO_AUTOSTART           = ${PPO_AUTOSTART}"
echo "    PPO_MODEL_PATH          = ${PPO_MODEL_PATH}"
echo "    PPO_DEPLOYMENT_MODE     = ${PPO_DEPLOYMENT_MODE}"
echo "    PPO_ONLINE_UPDATES      = ${PPO_ONLINE_UPDATES_ENABLED}"

# ── Step 4: Start Docker workers + observability ─────────────────────────────
separator "Step 4: Starting Docker workers (host-master topology)"

# Check if MongoDB is already running on :27017 (e.g. from database/docker-compose.yml)
MONGO_ALREADY_RUNNING=false
if curl -fsS --max-time 2 "mongodb://localhost:27017" >/dev/null 2>&1 \
   || docker ps 2>/dev/null | grep -q "27017"; then
    MONGO_ALREADY_RUNNING=true
fi

# Reuse existing containers — only rebuild if source code changed.
# `up -d` is idempotent: starts stopped containers, skips already-running ones.
if [[ "${MONGO_ALREADY_RUNNING}" == "true" ]]; then
    info "MongoDB already running on :27017 — starting workers without testbench mongo"
    docker compose -f "${COMPOSE_FILE}" up -d --scale mongo=0
else
    info "Starting stack (mongo, workers, prometheus, grafana)..."
    docker compose -f "${COMPOSE_FILE}" up -d
fi

ok "Docker workers started"

# ── Step 5: Start local master node ──────────────────────────────────────────
separator "Step 5: Starting local master node"

# Build master binary path
MASTER_BIN="master/masterNode"
if [[ ! -f "${MASTER_BIN}" ]]; then
    fail "Master binary not found at ${MASTER_BIN}. Run without --skip-build."
fi

info "Launching master node locally (PPO model updates write to local .pt)..."
CLOUDAI_HEADLESS=true \
MONGODB_HOST=localhost:27017 \
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

# Wait for master to become healthy
info "Waiting for master API to become reachable..."
MAX_WAIT=60
for i in $(seq 1 "${MAX_WAIT}"); do
    if curl -fsS "${MASTER_URL}/health" >/dev/null 2>&1; then
        break
    fi
    if ! kill -0 "${MASTER_PID}" 2>/dev/null; then
        fail "Master process exited unexpectedly"
    fi
    if [[ $i -eq ${MAX_WAIT} ]]; then
        fail "Master API did not become ready after ${MAX_WAIT}s"
    fi
    sleep 1
done
ok "Master API is up at ${MASTER_URL} (PID ${MASTER_PID})"

# ── Step 6: Register workers ────────────────────────────────────────────────
separator "Step 6: Registering workers"
MASTER_URL="${MASTER_URL}" \
WORKER_SPECS="${WORKER_SPECS}" \
    testbench/scripts/register_workers.sh
ok "Workers registered and active"

# ── Step 7: Prepare workflow images ──────────────────────────────────────────
separator "Step 7: Preparing workflow images"
testbench/scripts/prepare_workflow_images.sh
ok "Workflow images ready"

# ── Step 8: Run benchmark campaign ───────────────────────────────────────────
separator "Step 8: Running benchmark campaign (mode: ${CAMPAIGN_MODE})"

CAMPAIGN_ARGS=("--scenarios" "all")
RESULTS_DIR="results/campaign-$(date +%Y%m%d-%H%M%S)"

if [[ "${CAMPAIGN_MODE}" == "comprehensive" ]]; then
    CAMPAIGN_ARGS+=("--workloads" "heterogeneous-smoke,steady-cpu,bursty,memory-pressure")
    CAMPAIGN_ARGS+=("--timeout" "900")
elif [[ "${CAMPAIGN_MODE}" == "alibaba-test" ]]; then
    separator "Preparing Alibaba test split and workloads"

    "${VENV_PYTHON}" agentic_scheduler/scripts/create_alibaba_test_split.py \
        --source-trace-dir "${ALIBABA_TEST_SOURCE_DIR}" \
        --dest-trace-dir "${ALIBABA_TEST_TRACE_DIR}" \
        --task-count "${ALIBABA_TEST_TASKS}" \
        --start-row "${ALIBABA_TEST_START_ROW}" \
        --force

    "${VENV_PYTHON}" testbench/scripts/generate_alibaba_test_workloads.py \
        --trace-dir "${ALIBABA_TEST_TRACE_DIR}" \
        --output-dir testbench/workloads \
        --tasks-per-workload "${ALIBABA_TEST_TASKS_PER_WORKLOAD}"

    CAMPAIGN_ARGS+=("--workloads" "alibaba-test-cpu,alibaba-test-memory,alibaba-test-mixed,alibaba-test-bursty")
    CAMPAIGN_ARGS+=("--timeout" "1200")
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
"${VENV_PYTHON}" testbench/scripts/run_campaign.py "${CAMPAIGN_ARGS[@]}"
ok "Campaign completed"

# ── Step 9: Generate benchmark report ────────────────────────────────────────
separator "Step 9: Generating benchmark report"

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
