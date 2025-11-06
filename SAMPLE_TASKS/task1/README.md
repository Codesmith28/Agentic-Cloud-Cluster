# Sample Task 1: Data Processing

This directory contains a simple Python task that can be containerized and executed by CloudAI workers.

## What the Task Does

The task simulates a data processing workload:

1. Initializes the task environment
2. Processes random data points (50-150 items)
3. Computes statistics and saves results to `/output/result.json`
4. Prints progress and completion status

## Quick Start

### 1. Build and Push to Docker Hub

**Important**: Workers need to pull the image from a registry (Docker Hub). Local images won't work!

```bash
cd SAMPLE_TASKS/task1
chmod +x build.sh test_local.sh
./build.sh
```

The script will:
- Ask for your Docker Hub username
- Build the image as `<username>/cloudai-sample-task:latest`
- Optionally push it to Docker Hub (required for workers to access it)

### 2. Test Locally (Optional)

Before using with CloudAI, you can test locally:

```bash
./test_local.sh
```

### 3. Use in CloudAI

Once pushed to Docker Hub, assign it to a worker via master CLI:

```bash
master> task worker-1 <your-dockerhub-username>/cloudai-sample-task:latest
```

With custom resources:

```bash
master> task worker-1 <your-dockerhub-username>/cloudai-sample-task:latest -cpu_cores 2.0 -mem 1.0
```

## Alternative: Using Public Images

If you don't want to push to Docker Hub, you can use existing public images for testing:

```bash
# Simple hello-world test
master> task worker-1 docker.io/library/hello-world:latest

# Python environment
master> task worker-1 docker.io/library/python:3.9-slim
```

## How It Works

1. **Master sends task** → Master CLI sends task with Docker image name and resource specs
2. **Worker receives task** → Worker logs show full task details including all system requirements
3. **Worker pulls image** → Worker pulls the image from Docker Hub (or registry)
4. **Worker executes** → Worker runs the container and monitors execution
5. **Results reported** → Worker reports completion status back to master

## Customizing the Task

Modify `task.py` to:

- Perform actual ML training
- Process real datasets  
- Run data transformations
- Execute any Python workload

After changes:
```bash
./build.sh  # Rebuild and push
```

## Expected Output on Worker

When the task is received and executed, you'll see:

```
═══════════════════════════════════════════════════════
  📥 TASK RECEIVED FROM MASTER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Docker Image:      username/cloudai-sample-task:latest
  Command:           docker run --rm --cpus=1.0 --memory=0.5g ...
  Target Worker:     worker-1
───────────────────────────────────────────────────────
  System Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
  ✓ Task accepted - Starting execution...
═══════════════════════════════════════════════════════

[Task task-1730918400] Starting execution...
[Task task-1730918400] Pulling image: username/cloudai-sample-task:latest
[Task task-1730918400] Creating container...
[Task task-1730918400] Starting container: abc123def456
==================================================
CloudAI Sample Task - Starting Execution
==================================================

[Step 1/3] Initializing task environment...
[Step 2/3] Processing data...
  → Processing 87 data points...
  → Progress: 0/87
  → Progress: 20/87
  → Progress: 40/87
  → Progress: 60/87
  → Progress: 80/87
  → Completed: 87/87
[Step 3/3] Generating results...

==================================================
✓ Task completed successfully!
✓ Result saved to /output/result.json
==================================================

[Task task-1730918400] ✓ Completed successfully
```

## Output

The task writes a JSON result file with the following structure:

```json
{
  "task_status": "completed",
  "data_points_processed": 100,
  "computation_result": 5234,
  "average": 52.34,
  "execution_time": "approximately 10 seconds",
  "timestamp": 1634123456.789
}
```

This result file would be accessible to the master node via shared storage or S3 in a production setup.
