#!/usr/bin/env python3
"""
Workload Type: mixed
Description: ETL pipeline with data extraction, transformation, and loading
Resource Requirements: Balanced CPU and memory
Output: None (logs only)
"""

import datetime
import random
import time

def extract_data(num_records=10000):
    """Simulate data extraction from multiple sources"""
    print(f"\n[EXTRACT] Starting data extraction...")
    time.sleep(1.0)
    
    data = []
    for i in range(num_records):
        record = {
            'id': i,
            'timestamp': datetime.datetime.now().isoformat(),
            'value': random.uniform(0, 1000),
            'category': random.choice(['A', 'B', 'C', 'D']),
            'status': random.choice(['active', 'inactive', 'pending'])
        }
        data.append(record)
        
        if (i + 1) % 2000 == 0:
            print(f"  Extracted {i+1:,}/{num_records:,} records...")
    
    print(f"[EXTRACT] ✓ Extracted {len(data):,} records")
    return data

def transform_data(data):
    """Simulate data transformation with filtering and aggregation"""
    print(f"\n[TRANSFORM] Starting data transformation...")
    time.sleep(1.5)
    
    # Filter invalid records
    print(f"  Filtering records...")
    valid_data = [r for r in data if r['value'] > 100]
    print(f"  ✓ Filtered: {len(data):,} -> {len(valid_data):,} records")
    
    # Enrich data
    print(f"  Enriching records...")
    for record in valid_data:
        record['value_category'] = 'high' if record['value'] > 700 else 'medium' if record['value'] > 400 else 'low'
        record['processed_timestamp'] = datetime.datetime.now().isoformat()
    
    time.sleep(1.0)
    print(f"  ✓ Enriched {len(valid_data):,} records")
    
    # Aggregate by category
    print(f"  Aggregating data...")
    aggregations = {}
    for record in valid_data:
        cat = record['category']
        if cat not in aggregations:
            aggregations[cat] = {'count': 0, 'sum': 0, 'avg': 0}
        aggregations[cat]['count'] += 1
        aggregations[cat]['sum'] += record['value']
    
    for cat in aggregations:
        aggregations[cat]['avg'] = aggregations[cat]['sum'] / aggregations[cat]['count']
    
    print(f"  ✓ Generated aggregations for {len(aggregations)} categories")
    
    print(f"[TRANSFORM] ✓ Transformation complete")
    return valid_data, aggregations

def load_data(data, aggregations):
    """Simulate loading data to target systems"""
    print(f"\n[LOAD] Starting data load...")
    
    # Simulate batch writes
    batch_size = 1000
    num_batches = (len(data) + batch_size - 1) // batch_size
    
    print(f"  Loading {len(data):,} records in {num_batches} batches...")
    for i in range(num_batches):
        time.sleep(0.3)
        start_idx = i * batch_size
        end_idx = min(start_idx + batch_size, len(data))
        print(f"  Batch {i+1}/{num_batches}: Loaded records {start_idx}-{end_idx}")
    
    print(f"  ✓ Loaded {len(data):,} records")
    
    # Load aggregations
    print(f"  Loading aggregations...")
    time.sleep(0.5)
    print(f"  ✓ Loaded {len(aggregations)} aggregations")
    
    print(f"[LOAD] ✓ Load complete")

def run_etl_pipeline():
    print(f"[{datetime.datetime.now()}] Starting ETL Pipeline (Mixed Workload)...")
    print("⚖️  Mixed Mode - Balanced CPU & Memory")
    
    print(f"\n{'='*60}")
    print(f"ETL Pipeline Configuration")
    print(f"{'='*60}")
    print(f"✓ Pipeline: Extract -> Transform -> Load")
    print(f"✓ Source: Multiple data sources (simulated)")
    print(f"✓ Target: Data warehouse (simulated)")
    print(f"✓ Records: 10,000")
    
    start_time = datetime.datetime.now()
    
    # Extract
    data = extract_data(10000)
    
    # Transform
    transformed_data, aggregations = transform_data(data)
    
    # Load
    load_data(transformed_data, aggregations)
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    print(f"\n{'='*60}")
    print(f"ETL Pipeline Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s")
    print(f"Records Processed: {len(data):,}")
    print(f"Records Loaded: {len(transformed_data):,}")
    print(f"Throughput: {len(data)/duration:.0f} records/sec")
    
    print(f"\nAggregation Summary:")
    for cat in sorted(aggregations.keys()):
        agg = aggregations[cat]
        print(f"  Category {cat}: Count={agg['count']}, Avg Value={agg['avg']:.2f}")
    
    print(f"\n[{datetime.datetime.now()}] ETL Pipeline completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_etl_pipeline())
