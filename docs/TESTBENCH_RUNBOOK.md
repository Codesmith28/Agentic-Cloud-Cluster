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

- `testbench-suite` (recommended): start stack + prepare workflow images + register workers + run default workload
- `campaign` (smoke): evidence benchmark across schedulers and scenarios using heterogeneous-smoke workload
- `campaign-full`: full evidence benchmark using multiple workloads and scenarios
- manual flow: run each stage independently

## 3) Deterministic Workflow Image

The testbench uses a deterministic workflow image (`cloudai/workflow-deterministic:v1`) to generate repeatable load profiles for performance benchmarking.

### Building and Preparing the Workflow Image

Before running workloads or campaigns, prepare the image in worker DinD daemons:

```bash
make testbench-prepare-images
```

This script:
1. Builds the deterministic workflow Docker image from `testbench/workflow-image/`
2. Loads the image into each worker DinD daemon (worker-small, worker-medium, worker-large)
3. Verifies the image is ready for task execution

**Configuration:**
- `IMAGE_TAG`: Override the image tag (default: `cloudai/workflow-deterministic:v1`)
- `CLOUDAI_WORKFLOW_IMAGE_TAG`: Alternative env var for the tag
- `SKIP_BUILD`: Set to `true` to skip building and only load existing images

## 4) One-Command Run (Recommended)

From repository root:

```bash
make testbench-suite
```

What it does:

1. Builds and starts `mongo`, `master`, `worker-small`, `worker-medium`, `worker-large`
2. Prepares deterministic workflow image in worker DinD daemons
3. Registers workers through `POST /api/workers`
4. Submits workload from `testbench/workloads/heterogeneous-smoke.json`
5. Polls task status until completion
6. Writes summary JSON to `results/testbench/<timestamp>-summary.json`

## 5) Manual Stage-by-Stage Run

Start stack:

```bash
make testbench-up
```

Prepare workflow image:

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

## 6) Evidence Benchmark Campaign

Run multi-scenario benchmarks across schedulers with failure injection:

### Smoke Campaign (Quick Benchmark)

```bash
make campaign
```

Runs baseline, burst, overload, and failure-stressed scenarios using the heterogeneous-smoke workload.

### Full Campaign (Comprehensive Benchmark)

```bash
make campaign-full
```

Runs all scenarios across multiple workload profiles (heterogeneous-smoke, deterministic-full).

### Campaign Command-Line Options

```bash
python3 testbench/scripts/run_campaign.py \
  --master-url http://localhost:8080 \
  --schedulers "RR,RTS,PPO-pretrained,PPO-adapted,RR+recovery,RTS+recovery,PPO+recovery" \
  --scenarios all \
  --workloads "heterogeneous-smoke,deterministic-full" \
  --output-dir results/campaign \
  --prometheus-url http://localhost:9090 \
  --skip-observability-export false
```

**Key Options:**
- `--schedulers`: Comma-separated list of schedulers to test (default: all 7 variants)
- `--scenarios`: `baseline`, `burst`, `overload`, `failure-stressed`, or `all`
- `--workloads`: Comma-separated workload names from `testbench/workloads/`
- `--output-dir`: Directory for campaign results (default: `results/campaign`)
- `--prometheus-url`: Prometheus API endpoint for observability exports
- `--skip-observability-export`: Skip Prometheus/master metrics export (default: false)

**Recovery Scheduler Labels:**
- `RR+recovery`, `RTS+recovery`, `PPO+recovery`: Task prioritization and failure handling during recovery scenarios

## 7) Run a Custom Workload

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
      "docker_image": "cloudai/workflow-deterministic:v1",
      "workflow": {
        "profile": "cpu-light",
        "args": {
          "duration": 30,
          "workers": 2
        }
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

Alternatively, use `docker_image` + `command` for standard containers:

```json
{
  "tasks": [
    {
      "docker_image": "ubuntu:latest",
      "command": "echo 'hello world'",
      "cpu_required": 0.5,
      "memory_required": 0.256
    }
  ]
}
```

## 8) Failure Injection

The campaign framework injects controlled failures during failure-stressed scenarios:

```bash
python3 testbench/scripts/failure_injector.py \
  --action kill-worker \
  --worker worker-small \
  --delay-seconds 5
```

**Available Failure Actions:**
- `kill-worker`: Terminate the worker container
- `kill-dind`: Terminate the DinD daemon (causes containerized tasks to fail)
- `pause-worker-dind`: Pause the DinD daemon (tasks hang)
- `resume-worker-dind`: Resume a paused DinD daemon
- `restart-master`: Restart the master container
- `bad-image-tag`: Inject bad-image tasks (encoded in workload definitions)
- `replay-stale-result`: Stale-result replay (scenario-level orchestration)

Recovery schedulers use failure injection to test robustness during failure-stressed scenarios.

## 9) Observability: Prometheus, Metrics, and Dashboards

The master node exports real-time observability data:

### Master Telemetry Endpoints

```bash
# Health check
curl http://localhost:8080/health | jq

# Cluster telemetry (REST)
curl http://localhost:8080/telemetry | jq

# Per-worker telemetry (REST)
curl http://localhost:8080/telemetry/worker-1 | jq

# All workers summary
curl http://localhost:8080/workers | jq

# Prometheus metrics
curl http://localhost:8080/metrics

# WebSocket: Real-time all-workers telemetry
wscat -c ws://localhost:8080/ws/telemetry

# WebSocket: Real-time per-worker telemetry
wscat -c ws://localhost:8080/ws/telemetry/worker-1
```

### Campaign Observability Export

After campaign completion, export Prometheus metrics and diagnostics:

```bash
python3 testbench/scripts/export_metrics.py \
  --prometheus-url http://localhost:9090 \
  --output-dir results/campaign/observability \
  --start-time "$(date -u -d '1 hour ago' +%s)" \
  --end-time "$(date -u +%s)" \
  --step-seconds 10
```

This exports:
- Time-series Prometheus data (range queries)
- Final metric snapshots (instant queries)
- Scheduler performance metrics
- Resource utilization traces

Results are written to `prometheus-range.json` and `prometheus-instant.json`.

## 10) Verify Cluster and Task State

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

## 11) Where Results Go

- Workload summary: `results/testbench/<timestamp>-summary.json`
- Campaign report (markdown): `results/campaign/<timestamp>-report.md`
- Campaign report (HTML): `results/campaign/<timestamp>-report.html`
- Campaign JSON results: `results/campaign/<timestamp>-results.json`
- Observability artifacts: `results/campaign/observability/prometheus-*.json`
- Task-level outputs: stored by worker containers under `/var/cloudai/outputs` (inside worker volumes)

## 12) Tuning Heterogeneous Capacity

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
make testbench-prepare-images
make testbench-register
```

## 13) Troubleshooting

Workers not active:

```bash
testbench/scripts/register_workers.sh
docker compose -f testbench/docker-compose.yml logs -f worker-small worker-medium worker-large
```

Tasks stuck queued:

- confirm workers are `is_active=true` in `/api/workers`
- inspect master logs for scheduling/resource messages

Workflow image not found in workers:

```bash
make testbench-prepare-images
docker compose -f testbench/docker-compose.yml logs -f prepare_images
```

Clean reset:

```bash
make testbench-down
docker compose -f testbench/docker-compose.yml down -v --remove-orphans
```
