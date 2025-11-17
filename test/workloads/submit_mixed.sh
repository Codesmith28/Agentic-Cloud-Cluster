#!/usr/bin/env bash
set -euo pipefail
API=${API:-http://localhost:8080}

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

images=(
  "moinvinchhi/cloudai-mixed:1"
  "moinvinchhi/cloudai-mixed:2"
  "moinvinchhi/cloudai-mixed:3"
  "moinvinchhi/cloudai-mixed:4"
  "moinvinchhi/cloudai-mixed:5"
)

echo "🚀 Submitting Mixed workloads (5 tasks)..."
check_api_health || exit 1

submitted=0
failed=0

# Mixed workloads: varied resources
for img in "${images[@]}"; do
  cpu=$(awk -v min=1 -v max=4 'BEGIN{srand(); print int(min+rand()*(max-min+1))}')
  mem=$(awk -v min=2 -v max=12 'BEGIN{srand(); print int(min+rand()*(max-min+1))}')
  gpu=$(awk -v min=0 -v max=1 'BEGIN{srand(); print rand() < 0.3 ? 1 : 0}')
  
  echo -n "  Submitting $img (cpu=$cpu mem=$mem gpu=$gpu)... "
  if task_id=$(submit_task_with_retry "$img" "$cpu" "$mem" "$gpu"); then
    echo "✓ Task ID: $task_id"
    submitted=$((submitted + 1))
  else
    failed=$((failed + 1))
  fi
  sleep 0.7
done

echo ""
echo "✅ Mixed: $submitted submitted, $failed failed"
