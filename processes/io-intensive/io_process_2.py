#!/usr/bin/env python3
"""
IO Process 2: Database-like Operations with Random Access
Simulates database I/O patterns with many small reads/writes.
"""
import os
import time
import random
import psutil
import json
import sqlite3
from datetime import datetime

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 2: Database Operations")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    db_path = f"{results_dir}/test.db"
    
    stats = {
        "process_type": "IO-Intensive-2",
        "records_inserted": 0,
        "records_queried": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Create database and populate
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()
        
        cursor.execute('''
            CREATE TABLE data (
                id INTEGER PRIMARY KEY,
                value TEXT,
                timestamp REAL,
                metadata TEXT
            )
        ''')
        
        # Insert random records
        num_records = random.randint(5000, 15000)
        print(f"Inserting {num_records} records...")
        
        batch_size = 100
        for i in range(0, num_records, batch_size):
            batch = []
            for j in range(batch_size):
                record = (
                    i + j,
                    ''.join(random.choices('abcdefghijklmnopqrstuvwxyz', k=random.randint(50, 200))),
                    time.time(),
                    json.dumps({"index": i+j, "random": random.random()})
                )
                batch.append(record)
            
            cursor.executemany('INSERT INTO data VALUES (?, ?, ?, ?)', batch)
            conn.commit()
            stats["records_inserted"] += len(batch)
            
            if i % 1000 == 0:
                mem_mb = process.memory_info().rss / (1024 * 1024)
                stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                print(f"  Inserted {stats['records_inserted']} records, Memory: {mem_mb:.2f}MB")
        
        print(f"\nPerforming random queries...")
        # Random queries
        for _ in range(random.randint(500, 1500)):
            query_type = random.choice(['select', 'range', 'aggregate'])
            
            if query_type == 'select':
                rid = random.randint(0, num_records - 1)
                cursor.execute('SELECT * FROM data WHERE id = ?', (rid,))
                cursor.fetchall()
            elif query_type == 'range':
                start = random.randint(0, num_records - 1000)
                cursor.execute('SELECT * FROM data WHERE id BETWEEN ? AND ?', (start, start + 100))
                cursor.fetchall()
            else:
                cursor.execute('SELECT COUNT(*), AVG(timestamp) FROM data')
                cursor.fetchall()
            
            stats["records_queried"] += 1
            
            if stats["records_queried"] % 200 == 0:
                print(f"  Queries executed: {stats['records_queried']}")
        
        conn.close()
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save results
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Records inserted: {stats['records_inserted']}")
        print(f"Queries executed: {stats['records_queried']}")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
