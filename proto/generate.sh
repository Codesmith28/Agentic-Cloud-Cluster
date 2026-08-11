#!/bin/bash


# Generate Go and Python code from proto files
# Run this script from the proto directory or project root

set -e

export PATH=$PATH:$(go env GOPATH)/bin

echo "Generating gRPC code from proto files..."

# Create output directories for generated code
mkdir -p ./pb          # Go code
mkdir -p ./py          # Python code

echo "-> Generating Go code for master_worker.proto (Go <-> Go)..."
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    master_worker.proto

echo "-> Generating Go code for master_agent.proto (Master side -> Go)..."
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    master_agent.proto

echo "-> Generating Go code for ppo_scheduler.proto (Master <-> Python PPO)..."
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    ppo_scheduler.proto

# Check if venv exists (assuming it's in the project root)
if [ -f "../venv/bin/activate" ]; then
    echo "🐍 Activating Python virtual environment..."
    source ../venv/bin/activate
fi

echo "-> Generating Python code for master_worker.proto (shared messages)..."
python3 -m grpc_tools.protoc \
    --python_out=./py \
    --grpc_python_out=./py \
    --proto_path=. \
    master_worker.proto

echo "-> Generating Python code for master_agent.proto (Agent side -> Python)..."
python3 -m grpc_tools.protoc \
    --python_out=./py \
    --grpc_python_out=./py \
    --proto_path=. \
    master_agent.proto

echo "-> Generating Python code for ppo_scheduler.proto (PPO side -> Python)..."
python3 -m grpc_tools.protoc \
    --python_out=./py \
    --grpc_python_out=./py \
    --proto_path=. \
    ppo_scheduler.proto

# Create __init__.py to make py directory a Python package
echo "-> Creating Python package __init__.py..."
touch ./py/__init__.py

# Fix imports in generated Python files to use relative imports
echo "-> Fixing Python imports for package compatibility..."
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' 's/^import master_worker_pb2/from . import master_worker_pb2/g' ./py/master_worker_pb2_grpc.py
    sed -i '' 's/^import master_agent_pb2/from . import master_agent_pb2/g' ./py/master_agent_pb2_grpc.py
    sed -i '' 's/^import ppo_scheduler_pb2/from . import ppo_scheduler_pb2/g' ./py/ppo_scheduler_pb2_grpc.py
    sed -i '' 's/^import master_worker_pb2 as/from . import master_worker_pb2 as/g' ./py/ppo_scheduler_pb2.py
else
    # Linux
    sed -i 's/^import master_worker_pb2/from . import master_worker_pb2/g' ./py/master_worker_pb2_grpc.py
    sed -i 's/^import master_agent_pb2/from . import master_agent_pb2/g' ./py/master_agent_pb2_grpc.py
    sed -i 's/^import ppo_scheduler_pb2/from . import ppo_scheduler_pb2/g' ./py/ppo_scheduler_pb2_grpc.py
    sed -i 's/^import master_worker_pb2 as/from . import master_worker_pb2 as/g' ./py/ppo_scheduler_pb2.py
fi

echo "✓ gRPC code generation complete!"
echo "  Go files:     ./pb/"
echo "  Python files: ./py/"
