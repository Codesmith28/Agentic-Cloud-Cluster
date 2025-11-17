#!/usr/bin/env python3
"""
Workload Type: mixed
Description: Statistical analysis with hypothesis testing
Resource Requirements: Balanced CPU and memory
Output: None (logs only)
"""

import datetime
import random
import time
import math

def generate_sample_data(n, mean, std):
    """Generate normal distribution sample"""
    samples = []
    for _ in range(n):
        # Box-Muller transform for normal distribution
        u1 = random.random()
        u2 = random.random()
        z0 = math.sqrt(-2.0 * math.log(u1)) * math.cos(2.0 * math.pi * u2)
        sample = mean + z0 * std
        samples.append(sample)
    return samples

def calculate_statistics(data):
    """Calculate descriptive statistics"""
    n = len(data)
    mean = sum(data) / n
    
    variance = sum((x - mean) ** 2 for x in data) / (n - 1)
    std = math.sqrt(variance)
    
    sorted_data = sorted(data)
    median = sorted_data[n // 2] if n % 2 == 1 else (sorted_data[n // 2 - 1] + sorted_data[n // 2]) / 2
    
    q1 = sorted_data[n // 4]
    q3 = sorted_data[3 * n // 4]
    
    return {
        'n': n,
        'mean': mean,
        'std': std,
        'median': median,
        'q1': q1,
        'q3': q3,
        'min': min(data),
        'max': max(data)
    }

def t_test(sample1, sample2):
    """Perform independent samples t-test"""
    time.sleep(0.5)
    
    n1, n2 = len(sample1), len(sample2)
    mean1 = sum(sample1) / n1
    mean2 = sum(sample2) / n2
    
    var1 = sum((x - mean1) ** 2 for x in sample1) / (n1 - 1)
    var2 = sum((x - mean2) ** 2 for x in sample2) / (n2 - 1)
    
    # Pooled standard deviation
    pooled_std = math.sqrt(((n1 - 1) * var1 + (n2 - 1) * var2) / (n1 + n2 - 2))
    
    # T-statistic
    t_stat = (mean1 - mean2) / (pooled_std * math.sqrt(1/n1 + 1/n2))
    
    # Degrees of freedom
    df = n1 + n2 - 2
    
    # Simulated p-value (in real scenario would use t-distribution)
    p_value = random.uniform(0.01, 0.2) if abs(t_stat) < 2 else random.uniform(0.0001, 0.01)
    
    return t_stat, p_value, df

def anova_test(groups):
    """Perform one-way ANOVA"""
    time.sleep(1.0)
    
    # Grand mean
    all_values = [v for group in groups for v in group]
    grand_mean = sum(all_values) / len(all_values)
    
    # Between-group sum of squares
    ss_between = sum(len(group) * (sum(group)/len(group) - grand_mean) ** 2 for group in groups)
    
    # Within-group sum of squares
    ss_within = sum(sum((v - sum(group)/len(group)) ** 2 for v in group) for group in groups)
    
    # Degrees of freedom
    df_between = len(groups) - 1
    df_within = len(all_values) - len(groups)
    
    # Mean squares
    ms_between = ss_between / df_between
    ms_within = ss_within / df_within
    
    # F-statistic
    f_stat = ms_between / ms_within
    
    # Simulated p-value
    p_value = random.uniform(0.001, 0.1)
    
    return f_stat, p_value, df_between, df_within

def correlation_analysis(x, y):
    """Calculate Pearson correlation"""
    time.sleep(0.3)
    
    n = len(x)
    mean_x = sum(x) / n
    mean_y = sum(y) / n
    
    numerator = sum((x[i] - mean_x) * (y[i] - mean_y) for i in range(n))
    denominator = math.sqrt(sum((x[i] - mean_x) ** 2 for i in range(n)) * 
                            sum((y[i] - mean_y) ** 2 for i in range(n)))
    
    correlation = numerator / denominator if denominator != 0 else 0
    
    return correlation

def run_statistical_analysis():
    print(f"[{datetime.datetime.now()}] Starting Statistical Analysis (Mixed Workload)...")
    print("⚖️  Mixed Mode - Balanced CPU & Memory")
    
    print(f"\n{'='*60}")
    print(f"Statistical Analysis Configuration")
    print(f"{'='*60}")
    print(f"✓ Tests: Descriptive Stats, T-Test, ANOVA, Correlation")
    print(f"✓ Samples per group: 1,000")
    
    start_time = datetime.datetime.now()
    
    # Generate sample data
    print(f"\n[GENERATE] Generating sample data...")
    time.sleep(0.5)
    group_a = generate_sample_data(1000, mean=50, std=10)
    group_b = generate_sample_data(1000, mean=52, std=10)
    group_c = generate_sample_data(1000, mean=48, std=10)
    print(f"[GENERATE] ✓ Generated 3 groups with 1,000 samples each")
    
    # Descriptive statistics
    print(f"\n[DESCRIPTIVE] Computing descriptive statistics...")
    time.sleep(0.5)
    stats_a = calculate_statistics(group_a)
    stats_b = calculate_statistics(group_b)
    stats_c = calculate_statistics(group_c)
    
    print(f"\n  Group A:")
    print(f"    Mean: {stats_a['mean']:.2f}, Std: {stats_a['std']:.2f}")
    print(f"    Median: {stats_a['median']:.2f}, IQR: [{stats_a['q1']:.2f}, {stats_a['q3']:.2f}]")
    
    print(f"\n  Group B:")
    print(f"    Mean: {stats_b['mean']:.2f}, Std: {stats_b['std']:.2f}")
    print(f"    Median: {stats_b['median']:.2f}, IQR: [{stats_b['q1']:.2f}, {stats_b['q3']:.2f}]")
    
    print(f"\n  Group C:")
    print(f"    Mean: {stats_c['mean']:.2f}, Std: {stats_c['std']:.2f}")
    print(f"    Median: {stats_c['median']:.2f}, IQR: [{stats_c['q1']:.2f}, {stats_c['q3']:.2f}]")
    
    # T-test
    print(f"\n[T-TEST] Performing independent samples t-tests...")
    t_ab, p_ab, df_ab = t_test(group_a, group_b)
    print(f"  Group A vs B: t={t_ab:.4f}, p={p_ab:.4f}, df={df_ab}")
    
    t_ac, p_ac, df_ac = t_test(group_a, group_c)
    print(f"  Group A vs C: t={t_ac:.4f}, p={p_ac:.4f}, df={df_ac}")
    
    t_bc, p_bc, df_bc = t_test(group_b, group_c)
    print(f"  Group B vs C: t={t_bc:.4f}, p={p_bc:.4f}, df={df_bc}")
    
    # ANOVA
    print(f"\n[ANOVA] Performing one-way ANOVA...")
    f_stat, p_anova, df_between, df_within = anova_test([group_a, group_b, group_c])
    print(f"  F-statistic: {f_stat:.4f}")
    print(f"  p-value: {p_anova:.4f}")
    print(f"  df: ({df_between}, {df_within})")
    
    # Correlation
    print(f"\n[CORRELATION] Computing correlations...")
    time.sleep(0.3)
    cor_ab = correlation_analysis(group_a[:1000], group_b[:1000])
    cor_ac = correlation_analysis(group_a[:1000], group_c[:1000])
    print(f"  Group A & B: r={cor_ab:.4f}")
    print(f"  Group A & C: r={cor_ac:.4f}")
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    print(f"\n{'='*60}")
    print(f"Statistical Analysis Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s")
    print(f"Total Samples Analyzed: {len(group_a) + len(group_b) + len(group_c):,}")
    print(f"Tests Performed: 7")
    
    print(f"\n[{datetime.datetime.now()}] Statistical Analysis completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_statistical_analysis())
