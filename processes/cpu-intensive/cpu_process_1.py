#!/usr/bin/env python3
"""
CPU Process 1: Matrix Operations and Linear Algebra
Performs intensive matrix multiplications with growing matrices.
"""
import os
import time
import random
import psutil

import json
import warnings
import numpy as np
from datetime import datetime

# Suppress complex casting warnings
warnings.filterwarnings('ignore', category=np.ComplexWarning)

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 1: Matrix Operations")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-1",
        "matrices_multiplied": 0,
        "total_flops_estimate": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Dynamic sizing
        num_operations = random.randint(50, 150)
        base_size = random.randint(200, 500)
        
        print(f"Performing {num_operations} matrix operations starting from {base_size}x{base_size}...")
        
        results = []
        
        for i in range(num_operations):
            # Gradually increase matrix size
            size = base_size + (i * random.randint(10, 50))
            
            # Generate random matrices
            A = np.random.rand(size, size)
            B = np.random.rand(size, size)
            
            # Matrix multiplication
            op_start = time.time()
            C = np.matmul(A, B)
            op_duration = time.time() - op_start
            
            # Estimate FLOPS (2*n^3 for matrix multiplication)
            flops = 2 * (size ** 3)
            stats["total_flops_estimate"] += flops
            stats["matrices_multiplied"] += 1
            
            # Perform additional operations
            eigenvalues = np.linalg.eigvals(C[:min(100, size), :min(100, size)])
            determinant = np.linalg.det(C[:min(50, size), :min(50, size)])
            
            # Track metrics
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            results.append({
                'operation': i + 1,
                'matrix_size': size,
                'duration': op_duration,
                'flops': flops,
                'eigenvalue_sum': float(np.sum(np.abs(eigenvalues))),
                'determinant': float(determinant)
            })
            
            if (i + 1) % 10 == 0:
                print(f"  Completed {i+1}/{num_operations} operations, "
                      f"Size: {size}x{size}, CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            # Brief pause to allow system monitoring
            time.sleep(random.uniform(0.01, 0.05))
        
        # Additional complex operations
        print("\nPerforming final intensive operations...")
        
        # Singular Value Decomposition
        final_size = random.randint(400, 800)
        M = np.random.rand(final_size, final_size)
        U, s, Vt = np.linalg.svd(M[:300, :300])
        print(f"  SVD completed on {300}x{300} matrix")
        
        # QR Decomposition
        Q, R = np.linalg.qr(M[:400, :400])
        print(f"  QR decomposition completed on {400}x{400} matrix")
        
        stats["duration_seconds"] = time.time() - start_time
        stats["gflops"] = stats["total_flops_estimate"] / (stats["duration_seconds"] * 1e9)
        
        # Save detailed results
        with open(f"{results_dir}/matrix_operations.json", 'w') as f:
            json.dump(results, f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Matrices multiplied: {stats['matrices_multiplied']}")
        print(f"Estimated GFLOPS: {stats['gflops']:.2f}")
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
