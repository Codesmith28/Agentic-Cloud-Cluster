#!/usr/bin/env python3
"""
Workload Type: memory-heavy
Description: Process large datasets and merge multiple data structures
Resource Requirements: Moderate CPU, high memory
Output: merged_dataset.csv (generated file)
"""

import datetime
import random
import csv

def generate_large_dataset(rows=1_000_000):
    """Generate a large dataset in memory"""
    print(f"  Generating dataset with {rows:,} rows...")
    
    dataset = []
    for i in range(rows):
        record = {
            'id': i,
            'name': f'User{i}',
            'age': random.randint(18, 80),
            'score': random.uniform(0, 100),
            'city': random.choice(['NYC', 'LA', 'Chicago', 'Houston', 'Phoenix']),
            'active': random.choice([True, False])
        }
        dataset.append(record)
        
        if (i + 1) % 200000 == 0:
            print(f"    Generated {i+1:,} rows...")
    
    return dataset

def merge_datasets():
    print(f"[{datetime.datetime.now()}] Starting Dataset Merge...")
    
    # Generate multiple datasets
    print("\nGenerating datasets:")
    dataset1 = generate_large_dataset(500_000)
    dataset2 = generate_large_dataset(500_000)
    dataset3 = generate_large_dataset(500_000)
    
    print(f"\n✓ Generated {len(dataset1):,} + {len(dataset2):,} + {len(dataset3):,} records")
    
    # Merge datasets
    print("\nMerging datasets...")
    merged = dataset1 + dataset2 + dataset3
    print(f"✓ Merged total: {len(merged):,} records")
    
    # Compute statistics
    print("\nComputing statistics...")
    total_score = sum(r['score'] for r in merged)
    avg_age = sum(r['age'] for r in merged) / len(merged)
    active_count = sum(1 for r in merged if r['active'])
    
    print(f"  Total Score: {total_score:,.2f}")
    print(f"  Average Age: {avg_age:.2f}")
    print(f"  Active Users: {active_count:,} ({active_count/len(merged)*100:.1f}%)")
    
    # Group by city
    city_counts = {}
    for record in merged:
        city = record['city']
        city_counts[city] = city_counts.get(city, 0) + 1
    
    print("\nRecords by City:")
    for city, count in sorted(city_counts.items()):
        print(f"  {city}: {count:,}")
    
    # Save summary to CSV
    with open('/output/merged_dataset.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Metric', 'Value'])
        writer.writerow(['Total Records', len(merged)])
        writer.writerow(['Total Score', f'{total_score:.2f}'])
        writer.writerow(['Average Age', f'{avg_age:.2f}'])
        writer.writerow(['Active Users', active_count])
        writer.writerow(['NYC', city_counts.get('NYC', 0)])
        writer.writerow(['LA', city_counts.get('LA', 0)])
        writer.writerow(['Chicago', city_counts.get('Chicago', 0)])
        writer.writerow(['Houston', city_counts.get('Houston', 0)])
        writer.writerow(['Phoenix', city_counts.get('Phoenix', 0)])
    
    print(f"\n✓ Generated merged_dataset.csv")
    print(f"[{datetime.datetime.now()}] Dataset Merge completed ✓")
    return 0

if __name__ == "__main__":
    exit(merge_datasets())
