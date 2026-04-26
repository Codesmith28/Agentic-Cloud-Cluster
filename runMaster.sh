#!/bin/bash
set -euo pipefail

# Usage: ./runMaster.sh [cli|tui] [--ppo]
#
# This script builds and launches the master node with optional PPO scheduling.
# Workers on other machines connect via gRPC on :50051.
#
# Options:
#   cli|tui     UI mode (default: cli)
#   --ppo       Enable PPO scheduler (loads latest trained model)
#
# Quick start:
#   ./runMaster.sh              # Start with RTS scheduler
#   ./runMaster.sh --ppo        # Start with PPO scheduler
#   ./runMaster.sh tui --ppo    # TUI mode + PPO
#
# Then on worker machines:
#   ./runWorker.sh
#   # Use the displayed address to register:
#   curl -X POST http://<master-ip>:8080/api/workers \
#     -H 'Content-Type: application/json' \
#     -d '{"worker_id":"worker-1","worker_ip":"<worker-ip>:50052"}'

UI_MODE="cli"
ENABLE_PPO=false

for arg in "$@"; do
    case "$arg" in
        cli|tui) UI_MODE="$arg" ;;
        --ppo)   ENABLE_PPO=true ;;
        --help|-h)
            echo "Usage: $0 [cli|tui] [--ppo]"
            echo ""
            echo "  cli|tui   UI mode (default: cli)"
            echo "  --ppo     Enable PPO scheduler with latest trained model"
            echo ""
            echo "Environment variables (all optional):"
            echo "  SCHED_ALGO          Scheduler: RTS (default), PPO, RR"
            echo "  PPO_MODEL_PATH      Path to .pt model (default: auto-detect latest)"
            echo "  PPO_AUTOSTART       Auto-start Python gRPC service (default: true)"
            echo "  MONGODB_HOST        MongoDB host (default: localhost:27017)"
            echo "  MONGODB_USERNAME    MongoDB user (default: cloudai)"
            echo "  MONGODB_PASSWORD    MongoDB password (default: from .env)"
            echo "  WEBUI_PORT          Frontend dev port (default: 3001)"
            exit 0
            ;;
    esac
done

echo "Starting Master Node (mode: ${UI_MODE})"
UI_PORT="${WEBUI_PORT:-3001}"

# ── MongoDB ──────────────────────────────────────────────────────────────────
if ! docker ps 2>/dev/null | grep -q mongo; then
    echo "Starting MongoDB..."
    if [[ -f database/docker-compose.yml ]]; then
        # Set MONGO_PASSWORD if not already set (required by database compose)
        export MONGO_PASSWORD="${MONGO_PASSWORD:-cloudai-stress-test}"
        docker compose -f database/docker-compose.yml up -d
        echo "✓ MongoDB started"
        # Wait for healthy
        echo -n "  Waiting for MongoDB..."
        for i in $(seq 1 30); do
            if docker exec cloudai-mongo mongosh --quiet --eval "db.runCommand({ping:1}).ok" 2>/dev/null | grep -q 1; then
                echo " ready"
                break
            fi
            if [[ $i -eq 30 ]]; then
                echo " timeout (continuing anyway)"
            fi
            sleep 1
        done
    else
        echo "⚠️  Warning: MongoDB not running and database/docker-compose.yml not found."
        echo "   Start MongoDB manually for persistent storage."
    fi
else
    echo "✓ MongoDB already running"
fi
echo ""

# ── PPO configuration ────────────────────────────────────────────────────────
if [[ "${ENABLE_PPO}" == "true" ]]; then
    export SCHED_ALGO="${SCHED_ALGO:-PPO}"
    export PPO_AUTOSTART="${PPO_AUTOSTART:-true}"
    export PPO_MODEL_PATH="${PPO_MODEL_PATH:-latest}"
    export PPO_DEPLOYMENT_MODE="${PPO_DEPLOYMENT_MODE:-active}"
    export PPO_ONLINE_UPDATES_ENABLED="${PPO_ONLINE_UPDATES_ENABLED:-true}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  PPO Scheduling Enabled"
    echo "  Model: ${PPO_MODEL_PATH}"
    echo "  Mode:  ${PPO_DEPLOYMENT_MODE}"
    echo "  Online updates: ${PPO_ONLINE_UPDATES_ENABLED}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
fi

# ── Frontend ─────────────────────────────────────────────────────────────────
echo "Starting UI (npm run dev) in background..."
(
    cd ui || exit
    npm run dev -- --port "$UI_PORT" --strictPort
) &
UI_PID=$!
echo "Frontend started on port $UI_PORT (PID: $UI_PID)"
echo "Web UI URL: http://localhost:$UI_PORT"

# Function to safely cleanup UI
cleanup() {
    echo ""
    echo "Shutting down UI server..."

    # Kill ONLY the npm process we started
    kill "$UI_PID" 2>/dev/null
    wait "$UI_PID" 2>/dev/null

    echo "✓ UI server stopped"

    PIDS="$(pgrep -f "ui/.*(node|vite|npm)" 2>/dev/null || true)"
    if [ -n "$PIDS" ]; then
        echo "$PIDS" | xargs kill -9 2>/dev/null || true
    fi

    exit 0
}

# Trap EXIT, INT, TERM signals
trap cleanup EXIT INT TERM

# Build master node
make master

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Starting Master Node..."
echo "Master gRPC:  :50051"
echo "Master HTTP:  :8080"
echo "Scheduler:    ${SCHED_ALGO:-RTS}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To register workers from other machines:"
echo "  curl -X POST http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo '<master-ip>'):8080/api/workers \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"worker_id\":\"worker-1\",\"worker_ip\":\"<worker-ip>:50052\"}'"
echo ""

# Change to master directory
cd master

# Check if binary exists
if [ ! -f "masterNode" ]; then
    echo "Error: masterNode binary not found. Please run 'make master' first."
    exit 1
fi

# Start the master node
echo "Launching master node (mode: $UI_MODE)..."
./masterNode --mode "$UI_MODE"

# After master exits, cleanup will be called automatically
