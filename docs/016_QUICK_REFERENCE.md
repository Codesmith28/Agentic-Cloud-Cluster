# CloudAI Quick Reference

## 🚀 Quick Start (TL;DR)

```bash
# 1. Generate proto code
cd proto && ./generate.sh && cd ..

# 2. Build sample task
cd sample_task
docker build -t <username>/cloudai-sample-task:latest .
docker push <username>/cloudai-sample-task:latest
cd ..

# 3. Start MongoDB
cd database && docker-compose up -d && cd ..

# 4. Start Master (Terminal 1)
cd master
ln -s ../proto/pb proto
go mod tidy
go build -o master-node .
./master-node

# 5. Start Worker (Terminal 2)
cd worker
ln -s ../proto/pb proto
go mod tidy
go build -o worker-node .
./worker-node

# 6. Assign Task (In Master CLI)
master> task docker.io/<username>/cloudai-sample-task:latest
```

---

## 📂 Directory Reference

| Path           | Purpose                    |
| -------------- | -------------------------- |
| `proto/`       | gRPC protocol definitions  |
| `proto/pb/`    | Generated Go code          |
| `proto/py/`    | Generated Python code      |
| `master/`      | Master node implementation |
| `worker/`      | Worker node implementation |
| `sample_task/` | Example Docker task        |
| `database/`    | MongoDB setup              |

---

## 🔧 Common Commands

### Master CLI

```bash
help                                    # Show help
status                                  # Cluster status
workers                                 # List workers
task <docker_image> [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]  # Submit task (scheduler selects worker)
dispatch <worker_id> <docker_image> [options]  # Dispatch task directly to specific worker
exit                                    # Shutdown
```

### Worker Flags

```bash
# Worker auto-detects system information - no flags required for basic usage
# Optional flags for advanced configuration:
-master <address>       # Master address (default: localhost:50051)
-port <port>            # Worker port (default: auto-selected available port)
```

---

## 📡 gRPC Ports

| Service  | Port  |
| -------- | ----- |
| Master   | 50051 |
| Worker-1 | 50052 |
| Worker-2 | 50053 |
| Worker-N | 5005N |
| MongoDB  | 27017 |

---

## 🐛 Quick Fixes

**Proto generation fails:**

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH=$PATH:$(go env GOPATH)/bin
```

**Worker can't connect:**

```bash
# Check master is running
ps aux | grep master-node

# Test connection
telnet localhost 50051
```

**Docker permission denied:**

```bash
sudo usermod -aG docker $USER
# Logout and login again
```

**Build fails:**

```bash
cd master  # or worker
rm -rf proto
ln -s ../proto/pb proto
go mod tidy
```

---

## 📊 System Flow

```
1. Worker registers → Master stores worker info
2. Worker sends heartbeat → Master monitors health
3. User assigns task → Master sends to Worker
4. Worker pulls Docker image → Runs container
5. Worker collects logs → Sends to Master
6. Worker reports result → Master stores result
```

---

## 🏗️ Code Structure

### Master

```
master/
├── main.go                    # Entry point + gRPC server
├── internal/
│   ├── server/
│   │   └── master_server.go   # gRPC handlers
│   ├── cli/
│   │   └── cli.go             # Interactive CLI
│   └── db/
│       └── init.go            # MongoDB setup
```

### Worker

```
worker/
├── main.go                    # Entry point + registration
├── internal/
│   ├── server/
│   │   └── worker_server.go   # gRPC handlers
│   ├── executor/
│   │   └── executor.go        # Docker execution
│   └── telemetry/
│       └── telemetry.go       # Heartbeat + monitoring
```

---

## 🔑 Key Functions

### Master Server

- `RegisterWorker` - Handle worker registration
- `SendHeartbeat` - Process heartbeat messages
- `ReportTaskCompletion` - Receive task results

### Worker Server

- `AssignTask` - Receive task from master
- `ExecuteTask` - Run Docker container
- `sendHeartbeat` - Send periodic telemetry

---

## 📝 Environment Variables

Create `.env` in project root:

```bash
MONGODB_USERNAME=cloudai
MONGODB_PASSWORD=cloudai_secret
```

---

## 🧪 Testing Checklist

- [ ] Proto code generated successfully
- [ ] Master starts without errors
- [ ] Worker registers with master
- [ ] Heartbeats visible in master logs
- [ ] Docker image accessible
- [ ] Task assignment succeeds
- [ ] Task executes and completes
- [ ] Logs appear in master
- [ ] Result reported successfully

---

## 📚 Full Documentation

See [SETUP.md](./SETUP.md) for comprehensive instructions.
