#!/usr/bin/env python3
"""
GPU Process 6: Generative Models and Variational Autoencoders
Tests GPU with generative model training.
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

class VAE(nn.Module):
    def __init__(self, input_dim, hidden_dim, latent_dim):
        super(VAE, self).__init__()
        
        # Encoder
        self.fc1 = nn.Linear(input_dim, hidden_dim)
        self.fc21 = nn.Linear(hidden_dim, latent_dim)  # mu
        self.fc22 = nn.Linear(hidden_dim, latent_dim)  # logvar
        
        # Decoder
        self.fc3 = nn.Linear(latent_dim, hidden_dim)
        self.fc4 = nn.Linear(hidden_dim, input_dim)
    
    def encode(self, x):
        h1 = F.relu(self.fc1(x))
        return self.fc21(h1), self.fc22(h1)
    
    def reparameterize(self, mu, logvar):
        std = torch.exp(0.5 * logvar)
        eps = torch.randn_like(std)
        return mu + eps * std
    
    def decode(self, z):
        h3 = F.relu(self.fc3(z))
        return torch.sigmoid(self.fc4(h3))
    
    def forward(self, x):
        mu, logvar = self.encode(x.view(-1, self.fc1.in_features))
        z = self.reparameterize(mu, logvar)
        return self.decode(z), mu, logvar

def vae_loss(recon_x, x, mu, logvar):
    BCE = F.binary_cross_entropy(recon_x, x.view(-1, recon_x.size(1)), reduction='sum')
    KLD = -0.5 * torch.sum(1 + logvar - mu.pow(2) - logvar.exp())
    return BCE + KLD

def main():
    print(f"[{datetime.now()}] Starting GPU-Intensive Process 6: Generative Models")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    stats = {
        "process_type": "GPU-Intensive-6",
        "pytorch_available": HAS_PYTORCH,
        "samples_generated": 0,
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
        
        # Phase 1: VAE training
        print(f"\nPhase 1: Variational Autoencoder training...")
        
        input_dim = random.choice([784, 1024, 2048])
        hidden_dim = random.choice([256, 512, 1024])
        latent_dim = random.choice([32, 64, 128])
        
        print(f"  Input: {input_dim}, Hidden: {hidden_dim}, Latent: {latent_dim}")
        
        vae = VAE(input_dim, hidden_dim, latent_dim).to(device)
        optimizer = torch.optim.Adam(vae.parameters(), lr=0.001)
        
        print(f"  Parameters: {sum(p.numel() for p in vae.parameters()):,}")
        
        num_epochs = random.randint(5, 15)
        training_history = []
        
        for epoch in range(num_epochs):
            batch_size = random.randint(64, 256)
            num_batches = random.randint(30, 100)
            
            epoch_loss = 0.0
            epoch_start = time.time()
            
            print(f"\nEpoch {epoch+1}/{num_epochs} - Batch size: {batch_size}")
            
            for batch_idx in range(num_batches):
                # Generate random data
                data = torch.rand(batch_size, input_dim).to(device)
                
                # Forward pass
                optimizer.zero_grad()
                recon_batch, mu, logvar = vae(data)
                loss = vae_loss(recon_batch, data, mu, logvar)
                
                # Backward pass
                loss.backward()
                optimizer.step()
                
                epoch_loss += loss.item()
                
                if torch.cuda.is_available():
                    gpu_memory_mb = torch.cuda.memory_allocated() / (1024 * 1024)
                    stats["peak_gpu_memory_mb"] = max(stats["peak_gpu_memory_mb"], gpu_memory_mb)
                
                if (batch_idx + 1) % 15 == 0:
                    avg_loss = epoch_loss / (batch_idx + 1)
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        print(f"  Batch {batch_idx+1}/{num_batches}, Loss: {avg_loss:.2f}, "
                              f"GPU Mem: {gpu_mem:.2f}MB")
                    else:
                        print(f"  Batch {batch_idx+1}/{num_batches}, Loss: {avg_loss:.2f}")
                
                del data, recon_batch, mu, logvar
                
                if batch_idx % 10 == 0:
                    time.sleep(random.uniform(0.01, 0.03))
            
            epoch_duration = time.time() - epoch_start
            avg_epoch_loss = epoch_loss / num_batches
            
            training_history.append({
                'epoch': epoch + 1,
                'avg_loss': avg_epoch_loss,
                'duration': epoch_duration
            })
            
            stats["epochs_completed"] += 1
            
            print(f"  Epoch completed in {epoch_duration:.2f}s, Avg Loss: {avg_epoch_loss:.2f}")
            
            if torch.cuda.is_available() and epoch % 2 == 0:
                torch.cuda.empty_cache()
        
        # Phase 2: Sample generation
        print(f"\nPhase 2: Generating samples...")
        
        vae.eval()
        num_samples = random.randint(500, 2000)
        
        with torch.no_grad():
            for i in range(0, num_samples, batch_size):
                current_batch = min(batch_size, num_samples - i)
                
                # Sample from latent space
                z = torch.randn(current_batch, latent_dim).to(device)
                samples = vae.decode(z)
                
                stats["samples_generated"] += current_batch
                
                if (i + current_batch) % 200 == 0:
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        print(f"  Generated {i + current_batch}/{num_samples} samples, "
                              f"GPU Mem: {gpu_mem:.2f}MB")
                
                del z, samples
        
        # Phase 3: Conditional generation simulation
        print(f"\nPhase 3: Conditional generation...")
        
        for i in range(random.randint(20, 50)):
            batch_size = random.randint(32, 128)
            
            # Random conditioning
            condition = torch.randn(batch_size, latent_dim // 2).to(device)
            noise = torch.randn(batch_size, latent_dim // 2).to(device)
            z = torch.cat([condition, noise], dim=1)
            
            with torch.no_grad():
                generated = vae.decode(z)
            
            stats["samples_generated"] += batch_size
            
            if (i + 1) % 10 == 0:
                if torch.cuda.is_available():
                    gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                    print(f"  Conditional {i+1}: GPU Mem: {gpu_mem:.2f}MB")
            
            del condition, noise, z, generated
            
            time.sleep(random.uniform(0.02, 0.06))
        
        # Phase 4: Latent space interpolation
        print(f"\nPhase 4: Latent space interpolation...")
        
        num_interpolations = random.randint(30, 80)
        
        with torch.no_grad():
            for i in range(num_interpolations):
                # Two random points in latent space
                z1 = torch.randn(1, latent_dim).to(device)
                z2 = torch.randn(1, latent_dim).to(device)
                
                # Interpolate
                num_steps = random.randint(10, 50)
                alphas = torch.linspace(0, 1, num_steps).unsqueeze(1).to(device)
                
                z_interp = z1 * (1 - alphas) + z2 * alphas
                
                # Generate samples along interpolation
                samples = vae.decode(z_interp)
                
                stats["samples_generated"] += num_steps
                
                if (i + 1) % 10 == 0:
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        print(f"  Interpolation {i+1}/{num_interpolations}: {num_steps} steps, "
                              f"GPU Mem: {gpu_mem:.2f}MB")
                
                del z1, z2, alphas, z_interp, samples
                
                time.sleep(random.uniform(0.02, 0.05))
        
        # Phase 5: Reconstruction quality test
        print(f"\nPhase 5: Reconstruction quality test...")
        
        test_samples = random.randint(500, 1500)
        reconstruction_errors = []
        
        with torch.no_grad():
            for i in range(0, test_samples, batch_size):
                current_batch = min(batch_size, test_samples - i)
                
                # Original data
                original = torch.rand(current_batch, input_dim).to(device)
                
                # Encode and decode
                mu, logvar = vae.encode(original)
                z = vae.reparameterize(mu, logvar)
                reconstructed = vae.decode(z)
                
                # Calculate reconstruction error
                error = F.mse_loss(reconstructed, original, reduction='mean')
                reconstruction_errors.append(error.item())
                
                if (i + current_batch) % 200 == 0:
                    if torch.cuda.is_available():
                        gpu_mem = torch.cuda.memory_allocated() / (1024 * 1024)
                        print(f"  Tested {i + current_batch}/{test_samples}, "
                              f"Recon Error: {error.item():.6f}, GPU Mem: {gpu_mem:.2f}MB")
                
                del original, mu, logvar, z, reconstructed
        
        avg_recon_error = sum(reconstruction_errors) / len(reconstruction_errors)
        stats["avg_reconstruction_error"] = avg_recon_error
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save training history
        with open(f"{results_dir}/training_history.json", 'w') as f:
            json.dump(training_history, f, indent=2)
        
        # Save model
        torch.save(vae.state_dict(), f"{results_dir}/vae_model.pth")
        
        # Save stats
        with open(f"{results_dir}/gpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Device: {device}")
        print(f"Epochs completed: {stats['epochs_completed']}")
        print(f"Samples generated: {stats['samples_generated']:,}")
        print(f"Avg reconstruction error: {avg_recon_error:.6f}")
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
