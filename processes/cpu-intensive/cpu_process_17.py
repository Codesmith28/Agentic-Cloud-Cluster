#!/usr/bin/env python3
"""
CPU Process 17: Sorting Algorithm Benchmarks
Tests various sorting algorithms on large datasets.
"""
import os
import time
import random
import psutil
import json
import csv
import numpy as np
from datetime import datetime

def bubble_sort(arr):
    """Bubble sort implementation"""
    n = len(arr)
    for i in range(n):
        for j in range(0, n-i-1):
            if arr[j] > arr[j+1]:
                arr[j], arr[j+1] = arr[j+1], arr[j]
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
    def heapify(arr, n, i):
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
    
    n = len(arr)
    for i in range(n // 2 - 1, -1, -1):
        heapify(arr, n, i)
    
    for i in range(n - 1, 0, -1):
        arr[0], arr[i] = arr[i], arr[0]
        heapify(arr, i, 0)
    
    return arr

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 17: Sorting Algorithms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-17",
        "arrays_sorted": 0,
        "total_elements": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    results = []
    
    try:
        # Test different array sizes
        array_sizes = [1000, 5000, 10000, 20000]
        
        print("Testing sorting algorithms on various array sizes...")
        
        # Quick sort on large arrays
        print("\nPhase 1: Quick sort...")
        for size in array_sizes:
            for trial in range(random.randint(5, 15)):
                arr = [random.randint(1, 100000) for _ in range(size)]
                
                sort_start = time.time()
                sorted_arr = quick_sort(arr.copy())
                sort_time = time.time() - sort_start
                
                results.append({
                    'algorithm': 'quick_sort',
                    'size': size,
                    'time': sort_time,
                    'trial': trial
                })
                
                stats["arrays_sorted"] += 1
                stats["total_elements"] += size
                
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                time.sleep(random.uniform(0.01, 0.05))
        
        print(f"  Quick sort completed, CPU: {cpu_percent:.1f}%")
        
        # Merge sort
        print("\nPhase 2: Merge sort...")
        for size in array_sizes:
            for trial in range(random.randint(5, 15)):
                arr = [random.randint(1, 100000) for _ in range(size)]
                
                sort_start = time.time()
                sorted_arr = merge_sort(arr.copy())
                sort_time = time.time() - sort_start
                
                results.append({
                    'algorithm': 'merge_sort',
                    'size': size,
                    'time': sort_time,
                    'trial': trial
                })
                
                stats["arrays_sorted"] += 1
                stats["total_elements"] += size
                
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                time.sleep(random.uniform(0.01, 0.05))
        
        print(f"  Merge sort completed, CPU: {cpu_percent:.1f}%")
        
        # Heap sort
        print("\nPhase 3: Heap sort...")
        for size in array_sizes:
            for trial in range(random.randint(5, 15)):
                arr = [random.randint(1, 100000) for _ in range(size)]
                
                sort_start = time.time()
                sorted_arr = heap_sort(arr.copy())
                sort_time = time.time() - sort_start
                
                results.append({
                    'algorithm': 'heap_sort',
                    'size': size,
                    'time': sort_time,
                    'trial': trial
                })
                
                stats["arrays_sorted"] += 1
                stats["total_elements"] += size
                
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                time.sleep(random.uniform(0.01, 0.05))
        
        print(f"  Heap sort completed, CPU: {cpu_percent:.1f}%")
        
        # NumPy sort for comparison (very large arrays)
        print("\nPhase 4: NumPy sort on large arrays...")
        large_sizes = [100000, 500000, 1000000]
        
        for size in large_sizes:
            for trial in range(random.randint(3, 8)):
                arr = np.random.randint(1, 1000000, size)
                
                sort_start = time.time()
                sorted_arr = np.sort(arr)
                sort_time = time.time() - sort_start
                
                results.append({
                    'algorithm': 'numpy_sort',
                    'size': size,
                    'time': sort_time,
                    'trial': trial
                })
                
                stats["arrays_sorted"] += 1
                stats["total_elements"] += size
                
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                
                print(f"  Sorted {size:,} elements in {sort_time:.4f}s, CPU: {cpu_percent:.1f}%")
                time.sleep(random.uniform(0.1, 0.3))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results to CSV
        with open(f"{results_dir}/sorting_benchmark.csv", 'w', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=['algorithm', 'size', 'time', 'trial'])
            writer.writeheader()
            writer.writerows(results)
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Arrays sorted: {stats['arrays_sorted']}")
        print(f"Total elements: {stats['total_elements']:,}")
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
