#!/usr/bin/env python3
"""
CPU Process 20: Fourier Transforms and Signal Processing
FFT computations and signal analysis.
"""
import os
import time
import random
import psutil
import json
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from datetime import datetime

def generate_signal(duration, sample_rate, frequencies, amplitudes):
    """Generate composite signal"""
    t = np.linspace(0, duration, int(sample_rate * duration))
    signal = np.zeros_like(t)
    
    for freq, amp in zip(frequencies, amplitudes):
        signal += amp * np.sin(2 * np.pi * freq * t)
    
    return t, signal

def add_noise(signal, noise_level):
    """Add random noise to signal"""
    noise = np.random.normal(0, noise_level, len(signal))
    return signal + noise

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 20: Signal Processing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-20",
        "ffts_computed": 0,
        "signals_processed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Phase 1: Generate and analyze signals
        print("Phase 1: FFT analysis of signals...")
        
        num_signals = random.randint(100, 300)
        
        for i in range(num_signals):
            duration = random.uniform(1.0, 5.0)
            sample_rate = random.randint(1000, 10000)
            
            # Random frequency components
            num_freqs = random.randint(3, 10)
            frequencies = [random.uniform(1, 500) for _ in range(num_freqs)]
            amplitudes = [random.uniform(0.1, 2.0) for _ in range(num_freqs)]
            
            t, signal = generate_signal(duration, sample_rate, frequencies, amplitudes)
            noisy_signal = add_noise(signal, random.uniform(0.1, 0.5))
            
            # Compute FFT
            fft_result = np.fft.fft(noisy_signal)
            fft_freq = np.fft.fftfreq(len(noisy_signal), 1/sample_rate)
            
            stats["ffts_computed"] += 1
            stats["signals_processed"] += 1
            
            if (i + 1) % 50 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Processed {i+1}/{num_signals} signals, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 2: 2D FFT on images
        print("\nPhase 2: 2D FFT on image data...")
        
        num_images = random.randint(30, 80)
        
        for i in range(num_images):
            size = random.choice([128, 256, 512])
            image = np.random.rand(size, size)
            
            # Add some structure
            x, y = np.meshgrid(np.linspace(-5, 5, size), np.linspace(-5, 5, size))
            image += np.sin(x) * np.cos(y)
            
            # 2D FFT
            fft_2d = np.fft.fft2(image)
            fft_2d_shifted = np.fft.fftshift(fft_2d)
            
            stats["ffts_computed"] += 1
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Processed {i+1}/{num_images} 2D FFTs ({size}x{size}), CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        # Phase 3: Convolution operations
        print("\nPhase 3: Convolution operations...")
        
        num_convolutions = random.randint(500, 1500)
        
        for i in range(num_convolutions):
            signal_len = random.randint(1000, 10000)
            kernel_len = random.randint(10, 100)
            
            signal = np.random.rand(signal_len)
            kernel = np.random.rand(kernel_len)
            
            # Convolution
            result = np.convolve(signal, kernel, mode='same')
            
            if (i + 1) % 200 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Completed {i+1}/{num_convolutions} convolutions, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.001, 0.01))
        
        # Generate visualization
        print("\nGenerating signal processing visualization...")
        
        # Create sample signal for visualization
        t, signal = generate_signal(2.0, 4000, [10, 50, 120], [1.0, 0.5, 0.3])
        noisy_signal = add_noise(signal, 0.2)
        fft_result = np.fft.fft(noisy_signal)
        fft_freq = np.fft.fftfreq(len(noisy_signal), 1/4000)
        
        fig, axes = plt.subplots(2, 2, figsize=(12, 10))
        
        # Time domain
        axes[0, 0].plot(t[:1000], signal[:1000], 'b-', label='Original')
        axes[0, 0].plot(t[:1000], noisy_signal[:1000], 'r-', alpha=0.5, label='Noisy')
        axes[0, 0].set_xlabel('Time (s)')
        axes[0, 0].set_ylabel('Amplitude')
        axes[0, 0].set_title('Time Domain Signal')
        axes[0, 0].legend()
        axes[0, 0].grid(True)
        
        # Frequency domain
        axes[0, 1].plot(fft_freq[:len(fft_freq)//2], np.abs(fft_result)[:len(fft_result)//2])
        axes[0, 1].set_xlabel('Frequency (Hz)')
        axes[0, 1].set_ylabel('Magnitude')
        axes[0, 1].set_title('Frequency Domain (FFT)')
        axes[0, 1].grid(True)
        
        # 2D FFT sample
        sample_image = np.random.rand(128, 128)
        x, y = np.meshgrid(np.linspace(-5, 5, 128), np.linspace(-5, 5, 128))
        sample_image += np.sin(x) * np.cos(y)
        fft_2d_sample = np.fft.fft2(sample_image)
        
        axes[1, 0].imshow(sample_image, cmap='viridis')
        axes[1, 0].set_title('Sample 2D Signal')
        axes[1, 0].axis('off')
        
        axes[1, 1].imshow(np.log(np.abs(np.fft.fftshift(fft_2d_sample)) + 1), cmap='hot')
        axes[1, 1].set_title('2D FFT Magnitude (log scale)')
        axes[1, 1].axis('off')
        
        plt.tight_layout()
        plt.savefig(f"{results_dir}/signal_processing.png", dpi=150)
        plt.close()
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"FFTs computed: {stats['ffts_computed']}")
        print(f"Signals processed: {stats['signals_processed']}")
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
