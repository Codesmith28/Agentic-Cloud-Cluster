#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# execute-tests.sh — End-to-end deployment and benchmark runner
#
# Deploys the trained PPO model into the live testbench, starts the full
# Docker stack (Mongo, master, workers, observability), registers workers,
# and runs the evidence benchmark campaign (PPO vs RTS vs RR).
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
CAMPAIGN_MODE="smoke"       # "smoke" or "full"
SKIP_BUILD=false
TEARDOWN_ONLY=false
MASTER_URL="http://localhost:8080"

# ── Parse arguments ─────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --full)
            CAMPAIGN_MODE="full"
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
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --full          Run full campaign (all workloads + scenarios)"
            echo "  --model <path>  Path to .pt model checkpoint (default: $MODEL_SRC)"
            echo "  --skip-build    Skip building master/worker binaries"
            echo "  --teardown      Only tear down the testbench stack"
            echo "  -h, --help      Show this help"
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
    if [[ "${TEARDOWN_DONE:-false}" != "true" ]]; then
        warn "Script interrupted — testbench stack is still running."
        warn "Run './execute-tests.sh --teardown' to stop it."
    fi
}
trap cleanup EXIT

# ── Teardown mode ────────────────────────────────────────────────────────────
if [[ "${TEARDOWN_ONLY}" == "true" ]]; then
    separator "Tearing down testbench stack"
    docker compose -f testbench/docker-compose.yml down --remove-orphans
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
separator "Step 1: Deploying trained model"

mkdir -p "$(dirname "${MODEL_DST}")"
cp "${MODEL_SRC}" "${MODEL_DST}"
ok "Copied model: ${MODEL_SRC} → ${MODEL_DST}"

# ── Step 2: Build binaries ──────────────────────────────────────────────────
if [[ "${SKIP_BUILD}" == "true" ]]; then
    info "Skipping build (--skip-build)"
else
    separator "Step 2: Building master and worker"
    make master
    make worker
    ok "Binaries built"
fi

# ── Step 3: Set environment for PPO ─────────────────────────────────────────
separator "Step 3: Configuring PPO environment"

export SCHED_ALGO=PPO
export PPO_AUTOSTART=true
export PPO_MODEL_PATH="${MODEL_DST}"
export PPO_DEPLOYMENT_MODE=active

# Ensure Grafana .env var is set (required by docker-compose)
if [[ -z "${GF_ADMIN_PASSWORD:-}" ]]; then
    export GF_ADMIN_PASSWORD="benchpass"
    info "Set GF_ADMIN_PASSWORD=benchpass (default for benchmarking)"
fi

ok "Environment configured:"
echo "    SCHED_ALGO        = ${SCHED_ALGO}"
echo "    PPO_AUTOSTART     = ${PPO_AUTOSTART}"
echo "    PPO_MODEL_PATH    = ${PPO_MODEL_PATH}"
echo "    PPO_DEPLOYMENT_MODE = ${PPO_DEPLOYMENT_MODE}"

# ── Step 4: Start testbench stack ────────────────────────────────────────────
separator "Step 4: Starting testbench Docker stack"

# Bring down any existing stack first
info "Cleaning up any previous testbench stack..."
docker compose -f testbench/docker-compose.yml down --remove-orphans 2>/dev/null || true

info "Starting stack (mongo, master, workers, observability)..."
docker compose -f testbench/docker-compose.yml up -d --build

# Wait for master to become healthy
info "Waiting for master API to become reachable..."
MAX_WAIT=60
for i in $(seq 1 "${MAX_WAIT}"); do
    if curl -fsS "${MASTER_URL}/telemetry" >/dev/null 2>&1; then
        break
    fi
    if [[ $i -eq ${MAX_WAIT} ]]; then
        fail "Master API did not become ready after ${MAX_WAIT}s"
    fi
    sleep 1
done
ok "Master API is up at ${MASTER_URL}"

# ── Step 5: Register workers ────────────────────────────────────────────────
separator "Step 5: Registering workers"
testbench/scripts/register_workers.sh
ok "Workers registered and active"

# ── Step 6: Prepare workflow images ──────────────────────────────────────────
separator "Step 6: Preparing workflow images"
testbench/scripts/prepare_workflow_images.sh
ok "Workflow images ready"

# ── Step 7: Run benchmark campaign ───────────────────────────────────────────
separator "Step 7: Running benchmark campaign (mode: ${CAMPAIGN_MODE})"

CAMPAIGN_ARGS=("--scenarios" "all")
RESULTS_DIR="results/campaign-$(date +%Y%m%d-%H%M%S)"

if [[ "${CAMPAIGN_MODE}" == "full" ]]; then
    CAMPAIGN_ARGS+=("--workloads" "heterogeneous-smoke,steady-cpu,steady-mixed,memory-pressure,bursty,long-tail,failure-stressed")
else
    CAMPAIGN_ARGS+=("--workloads" "heterogeneous-smoke")
fi

CAMPAIGN_ARGS+=("--output-dir" "${RESULTS_DIR}")

info "Results will be saved to: ${RESULTS_DIR}"
python3 testbench/scripts/run_campaign.py "${CAMPAIGN_ARGS[@]}"
ok "Campaign completed"

# ── Step 8: Summary ─────────────────────────────────────────────────────────
separator "Done!"

echo "Results directory: ${RESULTS_DIR}"
echo ""
if [[ -d "${RESULTS_DIR}" ]]; then
    echo "Result files:"
    find "${RESULTS_DIR}" -type f -name "*.json" -o -name "*.md" 2>/dev/null | head -20 | sed 's/^/  /'
fi
echo ""
echo "Testbench stack is still running. To tear down:"
echo "  ./execute-tests.sh --teardown"
echo ""
echo "To view Grafana dashboards: http://localhost:3000 (admin/${GF_ADMIN_PASSWORD})"
echo "To view Prometheus:         http://localhost:9090"

TEARDOWN_DONE=true
