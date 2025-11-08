#!/usr/bin/env python3
"""
WebSocket Telemetry Test Client
This script demonstrates how to connect to the WebSocket telemetry endpoints
"""

import asyncio
import websockets
import json
import sys
from datetime import datetime

# Colors for output
GREEN = '\033[0;32m'
BLUE = '\033[0;34m'
YELLOW = '\033[1;33m'
RED = '\033[0;31m'
BOLD = '\033[1m'
NC = '\033[0m'  # No Color

def print_header(text):
    print(f"\n{BOLD}{'='*60}{NC}")
    print(f"{BOLD}{text}{NC}")
    print(f"{BOLD}{'='*60}{NC}\n")

def print_section(text):
    print(f"\n{BLUE}{text}{NC}")

async def test_health_endpoint():
    """Test the HTTP health endpoint"""
    print_section("Testing Health Endpoint (HTTP)...")
    
    try:
        import urllib.request
        
        url = "http://localhost:8080/health"
        with urllib.request.urlopen(url, timeout=2) as response:
            data = json.loads(response.read())
            print(f"{GREEN}✓ Health endpoint responded:{NC}")
            print(f"  Status:         {data.get('status', 'N/A')}")
            print(f"  Active Clients: {data.get('active_clients', 0)}")
            print(f"  Workers:        {data.get('workers', 0)}")
            print(f"  Active Workers: {data.get('active_workers', 0)}")
            return True
    except Exception as e:
        print(f"{RED}✗ Health endpoint failed: {e}{NC}")
        print(f"{YELLOW}Make sure the master is running!{NC}")
        return False

async def stream_all_workers():
    """Stream telemetry for all workers"""
    uri = "ws://localhost:8080/ws/telemetry"
    
    print_section(f"Connecting to {uri}...")
    
    try:
        async with websockets.connect(uri) as websocket:
            print(f"{GREEN}✓ Connected to telemetry stream{NC}")
            print(f"{YELLOW}Streaming real-time telemetry data (Ctrl+C to stop)...{NC}\n")
            
            update_count = 0
            while True:
                try:
                    message = await websocket.recv()
                    data = json.loads(message)
                    
                    update_count += 1
                    timestamp = datetime.now().strftime("%H:%M:%S")
                    
                    print(f"\n{BOLD}[{timestamp}] Update #{update_count}{NC} {'─'*40}")
                    
                    if not data:
                        print(f"  {YELLOW}No workers registered yet{NC}")
                    
                    for worker_id, telemetry in data.items():
                        print(f"\n  {BOLD}Worker: {worker_id}{NC}")
                        print(f"  {'─'*54}")
                        
                        # Color code based on thresholds
                        cpu_color = GREEN if telemetry['cpu_usage'] < 80 else RED
                        mem_color = GREEN if telemetry['memory_usage'] < 80 else RED
                        gpu_color = GREEN if telemetry['gpu_usage'] < 80 else RED
                        
                        print(f"    CPU:    {cpu_color}{telemetry['cpu_usage']:6.2f}%{NC}")
                        print(f"    Memory: {mem_color}{telemetry['memory_usage']:6.2f}%{NC}")
                        print(f"    GPU:    {gpu_color}{telemetry['gpu_usage']:6.2f}%{NC}")
                        print(f"    Active: {GREEN if telemetry['is_active'] else RED}{'Yes' if telemetry['is_active'] else 'No'}{NC}")
                        print(f"    Tasks:  {len(telemetry['running_tasks'])}")
                        
                        if telemetry['running_tasks']:
                            for task in telemetry['running_tasks']:
                                print(f"      {BLUE}├─ Task {task['task_id']}: {task['status']}{NC}")
                                print(f"      {BLUE}│  CPU: {task['cpu_allocated']:.1f}, "
                                      f"Mem: {task['memory_allocated']:.1f}, "
                                      f"GPU: {task['gpu_allocated']:.1f}{NC}")
                    
                    print(f"\n{BOLD}{'─'*60}{NC}")
                    
                except websockets.exceptions.ConnectionClosed:
                    print(f"\n{RED}✗ Connection closed{NC}")
                    break
                    
    except KeyboardInterrupt:
        print(f"\n\n{YELLOW}Disconnected by user{NC}")
    except ConnectionRefusedError:
        print(f"\n{RED}✗ Connection refused{NC}")
        print(f"{YELLOW}Make sure the master is running on port 8080{NC}")
    except Exception as e:
        print(f"\n{RED}✗ Error: {e}{NC}")

async def stream_worker(worker_id):
    """Stream telemetry for a specific worker"""
    uri = f"ws://localhost:8080/ws/telemetry/{worker_id}"
    
    print_section(f"Connecting to {uri}...")
    
    try:
        async with websockets.connect(uri) as websocket:
            print(f"{GREEN}✓ Connected to worker {worker_id} telemetry stream{NC}")
            print(f"{YELLOW}Streaming telemetry data (Ctrl+C to stop)...{NC}\n")
            
            while True:
                try:
                    message = await websocket.recv()
                    data = json.loads(message)
                    
                    timestamp = datetime.now().strftime("%H:%M:%S")
                    
                    if worker_id in data:
                        telemetry = data[worker_id]
                        cpu_color = GREEN if telemetry['cpu_usage'] < 80 else RED
                        mem_color = GREEN if telemetry['memory_usage'] < 80 else RED
                        gpu_color = GREEN if telemetry['gpu_usage'] < 80 else RED
                        
                        print(f"[{timestamp}] "
                              f"CPU: {cpu_color}{telemetry['cpu_usage']:6.2f}%{NC} | "
                              f"Mem: {mem_color}{telemetry['memory_usage']:6.2f}%{NC} | "
                              f"GPU: {gpu_color}{telemetry['gpu_usage']:6.2f}%{NC} | "
                              f"Tasks: {len(telemetry['running_tasks'])}")
                    
                except websockets.exceptions.ConnectionClosed:
                    print(f"\n{RED}✗ Connection closed{NC}")
                    break
                    
    except KeyboardInterrupt:
        print(f"\n\n{YELLOW}Disconnected by user{NC}")
    except Exception as e:
        print(f"\n{RED}✗ Error: {e}{NC}")

def show_examples():
    """Show usage examples for other languages"""
    print_header("WebSocket Telemetry - Usage Examples")
    
    print(f"{BLUE}Available Endpoints:{NC}")
    print("  • ws://localhost:8080/ws/telemetry          - Stream all workers")
    print("  • ws://localhost:8080/ws/telemetry/{{id}}     - Stream specific worker")
    print("  • http://localhost:8080/health              - Health check")
    
    print(f"\n{BLUE}JavaScript (Browser Console):{NC}")
    print("""
    const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
    ws.onmessage = (e) => console.log(JSON.parse(e.data));
    """)
    
    print(f"{BLUE}Node.js:{NC}")
    print("""
    const WebSocket = require('ws');
    const ws = new WebSocket('ws://localhost:8080/ws/telemetry');
    ws.on('message', data => console.log(JSON.parse(data)));
    """)
    
    print(f"{BLUE}Go:{NC}")
    print("""
    import "github.com/gorilla/websocket"
    
    conn, _, _ := websocket.DefaultDialer.Dial("ws://localhost:8080/ws/telemetry", nil)
    for {
        _, msg, _ := conn.ReadMessage()
        fmt.Println(string(msg))
    }
    """)
    
    print(f"{BLUE}Python (this script):{NC}")
    print("    python3 test_telemetry_websocket.py")
    print("    python3 test_telemetry_websocket.py <worker_id>")
    print()

async def main():
    """Main function"""
    print_header("WebSocket Telemetry Test Client")
    
    # Check if websockets module is installed
    try:
        import websockets
    except ImportError:
        print(f"{RED}✗ Python websockets module not installed{NC}")
        print(f"{YELLOW}Install with: pip3 install websockets{NC}")
        sys.exit(1)
    
    # Parse arguments
    if len(sys.argv) > 1:
        if sys.argv[1] in ["--help", "-h"]:
            print(f"{YELLOW}Usage:{NC}")
            print("  python3 test_telemetry_websocket.py              # Stream all workers")
            print("  python3 test_telemetry_websocket.py <worker_id>  # Stream specific worker")
            print("  python3 test_telemetry_websocket.py --examples   # Show examples only")
            print("  python3 test_telemetry_websocket.py --help       # Show this help")
            sys.exit(0)
        elif sys.argv[1] == "--examples":
            show_examples()
            sys.exit(0)
    
    # Test health endpoint first
    if not await test_health_endpoint():
        print(f"\n{YELLOW}Skipping WebSocket test (server not responding){NC}")
        print(f"\n{BLUE}To start the server:{NC}")
        print("  cd master && ./masterNode")
        print(f"\n{BLUE}To check if server is running:{NC}")
        print("  curl http://localhost:8080/health")
        sys.exit(1)
    
    # Show examples
    show_examples()
    
    # Stream telemetry
    if len(sys.argv) > 1 and sys.argv[1] not in ["--help", "-h", "--examples"]:
        worker_id = sys.argv[1]
        await stream_worker(worker_id)
    else:
        await stream_all_workers()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print(f"\n{YELLOW}Exiting...{NC}")
        sys.exit(0)
