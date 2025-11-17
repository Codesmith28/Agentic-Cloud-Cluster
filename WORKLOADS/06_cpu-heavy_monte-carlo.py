#!/usr/bin/env python3
"""
Workload Type: cpu-heavy
Description: Monte Carlo simulation for Pi estimation
Resource Requirements: High CPU, moderate memory
Output: pi_estimation.csv (generated file)
"""

import datetime
import random
import csv

def estimate_pi(num_samples=10_000_000):
    """Estimate Pi using Monte Carlo method"""
    print(f"[{datetime.datetime.now()}] Starting Monte Carlo Pi Estimation...")
    print(f"Running {num_samples:,} samples...")
    
    inside_circle = 0
    
    for i in range(num_samples):
        x = random.uniform(-1, 1)
        y = random.uniform(-1, 1)
        
        if x*x + y*y <= 1:
            inside_circle += 1
        
        if (i + 1) % 1_000_000 == 0:
            current_pi = 4 * inside_circle / (i + 1)
            print(f"  Progress: {i+1:,} samples, Current Pi estimate: {current_pi:.6f}")
    
    pi_estimate = 4 * inside_circle / num_samples
    error = abs(pi_estimate - 3.14159265359)
    
    print(f"\n✓ Final Pi Estimate: {pi_estimate:.10f}")
    print(f"✓ Error: {error:.10f}")
    print(f"✓ Accuracy: {(1 - error/3.14159) * 100:.4f}%")
    
    # Save results
    with open('/output/pi_estimation.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Metric', 'Value'])
        writer.writerow(['Samples', num_samples])
        writer.writerow(['Pi Estimate', pi_estimate])
        writer.writerow(['Actual Pi', 3.14159265359])
        writer.writerow(['Error', error])
        writer.writerow(['Inside Circle', inside_circle])
    
    print(f"✓ Generated pi_estimation.csv")
    print(f"[{datetime.datetime.now()}] Monte Carlo completed ✓")
    return 0

if __name__ == "__main__":
    exit(estimate_pi())
