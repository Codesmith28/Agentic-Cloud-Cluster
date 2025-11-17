#!/usr/bin/env python3
"""
Workload Type: mixed
Description: ML data preprocessing with feature engineering
Resource Requirements: Balanced CPU and memory
Output: preprocessed_features.csv (generated file)
"""

import datetime
import random
import csv
import time
import math

def load_raw_data(num_samples=5000):
    """Simulate loading raw data"""
    print(f"\n[LOAD] Loading raw data...")
    time.sleep(0.5)
    
    data = []
    for i in range(num_samples):
        sample = {
            'id': i,
            'feature_1': random.uniform(-10, 10),
            'feature_2': random.uniform(0, 100),
            'feature_3': random.uniform(-1, 1),
            'feature_4': random.randint(0, 10),
            'label': random.choice([0, 1])
        }
        data.append(sample)
    
    print(f"[LOAD] ✓ Loaded {len(data):,} samples")
    return data

def normalize_features(data):
    """Simulate feature normalization"""
    print(f"\n[NORMALIZE] Normalizing features...")
    time.sleep(1.0)
    
    # Calculate mean and std for each feature
    features = ['feature_1', 'feature_2', 'feature_3']
    stats = {}
    
    for feat in features:
        values = [s[feat] for s in data]
        mean = sum(values) / len(values)
        variance = sum((v - mean) ** 2 for v in values) / len(values)
        std = math.sqrt(variance)
        stats[feat] = {'mean': mean, 'std': std}
    
    # Normalize
    for sample in data:
        for feat in features:
            sample[f'{feat}_norm'] = (sample[feat] - stats[feat]['mean']) / (stats[feat]['std'] + 1e-8)
    
    print(f"[NORMALIZE] ✓ Normalized {len(features)} features")
    return data

def engineer_features(data):
    """Simulate feature engineering"""
    print(f"\n[ENGINEER] Engineering new features...")
    time.sleep(1.5)
    
    for sample in data:
        # Polynomial features
        sample['f1_squared'] = sample['feature_1'] ** 2
        sample['f2_squared'] = sample['feature_2'] ** 2
        
        # Interaction features
        sample['f1_f2_interaction'] = sample['feature_1'] * sample['feature_2']
        sample['f1_f3_interaction'] = sample['feature_1'] * sample['feature_3']
        
        # Log features (with safety for negative values)
        sample['f2_log'] = math.log(abs(sample['feature_2']) + 1)
        
        # Binned features
        sample['f4_binned'] = 'low' if sample['feature_4'] < 4 else 'medium' if sample['feature_4'] < 8 else 'high'
    
    print(f"[ENGINEER] ✓ Created 7 new features")
    return data

def handle_missing_values(data):
    """Simulate missing value imputation"""
    print(f"\n[IMPUTE] Handling missing values...")
    time.sleep(0.5)
    
    # Simulate some missing values
    num_missing = int(len(data) * 0.02)  # 2% missing
    missing_indices = random.sample(range(len(data)), num_missing)
    
    for idx in missing_indices:
        data[idx]['feature_2'] = None
    
    # Impute with mean
    non_missing = [s['feature_2'] for s in data if s.get('feature_2') is not None]
    mean_value = sum(non_missing) / len(non_missing)
    
    for sample in data:
        if sample.get('feature_2') is None:
            sample['feature_2'] = mean_value
    
    print(f"[IMPUTE] ✓ Imputed {num_missing} missing values")
    return data

def split_dataset(data):
    """Simulate train/test split"""
    print(f"\n[SPLIT] Splitting dataset...")
    time.sleep(0.3)
    
    random.shuffle(data)
    split_point = int(len(data) * 0.8)
    
    train_data = data[:split_point]
    test_data = data[split_point:]
    
    print(f"[SPLIT] ✓ Train: {len(train_data):,}, Test: {len(test_data):,}")
    return train_data, test_data

def run_ml_preprocessing():
    print(f"[{datetime.datetime.now()}] Starting ML Data Preprocessing (Mixed Workload)...")
    print("⚖️  Mixed Mode - Balanced CPU & Memory")
    
    print(f"\n{'='*60}")
    print(f"Preprocessing Pipeline")
    print(f"{'='*60}")
    print(f"✓ Steps: Load -> Normalize -> Engineer -> Impute -> Split")
    print(f"✓ Samples: 5,000")
    print(f"✓ Original Features: 5")
    
    start_time = datetime.datetime.now()
    
    # Pipeline steps
    data = load_raw_data(5000)
    data = normalize_features(data)
    data = engineer_features(data)
    data = handle_missing_values(data)
    train_data, test_data = split_dataset(data)
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    # Calculate feature statistics
    train_pos = sum(1 for s in train_data if s['label'] == 1)
    test_pos = sum(1 for s in test_data if s['label'] == 1)
    
    print(f"\n{'='*60}")
    print(f"Preprocessing Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s")
    print(f"Total Features: {len([k for k in train_data[0].keys() if 'feature' in k or 'f1' in k or 'f2' in k or 'f4' in k])}")
    print(f"\nDataset Split:")
    print(f"  Train: {len(train_data):,} samples ({train_pos} positive, {len(train_data)-train_pos} negative)")
    print(f"  Test: {len(test_data):,} samples ({test_pos} positive, {len(test_data)-test_pos} negative)")
    
    # Save sample of preprocessed features
    sample_size = min(100, len(train_data))
    with open('/output/preprocessed_features.csv', 'w', newline='') as f:
        fieldnames = ['id', 'feature_1', 'feature_2', 'feature_3', 'f1_squared', 'f1_f2_interaction', 'label']
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        
        for sample in train_data[:sample_size]:
            writer.writerow({
                'id': sample['id'],
                'feature_1': f"{sample['feature_1']:.4f}",
                'feature_2': f"{sample['feature_2']:.4f}",
                'feature_3': f"{sample['feature_3']:.4f}",
                'f1_squared': f"{sample['f1_squared']:.4f}",
                'f1_f2_interaction': f"{sample['f1_f2_interaction']:.4f}",
                'label': sample['label']
            })
    
    print(f"\n✓ Generated preprocessed_features.csv (first {sample_size} samples)")
    print(f"[{datetime.datetime.now()}] ML Data Preprocessing completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_ml_preprocessing())
