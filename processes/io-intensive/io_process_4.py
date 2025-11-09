#!/usr/bin/env python3
"""
IO Process 4: Network-like I/O Simulation
Simulates network packet capture and processing with buffering.
"""
import os
import time
import random
import psutil
import json
import struct
from datetime import datetime

def generate_packet():
    """Generate simulated network packet"""
    src_ip = f"{random.randint(1,255)}.{random.randint(1,255)}.{random.randint(1,255)}.{random.randint(1,255)}"
    dst_ip = f"{random.randint(1,255)}.{random.randint(1,255)}.{random.randint(1,255)}.{random.randint(1,255)}"
    protocol = random.choice(['TCP', 'UDP', 'ICMP', 'HTTP'])
    size = random.randint(64, 1500)
    payload = os.urandom(size)
    
    return {
        'timestamp': time.time(),
        'src_ip': src_ip,
        'dst_ip': dst_ip,
        'protocol': protocol,
        'size': size,
        'payload': payload
    }

def main():
    print(f"[{datetime.now()}] Starting IO-Intensive Process 4: Network I/O Simulation")
    
    results_dir = "/results"
    os.makedirs(results_dir, exist_ok=True)
    
    process = psutil.Process(os.getpid())
    
    stats = {
        "process_type": "IO-Intensive-4",
        "packets_generated": 0,
        "packets_written": 0,
        "total_bytes": 0,
        "peak_memory_mb": 0,
        "duration_seconds": 0
    }
    
    start_time = time.time()
    buffer = []
    buffer_size_threshold = random.randint(50, 200)
    
    try:
        num_packets = random.randint(50000, 150000)
        print(f"Generating and processing {num_packets} packets...")
        
        capture_file = f"{results_dir}/capture.pcap"
        
        with open(capture_file, 'wb') as f:
            for i in range(num_packets):
                packet = generate_packet()
                buffer.append(packet)
                stats["packets_generated"] += 1
                
                # Flush buffer to disk periodically
                if len(buffer) >= buffer_size_threshold:
                    for pkt in buffer:
                        # Write packet header and payload
                        header = struct.pack('!LLHH', 
                                           int(pkt['timestamp']), 
                                           pkt['size'],
                                           len(pkt['src_ip']),
                                           len(pkt['dst_ip']))
                        f.write(header)
                        f.write(pkt['src_ip'].encode())
                        f.write(pkt['dst_ip'].encode())
                        f.write(pkt['protocol'].encode().ljust(8, b'\x00'))
                        f.write(pkt['payload'])
                        
                        stats["total_bytes"] += pkt['size']
                        stats["packets_written"] += 1
                    
                    buffer.clear()
                    
                    # Memory tracking
                    mem_mb = process.memory_info().rss / (1024 * 1024)
                    stats["peak_memory_mb"] = max(stats["peak_memory_mb"], mem_mb)
                    
                    if stats["packets_written"] % 10000 == 0:
                        print(f"  Processed {stats['packets_written']} packets, Memory: {mem_mb:.2f}MB")
                    
                    time.sleep(random.uniform(0.01, 0.05))
            
            # Flush remaining buffer
            for pkt in buffer:
                header = struct.pack('!LLHH', 
                                   int(pkt['timestamp']), 
                                   pkt['size'],
                                   len(pkt['src_ip']),
                                   len(pkt['dst_ip']))
                f.write(header)
                f.write(pkt['src_ip'].encode())
                f.write(pkt['dst_ip'].encode())
                f.write(pkt['protocol'].encode().ljust(8, b'\x00'))
                f.write(pkt['payload'])
                stats["total_bytes"] += pkt['size']
                stats["packets_written"] += 1
        
        # Read back and analyze
        print(f"\nAnalyzing captured packets...")
        protocol_counts = {}
        
        with open(capture_file, 'rb') as f:
            packets_read = 0
            while True:
                header_data = f.read(16)
                if not header_data or len(header_data) < 16:
                    break
                
                timestamp, size, src_len, dst_len = struct.unpack('!LLHH', header_data)
                src_ip = f.read(src_len).decode()
                dst_ip = f.read(dst_len).decode()
                protocol = f.read(8).decode().strip('\x00')
                payload = f.read(size)
                
                protocol_counts[protocol] = protocol_counts.get(protocol, 0) + 1
                packets_read += 1
                
                if packets_read % 10000 == 0:
                    mem_mb = process.memory_info().rss / (1024 * 1024)
                    print(f"  Analyzed {packets_read} packets, Memory: {mem_mb:.2f}MB")
        
        stats["duration_seconds"] = time.time() - start_time
        stats["protocol_distribution"] = protocol_counts
        
        # Save stats
        with open(f"{results_dir}/io_stats.json", 'w') as f:
            json.dump(stats, f, indent=2)
        
        print(f"\n{'='*60}")
        print(f"Process completed successfully!")
        print(f"Packets generated: {stats['packets_generated']}")
        print(f"Packets written: {stats['packets_written']}")
        print(f"Total bytes: {stats['total_bytes'] / (1024**2):.2f} MB")
        print(f"Peak memory: {stats['peak_memory_mb']:.2f} MB")
        print(f"Duration: {stats['duration_seconds']:.2f} seconds")
        print(f"{'='*60}")
        
    except Exception as e:
        print(f"Error: {e}")
        stats["error"] = str(e)
        raise

if __name__ == "__main__":
    main()
