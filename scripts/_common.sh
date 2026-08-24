#!/usr/bin/env bash
# =============================================================================
# Agentic Cloud Cluster — Common Shell Utilities & Defaults
# =============================================================================

# Resolve repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Color formatters
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

log()  { echo -e "${CYAN}▶${RESET} $*"; }
ok()   { echo -e "${GREEN}✓${RESET} $*"; }
warn() { echo -e "${YELLOW}⚠${RESET} $*"; }
err()  { echo -e "${RED}✗${RESET} $*" >&2; }
die()  { err "$*"; exit 1; }

# Python executable resolver
resolve_python() {
  if [[ -f "${REPO_ROOT}/venv/bin/python3" ]]; then
    echo "${REPO_ROOT}/venv/bin/python3"
  elif command -v python3 >/dev/null 2>&1; then
    command -v python3
  else
    die "Python 3 is required but not found"
  fi
}
