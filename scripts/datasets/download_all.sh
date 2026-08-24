#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — Full Dataset Download & Staging Orchestrator
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}"

# Source shared helpers
# shellcheck source=../_common.sh
source "${SCRIPT_DIR}/../_common.sh"

PYTHON="$(resolve_python)"

log "═══════════════════════════════════════════════════════════════"
log "  Agentic Cloud Cluster — Staging Full Datasets (Azure & Alibaba)"
log "═══════════════════════════════════════════════════════════════"

mkdir -p "${REPO_ROOT}/testbench/data/raw"

# 1. Stage Full Microsoft Azure VM Dataset (2,695,548 records)
log "1/2 Staging Full Microsoft Azure VM Dataset (2,695,548 records)..."
"${PYTHON}" scripts/datasets/download_azure.py "$@"

# 2. Stage Full Alibaba Cluster Trace v2018 Dataset (300,000 tasks)
log "2/2 Staging Full Alibaba Cluster Trace v2018 Dataset (300,000 tasks)..."
"${PYTHON}" scripts/datasets/download_alibaba.py --count 300000

ok "All production datasets successfully staged into testbench/data/raw/ (git-ignored)!"
log "Available mapped datasets for benchmarking:"
log "  - Azure (Full):   testbench/data/raw/azure_vmtable_full.csv.gz (mapping: testbench/configs/azure_mapping.yaml)"
log "  - Alibaba (Full): testbench/data/raw/alibaba_batch_task.csv   (mapping: testbench/configs/alibaba_mapping.yaml)"
