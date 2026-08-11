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

# ── Cleanup trap (registered before any background processes) ─────────────────
UI_PID=""
UI_PGID=""

cleanup() {
    echo ""
    echo "Shutting down..."

    # Kill the entire UI process group (deferred-launcher subshell + npm + vite)
    if [[ -n "${UI_PGID:-}" && "${UI_PGID}" != "0" ]]; then
        kill -- "-${UI_PGID}" 2>/dev/null || true
    fi
    [[ -n "${UI_PID:-}" ]] && kill "${UI_PID}" 2>/dev/null || true
    [[ -n "${UI_PID:-}" ]] && wait "${UI_PID}" 2>/dev/null || true

    # Force-kill any surviving vite/npm/node processes
    SURVIVORS="$(pgrep -f "vite\|npm.*dev\|node.*vite" 2>/dev/null || true)"
    [[ -n "$SURVIVORS" ]] && echo "$SURVIVORS" | xargs kill -9 2>/dev/null || true

    # Free PPO and UI ports (master and gRPC ports belong to masterNode, which cleans up itself)
    for _PORT in 50050 "${UI_PORT}"; do
        _PID="$(lsof -ti :"${_PORT}" -sTCP:LISTEN 2>/dev/null || true)"
        [[ -n "$_PID" ]] && kill -9 "$_PID" 2>/dev/null || true
    done

    echo "✓ Shutdown complete"
}

trap cleanup EXIT INT TERM

# ── Build ─────────────────────────────────────────────────────────────────────
make master

if [ ! -f "master/masterNode" ]; then
    echo "Error: masterNode binary not found. Please run 'make master' first."
    exit 1
fi

# ── UI port check (interactive — must happen before master takes over stdin) ───
if lsof -Pi :"$UI_PORT" -sTCP:LISTEN -t >/dev/null 2>&1; then
    EXISTING_PID=$(lsof -t -i :"$UI_PORT" 2>/dev/null | head -1)
    echo "⚠️  Port $UI_PORT is already in use (PID: $EXISTING_PID)"
    read -p "Kill existing process? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kill -9 "$EXISTING_PID" 2>/dev/null || true
        sleep 1
        echo "✓ Killed existing process on port $UI_PORT"
    else
        echo "Using alternative port..."
        UI_PORT=$((UI_PORT + 1))
        while lsof -Pi :"$UI_PORT" -sTCP:LISTEN -t >/dev/null 2>&1; do
            UI_PORT=$((UI_PORT + 1))
        done
        echo "  Will use port $UI_PORT instead"
    fi
fi

# ── Deferred Vite launcher — polls :8080, starts npm only once master is ready ─
# This eliminates ECONNREFUSED proxy spam. Runs in background; master stays
# foreground so it retains stdin for the interactive CLI.
REPO_ROOT="$(pwd)"
(
    _W=0
    while ! lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1; do
        sleep 0.5
        _W=$((_W + 1))
        [[ $_W -ge 120 ]] && break  # 60 s hard timeout
    done
    cd "${REPO_ROOT}/ui" || exit 1
    exec npm run dev -- --port "${UI_PORT}" --strictPort
) &
UI_PID=$!
UI_PGID=$(ps -o pgid= -p "$UI_PID" 2>/dev/null | tr -d ' ') || UI_PGID=""

# ── Start master in foreground (owns stdin for interactive CLI) ───────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Starting Master Node..."
echo "Master gRPC:  :50051"
echo "Master HTTP:  :8080"
echo "Scheduler:    ${SCHED_ALGO:-RTS}"
echo "Web UI:       http://localhost:${UI_PORT} (starts once master is ready)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "To register workers from other machines:"
echo "  curl -X POST http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo '<master-ip>'):8080/api/workers \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"worker_id\":\"worker-1\",\"worker_ip\":\"<worker-ip>:50052\"}'"
echo ""

cd master
./masterNode --mode "$UI_MODE"
# masterNode exiting (normally or via signal) triggers the EXIT trap → cleanup
