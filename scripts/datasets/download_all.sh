#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — Dataset Download & Staging Orchestrator
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
log "  Agentic Cloud Cluster — Staging All Cloud Datasets (Local)"
log "═══════════════════════════════════════════════════════════════"

mkdir -p "${REPO_ROOT}/testbench/data/raw"

# 1. Stage Microsoft Azure VM Dataset
log "1/3 Staging Microsoft Azure Public Dataset..."
"${PYTHON}" scripts/datasets/download_azure.py "$@"

# 2. Stage Alibaba Cluster Trace Dataset
log "2/3 Staging Alibaba Cluster Trace Dataset..."
"${PYTHON}" scripts/datasets/download_alibaba.py --count 10000

# 3. Stage Google ClusterData 2019 Dataset
log "3/3 Staging Google ClusterData 2019 Dataset..."
"${PYTHON}" scripts/datasets/download_google.py --count 10000

ok "All production datasets staged into testbench/data/raw/ (git-ignored)!"
log "Available mapped datasets for benchmarking:"
log "  - Azure:   testbench/data/raw/azure_vmtable_full.csv.gz (mapping: testbench/configs/azure_mapping.yaml)"
log "  - Alibaba: testbench/data/raw/alibaba_batch_task.csv   (mapping: testbench/configs/alibaba_mapping.yaml)"
log "  - Google:  testbench/data/raw/google_clusterdata.csv    (mapping: testbench/configs/google_mapping.yaml)"
