#!/usr/bin/env python3
"""
IO Process 6: Sequential and Random File Access Mix
Tests various I/O patterns with memory-mapped files.
"""
import os
import time
import random
import psutil
import json
import mmap
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 6: Mixed Access Patterns")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "IO-Intensive-6",
        "sequential_operations": 0,
        "random_operations": 0,
        "mmap_operations": 0,
        "total_bytes_accessed": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Create base files
        num_files = random.randint(8, 15)
        file_size_mb = random.randint(10, 30)
        
        print(f"Creating {num_files} files of {file_size_mb}MB each...")
        files = []
        
        for i in range(num_files):
            filename = f"{results_dir}/data_{i}.bin"
            files.append(filename)
            
            with open(filename, 'wb') as f:
                # Create file with varying patterns
                for mb in range(file_size_mb):
                    pattern_type = random.choice(['zeros', 'ones', 'random', 'sequential'])
                    
                    if pattern_type == 'zeros':
                        chunk = bytes(1024 * 1024)
                    elif pattern_type == 'ones':
                        chunk = bytes([255] * (1024 * 1024))
                    elif pattern_type == 'random':
                        chunk = os.urandom(1024 * 1024)
                    else:
                        chunk = bytes([(mb + j) % 256 for j in range(1024 * 1024)])
                    
                    f.write(chunk)
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            print(f"  Created {filename}, Memory: {mem_mb:.2f}MB")
        
        # Sequential read test
        print(f"\nPhase 1: Sequential reads...")
        for filename in files:
            with open(filename, 'rb') as f:
                while True:
                    chunk = f.read(1024 * 1024)
                    if not chunk:
                        break
                    stats["sequential_operations"] += 1
                    stats["total_bytes_accessed"] += len(chunk)
            
            print(f"  Sequential read: {os.path.basename(filename)}")
            time.sleep(random.uniform(0.05, 0.15))
        
        # Random access test
        print(f"\nPhase 2: Random access...")
        for _ in range(random.randint(100, 300)):
            filename = random.choice(files)
            file_size = os.path.getsize(filename)
            
            with open(filename, 'rb') as f:
                # Random seek and read
                position = random.randint(0, max(0, file_size - 1024 * 1024))
                f.seek(position)
                chunk = f.read(random.randint(4096, 1024 * 1024))
                stats["random_operations"] += 1
                stats["total_bytes_accessed"] += len(chunk)
            
            if stats["random_operations"] % 50 == 0:
                mem_mb = process.memory_info().rss / (1024 * 1024)
                print(f"  Random operations: {stats['random_operations']}, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Memory-mapped file test
        print(f"\nPhase 3: Memory-mapped access...")
        sample_files = random.sample(files, min(3, len(files)))
        
        for filename in sample_files:
            with open(filename, 'r+b') as f:
                with mmap.mmap(f.fileno(), 0) as mmapped:
                    file_size = len(mmapped)
                    
                    # Random access through mmap
                    for _ in range(random.randint(50, 150)):
                        position = random.randint(0, max(0, file_size - 1024))
                        size = random.randint(512, 4096)
                        data = mmapped[position:position + size]
                        stats["mmap_operations"] += 1
                        stats["total_bytes_accessed"] += len(data)
                        
                        # Occasionally write
                        if random.random() < 0.1:
                            write_pos = random.randint(0, max(0, file_size - 1024))
                            mmapped[write_pos:write_pos + 512] = os.urandom(512)
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            print(f"  Memory-mapped: {os.path.basename(filename)}, Memory: {mem_mb:.2f}MB")
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save stats
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Sequential operations: {stats['sequential_operations']}")
        print(f"Random operations: {stats['random_operations']}")
        print(f"Memory-mapped operations: {stats['mmap_operations']}")
        print(f"Total bytes accessed: {stats['total_bytes_accessed'] / (1024**3):.2f} GB")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
