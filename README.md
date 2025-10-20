# CloudAI - Distributed Task Execution System

CloudAI is a distributed computing platform that orchestrates Docker-based task execution across a cluster of worker nodes. Built with Go and gRPC, it provides a robust foundation for distributed workload processing with real-time monitoring and management.

## 🌟 Features

- **🎮 Interactive Master CLI**: Manage tasks and monitor cluster from a command-line interface
- **📡 gRPC Communication**: Efficient, type-safe communication between master and workers
- **🐳 Docker Integration**: Run any containerized workload
- **📊 Real-time Telemetry**: Worker health monitoring with periodic heartbeats
- **🗃️ MongoDB Backend**: Persistent storage for tasks, workers, and results
- **🔄 Auto-Registration**: Workers automatically register with master on startup
- **📝 Log Collection**: Capture and display task execution logs
- **⚡ Async Execution**: Non-blocking task processing

## 🏗️ Architecture

```
┌─────────────┐
│   Master    │  (Port 50051)
│   - gRPC    │  • Task assignment
│   - MongoDB │  • Worker management
│   - CLI     │  • Telemetry collection
└──────┬──────┘
       │
       │ gRPC (TLS Ready)
       │
    ┌──┴──────────┬──────────────┐
    │             │              │
┌───▼────┐   ┌───▼────┐    ┌───▼────┐
│Worker-1│   │Worker-2│    │Worker-N│
│- Docker│   │- Docker│    │- Docker│
│- gRPC  │   │- gRPC  │    │- gRPC  │
└────────┘   └────────┘    └────────┘
```

## 📂 Project Structure

```
CloudAI/
├── proto/              # gRPC protocol definitions
│   ├── master_worker.proto
│   ├── master_agent.proto
│   └── generate.sh
├── master/             # Master node (Go)
│   ├── main.go
│   └── internal/
│       ├── server/     # gRPC handlers
│       ├── cli/        # Interactive CLI
│       └── db/         # MongoDB integration
├── worker/             # Worker node (Go)
│   ├── main.go
│   └── internal/
│       ├── server/     # gRPC server
│       ├── executor/   # Docker execution
│       └── telemetry/  # Monitoring
├── sample_task/        # Example Docker task
├── database/           # MongoDB setup
└── docs/               # Documentation
```

## 🚀 Quick Start

### Prerequisites

- **Go 1.22+**
- **Docker** with daemon running
- **Protocol Buffers** compiler (`protoc`)
- **MongoDB** (via docker-compose)

### Clone with Submodules

For newcomers cloning this repository:

```bash
# Clone the repository with all submodules
git clone --recurse-submodules git@github.com:Codesmith28/CloudAI.git

# OR if already cloned without submodules:
git submodule update --init --recursive
```

### Installation

```bash
# 1. Generate gRPC code
cd proto && ./generate.sh && cd ..

# 2. Start MongoDB
cd database && docker-compose up -d && cd ..

# 3. Create .env file
cat > .env << EOF
MONGODB_USERNAME=admin
MONGODB_PASSWORD=password123
EOF

# 4. Build and run Master (Terminal 1)
cd master
ln -s ../proto/pb proto
go mod tidy
go build -o master-node .
./master-node

# 5. Build and run Worker (Terminal 2)
cd worker
ln -s ../proto/pb proto
go mod tidy
go build -o worker-node .
./worker-node -id worker-1

# 6. Assign a task (in Master CLI)
master> task worker-1 docker.io/username/cloudai-sample-task:latest
```

## 📚 Documentation

- **[SETUP.md](./SETUP.md)** - Comprehensive setup and testing guide
- **[QUICK_REFERENCE.md](./QUICK_REFERENCE.md)** - Command reference and quick fixes
- **[master/README.md](./master/README.md)** - Master node documentation
- **[worker/README.md](./worker/README.md)** - Worker node documentation
- **[proto/README.md](./proto/README.md)** - Protocol buffer guide
- **[docs/schema.md](./docs/schema.md)** - Database schema

## 🎮 Master CLI Commands

```bash
help                                  # Show available commands
status                                # Show cluster status
workers                               # List registered workers
task <worker_id> <docker_image>      # Assign task to worker
exit                                  # Shutdown master
```

## 🔧 Worker Configuration

```bash
./worker-node \
  -id worker-1 \                    # Worker identifier
  -ip localhost \                   # Worker IP address
  -master localhost:50051 \         # Master server address
  -port :50052                      # Worker gRPC port
```

## 📊 System Flow

```
1. Worker Registration
   Worker → Master: RegisterWorker(WorkerInfo)

2. Heartbeat Monitoring
   Worker → Master: SendHeartbeat (every 5s)

3. Task Assignment
   User → Master CLI: task worker-1 docker.io/image
   Master → Worker: AssignTask(Task)

4. Task Execution
   Worker: Pull Docker image → Run container → Collect logs

5. Result Reporting
   Worker → Master: ReportTaskCompletion(TaskResult)
```

## 🐳 Creating Custom Tasks

Tasks are Docker containers. Create your own:

```python
# task.py
import time
print("Processing data...")
time.sleep(5)
print("Task completed!")
```

```dockerfile
# Dockerfile
FROM python:3.11-slim
COPY task.py .
CMD ["python", "task.py"]
```

```bash
docker build -t username/my-task:latest .
docker push username/my-task:latest
```

Then assign it:

```bash
master> task worker-1 docker.io/username/my-task:latest
```

## 🗄️ MongoDB Collections

| Collection        | Purpose                     |
| ----------------- | --------------------------- |
| `USERS`           | User authentication         |
| `WORKER_REGISTRY` | Worker nodes and resources  |
| `TASKS`           | Task definitions and status |
| `ASSIGNMENTS`     | Task-to-worker mappings     |
| `RESULTS`         | Task execution results      |

## 🔌 gRPC Services

### MasterWorker Service

- `RegisterWorker` - Worker registration
- `SendHeartbeat` - Health monitoring
- `ReportTaskCompletion` - Task results
- `AssignTask` - Task assignment
- `CancelTask` - Task cancellation

### MasterAgent Service (Future)

- `FetchClusterState` - Get cluster info
- `SubmitAssignments` - AI-based scheduling

## 🛠️ Troubleshooting

**Problem**: protoc: command not found

```bash
# Ubuntu/Debian
sudo apt-get install protobuf-compiler
# macOS
brew install protobuf
```

**Problem**: Worker can't connect to master

```bash
# Check master is running
ps aux | grep master-node
# Verify port
netstat -tlnp | grep 50051
```

**Problem**: Docker permission denied

```bash
sudo usermod -aG docker $USER
# Logout and login again
```

See [SETUP.md](./SETUP.md) for more troubleshooting.

## 🎯 Use Cases

- **Machine Learning Training**: Distribute ML training jobs across workers
- **Data Processing**: Process large datasets in parallel
- **CI/CD Pipelines**: Run build and test jobs on demand
- **Batch Jobs**: Execute scheduled batch processing tasks
- **Scientific Computing**: Run simulations and computations
- **Video Processing**: Transcode and process media files

## 🚧 Roadmap

- [x] Basic master-worker communication
- [x] Docker task execution
- [x] Real-time telemetry
- [x] Interactive CLI
- [ ] Task queuing system
- [ ] AI-based scheduling agent (Python)
- [ ] Web dashboard
- [ ] TLS authentication
- [ ] GPU support
- [ ] S3 result storage
- [ ] Multi-master HA setup

## 🤝 Contributing

Contributions are welcome! Areas for improvement:

- Enhanced resource monitoring
- Load balancing algorithms
- Task priority scheduling
- Container networking
- Security hardening
- Performance optimization

## 📄 License

[Add your license here]

## 🙏 Acknowledgments

Built with:

- [gRPC](https://grpc.io/) - RPC framework
- [Protocol Buffers](https://protobuf.dev/) - Serialization
- [Docker SDK](https://docs.docker.com/engine/api/sdk/) - Container management
- [MongoDB Go Driver](https://www.mongodb.com/docs/drivers/go/current/) - Database

---

**Status**: ✅ Minimal viable version complete and tested

For detailed setup instructions, see [SETUP.md](./SETUP.md)
