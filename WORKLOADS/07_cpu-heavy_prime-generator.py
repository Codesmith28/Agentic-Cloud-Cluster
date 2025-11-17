#!/usr/bin/env python3
"""
Workload Type: cpu-heavy
Description: Compute first N prime numbers
Resource Requirements: High CPU, moderate memory
Output: primes.csv (generated file)
"""

import datetime
import csv
import math

def is_prime(n):
    """Check if number is prime"""
    if n < 2:
        return False
    if n == 2:
        return True
    if n % 2 == 0:
        return False
    
    sqrt_n = int(math.sqrt(n))
    for i in range(3, sqrt_n + 1, 2):
        if n % i == 0:
            return False
    return True

def find_primes(target_count=100_000):
    """Find first N prime numbers"""
    print(f"[{datetime.datetime.now()}] Starting Prime Number Generator...")
    print(f"Finding first {target_count:,} prime numbers...")
    
    primes = []
    candidate = 2
    
    while len(primes) < target_count:
        if is_prime(candidate):
            primes.append(candidate)
            
            if len(primes) % 10000 == 0:
                print(f"  Found {len(primes):,} primes, latest: {candidate:,}")
        
        candidate += 1
    
    print(f"\n✓ Found {len(primes):,} prime numbers")
    print(f"✓ Largest prime: {primes[-1]:,}")
    print(f"✓ Sum of all primes: {sum(primes):,}")
    
    # Save first 1000 and last 1000 primes to CSV
    sample_primes = primes[:500] + primes[-500:]
    
    with open('/output/primes.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Index', 'Prime Number'])
        for i, prime in enumerate(sample_primes, 1):
            writer.writerow([i, prime])
    
    print(f"✓ Generated primes.csv (sample of 1000 primes)")
    print(f"[{datetime.datetime.now()}] Prime Generator completed ✓")
    return 0

if __name__ == "__main__":
    exit(find_primes())
