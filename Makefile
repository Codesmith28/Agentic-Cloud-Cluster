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

.PHONY: help all proto master worker clean setup test test-unit test-unit-verbose testbench-up testbench-prepare-images testbench-register testbench-workload testbench-suite testbench-down

# Default target
help:
	@echo "CloudAI Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make all          - Build everything (setup + master + worker)"
	@echo "  make proto        - Generate gRPC code from proto files"
	@echo "  make master       - Build master node"
	@echo "  make worker       - Build worker node"
	@echo "  make setup        - Complete setup (proto + symlinks + deps)"
	@echo "  make clean        - Clean generated files and binaries"
	@echo "  make test         - Run basic connectivity tests"
	@echo "  make test-unit    - Run Go unit tests (all packages)"
	@echo "  make test-unit-verbose - Run Go unit tests with verbose output"
	@echo "  make testbench-up - Build and start Docker testbench stack"
	@echo "  make testbench-prepare-images - Build deterministic workflow images into worker DinD daemons"
	@echo "  make testbench-register - Register testbench workers with master"
	@echo "  make testbench-workload - Submit default workload and wait for completion"
	@echo "  make testbench-suite - Start stack, register workers, run workload"
	@echo "  make testbench-down - Stop and remove Docker testbench stack"
	@echo ""
	@echo "Quick start:"
	@echo "  make setup        # One-time setup"
	@echo "  make all          # Build everything"
	@echo "  make master       # Build master"
	@echo "  make worker       # Build worker"

# Build everything
all: setup master worker

# Generate gRPC code
proto:
	@echo "🔧 Generating gRPC code..."
	cd proto && chmod +x generate.sh && ./generate.sh

# Setup symlinks and dependencies
setup: deps proto
	@echo "🔗 Creating symlinks..."
	@cd master && (test -L proto || ln -s ../proto/pb proto)
	@cd worker && (test -L proto || ln -s ../proto/pb proto)
	@if [ -d agentic_scheduler ]; then \
		echo "� Creating agentic_scheduler proto symlink..."; \
		cd agentic_scheduler && (test -L proto && rm proto || true) && ln -s ../proto/py proto; \
	fi
	@echo "📦 Installing Go dependencies..."
	cd master && go mod tidy
	cd worker && go mod tidy
	@echo "✅ Setup complete!"

# Install external dependencies
deps:
	@echo "📦 Installing system and tool dependencies..."
	chmod +x scripts/install_deps.sh
	./scripts/install_deps.sh


# Build master node
master:
	@echo "🏗️  Building master node..."
	cd master && go build -o masterNode .
	@echo "✅ Master built: master/masterNode"

# Build worker node
worker:
	@echo "🏗️  Building worker node..."
	cd worker && go build -o workerNode .
	@echo "✅ Worker built: worker/workerNode"

# Clean generated files
clean:
	@echo "🧹 Cleaning..."
	rm -rf proto/pb proto/py
	rm -f master/masterNode
	rm -f worker/workerNode
	rm -rf venv
	cd master && (test -L proto && rm proto || true)
	cd worker && (test -L proto && rm proto || true)
	# cd agentic_scheduler && (test -L proto && rm proto || true)
	@echo "✅ Clean complete"

# Run basic tests
test:
	@echo "🧪 Running tests..."
	@echo "Checking Go version..."
	@go version
	@echo "Checking Docker..."

# Run Go unit test suite
test-unit:
	@echo "🧪 Running Go unit tests..."
	cd master && go test ./... -count=1 -timeout 120s
	cd worker && go test ./... -count=1 -timeout 120s
	@echo "✅ All unit tests passed"

# Run Go unit tests with verbose output
test-unit-verbose:
	@echo "🧪 Running Go unit tests (verbose)..."
	cd master && go test ./... -v -count=1 -timeout 120s
	cd worker && go test ./... -v -count=1 -timeout 120s
	@echo "✅ All unit tests passed"
	@docker version --format '{{.Server.Version}}'
	@echo "Checking protoc..."
	@protoc --version
	@echo "✅ All dependencies available"

# Build everything
all: setup master worker
	@echo "✅ All components built successfully!"

testbench-up:
	@docker compose -f testbench/docker-compose.yml up -d --build

testbench-prepare-images:
	@testbench/scripts/prepare_workflow_images.sh

testbench-register:
	@testbench/scripts/register_workers.sh

testbench-workload:
	@testbench/scripts/prepare_workflow_images.sh
	@python3 testbench/scripts/run_workload.py

testbench-suite:
	@testbench/scripts/run_suite.sh

testbench-down:
	@docker compose -f testbench/docker-compose.yml down --remove-orphans
