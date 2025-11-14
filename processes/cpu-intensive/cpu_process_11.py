#!/usr/bin/env python3
"""
CPU Process 11: Pattern Matching and String Search
Tests CPU with various string matching algorithms.
"""
import os
import time
import random
import psutil
import json
from datetime import datetime

def generate_random_text(length, alphabet_size=26):
    """Generate random text"""
    alphabet = 'abcdefghijklmnopqrstuvwxyz'[:alphabet_size]
    return ''.join(random.choice(alphabet) for _ in range(length))

def generate_pattern(length, text=None):
    """Generate search pattern"""
    if text and len(text) >= length and random.random() < 0.3:
        # Sometimes use substring from text
        start = random.randint(0, len(text) - length)
        return text[start:start + length]
    else:
        # Generate random pattern
        return generate_random_text(length, alphabet_size=random.randint(4, 26))

def naive_search(text, pattern):
    """Naive string matching algorithm"""
    matches = []
    n = len(text)
    m = len(pattern)
    
    for i in range(n - m + 1):
        j = 0
        while j < m and text[i + j] == pattern[j]:
            j += 1
        if j == m:
            matches.append(i)
    
    return matches

def kmp_search(text, pattern):
    """Knuth-Morris-Pratt algorithm"""
    n = len(text)
    m = len(pattern)
    
    # Compute LPS (Longest Proper Prefix which is also Suffix) array
    lps = [0] * m
    length = 0
    i = 1
    
    while i < m:
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
    
    # Search for pattern
    matches = []
    i = 0  # index for text
    j = 0  # index for pattern
    
    while i < n:
        if pattern[j] == text[i]:
            i += 1
            j += 1
        
        if j == m:
            matches.append(i - j)
            j = lps[j - 1]
        elif i < n and pattern[j] != text[i]:
            if j != 0:
                j = lps[j - 1]
            else:
                i += 1
    
    return matches

def boyer_moore_search(text, pattern):
    """Boyer-Moore algorithm (simplified)"""
    n = len(text)
    m = len(pattern)
    
    if m == 0:
        return []
    
    # Bad character heuristic
    bad_char = {}
    for i in range(m):
        bad_char[pattern[i]] = i
    
    matches = []
    s = 0  # shift of the pattern
    
    while s <= n - m:
        j = m - 1
        
        while j >= 0 and pattern[j] == text[s + j]:
            j -= 1
        
        if j < 0:
            matches.append(s)
            s += (m - bad_char.get(text[s + m], -1)) if s + m < n else 1
        else:
            s += max(1, j - bad_char.get(text[s + j], -1))
    
    return matches

def rabin_karp_search(text, pattern, prime=101):
    """Rabin-Karp algorithm"""
    n = len(text)
    m = len(pattern)
    d = 256  # number of characters in alphabet
    
    if m > n:
        return []
    
    matches = []
    pattern_hash = 0
    text_hash = 0
    h = 1
    
    # Calculate h = d^(m-1) % prime
    for i in range(m - 1):
        h = (h * d) % prime
    
    # Calculate hash of pattern and first window
    for i in range(m):
        pattern_hash = (d * pattern_hash + ord(pattern[i])) % prime
        text_hash = (d * text_hash + ord(text[i])) % prime
    
    # Slide the pattern over text
    for i in range(n - m + 1):
        # Check if hash matches
        if pattern_hash == text_hash:
            # Verify character by character
            if text[i:i + m] == pattern:
                matches.append(i)
        
        # Calculate hash for next window
        if i < n - m:
            text_hash = (d * (text_hash - ord(text[i]) * h) + ord(text[i + m])) % prime
            if text_hash < 0:
                text_hash += prime
    
    return matches

def levenshtein_distance(s1, s2):
    """Calculate Levenshtein (edit) distance"""
    m = len(s1)
    n = len(s2)
    
    # Create distance matrix
    dp = [[0] * (n + 1) for _ in range(m + 1)]
    
    # Initialize base cases
    for i in range(m + 1):
        dp[i][0] = i
    for j in range(n + 1):
        dp[0][j] = j
    
    # Fill matrix
    for i in range(1, m + 1):
        for j in range(1, n + 1):
            if s1[i - 1] == s2[j - 1]:
                dp[i][j] = dp[i - 1][j - 1]
            else:
                dp[i][j] = 1 + min(dp[i - 1][j],      # deletion
                                   dp[i][j - 1],      # insertion
                                   dp[i - 1][j - 1])  # substitution
    
    return dp[m][n]

def longest_common_subsequence(s1, s2):
    """Find length of longest common subsequence"""
    m = len(s1)
    n = len(s2)
    
    dp = [[0] * (n + 1) for _ in range(m + 1)]
    
    for i in range(1, m + 1):
        for j in range(1, n + 1):
            if s1[i - 1] == s2[j - 1]:
                dp[i][j] = dp[i - 1][j - 1] + 1
            else:
                dp[i][j] = max(dp[i - 1][j], dp[i][j - 1])
    
    return dp[m][n]

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 11: Pattern Matching")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-11",
        "searches_completed": 0,
        "total_matches_found": 0,
        "edit_distances_computed": 0,
        "lcs_computed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    search_results = []
    
    try:
        # Phase 1: Compare search algorithms
        print(f"Phase 1: Testing various string search algorithms...")
        
        num_search_tests = random.randint(20, 40)
        
        for i in range(num_search_tests):
            text_length = random.randint(10000, 100000)
            pattern_length = random.randint(5, 50)
            
            text = generate_random_text(text_length)
            pattern = generate_pattern(pattern_length, text)
            
            algorithms = {
                'naive': naive_search,
                'kmp': kmp_search,
                'boyer_moore': boyer_moore_search,
                'rabin_karp': rabin_karp_search
            }
            
            algo_name = random.choice(list(algorithms.keys()))
            search_func = algorithms[algo_name]
            
            search_start = time.time()
            matches = search_func(text, pattern)
            search_time = time.time() - search_start
            
            stats["searches_completed"] += 1
            stats["total_matches_found"] += len(matches)
            
            search_results.append({
                'algorithm': algo_name,
                'text_length': text_length,
                'pattern_length': pattern_length,
                'matches': len(matches),
                'time': search_time
            })
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Search {i+1}/{num_search_tests}: {algo_name}, "
                      f"Text={text_length}, Pattern={pattern_length}, "
                      f"Matches={len(matches)}, Time={search_time:.3f}s, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 2: Multiple pattern search
        print(f"\nPhase 2: Multiple pattern search...")
        
        num_multi_tests = random.randint(10, 20)
        
        for i in range(num_multi_tests):
            text_length = random.randint(50000, 200000)
            text = generate_random_text(text_length)
            
            num_patterns = random.randint(5, 20)
            total_matches = 0
            
            for _ in range(num_patterns):
                pattern_length = random.randint(3, 20)
                pattern = generate_pattern(pattern_length, text)
                
                matches = kmp_search(text, pattern)
                total_matches += len(matches)
                stats["searches_completed"] += 1
            
            stats["total_matches_found"] += total_matches
            
            if (i + 1) % 3 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Multi-pattern {i+1}/{num_multi_tests}: "
                      f"Text={text_length}, Patterns={num_patterns}, "
                      f"Total matches={total_matches}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        # Phase 3: Edit distance computations
        print(f"\nPhase 3: Computing edit distances...")
        
        num_edit_tests = random.randint(50, 100)
        
        for i in range(num_edit_tests):
            len1 = random.randint(50, 300)
            len2 = random.randint(50, 300)
            
            s1 = generate_random_text(len1, alphabet_size=10)
            s2 = generate_random_text(len2, alphabet_size=10)
            
            # Sometimes make strings similar
            if random.random() < 0.3:
                # Make s2 similar to s1
                s2 = list(s1[:min(len1, len2)])
                # Apply some edits
                num_edits = random.randint(5, 20)
                for _ in range(num_edits):
                    if len(s2) > 0:
                        edit_type = random.choice(['insert', 'delete', 'substitute'])
                        pos = random.randint(0, len(s2) - 1)
                        
                        if edit_type == 'insert':
                            s2.insert(pos, random.choice('abcdefghij'))
                        elif edit_type == 'delete' and len(s2) > 1:
                            s2.pop(pos)
                        else:  # substitute
                            s2[pos] = random.choice('abcdefghij')
                
                s2 = ''.join(s2)
            
            distance = levenshtein_distance(s1, s2)
            stats["edit_distances_computed"] += 1
            
            if (i + 1) % 20 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Edit distance {i+1}/{num_edit_tests}: "
                      f"Lengths ({len1}, {len2}), Distance={distance}, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.005, 0.02))
        
        # Phase 4: Longest common subsequence
        print(f"\nPhase 4: Computing longest common subsequences...")
        
        num_lcs_tests = random.randint(30, 60)
        
        for i in range(num_lcs_tests):
            len1 = random.randint(100, 500)
            len2 = random.randint(100, 500)
            
            s1 = generate_random_text(len1, alphabet_size=8)
            s2 = generate_random_text(len2, alphabet_size=8)
            
            lcs_length = longest_common_subsequence(s1, s2)
            stats["lcs_computed"] += 1
            
            if (i + 1) % 15 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  LCS {i+1}/{num_lcs_tests}: "
                      f"Lengths ({len1}, {len2}), LCS length={lcs_length}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.04))
        
        # Phase 5: Stress test with large texts
        print(f"\nPhase 5: Large text pattern matching...")
        
        num_large_tests = random.randint(5, 10)
        
        for i in range(num_large_tests):
            text_length = random.randint(500000, 2000000)
            print(f"  Generating large text of length {text_length:,}...")
            
            text = generate_random_text(text_length, alphabet_size=4)
            
            # Search for multiple patterns
            num_patterns = random.randint(3, 8)
            
            for j in range(num_patterns):
                pattern_length = random.randint(10, 50)
                pattern = generate_pattern(pattern_length, text)
                
                # Use KMP for efficiency
                search_start = time.time()
                matches = kmp_search(text, pattern)
                search_time = time.time() - search_start
                
                stats["searches_completed"] += 1
                stats["total_matches_found"] += len(matches)
                
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                if j == 0:
                    print(f"    Pattern {j+1}/{num_patterns}: Length={pattern_length}, "
                          f"Matches={len(matches)}, Time={search_time:.3f}s, "
                          f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["avg_matches_per_search"] = stats["total_matches_found"] / stats["searches_completed"] if stats["searches_completed"] > 0 else 0
        
        # Save search results
        with open(f"{results_dir}/search_results.json", 'w') as f:
            json.dump(search_results[:100], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Searches completed: {stats['searches_completed']}")
        print(f"Total matches found: {stats['total_matches_found']:,}")
        print(f"Avg matches/search: {stats['avg_matches_per_search']:.2f}")
        print(f"Edit distances computed: {stats['edit_distances_computed']}")
        print(f"LCS computed: {stats['lcs_computed']}")
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
