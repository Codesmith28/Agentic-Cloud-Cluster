# Docker Testbench for Heterogeneous Workers

This directory contains repeatable test harnesses and suite runners.

## Topologies

- `testbench/docker-compose.yml`: fully containerized stack (includes `master`)
- `testbench/docker-compose.host-master.yml`: host-master topology (master runs on host; compose runs mongo/prometheus/grafana/workers)

For the master-driven workflow (`master> test ...` / `./masterNode test ...`), use the **host-master** topology.

## Master command surface

Interactive command mode:

```bash
master> test list
master> test run <smoke|reliability|ui-smoke|evidence|full> [-profile <hetero-small|recovery-lab>] [-out <dir>] [-keep-env] [-ui-smoke] [-scheduler <current|RR|RTS>]
master> test cleanup
```

Non-interactive mode:

```bash
./masterNode test list
./masterNode test run <smoke|reliability|ui-smoke|evidence|full> [-profile <hetero-small|recovery-lab>] [-out <dir>] [-keep-env] [-ui-smoke] [-scheduler <current|RR|RTS>]
./masterNode test cleanup
```

Defaults:

- Compose file: `testbench/docker-compose.host-master.yml` (overridable with `TESTBENCH_COMPOSE_FILE`)
- Output dir: `results/testbench/<timestamp>-<suite>/`
- Host-routable worker registration defaults when using host-master compose:
  `worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054`

## Host-master quick start

```bash
make testbench-host-up
./runMaster.sh
make testbench-host-register
make testbench-host-suite-smoke
```

Prometheus in this topology uses:
`testbench/observability/prometheus/prometheus.host-master.yml`
(targets `host.docker.internal:8080` and worker metrics on `19101-19103`).

Grafana admin credentials are read from repo-root `.env` (`GF_ADMIN_USER`, `GF_ADMIN_PASSWORD`) with defaults `admin/password`.
- Host-master topology Grafana: `http://localhost:3300`
- Full-container topology Grafana: `http://localhost:3000`

## One-command integration + benchmark automation

Run the full Docker-backed gate (unit-test preflight + smoke + reliability + ui-smoke + evidence matrix):

```bash
make testbench-integration
```

This executes `testbench/scripts/run_integration.sh`, which:

- checks Docker daemon/compose availability before starting
- builds `master/masterNode` by default (`BUILD_MASTER=true`)
- runs `make test-unit` (disable with `RUN_UNIT_TESTS=false`)
- runs `./masterNode test run` for `smoke`, `reliability`, `ui-smoke`, and `evidence`
- uses `TESTBENCH_COMPOSE_FILE` (default: `testbench/docker-compose.host-master.yml`) and host-routable `WORKER_SPECS` defaults
- writes artifacts under `results/testbench/<timestamp>-integration/`

Useful overrides:

```bash
RUN_ROOT=results/testbench/my-run \
PROFILE=hetero-small \
BASE_SCHEDULER=current \
EVIDENCE_SCHEDULER=current \
TESTBENCH_COMPOSE_FILE=testbench/docker-compose.host-master.yml \
WORKER_SPECS=worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054 \
BUILD_MASTER=false \
KEEP_ENV=true \
make testbench-integration
```

## Scenario manifests + `run_suite.sh`

`testbench/scripts/run_suite.sh` reads `testbench/scenarios/*.json` (default selected by `SUITE_NAME`).
Current manifests: `smoke`, `reliability`, `ui-smoke`, `evidence`, `full`.

Supported `runner` values in manifests:

- `suite` / `workload`: run `run_workload.py`
- `ui-smoke`: run `run_ui_smoke.py`
- `campaign`: run `run_campaign.py`
- `composite`: run preflight + child suite sequence

Host-master invocation example:

```bash
COMPOSE_FILE=testbench/docker-compose.host-master.yml \
WORKER_SPECS=worker-small=host.docker.internal:55052,worker-medium=host.docker.internal:55053,worker-large=host.docker.internal:55054 \
SUITE_NAME=evidence \
testbench/scripts/run_suite.sh
```

## Artifacts

`run_suite.sh` writes a bundle under `results/testbench/<timestamp>-<suite>/` (or `RUN_ROOT`):

- `summary.json`
- `assertion_results.json`
- `logs/compose.log`
- `attempt_snapshots/index.json` (+ per-task JSON files when task IDs exist)
- `observability/prometheus-range.json`
- `observability/prometheus-instant.json`
- `observability/master-snapshot.json`
- `observability/metrics-summary.csv`
- campaign/composite extras under `campaign/` and per-child subdirectories

## Shared script utilities

- `testbench/scripts/shared_polling.py`: shared `request_json` + task polling helpers
- `testbench/scripts/export_attempt_snapshots.py`: writes `attempt_snapshots/index.json` and `<task_id>.json`
- `testbench/scripts/run_ui_smoke.py`: writes UI/auth smoke summary JSON to the `--output` path (used as `summary.json` by `run_suite.sh`)

Detailed runbook: [`docs/TESTBENCH_RUNBOOK.md`](../docs/TESTBENCH_RUNBOOK.md)
