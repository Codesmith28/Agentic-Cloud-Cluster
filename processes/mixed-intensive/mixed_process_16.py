#!/usr/bin/env python3
"""
Mixed Process 16: Combined Operations
Performs both CPU and I/O intensive operations.
"""
import os
import time
import random
import psutil
import json
import csv
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting Mixed-Intensive Process 16")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "Mixed-Intensive-16",
        "cpu_operations": 0,
        "io_operations": 0,
        "bytes_written": 0,
        "bytes_read": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        print("Phase 1: Loading/generating data...")
        num_files = random.randint(10, 30)
        data_arrays = []
        
        for i in range(num_files):
            filename = f"{results_dir}/mixed_input_{i}.dat"
            data_size = random.randint(1, 5) * 1024 * 1024
            data = os.urandom(data_size)
            
            with open(filename, 'wb') as f:
                f.write(data)
                stats["bytes_written"] += len(data)
            
            stats["io_operations"] += 1
            
            with open(filename, 'rb') as f:
                content = f.read()
                stats["bytes_read"] += len(content)
                arr = np.frombuffer(content, dtype=np.uint8)
                data_arrays.append(arr)
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            if (i + 1) % 5 == 0:
                print(f"  Loaded {i+1}/{num_files} files, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        print("\nPhase 2: Processing data...")
        results = []
        
        for i, arr in enumerate(data_arrays):
            size = int(np.sqrt(len(arr) // 4)) * 2
            if size > 0:
                matrix = arr[:size*size].reshape(size, size).astype(np.float64)
                
                mean = np.mean(matrix)
                std = np.std(matrix)
                
                results.append({
                    'file_index': i,
                    'matrix_size': size,
                    'mean': float(mean),
                    'std': float(std)
                })
                
                stats["cpu_operations"] += 1
                
                cpu_percent = process.cpu_percent()
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                
                if (i + 1) % 5 == 0:
                    print(f"  Processed {i+1}/{len(data_arrays)} arrays, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        print("\nPhase 3: Writing results...")
        
        with open(f"{results_dir}/processing_results.json", 'w') as f:
            json.dump(results, f, indent=2)
            stats["bytes_written"] += len(json.dumps(results))
        
        stats["io_operations"] += 1
        
        with open(f"{results_dir}/summary.csv", 'w', newline='') as f:
            if results:
                writer = csv.DictWriter(f, fieldnames=results[0].keys())
                writer.writeheader()
                writer.writerows(results)
        
        stats["io_operations"] += 1
        
        if results:
            fig, axes = plt.subplots(1, 2, figsize=(12, 5))
            
            axes[0].plot([r['file_index'] for r in results], [r['mean'] for r in results], 'bo-')
            axes[0].set_title('Mean Values')
            axes[0].set_xlabel('File Index')
            axes[0].set_ylabel('Mean')
            axes[0].grid(True)
            
            axes[1].plot([r['file_index'] for r in results], [r['std'] for r in results], 'ro-')
            axes[1].set_title('Standard Deviation')
            axes[1].set_xlabel('File Index')
            axes[1].set_ylabel('Std Dev')
            axes[1].grid(True)
            
            plt.tight_layout()
            plt.savefig(f"{results_dir}/analysis_results.png", dpi=150)
            plt.close()
            
            stats["io_operations"] += 1
        
        stats["duration_seconds"] = time.time() - start_time
        
        with open(f"{results_dir}/mixed_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print("\n" + "="*60)
        print("Process completed successfully!")
        print(f"CPU operations: {stats['cpu_operations']}")
        print(f"IO operations: {stats['io_operations']}")
        print(f"Bytes written: {stats['bytes_written'] / (1024**2):.2f} MB")
        print(f"Bytes read: {stats['bytes_read'] / (1024**2):.2f} MB")
        print(f"Peak CPU: {stats['peak_cpu_percent']:.1f}%")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print("="*60)
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
