# Docker Testbench for Heterogeneous Workers

This testbench provides an automated performance harness for CloudAI:

- `master` and `mongo` run in containers
- each `worker-*` runs in its own container
- each worker has its own Docker daemon (`worker-*-dind`) so task containers are isolated per worker node
- worker capabilities are intentionally heterogeneous via CPU/memory limits and explicit resource overrides
- deterministic workflow image (`cloudai/workflow-deterministic:v1`) enables repeatable load profiles

Detailed step-by-step instructions: [`docs/TESTBENCH_RUNBOOK.md`](../docs/TESTBENCH_RUNBOOK.md)

## Evidence Benchmark Campaign

The testbench includes a full evidence benchmark campaign runner that orchestrates multi-scenario testing:

```bash
make campaign              # Smoke run (heterogeneous-smoke workload only)
make campaign-full         # Full campaign (all workloads + all scenarios)
python3 testbench/scripts/run_campaign.py --help  # See all options
```

**Campaign Features:**
- Runs baseline, burst, overload, and failure-stressed scenarios
- Tests multiple schedulers: RR, RTS, PPO-pretrained, PPO-adapted, and recovery variants
- Injects controlled failures (worker kill, DinD pause/resume, master restart)
- Exports Prometheus metrics and observability artifacts to `results/campaign/`
- Generates markdown + HTML evidence reports

See [`docs/TESTBENCH_RUNBOOK.md`](../docs/TESTBENCH_RUNBOOK.md) for detailed campaign documentation.

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
make testbench-suite
```

This runs:

1. `docker compose -f testbench/docker-compose.yml up -d --build`
2. automatic registration of `worker-small`, `worker-medium`, `worker-large`
3. workload submission + polling until completion
4. summary JSON output under `results/testbench/`

## Manual Flow

```bash
make testbench-up
make testbench-register
make testbench-workload
make testbench-down
```

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
docker compose -f testbench/docker-compose.yml logs -f worker-small
```
