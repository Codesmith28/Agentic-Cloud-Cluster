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

# Usage: ./runWorker.sh
#
# This script builds and launches a worker node.
# The worker will auto-detect its IP address and find an available port.
# After starting, use the displayed worker details to register it with the master.

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  CloudAI Worker Node - Launch Script"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if Docker is running (required for task execution)
if ! docker info >/dev/null 2>&1; then
    echo "⚠️  Warning: Docker is not running. Task execution will fail."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "   Start Docker Desktop, then retry."
    else
        echo "   Start Docker first: sudo systemctl start docker"
    fi
    echo ""
fi

# Build the worker node
echo "Building worker node..."
make worker

echo ""

# Change to worker directory
cd worker

# Check if binary exists
if [ ! -f "workerNode" ]; then
    echo "❌ Error: workerNode binary not found."
    echo "   Please run 'make worker' first."
    exit 1
fi

# Start the worker node (no arguments needed - auto-detects everything)
echo "Launching worker node..."
echo ""
./workerNode
