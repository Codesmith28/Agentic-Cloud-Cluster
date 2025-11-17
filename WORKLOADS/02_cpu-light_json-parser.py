#!/usr/bin/env python3
"""
Workload Type: cpu-light
Description: Parse small JSON data and generate summary CSV
Resource Requirements: Minimal CPU, minimal memory
Output: summary.csv (generated file)
"""

import json
import csv
import datetime

def generate_sample_data():
    """Generate sample JSON data"""
    return [
        {"id": i, "name": f"User{i}", "score": i * 10, "active": i % 2 == 0}
        for i in range(100)
    ]

def parse_and_summarize():
    print(f"[{datetime.datetime.now()}] Starting JSON Parser...")
    
    # Generate data
    data = generate_sample_data()
    print(f"✓ Generated {len(data)} records")
    
    # Parse and compute summary
    total_score = sum(item['score'] for item in data)
    active_users = sum(1 for item in data if item['active'])
    avg_score = total_score / len(data)
    
    print(f"✓ Total Score: {total_score}")
    print(f"✓ Active Users: {active_users}")
    print(f"✓ Average Score: {avg_score:.2f}")
    
    # Generate CSV output
    with open('/output/summary.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Metric', 'Value'])
        writer.writerow(['Total Records', len(data)])
        writer.writerow(['Total Score', total_score])
        writer.writerow(['Active Users', active_users])
        writer.writerow(['Average Score', f'{avg_score:.2f}'])
    
    print(f"✓ Generated summary.csv")
    print(f"[{datetime.datetime.now()}] JSON Parser completed ✓")
    return 0

if __name__ == "__main__":
    exit(parse_and_summarize())
