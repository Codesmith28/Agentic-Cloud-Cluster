#!/usr/bin/env python3
"""
Workload Type: mixed
Description: Data validation and quality checks
Resource Requirements: Balanced CPU and memory
Output: validation_report.csv (generated file)
"""

import datetime
import random
import csv
import time

def generate_dataset(num_records=10000):
    """Generate a dataset with various data quality issues"""
    print(f"\n[GENERATE] Generating dataset with {num_records:,} records...")
    time.sleep(0.5)
    
    data = []
    for i in range(num_records):
        record = {
            'id': i,
            'name': f'Record_{i}' if random.random() > 0.02 else None,  # 2% missing
            'email': f'user{i}@example.com' if random.random() > 0.03 else 'invalid',  # 3% invalid
            'age': random.randint(18, 80) if random.random() > 0.01 else random.choice([-5, 150]),  # 1% outliers
            'income': round(random.uniform(20000, 150000), 2) if random.random() > 0.02 else None,  # 2% missing
            'city': random.choice(['NYC', 'LA', 'Chicago', 'Houston', 'Phoenix']) if random.random() > 0.015 else '',  # 1.5% missing
            'score': round(random.uniform(0, 100), 2) if random.random() > 0.025 else random.uniform(-10, 110)  # 2.5% out of range
        }
        data.append(record)
    
    print(f"[GENERATE] ✓ Generated {len(data):,} records")
    return data

def check_missing_values(data):
    """Check for missing values"""
    print(f"\n[CHECK] Checking for missing values...")
    time.sleep(0.8)
    
    fields = ['name', 'email', 'age', 'income', 'city', 'score']
    missing_counts = {}
    
    for field in fields:
        count = sum(1 for record in data if record[field] is None or record[field] == '')
        missing_counts[field] = count
        if count > 0:
            print(f"  ⚠️  {field}: {count} missing values ({count/len(data)*100:.2f}%)")
    
    total_missing = sum(missing_counts.values())
    print(f"[CHECK] Total missing values: {total_missing}")
    
    return missing_counts

def check_duplicates(data):
    """Check for duplicate records"""
    print(f"\n[CHECK] Checking for duplicates...")
    time.sleep(0.6)
    
    # Check for duplicate IDs
    ids = [record['id'] for record in data]
    duplicate_ids = len(ids) - len(set(ids))
    
    # Check for duplicate emails
    emails = [record['email'] for record in data if record['email'] not in [None, '', 'invalid']]
    duplicate_emails = len(emails) - len(set(emails))
    
    print(f"  Duplicate IDs: {duplicate_ids}")
    print(f"  Duplicate Emails: {duplicate_emails}")
    
    return {'ids': duplicate_ids, 'emails': duplicate_emails}

def check_data_types(data):
    """Validate data types"""
    print(f"\n[CHECK] Validating data types...")
    time.sleep(0.7)
    
    type_errors = {
        'age': 0,
        'income': 0,
        'score': 0
    }
    
    for record in data:
        if not isinstance(record.get('age'), (int, type(None))):
            type_errors['age'] += 1
        if not isinstance(record.get('income'), (float, type(None))):
            type_errors['income'] += 1
        if not isinstance(record.get('score'), (float, type(None))):
            type_errors['score'] += 1
    
    for field, count in type_errors.items():
        if count > 0:
            print(f"  ⚠️  {field}: {count} type errors")
    
    total_type_errors = sum(type_errors.values())
    print(f"[CHECK] Total type errors: {total_type_errors}")
    
    return type_errors

def check_value_ranges(data):
    """Check for values outside valid ranges"""
    print(f"\n[CHECK] Checking value ranges...")
    time.sleep(0.9)
    
    range_violations = {
        'age': 0,
        'score': 0,
        'email': 0
    }
    
    for record in data:
        # Age should be 0-120
        age = record.get('age')
        if age is not None and (age < 0 or age > 120):
            range_violations['age'] += 1
        
        # Score should be 0-100
        score = record.get('score')
        if score is not None and (score < 0 or score > 100):
            range_violations['score'] += 1
        
        # Email format validation (simple check)
        email = record.get('email')
        if email and email != 'invalid' and '@' not in email:
            range_violations['email'] += 1
    
    for field, count in range_violations.items():
        if count > 0:
            print(f"  ⚠️  {field}: {count} out-of-range values ({count/len(data)*100:.2f}%)")
    
    total_violations = sum(range_violations.values())
    print(f"[CHECK] Total range violations: {total_violations}")
    
    return range_violations

def check_consistency(data):
    """Check for logical consistency issues"""
    print(f"\n[CHECK] Checking data consistency...")
    time.sleep(0.5)
    
    consistency_issues = 0
    
    # Example: Check if income is suspiciously low for certain cities
    for record in data:
        income = record.get('income')
        city = record.get('city')
        
        if income and city == 'NYC' and income < 30000:
            consistency_issues += 1
    
    print(f"  Suspicious patterns found: {consistency_issues}")
    print(f"[CHECK] Consistency issues: {consistency_issues}")
    
    return consistency_issues

def run_data_validation():
    print(f"[{datetime.datetime.now()}] Starting Data Validation (Mixed Workload)...")
    print("⚖️  Mixed Mode - Balanced CPU & Memory")
    
    print(f"\n{'='*60}")
    print(f"Data Validation Pipeline")
    print(f"{'='*60}")
    print(f"✓ Checks: Missing Values, Duplicates, Types, Ranges, Consistency")
    print(f"✓ Records: 10,000")
    
    start_time = datetime.datetime.now()
    
    # Generate dataset
    data = generate_dataset(10000)
    
    # Run validation checks
    missing = check_missing_values(data)
    duplicates = check_duplicates(data)
    type_errors = check_data_types(data)
    range_violations = check_value_ranges(data)
    consistency = check_consistency(data)
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    # Calculate data quality score
    total_records = len(data)
    total_fields = total_records * 6  # 6 fields per record
    
    total_issues = (sum(missing.values()) + 
                   duplicates['ids'] + duplicates['emails'] +
                   sum(type_errors.values()) +
                   sum(range_violations.values()) +
                   consistency)
    
    quality_score = (1 - total_issues / total_fields) * 100
    
    print(f"\n{'='*60}")
    print(f"Data Validation Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s")
    print(f"Records Validated: {total_records:,}")
    print(f"Total Issues Found: {total_issues:,}")
    print(f"Data Quality Score: {quality_score:.2f}%")
    
    print(f"\nIssue Breakdown:")
    print(f"  Missing Values: {sum(missing.values())}")
    print(f"  Duplicates: {duplicates['ids'] + duplicates['emails']}")
    print(f"  Type Errors: {sum(type_errors.values())}")
    print(f"  Range Violations: {sum(range_violations.values())}")
    print(f"  Consistency Issues: {consistency}")
    
    # Save validation report
    with open('/output/validation_report.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Check Type', 'Field', 'Issues Found', 'Percentage'])
        
        for field, count in missing.items():
            writer.writerow(['Missing Values', field, count, f"{count/total_records*100:.2f}%"])
        
        writer.writerow(['Duplicates', 'IDs', duplicates['ids'], f"{duplicates['ids']/total_records*100:.2f}%"])
        writer.writerow(['Duplicates', 'Emails', duplicates['emails'], f"{duplicates['emails']/total_records*100:.2f}%"])
        
        for field, count in range_violations.items():
            writer.writerow(['Range Violations', field, count, f"{count/total_records*100:.2f}%"])
        
        writer.writerow(['Consistency', 'All Fields', consistency, f"{consistency/total_records*100:.2f}%"])
        
        writer.writerow([''])
        writer.writerow(['Summary', '', '', ''])
        writer.writerow(['Total Records', total_records, '', ''])
        writer.writerow(['Total Issues', total_issues, '', ''])
        writer.writerow(['Quality Score', f"{quality_score:.2f}%", '', ''])
    
    print(f"\n✓ Generated validation_report.csv")
    print(f"[{datetime.datetime.now()}] Data Validation completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_data_validation())
