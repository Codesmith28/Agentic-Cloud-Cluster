#!/usr/bin/env python3
"""
Workload Type: memory-heavy
Description: Join multiple large datasets in memory
Resource Requirements: Moderate CPU, high memory
Output: None (logs only)
"""

import datetime
import random

def generate_users(count=500_000):
    """Generate user data"""
    print(f"  Generating {count:,} users...")
    users = []
    for i in range(count):
        users.append({
            'user_id': i,
            'name': f'User{i}',
            'age': random.randint(18, 80),
            'country': random.choice(['USA', 'UK', 'Germany', 'France', 'Japan'])
        })
        if (i + 1) % 100000 == 0:
            print(f"    Generated {i+1:,} users...")
    return users

def generate_purchases(count=2_000_000):
    """Generate purchase data"""
    print(f"  Generating {count:,} purchases...")
    purchases = []
    for i in range(count):
        purchases.append({
            'purchase_id': i,
            'user_id': random.randint(0, 499999),
            'amount': random.uniform(10, 1000),
            'product': f'Product{random.randint(1, 1000)}'
        })
        if (i + 1) % 400000 == 0:
            print(f"    Generated {i+1:,} purchases...")
    return purchases

def generate_reviews(count=1_000_000):
    """Generate review data"""
    print(f"  Generating {count:,} reviews...")
    reviews = []
    for i in range(count):
        reviews.append({
            'review_id': i,
            'user_id': random.randint(0, 499999),
            'rating': random.randint(1, 5),
            'product': f'Product{random.randint(1, 1000)}'
        })
        if (i + 1) % 200000 == 0:
            print(f"    Generated {i+1:,} reviews...")
    return reviews

def join_datasets():
    print(f"[{datetime.datetime.now()}] Starting Dataset Join...")
    
    # Generate datasets
    print("\nGenerating datasets:")
    users = generate_users()
    purchases = generate_purchases()
    reviews = generate_reviews()
    
    print(f"\n✓ Users: {len(users):,}")
    print(f"✓ Purchases: {len(purchases):,}")
    print(f"✓ Reviews: {len(reviews):,}")
    
    # Create indices for faster joins
    print("\nCreating indices...")
    user_index = {u['user_id']: u for u in users}
    print(f"✓ User index created")
    
    # Join purchases with users
    print("\nJoining purchases with users...")
    joined_data = []
    for i, purchase in enumerate(purchases):
        user = user_index.get(purchase['user_id'])
        if user:
            joined_data.append({
                **purchase,
                'user_name': user['name'],
                'user_age': user['age'],
                'user_country': user['country']
            })
        
        if (i + 1) % 400000 == 0:
            print(f"  Joined {i+1:,} purchases...")
    
    print(f"✓ Joined {len(joined_data):,} records")
    
    # Compute statistics
    print("\nComputing statistics...")
    total_amount = sum(r['amount'] for r in joined_data)
    by_country = {}
    
    for record in joined_data:
        country = record['user_country']
        if country not in by_country:
            by_country[country] = {'count': 0, 'total': 0}
        by_country[country]['count'] += 1
        by_country[country]['total'] += record['amount']
    
    print(f"\nPurchases by Country:")
    for country, stats in sorted(by_country.items(), key=lambda x: x[1]['total'], reverse=True):
        avg = stats['total'] / stats['count']
        print(f"  {country}: {stats['count']:,} purchases, ${stats['total']:,.2f} total, ${avg:.2f} avg")
    
    print(f"\n[{datetime.datetime.now()}] Dataset Join completed ✓")
    return 0

if __name__ == "__main__":
    exit(join_datasets())
