#!/bin/bash

MASTER_ADDR="localhost:8080"
RESULTS_FILE="$1/metrics/stress_results.json"

echo "=========================================="
echo "CloudAI Comprehensive Stress Test"
echo "=========================================="

# Phase 1: Health Check
echo ""
echo "=== PHASE 1: Health Check ==="
HEALTH=$(curl -s -m 5 http://$MASTER_ADDR/health 2>/dev/null | wc -c)
if [ $HEALTH -gt 10 ]; then
  echo "✓ Master is healthy"
else
  echo "✗ Master health check failed"
  exit 1
fi

# Phase 2: Register Workers
echo ""
echo "=== PHASE 2: Registering Workers ==="
curl -s -X POST http://$MASTER_ADDR/api/register -H "Content-Type: application/json" \
  -d '{"worker_id":"worker-1","address":"10.225.184.232:50052"}' > /dev/null 2>&1
sleep 1
curl -s -X POST http://$MASTER_ADDR/api/register -H "Content-Type: application/json" \
  -d '{"worker_id":"worker-2","address":"10.225.184.232:50053"}' > /dev/null 2>&1
sleep 1
curl -s -X POST http://$MASTER_ADDR/api/register -H "Content-Type: application/json" \
  -d '{"worker_id":"worker-3","address":"10.225.184.232:50054"}' > /dev/null 2>&1

sleep 5
WORKERS=$(curl -s http://$MASTER_ADDR/workers 2>/dev/null | jq '.workers | length' 2>/dev/null || echo "0")
echo "✓ Registered $WORKERS workers"

# Phase 3: Load Test 1 - Light (50 tasks)
echo ""
echo "=== PHASE 3: Light Load (50 tasks) ==="
START_TIME=$(date +%s%N)
for i in {1..50}; do
  curl -s -X POST http://$MASTER_ADDR/api/tasks \
    -H "Content-Type: application/json" \
    -d "{\"docker_image\":\"ubuntu:latest\",\"command\":\"echo 'Task $i'\",\"cpu_required\":0.5,\"memory_required\":256}" \
    > /dev/null 2>&1 &
  
  if [ $((i % 10)) -eq 0 ]; then
    echo "  Submitted $i/50 tasks..."
  fi
done
wait
END_TIME=$(date +%s%N)
DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))
THROUGHPUT=$(echo "scale=2; 50000 / $DURATION_MS" | bc 2>/dev/null || echo "N/A")
echo "✓ Phase 3 complete: ${DURATION_MS}ms, Throughput: ${THROUGHPUT} tasks/sec"

# Monitor for 15 seconds
echo "  Monitoring execution..."
sleep 15

# Phase 4: Load Test 2 - Medium (150 tasks)
echo ""
echo "=== PHASE 4: Medium Load (150 tasks) ==="
START_TIME=$(date +%s%N)
for i in {1..150}; do
  curl -s -X POST http://$MASTER_ADDR/api/tasks \
    -H "Content-Type: application/json" \
    -d "{\"docker_image\":\"ubuntu:latest\",\"command\":\"echo 'Task $i'\",\"cpu_required\":0.5,\"memory_required\":256}" \
    > /dev/null 2>&1 &
  
  if [ $((i % 30)) -eq 0 ]; then
    echo "  Submitted $i/150 tasks..."
  fi
done
wait
END_TIME=$(date +%s%N)
DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))
echo "✓ Phase 4 complete: ${DURATION_MS}ms"

# Monitor for 20 seconds
echo "  Monitoring execution..."
sleep 20

# Phase 5: Load Test 3 - Heavy (300 tasks)
echo ""
echo "=== PHASE 5: Heavy Load (300 tasks) ==="
START_TIME=$(date +%s%N)
for i in {1..300}; do
  curl -s -X POST http://$MASTER_ADDR/api/tasks \
    -H "Content-Type: application/json" \
    -d "{\"docker_image\":\"ubuntu:latest\",\"command\":\"echo 'Task $i'\",\"cpu_required\":0.5,\"memory_required\":256}" \
    > /dev/null 2>&1 &
  
  if [ $((i % 50)) -eq 0 ]; then
    echo "  Submitted $i/300 tasks..."
  fi
done
wait
END_TIME=$(date +%s%N)
DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))
echo "✓ Phase 5 complete: ${DURATION_MS}ms"

# Final monitoring
echo "  Final monitoring (30 seconds)..."
sleep 30

# Get final stats
echo ""
echo "=== PHASE 6: Final Statistics ==="
TELEMETRY=$(curl -s http://$MASTER_ADDR/telemetry 2>/dev/null)
echo "$TELEMETRY" | jq '.telemetry' > "$RESULTS_FILE" 2>/dev/null || echo "{}" > "$RESULTS_FILE"

TOTAL_TASKS=$(echo "$TELEMETRY" | jq '.telemetry.total_tasks // 0' 2>/dev/null || echo "0")
RUNNING=$(echo "$TELEMETRY" | jq '.telemetry.running_tasks // 0' 2>/dev/null || echo "0")
COMPLETED=$(echo "$TELEMETRY" | jq '.telemetry.completed_tasks // 0' 2>/dev/null || echo "0")

echo "Total Tasks Submitted: $TOTAL_TASKS"
echo "Currently Running: $RUNNING"
echo "Completed Tasks: $COMPLETED"

echo ""
echo "=========================================="
echo "Stress Test Complete!"
echo "=========================================="
