#!/usr/bin/env python3
"""
CPU Process 18: Monte Carlo Simulations
Various Monte Carlo simulations for statistical analysis.
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

def estimate_pi(num_samples):
    """Estimate pi using Monte Carlo method"""
    inside_circle = 0
    for _ in range(num_samples):
        x, y = random.random(), random.random()
        if x*x + y*y <= 1:
            inside_circle += 1
    return 4 * inside_circle / num_samples

def simulate_random_walk(steps):
    """Simulate 2D random walk"""
    x, y = 0, 0
    positions = [(x, y)]
    
    for _ in range(steps):
        direction = random.choice([(0, 1), (0, -1), (1, 0), (-1, 0)])
        x += direction[0]
        y += direction[1]
        positions.append((x, y))
    
    return positions

def simulate_coin_flips(num_flips, num_simulations):
    """Simulate coin flips and analyze distribution"""
    results = []
    for _ in range(num_simulations):
        heads = sum(1 for _ in range(num_flips) if random.random() < 0.5)
        results.append(heads)
    return results

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 18: Monte Carlo Simulations")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-18",
        "simulations_run": 0,
        "samples_generated": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Phase 1: Estimate Pi
        print("Phase 1: Estimating Pi using Monte Carlo...")
        num_trials = random.randint(10, 30)
        pi_estimates = []
        
        for i in range(num_trials):
            samples = random.randint(1000000, 5000000)
            pi_est = estimate_pi(samples)
            pi_estimates.append({'samples': samples, 'estimate': pi_est, 'error': abs(pi_est - np.pi)})
            
            stats["simulations_run"] += 1
            stats["samples_generated"] += samples
            
            cpu_percent = process.cpu_percent()
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            print(f"  Trial {i+1}: Pi ≈ {pi_est:.6f}, samples={samples:,}, CPU: {cpu_percent:.1f}%")
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 2: Random walks
        print("\nPhase 2: Simulating random walks...")
        num_walks = random.randint(100, 500)
        walk_distances = []
        
        for i in range(num_walks):
            steps = random.randint(10000, 50000)
            walk = simulate_random_walk(steps)
            final_x, final_y = walk[-1]
            distance = np.sqrt(final_x**2 + final_y**2)
            walk_distances.append({'steps': steps, 'distance': distance})
            
            stats["simulations_run"] += 1
            stats["samples_generated"] += steps
            
            if (i + 1) % 50 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Walk {i+1}/{num_walks}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        # Phase 3: Coin flip simulations
        print("\nPhase 3: Simulating coin flips...")
        num_flips = random.randint(1000, 5000)
        num_simulations = random.randint(10000, 50000)
        
        flip_results = simulate_coin_flips(num_flips, num_simulations)
        stats["simulations_run"] += num_simulations
        stats["samples_generated"] += num_flips * num_simulations
        
        mean_heads = np.mean(flip_results)
        std_heads = np.std(flip_results)
        
        cpu_percent = process.cpu_percent()
        mem_mb = process.memory_info().rss / (1024 * 1024)
        stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
        stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
        
        print(f"  Simulated {num_simulations:,} trials of {num_flips} flips")
        print(f"  Mean heads: {mean_heads:.2f}, Std: {std_heads:.2f}, CPU: {cpu_percent:.1f}%")
        
        # Generate visualization
        print("\nGenerating Monte Carlo visualization...")
        fig, axes = plt.subplots(2, 2, figsize=(12, 10))
        
        # Pi estimates
        axes[0, 0].plot([e['samples'] for e in pi_estimates], [e['estimate'] for e in pi_estimates], 'bo-')
        axes[0, 0].axhline(y=np.pi, color='r', linestyle='--', label='Actual π')
        axes[0, 0].set_xlabel('Samples')
        axes[0, 0].set_ylabel('Pi Estimate')
        axes[0, 0].set_title('Pi Estimation Convergence')
        axes[0, 0].legend()
        axes[0, 0].grid(True)
        
        # Random walk distances
        axes[0, 1].scatter([w['steps'] for w in walk_distances], [w['distance'] for w in walk_distances], alpha=0.5)
        axes[0, 1].set_xlabel('Steps')
        axes[0, 1].set_ylabel('Final Distance from Origin')
        axes[0, 1].set_title('Random Walk Distances')
        axes[0, 1].grid(True)
        
        # Coin flip histogram
        axes[1, 0].hist(flip_results, bins=50, edgecolor='black')
        axes[1, 0].axvline(x=mean_heads, color='r', linestyle='--', label=f'Mean: {mean_heads:.1f}')
        axes[1, 0].set_xlabel('Number of Heads')
        axes[1, 0].set_ylabel('Frequency')
        axes[1, 0].set_title(f'Distribution of Heads ({num_flips} flips)')
        axes[1, 0].legend()
        axes[1, 0].grid(True)
        
        # Sample random walk
        sample_walk = simulate_random_walk(5000)
        xs, ys = zip(*sample_walk)
        axes[1, 1].plot(xs, ys, 'b-', alpha=0.6, linewidth=0.5)
        axes[1, 1].plot(0, 0, 'go', markersize=10, label='Start')
        axes[1, 1].plot(xs[-1], ys[-1], 'ro', markersize=10, label='End')
        axes[1, 1].set_xlabel('X Position')
        axes[1, 1].set_ylabel('Y Position')
        axes[1, 1].set_title('Sample Random Walk (5000 steps)')
        axes[1, 1].legend()
        axes[1, 1].grid(True)
        
        plt.tight_layout()
        plt.savefig(f"{results_dir}/monte_carlo_simulations.png", dpi=150)
        plt.close()
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/monte_carlo_results.json", 'w') as f:
            json.dump({
                'pi_estimates': pi_estimates,
                'walk_distances_sample': walk_distances[:100],
                'coin_flip_stats': {
                    'mean': mean_heads,
                    'std': std_heads,
                    'min': min(flip_results),
                    'max': max(flip_results)
                }
            }, f, indent=2)
        
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Simulations run: {stats['simulations_run']:,}")
        print(f"Samples generated: {stats['samples_generated']:,}")
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
