#!/usr/bin/env python3
"""
CPU Process 14: Numerical Integration and Calculus
Monte Carlo integration and numerical differentiation.
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

def monte_carlo_integration(func, a, b, n_samples):
    """Monte Carlo integration"""
    x_random = np.random.uniform(a, b, n_samples)
    y_values = func(x_random)
    integral = (b - a) * np.mean(y_values)
    return integral

def trapezoidal_rule(func, a, b, n):
    """Trapezoidal rule for integration"""
    x = np.linspace(a, b, n)
    y = func(x)
    h = (b - a) / (n - 1)
    integral = h * (y[0]/2 + np.sum(y[1:-1]) + y[-1]/2)
    return integral

def simpsons_rule(func, a, b, n):
    """Simpson's rule for integration"""
    if n % 2 == 0:
        n += 1
    x = np.linspace(a, b, n)
    y = func(x)
    h = (b - a) / (n - 1)
    integral = h/3 * (y[0] + 4*np.sum(y[1:-1:2]) + 2*np.sum(y[2:-2:2]) + y[-1])
    return integral

def numerical_derivative(func, x, h=1e-5):
    """Numerical derivative using central difference"""
    return (func(x + h) - func(x - h)) / (2 * h)

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 14: Numerical Integration")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-14",
        "integrations_computed": 0,
        "derivatives_computed": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    results = []
    
    try:
        # Define test functions
        functions = [
            (lambda x: np.sin(x), "sin(x)", 0, np.pi),
            (lambda x: np.exp(-x**2), "exp(-x^2)", -3, 3),
            (lambda x: x**2, "x^2", 0, 10),
            (lambda x: 1/(1+x**2), "1/(1+x^2)", -5, 5),
            (lambda x: np.log(x+1), "log(x+1)", 0, 10),
        ]
        
        # Phase 1: Monte Carlo Integration
        print("Phase 1: Monte Carlo integration...")
        
        for func, name, a, b in functions:
            n_samples = random.randint(1000000, 5000000)
            integral = monte_carlo_integration(func, a, b, n_samples)
            
            results.append({
                'method': 'monte_carlo',
                'function': name,
                'bounds': [a, b],
                'samples': n_samples,
                'result': float(integral)
            })
            
            stats["integrations_computed"] += 1
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  {name}: {integral:.6f} with {n_samples:,} samples, CPU: {cpu_percent:.1f}%")
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 2: Trapezoidal and Simpson's rule
        print("\nPhase 2: Trapezoidal and Simpson's rule...")
        
        for func, name, a, b in functions:
            n_points = random.randint(100000, 500000)
            
            trap_integral = trapezoidal_rule(func, a, b, n_points)
            simp_integral = simpsons_rule(func, a, b, n_points)
            
            results.append({
                'method': 'trapezoidal',
                'function': name,
                'points': n_points,
                'result': float(trap_integral)
            })
            
            results.append({
                'method': 'simpsons',
                'function': name,
                'points': n_points,
                'result': float(simp_integral)
            })
            
            stats["integrations_computed"] += 2
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  {name}: Trap={trap_integral:.6f}, Simp={simp_integral:.6f}, CPU: {cpu_percent:.1f}%")
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 3: Numerical derivatives
        print("\nPhase 3: Computing numerical derivatives...")
        
        num_derivatives = random.randint(50000, 100000)
        for i in range(num_derivatives):
            func_idx = random.randint(0, len(functions) - 1)
            func, name, a, b = functions[func_idx]
            x = random.uniform(a + 0.1, b - 0.1)
            
            deriv = numerical_derivative(func, x)
            stats["derivatives_computed"] += 1
            
            if (i + 1) % 10000 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Computed {i+1}/{num_derivatives} derivatives, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.00001, 0.0001))
        
        # Generate visualization
        print("\nGenerating integration visualization...")
        fig, axes = plt.subplots(2, 3, figsize=(15, 10))
        axes = axes.flatten()
        
        for idx, (func, name, a, b) in enumerate(functions):
            x = np.linspace(a, b, 1000)
            y = func(x)
            axes[idx].plot(x, y, 'b-', linewidth=2)
            axes[idx].fill_between(x, y, alpha=0.3)
            axes[idx].set_title(f'Integral of {name}')
            axes[idx].set_xlabel('x')
            axes[idx].set_ylabel('f(x)')
            axes[idx].grid(True)
        
        axes[-1].axis('off')
        plt.tight_layout()
        plt.savefig(f"{results_dir}/integration_visualization.png", dpi=150)
        plt.close()
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/integration_results.json", 'w') as f:
            json.dump(results, f, indent=2)
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Integrations computed: {stats['integrations_computed']}")
        print(f"Derivatives computed: {stats['derivatives_computed']:,}")
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
