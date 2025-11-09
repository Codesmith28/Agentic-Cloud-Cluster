#!/usr/bin/env python3
"""
IO Process 5: Image/Binary Data Processing
Processes large binary files with compression/decompression.
"""
import os
import time
import random
import psutil
import json
import zlib
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 5: Binary Data Processing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "IO-Intensive-5",
        "files_generated": 0,
        "files_compressed": 0,
        "original_size_mb": 0,
        "compressed_size_mb": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        num_files = random.randint(15, 30)
        print(f"Generating and processing {num_files} binary files...")
        
        # Phase 1: Generate binary files
        print("Phase 1: Generating binary files...")
        for i in range(num_files):
            file_size_mb = random.randint(5, 25)
            filename = f"{results_dir}/binary_{i}.dat"
            
            with open(filename, 'wb') as f:
                # Generate semi-random data (more realistic than pure random for compression)
                for _ in range(file_size_mb):
                    # Mix of random and repetitive data
                    if random.random() < 0.3:
                        chunk = bytes([random.randint(0, 255)] * (1024 * 1024))
                    else:
                        chunk = os.urandom(1024 * 1024)
                    f.write(chunk)
            
            stats["files_generated"] += 1
            file_stat = os.stat(filename)
            stats["original_size_mb"] += file_stat.st_size / (1024 * 1024)
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            print(f"  Generated {filename}: {file_size_mb}MB, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        # Phase 2: Compress files
        print("\nPhase 2: Compressing files...")
        for i in range(num_files):
            filename = f"{results_dir}/binary_{i}.dat"
            compressed_filename = f"{results_dir}/binary_{i}.dat.zlib"
            
            # Read and compress in chunks
            with open(filename, 'rb') as f_in, open(compressed_filename, 'wb') as f_out:
                compressor = zlib.compressobj(level=6)
                
                while True:
                    chunk = f_in.read(1024 * 1024)  # 1MB chunks
                    if not chunk:
                        break
                    compressed_chunk = compressor.compress(chunk)
                    if compressed_chunk:
                        f_out.write(compressed_chunk)
                
                # Flush remaining data
                final_chunk = compressor.flush()
                if final_chunk:
                    f_out.write(final_chunk)
            
            stats["files_compressed"] += 1
            compressed_stat = os.stat(compressed_filename)
            stats["compressed_size_mb"] += compressed_stat.st_size / (1024 * 1024)
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            print(f"  Compressed {os.path.basename(filename)}, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        # Phase 3: Decompress and verify (sample)
        print("\nPhase 3: Decompressing sample files...")
        sample_count = min(5, num_files)
        for i in random.sample(range(num_files), sample_count):
            compressed_filename = f"{results_dir}/binary_{i}.dat.zlib"
            decompressed_filename = f"{results_dir}/binary_{i}_decompressed.dat"
            
            with open(compressed_filename, 'rb') as f_in, open(decompressed_filename, 'wb') as f_out:
                decompressor = zlib.decompressobj()
                
                while True:
                    chunk = f_in.read(1024 * 1024)
                    if not chunk:
                        break
                    decompressed_chunk = decompressor.decompress(chunk)
                    if decompressed_chunk:
                        f_out.write(decompressed_chunk)
                
                final_chunk = decompressor.flush()
                if final_chunk:
                    f_out.write(final_chunk)
            
            print(f"  Decompressed {os.path.basename(compressed_filename)}")
            time.sleep(random.uniform(0.1, 0.3))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["compression_ratio"] = stats["compressed_size_mb"] / stats["original_size_mb"] if stats["original_size_mb"] > 0 else 0
        
        # Save stats
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Files generated: {stats['files_generated']}")
        print(f"Files compressed: {stats['files_compressed']}")
        print(f"Original size: {stats['original_size_mb']:.2f} MB")
        print(f"Compressed size: {stats['compressed_size_mb']:.2f} MB")
        print(f"Compression ratio: {stats['compression_ratio']:.2%}")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
