# CloudAI - Distributed Task Execution System

**Orchestrate Docker-based tasks across worker nodes with real-time monitoring and a web dashboard.**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED?style=flat&logo=docker)](https://docker.com)
[![MongoDB](https://img.shields.io/badge/MongoDB-7.0+-47A248?style=flat&logo=mongodb)](https://mongodb.com)

---

## 📖 Overview

CloudAI is a distributed computing platform for executing Docker-based workloads across a cluster of worker nodes. Built with **Go** for high performance.

**Use Cases:** ML training, batch processing, CI/CD pipelines, scientific computing, microservices testing

**📚 [Complete Documentation](DOCUMENTATION.md)** | **🚀 [Getting Started Guide](GETTING_STARTED.md)** 

---

## ✨ Key Features

- **Interactive CLI** - Manage cluster from command-line
- **Web Dashboard** - Real-time UI for monitoring and management
- **Real-Time Telemetry** - WebSocket streaming of cluster metrics
- **Docker Native** - Run any containerized workload
- **REST & gRPC APIs** - Full programmatic access
- **MongoDB Persistence** - Task history and results
- **Auto-Registration** - Workers connect automatically
- **Task Cancellation** - Graceful and forceful termination
- **Resource Tracking** - CPU, Memory, GPU, Storage
- **File Storage** - Secure file upload and download for tasks

## 🏗️ Architecture

```
User Interface (CLI/API)
         ↓
    Master Node ---> MongoDB (Persistence)
    (Go + gRPC)       
         ↓
    ┌────┼────┐
    ↓    ↓    ↓
 Worker Worker Worker (Go + Docker)
```

**Components:**
- **Master**: Task assignment, worker management, telemetry aggregation (gRPC: 50051, HTTP: 8080)
- **Worker**: Docker execution, heartbeat monitoring (Port 50052+)
- **Web UI**: React-based dashboard for monitoring (Port 3000)
- **Database**: MongoDB for persistence

**Communication:**
- gRPC for Master ↔ Worker
- HTTP/WebSocket for monitoring and API (Port 8080)
- MongoDB for data persistence

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22+
- Docker (daemon running)
- Protocol Buffers compiler (`protoc`)
- MongoDB (via Docker Compose)
- Node.js 18+ (for Web UI)
- Python 3.8+ (for future agent extensibility)

### Installation

```bash
# Clone repository
git clone https://github.com/Codesmith28/CloudAI.git
cd CloudAI

# Set up Python virtual environment (for future agent extensibility)
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# One-time setup (generates proto code, creates symlinks, installs deps)
make setup

# Build master and worker
make all

# Install UI dependencies (optional)
cd ui && npm install && cd ..
```

### Run the System

```bash
# Terminal 1: Start MongoDB
cd database && docker-compose up -d

# Terminal 2: Start Master (includes Web UI on port 3000)
./runMaster.sh

# Terminal 3: Start Worker  
./runWorker.sh
```

### Your First Task

```bash
# In master CLI
master> workers                              # List workers
master> task worker-1 hello-world:latest    # Submit task
master> monitor task-<id>                   # Watch execution
```

**📚 See [GETTING_STARTED.md](GETTING_STARTED.md) for detailed walkthrough**

---

## 📚 Usage Examples

### CLI Commands

```bash
# Cluster management
master> status                                    # Cluster overview
master> workers                                   # List all workers
master> register worker-3 192.168.1.102:50052    # Manual registration

# Task operations
master> task worker-1 python:3.9 -cpu_cores 2.0 -mem 4.0    # Submit task
master> monitor task-<id>                                     # Watch logs
master> cancel task-<id>                                      # Cancel task
```

### Monitoring

```bash
# REST API - Telemetry
curl http://localhost:8080/telemetry | jq
curl http://localhost:8080/workers | jq

# REST API - Tasks
curl http://localhost:8080/api/tasks | jq
curl http://localhost:8080/api/workers | jq

# WebSocket (real-time)
wscat -c ws://localhost:8080/ws/telemetry

# Submit Task via REST API
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"docker_image":"ubuntu:latest","command":"echo hello","cpu_required":1.0,"memory_required":512.0}'
```

**📖 See [DOCUMENTATION.md](DOCUMENTATION.md) for complete API reference**

---

## 📚 Documentation

- **[GETTING_STARTED.md](GETTING_STARTED.md)** - 5-minute setup guide
- **[DOCUMENTATION.md](DOCUMENTATION.md)** - Complete reference
- **[Example.md](Example.md)** - Usage examples

---

## 🐛 Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| **Worker not connecting** | Check `netstat -tuln \| grep 50051`, verify firewall |
| **Task fails** | Run `docker pull <image>` to test, check logs with `monitor` |
| **MongoDB error** | Run `docker-compose ps` in database/, restart if needed |

**Debug Logging:**
```bash
export LOG_LEVEL=debug
./masterNode
```

**📚 See [DOCUMENTATION.md](DOCUMENTATION.md) Section 12 for detailed troubleshooting**

---

## 🤝 Contributing

Contributions welcome! Areas of interest:
- New scheduling algorithms
- Dashboard/UI implementation  
- Authentication & authorization
- Performance optimizations
- Documentation improvements

**Process:** Fork → Feature Branch → Commit → Push → Pull Request

---

## 🗺️ Roadmap

**Current (v2.0)**
- ✅ Master-Worker architecture  
- ✅ Real-time telemetry
- ✅ Interactive CLI
- ✅ Task cancellation
- ✅ Web dashboard
- ✅ File storage

**Planned (v2.1)**
- 🔜 Task queuing improvements
- 🔜 Authentication enhancements
- 🔜 Cluster auto-scaling

---

<div align="center">

**⭐ Star this repo if you find it useful!**

Built with [gRPC](https://grpc.io/) • [MongoDB](https://www.mongodb.com/) • [Docker](https://www.docker.com/)

</div>
