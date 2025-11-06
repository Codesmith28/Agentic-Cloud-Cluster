#!/bin/bash

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
    echo "   Start Docker first: sudo systemctl start docker"
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