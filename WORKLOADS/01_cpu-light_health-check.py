#!/usr/bin/env python3
"""
Workload Type: cpu-light
Description: Simple health check script that validates system status
Resource Requirements: Minimal CPU, minimal memory
Output: None (logs only)
"""

import time
import sys
import socket
import datetime

def check_system_health():
    print(f"[{datetime.datetime.now()}] Starting Health Check...")
    
    # Check hostname
    hostname = socket.gethostname()
    print(f"✓ Hostname: {hostname}")
    
    # Check Python version
    print(f"✓ Python version: {sys.version}")
    
    # Simple CPU activity
    result = sum(i**2 for i in range(1000))
    print(f"✓ CPU test completed: {result}")
    
    # Memory check
    data = [i for i in range(10000)]
    print(f"✓ Memory test completed: {len(data)} items")
    
    time.sleep(2)
    
    print(f"[{datetime.datetime.now()}] Health Check PASSED ✓")
    return 0

if __name__ == "__main__":
    exit(check_system_health())
