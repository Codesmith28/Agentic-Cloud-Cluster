#!/bin/bash

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

# Usage: ./runMaster.sh
#
# This script builds and launches the master node.
# After master starts, use the CLI to register workers:
#   master> register <worker_id> <worker_ip:port>

echo "Starting Master Node from script"

# Check if MongoDB is running (optional but recommended)
if ! docker ps | grep -q mongo; then
    echo "⚠️  Warning: MongoDB not detected. Start it for persistent storage:"
    echo "   cd database && docker-compose up -d"
    echo ""
fi

# Start the frontend in the background
echo "Starting UI (npm run dev) in background..."
(
    cd ui || exit
    npm run dev
) &
UI_PID=$!
echo "Frontend started on port 3000 (PID: $UI_PID)"

# Function to safely cleanup UI
cleanup() {
    echo ""
    echo "Shutting down UI server..."

    # Kill ONLY the npm process we started
    kill "$UI_PID" 2>/dev/null
    wait "$UI_PID" 2>/dev/null

    echo "✓ UI server stopped"

    pgrep -f "ui/.*(node|vite|npm)" | xargs -r kill -9 2>/dev/null

    exit 0
}

# Trap EXIT, INT, TERM signals
trap cleanup EXIT INT TERM

# Build master node
make master

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Starting Master Node..."
echo "Master will listen on: :50051"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Change to master directory
cd master

# Check if binary exists
if [ ! -f "masterNode" ]; then
    echo "Error: masterNode binary not found. Please run 'make master' first."
    exit 1
fi

# Start the master node
echo "Launching master node..."
./masterNode

# After master exits, cleanup will be called automatically
