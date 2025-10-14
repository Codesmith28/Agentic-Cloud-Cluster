# Sample Task for CloudAI Testing

This directory contains a simple Python task that can be containerized and executed by CloudAI workers.

## What the Task Does

The task simulates a data processing workload:

1. Initializes the task environment
2. Processes random data points (50-150 items)
3. Computes statistics and saves results to `/output/result.json`
4. Prints progress and completion status

## Building the Docker Image

```bash
cd sample_task

# Build the image
docker build -t <your-dockerhub-username>/cloudai-sample-task:latest .

# Test locally (optional)
docker run --rm <your-dockerhub-username>/cloudai-sample-task:latest

# Push to Docker Hub
docker login
docker push <your-dockerhub-username>/cloudai-sample-task:latest
```

## Using in CloudAI

Once pushed to Docker Hub, you can assign this task via the master CLI:

```
master> task worker-1 docker.io/<your-dockerhub-username>/cloudai-sample-task:latest
```

## Customizing the Task

You can modify `task.py` to:

- Perform actual ML training
- Process real datasets
- Run data transformations
- Execute any Python workload

Just rebuild and push the image after making changes.

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
