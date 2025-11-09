#!/usr/bin/env python3
"""
GPU Process 4: Recurrent Neural Network with LSTMs
Trains RNN/LSTM networks with variable sequence lengths.
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

class LSTMModel(nn.Module):
    def __init__(self, input_size, hidden_size, num_layers, output_size):
        super(LSTMModel, self).__init__()
        self.hidden_size = hidden_size
        self.num_layers = num_layers
        
        self.lstm = nn.LSTM(input_size, hidden_size, num_layers, batch_first=True)
        self.fc = nn.Linear(hidden_size, output_size)
    
    def forward(self, x):
        h0 = torch.zeros(self.num_layers, x.size(0), self.hidden_size).to(x.device)
        c0 = torch.zeros(self.num_layers, x.size(0), self.hidden_size).to(x.device)
        
        out, _ = self.lstm(x, (h0, c0))
        out = self.fc(out[:, -1, :])
        return out

def main():
    print(f"[{datetime.now()}] Starting GPU-Intensive Process 4: LSTM Training")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    stats = {
        "process_type": "GPU-Intensive-4",
        "pytorch_available": HAS_PYTORCH,
        "sequences_processed": 0,
        "epochs_completed": 0,
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
        
        # Model parameters
        input_size = random.randint(50, 200)
        hidden_size = random.randint(128, 512)
        num_layers = random.randint(2, 4)
        output_size = random.randint(10, 50)
        
        print(f"\nModel configuration:")
        print(f"  Input size: {input_size}")
        print(f"  Hidden size: {hidden_size}")
        print(f"  Num layers: {num_layers}")
        print(f"  Output size: {output_size}")
        
        model = LSTMModel(input_size, hidden_size, num_layers, output_size).to(device)
        criterion = nn.CrossEntropyLoss()
        optimizer = optim.Adam(model.parameters(), lr=0.001)
        
        print(f"  Total parameters: {sum(p.numel() for p in model.parameters()):,}")
        
        # Training with variable sequence lengths
        num_epochs = random.randint(5, 15)
        
        training_history = []
        
        for epoch in range(num_epochs):
            # Variable sequence length per epoch
            seq_length = random.randint(50, 300)
            batch_size = random.randint(32, 128)
            num_batches = random.randint(30, 100)
            
            epoch_loss = 0.0
            epoch_start = time.time()
            
            print(f"\nEpoch {epoch+1}/{num_epochs} - Seq length: {seq_length}, Batch size: {batch_size}")
            
            for batch_idx in range(num_batches):
                # Generate synthetic sequences
                sequences = torch.randn(batch_size, seq_length, input_size).to(device)
                labels = torch.randint(0, output_size, (batch_size,)).to(device)
                
                # Forward pass
                optimizer.zero_grad()
                outputs = model(sequences)
                loss = criterion(outputs, labels)
                
                # Backward pass
                loss.backward()
                optimizer.step()
                
                epoch_loss += loss.item()
                stats["sequences_processed"] += batch_size
                
                if torch.cuda.is_available():
                    gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                    stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (batch_idx + 1) % 15 == 0:
                    avg_loss = epoch_loss / (batch_idx + 1)
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        print(f"  Batch {batch_idx+1}/{num_batches}, Loss: {avg_loss:.4f}, "
                              f"GPU Mem: {gpu_mem:.2f}MB")
                    else:
                        print(f"  Batch {batch_idx+1}/{num_batches}, Loss: {avg_loss:.4f}")
                
                if batch_idx % 10 == 0:
                    time.sleep(random.uniform(0.01, 0.03))
            
            epoch_duration = time.time() - epoch_start
            avg_epoch_loss = epoch_loss / num_batches
            
            training_history.append({
                'epoch': epoch + 1,
                'seq_length': seq_length,
                'batch_size': batch_size,
                'avg_loss': avg_epoch_loss,
                'duration': epoch_duration
            })
            
            stats["epochs_completed"] += 1
            
            print(f"  Epoch completed in {epoch_duration:.2f}s, Avg Loss: {avg_epoch_loss:.4f}")
            
            if torch.cuda.is_available() and epoch % 2 == 0:
                torch.cuda.empty_cache()
        
        # Phase 2: Bi-directional LSTM
        print(f"\nPhase 2: Bidirectional LSTM...")
        
        bidirectional_lstm = nn.LSTM(input_size, hidden_size, num_layers, 
                                     batch_first=True, bidirectional=True).to(device)
        
        for i in range(random.randint(20, 50)):
            batch_size = random.randint(16, 64)
            seq_length = random.randint(50, 200)
            
            sequences = torch.randn(batch_size, seq_length, input_size).to(device)
            
            h0 = torch.zeros(num_layers * 2, batch_size, hidden_size).to(device)
            c0 = torch.zeros(num_layers * 2, batch_size, hidden_size).to(device)
            
            output, (hn, cn) = bidirectional_lstm(sequences, (h0, c0))
            
            stats["sequences_processed"] += batch_size
            
            if torch.cuda.is_available() and (i + 1) % 10 == 0:
                gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_mem)
                print(f"  BiLSTM {i+1}: Seq={seq_length}, GPU Mem: {gpu_mem:.2f}MB")
            
            del sequences, output, hn, cn
            if torch.cuda.is_available() and i % 10 == 0:
                torch.cuda.empty_cache()
            
            time.sleep(random.uniform(0.02, 0.06))
        
        # Phase 3: GRU comparison
        print(f"\nPhase 3: GRU networks...")
        
        gru_model = nn.GRU(input_size, hidden_size, num_layers, batch_first=True).to(device)
        
        for i in range(random.randint(20, 50)):
            batch_size = random.randint(16, 64)
            seq_length = random.randint(50, 200)
            
            sequences = torch.randn(batch_size, seq_length, input_size).to(device)
            h0 = torch.zeros(num_layers, batch_size, hidden_size).to(device)
            
            output, hn = gru_model(sequences, h0)
            
            stats["sequences_processed"] += batch_size
            
            if torch.cuda.is_available() and (i + 1) % 10 == 0:
                gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_mem)
                print(f"  GRU {i+1}: Seq={seq_length}, GPU Mem: {gpu_mem:.2f}MB")
            
            del sequences, output, hn
            
            time.sleep(random.uniform(0.02, 0.06))
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save training history
        with open(f"{results_dir}/training_history.json", 'w') as f:
            json.dump(training_history, f, indent=2)
        
        # Save model
        torch.save(model.state_dict(), f"{results_dir}/lstm_model.pth")
        
        # Save stats
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Device: {device}")
        print(f"Sequences processed: {stats['sequences_processed']:,}")
        print(f"Epochs completed: {stats['epochs_completed']}")
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
