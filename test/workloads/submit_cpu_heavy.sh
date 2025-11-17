#!/usr/bin/env bash
set -euo pipefail
API=${API:-http://localhost:8080}

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

images=(
  "moinvinchhi/cloudai-cpu-heavy:1"
  "moinvinchhi/cloudai-cpu-heavy:2"
  "moinvinchhi/cloudai-cpu-heavy:3"
  "moinvinchhi/cloudai-cpu-heavy:4"
  "moinvinchhi/cloudai-cpu-heavy:5"
)

echo "🚀 Submitting CPU-Heavy workloads (5 tasks)..."
check_api_health || exit 1

submitted=0
failed=0

# CPU-heavy: moderate cpu, moderate mem
for img in "${images[@]}"; do
  echo -n "  Submitting $img... "
  if task_id=$(submit_task_with_retry "$img" 4.0 4.0 0.0); then
    echo "✓ Task ID: $task_id"
    submitted=$((submitted + 1))
  else
    failed=$((failed + 1))
  fi
  sleep 0.5
done

echo ""
echo "✅ CPU-Heavy: $submitted submitted, $failed failed"
