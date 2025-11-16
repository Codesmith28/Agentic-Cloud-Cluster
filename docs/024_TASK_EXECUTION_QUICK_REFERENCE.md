# Task Execution & Monitoring - Quick Reference

## Quick Start

### 1. Start the System

```bash
# Terminal 1: Start MongoDB (if using database)
cd database
docker-compose up -d

# Terminal 2: Start Master
cd master
./masterNode

# Terminal 3: Start Worker
cd worker
./workerNode
```

### 2. Register Worker

```bash
# In master CLI
master> register worker-1 localhost:50052
```

### 3. Submit a Task

```bash
# Basic task
master> task worker-1 hello-world:latest

# Task with resource limits
master> task worker-1 python:3.9 -cpu_cores 2.0 -mem 4.0 -gpu_cores 0.0

# Task with all parameters
master> task worker-1 alpine:latest -cpu_cores 1.0 -mem 0.5 -storage 5.0 -gpu_cores 0.0
```

### 4. Monitor Task

```bash
master> monitor task-1730899200

# Monitor with specific user
master> monitor task-1730899200 user-1

# Press any key to exit monitoring
```

## Command Reference

### Task Command
```
task <docker_image> [options]

Options:
  -cpu_cores <num>   CPU cores (default: 1.0)
  -mem <gb>          Memory in GB (default: 0.5)
  -storage <gb>      Storage in GB (default: 1.0)
  -gpu_cores <num>   GPU cores (default: 0.0)

Examples:
  task worker-1 nginx:latest
  task worker-2 python:3.9 -cpu_cores 2.0 -mem 4.0
  task worker-1 tensorflow/tensorflow:latest-gpu -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

### Monitor Command
```
monitor <task_id> [user_id]

Arguments:
  task_id    ID of the task to monitor (required)
  user_id    User ID for authorization (optional, default: admin)

Examples:
  monitor task-1730899200
  monitor task-1730899200 user-1
```

## Task Lifecycle

```
┌──────────────┐
│   Pending    │  Task created in database
└──────┬───────┘
       │ Worker accepts
       ↓
┌──────────────┐
│   Running    │  Container executing
└──────┬───────┘
       │ Container exits
       ↓
┌──────────────┐
│  Completed   │  Success (exit code 0)
│   or Failed  │  Failed (non-zero exit code)
└──────────────┘
```

## Resource Limits Explained

### CPU Cores
- **Value**: Number of CPU cores (can be fractional)
- **Examples**:
  - `1.0` = 1 full CPU core
  - `0.5` = Half a CPU core (50% of one core)
  - `2.0` = 2 full CPU cores
- **Docker Translation**: Converted to nanoCPUs (1 core = 1e9 nanoCPUs)

### Memory
- **Value**: Amount of RAM in gigabytes
- **Examples**:
  - `0.5` = 500 MB
  - `1.0` = 1 GB
  - `4.0` = 4 GB
- **Docker Translation**: Converted to bytes (1 GB = 1,073,741,824 bytes)

### Storage
- **Value**: Disk space in gigabytes
- **Note**: Currently recorded but not enforced by Docker limits
- **Future**: Will be used for volume allocation

### GPU Cores
- **Value**: Number of GPUs to allocate
- **Examples**:
  - `0.0` = No GPU
  - `1.0` = 1 GPU
  - `2.0` = 2 GPUs
- **Requirements**: Worker must have nvidia-docker runtime installed

## Container Commands

Tasks can execute custom commands inside containers:

```bash
# Default: Uses container's CMD/ENTRYPOINT
task worker-1 python:3.9

# With command: Overrides default
# The command is executed as: /bin/sh -c "python script.py"
task worker-1 python:3.9 -cpu_cores 1.0
```

## Monitoring Features

### Live Log Display
- Shows stdout and stderr from container
- Updates in real-time as logs are generated
- Timestamps included for each log line

### Exit Options
- Press any key to stop monitoring
- Returns to CLI prompt
- Task continues running in background

### Status Messages
```
Running:    Yellow header, streaming logs
Completed:  Green banner, task finished successfully
Failed:     Red banner, task failed with error
Not Found:  Red error, task doesn't exist
```

## Database Queries

### View All Tasks
```bash
mongo cluster_db
> db.TASKS.find().pretty()
```

### View Tasks by User
```javascript
db.TASKS.find({ user_id: "admin" })
```

### View Running Tasks
```javascript
db.TASKS.find({ status: "running" })
```

### View Task Assignments
```javascript
db.ASSIGNMENTS.find({ task_id: "task-1730899200" })
```

### Find Worker for Task
```javascript
db.ASSIGNMENTS.findOne({ task_id: "task-1730899200" })
```

## Troubleshooting

### Task Not Starting

**Problem**: Task stuck in "pending" status

**Solutions**:
1. Check worker is active: `master> workers`
2. Verify worker can pull image: `docker pull <image>`
3. Check Docker daemon is running on worker
4. Review worker logs for errors

### Cannot Monitor Task

**Problem**: "Task not found" error

**Solutions**:
1. Verify task ID is correct
2. Check task exists: Query `TASKS` collection
3. Ensure worker is still running
4. Confirm task hasn't completed yet

### Resource Limits Not Applied

**Problem**: Container using more resources than specified

**Solutions**:
1. Verify Docker version supports resource constraints
2. Check Docker is configured for resource limits
3. Review worker logs for Docker errors
4. Test manually: `docker run --cpus=1.0 --memory=1g <image>`

### Log Streaming Stops

**Problem**: Monitoring disconnects unexpectedly

**Solutions**:
1. Check network connection to worker
2. Verify worker hasn't crashed
3. Check container is still running
4. Review master logs for gRPC errors

## Example: Full Workflow

```bash
# 1. Start master
cd master && ./masterNode

# 2. In master CLI, check status
master> status

# 3. List workers (should be empty initially)
master> workers

# 4. In another terminal, start worker
cd worker && ./workerNode

# 5. Back in master CLI, register the worker
master> register worker-1 192.168.1.100:50052

# 6. Verify worker is registered
master> workers

# 7. Submit a simple task
master> task worker-1 alpine:latest -cpu_cores 0.5 -mem 0.25

# Note the task ID from output, e.g., task-1730899200

# 8. Monitor the task
master> monitor task-1730899200

# 9. Watch logs stream in real-time
# Press any key when done

# 10. Check task in database
# In another terminal:
mongo cluster_db --eval "db.TASKS.find({task_id: 'task-1730899200'}).pretty()"

# 11. Check assignment
mongo cluster_db --eval "db.ASSIGNMENTS.find({task_id: 'task-1730899200'}).pretty()"
```

## Sample Tasks for Testing

### 1. Hello World (Quick Test)
```bash
master> task worker-1 hello-world:latest -cpu_cores 0.5 -mem 0.25
```

### 2. Long-Running Task (For Monitoring)
```bash
master> task worker-1 alpine:latest -cpu_cores 0.5 -mem 0.25
# Container runs: sleep 30; echo "Done"
```

### 3. Python Script
```bash
master> task worker-1 python:3.9 -cpu_cores 1.0 -mem 1.0
# Executes default Python command
```

### 4. Resource-Intensive Task
```bash
master> task worker-1 ubuntu:latest -cpu_cores 2.0 -mem 4.0
# Container has 2 CPU cores and 4GB RAM available
```

## Tips & Best Practices

### 1. Resource Allocation
- Start with lower limits and increase as needed
- Monitor actual usage before assigning more resources
- Leave some resources free for system overhead

### 2. Task Monitoring
- Monitor new tasks to ensure they start correctly
- Use monitoring to debug container issues
- Press any key to exit monitoring (don't kill terminal)

### 3. Database Maintenance
- Regularly clean up completed tasks
- Archive old assignments
- Index frequently queried fields

### 4. Error Handling
- Check logs immediately if task fails
- Verify image exists before assigning task
- Test tasks locally with Docker first

### 5. Security
- Use specific image tags (not `latest`)
- Only run trusted containers
- Limit resource allocation per task
- Plan to implement authentication for production

## Quick Debugging Commands

```bash
# Check worker status
master> workers

# Check specific worker details
master> stats worker-1

# View cluster status (refreshes every 2s)
master> status

# In worker terminal, check Docker
docker ps  # See running containers
docker logs task-<id>  # View container logs directly

# In MongoDB
use cluster_db
db.TASKS.find({ status: "failed" })  # Find failed tasks
db.ASSIGNMENTS.find({ worker_id: "worker-1" })  # Tasks on worker
```

## Next Steps

After mastering basic usage:
1. Review `WEB_INTERFACE.md` for planned API features
2. Read `TASK_EXECUTION_MONITORING_SUMMARY.md` for technical details
3. Explore database schema in `docs/schema.md`
4. Set up monitoring dashboards
5. Plan user authentication system

---
*Quick Reference - Version 1.0*
*Last updated: November 6, 2025*
