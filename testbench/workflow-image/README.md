# CloudAI Deterministic Workflow Image

This directory contains the repo-owned benchmark image used by the Docker testbench.

- image tag: `cloudai-benchmark:1`
- build script: `testbench/scripts/prepare_workflow_images.sh`
- build target: each `worker-*-dind` daemon

Supported profiles:

- `cpu-light`
- `cpu-heavy`
- `memory-heavy`
- `mixed`
- `exit-nonzero`
- `hang`
- `slow-start`

Example command:

```bash
cloudai-benchmark cpu-heavy --seed 203 --iterations 12000 --output cpu-heavy-03.csv
```
