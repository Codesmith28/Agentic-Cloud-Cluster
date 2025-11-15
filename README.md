# CloudAI - Distributed Task Execution System

**Orchestrate Docker-based tasks across worker nodes with AI-powered scheduling and real-time monitoring.**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Python Version](https://img.shields.io/badge/Python-3.8+-3776AB?style=flat&logo=python)](https://python.org)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED?style=flat&logo=docker)](https://docker.com)
[![MongoDB](https://img.shields.io/badge/MongoDB-6.0+-47A248?style=flat&logo=mongodb)](https://mongodb.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## 📖 Overview

CloudAI is a distributed computing platform for executing Docker-based workloads across a cluster of worker nodes. Built with **Go** for performance and **Python** for AI-powered scheduling.

**Use Cases:** ML training, batch processing, CI/CD pipelines, scientific computing, microservices testing

**📚 [Complete Documentation](DOCUMENTATION.md)** | **🚀 [Getting Started Guide](GETTING_STARTED.md)** | **📑 [Documentation Index](DOCUMENTATION_INDEX.md)**

---

## ✨ Key Features

- **Interactive CLI** - Manage cluster from command-line
- **AI Scheduling** - 4 optimization strategies (30-40% better throughput)
- **Real-Time Telemetry** - WebSocket streaming of cluster metrics
- **Docker Native** - Run any containerized workload
- **REST & gRPC APIs** - Full programmatic access
- **MongoDB Persistence** - Task history and results
- **Auto-Registration** - Workers connect automatically
- **Task Cancellation** - Graceful and forceful termination
- **Resource Tracking** - CPU, Memory, GPU, Storage

## 🏗️ Architecture

```
User Interface (CLI/API)
         ↓
    Master Node ────→ MongoDB (Persistence)
    (Go + gRPC)       
         ↓
    ┌────┼────┐
    ↓    ↓    ↓
 Worker Worker Worker (Go + Docker)
```

**Components:**
- **Master**: Task assignment, worker management, telemetry aggregation (Port 50051)
- **Worker**: Docker execution, heartbeat monitoring (Port 50052+)
- **AI Scheduler**: Intelligent task placement (Python, optional)
- **Database**: MongoDB for persistence

**Communication:**
- gRPC for Master↔Worker
- HTTP/WebSocket for monitoring (Port 8080)
- MongoDB for data persistence

See [architecture.md](docs/architecture.md) for detailed diagrams.

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22+
- Docker (daemon running)
- Protocol Buffers compiler (`protoc`)
- MongoDB (via Docker Compose)
- Python 3.8+ (for AI Scheduler)

### Installation

```bash
# Clone repository
git clone --recurse-submodules https://github.com/Codesmith28/CloudAI.git
cd CloudAI

# One-time setup (generates proto code, creates symlinks, installs deps)
make setup

# Build master and worker
make all
```

### Run the System

```bash
# Terminal 1: Start MongoDB
cd database && docker-compose up -d

# Terminal 2: Start Master
cd master && ./masterNode

# Terminal 3: Start Worker  
cd worker && ./workerNode
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

### AI Scheduler

```bash
cd agentic_scheduler

python main.py ai              # Multi-Objective (balanced)
python main.py ai_aggressive   # Max resource utilization
python main.py ai_predictive   # Min completion time
python test_schedulers.py      # Compare all strategies
```

### Monitoring

```bash
# REST API
curl http://localhost:8080/telemetry | jq

# WebSocket (real-time)
wscat -c ws://localhost:8080/ws/telemetry
```

**📖 See [DOCUMENTATION.md](DOCUMENTATION.md) for complete API reference**

---

## � Documentation

- **[GETTING_STARTED.md](GETTING_STARTED.md)** - 5-minute setup guide
- **[DOCUMENTATION.md](DOCUMENTATION.md)** - Complete reference (50+ pages)
- **[DOCUMENTATION_INDEX.md](DOCUMENTATION_INDEX.md)** - Documentation navigator
- **[docs/architecture.md](docs/architecture.md)** - System architecture
- **[docs/TELEMETRY_QUICK_REFERENCE.md](docs/TELEMETRY_QUICK_REFERENCE.md)** - Monitoring guide
- **[agentic_scheduler/AI_SCHEDULER_USAGE.md](agentic_scheduler/AI_SCHEDULER_USAGE.md)** - AI scheduler

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
- ✅ AI scheduling (4 strategies)
- ✅ Real-time telemetry
- ✅ Interactive CLI
- ✅ Task cancellation

**Planned (v2.1)**
- 🔜 Task queuing
- 🔜 Web dashboard
- 🔜 Authentication

---

## 📜 License

MIT License - see [LICENSE](LICENSE) file

---

## 📞 Contact

- **Issues**: [GitHub Issues](https://github.com/Codesmith28/CloudAI/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Codesmith28/CloudAI/discussions)
- **Repository**: [github.com/Codesmith28/CloudAI](https://github.com/Codesmith28/CloudAI)

---

<div align="center">

**⭐ Star this repo if you find it useful!**

Built with [gRPC](https://grpc.io/) • [MongoDB](https://www.mongodb.com/) • [Docker](https://www.docker.com/)

</div>
