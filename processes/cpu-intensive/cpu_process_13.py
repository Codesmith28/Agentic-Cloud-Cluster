#!/usr/bin/env python3
"""
CPU Process 13: Prime Number Generation and Sieve Algorithms
Intensive prime number calculations and factorization.
"""
import os
import time
import random
import psutil
import json
import csv
from datetime import datetime

def sieve_of_eratosthenes(limit):
    """Generate all primes up to limit using sieve"""
    sieve = [True] * (limit + 1)
    sieve[0] = sieve[1] = False
    
    for i in range(2, int(limit**0.5) + 1):
        if sieve[i]:
            for j in range(i*i, limit + 1, i):
                sieve[j] = False
    
    return [i for i, is_prime in enumerate(sieve) if is_prime]

def is_prime(n):
    """Check if number is prime"""
    if n < 2:
        return False
    if n == 2:
        return True
    if n % 2 == 0:
        return False
    
    for i in range(3, int(n**0.5) + 1, 2):
        if n % i == 0:
            return False
    return True

def prime_factorization(n):
    """Get prime factors of n"""
    factors = []
    d = 2
    while d * d <= n:
        while n % d == 0:
            factors.append(d)
            n //= d
        d += 1
    if n > 1:
        factors.append(n)
    return factors

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 13: Prime Numbers")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-13",
        "primes_found": 0,
        "numbers_factorized": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Phase 1: Generate primes using sieve
        print("Phase 1: Generating primes using Sieve of Eratosthenes...")
        limit = random.randint(5000000, 10000000)
        primes = sieve_of_eratosthenes(limit)
        stats["primes_found"] = len(primes)
        
        cpu_percent = process.cpu_percent()
        mem_mb = process.memory_info().rss / (1024 * 1024)
        stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
        stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
        
        print(f"  Found {len(primes)} primes up to {limit}")
        print(f"  CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
        
        # Phase 2: Prime factorization
        print("\nPhase 2: Prime factorization of large numbers...")
        factorization_results = []
        
        num_factorizations = random.randint(5000, 10000)
        for i in range(num_factorizations):
            n = random.randint(100000, 10000000)
            factors = prime_factorization(n)
            
            factorization_results.append({
                'number': n,
                'factors': factors,
                'num_factors': len(factors)
            })
            
            stats["numbers_factorized"] += 1
            
            if (i + 1) % 1000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Factorized {i+1}/{num_factorizations} numbers, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.0001, 0.001))
        
        # Phase 3: Twin primes search
        print("\nPhase 3: Finding twin primes...")
        twin_primes = []
        for i in range(len(primes) - 1):
            if primes[i+1] - primes[i] == 2:
                twin_primes.append((primes[i], primes[i+1]))
        
        print(f"  Found {len(twin_primes)} twin prime pairs")
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save prime list to CSV
        with open(f"{results_dir}/primes.csv", 'w', newline='') as f:
            writer = csv.writer(f)
            writer.writerow(['Index', 'Prime'])
            for idx, prime in enumerate(primes[:10000]):  # Save first 10000
                writer.writerow([idx, prime])
        
        # Save factorization results
        with open(f"{results_dir}/factorizations.json", 'w') as f:
            json.dump(factorization_results[:1000], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Primes found: {stats['primes_found']:,}")
        print(f"Numbers factorized: {stats['numbers_factorized']:,}")
        print(f"Peak CPU: {stats['peak_cpu_percent']:.1f}%")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
