# ===========================================================================
# CloudAI – Agentic Cloud Cluster Build System
# ===========================================================================
#
# Port allocation scheme:
#   50050  PPO scheduler gRPC  (colocated with master)
#   50051  Master gRPC
#   50052+ Worker gRPC         (auto-increment per host)
#   8080   Master HTTP API
#   9101+  Worker metrics
#   27017  MongoDB
#   9090   Prometheus  |  3000/3300  Grafana
#
# Quick start:
#   make setup          # one-time: install deps, generate proto, symlinks
#   make build          # compile master + worker binaries
#   make run-master     # start master node (RTS scheduler)
#   make run-master-ppo # start master node (PPO scheduler)
#   make run-worker     # start a worker node
#
# ===========================================================================

.PHONY: help build all proto master worker clean setup deps \
	check vet fmt \
	test test-unit test-unit-verbose \
	venv pip-install \
	db-up db-down \
	run-master run-master-ppo run-worker ppo-server \
	testbench-up testbench-host-up testbench-down testbench-host-down \
	testbench-prepare-images testbench-register testbench-host-register \
	testbench-workload testbench-suite testbench-integration \
	testbench-suite-smoke testbench-suite-reliability testbench-suite-ui-smoke \
	testbench-suite-evidence testbench-suite-full \
	testbench-host-suite testbench-host-suite-smoke testbench-host-suite-reliability \
	testbench-host-suite-ui-smoke testbench-host-suite-evidence testbench-host-suite-full \
	campaign campaign-full campaign-final campaign-comprehensive \
	campaign-prereqs \
	model-promote model-promote-dry model-archive-list \
	deploy benchmark

# ---------------------------------------------------------------------------
# Variables (override with env or make VAR=value)
# ---------------------------------------------------------------------------
VENV          ?= venv
PYTHON        ?= $(VENV)/bin/python3
PIP           ?= $(VENV)/bin/pip
COMPOSE_HOST  ?= testbench/docker-compose.host-master.yml
COMPOSE_FULL  ?= testbench/docker-compose.yml
DB_COMPOSE    ?= database/docker-compose.yml
WORKER_SPECS  ?= worker-small=localhost:55052,worker-medium=localhost:55053,worker-large=localhost:55054
MASTER_URL    ?= http://localhost:8080

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

help:
	@echo "CloudAI Build System"
	@echo ""
	@echo "Build & Setup:"
	@echo "  make setup              One-time setup (deps + proto + symlinks + go mod tidy)"
	@echo "  make build              Build master + worker binaries"
	@echo "  make all                Full setup + build"
	@echo "  make proto              Generate gRPC code from .proto files"
	@echo "  make master             Build master binary only"
	@echo "  make worker             Build worker binary only"
	@echo "  make clean              Remove binaries, generated code, venv"
	@echo ""
	@echo "Quality:"
	@echo "  make check              Compile-check both Go modules (no binary output)"
	@echo "  make vet                Run go vet on master + worker"
	@echo "  make fmt                Run gofmt on master + worker"
	@echo ""
	@echo "Tests:"
	@echo "  make test               Verify toolchain (go, docker, protoc)"
	@echo "  make test-unit          Run Go unit tests"
	@echo "  make test-unit-verbose  Run Go unit tests with verbose output"
	@echo ""
	@echo "Python / PPO:"
	@echo "  make venv               Create Python virtualenv"
	@echo "  make pip-install        Install Python deps into venv"
	@echo "  make ppo-server         Start PPO gRPC server (venv)"
	@echo ""
	@echo "Database:"
	@echo "  make db-up              Start MongoDB (docker compose)"
	@echo "  make db-down            Stop MongoDB"
	@echo ""
	@echo "Run:"
	@echo "  make run-master         Build + start master (RTS scheduler)"
	@echo "  make run-master-ppo     Build + start master (PPO scheduler)"
	@echo "  make run-worker         Build + start worker"
	@echo ""
	@echo "Testbench (Docker workers):"
	@echo "  make testbench-up                Start full Docker stack (master + workers)"
	@echo "  make testbench-host-up           Start worker-only Docker stack"
	@echo "  make testbench-down              Tear down full Docker stack"
	@echo "  make testbench-host-down         Tear down worker-only Docker stack"
	@echo "  make testbench-prepare-images    Build workflow images into worker DinD"
	@echo "  make testbench-register          Register workers (full stack)"
	@echo "  make testbench-host-register     Register workers (host-master topology)"
	@echo "  make testbench-workload          Prepare images + submit default workload"
	@echo "  make testbench-suite SUITE_NAME=smoke   Run test suite"
	@echo "  make testbench-host-suite SUITE_NAME=smoke  Run suite (host-master)"
	@echo "  make testbench-integration       Full integration + benchmark"
	@echo ""
	@echo "Campaigns (requires running master; workers auto-start/register in host-master mode):"
	@echo "  make campaign           Evidence benchmark (smoke workload)"
	@echo "  make campaign-full      Full benchmark (all workloads + scenarios)"
	@echo "  make campaign-final     HEAVY final evaluation (50 tasks × 3 × 3 schedulers)"
	@echo ""
	@echo "Cluster Reset:"
	@echo "  make reset              Full clean-slate reset (stops all, wipes all volumes)"
	@echo "  make reset-soft         Stop services only (keep data)"
	@echo "  make reset-keep-dind    Reset but keep Docker-in-Docker layers (faster restart)"
	@echo ""
	@echo "PPO Testing (clean-slate, new model + resource-contention workload):"
	@echo "  make test-ppo           Full PPO test (all scenarios)"
	@echo "  make test-ppo-fast      Fast PPO test (baseline only, ~10 min)"
	@echo "  make test-ppo-full      Comprehensive PPO test (all workloads)"
	@echo ""
	@echo "Model Management:"
	@echo "  make model-promote      Promote latest trained model (archives old)"
	@echo "  make model-promote-dry  Dry-run promotion preview"
	@echo "  make model-archive-list List archived model versions"
	@echo ""
	@echo "Pipelines:"
	@echo "  make deploy             Build → promote model → benchmark"
	@echo "  make benchmark          Run execute-tests.sh end-to-end"

# ===========================================================================
# Build
# ===========================================================================

build: master worker

all: setup build
	@echo "✅ All components built successfully!"

proto:
	@echo "🔧 Generating gRPC code..."
	cd proto && chmod +x generate.sh && ./generate.sh

setup: deps proto
	@echo "🔗 Creating symlinks..."
	@cd master && (test -L proto || ln -s ../proto/pb proto)
	@cd worker && (test -L proto || ln -s ../proto/pb proto)
	@if [ -d agentic_scheduler ]; then \
		cd agentic_scheduler && (test -L proto && rm proto || true) && ln -s ../proto/py proto; \
	fi
	@echo "📦 Installing Go dependencies..."
	cd master && go mod tidy
	cd worker && go mod tidy
	@echo "✅ Setup complete!"

deps:
	@echo "📦 Installing system and tool dependencies..."
	chmod +x scripts/install_deps.sh
	./scripts/install_deps.sh

master:
	@echo "🏗️  Building master node..."
	cd master && go build -o masterNode .
	@echo "✅ Master built: master/masterNode"

worker:
	@echo "🏗️  Building worker node..."
	cd worker && go build -o workerNode .
	@echo "✅ Worker built: worker/workerNode"

clean:
	@echo "🧹 Cleaning..."
	rm -rf proto/pb proto/py
	rm -f master/masterNode worker/workerNode
	rm -rf $(VENV)
	cd master && (test -L proto && rm proto || true)
	cd worker && (test -L proto && rm proto || true)
	@echo "✅ Clean complete"

# ===========================================================================
# Quality
# ===========================================================================

check:
	@echo "🔍 Compile-checking Go code..."
	cd pkg && go build ./...
	cd master && go build -o /dev/null ./...
	cd worker && go build -o /dev/null ./...
	@echo "✅ Compile check passed"

vet:
	@echo "🔍 Running go vet..."
	cd pkg && go vet ./...
	cd master && go vet ./...
	cd worker && go vet ./...
	@echo "✅ Vet passed"

fmt:
	@echo "🔍 Running gofmt..."
	@gofmt -l pkg/ master/ worker/ | tee /dev/stderr | (! read) && echo "✅ All files formatted" || echo "⚠️  Files above need formatting (run: gofmt -w pkg/ master/ worker/)"

# ===========================================================================
# Tests
# ===========================================================================

test:
	@echo "🧪 Checking toolchain..."
	@printf "  Go:      " && go version
	@printf "  Docker:  " && docker version --format '{{.Server.Version}}' 2>/dev/null || echo "(not running)"
	@printf "  protoc:  " && protoc --version 2>/dev/null || echo "(not installed)"
	@printf "  Python:  " && python3 --version 2>/dev/null || echo "(not installed)"
	@echo "✅ Toolchain check complete"

test-unit:
	@echo "🧪 Running Go unit tests..."
	cd pkg && go test ./... -count=1 -timeout 120s
	cd master && go test ./... -count=1 -timeout 120s
	cd worker && go test ./... -count=1 -timeout 120s
	@echo "✅ All unit tests passed"

test-unit-verbose:
	@echo "🧪 Running Go unit tests (verbose)..."
	cd pkg && go test ./... -v -count=1 -timeout 120s
	cd master && go test ./... -v -count=1 -timeout 120s
	cd worker && go test ./... -v -count=1 -timeout 120s
	@echo "✅ All unit tests passed"

test-python:
	@echo "🐍 Running Python unit tests..."
	$(PYTHON) -m unittest discover -s agentic_scheduler/tests/ -v
	@echo "✅ Python unit tests passed"

# ===========================================================================
# Python / PPO
# ===========================================================================

venv:
	@if [ ! -d "$(VENV)" ]; then \
		echo "🐍 Creating virtualenv at $(VENV)..."; \
		python3 -m venv $(VENV); \
		echo "✅ Virtualenv created"; \
	else \
		echo "✅ Virtualenv already exists at $(VENV)"; \
	fi

pip-install: venv
	@echo "📦 Installing Python dependencies..."
	$(PIP) install --quiet -r requirements.txt
	@echo "✅ Python dependencies installed"

ppo-server: pip-install
	@echo "🧠 Starting PPO gRPC server..."
	$(PYTHON) -m agentic_scheduler.server

# ===========================================================================
# Database
# ===========================================================================

db-up:
	@echo "🗄️  Starting MongoDB..."
	docker compose -f $(DB_COMPOSE) up -d
	@echo "✅ MongoDB running on localhost:27017"

db-down:
	@echo "🗄️  Stopping MongoDB..."
	docker compose -f $(DB_COMPOSE) down
	@echo "✅ MongoDB stopped"

# ===========================================================================
# Run services
# ===========================================================================

run-master: master
	@echo "🚀 Starting master node (RTS scheduler)..."
	./runMaster.sh

run-master-ppo: master
	@echo "🚀 Starting master node (PPO scheduler)..."
	./runMaster.sh --ppo

run-worker: worker
	@echo "🚀 Starting worker node..."
	./runWorker.sh

# ===========================================================================
# Testbench – Docker stacks
# ===========================================================================

testbench-up:
	@docker compose -f $(COMPOSE_FULL) up -d --build

testbench-host-up:
	@docker compose -f $(COMPOSE_HOST) up -d

testbench-down:
	@docker compose -f $(COMPOSE_FULL) down --remove-orphans

testbench-host-down:
	@docker compose -f $(COMPOSE_HOST) down --remove-orphans

# ===========================================================================
# Testbench – Workers & Workloads
# ===========================================================================

testbench-prepare-images:
	@testbench/scripts/prepare_workflow_images.sh

testbench-register:
	@testbench/scripts/register_workers.sh

testbench-host-register:
	@MASTER_URL=$(MASTER_URL) \
	WORKER_SPECS=$(WORKER_SPECS) \
	testbench/scripts/register_workers.sh

testbench-workload: testbench-prepare-images
	@python3 testbench/scripts/run_workload.py

# ===========================================================================
# Testbench – Suites
# ===========================================================================

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
	@COMPOSE_FILE=$(COMPOSE_HOST) \
	WORKER_SPECS=$(WORKER_SPECS) \
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

# ===========================================================================
# Campaigns
# ===========================================================================

campaign-prereqs:
	@echo "🔧 Ensuring host-master campaign prerequisites (workers + registration + images)..."
	@$(MAKE) testbench-host-up
	@$(MAKE) testbench-host-register
	@$(MAKE) testbench-prepare-images

campaign: campaign-prereqs
	@$(PYTHON) testbench/scripts/run_campaign.py --scenarios all --workloads heterogeneous-smoke

campaign-full: campaign-prereqs
	@$(PYTHON) testbench/scripts/run_campaign.py --scenarios all --workloads heterogeneous-smoke,deterministic-full

campaign-final: campaign-prereqs
	@echo "🏋️ Running HEAVY final evaluation campaign (50 tasks × 3 scenarios × 3 schedulers = 450 decisions)..."
	@$(PYTHON) testbench/scripts/run_campaign.py \
		--scenarios baseline,burst,overload \
		--schedulers RR,RTS,PPO \
		--workloads stress-heavy

campaign-comprehensive: campaign-prereqs
	@echo "📊 Running COMPREHENSIVE benchmark (4 workloads × 3 scenarios × 3 schedulers)..."
	@$(PYTHON) testbench/scripts/run_campaign.py \
		--scenarios baseline,burst,overload \
		--schedulers RR,RTS,PPO \
		--workloads heterogeneous-smoke,steady-cpu,bursty,memory-pressure \
		--timeout 900

# ===========================================================================
# Model management
# ===========================================================================

model-promote:
	@scripts/model_promote.sh $(MODEL_PATH)

model-promote-dry:
	@scripts/model_promote.sh --dry-run $(MODEL_PATH)

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

# ===========================================================================
# Pipelines
# ===========================================================================

deploy: build model-promote benchmark

benchmark:
	@./execute-tests.sh

test-ppo:
	@echo "🤖 Running clean-slate PPO benchmark (all scenarios, resource-contention workload)..."
	@./run-ppo-test.sh

test-ppo-fast:
	@echo "🤖 Running fast PPO baseline benchmark (single scenario)..."
	@./run-ppo-test.sh --fast

test-ppo-full:
	@echo "🤖 Running comprehensive PPO benchmark (all workloads)..."
	@./run-ppo-test.sh --workloads resource-contention-ppo,heterogeneous-smoke,deterministic-full

reset:
	@echo "🧹 Full clean-slate cluster reset (all volumes wiped)..."
	@./reset-cluster.sh --yes

reset-soft:
	@echo "🛑 Soft reset — stopping services, keeping data..."
	@./reset-cluster.sh --soft --yes

reset-keep-dind:
	@echo "🧹 Cluster reset keeping Docker-in-Docker layers..."
	@./reset-cluster.sh --keep-dind --yes
