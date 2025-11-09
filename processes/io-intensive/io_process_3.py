#!/usr/bin/env python3
"""
IO Process 3: Log Processing and Text Analysis
Generates and processes large log files with pattern matching.
"""
import os
import time
import random
import psutil
import json
import re
from datetime import datetime
from collections import defaultdict

def generate_log_line():
    """Generate a realistic log line"""
    levels = ['INFO', 'WARN', 'ERROR', 'DEBUG']
    services = ['auth', 'api', 'database', 'cache', 'worker']
    messages = [
        'Request processed successfully',
        'Connection timeout',
        'Cache hit ratio: {:.2f}',
        'Database query took {}ms',
        'User authentication failed',
        'Service started on port {}'
    ]
    
    level = random.choice(levels)
    service = random.choice(services)
    message = random.choice(messages).format(random.randint(1, 1000))
    
    return f"{datetime.now().isoformat()} [{level}] {service}: {message}\n"

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 3: Log Processing")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "IO-Intensive-3",
        "log_lines_generated": 0,
        "log_lines_processed": 0,
        "errors_found": 0,
        "warnings_found": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    
    try:
        # Generate log files
        num_log_files = random.randint(5, 15)
        lines_per_file = random.randint(10000, 50000)
        
        print(f"Generating {num_log_files} log files with ~{lines_per_file} lines each...")
        
        log_files = []
        for i in range(num_log_files):
            log_file = f"{results_dir}/app_{i}.log"
            log_files.append(log_file)
            
            with open(log_file, 'w') as f:
                for _ in range(lines_per_file):
                    f.write(generate_log_line())
                    stats["log_lines_generated"] += 1
            
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            print(f"  Generated {log_file}: {lines_per_file} lines, Memory: {mem_mb:.2f}MB")
        
        # Process logs - find patterns
        print(f"\nProcessing log files for errors and warnings...")
        
        error_pattern = re.compile(r'\[ERROR\]')
        warn_pattern = re.compile(r'\[WARN\]')
        patterns = defaultdict(int)
        
        for log_file in log_files:
            print(f"  Processing {os.path.basename(log_file)}...")
            with open(log_file, 'r') as f:
                for line in f:
                    stats["log_lines_processed"] += 1
                    
                    if error_pattern.search(line):
                        stats["errors_found"] += 1
                    if warn_pattern.search(line):
                        stats["warnings_found"] += 1
                    
                    # Memory pressure simulation
                    if stats["log_lines_processed"] % 10000 == 0:
                        # Keep some data in memory
                        patterns[line[:50]] += 1
            
            # Periodic memory check
            mem_mb = process.memory_info().rss / (1024 * 1024)
            stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
            time.sleep(random.uniform(0.1, 0.3))
        
        # Generate summary report
        print(f"\nGenerating analysis report...")
        report_file = f"{results_dir}/log_analysis.txt"
        with open(report_file, 'w') as f:
            f.write(f"Log Analysis Report\n")
            f.write(f"{'='*60}\n")
            f.write(f"Total lines processed: {stats['log_lines_processed']}\n")
            f.write(f"Errors found: {stats['errors_found']}\n")
            f.write(f"Warnings found: {stats['warnings_found']}\n")
            f.write(f"Peak memory: {stats['peak_memory_mb']:.2f} MB\n")
        
        stats["duration_seconds"] = time.time() - start_time
        
        # Save stats
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Log lines generated: {stats['log_lines_generated']}")
        print(f"Log lines processed: {stats['log_lines_processed']}")
        print(f"Errors found: {stats['errors_found']}")
        print(f"Warnings found: {stats['warnings_found']}")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
