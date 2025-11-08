# CloudAI Sample Tasks

This directory contains sample tasks that can be executed by CloudAI workers to test and demonstrate the task sending/receiving functionality.

## Available Tasks

### Task 1: Data Processing Sample
**Location**: `task1/`

A simple Python-based task that simulates a data processing workload. Perfect for testing the CloudAI task execution flow.

**Features**:
- Processes random data points (50-150 items)
- Computes statistics
- Saves results to `/output/result.json`
- Displays progress during execution

## Quick Start

### 1. Build the Sample Task

**⚠️ IMPORTANT**: Workers pull images from Docker registries (like Docker Hub). Local-only images won't work when master and worker are on different machines!

```bash
cd SAMPLE_TASKS/task1
chmod +x build.sh test_local.sh
./build.sh
```

The script will:
- Ask for your Docker Hub username
- Build the image as `<username>/cloudai-sample-task:latest`
- Optionally push to Docker Hub (required for remote workers)

### 2. Test Locally (Optional)

```bash
./test_local.sh
```

### 3. Use in CloudAI

Once pushed to Docker Hub:

```
master> task worker-1 <your-dockerhub-username>/cloudai-sample-task:latest
```

With custom resources:

```
master> task worker-1 <your-dockerhub-username>/cloudai-sample-task:latest -cpu_cores 2.0 -mem 1.0
```

### Alternative: Use Public Images for Quick Testing

Don't want to push to Docker Hub? Use these public images:

```
# Simple test
master> task worker-1 docker.io/library/hello-world:latest

# Python environment  
master> task worker-1 docker.io/library/python:3.9-slim

# Ubuntu
master> task worker-1 docker.io/library/ubuntu:latest
```

## Expected Output

### On Master CLI:
```
═══════════════════════════════════════════════════════
  📤 SENDING TASK TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Target Worker:     worker-1
  Docker Image:      cloudai-sample-task:latest
  Command:           docker run --rm --cpus=1.0 --memory=0.5g cloudai-sample-task:latest
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════

✅ Task task-1730918400 assigned successfully!
```

### On Worker Logs:
```
═══════════════════════════════════════════════════════
  📥 TASK RECEIVED FROM MASTER
═══════════════════════════════════════════════════════
  Task ID:           task-1730918400
  Docker Image:      cloudai-sample-task:latest
  Command:           docker run --rm --cpus=1.0 --memory=0.5g cloudai-sample-task:latest
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
[Task task-1730918400] Pulling image: cloudai-sample-task:latest
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

## Creating Your Own Tasks

### 1. Create a New Task Directory

```bash
mkdir -p SAMPLE_TASKS/my-task
cd SAMPLE_TASKS/my-task
```

### 2. Create Your Task Script

Create your main script (e.g., `task.py`, `task.sh`, etc.):

```python
# task.py
print("Hello from my custom task!")
# Your task logic here...
```

### 3. Create a Dockerfile

```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY task.py .

CMD ["python", "-u", "task.py"]
```

### 4. Build and Test

```bash
docker build -t my-custom-task:latest .
docker run --rm my-custom-task:latest
```

### 5. Use in CloudAI

```
master> task worker-1 my-custom-task:latest -cpu_cores 2.0 -mem 1.0
```

## Task Development Best Practices

1. **Use unbuffered output**: Add `-u` flag to Python or flush output regularly
2. **Print progress**: Show task progress for better monitoring
3. **Handle errors gracefully**: Include proper error handling and logging
4. **Exit codes**: Use proper exit codes (0 for success, non-zero for failure)
5. **Resource awareness**: Be mindful of the resources you request
6. **Output directory**: Use `/output` for task results if needed

## Testing Checklist

- [ ] Task builds successfully locally
- [ ] Task runs successfully with `docker run`
- [ ] Task displays proper output
- [ ] Master CLI accepts and sends the task
- [ ] Worker receives and displays task details correctly
- [ ] Worker executes the task successfully
- [ ] Task completion is reported to master

## Troubleshooting

### Image not found
```
Error: No such image: cloudai-sample-task:latest
```
**Solution**: Build the image first with `./build.sh`

### Permission denied
```
bash: ./build.sh: Permission denied
```
**Solution**: Make the script executable with `chmod +x build.sh`

### Task fails to execute
Check worker logs for detailed error messages. Common issues:
- Insufficient resources allocated
- Missing dependencies in Docker image
- Network issues pulling the image

## Next Steps

1. Build the sample task: `cd task1 && ./build.sh`
2. Test locally: `./test_local.sh`
3. Start master and worker nodes
4. Register worker with master
5. Assign the task: `task worker-1 cloudai-sample-task:latest`
6. Monitor both master and worker logs

Enjoy testing CloudAI's task execution capabilities!
