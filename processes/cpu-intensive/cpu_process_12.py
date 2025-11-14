#!/usr/bin/env python3
"""
CPU Process 12: Combinatorial Optimization and Backtracking
Tests CPU with NP-hard problems and backtracking algorithms.
"""
import os
import time
import random
import psutil
import json
from datetime import datetime
from itertools import permutations, combinations

def solve_n_queens(n):
    """Solve N-Queens problem using backtracking"""
    solutions = []
    board = [-1] * n
    
    def is_safe(row, col):
        for i in range(row):
            if board[i] == col or \
               board[i] - i == col - row or \
               board[i] + i == col + row:
                return False
        return True
    
    def backtrack(row):
        if row == n:
            solutions.append(board[:])
            return
        
        for col in range(n):
            if is_safe(row, col):
                board[row] = col
                backtrack(row + 1)
                board[row] = -1
    
    backtrack(0)
    return solutions

def solve_sudoku(board):
    """Solve Sudoku puzzle using backtracking"""
    def is_valid(board, row, col, num):
        # Check row
        if num in board[row]:
            return False
        
        # Check column
        if num in [board[i][col] for i in range(9)]:
            return False
        
        # Check 3x3 box
        box_row, box_col = 3 * (row // 3), 3 * (col // 3)
        for i in range(box_row, box_row + 3):
            for j in range(box_col, box_col + 3):
                if board[i][j] == num:
                    return False
        
        return True
    
    def solve():
        for i in range(9):
            for j in range(9):
                if board[i][j] == 0:
                    for num in range(1, 10):
                        if is_valid(board, i, j, num):
                            board[i][j] = num
                            
                            if solve():
                                return True
                            
                            board[i][j] = 0
                    
                    return False
        return True
    
    solve()
    return board

def generate_sudoku():
    """Generate a random Sudoku puzzle"""
    board = [[0] * 9 for _ in range(9)]
    
    # Fill diagonal 3x3 boxes
    for box in range(0, 9, 3):
        nums = list(range(1, 10))
        random.shuffle(nums)
        idx = 0
        for i in range(box, box + 3):
            for j in range(box, box + 3):
                board[i][j] = nums[idx]
                idx += 1
    
    # Remove some numbers
    num_remove = random.randint(40, 55)
    for _ in range(num_remove):
        i, j = random.randint(0, 8), random.randint(0, 8)
        board[i][j] = 0
    
    return board

def knapsack_01(weights, values, capacity):
    """0/1 Knapsack problem using dynamic programming"""
    n = len(weights)
    dp = [[0] * (capacity + 1) for _ in range(n + 1)]
    
    for i in range(1, n + 1):
        for w in range(capacity + 1):
            if weights[i - 1] <= w:
                dp[i][w] = max(values[i - 1] + dp[i - 1][w - weights[i - 1]], 
                              dp[i - 1][w])
            else:
                dp[i][w] = dp[i - 1][w]
    
    # Backtrack to find items
    items = []
    w = capacity
    for i in range(n, 0, -1):
        if dp[i][w] != dp[i - 1][w]:
            items.append(i - 1)
            w -= weights[i - 1]
    
    return dp[n][capacity], items

def tsp_brute_force(distances):
    """Traveling Salesman Problem using brute force (small instances)"""
    n = len(distances)
    cities = list(range(1, n))
    min_path = None
    min_cost = float('inf')
    
    for perm in permutations(cities):
        cost = distances[0][perm[0]]
        
        for i in range(len(perm) - 1):
            cost += distances[perm[i]][perm[i + 1]]
        
        cost += distances[perm[-1]][0]
        
        if cost < min_cost:
            min_cost = cost
            min_path = [0] + list(perm) + [0]
    
    return min_cost, min_path

def subset_sum(numbers, target):
    """Find subset that sums to target using backtracking"""
    solutions = []
    
    def backtrack(start, current_subset, current_sum):
        if current_sum == target:
            solutions.append(current_subset[:])
            return
        
        if current_sum > target:
            return
        
        for i in range(start, len(numbers)):
            current_subset.append(numbers[i])
            backtrack(i + 1, current_subset, current_sum + numbers[i])
            current_subset.pop()
    
    backtrack(0, [], 0)
    return solutions

def graph_coloring(graph, num_colors):
    """Graph coloring using backtracking"""
    n = len(graph)
    colors = [-1] * n
    
    def is_safe(vertex, color):
        for i in range(n):
            if graph[vertex][i] and colors[i] == color:
                return False
        return True
    
    def backtrack(vertex):
        if vertex == n:
            return True
        
        for color in range(num_colors):
            if is_safe(vertex, color):
                colors[vertex] = color
                
                if backtrack(vertex + 1):
                    return True
                
                colors[vertex] = -1
        
        return False
    
    if backtrack(0):
        return colors
    return None

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 12: Combinatorial Optimization")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-12",
        "n_queens_solved": 0,
        "sudoku_solved": 0,
        "knapsacks_solved": 0,
        "tsp_solved": 0,
        "subset_sums_found": 0,
        "graph_colorings": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    optimization_results = []
    
    try:
        # Phase 1: N-Queens problem
        print(f"Phase 1: Solving N-Queens problem...")
        
        num_queens_tests = random.randint(5, 10)
        
        for i in range(num_queens_tests):
            n = random.randint(8, 12)
            
            solve_start = time.time()
            solutions = solve_n_queens(n)
            solve_time = time.time() - solve_start
            
            stats["n_queens_solved"] += 1
            
            optimization_results.append({
                'problem': 'n_queens',
                'n': n,
                'solutions': len(solutions),
                'time': solve_time
            })
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  {n}-Queens: {len(solutions)} solutions found in {solve_time:.3f}s, "
                  f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 2: Sudoku solving
        print(f"\nPhase 2: Solving Sudoku puzzles...")
        
        num_sudoku_tests = random.randint(10, 20)
        
        for i in range(num_sudoku_tests):
            puzzle = generate_sudoku()
            
            # Count empty cells
            empty_cells = sum(row.count(0) for row in puzzle)
            
            solve_start = time.time()
            solution = solve_sudoku([row[:] for row in puzzle])
            solve_time = time.time() - solve_start
            
            stats["sudoku_solved"] += 1
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Sudoku {i+1}/{num_sudoku_tests}: {empty_cells} empty cells, "
                      f"Time={solve_time:.3f}s, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        # Phase 3: Knapsack problem
        print(f"\nPhase 3: Solving 0/1 Knapsack problems...")
        
        num_knapsack_tests = random.randint(20, 40)
        
        for i in range(num_knapsack_tests):
            num_items = random.randint(20, 100)
            weights = [random.randint(1, 50) for _ in range(num_items)]
            values = [random.randint(1, 100) for _ in range(num_items)]
            capacity = random.randint(sum(weights) // 4, sum(weights) // 2)
            
            solve_start = time.time()
            max_value, selected_items = knapsack_01(weights, values, capacity)
            solve_time = time.time() - solve_start
            
            stats["knapsacks_solved"] += 1
            
            optimization_results.append({
                'problem': 'knapsack',
                'items': num_items,
                'capacity': capacity,
                'max_value': max_value,
                'selected': len(selected_items),
                'time': solve_time
            })
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Knapsack {i+1}/{num_knapsack_tests}: {num_items} items, "
                      f"Max value={max_value}, Selected={len(selected_items)}, "
                      f"Time={solve_time:.3f}s, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 4: Traveling Salesman Problem (small instances)
        print(f"\nPhase 4: Solving TSP...")
        
        num_tsp_tests = random.randint(5, 10)
        
        for i in range(num_tsp_tests):
            n_cities = random.randint(6, 9)
            
            # Generate random distance matrix
            distances = [[0] * n_cities for _ in range(n_cities)]
            for j in range(n_cities):
                for k in range(j + 1, n_cities):
                    dist = random.randint(10, 100)
                    distances[j][k] = dist
                    distances[k][j] = dist
            
            solve_start = time.time()
            min_cost, min_path = tsp_brute_force(distances)
            solve_time = time.time() - solve_start
            
            stats["tsp_solved"] += 1
            
            optimization_results.append({
                'problem': 'tsp',
                'cities': n_cities,
                'min_cost': min_cost,
                'time': solve_time
            })
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  TSP {i+1}/{num_tsp_tests}: {n_cities} cities, "
                  f"Min cost={min_cost}, Time={solve_time:.3f}s, "
                  f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.2, 0.5))
        
        # Phase 5: Subset sum problem
        print(f"\nPhase 5: Finding subset sums...")
        
        num_subset_tests = random.randint(15, 30)
        
        for i in range(num_subset_tests):
            n = random.randint(15, 25)
            numbers = [random.randint(1, 50) for _ in range(n)]
            target = random.randint(sum(numbers) // 4, sum(numbers) // 2)
            
            solve_start = time.time()
            solutions = subset_sum(numbers, target)
            solve_time = time.time() - solve_start
            
            stats["subset_sums_found"] += len(solutions)
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Subset sum {i+1}/{num_subset_tests}: {n} numbers, "
                      f"Target={target}, Solutions={len(solutions)}, "
                      f"Time={solve_time:.3f}s, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        # Phase 6: Graph coloring
        print(f"\nPhase 6: Graph coloring problems...")
        
        num_coloring_tests = random.randint(10, 20)
        
        for i in range(num_coloring_tests):
            n_vertices = random.randint(10, 30)
            edge_probability = random.uniform(0.2, 0.4)
            
            # Generate random graph
            graph = [[0] * n_vertices for _ in range(n_vertices)]
            edges = 0
            for j in range(n_vertices):
                for k in range(j + 1, n_vertices):
                    if random.random() < edge_probability:
                        graph[j][k] = 1
                        graph[k][j] = 1
                        edges += 1
            
            # Try with different number of colors
            num_colors = random.randint(3, 6)
            
            solve_start = time.time()
            coloring = graph_coloring(graph, num_colors)
            solve_time = time.time() - solve_start
            
            if coloring:
                stats["graph_colorings"] += 1
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Graph coloring {i+1}/{num_coloring_tests}: "
                      f"{n_vertices} vertices, {edges} edges, "
                      f"{num_colors} colors, "
                      f"Success={coloring is not None}, "
                      f"Time={solve_time:.3f}s, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save optimization results
        with open(f"{results_dir}/optimization_results.json", 'w') as f:
            json.dump(optimization_results, f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"N-Queens solved: {stats['n_queens_solved']}")
        print(f"Sudoku puzzles solved: {stats['sudoku_solved']}")
        print(f"Knapsack problems solved: {stats['knapsacks_solved']}")
        print(f"TSP instances solved: {stats['tsp_solved']}")
        print(f"Subset sums found: {stats['subset_sums_found']}")
        print(f"Graph colorings: {stats['graph_colorings']}")
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
