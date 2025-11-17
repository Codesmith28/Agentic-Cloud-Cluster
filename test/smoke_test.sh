#!/usr/bin/env bash
set -euo pipefail

# Quick smoke test - submits 2 tasks and verifies basic functionality

API=${API:-http://localhost:8080}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║      CloudAI Quick Smoke Test (2 tasks)       ║"
echo "╚════════════════════════════════════════════════╝"
echo ""

# Source common functions
source "$SCRIPT_DIR/workloads/common.sh"

# Check health
echo "🔍 Checking master API..."
if ! check_api_health; then
  echo -e "${RED}✗ Master API not reachable at $API${NC}"
  exit 1
fi
echo -e "${GREEN}✓ Master API is healthy${NC}"
echo ""

# Submit one cpu-light task
echo "📦 Test 1: Submitting CPU-light task..."
if task_id=$(submit_task_with_retry "moinvinchhi/cloudai-cpu-light:1" 1.0 2.0 0.0); then
  echo -e "${GREEN}✓ CPU-light task submitted: $task_id${NC}"
else
  echo -e "${RED}✗ Failed to submit CPU-light task${NC}"
  exit 1
fi

echo ""
sleep 1

# Submit one cpu-heavy task
echo "📦 Test 2: Submitting CPU-heavy task..."
if task_id=$(submit_task_with_retry "moinvinchhi/cloudai-cpu-heavy:1" 4.0 4.0 0.0); then
  echo -e "${GREEN}✓ CPU-heavy task submitted: $task_id${NC}"
else
  echo -e "${RED}✗ Failed to submit CPU-heavy task${NC}"
  exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✓ Smoke test passed!${NC}"
echo ""
echo "Waiting 5 seconds for tasks to be processed..."
sleep 5

# Try to fetch task status
echo ""
echo "📊 Checking task status from API..."
if tasks=$(curl -s "$API/api/tasks" 2>/dev/null); then
  count=$(echo "$tasks" | jq '. | length')
  echo "  Found $count tasks in system"
  echo "$tasks" | jq -r '.[] | "  - Task \(.id // .task_id): \(.status // "unknown")"' | head -10
else
  echo -e "${YELLOW}⚠  Could not fetch task status (API endpoint may differ)${NC}"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}Smoke test complete!${NC}"
echo ""
echo "Next steps:"
echo "  1. Check master logs to see task processing"
echo "  2. Run full test suite: bash test/submit_all.sh"
echo "  3. Verify results: bash test/verify/check_task_distribution.sh"
echo ""
