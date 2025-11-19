# CloudAI Process Library

This directory contains a comprehensive collection of test processes for benchmarking and testing the CloudAI distributed computing system.

## Overview

The process library includes **72 total processes** across three categories:
- **32 CPU-intensive processes** (cpu_process_1.py through cpu_process_32.py)
- **27 IO-intensive processes** (io_process_1.py through io_process_27.py)
- **20 Mixed-intensive processes** (mixed_process_1.py through mixed_process_20.py, combining CPU and IO operations)

## Process Categories

### CPU-Intensive Processes (`cpu-intensive/`)

These processes focus on computational workloads that heavily utilize CPU resources:

1. **cpu_process_1.py** - Matrix Operations and Linear Algebra
2. **cpu_process_2.py** - Fast Fourier Transforms
3. **cpu_process_3.py** - Image Processing (Convolutions)
4. **cpu_process_4.py** - Cryptographic Operations
5. **cpu_process_5.py** - Monte Carlo Simulations
6. **cpu_process_6.py** - Recursive Algorithms and Tree Traversal
7. **cpu_process_7.py** - Sorting and Searching Algorithms
8. **cpu_process_8.py** - Graph Theory Computations
9. **cpu_process_9.py** - Numerical Integration
10. **cpu_process_10.py** - Prime Number Generation
11. **cpu_process_11.py** - Polynomial Operations
12. **cpu_process_12.py** - Statistical Computations
13. **cpu_process_13.py** - Prime Number Generation and Sieve Algorithms
14. **cpu_process_14.py** - Numerical Integration and Calculus
15. **cpu_process_15.py** - Cryptographic Hash Computing
16. **cpu_process_16.py** - String Pattern Matching and Search
17. **cpu_process_17.py** - Sorting Algorithm Benchmarks
18. **cpu_process_18.py** - Monte Carlo Simulations
19. **cpu_process_19.py** - Graph Algorithms (Dijkstra, BFS, DFS)
20. **cpu_process_20.py** - Fourier Transforms and Signal Processing
21-32. Various intensive computational operations

**Key Features:**
- Generates performance metrics (GFLOPS, operations/second)
- Tracks CPU utilization and memory consumption
- Produces JSON result files with detailed statistics
- Some processes generate visualization graphs (PNG images)

### IO-Intensive Processes (`io-intensive/`)

These processes focus on disk I/O, file operations, and data transfer:

1. **io_process_1.py** - File Writing and Reading with Dynamic Memory Growth
2. **io_process_2.py** - Sequential File Access Patterns
3. **io_process_3.py** - Random File Access
4. **io_process_4.py** - Large File Streaming
5. **io_process_5.py** - Image/Binary Data Processing with Compression
6. **io_process_6.py** - Database Simulation
7. **io_process_7.py** - Log File Generation
8-27. Various I/O intensive operations including:
   - JSON and CSV processing
   - File encryption/decryption
   - Archive creation and extraction
   - Stream processing
   - Directory scanning
   - File synchronization
   - Data serialization

**Key Features:**
- Measures bytes read/written
- Tracks I/O throughput (MB/s)
- Tests various access patterns (sequential, random, streaming)
- Simulates real-world file operations
- Generates compressed and uncompressed data

### Mixed-Intensive Processes (`mixed-intensive/`)

These processes combine both CPU and I/O operations, simulating real-world data pipelines:

1. **mixed_process_1.py** - Data Analysis Pipeline
2. **mixed_process_2.py** - Image Processing Pipeline
3. **mixed_process_3.py** - Log Analysis
4. **mixed_process_4.py** - Scientific Data Processing
5. **mixed_process_5.py** - Machine Learning Prep
6. **mixed_process_6.py** - Video Frame Extraction
7. **mixed_process_7.py** - Text Mining
8. **mixed_process_8.py** - Cryptocurrency Mining Simulation
9. **mixed_process_9.py** - Weather Data Analysis
10. **mixed_process_10.py** - Genome Sequencing
11. **mixed_process_11.py** - Financial Modeling
12. **mixed_process_12.py** - Network Traffic Analysis
13. **mixed_process_13.py** - Audio Processing
14. **mixed_process_14.py** - Database Indexing
15. **mixed_process_15.py** - Compression Benchmark
16. **mixed_process_16.py** - Rendering Pipeline
17. **mixed_process_17.py** - Time Series Analysis
18. **mixed_process_18.py** - MapReduce Simulation
19. **mixed_process_19.py** - Web Scraping Simulation
20. **mixed_process_20.py** - Backup with Verification

**Key Features:**
- Three-phase execution: Data loading → Processing → Results writing
- Combines file I/O with numerical computations
- Generates multiple output formats (JSON, CSV, PNG)
- Tracks both CPU and I/O metrics
- Simulates real-world data science workflows

## Output Files

All processes write their results to the `/results` directory (mapped from the worker's results volume):

### Common Output Files:
- `*_stats.json` - Performance metrics and resource usage statistics
- `*.csv` - Tabular data results
- `*.png` - Visualization graphs (for processes that generate plots)
- `*.json` - Detailed computation results

### Statistics Tracked:
- Process type and execution duration
- CPU utilization (peak percentage)
- Memory usage (peak MB)
- Operations completed (varies by process type)
- Bytes read/written (I/O processes)
- GFLOPS or other performance metrics (CPU processes)

## Docker Images

Each process category has its own Dockerfile:

```bash
# CPU-intensive processes
docker build -t cloudai-cpu-intensive:latest ./cpu-intensive

# IO-intensive processes
docker build -t cloudai-io-intensive:latest ./io-intensive

# Mixed-intensive processes
docker build -t cloudai-mixed-intensive:latest ./mixed-intensive
```

## Running Processes

### Direct Execution:
```bash
python3 cpu-intensive/cpu_process_1.py
python3 io-intensive/io_process_1.py
python3 mixed-intensive/mixed_process_1.py
```

### Via Docker:
```bash
docker run -v $(pwd)/results:/results cloudai-cpu-intensive:latest python3 /app/cpu_process_1.py
```

### Via CloudAI System:
Use the CloudAI CLI to dispatch tasks to workers:
```bash
# CPU task
./master-cli dispatch -w worker1 -t cpu_intensive -c "python3 /app/cpu_process_1.py"

# IO task
./master-cli dispatch -w worker2 -t io_intensive -c "python3 /app/io_process_1.py"

# Mixed task
./master-cli dispatch -w worker3 -t mixed -c "python3 /app/mixed_process_1.py"
```

## Dependencies

All processes require:
- Python 3.11+
- numpy
- psutil
- matplotlib (for visualization processes)

These dependencies are included in the Docker images.

## Performance Characteristics

### CPU-Intensive Processes:
- **Duration**: Typically 30 seconds to 5 minutes
- **CPU Usage**: 80-100% of available cores
- **Memory**: 100MB - 2GB depending on matrix sizes
- **I/O**: Minimal (<100MB total)

### IO-Intensive Processes:
- **Duration**: Typically 1-10 minutes
- **CPU Usage**: 10-40%
- **Memory**: 50MB - 1GB
- **I/O**: Heavy (500MB - 10GB of reads/writes)

### Mixed-Intensive Processes:
- **Duration**: Typically 2-15 minutes
- **CPU Usage**: 40-80%
- **Memory**: 200MB - 3GB
- **I/O**: Moderate to Heavy (200MB - 5GB)

## Customization

All processes use randomization for:
- Number of iterations
- Data sizes
- Operation complexity

This ensures each run produces different workload characteristics, making them suitable for realistic testing scenarios.

## Use Cases

1. **Performance Testing**: Measure worker performance under various workload types
2. **Resource Management**: Test CPU and memory allocation strategies
3. **Scheduling**: Evaluate task scheduling algorithms
4. **Load Balancing**: Test distribution of mixed workloads
5. **Fault Tolerance**: Verify system behavior with long-running tasks
6. **Scalability**: Assess system performance as workload increases

## Notes

- All processes are designed to be self-contained and stateless
- Results are written to `/results` which should be mounted as a volume
- Processes include proper error handling and logging
- Random seeds ensure variability between runs
- Process duration and resource usage vary based on random parameters

## Contributing

To add new processes:
1. Follow the existing naming convention
2. Include proper docstrings
3. Track relevant performance metrics
4. Write results to `/results` directory
5. Update this README with process description
