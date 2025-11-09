# CloudAI Processes - Quick Start Summary

## 🎯 What You Have

20 Docker containerized processes (7 IO, 7 CPU, 6 GPU) with dynamic resource usage patterns designed for testing distributed computing workloads.

## ⚡ Quick Setup (3 Steps)

### 1. Set Your Docker Hub Username
```bash
export DOCKER_USERNAME="your_dockerhub_username"
```

### 2. Build All Images
```bash
cd /home/moin/Projects/CloudAI/processes
chmod +x *.sh
./build-local.sh
```

### 3. Push to Docker Hub
```bash
docker login
./push-images.sh
```

## 📦 Your Image Links

After pushing, your images will be available at:

### IO-Intensive (7 images)
```
docker.io/<username>/cloudai-io-intensive:1  # File operations
docker.io/<username>/cloudai-io-intensive:2  # Database operations
docker.io/<username>/cloudai-io-intensive:3  # Log processing
docker.io/<username>/cloudai-io-intensive:4  # Network I/O simulation
docker.io/<username>/cloudai-io-intensive:5  # Binary data processing
docker.io/<username>/cloudai-io-intensive:6  # Mixed access patterns
docker.io/<username>/cloudai-io-intensive:7  # CSV data processing
```

### CPU-Intensive (7 images)
```
docker.io/<username>/cloudai-cpu-intensive:1  # Matrix operations
docker.io/<username>/cloudai-cpu-intensive:2  # Prime numbers
docker.io/<username>/cloudai-cpu-intensive:3  # Sorting algorithms
docker.io/<username>/cloudai-cpu-intensive:4  # Cryptographic hashing
docker.io/<username>/cloudai-cpu-intensive:5  # Monte Carlo simulations
docker.io/<username>/cloudai-cpu-intensive:6  # Recursive algorithms
docker.io/<username>/cloudai-cpu-intensive:7  # Compression algorithms
```

### GPU-Intensive (6 images)
```
docker.io/<username>/cloudai-gpu-intensive:1  # CNN training
docker.io/<username>/cloudai-gpu-intensive:2  # Matrix operations (GPU)
docker.io/<username>/cloudai-gpu-intensive:3  # Image processing
docker.io/<username>/cloudai-gpu-intensive:4  # LSTM/RNN training
docker.io/<username>/cloudai-gpu-intensive:5  # Transformer models
docker.io/<username>/cloudai-gpu-intensive:6  # VAE/Generative models
```

## 🔗 Getting Image URLs

### On Docker Hub Web Interface
1. Go to: `https://hub.docker.com/r/<username>/cloudai-io-intensive`
2. Copy the pull command
3. The image URL is: `docker.io/<username>/<repo>:<tag>`

### Programmatically
Your images follow this pattern:
```
docker.io/<DOCKERHUB_USERNAME>/cloudai-<TYPE>:<NUMBER>
```

Example:
```
docker.io/johndoe/cloudai-cpu-intensive:3
```

## 🧪 Testing Locally

### Test IO Process
```bash
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-io-intensive:1
```

### Test CPU Process
```bash
docker run --rm -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-cpu-intensive:1
```

### Test GPU Process
```bash
docker run --rm --gpus all -v $(pwd)/results:/results \
  $DOCKER_USERNAME/cloudai-gpu-intensive:1
```

## 📋 Available Scripts

| Script | Purpose |
|--------|---------|
| `build-local.sh` | Build all 20 images locally |
| `push-images.sh` | Push all images to Docker Hub |
| `build-and-push.sh` | Build and push in one command |

## 📊 Process Characteristics

| Type | Count | Duration | CPU | Memory | GPU | I/O |
|------|-------|----------|-----|--------|-----|-----|
| IO   | 7     | 2-10 min | Low | Med-High | None | Very High |
| CPU  | 7     | 3-15 min | Very High | Med | None | Low |
| GPU  | 6     | 5-20 min | Med | High | Very High | Low |

## 🎲 Dynamic Features

✅ **Not Infinite**: Processes complete after randomized iterations
✅ **Dynamic Resources**: CPU, RAM, GPU usage varies during execution
✅ **Random Parameters**: Different problem sizes each run
✅ **Results Output**: Statistics saved to `/results` directory

## 📁 File Structure

```
processes/
├── io-intensive/
│   ├── Dockerfile
│   ├── io_process_1.py
│   ├── io_process_2.py
│   └── ... (7 total)
├── cpu-intensive/
│   ├── Dockerfile
│   ├── cpu_process_1.py
│   ├── cpu_process_2.py
│   └── ... (7 total)
├── gpu-intensive/
│   ├── Dockerfile
│   ├── gpu_process_1.py
│   ├── gpu_process_2.py
│   └── ... (6 total)
├── build-local.sh
├── push-images.sh
├── build-and-push.sh
├── README.md
└── DOCKER_HUB_GUIDE.md
```

## 🚀 Integration with CloudAI

Use these images in your master-worker task assignments:

```go
task := &pb.Task{
    TaskId:      "task-123",
    DockerImage: "johndoe/cloudai-cpu-intensive:3",  // Your username
    Command:     "python cpu_process_3.py",
    ReqCpu:      4,
    ReqMemory:   8,
    ReqGpu:      0,
}
```

## 🔧 Troubleshooting

### Build fails?
```bash
docker system prune -a
./build-local.sh
```

### Push fails?
```bash
docker logout
docker login
./push-images.sh
```

### GPU not detected?
```bash
# Verify NVIDIA Docker
docker run --rm --gpus all nvidia/cuda:12.1.0-base-ubuntu22.04 nvidia-smi
```

## 📚 Documentation

- **README.md**: Complete documentation
- **DOCKER_HUB_GUIDE.md**: Detailed push instructions
- **This file**: Quick reference

## ✅ Next Steps

1. **Set your Docker Hub username**
   ```bash
   export DOCKER_USERNAME="your_username"
   ```

2. **Build images**
   ```bash
   ./build-local.sh
   ```

3. **Login to Docker Hub**
   ```bash
   docker login
   ```

4. **Push images**
   ```bash
   ./push-images.sh
   ```

5. **Get your image links**
   - Visit: `https://hub.docker.com/u/<username>`
   - Your images: `docker.io/<username>/cloudai-<type>:<number>`

6. **Use in CloudAI**
   - Update master configuration with your image URLs
   - Test task assignment
   - Monitor execution

## 💡 Pro Tips

- **First build takes longest** (especially GPU images ~6-8 GB)
- **Subsequent builds are fast** (Docker layer caching)
- **Test locally before pushing** to catch issues early
- **Use specific tags** for production (not just `:latest`)
- **Monitor during execution** with `docker stats`

## 🆘 Need Help?

1. Check `README.md` for detailed documentation
2. Check `DOCKER_HUB_GUIDE.md` for push instructions
3. View container logs: `docker logs <container_id>`
4. Test locally before deploying

---

**Total Images: 20** (7 IO + 7 CPU + 6 GPU)
**Build Time: ~15-30 minutes** (depends on system)
**Total Size: ~15-20 GB** (GPU images are largest)
