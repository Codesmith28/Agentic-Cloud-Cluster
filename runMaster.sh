#!/bin/bash

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
