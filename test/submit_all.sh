#!/usr/bin/env bash
set -euo pipefail

# Full workload submission and monitoring script
# Submits all 30 Docker images and tracks completion

API=${API:-http://localhost:8080}
MONGO=${MONGO:-mongodb://localhost:27017}
DB=${DB:-cloudai}
POLL_INTERVAL=${POLL_INTERVAL:-10}
TIMEOUT=${TIMEOUT:-1800}  # 30 minutes default timeout

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKLOAD_DIR="$SCRIPT_DIR/workloads"
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$LOG_DIR/submission_${TIMESTAMP}.log"
RESULTS_FILE="$LOG_DIR/results_${TIMESTAMP}.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log() {
  echo -e "${CYAN}[$(date +%H:%M:%S)]${NC} $*" | tee -a "$LOG_FILE"
}

log_success() {
  echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*" | tee -a "$LOG_FILE"
}

log_error() {
  echo -e "${RED}[$(date +%H:%M:%S)]${NC} $*" | tee -a "$LOG_FILE"
}

log_warn() {
  echo -e "${YELLOW}[$(date +%H:%M:%S)]${NC} $*" | tee -a "$LOG_FILE"
}

# Check prerequisites
check_prerequisites() {
  log "Checking prerequisites..."
  
  # Check API health
  if ! curl -s -f -m 5 "$API/health" >/dev/null 2>&1 && \
     ! curl -s -f -m 5 "$API/" >/dev/null 2>&1; then
    log_error "Master API unreachable at $API"
    return 1
  fi
  log_success "Master API is reachable"
  
  # Check MongoDB (optional but warn if missing)
  if command -v mongo >/dev/null 2>&1; then
    if mongo --quiet "$MONGO/$DB" --eval "db.runCommand({ping:1})" >/dev/null 2>&1; then
      log_success "MongoDB is accessible"
    else
      log_warn "MongoDB not accessible - monitoring will be limited"
    fi
  else
    log_warn "mongo CLI not found - will use HTTP API for monitoring"
  fi
  
  # Check for workload scripts
  for script in cpu_light cpu_heavy memory_heavy gpu_inference gpu_training mixed; do
    if [ ! -f "$WORKLOAD_DIR/submit_${script}.sh" ]; then
      log_error "Missing workload script: submit_${script}.sh"
      return 1
    fi
  done
  log_success "All workload scripts found"
  
  return 0
}

# Submit all workloads
submit_all_workloads() {
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  log "Starting full workload submission (30 tasks)"
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  
  local start_time=$(date +%s)
  local total_submitted=0
  local total_failed=0
  
  # Array of workload types and their scripts
  workloads=(
    "cpu_light:CPU-Light"
    "cpu_heavy:CPU-Heavy"
    "memory_heavy:Memory-Heavy"
    "gpu_inference:GPU-Inference"
    "gpu_training:GPU-Training"
    "mixed:Mixed"
  )
  
  for workload in "${workloads[@]}"; do
    script="${workload%%:*}"
    name="${workload##*:}"
    
    log ""
    log "📦 Submitting $name workload..."
    
    # Run the submission script and capture output
    if output=$("$WORKLOAD_DIR/submit_${script}.sh" 2>&1); then
      # Count successes from output
      submitted=$(echo "$output" | grep -c "✓ Task ID:" || echo 0)
      total_submitted=$((total_submitted + submitted))
      log_success "$name: $submitted tasks submitted"
    else
      log_error "$name: submission failed"
      failed=$(echo "$output" | grep -c "❌" || echo 5)
      total_failed=$((total_failed + failed))
    fi
  done
  
  local end_time=$(date +%s)
  local duration=$((end_time - start_time))
  
  log ""
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  log_success "Submission complete in ${duration}s"
  log "  ✓ Submitted: $total_submitted tasks"
  [ $total_failed -gt 0 ] && log_warn "  ✗ Failed: $total_failed tasks"
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  
  return 0
}

# Get task status from API
get_task_status_api() {
  curl -s "$API/api/tasks" 2>/dev/null || echo "[]"
}

# Get task status from MongoDB
get_task_status_mongo() {
  if ! command -v mongo >/dev/null 2>&1; then
    echo "[]"
    return
  fi
  
  mongo --quiet "$MONGO/$DB" --eval '
    db.results.aggregate([
      { $group: { 
          _id: "$status", 
          count: { $sum: 1 } 
      }}
    ]).toArray()' 2>/dev/null | jq -c '.' || echo "[]"
}

# Monitor task completion
monitor_completion() {
  log ""
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  log "Monitoring task completion (polling every ${POLL_INTERVAL}s)"
  log "Timeout: ${TIMEOUT}s"
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  
  local start_time=$(date +%s)
  local elapsed=0
  local last_completed=0
  local stall_count=0
  
  while [ $elapsed -lt $TIMEOUT ]; do
    # Try MongoDB first, fallback to API
    status=$(get_task_status_mongo)
    if [ "$status" = "[]" ]; then
      status=$(get_task_status_api)
    fi
    
    # Parse status counts
    if [ "$status" != "[]" ]; then
      completed=$(echo "$status" | jq '[.[] | select(._id=="completed")] | .[0].count // 0')
      running=$(echo "$status" | jq '[.[] | select(._id=="running")] | .[0].count // 0')
      pending=$(echo "$status" | jq '[.[] | select(._id=="pending" or ._id=="queued")] | .[0].count // 0')
      failed=$(echo "$status" | jq '[.[] | select(._id=="failed")] | .[0].count // 0')
      
      total=$((completed + running + pending + failed))
      
      # Progress bar
      if [ $total -gt 0 ]; then
        pct=$((completed * 100 / total))
        bar_len=$((pct / 2))
        bar=$(printf "%-50s" "$(printf '#%.0s' $(seq 1 $bar_len))")
        
        log "[$bar] $pct% | Completed: $completed | Running: $running | Pending: $pending | Failed: $failed"
        
        # Check if all complete
        if [ $completed -eq 30 ] || [ $((completed + failed)) -eq 30 ]; then
          log_success "All tasks finished!"
          break
        fi
        
        # Stall detection
        if [ $completed -eq $last_completed ]; then
          stall_count=$((stall_count + 1))
          if [ $stall_count -gt 12 ]; then  # 2 minutes with no progress
            log_warn "No progress for 2 minutes - tasks may be stalled"
            stall_count=0
          fi
        else
          stall_count=0
          last_completed=$completed
        fi
      else
        log "No tasks found in database yet..."
      fi
    else
      log_warn "Unable to fetch task status"
    fi
    
    sleep $POLL_INTERVAL
    elapsed=$(( $(date +%s) - start_time ))
  done
  
  if [ $elapsed -ge $TIMEOUT ]; then
    log_error "Timeout reached after ${TIMEOUT}s"
    return 1
  fi
  
  return 0
}

# Generate summary report
generate_report() {
  log ""
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  log "Generating summary report..."
  log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  
  # Get final status
  if command -v mongo >/dev/null 2>&1; then
    mongo --quiet "$MONGO/$DB" --eval "
      var summary = {
        timestamp: new Date(),
        total_tasks: 30,
        status_breakdown: db.results.aggregate([
          { \$group: { _id: \"\$status\", count: { \$sum: 1 } } }
        ]).toArray(),
        task_type_breakdown: db.results.aggregate([
          { \$group: { 
              _id: \"\$task_type\", 
              count: { \$sum: 1 },
              avg_duration: { \$avg: \"\$execution_time\" },
              sla_success_rate: { \$avg: { \$cond: [\"\$sla_success\", 1, 0] } }
          }}
        ]).toArray(),
        worker_distribution: db.results.aggregate([
          { \$group: { _id: \"\$worker_id\", count: { \$sum: 1 } } }
        ]).toArray()
      };
      printjson(summary);
    " 2>/dev/null > "$RESULTS_FILE"
    
    log_success "Report saved to: $RESULTS_FILE"
    
    # Display key metrics
    if [ -f "$RESULTS_FILE" ]; then
      log ""
      log "📊 Key Metrics:"
      jq -r '
        "  Total Tasks: \(.total_tasks)",
        "  Status Breakdown:",
        (.status_breakdown[] | "    - \(._id): \(.count)"),
        "",
        "  Task Types:",
        (.task_type_breakdown[] | "    - \(._id): \(.count) tasks, avg \(.avg_duration | tonumber | floor)s")
      ' "$RESULTS_FILE" | while read -r line; do log "$line"; done
    fi
  else
    log_warn "MongoDB CLI not available - skipping detailed report"
  fi
  
  log ""
  log_success "Full log saved to: $LOG_FILE"
}

# Main execution
main() {
  echo ""
  echo "╔════════════════════════════════════════════════════╗"
  echo "║   CloudAI Full Workload Test - 30 Docker Images   ║"
  echo "╚════════════════════════════════════════════════════╝"
  echo ""
  
  log "Starting test run at $(date)"
  log "Master API: $API"
  log "MongoDB: $MONGO"
  log ""
  
  # Check prerequisites
  if ! check_prerequisites; then
    log_error "Prerequisites check failed"
    exit 1
  fi
  
  # Submit workloads
  if ! submit_all_workloads; then
    log_error "Workload submission failed"
    exit 1
  fi
  
  # Monitor completion
  if ! monitor_completion; then
    log_warn "Monitoring timed out or failed"
  fi
  
  # Generate report
  generate_report
  
  log ""
  log_success "Test run complete!"
  echo ""
}

# Run main with signal handling
trap 'log_error "Interrupted by user"; exit 130' INT TERM
main "$@"
