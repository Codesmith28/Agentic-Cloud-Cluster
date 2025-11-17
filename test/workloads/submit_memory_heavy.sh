#!/usr/bin/env bash
set -euo pipefail
API=${API:-http://localhost:8080}

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

images=(
  "moinvinchhi/cloudai-memory-heavy:1"
  "moinvinchhi/cloudai-memory-heavy:2"
  "moinvinchhi/cloudai-memory-heavy:3"
  "moinvinchhi/cloudai-memory-heavy:4"
  "moinvinchhi/cloudai-memory-heavy:5"
)

echo "🚀 Submitting Memory-Heavy workloads (5 tasks)..."
check_api_health || exit 1

submitted=0
failed=0

# Memory-heavy: low cpu, high mem
for img in "${images[@]}"; do
  echo -n "  Submitting $img... "
  if task_id=$(submit_task_with_retry "$img" 1.0 8.0 0.0); then
    echo "✓ Task ID: $task_id"
    submitted=$((submitted + 1))
  else
    failed=$((failed + 1))
  fi
  sleep 0.5
done

echo ""
echo "✅ Memory-Heavy: $submitted submitted, $failed failed"
