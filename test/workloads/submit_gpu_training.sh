#!/usr/bin/env bash
set -euo pipefail
API=${API:-http://localhost:8080}

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

images=(
  "moinvinchhi/cloudai-gpu-training:1"
  "moinvinchhi/cloudai-gpu-training:2"
  "moinvinchhi/cloudai-gpu-training:3"
  "moinvinchhi/cloudai-gpu-training:4"
  "moinvinchhi/cloudai-gpu-training:5"
)

echo "🚀 Submitting GPU-Training workloads (5 tasks)..."
check_api_health || exit 1

submitted=0
failed=0

# GPU-training: heavy gpu, high mem
for img in "${images[@]}"; do
  echo -n "  Submitting $img... "
  if task_id=$(submit_task_with_retry "$img" 4.0 12.0 1.0); then
    echo "✓ Task ID: $task_id"
    submitted=$((submitted + 1))
  else
    failed=$((failed + 1))
  fi
  sleep 1
done

echo ""
echo "✅ GPU-Training: $submitted submitted, $failed failed"
