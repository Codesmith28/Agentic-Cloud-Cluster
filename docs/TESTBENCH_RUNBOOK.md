# CloudAI Testbench Runbook

This runbook shows exactly how to run the Docker-based CloudAI testbenches for repeatable performance testing.

## 1) Prerequisites

- Docker Desktop or Docker Engine with `docker compose`
- Python 3.8+
- Optional: `jq` for pretty JSON output
- `GF_ADMIN_PASSWORD` exported for Grafana startup

Quick check:

```bash
docker --version
docker compose version
python3 --version
export GF_ADMIN_PASSWORD=admin
```

## 2) Testbench Types

- `testbench-suite` (recommended): start stack + register workers + run default workload
- manual flow: run each stage independently
- custom workload: run your own workload JSON file
- deterministic workflow image: repo-owned benchmark image built into worker DinD daemons

## 3) One-Command Run (Recommended)

From repository root:

```bash
export GF_ADMIN_PASSWORD=admin
make testbench-suite
```

What it does:

1. Builds and starts `mongo`, `master`, `worker-small`, `worker-medium`, `worker-large`
2. Starts `prometheus` and `grafana`
3. Builds `cloudai-benchmark:1` inside each worker-side Docker daemon
4. Registers workers through `POST /api/workers`
5. Submits workload from `testbench/workloads/heterogeneous-smoke.json`
6. Polls task status until completion
7. Writes summary JSON to `results/testbench/<timestamp>-summary.json`
8. Exports Prometheus-backed observability artifacts to `results/testbench/<timestamp>-observability/`

## 4) Manual Stage-by-Stage Run

Start stack:

```bash
export GF_ADMIN_PASSWORD=admin
make testbench-up
```

Prepare deterministic workflow image:

```bash
make testbench-prepare-images
```

Register workers:

```bash
make testbench-register
```

Run workload:

```bash
make testbench-workload
```

Teardown:

```bash
make testbench-down
```

## 5) Observability Access

Prometheus:

```bash
open http://localhost:9090
```

Grafana:

```bash
open http://localhost:3000
```

Grafana credentials:

- username: `admin` unless `GF_ADMIN_USER` is overridden
- password: value of `GF_ADMIN_PASSWORD`

Master metrics endpoint:

```bash
curl http://localhost:8080/metrics | head
```

Provisioned dashboards:

- `CloudAI Overview`
- `CloudAI Scheduler Queue`
- `CloudAI Recovery Incidents`
- `CloudAI Worker Runtime`
- `CloudAI Benchmark Summary`

## 6) Run a Custom Workload

Use the Python runner directly:

```bash
python3 testbench/scripts/run_workload.py \
  --master-url http://localhost:8080 \
  --workload /absolute/path/to/your-workload.json \
  --timeout-seconds 1200 \
  --poll-interval 2 \
  --fail-on-task-failure
```

Expected workload JSON shape:

```json
{
  "name": "my-workload",
  "tasks": [
    {
      "docker_image": "cloudai-benchmark:1",
      "workflow_profile": "cpu-light",
      "workflow_args": {
        "seed": 101,
        "iterations": 18000,
        "sleep_ms": 90,
        "output": "cpu-light-01.csv"
      },
      "cpu_required": 1.0,
      "memory_required": 0.5,
      "storage_required": 1,
      "tag": "cpu-light",
      "k_value": 2.0
    }
  ]
}
```

Deterministic workflow image:

- image tag: `cloudai-benchmark:1`
- source: `testbench/workflow-image/`
- loader: `testbench/scripts/prepare_workflow_images.sh`
- profiles:
  - `cpu-light`
  - `cpu-heavy`
  - `memory-heavy`
  - `mixed`
  - `exit-nonzero`
  - `hang`
  - `slow-start`

When `workflow_profile` is present, `run_workload.py` generates the container command automatically.
You can still pass `command` directly if you need a one-off task.

## 7) Verify Cluster and Task State

Workers:

```bash
curl http://localhost:8080/api/workers | jq
```

Tasks:

```bash
curl http://localhost:8080/api/tasks | jq
curl http://localhost:8080/api/tasks/<task_id>/attempts | jq
curl http://localhost:8080/metrics | head
```

Master logs:

```bash
docker compose -f testbench/docker-compose.yml logs -f master
```

Worker logs:

```bash
docker compose -f testbench/docker-compose.yml logs -f worker-small
```

Failure helper workload:

```bash
python3 testbench/scripts/run_workload.py \
  --master-url http://localhost:8080 \
  --workload testbench/workloads/failure-helpers.json
```

## 8) Where Results Go

- Workload summary: `results/testbench/<timestamp>-summary.json`
- Observability export:
  - `results/testbench/<timestamp>-observability/prometheus-range.json`
  - `results/testbench/<timestamp>-observability/prometheus-instant.json`
  - `results/testbench/<timestamp>-observability/master-snapshot.json`
  - `results/testbench/<timestamp>-observability/metrics-summary.csv`
- Task-level outputs: stored by worker containers under `/var/cloudai/outputs` (inside worker volumes)

## 9) Tuning Heterogeneous Capacity

Edit `testbench/docker-compose.yml`:

- worker daemon limits (`worker-*-dind`): `cpus`, `mem_limit`, `pids_limit`
- worker advertised resources (`worker-*`):
  - `WORKER_TOTAL_CPU`
  - `WORKER_TOTAL_MEMORY_GB`
  - `WORKER_TOTAL_STORAGE_GB`

After editing, restart the stack:

```bash
make testbench-down
make testbench-up
make testbench-register
```

## 10) Troubleshooting

Workers not active:

```bash
testbench/scripts/register_workers.sh
docker compose -f testbench/docker-compose.yml logs -f worker-small worker-medium worker-large
```

Tasks stuck queued:

- confirm workers are `is_active=true` in `/api/workers`
- inspect master logs for scheduling/resource messages

Clean reset:

```bash
make testbench-down
docker compose -f testbench/docker-compose.yml down -v --remove-orphans
```
