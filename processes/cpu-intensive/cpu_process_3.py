#!/usr/bin/env python3
"""
CPU Process 3: Sorting Algorithms Benchmark
Tests various sorting algorithms on large datasets.
"""
import os
import time
import random
import psutil
import json
from datetime import datetime

def bubble_sort(arr):
    """Bubble sort implementation"""
    n = len(arr)
    arr = arr.copy()
    for i in range(n):
        for j in range(0, n - i - 1):
            if arr[j] > arr[j + 1]:
                arr[j], arr[j + 1] = arr[j + 1], arr[j]
    return arr

def quick_sort(arr):
    """Quick sort implementation"""
    if len(arr) <= 1:
        return arr
    pivot = arr[len(arr) // 2]
    left = [x for x in arr if x < pivot]
    middle = [x for x in arr if x == pivot]
    right = [x for x in arr if x > pivot]
    return quick_sort(left) + middle + quick_sort(right)

def merge_sort(arr):
    """Merge sort implementation"""
    if len(arr) <= 1:
        return arr
    
    mid = len(arr) // 2
    left = merge_sort(arr[:mid])
    right = merge_sort(arr[mid:])
    
    return merge(left, right)

def merge(left, right):
    """Merge two sorted arrays"""
    result = []
    i = j = 0
    
    while i < len(left) and j < len(right):
        if left[i] <= right[j]:
            result.append(left[i])
            i += 1
        else:
            result.append(right[j])
            j += 1
    
    result.extend(left[i:])
    result.extend(right[j:])
    return result

def heap_sort(arr):
    """Heap sort implementation"""
    arr = arr.copy()
    n = len(arr)
    
    # Build max heap
    for i in range(n // 2 - 1, -1, -1):
        heapify(arr, n, i)
    
    # Extract elements
    for i in range(n - 1, 0, -1):
        arr[0], arr[i] = arr[i], arr[0]
        heapify(arr, i, 0)
    
    return arr

def heapify(arr, n, i):
    """Heapify subtree"""
    largest = i
    left = 2 * i + 1
    right = 2 * i + 2
    
    if left < n and arr[left] > arr[largest]:
        largest = left
    
    if right < n and arr[right] > arr[largest]:
        largest = right
    
    if largest != i:
        arr[i], arr[largest] = arr[largest], arr[i]
        heapify(arr, n, largest)

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 3: Sorting Algorithms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-3",
        "sorts_completed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0,
        "algorithm_stats": {}
    }
    
    start_time = time.time()
    
    try:
        algorithms = {
            'bubble_sort': (bubble_sort, random.randint(1000, 3000)),
            'quick_sort': (quick_sort, random.randint(10000, 50000)),
            'merge_sort': (merge_sort, random.randint(10000, 50000)),
            'heap_sort': (heap_sort, random.randint(10000, 50000)),
        }
        
        all_results = []
        
        for algo_name, (algo_func, size) in algorithms.items():
            print(f"\nTesting {algo_name} with array size {size:,}...")
            algo_stats = {
                'runs': 0,
                'total_time': 0,
                'avg_time': 0,
                'min_time': float('inf'),
                'max_time': 0
            }
            
            # Run multiple times with different data patterns
            runs = random.randint(3, 8)
            
            for run in range(runs):
                # Generate test data with different patterns
                if run % 4 == 0:
                    # Random data
                    data = [random.randint(1, 1000000) for _ in range(size)]
                    pattern = "random"
                elif run % 4 == 1:
                    # Nearly sorted
                    data = list(range(size))
                    for _ in range(size // 10):
                        i, j = random.randint(0, size-1), random.randint(0, size-1)
                        data[i], data[j] = data[j], data[i]
                    pattern = "nearly_sorted"
                elif run % 4 == 2:
                    # Reverse sorted
                    data = list(range(size, 0, -1))
                    pattern = "reverse_sorted"
                else:
                    # Many duplicates
                    data = [random.randint(1, 100) for _ in range(size)]
                    pattern = "many_duplicates"
                
                # Measure sort time
                start = time.time()
                sorted_data = algo_func(data)
                duration = time.time() - start
                
                # Verify sort
                is_sorted = all(sorted_data[i] <= sorted_data[i+1] for i in range(len(sorted_data)-1))
                
                algo_stats['runs'] += 1
                algo_stats['total_time'] += duration
                algo_stats['min_time'] = min(algo_stats['min_time'], duration)
                algo_stats['max_time'] = max(algo_stats['max_time'], duration)
                
                stats["sorts_completed"] += 1
                
                # Track resources
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                all_results.append({
                    'algorithm': algo_name,
                    'run': run + 1,
                    'size': size,
                    'pattern': pattern,
                    'duration': duration,
                    'is_sorted': is_sorted,
                    'cpu_percent': cpu_percent
                })
                
                print(f"  Run {run+1}/{runs} ({pattern}): {duration:.3f}s, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
                
                time.sleep(random.uniform(0.05, 0.15))
            
            algo_stats['avg_time'] = algo_stats['total_time'] / algo_stats['runs']
            stats["algorithm_stats"][algo_name] = algo_stats
            
            print(f"  {algo_name} completed: Avg={algo_stats['avg_time']:.3f}s, "
                  f"Min={algo_stats['min_time']:.3f}s, Max={algo_stats['max_time']:.3f}s")
        
        # Additional stress test with large dataset
        print(f"\nFinal stress test: sorting large random array...")
        large_size = random.randint(50000, 100000)
        large_data = [random.randint(1, 1000000) for _ in range(large_size)]
        
        start = time.time()
        sorted_large = merge_sort(large_data)
        stress_duration = time.time() - start
        
        print(f"  Sorted {large_size:,} elements in {stress_duration:.3f}s")
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save detailed results
        with open(f"{results_dir}/sort_results.json", 'w') as f:
            json.dump(all_results, f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Sorts completed: {stats['sorts_completed']}")
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
