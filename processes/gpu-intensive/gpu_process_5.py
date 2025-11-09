#!/usr/bin/env python3
"""
GPU Process 5: Transformer Models and Attention Mechanisms
Tests GPU with transformer architectures and attention computations.
"""
import os
import time
import random
import json
from datetime import datetime

try:
    import torch
    import torch.nn as nn
    import torch.nn.functional as F
    HAS_PYTORCH = True
except ImportError:
    HAS_PYTORCH = False

class SimpleTransformer(nn.Module):
    def __init__(self, d_model, nhead, num_layers, dim_feedforward):
        super(SimpleTransformer, self).__init__()
        encoder_layer = nn.TransformerEncoderLayer(d_model, nhead, dim_feedforward, batch_first=True)
        self.transformer = nn.TransformerEncoder(encoder_layer, num_layers)
        self.fc = nn.Linear(d_model, d_model)
    
    def forward(self, src):
        output = self.transformer(src)
        output = self.fc(output)
        return output

def main():
    print(f"[{datetime.now()}] Starting GPU-Intensive Process 5: Transformer Models")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    stats = {
        "process_type": "GPU-Intensive-5",
        "pytorch_available": HAS_PYTORCH,
        "sequences_processed": 0,
        "attention_operations": 0,
        "peak_gpu_memory_mb": 0,
        "duration_seconds": 0
    }
    
    if not HAS_PYTORCH:
        stats["error"] = "PyTorch not available"
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        print("PyTorch not available")
        return
    
    start_time = time.time()
    
    try:
        device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        stats["device"] = str(device)
        
        print(f"Using device: {device}")
        
        # Phase 1: Transformer encoder training
        print(f"\nPhase 1: Transformer encoder...")
        
        d_model = random.choice([256, 512, 768])
        nhead = random.choice([4, 8, 16])
        num_layers = random.randint(3, 8)
        dim_feedforward = d_model * 4
        
        print(f"  Model dim: {d_model}, Heads: {nhead}, Layers: {num_layers}")
        
        model = SimpleTransformer(d_model, nhead, num_layers, dim_feedforward).to(device)
        optimizer = torch.optim.Adam(model.parameters(), lr=0.0001)
        
        print(f"  Parameters: {sum(p.numel() for p in model.parameters()):,}")
        
        num_iterations = random.randint(50, 150)
        
        for i in range(num_iterations):
            batch_size = random.randint(16, 64)
            seq_length = random.randint(32, 256)
            
            # Generate input sequences
            src = torch.randn(batch_size, seq_length, d_model).to(device)
            
            # Forward pass
            optimizer.zero_grad()
            output = model(src)
            
            # Simple loss (mean of outputs)
            loss = output.mean()
            loss.backward()
            optimizer.step()
            
            stats["sequences_processed"] += batch_size
            stats["attention_operations"] += num_layers * nhead
            
            if torch.cuda.is_available():
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 15 == 0:
                    print(f"  Iter {i+1}/{num_iterations}: Batch={batch_size}, Seq={seq_length}, "
                          f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del src, output
            if torch.cuda.is_available() and i % 20 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.01, 0.04))
        
        # Phase 2: Multi-head attention operations
        print(f"\nPhase 2: Multi-head attention...")
        
        for i in range(random.randint(30, 80)):
            batch_size = random.randint(16, 64)
            seq_length = random.randint(64, 512)
            embed_dim = random.choice([256, 512, 768])
            num_heads = random.choice([4, 8, 16])
            
            # Create multi-head attention module
            mha = nn.MultiheadAttention(embed_dim, num_heads, batch_first=True).to(device)
            
            # Input tensors
            query = torch.randn(batch_size, seq_length, embed_dim).to(device)
            key = torch.randn(batch_size, seq_length, embed_dim).to(device)
            value = torch.randn(batch_size, seq_length, embed_dim).to(device)
            
            # Attention computation
            attn_output, attn_weights = mha(query, key, value)
            
            stats["sequences_processed"] += batch_size
            stats["attention_operations"] += num_heads
            
            if torch.cuda.is_available() and (i + 1) % 10 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Attention {i+1}: Heads={num_heads}, Seq={seq_length}, "
                      f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del mha, query, key, value, attn_output, attn_weights
            if torch.cuda.is_available() and i % 15 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.02, 0.06))
        
        # Phase 3: Cross-attention (encoder-decoder style)
        print(f"\nPhase 3: Cross-attention...")
        
        for i in range(random.randint(20, 50)):
            batch_size = random.randint(16, 48)
            src_seq_len = random.randint(64, 256)
            tgt_seq_len = random.randint(32, 128)
            embed_dim = random.choice([256, 512])
            num_heads = random.choice([4, 8])
            
            mha = nn.MultiheadAttention(embed_dim, num_heads, batch_first=True).to(device)
            
            # Source and target sequences
            query = torch.randn(batch_size, tgt_seq_len, embed_dim).to(device)
            key = torch.randn(batch_size, src_seq_len, embed_dim).to(device)
            value = torch.randn(batch_size, src_seq_len, embed_dim).to(device)
            
            attn_output, attn_weights = mha(query, key, value)
            
            stats["sequences_processed"] += batch_size
            stats["attention_operations"] += num_heads
            
            if torch.cuda.is_available() and (i + 1) % 5 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Cross-attn {i+1}: Src={src_seq_len}, Tgt={tgt_seq_len}, "
                      f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del mha, query, key, value, attn_output, attn_weights
            
            time.sleep(random.uniform(0.03, 0.08))
        
        # Phase 4: Positional encoding and embedding operations
        print(f"\nPhase 4: Embeddings and positional encoding...")
        
        vocab_size = random.randint(10000, 50000)
        embedding_dim = random.choice([256, 512, 768])
        
        embedding_layer = nn.Embedding(vocab_size, embedding_dim).to(device)
        
        for i in range(random.randint(40, 100)):
            batch_size = random.randint(32, 128)
            seq_length = random.randint(32, 512)
            
            # Token indices
            tokens = torch.randint(0, vocab_size, (batch_size, seq_length)).to(device)
            
            # Embedding lookup
            embeddings = embedding_layer(tokens)
            
            # Add positional encoding
            position = torch.arange(0, seq_length).unsqueeze(0).repeat(batch_size, 1).to(device)
            pos_encodings = torch.sin(position.unsqueeze(-1) / 10000 ** (torch.arange(0, embedding_dim).to(device) / embedding_dim))
            
            # Combine
            embedded = embeddings + pos_encodings
            
            stats["sequences_processed"] += batch_size
            
            if torch.cuda.is_available() and (i + 1) % 15 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Embedding {i+1}: Vocab={vocab_size}, Seq={seq_length}, "
                      f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del tokens, embeddings, position, pos_encodings, embedded
        
        # Phase 5: Scaled dot-product attention (manual implementation)
        print(f"\nPhase 5: Scaled dot-product attention...")
        
        for i in range(random.randint(30, 70)):
            batch_size = random.randint(16, 64)
            num_heads = random.choice([4, 8, 16])
            seq_length = random.randint(64, 256)
            head_dim = random.choice([32, 64])
            
            # Q, K, V for all heads
            Q = torch.randn(batch_size, num_heads, seq_length, head_dim).to(device)
            K = torch.randn(batch_size, num_heads, seq_length, head_dim).to(device)
            V = torch.randn(batch_size, num_heads, seq_length, head_dim).to(device)
            
            # Attention scores
            scores = torch.matmul(Q, K.transpose(-2, -1)) / (head_dim ** 0.5)
            attn_weights = F.softmax(scores, dim=-1)
            
            # Apply attention to values
            output = torch.matmul(attn_weights, V)
            
            stats["attention_operations"] += num_heads
            
            if torch.cuda.is_available() and (i + 1) % 10 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Scaled attention {i+1}: Heads={num_heads}, Seq={seq_length}, "
                      f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del Q, K, V, scores, attn_weights, output
            if torch.cuda.is_available() and i % 15 == 0:
                torch.cuda.empty_cache()
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save model
        torch.save(model.state_dict(), f"{results_dir}/transformer_model.pth")
        
        # Save stats
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Device: {device}")
        print(f"Sequences processed: {stats['sequences_processed']:,}")
        print(f"Attention operations: {stats['attention_operations']:,}")
        if torch.cuda.is_available():
            print(f"Peak GPU memory: {stats['peak_gpu_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
