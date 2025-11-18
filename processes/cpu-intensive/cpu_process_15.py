#!/usr/bin/env python3
"""
CPU Process 15: Cryptographic Hash Computing
Intensive hash calculations for various algorithms.
"""
import os
import time
import random
import psutil
import json
import hashlib
from datetime import datetime

def compute_hash(data, algorithm):
    """Compute hash using specified algorithm"""
    h = hashlib.new(algorithm)
    h.update(data)
    return h.hexdigest()

def hash_cascade(data, iterations):
    """Apply multiple hash iterations"""
    result = data
    for i in range(iterations):
        result = hashlib.sha256(result).digest()
    return result.hex()

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 15: Cryptographic Hashing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-15",
        "hashes_computed": 0,
        "bytes_hashed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        algorithms = ['md5', 'sha1', 'sha224', 'sha256', 'sha384', 'sha512']
        
        # Phase 1: Hash random data with different algorithms
        print("Phase 1: Hashing random data...")
        
        num_iterations = random.randint(10000, 30000)
        hash_results = []
        
        for i in range(num_iterations):
            data_size = random.randint(1024, 1024 * 1024)  # 1KB to 1MB
            data = os.urandom(data_size)
            algo = random.choice(algorithms)
            
            hash_value = compute_hash(data, algo)
            hash_results.append({
                'iteration': i,
                'algorithm': algo,
                'data_size': data_size,
                'hash': hash_value
            })
            
            stats["hashes_computed"] += 1
            stats["bytes_hashed"] += data_size
            
            if (i + 1) % 2000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Computed {i+1}/{num_iterations} hashes, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.0001, 0.001))
        
        # Phase 2: Hash cascading
        print("\nPhase 2: Hash cascading...")
        
        cascade_iterations = random.randint(1000, 3000)
        cascade_depth = random.randint(100, 500)
        
        for i in range(cascade_iterations):
            data = os.urandom(1024)
            cascade_hash = hash_cascade(data, cascade_depth)
            
            stats["hashes_computed"] += cascade_depth
            
            if (i + 1) % 200 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Cascade {i+1}/{cascade_iterations}, depth={cascade_depth}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.001, 0.01))
        
        # Phase 3: Collision searching (partial)
        print("\nPhase 3: Hash prefix matching...")
        
        target_prefix = "00000"
        attempts = 0
        max_attempts = random.randint(100000, 500000)
        matches = []
        
        while attempts < max_attempts:
            data = os.urandom(32)
            hash_val = hashlib.sha256(data).hexdigest()
            
            if hash_val.startswith(target_prefix):
                matches.append({
                    'attempts': attempts,
                    'hash': hash_val,
                    'data': data.hex()
                })
                print(f"  Found match at attempt {attempts}: {hash_val}")
            
            attempts += 1
            stats["hashes_computed"] += 1
            
            if attempts % 50000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Attempt {attempts}/{max_attempts}, matches found: {len(matches)}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.00001, 0.0001))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/hash_results.json", 'w') as f:
            json.dump({
                'sample_hashes': hash_results[:100],
                'matches_found': matches
            }, f, indent=2)
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Hashes computed: {stats['hashes_computed']:,}")
        print(f"Bytes hashed: {stats['bytes_hashed'] / (1024**2):.2f} MB")
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
