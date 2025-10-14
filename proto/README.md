# Proto Definitions

This directory contains the gRPC protocol buffer definitions for the CloudAI cluster.

## Proto Files

- **`master_worker.proto`**: Communication between Master (Go) ↔ Worker (Go)

  - Task assignment
  - Heartbeat/telemetry reporting
  - Task completion results

- **`master_agent.proto`**: Communication between Master (Go) ↔ AI Agent (Python)
  - Cluster state queries
  - AI-based task assignment decisions

## Prerequisites

Install required tools:

```bash
# Install protoc compiler
# Ubuntu/Debian:
sudo apt-get install -y protobuf-compiler

# macOS:
brew install protobuf

# Install Go plugins for protoc
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install Python plugins for protoc
pip install grpcio grpcio-tools
```

## Generate Code

Run the generation script:

```bash
cd proto
chmod +x generate.sh
./generate.sh
```

This will generate:

- **Go code** in `./pb/` directory (for master and worker)
- **Python code** in `./py/` directory (for AI agent)

## Generated Files Structure

```
proto/
├── pb/                          # Go generated code
│   ├── master_worker.pb.go
│   ├── master_worker_grpc.pb.go
│   ├── master_agent.pb.go
│   └── master_agent_grpc.pb.go
├── py/                          # Python generated code
│   ├── master_agent_pb2.py
│   └── master_agent_pb2_grpc.py
└── ...
```

## Import in Code

**Go (Master/Worker):**

```go
import pb "path/to/CloudAI/proto/pb"
```

**Python (Agent):**

```python
import sys
sys.path.append('path/to/CloudAI/proto/py')
import master_agent_pb2
import master_agent_pb2_grpc
```
