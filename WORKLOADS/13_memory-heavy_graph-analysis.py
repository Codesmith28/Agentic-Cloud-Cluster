#!/usr/bin/env python3
"""
Workload Type: memory-heavy
Description: Build and query large in-memory graph structure
Resource Requirements: Moderate CPU, high memory
Output: None (logs only)
"""

import datetime
import random

class Graph:
    def __init__(self):
        self.nodes = {}
        self.edges = []
    
    def add_node(self, node_id, data):
        self.nodes[node_id] = data
    
    def add_edge(self, from_node, to_node, weight):
        self.edges.append((from_node, to_node, weight))
    
    def get_neighbors(self, node_id):
        return [(to, w) for fr, to, w in self.edges if fr == node_id]

def build_graph(num_nodes=100_000, num_edges=500_000):
    """Build a large graph in memory"""
    print(f"  Building graph with {num_nodes:,} nodes and {num_edges:,} edges...")
    
    graph = Graph()
    
    # Add nodes
    for i in range(num_nodes):
        graph.add_node(i, {
            'name': f'Node{i}',
            'value': random.randint(1, 1000)
        })
        
        if (i + 1) % 20000 == 0:
            print(f"    Added {i+1:,} nodes...")
    
    # Add edges
    for i in range(num_edges):
        from_node = random.randint(0, num_nodes - 1)
        to_node = random.randint(0, num_nodes - 1)
        weight = random.uniform(1, 100)
        
        graph.add_edge(from_node, to_node, weight)
        
        if (i + 1) % 100000 == 0:
            print(f"    Added {i+1:,} edges...")
    
    return graph

def analyze_graph():
    print(f"[{datetime.datetime.now()}] Starting Graph Analysis...")
    
    # Build graph
    graph = build_graph()
    
    print(f"\n✓ Graph built: {len(graph.nodes):,} nodes, {len(graph.edges):,} edges")
    
    # Analyze connectivity
    print("\nAnalyzing connectivity...")
    sample_nodes = random.sample(range(len(graph.nodes)), 100)
    
    total_neighbors = 0
    max_neighbors = 0
    max_node = None
    
    for node_id in sample_nodes:
        neighbors = graph.get_neighbors(node_id)
        count = len(neighbors)
        total_neighbors += count
        
        if count > max_neighbors:
            max_neighbors = count
            max_node = node_id
    
    avg_neighbors = total_neighbors / len(sample_nodes)
    
    print(f"  Average neighbors (sample): {avg_neighbors:.2f}")
    print(f"  Max neighbors: {max_neighbors} (Node {max_node})")
    
    # Calculate average edge weight
    avg_weight = sum(w for _, _, w in graph.edges) / len(graph.edges)
    print(f"  Average edge weight: {avg_weight:.2f}")
    
    # Find high-value nodes
    print("\nTop 10 highest value nodes:")
    sorted_nodes = sorted(graph.nodes.items(), key=lambda x: x[1]['value'], reverse=True)[:10]
    for node_id, data in sorted_nodes:
        print(f"  Node {node_id}: value = {data['value']}")
    
    print(f"\n[{datetime.datetime.now()}] Graph Analysis completed ✓")
    return 0

if __name__ == "__main__":
    exit(analyze_graph())
