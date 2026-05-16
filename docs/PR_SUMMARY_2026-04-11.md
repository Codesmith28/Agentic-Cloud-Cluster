# PR Work Summary - 2026-04-11 (UTC)

## Snapshot

- Merged PRs: 2 (`#39`, `#40`)
- Total change volume: 258 files, `+62,053` / `-753`
- Main themes: PPO scheduling/training improvements, checkpoint/model lifecycle upgrades, and publication of benchmark evidence.

## PR #39 - feat(agentic_scheduler): add resumable checkpointing and GPU preference

- **Merged:** 2026-04-11 09:01 UTC
- **Scope:** 127 files, `+23,232` / `-619`
- Added resumable offline PPO training with periodic local `.pkl` checkpoints and resume controls (`agentic_scheduler/train_ppo.py`).
- Added Mongo-backed checkpoint persistence/resume paths and GPU-preferred execution with CPU fallback in scheduler runtime (`agentic_scheduler/service.py`, `agentic_scheduler/server.py`).
- Added testbench/integration workflow orchestration and new master-side test workflow engine/CLI surfaces (`.github/workflows/testbench-integration.yml`, `master/internal/testworkflow/*`, `master/internal/cli/test_workflow*`).
- Expanded testbench scenarios and scripts for suite, reliability, integration, and UI smoke paths (`testbench/scenarios/*`, `testbench/scripts/*`).
- Updated docs and operational wiring (`docs/PPO_TRACE_REPLAY.md`, `docs/TESTBENCH_RUNBOOK.md`, `testbench/README.md`, `Makefile`, `master/main.go`, `master/internal/config/config.go`).
- Added substantial evidence artifacts/logs under `results/`.

## PR #40 - feat: improve PPO scheduling and publish benchmark evidence

- **Merged:** 2026-04-11 19:03 UTC
- **Scope:** 131 files, `+38,821` / `-134`
- Improved PPO model/runtime behavior and defaults, including latest-model selection/persistence behavior at service lifecycle boundaries (`agentic_scheduler/model.py`, `agentic_scheduler/service.py`, `agentic_scheduler/server.py`, `agentic_scheduler/train_ppo.py`).
- Tuned trace replay/training surfaces for Alibaba trace-based workflows (`agentic_scheduler/training/trace_loader.py`, `agentic_scheduler/training/trace_replay_env.py`).
- Added multiple PPO checkpoint variants and optimization artifacts (`agentic_scheduler/models/*`).
- Published optimization documentation and consolidated reporting (`docs/PPO_PERFORMANCE_OPTIMIZATION.md`, `docs/PPO_OPTIMIZATION_CONSOLIDATED_REPORT_2026-04-11.md`, updates to `docs/PPO_TRACE_REPLAY.md`).
- Updated runtime/config glue in master/worker paths and startup scripts (`master/internal/config/config.go`, `runMaster.sh`, `worker/internal/system/runtime_config*.go`, `worker/internal/executor/executor.go`), plus UI API/build updates (`ui/src/api/client.js`, `ui/vite.config.js`, `ui/dist/*`).
- Added repeated PPO-vs-RTS benchmark runs and logs under `results/`, including optimized repeated-run evidence.

## PR links

- https://github.com/Codesmith28/Agentic-Cloud-Cluster/pull/39
- https://github.com/Codesmith28/Agentic-Cloud-Cluster/pull/40
