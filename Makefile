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

.PHONY: help all proto master worker sample-task clean setup test

# Default target
help:
	@echo "CloudAI Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make all          - Build everything (setup + master + worker)"
	@echo "  make proto        - Generate gRPC code from proto files"
	@echo "  make master       - Build master node"
	@echo "  make worker       - Build worker node"
	@echo "  make sample-task  - Build sample Docker task"
	@echo "  make setup        - Complete setup (proto + symlinks + deps)"
	@echo "  make clean        - Clean generated files and binaries"
	@echo "  make test         - Run basic connectivity tests"
	@echo ""
	@echo "Quick start:"
	@echo "  make setup        # One-time setup (includes agentic_scheduler)"
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
setup: proto
	@echo "🔗 Creating symlinks..."
	@cd master && (test -L proto || ln -s ../proto/pb proto)
	@cd worker && (test -L proto || ln -s ../proto/pb proto)
	@echo "� Creating agentic_scheduler proto symlink..."
	@cd agentic_scheduler && (test -L proto && rm proto || true) && ln -s ../proto/py proto
	@echo "�📦 Installing Go dependencies..."
	cd master && go mod tidy
	cd worker && go mod tidy
	@echo "✅ Setup complete!"

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

# Build sample task Docker image
sample-task:
	@echo "🐳 Building sample task Docker images..."
	@read -p "Enter your Docker Hub username: " username; \
	for task_dir in sample_tasks/*/; do \
		task_name=$$(basename $$task_dir); \
		echo "Building $$task_name..."; \
		cd $$task_dir && \
		docker build -t $$username/cloudai-$$task_name:latest . && \
		echo "✅ $$task_name built: $$username/cloudai-$$task_name:latest"; \
		cd ../..; \
	done && \
	echo "✅ All sample tasks built successfully!" && \
	echo "To push: docker push $$username/cloudai-<task_name>:latest"

# Clean generated files
clean:
	@echo "🧹 Cleaning..."
	rm -rf proto/pb proto/py
	rm -f master/master-node
	rm -f worker/worker-node
	cd master && (test -L proto && rm proto || true)
	cd worker && (test -L proto && rm proto || true)
	cd agentic_scheduler && (test -L proto && rm proto || true)
	@echo "✅ Clean complete"

# Run basic tests
test:
	@echo "🧪 Running tests..."
	@echo "Checking Go version..."
	@go version
	@echo "Checking Docker..."
	@docker version --format '{{.Server.Version}}'
	@echo "Checking protoc..."
	@protoc --version
	@echo "✅ All dependencies available"

# Build everything
all: setup master worker
	@echo "✅ All components built successfully!"
