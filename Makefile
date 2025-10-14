.PHONY: help proto master worker sample-task clean setup test

# Default target
help:
	@echo "CloudAI Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make proto        - Generate gRPC code from proto files"
	@echo "  make master       - Build master node"
	@echo "  make worker       - Build worker node"
	@echo "  make sample-task  - Build sample Docker task"
	@echo "  make setup        - Complete setup (proto + symlinks + deps)"
	@echo "  make clean        - Clean generated files and binaries"
	@echo "  make test         - Run basic connectivity tests"
	@echo ""
	@echo "Quick start:"
	@echo "  make setup        # One-time setup"
	@echo "  make master       # Build master"
	@echo "  make worker       # Build worker"

# Generate gRPC code
proto:
	@echo "🔧 Generating gRPC code..."
	cd proto && chmod +x generate.sh && ./generate.sh

# Setup symlinks and dependencies
setup: proto
	@echo "🔗 Creating symlinks..."
	@cd master && (test -L proto || ln -s ../proto/pb proto)
	@cd worker && (test -L proto || ln -s ../proto/pb proto)
	@echo "📦 Installing Go dependencies..."
	cd master && go mod tidy
	cd worker && go mod tidy
	@echo "✅ Setup complete!"

# Build master node
master:
	@echo "🏗️  Building master node..."
	cd master && go build -o master-node .
	@echo "✅ Master built: master/master-node"

# Build worker node
worker:
	@echo "🏗️  Building worker node..."
	cd worker && go build -o worker-node .
	@echo "✅ Worker built: worker/worker-node"

# Build sample task Docker image
sample-task:
	@echo "🐳 Building sample task Docker image..."
	@read -p "Enter your Docker Hub username: " username; \
	cd sample_task && \
	docker build -t $$username/cloudai-sample-task:latest . && \
	echo "✅ Sample task built: $$username/cloudai-sample-task:latest" && \
	echo "To push: docker push $$username/cloudai-sample-task:latest"

# Clean generated files
clean:
	@echo "🧹 Cleaning..."
	rm -rf proto/pb proto/py
	rm -f master/master-node
	rm -f worker/worker-node
	cd master && (test -L proto && rm proto || true)
	cd worker && (test -L proto && rm proto || true)
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
