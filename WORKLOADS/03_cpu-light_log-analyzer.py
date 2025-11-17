#!/usr/bin/env python3
"""
Workload Type: cpu-light
Description: Analyze simple log patterns and count occurrences
Resource Requirements: Minimal CPU, minimal memory
Output: None (logs only)
"""

import datetime
import random

def generate_fake_logs(count=500):
    """Generate fake log entries"""
    levels = ['INFO', 'WARN', 'ERROR', 'DEBUG']
    messages = [
        'User logged in',
        'Request processed',
        'Database query executed',
        'Cache hit',
        'Cache miss',
        'API call completed',
    ]
    
    logs = []
    for i in range(count):
        level = random.choice(levels)
        message = random.choice(messages)
        logs.append(f"{level}: {message}")
    
    return logs

def analyze_logs():
    print(f"[{datetime.datetime.now()}] Starting Log Analyzer...")
    
    logs = generate_fake_logs()
    print(f"✓ Generated {len(logs)} log entries")
    
    # Count by level
    counts = {}
    for log in logs:
        level = log.split(':')[0]
        counts[level] = counts.get(level, 0) + 1
    
    print("\nLog Analysis Results:")
    for level, count in sorted(counts.items()):
        print(f"  {level}: {count} entries")
    
    print(f"\n[{datetime.datetime.now()}] Log Analyzer completed ✓")
    return 0

if __name__ == "__main__":
    exit(analyze_logs())
