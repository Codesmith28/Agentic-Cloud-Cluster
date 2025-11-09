# CloudAI Process Containers

This directory contains 20 different containerized processes designed for distributed computing workload testing and benchmarking.

## 📦 Process Types

### IO-Intensive Processes (7 processes)
Dynamic file operations, database operations, and network I/O patterns with varying RAM usage.

1. **io_process_1**: File Writing and Reading with Dynamic Memory Growth
2. **io_process_2**: Database-like Operations with Random Access (SQLite)
3. **io_process_3**: Log Processing and Text Analysis
4. **io_process_4**: Network-like I/O Simulation with Buffering
5. **io_process_5**: Image/Binary Data Processing with Compression
6. **io_process_6**: Sequential and Random File Access Mix
7. **io_process_7**: CSV Data Processing with Aggregations

### CPU-Intensive Processes (7 processes)
Computational workloads with dynamic CPU usage patterns.

1. **cpu_process_1**: Matrix Operations and Linear Algebra
2. **cpu_process_2**: Prime Number Generation and Factorization
3. **cpu_process_3**: Sorting Algorithms Benchmark
4. **cpu_process_4**: Cryptographic Hashing and String Processing
5. **cpu_process_5**: Monte Carlo Simulations
6. **cpu_process_6**: Recursive Algorithms and Tree Traversal
7. **cpu_process_7**: Compression Algorithms and Data Encoding

### GPU-Intensive Processes (6 processes)
GPU-accelerated workloads using PyTorch with dynamic GPU memory usage.

1. **gpu_process_1**: Neural Network Training with Dynamic Batch Sizes (CNN)
2. **gpu_process_2**: Large Matrix Operations on GPU
3. **gpu_process_3**: Image Processing and Convolutions
4. **gpu_process_4**: Recurrent Neural Network with LSTMs
5. **gpu_process_5**: Transformer Models and Attention Mechanisms
6. **gpu_process_6**: Generative Models and Variational Autoencoders (VAE)

## 🚀 Quick Start

### Prerequisites
- Docker installed on your system
- Docker Hub account (for pushing images)
- For GPU processes: NVIDIA GPU with CUDA support and nvidia-docker runtime

### Step 1: Set Your Docker Hub Username

```bash
export DOCKER_USERNAME="your_dockerhub_username"
```

Or edit the scripts and replace `your_dockerhub_username` with your actual username.

### Step 2: Build All Images

```bash
cd processes
chmod +x build-local.sh
./build-local.sh
```

This will build all 20 images locally.

### Step 3: Login to Docker Hub

```bash
docker login
```

Enter your Docker Hub credentials when prompted.

### Step 4: Push Images to Docker Hub

```bash
chmod +x push-images.sh
./push-images.sh
```

Or use the combined build and push script:

```bash
chmod +x build-and-push.sh
./build-and-push.sh
```

## 📋 Detailed Instructions

### Building Specific Process Types

#### Build Only IO-Intensive Images
```bash
cd processes/io-intensive
docker build -t $DOCKER_USERNAME/cloudai-io-intensive:1 .
```

#### Build Only CPU-Intensive Images
```bash
cd processes/cpu-intensive
docker build -t $DOCKER_USERNAME/cloudai-cpu-intensive:1 .
```

#### Build Only GPU-Intensive Images
```bash
cd processes/gpu-intensive
docker build -t $DOCKER_USERNAME/cloudai-gpu-intensive:1 .
```

### Running Processes Locally

#### Run IO Process
```bash
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-io-intensive:1 python io_process_1.py
```

#### Run CPU Process
```bash
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-cpu-intensive:1 python cpu_process_1.py
```

#### Run GPU Process
```bash
docker run --rm --gpus all -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-gpu-intensive:1 python3 gpu_process_1.py
```

### Running Different Process Variants

Each Docker image contains multiple process variants. You can specify which one to run:

```bash
# Run IO process variant 3
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-io-intensive:1 python io_process_3.py

# Run CPU process variant 5
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-cpu-intensive:1 python cpu_process_5.py

# Run GPU process variant 4
docker run --rm --gpus all -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-gpu-intensive:1 python3 gpu_process_4.py
```

## 🌐 Getting Docker Hub Image Links

After pushing your images to Docker Hub, they will be available at:

### Format
```
docker.io/<username>/cloudai-<type>:<tag>
```

### Examples
If your Docker Hub username is `johndoe`, your images will be:

**IO-Intensive:**
```
docker.io/johndoe/cloudai-io-intensive:1
docker.io/johndoe/cloudai-io-intensive:2
...
docker.io/johndoe/cloudai-io-intensive:7
docker.io/johndoe/cloudai-io-intensive:latest
```

**CPU-Intensive:**
```
docker.io/johndoe/cloudai-cpu-intensive:1
docker.io/johndoe/cloudai-cpu-intensive:2
...
docker.io/johndoe/cloudai-cpu-intensive:7
docker.io/johndoe/cloudai-cpu-intensive:latest
```

**GPU-Intensive:**
```
docker.io/johndoe/cloudai-gpu-intensive:1
docker.io/johndoe/cloudai-gpu-intensive:2
...
docker.io/johndoe/cloudai-gpu-intensive:6
docker.io/johndoe/cloudai-gpu-intensive:latest
```

### Web URLs
View your images on Docker Hub at:
- `https://hub.docker.com/r/<username>/cloudai-io-intensive`
- `https://hub.docker.com/r/<username>/cloudai-cpu-intensive`
- `https://hub.docker.com/r/<username>/cloudai-gpu-intensive`

### Pulling Images
Anyone can pull your images (if public) using:
```bash
docker pull <username>/cloudai-io-intensive:1
docker pull <username>/cloudai-cpu-intensive:1
docker pull <username>/cloudai-gpu-intensive:1
```

## 📊 Process Characteristics

### Resource Usage Patterns

| Process Type | CPU | Memory | GPU | Disk I/O | Duration |
|-------------|-----|--------|-----|----------|----------|
| IO-Intensive | Low-Medium | Medium-High | None | Very High | 2-10 min |
| CPU-Intensive | Very High | Medium | None | Low | 3-15 min |
| GPU-Intensive | Medium | High | Very High | Low | 5-20 min |

### Dynamic Behavior

All processes have dynamic characteristics:
- **Variable Duration**: Processes don't run infinitely; they complete after a randomized number of operations
- **Dynamic Resource Usage**: RAM, CPU, and GPU usage varies throughout execution
- **Randomized Parameters**: Each run uses different problem sizes and configurations
- **Results Output**: All processes write statistics and results to `/results` directory

## 🛠️ Advanced Usage

### Custom Tagging
```bash
# Build with custom tag
docker build -t $DOCKER_USERNAME/cloudai-io-intensive:v1.0 \
  -f io-intensive/Dockerfile io-intensive/

# Push custom tag
docker push $DOCKER_USERNAME/cloudai-io-intensive:v1.0
```

### Multi-Architecture Builds
```bash
# Build for multiple architectures
docker buildx build --platform linux/amd64,linux/arm64 \
  -t $DOCKER_USERNAME/cloudai-io-intensive:1 \
  --push \
  -f io-intensive/Dockerfile io-intensive/
```

### Resource Limits
```bash
# Limit CPU and memory
docker run --rm \
  --cpus="2.0" \
  --memory="4g" \
  -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-cpu-intensive:1

# Limit GPU memory
docker run --rm \
  --gpus '"device=0"' \
  --memory="8g" \
  -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-gpu-intensive:1
```

## 📈 Monitoring

### View Container Stats
```bash
# While container is running
docker stats <container_id>
```

### GPU Monitoring (for GPU processes)
```bash
# In another terminal
watch -n 1 nvidia-smi
```

### Check Results
```bash
# After process completes
ls -lh results/
cat results/cpu_stats.json
cat results/gpu_stats.json
cat results/io_stats.json
```

## 🔧 Troubleshooting

### Docker Build Fails
```bash
# Clean up and rebuild
docker system prune -a
./build-local.sh
```

### Push to Docker Hub Fails
```bash
# Re-login
docker logout
docker login

# Verify login
docker info | grep Username
```

### GPU Process Doesn't Use GPU
```bash
# Verify NVIDIA Docker runtime
docker run --rm --gpus all nvidia/cuda:12.1.0-base-ubuntu22.04 nvidia-smi

# Check CUDA availability in container
docker run --rm --gpus all $DOCKER_USERNAME/cloudai-gpu-intensive:1 \
  python3 -c "import torch; print(torch.cuda.is_available())"
```

### Container Exits Immediately
```bash
# Check logs
docker logs <container_id>

# Run interactively for debugging
docker run --rm -it -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-io-intensive:1 /bin/bash
```

## 📝 Image Tags Reference

### Tag Scheme
- `1-7`: Individual process variants (IO and CPU)
- `1-6`: Individual process variants (GPU)
- `latest`: Points to process variant 1 for each type

### All Available Tags

**IO-Intensive**: `1, 2, 3, 4, 5, 6, 7, latest`
**CPU-Intensive**: `1, 2, 3, 4, 5, 6, 7, latest`
**GPU-Intensive**: `1, 2, 3, 4, 5, 6, latest`

## 🤝 Integration with CloudAI Master/Worker

These images are designed to work with the CloudAI master-worker architecture:

```go
// Example task assignment
task := &pb.Task{
    TaskId: "task-123",
    DockerImage: "johndoe/cloudai-cpu-intensive:3",
    Command: "python cpu_process_3.py",
    ReqCpu: 4,
    ReqMemory: 8,
    ReqGpu: 0,
}
```

## 📦 Image Sizes

Approximate sizes:
- **IO-Intensive**: ~200 MB
- **CPU-Intensive**: ~250 MB
- **GPU-Intensive**: ~6-8 GB (includes CUDA and PyTorch)

## 🔐 Making Images Public/Private

By default, pushed images are public. To make them private:

1. Go to Docker Hub: `https://hub.docker.com/r/<username>/cloudai-io-intensive`
2. Click "Settings"
3. Change visibility to "Private"

## 📄 License

These processes are part of the CloudAI project and are provided as-is for testing and benchmarking purposes.

## 🆘 Support

For issues or questions:
1. Check the troubleshooting section above
2. Review container logs: `docker logs <container_id>`
3. Test processes locally before pushing to production
