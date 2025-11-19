#!/usr/bin/env python3
"""
IO Process 26: File Operations
Performs intensive I/O operations.
"""
import os
import time
import random
import psutil
import json
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 26")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "IO-Intensive-26",
        "files_processed": 0,
        "bytes_written": 0,
        "bytes_read": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        num_files = random.randint(20, 50)
        print(f"Processing {num_files} files...")
        
        for i in range(num_files):
            filename = f"{results_dir}/io_data_{i}.dat"
            file_size_mb = random.randint(5, 20)
            
            with open(filename, 'wb') as f:
                data = os.urandom(file_size_mb * 1024 * 1024)
                f.write(data)
                stats["bytes_written"] += len(data)
            
            stats["files_processed"] += 1
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            if (i + 1) % 10 == 0:
                print(f"  Written {i+1}/{num_files} files, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        print(f"\nReading {num_files} files...")
        for i in range(num_files):
            filename = f"{results_dir}/io_data_{i}.dat"
            
            with open(filename, 'rb') as f:
                data = f.read()
                stats["bytes_read"] += len(data)
            
            if (i + 1) % 10 == 0:
                print(f"  Read {i+1}/{num_files} files")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        stats["duration_seconds"] = time.time() - start_time
        
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Files processed: {stats['files_processed']}")
        print(f"Bytes written: {stats['bytes_written'] / (1024**2):.2f} MB")
        print(f"Bytes read: {stats['bytes_read'] / (1024**2):.2f} MB")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
