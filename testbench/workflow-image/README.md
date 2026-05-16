# Deterministic workflow image

This directory contains the repo-owned benchmark workflow image used by the testbench.

## Image tag

- Default pinned tag: `cloudai/workflow-deterministic:v1`

## Build locally

```bash
docker build -t cloudai/workflow-deterministic:v1 testbench/workflow-image
```

## Prepare DinD workers

```bash
testbench/scripts/prepare_workflow_images.sh
```

The preparation script builds the image and loads it into each worker DinD daemon used by `testbench/docker-compose.yml`.

## Workflow profiles/subcommands

The image includes `/usr/local/bin/workflow` (and defaults to `cpu-light`) with:

- `cpu-light`
- `cpu-heavy`
- `memory-heavy`
- `mixed`
- `exit-nonzero`
- `hang`
- `slow-start`

Every profile accepts:

- `--seed <int>` (default `1337`)
- `--result-file <path>` (default `/output/workflow-result.json`)

Examples:

```bash
# Deterministic CPU-light profile
docker run --rm cloudai/workflow-deterministic:v1 cpu-light --iterations 500000 --seed 101

# Deterministic memory-heavy profile
docker run --rm cloudai/workflow-deterministic:v1 memory-heavy --memory-mib 384 --passes 2 --seed 404

# Failure helper
docker run --rm cloudai/workflow-deterministic:v1 exit-nonzero --code 23
```
