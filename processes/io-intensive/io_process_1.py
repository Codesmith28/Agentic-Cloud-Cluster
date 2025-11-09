#!/usr/bin/env python3
"""
IO Process 1: File Writing and Reading with Dynamic Memory Growth
Generates large files, reads them back, and gradually increases memory usage.
"""
import os
import time
import random
import psutil
import json
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 1: File Operations")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    start_time = time.time()
    
    # Dynamic parameters
    num_files = random.randint(10, 30)
    file_size_mb = random.randint(5, 20)
    memory_chunks = []
    
    stats = {
        "process_type": "IO-Intensive-1",
        "files_created": 0,
        "total_bytes_written": 0,
        "total_bytes_read": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    try:
        # Phase 1: Write files with growing memory footprint
        print(f"Phase 1: Creating {num_files} files...")
        for i in range(num_files):
            filename = f"{results_dir}/io_file_{i}.dat"
            
            # Allocate memory chunks (simulating data processing)
            chunk_size = random.randint(1024 * 1024, 5 * 1024 * 1024)  # 1-5 MB
            memory_chunks.append(bytearray(chunk_size))
            
            # Write file
            with open(filename, 'wb') as f:
                data = os.urandom(file_size_mb * 1024 * 1024)
                f.write(data)
                stats["total_bytes_written"] += len(data)
            
            stats["files_created"] += 1
            
            # Log progress
            mem_info = process.memory_info()
            current_mem_mb = mem_info.rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], current_mem_mb)
            
            print(f"  File {i+1}/{num_files}: {file_size_mb}MB written, Memory: {current_mem_mb:.2f}MB")
            time.sleep(random.uniform(0.1, 0.5))
        
        # Phase 2: Read files back
        print(f"\nPhase 2: Reading {num_files} files...")
        for i in range(num_files):
            filename = f"{results_dir}/io_file_{i}.dat"
            with open(filename, 'rb') as f:
                data = f.read()
                stats["total_bytes_read"] += len(data)
            
            print(f"  File {i+1}/{num_files}: Read {len(data)} bytes")
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 3: Random access pattern
        print(f"\nPhase 3: Random access pattern...")
        for _ in range(num_files * 2):
            idx = random.randint(0, num_files - 1)
            filename = f"{results_dir}/io_file_{idx}.dat"
            with open(filename, 'rb') as f:
                # Seek to random position
                f.seek(random.randint(0, file_size_mb * 1024 * 1024 // 2))
                f.read(random.randint(1024, 1024 * 1024))
            time.sleep(random.uniform(0.05, 0.2))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Files created: {stats['files_created']}")
        print(f"Total written: {stats['total_bytes_written'] / (1024**2):.2f} MB")
        print(f"Total read: {stats['total_bytes_read'] / (1024**2):.2f} MB")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
