#!/usr/bin/env python3
"""
Workload Type: cpu-heavy
Description: Matrix multiplication and operations
Resource Requirements: High CPU, moderate memory
Output: matrix_results.csv (generated file)
"""

import datetime
import random
import csv

def create_matrix(rows, cols):
    """Create a random matrix"""
    return [[random.uniform(-10, 10) for _ in range(cols)] for _ in range(rows)]

def multiply_matrices(A, B):
    """Multiply two matrices"""
    rows_A, cols_A = len(A), len(A[0])
    rows_B, cols_B = len(B), len(B[0])
    
    if cols_A != rows_B:
        raise ValueError("Matrix dimensions don't match")
    
    result = [[0 for _ in range(cols_B)] for _ in range(rows_A)]
    
    for i in range(rows_A):
        for j in range(cols_B):
            for k in range(cols_A):
                result[i][j] += A[i][k] * B[k][j]
    
    return result

def matrix_operations():
    print(f"[{datetime.datetime.now()}] Starting Matrix Operations...")
    
    sizes = [(100, 100), (200, 200), (300, 300)]
    results = []
    
    for size in sizes:
        rows, cols = size
        print(f"\n  Processing {rows}x{cols} matrices...")
        
        start = datetime.datetime.now()
        
        # Create matrices
        A = create_matrix(rows, cols)
        B = create_matrix(rows, cols)
        
        # Multiply
        C = multiply_matrices(A, B)
        
        end = datetime.datetime.now()
        duration = (end - start).total_seconds()
        
        # Calculate sum for verification
        total_sum = sum(sum(row) for row in C)
        
        print(f"  ✓ Multiplication completed in {duration:.2f}s")
        print(f"  ✓ Result sum: {total_sum:.2f}")
        
        results.append({
            'Size': f"{rows}x{cols}",
            'Duration': f"{duration:.2f}",
            'Sum': f"{total_sum:.2f}"
        })
    
    # Save results
    with open('/output/matrix_results.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['Size', 'Duration', 'Sum'])
        writer.writeheader()
        writer.writerows(results)
    
    print(f"\n✓ Generated matrix_results.csv")
    print(f"[{datetime.datetime.now()}] Matrix Operations completed ✓")
    return 0

if __name__ == "__main__":
    exit(matrix_operations())
