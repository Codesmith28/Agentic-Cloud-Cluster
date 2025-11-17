#!/usr/bin/env bash
set -euo pipefail
API=${API:-http://localhost:8080}

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

images=(
  "moinvinchhi/cloudai-gpu-inference:1"
  "moinvinchhi/cloudai-gpu-inference:2"
  "moinvinchhi/cloudai-gpu-inference:3"
  "moinvinchhi/cloudai-gpu-inference:4"
  "moinvinchhi/cloudai-gpu-inference:5"
)

echo "🚀 Submitting GPU-Inference workloads (5 tasks)..."
check_api_health || exit 1

submitted=0
failed=0

# GPU-inference: small gpu, low mem
for img in "${images[@]}"; do
  echo -n "  Submitting $img... "
  if task_id=$(submit_task_with_retry "$img" 1.0 4.0 0.5); then
    echo "✓ Task ID: $task_id"
    submitted=$((submitted + 1))
  else
    failed=$((failed + 1))
  fi
  sleep 0.5
done

echo ""
echo "✅ GPU-Inference: $submitted submitted, $failed failed"
