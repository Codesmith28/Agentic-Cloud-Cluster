#!/usr/bin/env python3
"""
CPU Process 6: Recursive Algorithms and Tree Traversal
Tests CPU with recursive computations.
"""
import os
import time
import random
import psutil
import json
import sys
from datetime import datetime

# Increase recursion limit
sys.setrecursionlimit(50000)

def fibonacci_recursive(n, memo=None):
    """Fibonacci with memoization"""
    if memo is None:
        memo = {}
    
    if n in memo:
        return memo[n]
    
    if n <= 1:
        return n
    
    memo[n] = fibonacci_recursive(n-1, memo) + fibonacci_recursive(n-2, memo)
    return memo[n]

def factorial_recursive(n):
    """Recursive factorial"""
    if n <= 1:
        return 1
    return n * factorial_recursive(n-1)

def ackermann(m, n):
    """Ackermann function - extremely recursive"""
    if m == 0:
        return n + 1
    elif n == 0:
        return ackermann(m - 1, 1)
    else:
        return ackermann(m - 1, ackermann(m, n - 1))

def tower_of_hanoi(n, source, destination, auxiliary, moves):
    """Tower of Hanoi solution"""
    if n == 1:
        moves.append((source, destination))
        return
    
    tower_of_hanoi(n-1, source, auxiliary, destination, moves)
    moves.append((source, destination))
    tower_of_hanoi(n-1, auxiliary, destination, source, moves)

class TreeNode:
    def __init__(self, value):
        self.value = value
        self.left = None
        self.right = None

def build_random_tree(depth, current_depth=0):
    """Build random binary tree"""
    if current_depth >= depth:
        return None
    
    node = TreeNode(random.randint(1, 1000))
    
    if random.random() < 0.7:  # 70% chance of left child
        node.left = build_random_tree(depth, current_depth + 1)
    
    if random.random() < 0.7:  # 70% chance of right child
        node.right = build_random_tree(depth, current_depth + 1)
    
    return node

def tree_sum(node):
    """Calculate sum of all nodes"""
    if node is None:
        return 0
    return node.value + tree_sum(node.left) + tree_sum(node.right)

def tree_height(node):
    """Calculate tree height"""
    if node is None:
        return 0
    return 1 + max(tree_height(node.left), tree_height(node.right))

def count_nodes(node):
    """Count total nodes"""
    if node is None:
        return 0
    return 1 + count_nodes(node.left) + count_nodes(node.right)

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 6: Recursive Algorithms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-6",
        "recursive_calls": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    results = []
    
    try:
        # Phase 1: Fibonacci numbers
        print(f"Phase 1: Computing Fibonacci numbers...")
        
        fib_numbers = []
        for n in range(random.randint(100, 500), random.randint(501, 1000)):
            fib_n = fibonacci_recursive(n)
            fib_numbers.append((n, len(str(fib_n))))
            stats["recursive_calls"] += n
            
            if n % 50 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  fib({n}) has {len(str(fib_n))} digits, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
        
        # Phase 2: Factorials
        print(f"\nPhase 2: Computing factorials...")
        
        for n in range(random.randint(100, 500), random.randint(501, 1500)):
            fact_n = factorial_recursive(n)
            stats["recursive_calls"] += n
            
            if n % 100 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  {n}! has {len(str(fact_n))} digits, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.001, 0.005))
        
        # Phase 3: Ackermann function (careful with parameters)
        print(f"\nPhase 3: Computing Ackermann function...")
        
        for m in range(0, 4):
            for n in range(0, min(4, 10 - m)):
                try:
                    result = ackermann(m, n)
                    stats["recursive_calls"] += result  # Rough estimate
                    
                    results.append({
                        'function': 'ackermann',
                        'm': m,
                        'n': n,
                        'result': result
                    })
                    
                    cpu_percent = process.cpu_percent()
                    stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                    print(f"  A({m}, {n}) = {result}, CPU: {cpu_percent:.1f}%")
                    
                    time.sleep(random.uniform(0.01, 0.05))
                except RecursionError:
                    print(f"  A({m}, {n}) - recursion limit exceeded")
        
        # Phase 4: Tower of Hanoi
        print(f"\nPhase 4: Solving Tower of Hanoi...")
        
        for n_disks in range(random.randint(10, 15), random.randint(16, 22)):
            moves = []
            tower_of_hanoi(n_disks, 'A', 'C', 'B', moves)
            
            stats["recursive_calls"] += len(moves)
            
            results.append({
                'function': 'tower_of_hanoi',
                'disks': n_disks,
                'moves': len(moves),
                'optimal': len(moves) == (2**n_disks - 1)
            })
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  {n_disks} disks: {len(moves)} moves, "
                  f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 5: Binary tree traversals
        print(f"\nPhase 5: Binary tree operations...")
        
        num_trees = random.randint(20, 50)
        
        for i in range(num_trees):
            depth = random.randint(10, 18)
            tree = build_random_tree(depth)
            
            # Perform various recursive operations
            total_sum = tree_sum(tree)
            height = tree_height(tree)
            num_nodes = count_nodes(tree)
            
            stats["recursive_calls"] += num_nodes * 3  # Three traversals
            
            results.append({
                'function': 'tree_operations',
                'depth': depth,
                'nodes': num_nodes,
                'height': height,
                'sum': total_sum
            })
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Tree {i+1}/{num_trees}: depth={depth}, nodes={num_nodes}, "
                      f"height={height}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.02, 0.08))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/recursive_results.json", 'w') as f:
            json.dump(results, f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Recursive calls (estimate): {stats['recursive_calls']:,}")
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
