#!/usr/bin/env python3
"""
Workload Type: gpu-inference
Description: Simulated sentiment analysis inference
Resource Requirements: Low CPU, moderate memory, GPU required
Output: sentiment_analysis.csv (generated file)
"""

import datetime
import random
import csv
import time

def simulate_sentiment_analysis(text):
    """Simulate GPU-based sentiment analysis"""
    time.sleep(0.005)  # Simulate GPU inference
    
    sentiments = ['positive', 'negative', 'neutral']
    sentiment = random.choice(sentiments)
    
    # Generate scores that sum to 1
    scores = {s: random.uniform(0, 1) for s in sentiments}
    total = sum(scores.values())
    scores = {k: v/total for k, v in scores.items()}
    
    # Boost the selected sentiment
    scores[sentiment] = max(scores[sentiment], 0.6)
    total = sum(scores.values())
    scores = {k: v/total for k, v in scores.items()}
    
    return sentiment, scores

def run_sentiment_analysis():
    print(f"[{datetime.datetime.now()}] Starting Sentiment Analysis (GPU Inference)...")
    print("🖥️  GPU Inference Mode")
    
    # Generate sample texts
    sample_texts = [
        "This product is amazing and works great!",
        "Terrible experience, would not recommend.",
        "It's okay, nothing special.",
        "Best purchase I've ever made!",
        "Complete waste of money.",
    ]
    
    # Replicate to create larger dataset
    texts = []
    for i in range(500):
        base_text = random.choice(sample_texts)
        texts.append(f"{base_text} (Review #{i})")
    
    print(f"\n✓ Analyzing {len(texts)} text samples")
    print(f"  Sentiment classes: positive, negative, neutral")
    
    results = []
    
    print("\nProcessing texts...")
    start = datetime.datetime.now()
    
    for i, text in enumerate(texts):
        sentiment, scores = simulate_sentiment_analysis(text)
        results.append({
            'text_id': i,
            'sentiment': sentiment,
            'positive_score': scores['positive'],
            'negative_score': scores['negative'],
            'neutral_score': scores['neutral']
        })
        
        if (i + 1) % 100 == 0:
            print(f"  Processed {i+1}/{len(texts)} texts...")
    
    end = datetime.datetime.now()
    duration = (end - start).total_seconds()
    throughput = len(texts) / duration
    
    print(f"\n✓ Analyzed {len(results)} texts")
    print(f"✓ Time: {duration:.2f}s")
    print(f"✓ Throughput: {throughput:.2f} texts/sec")
    
    # Count sentiments
    sentiment_counts = {'positive': 0, 'negative': 0, 'neutral': 0}
    for r in results:
        sentiment_counts[r['sentiment']] += 1
    
    print("\nSentiment Distribution:")
    for sentiment, count in sorted(sentiment_counts.items()):
        pct = count / len(results) * 100
        print(f"  {sentiment}: {count} ({pct:.1f}%)")
    
    # Average scores
    avg_positive = sum(r['positive_score'] for r in results) / len(results)
    avg_negative = sum(r['negative_score'] for r in results) / len(results)
    avg_neutral = sum(r['neutral_score'] for r in results) / len(results)
    
    print(f"\nAverage Scores:")
    print(f"  Positive: {avg_positive:.4f}")
    print(f"  Negative: {avg_negative:.4f}")
    print(f"  Neutral: {avg_neutral:.4f}")
    
    # Save results
    with open('/output/sentiment_analysis.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['text_id', 'sentiment', 'positive_score', 'negative_score', 'neutral_score'])
        writer.writeheader()
        for r in results:
            writer.writerow({
                'text_id': r['text_id'],
                'sentiment': r['sentiment'],
                'positive_score': f"{r['positive_score']:.4f}",
                'negative_score': f"{r['negative_score']:.4f}",
                'neutral_score': f"{r['neutral_score']:.4f}"
            })
    
    print(f"\n✓ Generated sentiment_analysis.csv")
    print(f"[{datetime.datetime.now()}] Sentiment Analysis completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_sentiment_analysis())
