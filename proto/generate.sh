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

# Generate Go and Python code from proto files
# Run this script from the proto directory or project root

set -e

echo "Generating gRPC code from proto files..."

# Create output directories for generated code
mkdir -p ./pb          # Go code (master_worker)
mkdir -p ./py          # Python code

echo "-> Generating Go code for master_worker.proto (Go <-> Go)..."
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    master_worker.proto

echo "-> Generating Go code for master_agent.proto (Master side - Go)..."
protoc --go_out=./pb --go_opt=paths=source_relative \
    --go-grpc_out=./pb --go-grpc_opt=paths=source_relative \
    master_agent.proto

echo "-> Generating Python code for master_agent.proto (Agent side - Python)..."
python3 -m grpc_tools.protoc \
    --python_out=./py \
    --grpc_python_out=./py \
    --proto_path=. \
    master_agent.proto

echo "✓ gRPC code generation complete!"
echo "  Go files:     ./pb/"
echo "  Python files: ./py/"
