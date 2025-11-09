#!/usr/bin/env python3
"""
CPU Process 4: Cryptographic Hashing and String Processing
Tests CPU with hash computations and string manipulation.
"""
import os
import time
import random
import psutil
import json
import hashlib
import hmac
from datetime import datetime

def generate_random_string(length):
    """Generate random string"""
    chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
    return ''.join(random.choice(chars) for _ in range(length))

def compute_hashes(data):
    """Compute multiple hash algorithms"""
    hashes = {}
    
    # MD5
    hashes['md5'] = hashlib.md5(data.encode()).hexdigest()
    
    # SHA family
    hashes['sha1'] = hashlib.sha1(data.encode()).hexdigest()
    hashes['sha256'] = hashlib.sha256(data.encode()).hexdigest()
    hashes['sha512'] = hashlib.sha512(data.encode()).hexdigest()
    
    # SHA3 family
    hashes['sha3_256'] = hashlib.sha3_256(data.encode()).hexdigest()
    hashes['sha3_512'] = hashlib.sha3_512(data.encode()).hexdigest()
    
    # BLAKE2
    hashes['blake2b'] = hashlib.blake2b(data.encode()).hexdigest()
    hashes['blake2s'] = hashlib.blake2s(data.encode()).hexdigest()
    
    return hashes

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 4: Cryptographic Hashing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-4",
        "strings_processed": 0,
        "hashes_computed": 0,
        "hmacs_computed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    hash_results = []
    
    try:
        # Phase 1: Hash random strings of varying lengths
        num_strings = random.randint(10000, 50000)
        print(f"Phase 1: Hashing {num_strings:,} random strings...")
        
        for i in range(num_strings):
            # Varying string lengths
            if i % 3 == 0:
                str_len = random.randint(10, 100)
            elif i % 3 == 1:
                str_len = random.randint(100, 1000)
            else:
                str_len = random.randint(1000, 10000)
            
            data = generate_random_string(str_len)
            hashes = compute_hashes(data)
            
            stats["strings_processed"] += 1
            stats["hashes_computed"] += len(hashes)
            
            # Store sample results
            if i < 100:
                hash_results.append({
                    'string_length': str_len,
                    'hashes': hashes
                })
            
            if (i + 1) % 5000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Processed {i+1:,}/{num_strings:,} strings, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            if i % 100 == 0:
                time.sleep(random.uniform(0.001, 0.01))
        
        # Phase 2: HMAC computations
        print(f"\nPhase 2: Computing HMACs...")
        
        num_hmacs = random.randint(5000, 20000)
        secret_key = generate_random_string(32).encode()
        
        for i in range(num_hmacs):
            message = generate_random_string(random.randint(50, 500))
            
            # Compute HMAC with different algorithms
            hmac_sha256 = hmac.new(secret_key, message.encode(), hashlib.sha256).hexdigest()
            hmac_sha512 = hmac.new(secret_key, message.encode(), hashlib.sha512).hexdigest()
            
            stats["hmacs_computed"] += 2
            
            if (i + 1) % 2000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Computed {i+1:,}/{num_hmacs:,} HMACs, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            if i % 100 == 0:
                time.sleep(random.uniform(0.001, 0.01))
        
        # Phase 3: Hash chain (blockchain-like)
        print(f"\nPhase 3: Computing hash chain...")
        
        chain_length = random.randint(10000, 30000)
        previous_hash = hashlib.sha256(b"genesis").hexdigest()
        
        for i in range(chain_length):
            # Create new hash based on previous
            data = f"{previous_hash}{i}{generate_random_string(50)}"
            previous_hash = hashlib.sha256(data.encode()).hexdigest()
            
            if (i + 1) % 2000 == 0:
                cpu_percent = process.cpu_percent()
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                print(f"  Chain link {i+1:,}/{chain_length:,}, CPU: {cpu_percent:.1f}%")
            
            if i % 100 == 0:
                time.sleep(random.uniform(0.001, 0.005))
        
        print(f"  Final hash: {previous_hash}")
        
        # Phase 4: Password hashing simulation
        print(f"\nPhase 4: Password hashing...")
        
        num_passwords = random.randint(1000, 5000)
        salt = os.urandom(32)
        
        for i in range(num_passwords):
            password = generate_random_string(random.randint(8, 20))
            
            # Simulate expensive password hashing (multiple rounds)
            hashed = password.encode()
            rounds = random.randint(10000, 50000)
            
            for _ in range(rounds):
                hashed = hashlib.sha256(hashed + salt).digest()
            
            if (i + 1) % 200 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Hashed {i+1}/{num_passwords} passwords, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.005, 0.02))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["hashes_per_second"] = stats["hashes_computed"] / stats["duration_seconds"]
        
        # Save hash results
        with open(f"{results_dir}/hash_results.json", 'w') as f:
            json.dump(hash_results, f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Strings processed: {stats['strings_processed']:,}")
        print(f"Total hashes computed: {stats['hashes_computed']:,}")
        print(f"HMACs computed: {stats['hmacs_computed']:,}")
        print(f"Hashes/second: {stats['hashes_per_second']:.2f}")
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
