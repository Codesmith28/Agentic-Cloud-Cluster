#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — Worker Node Launcher
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Source shared helpers
# shellcheck source=../_common.sh
source "${SCRIPT_DIR}/../_common.sh"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Agentic Cloud Cluster Worker Node"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check Docker runtime
if ! docker info >/dev/null 2>&1; then
    warn "Docker is not running. Task execution will fail."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "   Start Docker Desktop, then retry."
    else
        echo "   Start Docker first: sudo systemctl start docker"
    fi
    echo ""
fi

# Build worker
log "Building worker node..."
make -C "${REPO_ROOT}" worker

if [ ! -f "${REPO_ROOT}/worker/workerNode" ]; then
    die "workerNode binary not found. Please run 'make worker' first."
fi

ok "Worker built successfully"
log "Launching worker node..."
echo ""

cd "${REPO_ROOT}/worker"
./workerNode
