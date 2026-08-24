# Agentic Cloud Cluster Testbench Runbook

Practical reference for the implemented master-driven test workflow and suite runners.

## 1) Prerequisites

- Docker with `docker compose`
- Go (for `masterNode` and optional preflight tests)
- Python 3.8+

Quick check:

```bash
docker --version
docker compose version
python3 --version
go version
```

## 2) Master command surface

### Interactive (`master>` prompt)

```bash
master> test list
master> test run <smoke|reliability|ui-smoke|evidence|full> [-profile <hetero-small|recovery-lab>] [-out <dir>] [-keep-env] [-ui-smoke] [-scheduler <current|RR|RTS>]
master> test cleanup
```

### Non-interactive (`masterNode` CLI mode)

```bash
./masterNode test list
./masterNode test run <smoke|reliability|ui-smoke|evidence|full> [-profile <hetero-small|recovery-lab>] [-out <dir>] [-keep-env] [-ui-smoke] [-scheduler <current|RR|RTS>]
./masterNode test cleanup
```

### Flag behavior

- `-profile`: `hetero-small` or `recovery-lab`
- `-out`: custom artifact directory (default: `results/testbench/<timestamp>-<suite>/`)
- `-keep-env`: skip compose teardown at end
- `-ui-smoke`: enable extra UI/API smoke verification in smoke runs
- `-scheduler`: `current`, `RR`, or `RTS`

Notes:

- `test cleanup` runs `docker compose ... down --remove-orphans` against the configured compose file.
- Non-interactive `test run evidence` with default scheduler (`current`) runs an RR+RTS matrix and writes a matrix summary.

## 3) Host-master topology (recommended for master-driven tests)

Use:

- Compose file: `testbench/docker-compose.host-master.yml`
- Prometheus config: `testbench/observability/prometheus/prometheus.host-master.yml`

This topology keeps `master` on the host and runs MongoDB/Prometheus/Grafana/workers in compose.
Prometheus scrapes:

- `host.docker.internal:8080` (master metrics)
- `host.docker.internal:19101-19103` (worker metrics)

### Bring up and register workers

```bash
make testbench-host-up
make testbench-host-register
```

`WORKER_SPECS` (host-routable registration) defaults to:

```bash
worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054
```

Custom format accepted by `register_workers.sh`:

- comma/semicolon-separated entries
- each entry as `worker_id=host:port` (or whitespace-separated `worker_id host:port`)

Start master on host (example):

```bash
./scripts/master/run.sh
```

Teardown:

```bash
make testbench-host-down
```

## 4) One-command integration + benchmark automation

Run the full integration gate and evidence benchmark generation with one command:

```bash
make testbench-integration
```

Behavior:

- validates Docker daemon + `docker compose` availability up front
- builds `master/masterNode` by default (`BUILD_MASTER=true`)
- runs `make test-unit` preflight (override with `RUN_UNIT_TESTS=false`)
- runs `./masterNode test run` suites in sequence: `smoke`, `reliability`, `ui-smoke`, `evidence`
- uses `TESTBENCH_COMPOSE_FILE` (default: `testbench/docker-compose.host-master.yml`) and host-routable `WORKER_SPECS` defaults
- stores artifacts under `results/testbench/<timestamp>-integration/`
- writes `integration-summary.json` with suite output paths

Common overrides:

```bash
RUN_ROOT=results/testbench/custom-integration \
PROFILE=hetero-small \
BASE_SCHEDULER=current \
EVIDENCE_SCHEDULER=current \
TESTBENCH_COMPOSE_FILE=testbench/docker-compose.host-master.yml \
WORKER_SPECS=worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054 \
BUILD_MASTER=false \
KEEP_ENV=true \
make testbench-integration
```

CI automation is defined in:

- `.github/workflows/testbench-integration.yml`

It supports manual dispatch and nightly runs, and uploads the full run bundle as a workflow artifact.

## 5) Scenario manifests and `run_suite.sh`

Scenario manifests live in `testbench/scenarios/*.json`:

- `smoke.json`
- `reliability.json`
- `ui-smoke.json`
- `evidence.json`
- `full.json`

`testbench/scripts/run_suite.sh` selects `testbench/scenarios/${SUITE_NAME}.json` by default.

Recognized manifest fields:

- `suite`, `runner`, `description`
- `workload`, `ui_url`
- `scenarios`, `workloads`, `schedulers`
- `sequence`
- `assertions`
- `preflight.go_test`
- `stop_on_failure`

Runner types implemented by `run_suite.sh`:

- `suite` / `workload`: execute `run_workload.py`
- `ui-smoke`: execute `run_ui_smoke.py`
- `campaign`: execute `run_campaign.py`
- `composite`: run optional preflight `go test`, then child suite sequence

Examples:

```bash
# Default topology (containerized master) unless COMPOSE_FILE is set
SUITE_NAME=smoke testbench/scripts/run_suite.sh

# Host-master topology
COMPOSE_FILE=testbench/docker-compose.host-master.yml \
WORKER_SPECS=worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054 \
SUITE_NAME=evidence \
testbench/scripts/run_suite.sh
```

Host-master make wrappers:

```bash
make testbench-host-suite
make testbench-host-suite-smoke
make testbench-host-suite-reliability
make testbench-host-suite-ui-smoke
make testbench-host-suite-evidence
make testbench-host-suite-full
```

## 6) Script utilities

- `testbench/scripts/shared_polling.py`
  - shared HTTP helper (`request_json`) and task polling (`poll_task_completion`)
  - used by `run_workload.py`, `run_campaign.py`, `run_ui_smoke.py`, `export_attempt_snapshots.py`
- `testbench/scripts/export_attempt_snapshots.py`
  - exports per-task snapshots from master APIs
  - writes `attempt_snapshots/index.json` and `<task_id>.json` files
- `testbench/scripts/run_ui_smoke.py`
  - checks API health, workers/tasks endpoints, UI root, and auth register/login/me
  - writes summary JSON to its `--output` path (in suites this is `summary.json`)

## 7) Where artifacts land

### `run_suite.sh` bundle

Default root:

`results/testbench/<timestamp>-<suite>/` (override with `RUN_ROOT`)

Common outputs:

- `summary.json`
- `assertion_results.json`
- `logs/compose.log`
- `attempt_snapshots/index.json` (+ per-task files when task IDs exist)
- `observability/prometheus-range.json`
- `observability/prometheus-instant.json`
- `observability/master-snapshot.json`
- `observability/metrics-summary.csv`

Runner-specific:

- `campaign/` timestamped campaign report artifacts (`campaign-report.json`, `REPORT.md`, `REPORT.html`, CSV exports)
- `composite` creates per-child suite subdirectories; top-level `attempt_snapshots/index.json` and `observability/index.json` point to sub-runs

### `master> test run` / `./masterNode test run`

Default root:

`results/testbench/<timestamp>-<suite>/` (override with `-out`)

The workflow writes `run-result.json` with executed steps and step logs under `logs/`.
Smoke/UI smoke runs also emit workload summary + attempt/observability exports.
Reliability/Evidence runs emit campaign artifacts under `campaign/<timestamp>/...`.

## 8) Make targets (current)

- `make testbench-up` / `make testbench-down` (containerized master topology)
- `make testbench-host-up` / `make testbench-host-down` (host-master topology)
- `make testbench-register` / `make testbench-host-register`
- `make testbench-suite` (default `SUITE_NAME=smoke`)
- `make testbench-host-suite` (default `SUITE_NAME=smoke`, host-master compose + host-routable `WORKER_SPECS`)
- `make testbench-integration` (one-command full integration + benchmark automation)
- `make campaign`, `make campaign-full`

## 9) Quick troubleshooting

- Master unreachable: verify `curl http://localhost:8080/health`
- Worker registration issues: rerun `make testbench-host-register` and check `WORKER_SPECS`
- Missing artifacts: inspect `logs/compose.log` and suite step logs under each run directory
