#!/usr/bin/env python3
"""
Workload Type: memory-heavy
Description: Time series data processing and analysis
Resource Requirements: Moderate CPU, high memory
Output: timeseries_stats.csv (generated file)
"""

import datetime
import random
import csv
from collections import defaultdict

def generate_timeseries(num_series=1000, points_per_series=10000):
    """Generate multiple time series in memory"""
    print(f"  Generating {num_series} time series with {points_per_series} points each...")
    
    all_series = {}
    
    for series_id in range(num_series):
        series_data = []
        value = random.uniform(50, 150)
        
        for t in range(points_per_series):
            # Random walk
            change = random.uniform(-5, 5)
            value += change
            value = max(0, value)  # Ensure non-negative
            
            series_data.append({
                'timestamp': t,
                'value': value
            })
        
        all_series[f'series_{series_id}'] = series_data
        
        if (series_id + 1) % 200 == 0:
            print(f"    Generated {series_id + 1} series...")
    
    return all_series

def analyze_timeseries():
    print(f"[{datetime.datetime.now()}] Starting Time Series Analysis...")
    
    # Generate data
    all_series = generate_timeseries()
    
    total_points = sum(len(series) for series in all_series.values())
    print(f"\n✓ Generated {len(all_series)} time series")
    print(f"✓ Total data points: {total_points:,}")
    
    # Analyze each series
    print("\nAnalyzing time series...")
    stats = []
    
    for series_id, series_data in list(all_series.items())[:10]:  # Analyze first 10 for demo
        values = [point['value'] for point in series_data]
        
        mean_val = sum(values) / len(values)
        min_val = min(values)
        max_val = max(values)
        
        # Calculate volatility (std dev approximation)
        variance = sum((x - mean_val) ** 2 for x in values) / len(values)
        std_dev = variance ** 0.5
        
        stats.append({
            'series_id': series_id,
            'mean': mean_val,
            'min': min_val,
            'max': max_val,
            'std_dev': std_dev,
            'range': max_val - min_val
        })
        
        print(f"  {series_id}: mean={mean_val:.2f}, std={std_dev:.2f}, range={max_val-min_val:.2f}")
    
    # Save statistics
    with open('/output/timeseries_stats.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['series_id', 'mean', 'min', 'max', 'std_dev', 'range'])
        writer.writeheader()
        writer.writerows(stats)
    
    print(f"\n✓ Generated timeseries_stats.csv")
    print(f"[{datetime.datetime.now()}] Time Series Analysis completed ✓")
    return 0

if __name__ == "__main__":
    exit(analyze_timeseries())
