#!/usr/bin/env python3
"""
Workload Type: cpu-heavy
Description: Sorting algorithm comparison and benchmarking
Resource Requirements: High CPU, moderate memory
Output: sort_benchmark.csv (generated file)
"""

import datetime
import random
import csv
import time

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

def benchmark_sorts():
    print(f"[{datetime.datetime.now()}] Starting Sort Benchmark...")
    
    sizes = [1000, 5000, 10000]
    algorithms = [
        ('Bubble Sort', bubble_sort),
        ('Quick Sort', quick_sort),
        ('Merge Sort', merge_sort),
        ('Built-in Sort', sorted),
    ]
    
    results = []
    
    for size in sizes:
        print(f"\n  Testing with {size:,} elements:")
        data = [random.randint(1, 10000) for _ in range(size)]
        
        for name, sort_func in algorithms:
            test_data = data.copy()
            
            start = time.time()
            if name == 'Bubble Sort' and size > 5000:
                # Skip bubble sort for large arrays (too slow)
                duration = 'N/A'
                print(f"    {name}: Skipped (too slow for {size} elements)")
            else:
                sorted_data = sort_func(test_data)
                end = time.time()
                duration = f"{(end - start):.4f}"
                print(f"    {name}: {duration}s")
            
            results.append({
                'Algorithm': name,
                'Size': size,
                'Duration': duration
            })
    
    # Save results
    with open('/output/sort_benchmark.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['Algorithm', 'Size', 'Duration'])
        writer.writeheader()
        writer.writerows(results)
    
    print(f"\n✓ Generated sort_benchmark.csv")
    print(f"[{datetime.datetime.now()}] Sort Benchmark completed ✓")
    return 0

if __name__ == "__main__":
    exit(benchmark_sorts())
