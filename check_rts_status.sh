#!/bin/bash

# Manual RTS Verification Checklist
# Based on what you see in the master terminal output

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  RTS Integration Manual Verification"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Look at your master terminal (where you ran ./runMaster.sh)"
echo "and check if you see these log messages:"
echo ""

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}Required RTS Initialization Logs:${NC}"
echo ""
echo "  ✓ Round-Robin scheduler created (fallback)"
echo "  ✓ Telemetry source adapter created"
echo "  ✓ RTS Scheduler initialized with params from config/ga_output.json"
echo "  ✓ RTS scheduler initialized (params: config/ga_output.json)"
echo "    - Scheduler: RTS"
echo "    - Fallback: Round-Robin"
echo "    - Parameter hot-reload: enabled (every 30s)"
echo "  ✓ Master server configured with RTS scheduler"
echo ""

# Check if master is running
if pgrep -f "masterNode" > /dev/null; then
    echo -e "${GREEN}✓ Master process is running${NC}"
    echo "  PID: $(pgrep -f masterNode)"
else
    echo -e "${YELLOW}⚠ Master process not detected${NC}"
fi

# Check if workers are running
WORKER_COUNT=$(pgrep -f "workerNode" | wc -l)
if [ "$WORKER_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ ${WORKER_COUNT} worker(s) running${NC}"
else
    echo -e "${YELLOW}⚠ No workers detected${NC}"
fi

# Check telemetry endpoint
if curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Telemetry endpoint is active${NC}"
    echo "  Test it: curl http://localhost:8080/telemetry"
else
    echo -e "${YELLOW}⚠ Telemetry endpoint not responding${NC}"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}Based on your terminal output:${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "If you see ALL the required logs above in Terminal 1,"
echo -e "then ${GREEN}✅ RTS IS SUCCESSFULLY INTEGRATED AND RUNNING!${NC}"
echo ""
echo "Next steps:"
echo "  1. Register workers in master CLI (Terminal 1):"
echo "     master> register Topology 10.194.23.182:50052"
echo "     master> register Topology 10.194.23.182:50053"
echo ""
echo "  2. List workers to verify:"
echo "     master> list_workers"
echo ""
echo "  3. Submit a test task to see RTS in action!"
echo ""
echo "  4. Watch for RTS decisions in Terminal 1:"
echo "     Look for: 'RTS: Selected worker <id> (score: X.XX)'"
echo ""
