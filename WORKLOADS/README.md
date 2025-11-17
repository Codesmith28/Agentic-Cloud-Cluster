# CloudAI Workload Test Suite

This directory contains 30 diverse Python workload scripts designed to test the CloudAI distributed task scheduling system across various resource profiles and use cases.

## Overview

- **Total Workloads**: 30
- **Workload Types**: 6 categories
- **File Output**: 12 workloads generate CSV files for analysis
- **Language**: Python 3
- **Execution**: Run via CloudAI master node with Docker containerization

## Workload Categories

### 1. CPU-Light (5 workloads)
Lightweight tasks with minimal resource requirements, suitable for quick validation.

| # | Name | Description | Output File |
|---|------|-------------|-------------|
| 01 | health-check | System status verification | None |
| 02 | json-parser | JSON data parsing and validation | parsed_data.csv ✓ |
| 03 | log-analyzer | Log file analysis with pattern matching | None |
| 04 | string-processor | String manipulation and analysis | string_analysis.csv ✓ |
| 05 | ping-test | Network connectivity simulation | None |

**Typical Resources**: Low CPU (1-2 cores), Low Memory (256MB-512MB)

### 2. CPU-Heavy (5 workloads)
Computationally intensive tasks that stress CPU resources.

| # | Name | Description | Output File |
|---|------|-------------|-------------|
| 06 | monte-carlo | Monte Carlo simulation for π estimation | simulation_results.csv ✓ |
| 07 | prime-generator | Prime number generation | primes.csv ✓ |
| 08 | matrix-operations | Matrix multiplication and operations | matrix_results.csv ✓ |
| 09 | fibonacci | Fibonacci sequence computation | None |
| 10 | sort-benchmark | Sorting algorithm performance test | sort_benchmark.csv ✓ |

**Typical Resources**: High CPU (4-8 cores), Medium Memory (1GB-2GB)

### 3. Memory-Heavy (5 workloads)
Tasks that require significant memory allocation for large dataset processing.

| # | Name | Description | Output File |
|---|------|-------------|-------------|
| 11 | dataset-merge | Large dataset merging | merged_data.csv ✓ |
| 12 | data-aggregation | In-memory data aggregation | aggregated_results.csv ✓ |
| 13 | graph-analysis | Graph traversal and analysis | None |
| 14 | timeseries | Time series data processing | timeseries_summary.csv ✓ |
| 15 | dataset-join | Multi-dataset join operations | None |

**Typical Resources**: Medium CPU (2-4 cores), High Memory (4GB-8GB)

### 4. GPU-Inference (5 workloads)
Simulated GPU inference tasks for machine learning models.

| # | Name | Description | Output File |
|---|------|-------------|-------------|
| 16 | text-embedding | Text embedding generation | embeddings.csv ✓ |
| 17 | image-classification | Batch image classification | None |
| 18 | speech-recognition | Audio transcription simulation | transcriptions.csv ✓ |
| 19 | object-detection | Object detection in images | detections.csv ✓ |
| 20 | sentiment-analysis | Text sentiment analysis | sentiment_analysis.csv ✓ |

**Typical Resources**: Medium CPU (2-4 cores), Medium Memory (2GB-4GB), GPU (1 GPU, 4GB VRAM)

### 5. GPU-Training (5 workloads)
Long-running GPU training simulations with high resource demands.

| # | Name | Description | Output File |
|---|------|-------------|-------------|
| 21 | neural-network | Neural network training (ResNet-50) | training_metrics.csv ✓ |
| 22 | llm-finetuning | Large language model fine-tuning | finetuning_checkpoints.csv ✓ |
| 23 | reinforcement-learning | RL training loop (PPO algorithm) | None |
| 24 | gan | GAN training (DCGAN architecture) | gan_training_log.csv ✓ |
| 25 | hyperparameter-tuning | Hyperparameter search with multiple trials | hyperparameter_results.csv ✓ |

**Typical Resources**: High CPU (4-8 cores), High Memory (8GB-16GB), GPU (1-2 GPUs, 8GB+ VRAM)

### 6. Mixed (5 workloads)
Balanced workloads with moderate CPU and memory requirements.

| # | Name | Description | Output File |
|---|------|-------------|-------------|
| 26 | etl-pipeline | Extract-Transform-Load pipeline | None |
| 27 | ml-preprocessing | ML data preprocessing with feature engineering | preprocessed_features.csv ✓ |
| 28 | statistical-analysis | Statistical tests and hypothesis testing | None |
| 29 | image-processing | Batch image processing with filters | processing_report.csv ✓ |
| 30 | data-validation | Data quality checks and validation | validation_report.csv ✓ |

**Typical Resources**: Medium CPU (2-4 cores), Medium Memory (2GB-4GB)

## Output Files Summary

**Total workloads with file output: 22**

| Workload Type | Files Generated | Count |
|---------------|-----------------|-------|
| cpu-light | 02, 04 | 2 |
| cpu-heavy | 06, 07, 08, 10 | 4 |
| memory-heavy | 11, 12, 14 | 3 |
| gpu-inference | 16, 18, 19, 20 | 4 |
| gpu-training | 21, 22, 24, 25 | 4 |
| mixed | 27, 29, 30 | 3 |

All output files are generated in CSV format and saved to `/output/` directory within the container.

## Usage

### Running Individual Workloads

```bash
# CPU-light example
python3 01_cpu-light_health-check.py

# GPU-training example
python3 21_gpu-training_neural-network.py
```

### Running via CloudAI System

```bash
# Submit a task using the CloudAI CLI
./master/masterNode submit-task \
  --task-id "test-task-1" \
  --workload-path "/path/to/WORKLOADS/06_cpu-heavy_monte-carlo.py" \
  --cpu 4 \
  --memory 2048 \
  --timeout 600
```

### Batch Submission

```bash
# Submit all CPU-heavy workloads
for i in {06..10}; do
  file=$(ls ${i}_cpu-heavy_*.py)
  ./master/masterNode submit-task --task-id "cpu-heavy-$i" --workload-path "$PWD/$file" --cpu 4 --memory 2048
done
```

## File Naming Convention

All files follow this pattern:
```
<number>_<type>_<descriptive-name>.py
```

- **number**: Sequential identifier (01-30)
- **type**: Workload category (cpu-light, cpu-heavy, memory-heavy, gpu-inference, gpu-training, mixed)
- **descriptive-name**: Short description of the workload

## Workload Structure

Each workload script follows this template:

```python
#!/usr/bin/env python3
"""
Workload Type: <category>
Description: <what the workload does>
Resource Requirements: <CPU/Memory/GPU needs>
Output: <output file name or "None (logs only)">
"""

import datetime
import time

def run_workload():
    print(f"[{datetime.datetime.now()}] Starting <workload name>...")
    # Workload logic here
    print(f"[{datetime.datetime.now()}] <workload name> completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_workload())
```

## Resource Allocation Guidelines

### Recommended Resource Allocations

| Category | CPU Cores | Memory (GB) | Storage (GB) | GPU |
|----------|-----------|-------------|--------------|-----|
| cpu-light | 1-2 | 0.5-1 | 1 | 0 |
| cpu-heavy | 4-8 | 1-2 | 1 | 0 |
| memory-heavy | 2-4 | 4-8 | 2 | 0 |
| gpu-inference | 2-4 | 2-4 | 2 | 1 |
| gpu-training | 4-8 | 8-16 | 5 | 1-2 |
| mixed | 2-4 | 2-4 | 2 | 0 |

### Timeout Recommendations

| Category | Typical Duration | Recommended Timeout |
|----------|------------------|---------------------|
| cpu-light | 5-30 seconds | 60 seconds |
| cpu-heavy | 1-5 minutes | 600 seconds |
| memory-heavy | 30s-3 minutes | 300 seconds |
| gpu-inference | 30s-2 minutes | 300 seconds |
| gpu-training | 3-10 minutes | 900 seconds |
| mixed | 1-5 minutes | 600 seconds |

## Testing Scenarios

### 1. Basic Functionality Test
Run all CPU-light workloads to verify system is operational:
```bash
for i in {01..05}; do python3 ${i}_cpu-light_*.py; done
```

### 2. Resource Stress Test
Run multiple CPU-heavy workloads simultaneously:
```bash
# Submit 5 parallel tasks
for i in {06..10}; do
  ./master/masterNode submit-task --task-id "stress-$i" --workload-path "$PWD/$(ls ${i}_*.py)" --cpu 4 --memory 2048 &
done
```

### 3. GPU Utilization Test
Run GPU-training workloads sequentially:
```bash
for i in {21..25}; do
  python3 $(ls ${i}_gpu-training_*.py)
done
```

### 4. Mixed Workload Test
Submit diverse workloads to test scheduling:
```bash
# Mix of different types
./master/masterNode submit-task --task-id "mix-1" --workload-path "$PWD/01_cpu-light_health-check.py" --cpu 1 --memory 512
./master/masterNode submit-task --task-id "mix-2" --workload-path "$PWD/11_memory-heavy_dataset-merge.py" --cpu 2 --memory 4096
./master/masterNode submit-task --task-id "mix-3" --workload-path "$PWD/21_gpu-training_neural-network.py" --cpu 4 --memory 8192 --gpu 1
```

## Output Analysis

### Collecting Results

After execution, output files are stored in the container's `/output/` directory. To extract them:

```bash
# Copy outputs from container
docker cp <container-id>:/output/. ./results/

# List all generated CSV files
ls -lh ./results/*.csv
```

### Sample Analysis

```python
import pandas as pd

# Analyze training metrics
df = pd.read_csv('results/training_metrics.csv')
print(df.describe())
print(f"Best epoch: {df.loc[df['accuracy'].idxmax()]}")

# Compare sort benchmarks
df_sort = pd.read_csv('results/sort_benchmark.csv')
print(df_sort.groupby('algorithm')['execution_time'].mean())
```

## Troubleshooting

### Common Issues

**Issue**: Workload exits with error code
- Check container logs: `docker logs <container-id>`
- Verify Python environment has required packages
- Check resource allocation meets minimum requirements

**Issue**: Output file not generated
- Verify `/output/` directory exists and is writable
- Check workload completed successfully (exit code 0)
- Some workloads produce logs only (see "Output File" column)

**Issue**: Workload times out
- Increase timeout value for resource-intensive tasks
- Check system resources are actually available
- Review workload logs for bottlenecks

## Performance Benchmarking

Use these workloads to benchmark CloudAI system performance:

1. **Throughput**: Submit all cpu-light workloads, measure total completion time
2. **Latency**: Measure time from submission to completion for single task
3. **Concurrency**: Run multiple workloads simultaneously, measure throughput degradation
4. **Resource Utilization**: Monitor CPU/Memory/GPU usage during execution
5. **Failure Recovery**: Cancel tasks mid-execution, verify cleanup

## Contributing

To add new workloads:

1. Follow the file naming convention
2. Include docstring header with type, description, requirements, output
3. Use structured logging with timestamps
4. Return exit code 0 on success
5. Save output to `/output/` if applicable
6. Update this README with workload details

## License

Part of the CloudAI distributed task scheduling system.
