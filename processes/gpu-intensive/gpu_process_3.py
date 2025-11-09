#!/usr/bin/env python3
"""
GPU Process 3: Image Processing and Convolutions
Performs GPU-accelerated image processing operations.
"""
import os
import time
import random
import json
from datetime import datetime

try:
    import torch
    import torch.nn.functional as F
    HAS_PYTORCH = True
except ImportError:
    HAS_PYTORCH = False

def generate_synthetic_images(batch_size, channels, height, width, device):
    """Generate synthetic images"""
    return torch.randn(batch_size, channels, height, width, device=device)

def apply_convolutions(images, num_filters, kernel_sizes, device):
    """Apply multiple convolution operations"""
    results = []
    
    for kernel_size in kernel_sizes:
        # Create random convolution kernel
        kernel = torch.randn(num_filters, images.shape[1], kernel_size, kernel_size, device=device)
        
        # Apply convolution
        output = F.conv2d(images, kernel, padding=kernel_size//2)
        results.append(output)
    
    return results

def main():
    print(f"[{datetime.now()}] Starting GPU-Intensive Process 3: Image Processing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    stats = {
        "process_type": "GPU-Intensive-3",
        "pytorch_available": HAS_PYTORCH,
        "images_processed": 0,
        "convolutions_applied": 0,
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
        
        # Phase 1: Multi-scale convolutions
        print(f"\nPhase 1: Multi-scale convolutions...")
        
        num_batches = random.randint(30, 80)
        
        for i in range(num_batches):
            batch_size = random.randint(16, 64)
            image_size = random.choice([64, 128, 256, 512])
            channels = random.choice([3, 32, 64])
            
            # Generate images
            images = generate_synthetic_images(batch_size, channels, image_size, image_size, device)
            
            # Apply various convolutions
            kernel_sizes = [3, 5, 7]
            num_filters = random.randint(32, 128)
            
            results = apply_convolutions(images, num_filters, kernel_sizes, device)
            
            stats["images_processed"] += batch_size
            stats["convolutions_applied"] += len(kernel_sizes)
            
            if device.type == 'cuda':
                torch.cuda.synchronize()
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 10 == 0:
                    print(f"  Batch {i+1}/{num_batches}: {batch_size}x{channels}x{image_size}x{image_size}, "
                          f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del images, results
            if torch.cuda.is_available() and i % 10 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 2: Pooling operations
        print(f"\nPhase 2: Pooling operations...")
        
        for i in range(random.randint(40, 100)):
            batch_size = random.randint(32, 96)
            channels = random.randint(64, 256)
            size = random.choice([32, 64, 128])
            
            images = generate_synthetic_images(batch_size, channels, size, size, device)
            
            # Max pooling
            max_pooled = F.max_pool2d(images, kernel_size=2, stride=2)
            
            # Average pooling
            avg_pooled = F.avg_pool2d(images, kernel_size=2, stride=2)
            
            # Adaptive pooling
            adaptive_pooled = F.adaptive_avg_pool2d(images, (7, 7))
            
            stats["images_processed"] += batch_size
            
            if torch.cuda.is_available() and (i + 1) % 15 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Pooling {i+1}: GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del images, max_pooled, avg_pooled, adaptive_pooled
        
        # Phase 3: Batch normalization and activations
        print(f"\nPhase 3: Normalization and activations...")
        
        for i in range(random.randint(50, 120)):
            batch_size = random.randint(32, 128)
            channels = random.randint(64, 256)
            size = random.randint(32, 128)
            
            images = generate_synthetic_images(batch_size, channels, size, size, device)
            
            # Batch normalization
            normalized = F.batch_norm(images, 
                                     torch.zeros(channels, device=device),
                                     torch.ones(channels, device=device))
            
            # Various activations
            relu_out = F.relu(normalized)
            sigmoid_out = torch.sigmoid(normalized)
            tanh_out = torch.tanh(normalized)
            leaky_relu_out = F.leaky_relu(normalized, 0.2)
            
            stats["images_processed"] += batch_size
            
            if torch.cuda.is_available() and (i + 1) % 20 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Normalization {i+1}: GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del images, normalized, relu_out, sigmoid_out, tanh_out, leaky_relu_out
        
        # Phase 4: Transposed convolutions (upsampling)
        print(f"\nPhase 4: Transposed convolutions...")
        
        for i in range(random.randint(20, 50)):
            batch_size = random.randint(16, 48)
            in_channels = random.randint(64, 256)
            out_channels = random.randint(32, 128)
            size = random.randint(16, 64)
            
            images = generate_synthetic_images(batch_size, in_channels, size, size, device)
            
            # Transposed convolution (upsampling)
            kernel = torch.randn(in_channels, out_channels, 4, 4, device=device)
            upsampled = F.conv_transpose2d(images, kernel, stride=2, padding=1)
            
            stats["images_processed"] += batch_size
            stats["convolutions_applied"] += 1
            
            if torch.cuda.is_available() and (i + 1) % 5 == 0:
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                print(f"  Transposed conv {i+1}: {size}x{size} -> {upsampled.shape[2]}x{upsampled.shape[3]}, "
                      f"GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del images, kernel, upsampled
            if torch.cuda.is_available() and i % 10 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.02, 0.07))
        
        # Phase 5: Complex filter compositions
        print(f"\nPhase 5: Complex filter pipelines...")
        
        for i in range(random.randint(15, 40)):
            batch_size = random.randint(8, 32)
            size = random.choice([128, 256])
            
            images = generate_synthetic_images(batch_size, 3, size, size, device)
            
            # Multi-stage processing pipeline
            # Stage 1: Initial convolution
            conv1 = F.conv2d(images, 
                           torch.randn(64, 3, 3, 3, device=device), 
                           padding=1)
            conv1 = F.relu(conv1)
            pool1 = F.max_pool2d(conv1, 2)
            
            # Stage 2: Deeper features
            conv2 = F.conv2d(pool1,
                           torch.randn(128, 64, 3, 3, device=device),
                           padding=1)
            conv2 = F.relu(conv2)
            pool2 = F.max_pool2d(conv2, 2)
            
            # Stage 3: Even deeper
            conv3 = F.conv2d(pool2,
                           torch.randn(256, 128, 3, 3, device=device),
                           padding=1)
            conv3 = F.relu(conv3)
            
            stats["images_processed"] += batch_size
            stats["convolutions_applied"] += 3
            
            if torch.cuda.is_available():
                torch.cuda.synchronize()
                gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (i + 1) % 5 == 0:
                    print(f"  Pipeline {i+1}: GPU Mem: {gpu_memory_mb:.2f}MB")
            
            del images, conv1, pool1, conv2, pool2, conv3
            if torch.cuda.is_available() and i % 5 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.03, 0.1))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save stats
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Device: {device}")
        print(f"Images processed: {stats['images_processed']:,}")
        print(f"Convolutions applied: {stats['convolutions_applied']}")
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
