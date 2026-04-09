# CloudAI Workflow Image

This directory contains the deterministic workflow image used by the testbench evidence campaign.

Image tag:

- `cloudai-benchmark:1`

Supported profiles:

- `cpu-light`
- `cpu-heavy`
- `memory-heavy`
- `mixed`
- `exit-nonzero`
- `hang`
- `slow-start`

Example:

```bash
cloudai-benchmark cpu-heavy --seconds 8 --label burst-a
```
