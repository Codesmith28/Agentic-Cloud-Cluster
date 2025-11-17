#!/usr/bin/env python3
"""
Workload Type: gpu-inference
Description: Simulated text embedding generation (LLM inference)
Resource Requirements: Low CPU, moderate memory, GPU required
Output: embeddings.csv (generated file)
"""

import datetime
import random
import csv
import time

def simulate_embedding(text, dimension=768):
    """Simulate generating an embedding vector"""
    # In real scenario, this would use GPU for transformer model
    time.sleep(0.01)  # Simulate GPU computation
    return [random.uniform(-1, 1) for _ in range(dimension)]

def batch_embed_texts(texts, batch_size=32):
    """Process texts in batches"""
    embeddings = []
    
    for i in range(0, len(texts), batch_size):
        batch = texts[i:i+batch_size]
        batch_embeddings = [simulate_embedding(text) for text in batch]
        embeddings.extend(batch_embeddings)
        
        if (i + batch_size) % 320 == 0:
            print(f"  Processed {min(i+batch_size, len(texts))}/{len(texts)} texts...")
    
    return embeddings

def run_text_embedding():
    print(f"[{datetime.datetime.now()}] Starting Text Embedding (GPU Inference)...")
    print("🖥️  GPU Inference Mode")
    
    # Generate sample texts
    texts = [
        f"Sample text number {i} for embedding generation. This simulates LLM inference workload."
        for i in range(1000)
    ]
    
    print(f"\n✓ Generated {len(texts)} text samples")
    print(f"  Embedding dimension: 768")
    print(f"  Batch size: 32")
    
    # Process embeddings
    print("\nGenerating embeddings (simulated GPU)...")
    start = datetime.datetime.now()
    embeddings = batch_embed_texts(texts)
    end = datetime.datetime.now()
    
    duration = (end - start).total_seconds()
    throughput = len(texts) / duration
    
    print(f"\n✓ Generated {len(embeddings)} embeddings")
    print(f"✓ Time: {duration:.2f}s")
    print(f"✓ Throughput: {throughput:.2f} texts/sec")
    
    # Compute some statistics
    avg_norm = sum(sum(abs(v) for v in emb) for emb in embeddings) / len(embeddings)
    print(f"✓ Average L1 norm: {avg_norm:.4f}")
    
    # Save sample embeddings
    with open('/output/embeddings.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['text_id', 'embedding_sample', 'dimension'])
        for i, emb in enumerate(embeddings[:10]):  # Save first 10
            sample = f"[{emb[0]:.4f}, {emb[1]:.4f}, ..., {emb[-1]:.4f}]"
            writer.writerow([i, sample, len(emb)])
    
    print(f"✓ Generated embeddings.csv (sample)")
    print(f"\n[{datetime.datetime.now()}] Text Embedding completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_text_embedding())
