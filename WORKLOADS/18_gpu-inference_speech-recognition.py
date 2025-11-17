#!/usr/bin/env python3
"""
Workload Type: gpu-inference
Description: Simulated speech-to-text inference
Resource Requirements: Low CPU, moderate memory, GPU required
Output: transcriptions.csv (generated file)
"""

import datetime
import random
import csv
import time

def simulate_speech_recognition(audio_duration_seconds):
    """Simulate GPU-based speech recognition"""
    time.sleep(audio_duration_seconds * 0.1)  # Simulate GPU processing
    
    words = ['hello', 'world', 'machine', 'learning', 'artificial', 'intelligence', 
             'speech', 'recognition', 'system', 'processing']
    
    num_words = int(audio_duration_seconds * 2)  # ~2 words per second
    transcription = ' '.join(random.choice(words) for _ in range(num_words))
    confidence = random.uniform(0.85, 0.99)
    
    return transcription, confidence

def run_speech_recognition():
    print(f"[{datetime.datetime.now()}] Starting Speech Recognition (GPU Inference)...")
    print("🖥️  GPU Inference Mode")
    
    # Simulate multiple audio files
    audio_files = [
        {'id': i, 'duration': random.uniform(5, 30)}
        for i in range(100)
    ]
    
    total_duration = sum(f['duration'] for f in audio_files)
    
    print(f"\n✓ Processing {len(audio_files)} audio files")
    print(f"✓ Total audio duration: {total_duration:.1f}s")
    
    results = []
    
    print("\nTranscribing audio files...")
    start = datetime.datetime.now()
    
    for i, audio in enumerate(audio_files):
        transcription, confidence = simulate_speech_recognition(audio['duration'])
        results.append({
            'file_id': audio['id'],
            'duration': audio['duration'],
            'transcription': transcription,
            'confidence': confidence,
            'word_count': len(transcription.split())
        })
        
        if (i + 1) % 20 == 0:
            print(f"  Processed {i+1}/{len(audio_files)} files...")
    
    end = datetime.datetime.now()
    processing_time = (end - start).total_seconds()
    real_time_factor = total_duration / processing_time
    
    print(f"\n✓ Transcribed {len(results)} audio files")
    print(f"✓ Processing time: {processing_time:.2f}s")
    print(f"✓ Real-time factor: {real_time_factor:.2f}x")
    
    # Statistics
    avg_confidence = sum(r['confidence'] for r in results) / len(results)
    total_words = sum(r['word_count'] for r in results)
    
    print(f"✓ Average confidence: {avg_confidence:.4f}")
    print(f"✓ Total words transcribed: {total_words}")
    
    # Save results
    with open('/output/transcriptions.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['file_id', 'duration', 'word_count', 'confidence'])
        writer.writeheader()
        for r in results:
            writer.writerow({
                'file_id': r['file_id'],
                'duration': f"{r['duration']:.2f}",
                'word_count': r['word_count'],
                'confidence': f"{r['confidence']:.4f}"
            })
    
    print(f"✓ Generated transcriptions.csv")
    print(f"\n[{datetime.datetime.now()}] Speech Recognition completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_speech_recognition())
