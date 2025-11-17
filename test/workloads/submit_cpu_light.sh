#!/usr/bin/env bash
set -euo pipefail
API=${API:-http://localhost:8080}

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

images=(
  "moinvinchhi/cloudai-cpu-light:1"
  "moinvinchhi/cloudai-cpu-light:2"
  "moinvinchhi/cloudai-cpu-light:3"
  "moinvinchhi/cloudai-cpu-light:4"
  "moinvinchhi/cloudai-cpu-light:5"
)

echo "🚀 Submitting CPU-Light workloads (5 tasks)..."
check_api_health || exit 1

submitted=0
failed=0

# CPU-light: small cpu, small mem
for img in "${images[@]}"; do
  echo -n "  Submitting $img... "
  if task_id=$(submit_task_with_retry "$img" 1.0 2.0 0.0); then
    echo "✓ Task ID: $task_id"
    submitted=$((submitted + 1))
  else
    failed=$((failed + 1))
  fi
  sleep 0.2
done

echo ""
echo "✅ CPU-Light: $submitted submitted, $failed failed"
