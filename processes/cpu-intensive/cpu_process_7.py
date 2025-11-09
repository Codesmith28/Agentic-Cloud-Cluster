#!/usr/bin/env python3
"""
CPU Process 7: Compression Algorithms and Data Encoding
Tests CPU with various compression and encoding schemes.
"""
import os
import time
import random
import psutil
import json
import zlib
import bz2
import lzma
import base64
from datetime import datetime

def generate_compressible_data(size_mb, pattern='mixed'):
    """Generate data with varying compressibility"""
    size_bytes = size_mb * 1024 * 1024
    
    if pattern == 'highly_compressible':
        # Lots of repetition
        return bytes([random.randint(0, 10)] * size_bytes)
    elif pattern == 'moderately_compressible':
        # Some patterns
        data = bytearray()
        while len(data) < size_bytes:
            chunk = bytes([random.randint(0, 255)] * random.randint(10, 100))
            data.extend(chunk)
        return bytes(data[:size_bytes])
    elif pattern == 'random':
        # Hard to compress
        return os.urandom(size_bytes)
    else:  # mixed
        # Mix of all types
        data = bytearray()
        while len(data) < size_bytes:
            pattern_type = random.choice(['repetitive', 'semi_random', 'random'])
            chunk_size = random.randint(1000, 10000)
            
            if pattern_type == 'repetitive':
                chunk = bytes([random.randint(0, 50)] * chunk_size)
            elif pattern_type == 'semi_random':
                chunk = bytes([random.randint(0, 255) for _ in range(chunk_size)])
            else:
                chunk = os.urandom(chunk_size)
            
            data.extend(chunk)
        
        return bytes(data[:size_bytes])

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 7: Compression Algorithms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-7",
        "compressions_completed": 0,
        "decompressions_completed": 0,
        "total_original_mb": 0,
        "total_compressed_mb": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    compression_results = []
    
    try:
        # Test different compression algorithms
        algorithms = {
            'zlib_level_1': (lambda d: zlib.compress(d, level=1), zlib.decompress),
            'zlib_level_6': (lambda d: zlib.compress(d, level=6), zlib.decompress),
            'zlib_level_9': (lambda d: zlib.compress(d, level=9), zlib.decompress),
            'bz2': (bz2.compress, bz2.decompress),
            'lzma': (lzma.compress, lzma.decompress),
        }
        
        data_patterns = ['highly_compressible', 'moderately_compressible', 'random', 'mixed']
        
        # Phase 1: Compression tests
        print(f"Phase 1: Testing compression algorithms...")
        
        for pattern in data_patterns:
            print(f"\n  Testing with {pattern} data...")
            
            num_tests = random.randint(3, 8)
            
            for test_num in range(num_tests):
                data_size_mb = random.randint(1, 10)
                data = generate_compressible_data(data_size_mb, pattern)
                
                stats["total_original_mb"] += data_size_mb
                
                for algo_name, (compress_func, decompress_func) in algorithms.items():
                    # Compress
                    compress_start = time.time()
                    compressed = compress_func(data)
                    compress_time = time.time() - compress_start
                    
                    compressed_size_mb = len(compressed) / (1024 * 1024)
                    stats["total_compressed_mb"] += compressed_size_mb
                    stats["compressions_completed"] += 1
                    
                    # Decompress
                    decompress_start = time.time()
                    decompressed = decompress_func(compressed)
                    decompress_time = time.time() - decompress_start
                    
                    stats["decompressions_completed"] += 1
                    
                    # Verify
                    is_correct = (data == decompressed)
                    
                    ratio = len(compressed) / len(data) if len(data) > 0 else 0
                    
                    compression_results.append({
                        'algorithm': algo_name,
                        'pattern': pattern,
                        'original_mb': data_size_mb,
                        'compressed_mb': compressed_size_mb,
                        'ratio': ratio,
                        'compress_time': compress_time,
                        'decompress_time': decompress_time,
                        'verified': is_correct
                    })
                    
                    # Track resources
                    cpu_percent = process.cpu_percent()
                    mem_mb = process.memory_info().rss / (1024 * 1024)
                    stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                    stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                if test_num % 2 == 0:
                    cpu_percent = process.cpu_percent()
                    mem_mb = process.memory_info().rss / (1024 * 1024)
                    print(f"    Test {test_num+1}/{num_tests} completed, "
                          f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
                
                time.sleep(random.uniform(0.05, 0.15))
        
        # Phase 2: Encoding tests
        print(f"\nPhase 2: Testing encoding schemes...")
        
        encoding_tests = random.randint(50, 150)
        
        for i in range(encoding_tests):
            data_size = random.randint(1024, 1024 * 100)
            data = os.urandom(data_size)
            
            # Base64 encoding
            encoded = base64.b64encode(data)
            decoded = base64.b64decode(encoded)
            
            # Verify
            is_correct = (data == decoded)
            
            # URL-safe encoding
            url_encoded = base64.urlsafe_b64encode(data)
            url_decoded = base64.urlsafe_b64decode(url_encoded)
            
            # Hex encoding
            hex_encoded = data.hex()
            hex_decoded = bytes.fromhex(hex_encoded)
            
            if (i + 1) % 30 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Encoded {i+1}/{encoding_tests} samples, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            if i % 10 == 0:
                time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 3: Streaming compression
        print(f"\nPhase 3: Streaming compression...")
        
        stream_tests = random.randint(5, 15)
        
        for i in range(stream_tests):
            # Simulate streaming data
            data_chunks = []
            total_size = 0
            
            num_chunks = random.randint(10, 50)
            compressor = zlib.compressobj(level=6)
            compressed_chunks = []
            
            for _ in range(num_chunks):
                chunk_size = random.randint(1024, 10240)
                chunk = os.urandom(chunk_size)
                data_chunks.append(chunk)
                total_size += chunk_size
                
                # Compress chunk
                compressed_chunk = compressor.compress(chunk)
                if compressed_chunk:
                    compressed_chunks.append(compressed_chunk)
            
            # Flush compressor
            final_chunk = compressor.flush()
            if final_chunk:
                compressed_chunks.append(final_chunk)
            
            compressed_data = b''.join(compressed_chunks)
            original_data = b''.join(data_chunks)
            
            # Decompress
            decompressed = zlib.decompress(compressed_data)
            is_correct = (original_data == decompressed)
            
            ratio = len(compressed_data) / len(original_data) if len(original_data) > 0 else 0
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  Stream {i+1}/{stream_tests}: {num_chunks} chunks, "
                  f"Ratio: {ratio:.2%}, Verified: {is_correct}, "
                  f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["avg_compression_ratio"] = stats["total_compressed_mb"] / stats["total_original_mb"] if stats["total_original_mb"] > 0 else 0
        
        # Save compression results
        with open(f"{results_dir}/compression_results.json", 'w') as f:
            json.dump(compression_results, f, indent=2)
        
        # Generate summary report
        with open(f"{results_dir}/compression_summary.txt", 'w') as f:
            f.write("Compression Algorithm Performance Summary\n")
            f.write("="*60 + "\n\n")
            
            for algo_name in algorithms.keys():
                algo_results = [r for r in compression_results if r['algorithm'] == algo_name]
                if algo_results:
                    avg_ratio = sum(r['ratio'] for r in algo_results) / len(algo_results)
                    avg_compress_time = sum(r['compress_time'] for r in algo_results) / len(algo_results)
                    avg_decompress_time = sum(r['decompress_time'] for r in algo_results) / len(algo_results)
                    
                    f.write(f"{algo_name}:\n")
                    f.write(f"  Average Compression Ratio: {avg_ratio:.2%}\n")
                    f.write(f"  Average Compression Time: {avg_compress_time:.3f}s\n")
                    f.write(f"  Average Decompression Time: {avg_decompress_time:.3f}s\n\n")
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Compressions completed: {stats['compressions_completed']}")
        print(f"Decompressions completed: {stats['decompressions_completed']}")
        print(f"Total original: {stats['total_original_mb']:.2f} MB")
        print(f"Total compressed: {stats['total_compressed_mb']:.2f} MB")
        print(f"Average ratio: {stats['avg_compression_ratio']:.2%}")
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
