#!/usr/bin/env python3
"""
CPU Process 2: Prime Number Generation and Factorization
Tests CPU with number theory operations.
"""
import os
import time
import random
import psutil
import json
import math
from datetime import datetime

def is_prime(n):
    """Check if number is prime using trial division"""
    if n < 2:
        return False
    if n == 2:
        return True
    if n % 2 == 0:
        return False
    
    for i in range(3, int(math.sqrt(n)) + 1, 2):
        if n % i == 0:
            return False
    return True

def factorize(n):
    """Factorize a number into prime factors"""
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

def sieve_of_eratosthenes(limit):
    """Generate all primes up to limit"""
    sieve = [True] * (limit + 1)
    sieve[0] = sieve[1] = False
    
    for i in range(2, int(math.sqrt(limit)) + 1):
        if sieve[i]:
            for j in range(i*i, limit + 1, i):
                sieve[j] = False
    
    return [i for i in range(limit + 1) if sieve[i]]

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 2: Prime Numbers")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-2",
        "primes_found": 0,
        "numbers_factorized": 0,
        "largest_prime": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Phase 1: Generate primes using sieve
        limit = random.randint(8000000, 9000000)  # 8M-9M for consistent high intensity
        print(f"Phase 1: Generating primes up to {limit:,}...")
        
        primes = sieve_of_eratosthenes(limit)
        stats["primes_found"] = len(primes)
        stats["largest_prime"] = primes[-1] if primes else 0
        
        mem_mb = process.memory_info().rss / (1024 * 1024)
        cpu_percent = process.cpu_percent(interval=0.1) / psutil.cpu_count()
        stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
        stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
        
        print(f"  Found {stats['primes_found']:,} primes")
        print(f"  Largest prime: {stats['largest_prime']:,}")
        print(f"  CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
        
        # Phase 2: Test primality of random large numbers
        print(f"\nPhase 2: Testing primality of large numbers...")
        
        primality_tests = random.randint(450, 500)  # Small range, more tests
        prime_count = 0
        
        for i in range(primality_tests):
            # Generate large random number
            num = random.randint(5 * 10**7, 10**8)  # 50M-100M for higher intensity
            
            if is_prime(num):
                prime_count += 1
            
            if (i + 1) % 50 == 0:
                cpu_percent = process.cpu_percent(interval=0.1) / psutil.cpu_count()
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                print(f"  Tested {i+1}/{primality_tests} numbers, "
                      f"Found {prime_count} primes, CPU: {cpu_percent:.1f}%")
        
        print(f"  Found {prime_count} primes in {primality_tests} tests")
        
        # Phase 3: Factorize composite numbers
        print(f"\nPhase 3: Factorizing composite numbers...")
        
        num_factorizations = random.randint(180, 200)  # Small range, more factorizations
        factorization_results = []
        
        for i in range(num_factorizations):
            # Generate semi-prime or composite number
            if random.random() < 0.5:
                # Semi-prime (product of two primes)
                p1 = random.choice(primes[-1000:])
                p2 = random.choice(primes[-1000:])
                num = p1 * p2
            else:
                # Random composite
                num = random.randint(5 * 10**8, 10**9)  # 500M-1B for higher intensity
            
            factors = factorize(num)
            stats["numbers_factorized"] += 1
            
            factorization_results.append({
                'number': num,
                'factors': factors,
                'num_factors': len(factors)
            })
            
            if (i + 1) % 25 == 0:
                cpu_percent = process.cpu_percent(interval=0.1) / psutil.cpu_count()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Factorized {i+1}/{num_factorizations} numbers, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
        
        # Phase 4: Find prime gaps
        print(f"\nPhase 4: Analyzing prime gaps...")
        
        gaps = []
        for i in range(len(primes) - 1):
            gap = primes[i + 1] - primes[i]
            gaps.append(gap)
        
        max_gap = max(gaps) if gaps else 0
        avg_gap = sum(gaps) / len(gaps) if gaps else 0
        
        print(f"  Maximum prime gap: {max_gap}")
        print(f"  Average prime gap: {avg_gap:.2f}")
        
        stats["duration_seconds"] = time.time() - start_time
        stats["max_prime_gap"] = max_gap
        stats["avg_prime_gap"] = avg_gap
        
        # Save factorization results (sample)
        with open(f"{results_dir}/factorizations.json", 'w') as f:
            json.dump(factorization_results[:100], f, indent=2)
        
        # Save prime list (sample)
        with open(f"{results_dir}/primes_sample.txt", 'w') as f:
            f.write(f"First 1000 primes:\n")
            for i, p in enumerate(primes[:1000], 1):
                f.write(f"{p} ")
                if i % 20 == 0:
                    f.write("\n")
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Primes found: {stats['primes_found']:,}")
        print(f"Numbers factorized: {stats['numbers_factorized']}")
        print(f"Largest prime: {stats['largest_prime']:,}")
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
