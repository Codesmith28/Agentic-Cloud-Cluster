#!/usr/bin/env python3
"""
CPU Process 9: Fourier Transforms and Signal Processing
Tests CPU with FFT and signal processing operations.
"""
import os
import time
import random
import psutil
import json
import math
from datetime import datetime

def dft(signal):
    """Discrete Fourier Transform (naive implementation)"""
    N = len(signal)
    result = []
    
    for k in range(N):
        real = 0
        imag = 0
        for n in range(N):
            angle = 2 * math.pi * k * n / N
            real += signal[n] * math.cos(angle)
            imag -= signal[n] * math.sin(angle)
        result.append((real, imag))
    
    return result

def idft(spectrum):
    """Inverse Discrete Fourier Transform"""
    N = len(spectrum)
    result = []
    
    for n in range(N):
        real = 0
        for k in range(N):
            angle = 2 * math.pi * k * n / N
            real += spectrum[k][0] * math.cos(angle) - spectrum[k][1] * math.sin(angle)
        result.append(real / N)
    
    return result

def fft_cooley_tukey(signal):
    """Fast Fourier Transform using Cooley-Tukey algorithm"""
    N = len(signal)
    
    if N <= 1:
        return [(signal[0], 0)] if N == 1 else []
    
    # Ensure power of 2
    if N & (N - 1) != 0:
        # Pad with zeros
        next_power = 1
        while next_power < N:
            next_power *= 2
        signal = signal + [0] * (next_power - N)
        N = next_power
    
    # Base case
    if N == 1:
        return [(signal[0], 0)]
    
    # Divide
    even = fft_cooley_tukey([signal[i] for i in range(0, N, 2)])
    odd = fft_cooley_tukey([signal[i] for i in range(1, N, 2)])
    
    # Conquer
    result = [(0, 0)] * N
    for k in range(N // 2):
        angle = -2 * math.pi * k / N
        w_real = math.cos(angle)
        w_imag = math.sin(angle)
        
        # Complex multiplication
        t_real = w_real * odd[k][0] - w_imag * odd[k][1]
        t_imag = w_real * odd[k][1] + w_imag * odd[k][0]
        
        result[k] = (even[k][0] + t_real, even[k][1] + t_imag)
        result[k + N // 2] = (even[k][0] - t_real, even[k][1] - t_imag)
    
    return result

def generate_signal(length, signal_type='mixed'):
    """Generate test signals"""
    signal = []
    
    if signal_type == 'sine':
        freq = random.uniform(1, 10)
        for i in range(length):
            signal.append(math.sin(2 * math.pi * freq * i / length))
    
    elif signal_type == 'cosine':
        freq = random.uniform(1, 10)
        for i in range(length):
            signal.append(math.cos(2 * math.pi * freq * i / length))
    
    elif signal_type == 'square':
        freq = random.randint(2, 8)
        for i in range(length):
            signal.append(1 if (i % (length // freq)) < (length // (2 * freq)) else -1)
    
    elif signal_type == 'sawtooth':
        freq = random.randint(2, 8)
        for i in range(length):
            signal.append(2 * ((i % (length // freq)) / (length // freq)) - 1)
    
    elif signal_type == 'noise':
        signal = [random.uniform(-1, 1) for _ in range(length)]
    
    else:  # mixed
        # Combine multiple frequencies
        num_components = random.randint(2, 5)
        signal = [0] * length
        
        for _ in range(num_components):
            freq = random.uniform(0.5, 10)
            amplitude = random.uniform(0.3, 1.0)
            phase = random.uniform(0, 2 * math.pi)
            
            for i in range(length):
                signal[i] += amplitude * math.sin(2 * math.pi * freq * i / length + phase)
        
        # Add noise
        noise_level = random.uniform(0.1, 0.3)
        for i in range(length):
            signal[i] += random.uniform(-noise_level, noise_level)
    
    return signal

def convolve(signal1, signal2):
    """Convolution of two signals"""
    n1 = len(signal1)
    n2 = len(signal2)
    result = [0] * (n1 + n2 - 1)
    
    for i in range(n1):
        for j in range(n2):
            result[i + j] += signal1[i] * signal2[j]
    
    return result

def compute_power_spectrum(spectrum):
    """Compute power spectrum from FFT result"""
    return [real**2 + imag**2 for real, imag in spectrum]

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 9: Fourier Transforms")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-9",
        "transforms_computed": 0,
        "convolutions_computed": 0,
        "total_signal_length": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    transform_results = []
    
    try:
        # Phase 1: DFT on small signals
        print(f"Phase 1: Computing DFT on various signals...")
        
        signal_types = ['sine', 'cosine', 'square', 'sawtooth', 'noise', 'mixed']
        num_small_signals = random.randint(30, 60)
        
        for i in range(num_small_signals):
            signal_type = random.choice(signal_types)
            signal_length = random.randint(64, 256)
            
            signal = generate_signal(signal_length, signal_type)
            
            # Compute DFT
            dft_start = time.time()
            spectrum = dft(signal)
            dft_time = time.time() - dft_start
            
            stats["transforms_computed"] += 1
            stats["total_signal_length"] += signal_length
            
            # Compute power spectrum
            power = compute_power_spectrum(spectrum)
            max_power_idx = power.index(max(power))
            
            transform_results.append({
                'method': 'DFT',
                'signal_type': signal_type,
                'length': signal_length,
                'time': dft_time,
                'dominant_frequency_bin': max_power_idx
            })
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  DFT {i+1}/{num_small_signals}: {signal_type} signal, "
                      f"Length={signal_length}, Time={dft_time:.3f}s, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 2: FFT on larger signals
        print(f"\nPhase 2: Computing FFT on larger signals...")
        
        num_fft_tests = random.randint(20, 40)
        
        for i in range(num_fft_tests):
            signal_type = random.choice(signal_types)
            # FFT works best with power of 2
            power = random.randint(8, 12)
            signal_length = 2 ** power
            
            signal = generate_signal(signal_length, signal_type)
            
            # Compute FFT
            fft_start = time.time()
            spectrum = fft_cooley_tukey(signal)
            fft_time = time.time() - fft_start
            
            stats["transforms_computed"] += 1
            stats["total_signal_length"] += signal_length
            
            # Compute power spectrum
            power_spec = compute_power_spectrum(spectrum)
            
            transform_results.append({
                'method': 'FFT',
                'signal_type': signal_type,
                'length': signal_length,
                'time': fft_time,
                'max_power': max(power_spec)
            })
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  FFT {i+1}/{num_fft_tests}: {signal_type} signal, "
                      f"Length={signal_length}, Time={fft_time:.3f}s, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.02, 0.06))
        
        # Phase 3: Inverse transforms
        print(f"\nPhase 3: Testing inverse transforms...")
        
        num_inverse_tests = random.randint(15, 30)
        
        for i in range(num_inverse_tests):
            signal_length = random.choice([64, 128, 256])
            original_signal = generate_signal(signal_length, 'mixed')
            
            # Forward transform
            spectrum = dft(original_signal)
            
            # Inverse transform
            reconstructed = idft(spectrum)
            
            # Compute reconstruction error
            error = sum((original_signal[j] - reconstructed[j])**2 
                       for j in range(signal_length)) / signal_length
            
            stats["transforms_computed"] += 2  # Forward and inverse
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Inverse DFT {i+1}/{num_inverse_tests}: Length={signal_length}, "
                      f"Reconstruction error={error:.6f}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.02, 0.05))
        
        # Phase 4: Convolution operations
        print(f"\nPhase 4: Computing convolutions...")
        
        num_conv_tests = random.randint(20, 50)
        
        for i in range(num_conv_tests):
            len1 = random.randint(50, 200)
            len2 = random.randint(20, 100)
            
            signal1 = generate_signal(len1, random.choice(signal_types))
            signal2 = generate_signal(len2, random.choice(signal_types))
            
            # Compute convolution
            conv_start = time.time()
            result = convolve(signal1, signal2)
            conv_time = time.time() - conv_start
            
            stats["convolutions_computed"] += 1
            
            if (i + 1) % 10 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Convolution {i+1}/{num_conv_tests}: "
                      f"{len1}⊗{len2}→{len(result)}, Time={conv_time:.3f}s, "
                      f"CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.03))
        
        # Phase 5: Spectral analysis
        print(f"\nPhase 5: Spectral analysis of complex signals...")
        
        num_analysis_tests = random.randint(10, 20)
        
        for i in range(num_analysis_tests):
            signal_length = 2 ** random.randint(9, 11)
            
            # Create signal with multiple frequency components
            signal = [0] * signal_length
            num_freqs = random.randint(3, 7)
            frequencies = []
            
            for _ in range(num_freqs):
                freq = random.uniform(1, signal_length // 10)
                amplitude = random.uniform(0.5, 2.0)
                frequencies.append(freq)
                
                for j in range(signal_length):
                    signal[j] += amplitude * math.sin(2 * math.pi * freq * j / signal_length)
            
            # Add noise
            noise_level = random.uniform(0.2, 0.5)
            for j in range(signal_length):
                signal[j] += random.uniform(-noise_level, noise_level)
            
            # Compute FFT
            spectrum = fft_cooley_tukey(signal)
            power = compute_power_spectrum(spectrum)
            
            # Find dominant frequencies
            sorted_indices = sorted(range(len(power)), key=lambda k: power[k], reverse=True)
            top_frequencies = sorted_indices[:num_freqs]
            
            stats["transforms_computed"] += 1
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            if (i + 1) % 3 == 0:
                print(f"  Analysis {i+1}/{num_analysis_tests}: Length={signal_length}, "
                      f"Components={num_freqs}, CPU: {cpu_percent:.1f}%, "
                      f"Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.1, 0.3))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["avg_signal_length"] = stats["total_signal_length"] / stats["transforms_computed"] if stats["transforms_computed"] > 0 else 0
        
        # Save transform results
        with open(f"{results_dir}/transform_results.json", 'w') as f:
            json.dump(transform_results[:100], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Transforms computed: {stats['transforms_computed']}")
        print(f"Convolutions computed: {stats['convolutions_computed']}")
        print(f"Total signal length: {stats['total_signal_length']:,}")
        print(f"Avg signal length: {stats['avg_signal_length']:.1f}")
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
