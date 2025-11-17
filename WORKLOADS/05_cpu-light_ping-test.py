#!/usr/bin/env python3
"""
Workload Type: cpu-light
Description: Network connectivity test (simulated)
Resource Requirements: Minimal CPU, minimal memory
Output: None (logs only)
"""

import time
import datetime
import random

def simulate_ping(host, count=10):
    """Simulate ping test"""
    print(f"\nPinging {host}...")
    times = []
    
    for i in range(count):
        # Simulate ping time
        ping_time = random.uniform(10, 50)
        times.append(ping_time)
        print(f"  Reply from {host}: time={ping_time:.2f}ms")
        time.sleep(0.1)
    
    avg_time = sum(times) / len(times)
    min_time = min(times)
    max_time = max(times)
    
    print(f"\n  Statistics:")
    print(f"    Packets: Sent = {count}, Received = {count}, Lost = 0 (0% loss)")
    print(f"    Minimum = {min_time:.2f}ms, Maximum = {max_time:.2f}ms, Average = {avg_time:.2f}ms")

def run_ping_tests():
    print(f"[{datetime.datetime.now()}] Starting Ping Tests...")
    
    hosts = ['google.com', 'github.com', 'cloudflare.com']
    
    for host in hosts:
        simulate_ping(host, count=5)
    
    print(f"\n[{datetime.datetime.now()}] Ping Tests completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_ping_tests())
