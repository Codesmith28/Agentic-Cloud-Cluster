#!/usr/bin/env python3
"""
GPU Process 2: Large Matrix Operations on GPU
Performs intensive GPU matrix computations with dynamic sizes.
"""
import os
import time
import random
import json
from datetime import datetime

try:
    import torch
    HAS_PYTORCH = True
except ImportError:
    HAS_PYTORCH = False
    print("PyTorch not available, will use fallback mode")

def main():
    print(f"[{datetime.now()}] Starting GPU-Intensive Process 2: Matrix Operations")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    stats = {
        "process_type": "GPU-Intensive-2",
        "pytorch_available": HAS_PYTORCH,
        "operations_completed": 0,
        "peak_gpu_memory_mb": 0,
        "duration_seconds": 0
    }
    
    if not HAS_PYTORCH:
        stats["error"] = "PyTorch not available"
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        print("PyTorch not available. Please install: pip install torch")
        return
    
    start_time = time.time()
    operation_results = []
    
    try:
        device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        stats["device"] = str(device)
        
        print(f"Using device: {device}")
        
        if torch.cuda.is_available():
            print(f"GPU: {torch.cuda.get_device_name(0)}")
        
        # Phase 1: Large matrix multiplications
        print(f"\nPhase 1: Matrix multiplications...")
        
        num_ops = random.randint(20, 50)
        base_size = random.randint(1000, 2000)
        
        for i in range(num_ops):
            size = base_size + (i * random.randint(50, 200))
            
            # Create large matrices
            A = torch.randn(size, size, device=device)
            B = torch.randn(size, size, device=device)
            
            # Matrix multiplication
            op_start = time.time()
            C = torch.matmul(A, B)
            
            # Ensure operation completes
            if device.type == 'cuda':
                torch.cuda.synchronize()
            
            op_duration = time.time() - op_start
            
            stats["operations_completed"] += 1
            
            # Track GPU memory
            if torch.cuda.is_available():
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 5 == 0:
                    print(f"  MatMul {i+1}/{num_ops}: {size}x{size}, "
                          f"Time: {op_duration:.3f}s, GPU Mem: {gpu_memory_mb:.2f}MB")
            else:
                if (i + 1) % 5 == 0:
                    print(f"  MatMul {i+1}/{num_ops}: {size}x{size}, Time: {op_duration:.3f}s")
            
            operation_results.append({
                'operation': 'matmul',
                'size': size,
                'duration': op_duration
            })
            
            # Clear some memory
            del A, B, C
            if torch.cuda.is_available() and i % 10 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 2: Eigenvalue decomposition
        print(f"\nPhase 2: Eigenvalue decomposition...")
        
        num_eigen = random.randint(10, 30)
        
        for i in range(num_eigen):
            size = random.randint(500, 1500)
            
            # Create symmetric matrix for stable eigenvalue computation
            A = torch.randn(size, size, device=device)
            A = A + A.T  # Make symmetric
            
            op_start = time.time()
            eigenvalues = torch.linalg.eigvalsh(A)
            
            if device.type == 'cuda':
                torch.cuda.synchronize()
            
            op_duration = time.time() - op_start
            
            stats["operations_completed"] += 1
            
            if torch.cuda.is_available():
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 3 == 0:
                    print(f"  Eigen {i+1}/{num_eigen}: {size}x{size}, "
                          f"Time: {op_duration:.3f}s, GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del A, eigenvalues
            if torch.cuda.is_available() and i % 5 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.02, 0.08))
        
        # Phase 3: SVD decomposition
        print(f"\nPhase 3: Singular Value Decomposition...")
        
        num_svd = random.randint(10, 25)
        
        for i in range(num_svd):
            rows = random.randint(800, 1200)
            cols = random.randint(800, 1200)
            
            A = torch.randn(rows, cols, device=device)
            
            op_start = time.time()
            U, S, Vh = torch.linalg.svd(A, full_matrices=False)
            
            if device.type == 'cuda':
                torch.cuda.synchronize()
            
            op_duration = time.time() - op_start
            
            stats["operations_completed"] += 1
            
            if torch.cuda.is_available():
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 3 == 0:
                    print(f"  SVD {i+1}/{num_svd}: {rows}x{cols}, "
                          f"Time: {op_duration:.3f}s, GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del A, U, S, Vh
            if torch.cuda.is_available() and i % 5 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.03, 0.1))
        
        # Phase 4: Batch operations
        print(f"\nPhase 4: Batch matrix operations...")
        
        num_batches = random.randint(15, 40)
        
        for i in range(num_batches):
            batch_size = random.randint(32, 128)
            matrix_size = random.randint(256, 512)
            
            # Batch of matrices
            A = torch.randn(batch_size, matrix_size, matrix_size, device=device)
            B = torch.randn(batch_size, matrix_size, matrix_size, device=device)
            
            op_start = time.time()
            
            # Batch matrix multiplication
            C = torch.bmm(A, B)
            
            # Batch determinant
            dets = torch.linalg.det(A)
            
            # Batch inverse (for smaller matrices)
            if matrix_size <= 256:
                A_inv = torch.linalg.inv(A)
            
            if device.type == 'cuda':
                torch.cuda.synchronize()
            
            op_duration = time.time() - op_start
            
            stats["operations_completed"] += 1
            
            if torch.cuda.is_available():
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 5 == 0:
                    print(f"  Batch {i+1}/{num_batches}: {batch_size}x{matrix_size}x{matrix_size}, "
                          f"Time: {op_duration:.3f}s, GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del A, B, C, dets
            if torch.cuda.is_available() and i % 5 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.02, 0.06))
        
        # Phase 5: Element-wise operations on large tensors
        print(f"\nPhase 5: Element-wise operations...")
        
        for i in range(random.randint(30, 60)):
            shape = (random.randint(1000, 3000), random.randint(1000, 3000))
            
            A = torch.randn(shape, device=device)
            B = torch.randn(shape, device=device)
            
            # Various element-wise operations
            C = torch.sin(A) + torch.cos(B)
            D = torch.exp(-torch.abs(A))
            E = torch.log(torch.abs(A) + 1)
            F = torch.tanh(A * B)
            
            stats["operations_completed"] += 1
            
            if torch.cuda.is_available() and (i + 1) % 10 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Element-wise {i+1}: {shape}, GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del A, B, C, D, E, F
            if torch.cuda.is_available() and i % 10 == 0:
                torch.cuda.empty_cache()
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save operation results (sample)
        with open(f"{results_dir}/operation_results.json", 'w') as f:
            json.dump(operation_results[:50], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Device: {device}")
        print(f"Operations completed: {stats['operations_completed']}")
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
