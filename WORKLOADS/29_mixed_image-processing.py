#!/usr/bin/env python3
"""
Workload Type: mixed
Description: Batch image processing with transformations
Resource Requirements: Balanced CPU and memory
Output: processing_report.csv (generated file)
"""

import datetime
import random
import csv
import time

def load_image_batch(batch_num, batch_size=50):
    """Simulate loading a batch of images"""
    images = []
    for i in range(batch_size):
        image = {
            'id': f'img_{batch_num}_{i}',
            'width': random.choice([1920, 2560, 3840]),
            'height': random.choice([1080, 1440, 2160]),
            'format': random.choice(['jpg', 'png', 'bmp']),
            'size_mb': random.uniform(1.0, 10.0)
        }
        images.append(image)
    return images

def resize_images(images, target_width=1024, target_height=768):
    """Simulate image resizing"""
    time.sleep(0.5)
    
    for img in images:
        img['original_width'] = img['width']
        img['original_height'] = img['height']
        img['width'] = target_width
        img['height'] = target_height
        img['resized'] = True

def apply_filters(images):
    """Simulate applying image filters"""
    time.sleep(0.8)
    
    filters = ['sharpen', 'blur', 'contrast', 'brightness', 'saturation']
    for img in images:
        img['filters_applied'] = random.sample(filters, k=random.randint(1, 3))

def compress_images(images, quality=85):
    """Simulate image compression"""
    time.sleep(0.6)
    
    for img in images:
        compression_ratio = random.uniform(0.3, 0.7)
        img['original_size_mb'] = img['size_mb']
        img['size_mb'] = img['size_mb'] * compression_ratio
        img['quality'] = quality
        img['compressed'] = True

def generate_thumbnails(images, thumb_size=256):
    """Simulate thumbnail generation"""
    time.sleep(0.4)
    
    for img in images:
        img['thumbnail_size'] = thumb_size
        img['thumbnail_mb'] = img['size_mb'] * 0.05

def extract_metadata(images):
    """Simulate metadata extraction"""
    time.sleep(0.3)
    
    for img in images:
        img['metadata'] = {
            'created': datetime.datetime.now().isoformat(),
            'aspect_ratio': f"{img['width']}:{img['height']}",
            'processed': True
        }

def run_image_processing():
    print(f"[{datetime.datetime.now()}] Starting Batch Image Processing (Mixed Workload)...")
    print("⚖️  Mixed Mode - Balanced CPU & Memory")
    
    num_batches = 10
    batch_size = 50
    total_images = num_batches * batch_size
    
    print(f"\n{'='*60}")
    print(f"Image Processing Pipeline")
    print(f"{'='*60}")
    print(f"✓ Pipeline: Load -> Resize -> Filter -> Compress -> Thumbnail -> Metadata")
    print(f"✓ Total Images: {total_images}")
    print(f"✓ Batch Size: {batch_size}")
    print(f"✓ Batches: {num_batches}")
    
    all_images = []
    processing_report = []
    
    start_time = datetime.datetime.now()
    
    for batch_num in range(1, num_batches + 1):
        print(f"\n[BATCH {batch_num}/{num_batches}] Processing batch...")
        
        batch_start = datetime.datetime.now()
        
        # Load images
        print(f"  Loading images...")
        images = load_image_batch(batch_num, batch_size)
        
        original_size = sum(img['size_mb'] for img in images)
        
        # Process images
        print(f"  Resizing to 1024x768...")
        resize_images(images)
        
        print(f"  Applying filters...")
        apply_filters(images)
        
        print(f"  Compressing (quality=85)...")
        compress_images(images, quality=85)
        
        print(f"  Generating thumbnails...")
        generate_thumbnails(images)
        
        print(f"  Extracting metadata...")
        extract_metadata(images)
        
        batch_end = datetime.datetime.now()
        batch_duration = (batch_end - batch_start).total_seconds()
        
        final_size = sum(img['size_mb'] for img in images)
        compression_ratio = (1 - final_size / original_size) * 100
        
        print(f"  ✓ Batch {batch_num} complete in {batch_duration:.2f}s")
        print(f"    Original: {original_size:.2f} MB -> Final: {final_size:.2f} MB ({compression_ratio:.1f}% reduction)")
        
        all_images.extend(images)
        
        processing_report.append({
            'batch': batch_num,
            'images': len(images),
            'duration': batch_duration,
            'original_size_mb': original_size,
            'final_size_mb': final_size,
            'compression_pct': compression_ratio
        })
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    # Calculate overall statistics
    total_original_size = sum(r['original_size_mb'] for r in processing_report)
    total_final_size = sum(r['final_size_mb'] for r in processing_report)
    overall_compression = (1 - total_final_size / total_original_size) * 100
    
    print(f"\n{'='*60}")
    print(f"Image Processing Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s ({duration/60:.2f} minutes)")
    print(f"Images Processed: {len(all_images):,}")
    print(f"Throughput: {len(all_images)/duration:.1f} images/sec")
    print(f"Total Size Reduction: {total_original_size:.2f} MB -> {total_final_size:.2f} MB ({overall_compression:.1f}%)")
    
    # Save processing report
    with open('/output/processing_report.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['batch', 'images', 'duration', 'original_size_mb', 
                                                'final_size_mb', 'compression_pct'])
        writer.writeheader()
        for r in processing_report:
            writer.writerow({
                'batch': r['batch'],
                'images': r['images'],
                'duration': f"{r['duration']:.2f}",
                'original_size_mb': f"{r['original_size_mb']:.2f}",
                'final_size_mb': f"{r['final_size_mb']:.2f}",
                'compression_pct': f"{r['compression_pct']:.1f}"
            })
    
    print(f"\n✓ Generated processing_report.csv")
    print(f"[{datetime.datetime.now()}] Batch Image Processing completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_image_processing())
