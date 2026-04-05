# Docker Testbench for Heterogeneous Workers

This testbench provides an automated performance harness for CloudAI:

- `master` and `mongo` run in containers
- each `worker-*` runs in its own container
- each worker has its own Docker daemon (`worker-*-dind`) so task containers are isolated per worker node
- worker capabilities are intentionally heterogeneous via CPU/memory limits and explicit resource overrides
- Prometheus scrapes master and worker metrics during every run
- Grafana ships with provisioned dashboards for queueing, recovery, runtime, and benchmark summaries

Detailed step-by-step instructions: [`docs/TESTBENCH_RUNBOOK.md`](../docs/TESTBENCH_RUNBOOK.md)

## Why This Matches Your Goal

Yes, this is possible: worker nodes can be containerized and still run many Docker tasks.

In this setup, each worker talks to its own Docker daemon (`DOCKER_HOST=tcp://worker-*-dind:2375`), so submitted workloads run in a worker-scoped task environment instead of sharing one global host daemon.

## Files

- `testbench/docker-compose.yml`: full cluster topology
- `testbench/workloads/heterogeneous-smoke.json`: default mixed workload
- `testbench/scripts/register_workers.sh`: automatic worker registration
- `testbench/scripts/run_workload.py`: submits tasks and waits for completion
- `testbench/scripts/run_suite.sh`: one-shot end-to-end execution

## Quick Start

From repo root:

```bash
export GF_ADMIN_PASSWORD=admin
make testbench-suite
```

This runs:

1. `docker compose -f testbench/docker-compose.yml up -d --build`
2. automatic registration of `worker-small`, `worker-medium`, `worker-large`
3. workload submission + polling until completion
4. summary JSON output under `results/testbench/`
5. Prometheus-backed observability export under `results/testbench/<timestamp>-observability/`

## Manual Flow

```bash
make testbench-up
make testbench-register
make testbench-workload
make testbench-down
```

## Observability Access

Before starting the stack, set the Grafana admin password expected by `docker compose`:

```bash
export GF_ADMIN_PASSWORD=admin
```

Once the stack is running:

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`
- Grafana login: `admin` / `$GF_ADMIN_PASSWORD`
- Master metrics endpoint: `http://localhost:8080/metrics`

Provisioned dashboards:

- `CloudAI Overview`
- `CloudAI Scheduler Queue`
- `CloudAI Recovery Incidents`
- `CloudAI Worker Runtime`
- `CloudAI Benchmark Summary`

Artifacts exported per `run_suite.sh` execution:

- `results/testbench/<timestamp>-summary.json`
- `results/testbench/<timestamp>-observability/prometheus-range.json`
- `results/testbench/<timestamp>-observability/prometheus-instant.json`
- `results/testbench/<timestamp>-observability/master-snapshot.json`
- `results/testbench/<timestamp>-observability/metrics-summary.csv`

## Adjusting Heterogeneity

Edit each `worker-*` service in `testbench/docker-compose.yml`:

- cgroup bounds on `worker-*-dind` (`cpus`, `mem_limit`, `pids_limit`)
- scheduler-visible resources on `worker-*`:
  - `WORKER_TOTAL_CPU`
  - `WORKER_TOTAL_MEMORY_GB`
  - `WORKER_TOTAL_STORAGE_GB`

## Useful Checks

```bash
curl http://localhost:8080/api/workers | jq
curl http://localhost:8080/api/tasks | jq
curl http://localhost:8080/api/tasks/<task_id>/attempts | jq
curl http://localhost:8080/metrics | head
docker compose -f testbench/docker-compose.yml logs -f worker-small
```
