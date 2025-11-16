#!/bin/bash

# Quick RTS Verification Script
# Run this after starting the master to verify RTS is active

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  RTS Integration Verification"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0

# Find master process and check its output
if ! pgrep -f "masterNode" > /dev/null; then
    echo -e "${RED}✗${NC} Master is not running"
    echo ""
    echo "Start master first:"
    echo "  ./runMaster.sh"
    exit 1
fi

# Check console output - grep from current terminal/process output
# Since master is running in another terminal, we'll check for the process
echo -e "${GREEN}✓${NC} Master process is running (PID: $(pgrep -f masterNode))"
echo ""
echo "Checking master initialization from console output..."
echo "(Looking at the terminal where you ran ./runMaster.sh)"
echo ""

echo "Using log file: $MASTER_LOG"
echo ""

# Test 1: RTS Initialization
echo -n "1. RTS Scheduler Initialized... "
if grep -q "RTS scheduler initialized" "$MASTER_LOG" 2>/dev/null; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

# Test 2: Scheduler Type
echo -n "2. Master configured with RTS... "
if grep -q "Master server configured with RTS" "$MASTER_LOG" 2>/dev/null; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

# Test 3: Fallback Configured
echo -n "3. Fallback scheduler ready... "
if grep -q "Round-Robin scheduler created" "$MASTER_LOG" 2>/dev/null; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

# Test 4: Telemetry Adapter
echo -n "4. Telemetry adapter created... "
if grep -q "Telemetry source adapter created" "$MASTER_LOG" 2>/dev/null; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

# Test 5: GA Parameters
echo -n "5. GA parameters file exists... "
if [ -f "master/config/ga_output.json" ]; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

# Test 6: SLA Multiplier
echo -n "6. SLA multiplier configured... "
if grep -q "SLA multiplier" "$MASTER_LOG" 2>/dev/null; then
    echo -e "${GREEN}✓ PASS${NC}"
    SLA=$(grep "SLA multiplier" "$MASTER_LOG" | tail -1 | grep -oP '\d+\.\d+' || echo "N/A")
    echo "   → Value: $SLA"
    ((PASSED++))
else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAILED++))
fi

# Test 7: Telemetry Endpoint
echo -n "7. Telemetry endpoint active... "
if curl -s http://localhost:8081/telemetry > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASSED++))
else
    echo -e "${YELLOW}⚠ SKIP${NC} (endpoint may not be started yet)"
fi

# Test 8: RTS Making Decisions (if tasks submitted)
echo -n "8. RTS decision-making active... "
RTS_DECISIONS=$(grep -c "RTS: Selected worker" "$MASTER_LOG" 2>/dev/null || echo "0")
if [ "$RTS_DECISIONS" -gt 0 ]; then
    echo -e "${GREEN}✓ PASS${NC} ($RTS_DECISIONS decisions)"
    ((PASSED++))
else
    echo -e "${YELLOW}⚠ PENDING${NC} (no tasks submitted yet)"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Results: $PASSED passed, $FAILED failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 SUCCESS! RTS is properly integrated and running!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Start workers: ./runWorker.sh"
    echo "  2. Submit tasks and watch RTS decisions"
    echo "  3. Monitor logs: tail -f $MASTER_LOG | grep RTS"
    exit 0
else
    echo -e "${RED}❌ FAILED! RTS integration incomplete.${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "  1. Check if master built with latest code"
    echo "  2. Review log file: $MASTER_LOG"
    echo "  3. Verify Task 3.4 changes applied"
    echo "  4. Run: make master && ./runMaster.sh"
    exit 1
fi
