#!/usr/bin/env python3
"""
CPU Process 16: String Pattern Matching and Search
Intensive string operations and pattern matching algorithms.
"""
import os
import time
import random
import psutil
import json
import re
from datetime import datetime

def generate_random_text(length):
    """Generate random text"""
    chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
    return ''.join(random.choice(chars) for _ in range(length))

def naive_search(text, pattern):
    """Naive pattern search"""
    matches = []
    for i in range(len(text) - len(pattern) + 1):
        if text[i:i+len(pattern)] == pattern:
            matches.append(i)
    return matches

def kmp_search(text, pattern):
    """KMP pattern matching algorithm"""
    def compute_lps(pattern):
        lps = [0] * len(pattern)
        length = 0
        i = 1
        
        while i < len(pattern):
            if pattern[i] == pattern[length]:
                length += 1
                lps[i] = length
                i += 1
            else:
                if length != 0:
                    length = lps[length - 1]
                else:
                    lps[i] = 0
                    i += 1
        return lps
    
    matches = []
    lps = compute_lps(pattern)
    i = j = 0
    
    while i < len(text):
        if pattern[j] == text[i]:
            i += 1
            j += 1
        
        if j == len(pattern):
            matches.append(i - j)
            j = lps[j - 1]
        elif i < len(text) and pattern[j] != text[i]:
            if j != 0:
                j = lps[j - 1]
            else:
                i += 1
    
    return matches

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 16: String Pattern Matching")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-16",
        "searches_performed": 0,
        "total_text_length": 0,
        "total_matches": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    results = []
    
    try:
        # Phase 1: Generate large text corpus
        print("Phase 1: Generating text corpus...")
        
        num_documents = random.randint(50, 100)
        documents = []
        
        for i in range(num_documents):
            doc_length = random.randint(100000, 500000)
            doc = generate_random_text(doc_length)
            documents.append(doc)
            stats["total_text_length"] += doc_length
            
            if (i + 1) % 10 == 0:
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Generated {i+1}/{num_documents} documents, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 2: Naive pattern search
        print("\nPhase 2: Naive pattern search...")
        
        patterns = [generate_random_text(random.randint(5, 15)) for _ in range(50)]
        
        for pattern in patterns:
            total_matches = 0
            for doc in documents[:20]:  # Search in subset
                matches = naive_search(doc, pattern)
                total_matches += len(matches)
            
            results.append({
                'method': 'naive',
                'pattern': pattern,
                'matches': total_matches
            })
            
            stats["searches_performed"] += 1
            stats["total_matches"] += total_matches
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            time.sleep(random.uniform(0.01, 0.05))
        
        print(f"  Completed naive search, CPU: {cpu_percent:.1f}%")
        
        # Phase 3: KMP pattern search
        print("\nPhase 3: KMP pattern search...")
        
        for pattern in patterns:
            total_matches = 0
            for doc in documents[:20]:
                matches = kmp_search(doc, pattern)
                total_matches += len(matches)
            
            results.append({
                'method': 'kmp',
                'pattern': pattern,
                'matches': total_matches
            })
            
            stats["searches_performed"] += 1
            
            cpu_percent = process.cpu_percent()
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            
            time.sleep(random.uniform(0.01, 0.05))
        
        print(f"  Completed KMP search, CPU: {cpu_percent:.1f}%")
        
        # Phase 4: Regex search
        print("\nPhase 4: Regular expression search...")
        
        regex_patterns = [
            r'\b\w{5}\b',  # 5-letter words
            r'\d+',  # Numbers
            r'[A-Z][a-z]+',  # Capitalized words
            r'\b\w+@\w+\.\w+\b',  # Email-like patterns
        ]
        
        for pattern in regex_patterns:
            total_matches = 0
            for doc in documents[:30]:
                matches = re.findall(pattern, doc)
                total_matches += len(matches)
            
            results.append({
                'method': 'regex',
                'pattern': pattern,
                'matches': total_matches
            })
            
            stats["searches_performed"] += 1
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  Pattern '{pattern}': {total_matches} matches, CPU: {cpu_percent:.1f}%")
            time.sleep(random.uniform(0.1, 0.3))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/search_results.json", 'w') as f:
            json.dump(results, f, indent=2)
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Searches performed: {stats['searches_performed']}")
        print(f"Total text processed: {stats['total_text_length'] / (1024**2):.2f} MB")
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
