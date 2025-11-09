#!/usr/bin/env python3
"""
CPU Process 5: Monte Carlo Simulations
Performs Monte Carlo simulations for various problems.
"""
import os
import time
import random
import psutil
import json
import math
from datetime import datetime

def estimate_pi(num_samples):
    """Estimate Pi using Monte Carlo method"""
    inside_circle = 0
    
    for _ in range(num_samples):
        x = random.uniform(-1, 1)
        y = random.uniform(-1, 1)
        
        if x*x + y*y <= 1:
            inside_circle += 1
    
    return 4 * inside_circle / num_samples

def simulate_random_walk(steps):
    """2D random walk simulation"""
    x, y = 0, 0
    positions = [(0, 0)]
    
    for _ in range(steps):
        direction = random.choice([(0,1), (0,-1), (1,0), (-1,0)])
        x += direction[0]
        y += direction[1]
        positions.append((x, y))
    
    distance = math.sqrt(x**2 + y**2)
    return distance, positions

def monte_carlo_integration(func, a, b, num_samples):
    """Monte Carlo integration"""
    total = 0
    for _ in range(num_samples):
        x = random.uniform(a, b)
        total += func(x)
    
    return (b - a) * total / num_samples

def main():
    print(f"[{datetime.now()}] Starting CPU-Intensive Process 5: Monte Carlo Simulations")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "CPU-Intensive-5",
        "simulations_completed": 0,
        "total_samples": 0,
        "peak_cpu_percent": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    simulation_results = []
    
    try:
        # Phase 1: Estimate Pi
        print(f"Phase 1: Estimating Pi using Monte Carlo...")
        
        num_estimations = random.randint(10, 30)
        samples_per_estimation = random.randint(500000, 2000000)
        
        pi_estimates = []
        
        for i in range(num_estimations):
            pi_estimate = estimate_pi(samples_per_estimation)
            pi_estimates.append(pi_estimate)
            
            stats["simulations_completed"] += 1
            stats["total_samples"] += samples_per_estimation
            
            error = abs(pi_estimate - math.pi)
            
            if (i + 1) % 5 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Estimation {i+1}/{num_estimations}: π ≈ {pi_estimate:.6f}, "
                      f"Error: {error:.6f}, CPU: {cpu_percent:.1f}%")
            
            time.sleep(random.uniform(0.01, 0.05))
        
        avg_pi = sum(pi_estimates) / len(pi_estimates)
        print(f"  Average π estimate: {avg_pi:.6f}")
        
        # Phase 2: Random Walk Simulations
        print(f"\nPhase 2: Random walk simulations...")
        
        num_walks = random.randint(100, 500)
        steps_per_walk = random.randint(10000, 50000)
        
        walk_distances = []
        
        for i in range(num_walks):
            distance, positions = simulate_random_walk(steps_per_walk)
            walk_distances.append(distance)
            
            stats["simulations_completed"] += 1
            stats["total_samples"] += steps_per_walk
            
            simulation_results.append({
                'simulation_type': 'random_walk',
                'steps': steps_per_walk,
                'final_distance': distance,
                'final_position': positions[-1]
            })
            
            if (i + 1) % 50 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Walk {i+1}/{num_walks}: Distance={distance:.2f}, "
                      f"CPU: {cpu_percent:.1f}%, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.005, 0.02))
        
        avg_distance = sum(walk_distances) / len(walk_distances)
        print(f"  Average final distance: {avg_distance:.2f}")
        
        # Phase 3: Monte Carlo Integration
        print(f"\nPhase 3: Monte Carlo integration...")
        
        functions = [
            (lambda x: x**2, 0, 1, "x^2"),
            (lambda x: math.sin(x), 0, math.pi, "sin(x)"),
            (lambda x: math.exp(-x**2), -2, 2, "exp(-x^2)"),
            (lambda x: 1/(1 + x**2), 0, 1, "1/(1+x^2)"),
        ]
        
        integration_samples = random.randint(1000000, 5000000)
        
        for func, a, b, name in functions:
            result = monte_carlo_integration(func, a, b, integration_samples)
            
            stats["simulations_completed"] += 1
            stats["total_samples"] += integration_samples
            
            simulation_results.append({
                'simulation_type': 'integration',
                'function': name,
                'interval': [a, b],
                'result': result,
                'samples': integration_samples
            })
            
            cpu_percent = process.cpu_percent()
            stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
            
            print(f"  ∫{name} from {a} to {b} ≈ {result:.6f}, CPU: {cpu_percent:.1f}%")
            time.sleep(random.uniform(0.05, 0.15))
        
        # Phase 4: Coin flip simulation
        print(f"\nPhase 4: Probability simulations...")
        
        num_trials = random.randint(100, 500)
        flips_per_trial = random.randint(10000, 100000)
        
        for i in range(num_trials):
            heads = sum(1 for _ in range(flips_per_trial) if random.random() < 0.5)
            heads_ratio = heads / flips_per_trial
            
            stats["simulations_completed"] += 1
            stats["total_samples"] += flips_per_trial
            
            if (i + 1) % 50 == 0:
                cpu_percent = process.cpu_percent()
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_cpu_percent"] = max(stats["peak_cpu_percent"], cpu_percent)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Trial {i+1}/{num_trials}: Heads ratio={heads_ratio:.4f}, "
                      f"CPU: {cpu_percent:.1f}%")
            
            if i % 20 == 0:
                time.sleep(random.uniform(0.01, 0.03))
        
        stats["duration_seconds"] = time.time() - start_time
        stats["samples_per_second"] = stats["total_samples"] / stats["duration_seconds"]
        stats["pi_estimate"] = avg_pi
        
        # Save simulation results
        with open(f"{results_dir}/simulation_results.json", 'w') as f:
            json.dump(simulation_results[:100], f, indent=2)
        
        # Save stats
        with open(f"{results_dir}/cpu_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Simulations completed: {stats['simulations_completed']}")
        print(f"Total samples: {stats['total_samples']:,}")
        print(f"Samples/second: {stats['samples_per_second']:.2f}")
        print(f"π estimate: {stats['pi_estimate']:.6f}")
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
