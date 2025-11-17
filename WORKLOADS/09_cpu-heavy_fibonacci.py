#!/usr/bin/env python3
"""
Workload Type: cpu-heavy
Description: Fibonacci sequence computation using recursion and memoization
Resource Requirements: High CPU, moderate memory
Output: None (logs only)
"""

import datetime
from functools import lru_cache

@lru_cache(maxsize=None)
def fibonacci_memo(n):
    """Compute Fibonacci with memoization"""
    if n <= 1:
        return n
    return fibonacci_memo(n-1) + fibonacci_memo(n-2)

def fibonacci_iterative(n):
    """Compute Fibonacci iteratively"""
    if n <= 1:
        return n
    
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b

def run_fibonacci_tests():
    print(f"[{datetime.datetime.now()}] Starting Fibonacci Computation...")
    
    test_values = [100, 500, 1000, 2000, 5000]
    
    print("\nComputing Fibonacci numbers (memoized):")
    for n in test_values:
        start = datetime.datetime.now()
        result = fibonacci_memo(n)
        end = datetime.datetime.now()
        duration = (end - start).total_seconds()
        
        # Show first and last 20 digits for large numbers
        result_str = str(result)
        if len(result_str) > 40:
            display = f"{result_str[:20]}...{result_str[-20:]}"
        else:
            display = result_str
        
        print(f"  F({n}) = {display}")
        print(f"    Digits: {len(result_str)}, Time: {duration:.4f}s")
    
    # Compute sum of first 100 Fibonacci numbers
    print("\nComputing sum of first 100 Fibonacci numbers...")
    total = sum(fibonacci_iterative(i) for i in range(100))
    print(f"  ✓ Sum: {total:,}")
    
    print(f"\n[{datetime.datetime.now()}] Fibonacci Computation completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_fibonacci_tests())
