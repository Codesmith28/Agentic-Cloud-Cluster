#!/usr/bin/env python3
"""
Workload Type: memory-heavy
Description: In-memory data aggregation and grouping operations
Resource Requirements: Moderate CPU, high memory
Output: aggregated_data.csv (generated file)
"""

import datetime
import random
import csv
from collections import defaultdict

def generate_transactions(count=2_000_000):
    """Generate transaction data"""
    print(f"  Generating {count:,} transactions...")
    
    categories = ['Electronics', 'Clothing', 'Food', 'Books', 'Sports', 'Home', 'Beauty', 'Toys']
    countries = ['USA', 'UK', 'Germany', 'France', 'Japan', 'Canada', 'Australia']
    
    transactions = []
    for i in range(count):
        trans = {
            'id': i,
            'amount': random.uniform(10, 1000),
            'category': random.choice(categories),
            'country': random.choice(countries),
            'year': random.randint(2020, 2024),
            'month': random.randint(1, 12)
        }
        transactions.append(trans)
        
        if (i + 1) % 400000 == 0:
            print(f"    Generated {i+1:,} transactions...")
    
    return transactions

def aggregate_data():
    print(f"[{datetime.datetime.now()}] Starting Data Aggregation...")
    
    # Generate data
    transactions = generate_transactions()
    print(f"\n✓ Generated {len(transactions):,} transactions")
    
    # Aggregate by category
    print("\nAggregating by category...")
    by_category = defaultdict(lambda: {'count': 0, 'total': 0})
    
    for trans in transactions:
        cat = trans['category']
        by_category[cat]['count'] += 1
        by_category[cat]['total'] += trans['amount']
    
    print("\nCategory Analysis:")
    for cat, data in sorted(by_category.items()):
        avg = data['total'] / data['count']
        print(f"  {cat}: {data['count']:,} transactions, ${data['total']:,.2f} total, ${avg:.2f} avg")
    
    # Aggregate by country
    print("\nAggregating by country...")
    by_country = defaultdict(lambda: {'count': 0, 'total': 0})
    
    for trans in transactions:
        country = trans['country']
        by_country[country]['count'] += 1
        by_country[country]['total'] += trans['amount']
    
    print("\nCountry Analysis:")
    for country, data in sorted(by_country.items(), key=lambda x: x[1]['total'], reverse=True):
        print(f"  {country}: {data['count']:,} transactions, ${data['total']:,.2f}")
    
    # Save aggregations to CSV
    with open('/output/aggregated_data.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Type', 'Name', 'Count', 'Total Amount', 'Average Amount'])
        
        for cat, data in sorted(by_category.items()):
            avg = data['total'] / data['count']
            writer.writerow(['Category', cat, data['count'], f"{data['total']:.2f}", f"{avg:.2f}"])
        
        for country, data in sorted(by_country.items()):
            avg = data['total'] / data['count']
            writer.writerow(['Country', country, data['count'], f"{data['total']:.2f}", f"{avg:.2f}"])
    
    print(f"\n✓ Generated aggregated_data.csv")
    print(f"[{datetime.datetime.now()}] Data Aggregation completed ✓")
    return 0

if __name__ == "__main__":
    exit(aggregate_data())
