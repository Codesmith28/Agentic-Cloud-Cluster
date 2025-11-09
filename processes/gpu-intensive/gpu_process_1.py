#!/usr/bin/env python3
"""
GPU Process 1: Neural Network Training with Dynamic Batch Sizes
Trains a CNN on synthetic data with varying GPU memory usage.
"""
import os
import time
import random
import json
from datetime import datetime

try:
    import torch
    import torch.nn as nn
    import torch.optim as optim
    HAS_PYTORCH = True
except ImportError:
    HAS_PYTORCH = False
    print("PyTorch not available, will use fallback mode")

class SimpleCNN(nn.Module):
    def __init__(self, num_classes=10):
        super(SimpleCNN, self).__init__()
        self.conv1 = nn.Conv2d(3, 32, kernel_size=3, padding=1)
        self.conv2 = nn.Conv2d(32, 64, kernel_size=3, padding=1)
        self.conv3 = nn.Conv2d(64, 128, kernel_size=3, padding=1)
        self.pool = nn.MaxPool2d(2, 2)
        self.fc1 = nn.Linear(128 * 4 * 4, 512)
        self.fc2 = nn.Linear(512, num_classes)
        self.relu = nn.ReLU()
        self.dropout = nn.Dropout(0.5)
    
    def forward(self, x):
        x = self.pool(self.relu(self.conv1(x)))
        x = self.pool(self.relu(self.conv2(x)))
        x = self.pool(self.relu(self.conv3(x)))
        x = x.view(-1, 128 * 4 * 4)
        x = self.dropout(self.relu(self.fc1(x)))
        x = self.fc2(x)
        return x

def main():
    print(f"[{datetime.now()}] Starting GPU-Intensive Process 1: Neural Network Training")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    stats = {
        "process_type": "GPU-Intensive-1",
        "pytorch_available": HAS_PYTORCH,
        "epochs_completed": 0,
        "batches_processed": 0,
        "peak_gpu_memory_mb": 0,
        "duration_seconds": 0
    }
    
    if not HAS_PYTORCH:
        stats["error"] = "PyTorch not available"
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        print("PyTorch not available. Please install: pip install torch torchvision")
        return
    
    start_time = time.time()
    
    try:
        # Check GPU availability
        device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        stats["device"] = str(device)
        
        print(f"Using device: {device}")
        
        if torch.cuda.is_available():
            print(f"GPU: {torch.cuda.get_device_name(0)}")
            print(f"GPU Memory: {torch.cuda.get_device_properties(0).total_memory / 1024**3:.2f} GB")
        
        # Create model
        model = SimpleCNN(num_classes=10).to(device)
        criterion = nn.CrossEntropyLoss()
        optimizer = optim.Adam(model.parameters(), lr=0.001)
        
        print(f"\nModel parameters: {sum(p.numel() for p in model.parameters()):,}")
        
        # Training with dynamic batch sizes
        num_epochs = random.randint(5, 15)
        base_batch_size = random.randint(16, 64)
        image_size = 32
        
        training_history = []
        
        for epoch in range(num_epochs):
            # Vary batch size across epochs
            batch_size = base_batch_size + (epoch * random.randint(4, 16))
            num_batches = random.randint(50, 200)
            
            epoch_loss = 0.0
            epoch_start = time.time()
            
            print(f"\nEpoch {epoch+1}/{num_epochs} - Batch size: {batch_size}")
            
            for batch_idx in range(num_batches):
                # Generate synthetic data
                images = torch.randn(batch_size, 3, image_size, image_size).to(device)
                labels = torch.randint(0, 10, (batch_size,)).to(device)
                
                # Forward pass
                optimizer.zero_grad()
                outputs = model(images)
                loss = criterion(outputs, labels)
                
                # Backward pass
                loss.backward()
                optimizer.step()
                
                epoch_loss += loss.item()
                stats["batches_processed"] += 1
                
                # Track GPU memory
                if torch.cuda.is_available():
                    gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                    stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (batch_idx + 1) % 25 == 0:
                    avg_loss = epoch_loss / (batch_idx + 1)
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        gpu_cached = torch.cuda.memory_reserved() / (1024 * 1024)
                        print(f"  Batch {batch_idx+1}/{num_batches}, Loss: {avg_loss:.4f}, "
                              f"GPU Mem: {gpu_mem:.2f}MB (Cached: {gpu_cached:.2f}MB)")
                    else:
                        print(f"  Batch {batch_idx+1}/{num_batches}, Loss: {avg_loss:.4f}")
                
                # Simulate variable processing time
                if batch_idx % 10 == 0:
                    time.sleep(random.uniform(0.01, 0.05))
            
            epoch_duration = time.time() - epoch_start
            avg_epoch_loss = epoch_loss / num_batches
            
            training_history.append({
                'epoch': epoch + 1,
                'batch_size': batch_size,
                'num_batches': num_batches,
                'avg_loss': avg_epoch_loss,
                'duration': epoch_duration
            })
            
            stats["epochs_completed"] += 1
            
            print(f"  Epoch completed in {epoch_duration:.2f}s, Avg Loss: {avg_epoch_loss:.4f}")
            
            # Clear cache periodically
            if torch.cuda.is_available() and epoch % 3 == 0:
                torch.cuda.empty_cache()
        
        # Inference test
        print(f"\nRunning inference test...")
        model.eval()
        
        with torch.no_grad():
            test_batches = random.randint(10, 50)
            inference_batch_size = random.randint(32, 128)
            
            for i in range(test_batches):
                test_images = torch.randn(inference_batch_size, 3, image_size, image_size).to(device)
                outputs = model(test_images)
                predictions = torch.argmax(outputs, dim=1)
                
                if (i + 1) % 10 == 0:
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        print(f"  Inference batch {i+1}/{test_batches}, GPU Mem: {gpu_mem:.2f}MB")
                    else:
                        print(f"  Inference batch {i+1}/{test_batches}")
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save training history
        with open(f"{results_dir}/training_history.json", 'w') as f:
            json.dump(training_history, f, indent=2)
        
        # Save model checkpoint
        torch.save(model.state_dict(), f"{results_dir}/model_checkpoint.pth")
        
        # Save stats
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Device: {device}")
        print(f"Epochs completed: {stats['epochs_completed']}")
        print(f"Batches processed: {stats['batches_processed']}")
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
