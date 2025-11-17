#!/usr/bin/env python3
"""
Workload Type: gpu-inference
Description: Simulated object detection in images
Resource Requirements: Low CPU, moderate memory, GPU required
Output: detections.csv (generated file)
"""

import datetime
import random
import csv
import time

def simulate_object_detection(image):
    """Simulate GPU-based object detection"""
    time.sleep(0.03)  # Simulate GPU inference
    
    objects = ['person', 'car', 'bicycle', 'dog', 'cat', 'traffic_light', 'stop_sign']
    num_objects = random.randint(1, 5)
    
    detections = []
    for _ in range(num_objects):
        obj = random.choice(objects)
        confidence = random.uniform(0.7, 0.99)
        bbox = {
            'x': random.randint(0, 640),
            'y': random.randint(0, 480),
            'width': random.randint(50, 200),
            'height': random.randint(50, 200)
        }
        detections.append({
            'object': obj,
            'confidence': confidence,
            'bbox': bbox
        })
    
    return detections

def run_object_detection():
    print(f"[{datetime.datetime.now()}] Starting Object Detection (GPU Inference)...")
    print("🖥️  GPU Inference Mode")
    
    num_images = 500
    
    print(f"\n✓ Processing {num_images} images")
    print(f"  Detection classes: 7")
    
    all_detections = []
    
    print("\nDetecting objects...")
    start = datetime.datetime.now()
    
    for i in range(num_images):
        detections = simulate_object_detection(i)
        all_detections.extend([(i, d) for d in detections])
        
        if (i + 1) % 100 == 0:
            print(f"  Processed {i+1}/{num_images} images...")
    
    end = datetime.datetime.now()
    duration = (end - start).total_seconds()
    throughput = num_images / duration
    
    print(f"\n✓ Detected {len(all_detections)} objects in {num_images} images")
    print(f"✓ Time: {duration:.2f}s")
    print(f"✓ Throughput: {throughput:.2f} images/sec")
    print(f"✓ Avg objects per image: {len(all_detections)/num_images:.2f}")
    
    # Count objects
    object_counts = {}
    for _, det in all_detections:
        obj = det['object']
        object_counts[obj] = object_counts.get(obj, 0) + 1
    
    print("\nObject Distribution:")
    for obj, count in sorted(object_counts.items(), key=lambda x: x[1], reverse=True):
        print(f"  {obj}: {count}")
    
    # Confidence statistics
    confidences = [det['confidence'] for _, det in all_detections]
    avg_conf = sum(confidences) / len(confidences)
    min_conf = min(confidences)
    max_conf = max(confidences)
    
    print(f"\nConfidence Statistics:")
    print(f"  Average: {avg_conf:.4f}")
    print(f"  Min: {min_conf:.4f}")
    print(f"  Max: {max_conf:.4f}")
    
    # Save results
    with open('/output/detections.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['image_id', 'object', 'confidence', 'bbox_x', 'bbox_y', 'bbox_width', 'bbox_height'])
        for img_id, det in all_detections[:100]:  # Save first 100
            writer.writerow([
                img_id,
                det['object'],
                f"{det['confidence']:.4f}",
                det['bbox']['x'],
                det['bbox']['y'],
                det['bbox']['width'],
                det['bbox']['height']
            ])
    
    print(f"\n✓ Generated detections.csv (sample)")
    print(f"[{datetime.datetime.now()}] Object Detection completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_object_detection())
