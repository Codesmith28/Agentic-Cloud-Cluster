#!/usr/bin/env python3
"""
IO Process 7: CSV Data Processing with Aggregations
Simulates data analytics workload with large CSV files.
"""
import os
import time
import random
import psutil
import json
import csv
from datetime import datetime
from collections import defaultdict

def generate_csv_row(row_id):
    """Generate a realistic CSV data row"""
    categories = ['Electronics', 'Clothing', 'Food', 'Books', 'Toys', 'Sports']
    regions = ['North', 'South', 'East', 'West', 'Central']
    
    return {
        'id': row_id,
        'timestamp': datetime.now().isoformat(),
        'category': random.choice(categories),
        'region': random.choice(regions),
        'quantity': random.randint(1, 100),
        'price': round(random.uniform(10.0, 1000.0), 2),
        'customer_id': random.randint(1000, 9999),
        'status': random.choice(['completed', 'pending', 'cancelled'])
    }

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 7: CSV Data Processing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "IO-Intensive-7",
        "rows_generated": 0,
        "rows_processed": 0,
        "files_created": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        num_files = random.randint(5, 12)
        rows_per_file = random.randint(50000, 200000)
        
        print(f"Generating {num_files} CSV files with ~{rows_per_file} rows each...")
        
        csv_files = []
        
        # Phase 1: Generate CSV files
        for i in range(num_files):
            filename = f"{results_dir}/data_{i}.csv"
            csv_files.append(filename)
            
            with open(filename, 'w', newline='') as f:
                fieldnames = ['id', 'timestamp', 'category', 'region', 'quantity', 'price', 'customer_id', 'status']
                writer = csv.DictWriter(f, fieldnames=fieldnames)
                writer.writeheader()
                
                for row_id in range(rows_per_file):
                    row = generate_csv_row(stats["rows_generated"])
                    writer.writerow(row)
                    stats["rows_generated"] += 1
                    
                    # Periodic memory tracking
                    if stats["rows_generated"] % 50000 == 0:
                        mem_mb = process.memory_info().rss / (1024 * 1024)
                        stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            
            stats["files_created"] += 1
            mem_mb = process.memory_info().rss / (1024 * 1024)
            print(f"  Generated {filename}: {rows_per_file} rows, Memory: {mem_mb:.2f}MB")
            time.sleep(random.uniform(0.1, 0.3))
        
        # Phase 2: Aggregate data
        print(f"\nPhase 2: Processing and aggregating data...")
        
        aggregations = {
            'total_revenue': 0.0,
            'category_sales': defaultdict(float),
            'region_sales': defaultdict(float),
            'status_counts': defaultdict(int),
            'customer_purchases': defaultdict(int)
        }
        
        for filename in csv_files:
            print(f"  Processing {os.path.basename(filename)}...")
            
            with open(filename, 'r') as f:
                reader = csv.DictReader(f)
                
                for row in reader:
                    stats["rows_processed"] += 1
                    
                    # Perform aggregations
                    revenue = float(row['price']) * int(row['quantity'])
                    aggregations['total_revenue'] += revenue
                    aggregations['category_sales'][row['category']] += revenue
                    aggregations['region_sales'][row['region']] += revenue
                    aggregations['status_counts'][row['status']] += 1
                    aggregations['customer_purchases'][row['customer_id']] += 1
                    
                    # Memory check
                    if stats["rows_processed"] % 100000 == 0:
                        mem_mb = process.memory_info().rss / (1024 * 1024)
                        stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                        print(f"    Processed {stats['rows_processed']} rows, Memory: {mem_mb:.2f}MB")
            
            time.sleep(random.uniform(0.05, 0.2))
        
        # Phase 3: Generate reports
        print(f"\nPhase 3: Generating analysis reports...")
        
        # Summary report
        with open(f"{results_dir}/summary_report.txt", 'w') as f:
            f.write("Data Analysis Summary Report\n")
            f.write("="*60 + "\n\n")
            f.write(f"Total Revenue: ${aggregations['total_revenue']:,.2f}\n\n")
            
            f.write("Sales by Category:\n")
            for category, amount in sorted(aggregations['category_sales'].items(), key=lambda x: x[1], reverse=True):
                f.write(f"  {category}: ${amount:,.2f}\n")
            
            f.write("\nSales by Region:\n")
            for region, amount in sorted(aggregations['region_sales'].items(), key=lambda x: x[1], reverse=True):
                f.write(f"  {region}: ${amount:,.2f}\n")
            
            f.write("\nStatus Distribution:\n")
            for status, count in aggregations['status_counts'].items():
                f.write(f"  {status}: {count:,}\n")
        
        # Top customers report
        with open(f"{results_dir}/top_customers.csv", 'w', newline='') as f:
            writer = csv.writer(f)
            writer.writerow(['Customer ID', 'Purchase Count'])
            
            top_customers = sorted(aggregations['customer_purchases'].items(), 
                                 key=lambda x: x[1], reverse=True)[:100]
            
            for customer_id, count in top_customers:
                writer.writerow([customer_id, count])
        
        stats["duration_seconds"] = time.time() - start_time
        stats["aggregations"] = {
            'total_revenue': aggregations['total_revenue'],
            'unique_customers': len(aggregations['customer_purchases'])
        }
        
        # Save stats
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Rows generated: {stats['rows_generated']}")
        print(f"Rows processed: {stats['rows_processed']}")
        print(f"Total revenue: ${aggregations['total_revenue']:,.2f}")
        print(f"Unique customers: {len(aggregations['customer_purchases'])}")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
