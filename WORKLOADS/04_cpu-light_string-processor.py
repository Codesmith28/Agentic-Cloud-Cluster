#!/usr/bin/env python3
"""
Workload Type: cpu-light
Description: Process strings and count word frequencies
Resource Requirements: Minimal CPU, minimal memory
Output: word_frequencies.csv (generated file)
"""

import datetime
import csv
from collections import Counter

def process_strings():
    print(f"[{datetime.datetime.now()}] Starting String Processor...")
    
    # Sample text
    text = """
    Cloud computing enables on-demand access to computing resources.
    Cloud infrastructure provides scalability and flexibility.
    Computing power can be scaled up or down based on demand.
    Modern cloud platforms offer various services for developers.
    """ * 20  # Repeat to make it more substantial
    
    # Process
    words = text.lower().split()
    word_count = len(words)
    unique_words = len(set(words))
    
    print(f"✓ Total words: {word_count}")
    print(f"✓ Unique words: {unique_words}")
    
    # Get top 10 most common words
    counter = Counter(words)
    top_words = counter.most_common(10)
    
    print("\nTop 10 Most Common Words:")
    for word, count in top_words:
        print(f"  {word}: {count}")
    
    # Generate CSV
    with open('/output/word_frequencies.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['Word', 'Frequency'])
        for word, count in top_words:
            writer.writerow([word, count])
    
    print(f"\n✓ Generated word_frequencies.csv")
    print(f"[{datetime.datetime.now()}] String Processor completed ✓")
    return 0

if __name__ == "__main__":
    exit(process_strings())
