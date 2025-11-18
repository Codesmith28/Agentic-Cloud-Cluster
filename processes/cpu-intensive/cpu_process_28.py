#!/usr/bin/env python3
"""
CPU Process 28: Intensive Computation
Performs intensive CPU operations.
"""
import os
import time
import random
import psutil
import json
import numpy as np
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 28")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-28",
        "operations_completed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        num_iterations = random.randint(10000, 50000)
        print(f"Performing {num_iterations} intensive computations...")
        
        for i in range(num_iterations):
            size = random.randint(100, 500)
            matrix = np.random.rand(size, size)
            result = np.linalg.det(matrix) + np.trace(matrix)
            
            stats["operations_completed"] += 1
            
            if (i + 1) % 5000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Progress: {i+1}/{num_iterations}, CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.0001, 0.001))
        
        stats["duration_seconds"] = time.time() - start_time
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Operations completed: {stats['operations_completed']:,}")
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
