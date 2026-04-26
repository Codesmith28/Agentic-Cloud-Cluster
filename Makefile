.PHONY: help all proto master worker clean setup test test-unit test-unit-verbose testbench-up testbench-prepare-images testbench-register testbench-workload testbench-suite testbench-suite-smoke testbench-suite-reliability testbench-suite-ui-smoke testbench-suite-evidence testbench-suite-full testbench-integration testbench-down testbench-host-up testbench-host-register testbench-host-suite testbench-host-suite-smoke testbench-host-suite-reliability testbench-host-suite-ui-smoke testbench-host-suite-evidence testbench-host-suite-full testbench-host-down campaign campaign-full model-promote model-promote-dry model-archive-list deploy

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
	@echo "  make testbench-host-up - Start host-master worker stack (no master container)"
	@echo "  make testbench-prepare-images - Build/load deterministic workflow image into worker DinD daemons"
	@echo "  make testbench-register - Register testbench workers with master"
	@echo "  make testbench-host-register - Register host-master workers with host-routable addresses"
	@echo "  make testbench-workload - Prepare image, submit default workload, wait for completion"
	@echo "  make testbench-suite [SUITE_NAME=...] - Run scenario manifest (default: smoke)"
	@echo "  make testbench-host-suite [SUITE_NAME=...] - Run suite against host-run master (default: smoke)"
	@echo "  make testbench-suite-{smoke,reliability,ui-smoke,evidence,full} - Scenario shortcuts"
	@echo "  make testbench-host-suite-{smoke,reliability,ui-smoke,evidence,full} - Host topology shortcuts"
	@echo "  make testbench-integration - Run full Docker-backed integration + benchmark automation"
	@echo "  make testbench-down - Stop and remove Docker testbench stack"
	@echo "  make testbench-host-down - Stop host-master worker stack"
	@echo "  make campaign     - Run evidence benchmark campaign (smoke workload)"
	@echo "  make campaign-full - Run full campaign (all workloads + all scenarios)"
	@echo ""
	@echo "Model management:"
	@echo "  make model-promote     - Promote latest trained model (archive old version)"
	@echo "  make model-promote-dry - Preview promotion without changes"
	@echo "  make model-archive-list - List archived model versions"
	@echo "  make deploy            - Build + promote model + start testbench + run campaign"
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

testbench-host-up:
	@docker compose -f testbench/docker-compose.host-master.yml up -d --build

testbench-prepare-images:
	@testbench/scripts/prepare_workflow_images.sh

testbench-register:
	@testbench/scripts/register_workers.sh

testbench-host-register:
	@MASTER_URL=$${MASTER_URL:-http://localhost:8080} \
	WORKER_SPECS=$${WORKER_SPECS:-worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054} \
	testbench/scripts/register_workers.sh

testbench-workload: testbench-prepare-images
	@python3 testbench/scripts/run_workload.py

testbench-suite:
	@SUITE_NAME=$${SUITE_NAME:-smoke} testbench/scripts/run_suite.sh

testbench-suite-smoke:
	@$(MAKE) testbench-suite SUITE_NAME=smoke

testbench-suite-reliability:
	@$(MAKE) testbench-suite SUITE_NAME=reliability

testbench-suite-ui-smoke:
	@$(MAKE) testbench-suite SUITE_NAME=ui-smoke

testbench-suite-evidence:
	@$(MAKE) testbench-suite SUITE_NAME=evidence

testbench-suite-full:
	@$(MAKE) testbench-suite SUITE_NAME=full

testbench-integration:
	@testbench/scripts/run_integration.sh

testbench-host-suite:
	@COMPOSE_FILE=testbench/docker-compose.host-master.yml \
	WORKER_SPECS=$${WORKER_SPECS:-worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054} \
	SUITE_NAME=$${SUITE_NAME:-smoke} \
	testbench/scripts/run_suite.sh

testbench-host-suite-smoke:
	@$(MAKE) testbench-host-suite SUITE_NAME=smoke

testbench-host-suite-reliability:
	@$(MAKE) testbench-host-suite SUITE_NAME=reliability

testbench-host-suite-ui-smoke:
	@$(MAKE) testbench-host-suite SUITE_NAME=ui-smoke

testbench-host-suite-evidence:
	@$(MAKE) testbench-host-suite SUITE_NAME=evidence

testbench-host-suite-full:
	@$(MAKE) testbench-host-suite SUITE_NAME=full

testbench-down:
	@docker compose -f testbench/docker-compose.yml down --remove-orphans

testbench-host-down:
	@docker compose -f testbench/docker-compose.host-master.yml down --remove-orphans

# Run evidence benchmark campaign across schedulers and scenarios
campaign:
	@python3 testbench/scripts/run_campaign.py --scenarios all --workloads heterogeneous-smoke

campaign-full:
	@python3 testbench/scripts/run_campaign.py --scenarios all --workloads heterogeneous-smoke,deterministic-full

# ---------------------------------------------------------------------------
# Model management
# ---------------------------------------------------------------------------

# Promote the latest trained model to active, archiving the previous version
model-promote:
	@scripts/model_promote.sh $(MODEL_PATH)

# Dry-run: preview what model-promote would do
model-promote-dry:
	@scripts/model_promote.sh --dry-run $(MODEL_PATH)

# List all archived model versions
model-archive-list:
	@echo "Archived models:"
	@ls -lh agentic_scheduler/models/archive/*.pt 2>/dev/null \
		| awk '{print "  " $$NF " (" $$5 ")"}' \
		|| echo "  (no archived models yet)"
	@echo ""
	@if [ -f agentic_scheduler/models/archive/VERSION ]; then \
		echo "Current version: v$$(printf '%03d' $$(cat agentic_scheduler/models/archive/VERSION))"; \
	else \
		echo "Current version: (none)"; \
	fi

# Full deploy pipeline: build → promote model → start testbench → run campaign
deploy: master worker model-promote
	@./execute-tests.sh --skip-build
