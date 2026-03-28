# CloudAI Testbench Runbook

This runbook shows exactly how to run the Docker-based CloudAI testbenches for repeatable performance testing.

## 1) Prerequisites

- Docker Desktop or Docker Engine with `docker compose`
- Python 3.8+
- Optional: `jq` for pretty JSON output

Quick check:

```bash
docker --version
docker compose version
python3 --version
```

## 2) Testbench Types

- `testbench-suite` (recommended): start stack + register workers + run default workload
- manual flow: run each stage independently
- custom workload: run your own workload JSON file

## 3) One-Command Run (Recommended)

From repository root:

```bash
make testbench-suite
```

What it does:

1. Builds and starts `mongo`, `master`, `worker-small`, `worker-medium`, `worker-large`
2. Registers workers through `POST /api/workers`
3. Submits workload from `testbench/workloads/heterogeneous-smoke.json`
4. Polls task status until completion
5. Writes summary JSON to `results/testbench/<timestamp>-summary.json`

## 4) Manual Stage-by-Stage Run

Start stack:

```bash
make testbench-up
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

## 5) Run a Custom Workload

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
      "docker_image": "moinvinchhi/cloudai-cpu-light:1",
      "cpu_required": 1.0,
      "memory_required": 0.5,
      "storage_required": 1,
      "tag": "cpu-light",
      "k_value": 2.0
    }
  ]
}
```

Each task type has 5 versioned images on Docker Hub (see `DOCKER_IMAGES.txt`).
The `command` field is optional — when omitted, the image's built-in entrypoint runs.

## 6) Verify Cluster and Task State

Workers:

```bash
curl http://localhost:8080/api/workers | jq
```

Tasks:

```bash
curl http://localhost:8080/api/tasks | jq
curl http://localhost:8080/api/tasks/<task_id>/attempts | jq
```

Master logs:

```bash
docker compose -f testbench/docker-compose.yml logs -f master
```

Worker logs:

```bash
docker compose -f testbench/docker-compose.yml logs -f worker-small
```

## 7) Where Results Go

- Workload summary: `results/testbench/<timestamp>-summary.json`
- Task-level outputs: stored by worker containers under `/var/cloudai/outputs` (inside worker volumes)

## 8) Tuning Heterogeneous Capacity

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

## 9) Troubleshooting

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
