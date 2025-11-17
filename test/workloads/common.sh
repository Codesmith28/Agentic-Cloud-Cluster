#!/usr/bin/env bash
# Common functions for workload submission scripts

# Retry logic with exponential backoff
submit_task_with_retry() {
  local img="$1"
  local cpu="$2"
  local mem="$3"
  local gpu="$4"
  local max_retries=5
  local retry_count=0
  local backoff=1

  while [ $retry_count -lt $max_retries ]; do
    response=$(curl -s -w "\n%{http_code}" -X POST "$API/api/tasks" \
      -H "Content-Type: application/json" \
      -d "{\"docker_image\": \"$img\", \"cpu_required\": $cpu, \"memory_required\": $mem, \"gpu_required\": $gpu}" 2>&1)
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "201" ]; then
      echo "$body" | jq -r '.task_id // .id // "submitted"'
      return 0
    else
      retry_count=$((retry_count + 1))
      if [ $retry_count -lt $max_retries ]; then
        echo "⚠️  Retry $retry_count/$max_retries for $img (HTTP $http_code)" >&2
        sleep $backoff
        backoff=$((backoff * 2))
      fi
    fi
  done
  
  echo "❌ Failed to submit $img after $max_retries attempts" >&2
  return 1
}

# Check if API is reachable
check_api_health() {
  if ! curl -s -f -m 5 "$API/health" >/dev/null 2>&1; then
    # Try root endpoint as fallback
    if ! curl -s -f -m 5 "$API/" >/dev/null 2>&1; then
      echo "❌ Master API unreachable at $API" >&2
      return 1
    fi
  fi
  return 0
}

# Export functions for use in other scripts
export -f submit_task_with_retry
export -f check_api_health
