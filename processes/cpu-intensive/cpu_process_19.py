#!/usr/bin/env python3
"""
CPU Process 19: Graph Algorithms
Shortest path, spanning tree, and graph traversal computations.
"""
import os
import time
import random
import psutil
import json
import numpy as np
from datetime import datetime
from collections import deque, defaultdict
import heapq

class Graph:
    def __init__(self, vertices):
        self.V = vertices
        self.adj = defaultdict(list)
        self.weights = {}
    
    def add_edge(self, u, v, weight=1):
        self.adj[u].append(v)
        self.weights[(u, v)] = weight
    
    def dijkstra(self, start):
        """Dijkstra's shortest path algorithm"""
        dist = {i: float('inf') for i in range(self.V)}
        dist[start] = 0
        pq = [(0, start)]
        
        while pq:
            d, u = heapq.heappop(pq)
            
            if d > dist[u]:
                continue
            
            for v in self.adj[u]:
                weight = self.weights.get((u, v), 1)
                if dist[u] + weight < dist[v]:
                    dist[v] = dist[u] + weight
                    heapq.heappush(pq, (dist[v], v))
        
        return dist
    
    def bfs(self, start):
        """Breadth-first search"""
        visited = [False] * self.V
        queue = deque([start])
        visited[start] = True
        order = []
        
        while queue:
            u = queue.popleft()
            order.append(u)
            
            for v in self.adj[u]:
                if not visited[v]:
                    visited[v] = True
                    queue.append(v)
        
        return order
    
    def dfs(self, start, visited=None):
        """Depth-first search"""
        if visited is None:
            visited = [False] * self.V
        
        visited[start] = True
        order = [start]
        
        for v in self.adj[start]:
            if not visited[v]:
                order.extend(self.dfs(v, visited))
        
        return order

def generate_random_graph(num_vertices, edge_probability=0.1):
    """Generate random graph"""
    g = Graph(num_vertices)
    
    for i in range(num_vertices):
        for j in range(i + 1, num_vertices):
            if random.random() < edge_probability:
                weight = random.randint(1, 100)
                g.add_edge(i, j, weight)
                g.add_edge(j, i, weight)  # Undirected
    
    return g

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 19: Graph Algorithms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-19",
        "graphs_processed": 0,
        "shortest_paths_computed": 0,
        "traversals_completed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    results = []
    
    try:
        # Phase 1: Dijkstra on multiple graphs
        print("Phase 1: Computing shortest paths with Dijkstra...")
        
        num_graphs = random.randint(20, 50)
        
        for i in range(num_graphs):
            num_vertices = random.randint(100, 1000)
            edge_prob = random.uniform(0.05, 0.2)
            
            g = generate_random_graph(num_vertices, edge_prob)
            
            # Run Dijkstra from random sources
            num_sources = min(10, num_vertices)
            for source in random.sample(range(num_vertices), num_sources):
                distances = g.dijkstra(source)
                avg_dist = np.mean([d for d in distances.values() if d != float('inf')])
                
                results.append({
                    'algorithm': 'dijkstra',
                    'vertices': num_vertices,
                    'source': source,
                    'avg_distance': float(avg_dist)
                })
                
                stats["shortest_paths_computed"] += 1
            
            stats["graphs_processed"] += 1
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Processed {i+1}/{num_graphs} graphs, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        # Phase 2: BFS traversals
        print("\nPhase 2: Breadth-first search traversals...")
        
        for i in range(num_graphs // 2):
            num_vertices = random.randint(500, 2000)
            g = generate_random_graph(num_vertices, 0.05)
            
            source = random.randint(0, num_vertices - 1)
            bfs_order = g.bfs(source)
            
            results.append({
                'algorithm': 'bfs',
                'vertices': num_vertices,
                'visited': len(bfs_order)
            })
            
            stats["traversals_completed"] += 1
            
            cpu_percent = process.cpu_percent()
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            
            time.sleep(random.uniform(0.05, 0.2))
        
        print(f"  BFS completed, CPU: {cpu_percent:.1f}%")
        
        # Phase 3: DFS traversals
        print("\nPhase 3: Depth-first search traversals...")
        
        for i in range(num_graphs // 2):
            num_vertices = random.randint(500, 2000)
            g = generate_random_graph(num_vertices, 0.05)
            
            source = random.randint(0, num_vertices - 1)
            dfs_order = g.dfs(source)
            
            results.append({
                'algorithm': 'dfs',
                'vertices': num_vertices,
                'visited': len(dfs_order)
            })
            
            stats["traversals_completed"] += 1
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            time.sleep(random.uniform(0.05, 0.2))
        
        print(f"  DFS completed, CPU: {cpu_percent:.1f}%")
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/graph_results.json", 'w') as f:
            json.dump(results, f, indent=2)
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Graphs processed: {stats['graphs_processed']}")
        print(f"Shortest paths computed: {stats['shortest_paths_computed']}")
        print(f"Traversals completed: {stats['traversals_completed']}")
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
