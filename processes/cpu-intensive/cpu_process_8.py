#!/usr/bin/env python3
"""
CPU Process 8: Graph Algorithms and Network Analysis
Tests CPU with graph traversal and analysis algorithms.
"""
import os
import time
import random
import psutil
import json
from datetime import datetime
from collections import deque, defaultdict

class Graph:
    def __init__(self, vertices):
        self.V = vertices
        self.adj = defaultdict(list)
        self.weights = {}
    
    def add_edge(self, u, v, weight=1):
        self.adj[u].append(v)
        self.weights[(u, v)] = weight
    
    def get_weight(self, u, v):
        return self.weights.get((u, v), 1)

def generate_random_graph(num_vertices, edge_probability=0.3, directed=False):
    """Generate a random graph"""
    graph = Graph(num_vertices)
    
    for i in range(num_vertices):
        for j in range(num_vertices):
            if i != j and random.random() < edge_probability:
                weight = random.randint(1, 100)
                graph.add_edge(i, j, weight)
                if not directed:
                    graph.add_edge(j, i, weight)
    
    return graph

def bfs(graph, start):
    """Breadth-First Search"""
    visited = set()
    queue = deque([start])
    visited.add(start)
    order = []
    
    while queue:
        vertex = queue.popleft()
        order.append(vertex)
        
        for neighbor in graph.adj[vertex]:
            if neighbor not in visited:
                visited.add(neighbor)
                queue.append(neighbor)
    
    return order

def dfs(graph, start, visited=None):
    """Depth-First Search"""
    if visited is None:
        visited = set()
    
    visited.add(start)
    order = [start]
    
    for neighbor in graph.adj[start]:
        if neighbor not in visited:
            order.extend(dfs(graph, neighbor, visited))
    
    return order

def dijkstra(graph, start):
    """Dijkstra's shortest path algorithm"""
    distances = {i: float('infinity') for i in range(graph.V)}
    distances[start] = 0
    visited = set()
    
    for _ in range(graph.V):
        # Find unvisited vertex with minimum distance
        min_dist = float('infinity')
        min_vertex = None
        
        for v in range(graph.V):
            if v not in visited and distances[v] < min_dist:
                min_dist = distances[v]
                min_vertex = v
        
        if min_vertex is None:
            break
        
        visited.add(min_vertex)
        
        # Update distances to neighbors
        for neighbor in graph.adj[min_vertex]:
            weight = graph.get_weight(min_vertex, neighbor)
            distance = distances[min_vertex] + weight
            
            if distance < distances[neighbor]:
                distances[neighbor] = distance
    
    return distances

def detect_cycle_directed(graph):
    """Detect cycle in directed graph using DFS"""
    visited = set()
    rec_stack = set()
    
    def dfs_cycle(v):
        visited.add(v)
        rec_stack.add(v)
        
        for neighbor in graph.adj[v]:
            if neighbor not in visited:
                if dfs_cycle(neighbor):
                    return True
            elif neighbor in rec_stack:
                return True
        
        rec_stack.remove(v)
        return False
    
    for vertex in range(graph.V):
        if vertex not in visited:
            if dfs_cycle(vertex):
                return True
    
    return False

def find_strongly_connected_components(graph):
    """Find strongly connected components using Kosaraju's algorithm"""
    visited = set()
    stack = []
    
    def dfs_fill(v):
        visited.add(v)
        for neighbor in graph.adj[v]:
            if neighbor not in visited:
                dfs_fill(neighbor)
        stack.append(v)
    
    # Fill stack with vertices in order of completion
    for v in range(graph.V):
        if v not in visited:
            dfs_fill(v)
    
    # Create transpose graph
    transpose = Graph(graph.V)
    for v in range(graph.V):
        for neighbor in graph.adj[v]:
            transpose.add_edge(neighbor, v)
    
    # DFS on transpose in reverse order
    visited.clear()
    components = []
    
    def dfs_component(v, component):
        visited.add(v)
        component.append(v)
        for neighbor in transpose.adj[v]:
            if neighbor not in visited:
                dfs_component(neighbor, component)
    
    while stack:
        v = stack.pop()
        if v not in visited:
            component = []
            dfs_component(v, component)
            components.append(component)
    
    return components

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 8: Graph Algorithms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-8",
        "graphs_processed": 0,
        "algorithms_executed": 0,
        "total_vertices": 0,
        "total_edges": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    algorithm_results = []
    
    try:
        # Phase 1: BFS and DFS on various graphs
        print(f"Phase 1: Testing BFS and DFS...")
        
        num_graphs = random.randint(20, 50)
        
        for i in range(num_graphs):
            num_vertices = random.randint(50, 300)
            edge_prob = random.uniform(0.1, 0.4)
            
            graph = generate_random_graph(num_vertices, edge_prob)
            
            # Count edges
            num_edges = sum(len(graph.adj[v]) for v in range(graph.V))
            stats["total_vertices"] += num_vertices
            stats["total_edges"] += num_edges
            stats["graphs_processed"] += 1
            
            # Run BFS from random start
            start_vertex = random.randint(0, num_vertices - 1)
            bfs_order = bfs(graph, start_vertex)
            stats["algorithms_executed"] += 1
            
            # Run DFS from same start
            dfs_order = dfs(graph, start_vertex)
            stats["algorithms_executed"] += 1
            
            algorithm_results.append({
                'graph_id': i,
                'vertices': num_vertices,
                'edges': num_edges,
                'bfs_visited': len(bfs_order),
                'dfs_visited': len(dfs_order)
            })
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Graph {i+1}/{num_graphs}: {num_vertices} vertices, {num_edges} edges, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 2: Shortest path algorithms
        print(f"\nPhase 2: Computing shortest paths...")
        
        num_path_tests = random.randint(15, 30)
        
        for i in range(num_path_tests):
            num_vertices = random.randint(100, 500)
            graph = generate_random_graph(num_vertices, edge_probability=0.2, directed=True)
            
            # Run Dijkstra from random sources
            num_sources = random.randint(3, 8)
            
            for _ in range(num_sources):
                source = random.randint(0, num_vertices - 1)
                distances = dijkstra(graph, source)
                stats["algorithms_executed"] += 1
                
                # Find max distance
                finite_distances = [d for d in distances.values() if d != float('infinity')]
                max_dist = max(finite_distances) if finite_distances else 0
                avg_dist = sum(finite_distances) / len(finite_distances) if finite_distances else 0
            
            stats["graphs_processed"] += 1
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Shortest paths {i+1}/{num_path_tests}: {num_vertices} vertices, "
                      f"Max dist: {max_dist:.0f}, Avg: {avg_dist:.2f}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        # Phase 3: Cycle detection
        print(f"\nPhase 3: Detecting cycles...")
        
        num_cycle_tests = random.randint(30, 60)
        cycles_found = 0
        
        for i in range(num_cycle_tests):
            num_vertices = random.randint(50, 200)
            edge_prob = random.uniform(0.2, 0.5)
            
            graph = generate_random_graph(num_vertices, edge_prob, directed=True)
            has_cycle = detect_cycle_directed(graph)
            
            if has_cycle:
                cycles_found += 1
            
            stats["graphs_processed"] += 1
            stats["algorithms_executed"] += 1
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Cycle detection {i+1}/{num_cycle_tests}: "
                      f"Found {cycles_found} graphs with cycles, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 4: Strongly connected components
        print(f"\nPhase 4: Finding strongly connected components...")
        
        num_scc_tests = random.randint(10, 25)
        
        for i in range(num_scc_tests):
            num_vertices = random.randint(100, 400)
            graph = generate_random_graph(num_vertices, edge_probability=0.15, directed=True)
            
            components = find_strongly_connected_components(graph)
            
            stats["graphs_processed"] += 1
            stats["algorithms_executed"] += 1
            
            num_components = len(components)
            largest_component = max(len(c) for c in components) if components else 0
            
            algorithm_results.append({
                'algorithm': 'scc',
                'vertices': num_vertices,
                'num_components': num_components,
                'largest_component': largest_component
            })
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            if (i + 1) % 5 == 0:
                print(f"  SCC {i+1}/{num_scc_tests}: {num_vertices} vertices, "
                      f"{num_components} components, Largest: {largest_component}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 5: Graph metrics
        print(f"\nPhase 5: Computing graph metrics...")
        
        for i in range(random.randint(10, 20)):
            num_vertices = random.randint(200, 600)
            graph = generate_random_graph(num_vertices, edge_probability=0.1)
            
            # Compute various metrics
            degrees = [len(graph.adj[v]) for v in range(graph.V)]
            avg_degree = sum(degrees) / len(degrees) if degrees else 0
            max_degree = max(degrees) if degrees else 0
            
            # Count connected components (for undirected graph)
            visited = set()
            num_components = 0
            
            for v in range(graph.V):
                if v not in visited:
                    bfs_result = bfs(graph, v)
                    visited.update(bfs_result)
                    num_components += 1
            
            stats["graphs_processed"] += 1
            stats["algorithms_executed"] += 1
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Metrics {i+1}: Avg degree: {avg_degree:.2f}, "
                      f"Max degree: {max_degree}, Components: {num_components}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.15))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["avg_vertices_per_graph"] = stats["total_vertices"] / stats["graphs_processed"] if stats["graphs_processed"] > 0 else 0
        stats["avg_edges_per_graph"] = stats["total_edges"] / stats["graphs_processed"] if stats["graphs_processed"] > 0 else 0
        
        # Save results
        with open(f"{results_dir}/graph_results.json", 'w') as f:
            json.dump(algorithm_results[:100], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Graphs processed: {stats['graphs_processed']}")
        print(f"Algorithms executed: {stats['algorithms_executed']}")
        print(f"Total vertices: {stats['total_vertices']:,}")
        print(f"Total edges: {stats['total_edges']:,}")
        print(f"Avg vertices/graph: {stats['avg_vertices_per_graph']:.1f}")
        print(f"Avg edges/graph: {stats['avg_edges_per_graph']:.1f}")
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
