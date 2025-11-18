# Docker Build and Push Guide

## Quick Start

### Build All Images Locally
```bash
cd /home/moin/Projects/CloudAI/processes
./build-local.sh
```

### Push All Images to Docker Hub
```bash
# First, login to Docker Hub
docker login

# Then push all images
./push-images.sh
```

## What Gets Built

### Image Counts
- **IO-Intensive**: 27 processes + 1 latest tag = 28 images
- **CPU-Intensive**: 32 processes + 1 latest tag = 33 images
- **Mixed-Intensive**: 20 processes + 1 latest tag = 21 images
- **GPU-Intensive**: 6 processes + 1 latest tag = 7 images
- **Total**: 89 Docker images

### Image Tags

Each process gets its own numbered tag:

```bash
# IO-Intensive
moinvinchhi/cloudai-io-intensive:1
moinvinchhi/cloudai-io-intensive:2
...
moinvinchhi/cloudai-io-intensive:27
moinvinchhi/cloudai-io-intensive:latest

# CPU-Intensive
moinvinchhi/cloudai-cpu-intensive:1
moinvinchhi/cloudai-cpu-intensive:2
...
moinvinchhi/cloudai-cpu-intensive:32
moinvinchhi/cloudai-cpu-intensive:latest

# Mixed-Intensive
moinvinchhi/cloudai-mixed-intensive:1
moinvinchhi/cloudai-mixed-intensive:2
...
moinvinchhi/cloudai-mixed-intensive:20
moinvinchhi/cloudai-mixed-intensive:latest

# GPU-Intensive
moinvinchhi/cloudai-gpu-intensive:1
...
moinvinchhi/cloudai-gpu-intensive:6
moinvinchhi/cloudai-gpu-intensive:latest
```

## Usage Examples

### Run Specific Process
```bash
# Run CPU process 14 (Numerical Integration)
docker run -v $(pwd)/results:/results \
    moinvinchhi/cloudai-cpu-intensive:14

# Run IO process 8 (JSON Processing)
docker run -v $(pwd)/results:/results \
    moinvinchhi/cloudai-io-intensive:8

# Run Mixed process 5 (Machine Learning Prep)
docker run -v $(pwd)/results:/results \
    moinvinchhi/cloudai-mixed-intensive:5
```

### Run Latest Version
```bash
# Run latest CPU-intensive process (defaults to process 1)
docker run -v $(pwd)/results:/results \
    moinvinchhi/cloudai-cpu-intensive:latest
```

### Build Single Image
```bash
# Build only CPU process 14
docker build -t moinvinchhi/cloudai-cpu-intensive:14 \
    -f cpu-intensive/Dockerfile \
    --build-arg PROCESS_NUM=14 \
    cpu-intensive/

# Build only Mixed process 10
docker build -t moinvinchhi/cloudai-mixed-intensive:10 \
    -f mixed-intensive/Dockerfile \
    --build-arg PROCESS_NUM=10 \
    mixed-intensive/
```

### Push Single Image
```bash
# Push only CPU process 14
docker push moinvinchhi/cloudai-cpu-intensive:14

# Push only Mixed process 10
docker push moinvinchhi/cloudai-mixed-intensive:10
```

## Build Arguments

All Dockerfiles accept the `PROCESS_NUM` build argument:

```dockerfile
ARG PROCESS_NUM=1
CMD sh -c "python3 cpu_process_${PROCESS_NUM}.py"
```

This allows building specific process images without modifying the Dockerfile.

## Customization

### Change Docker Username
Set the `DOCKER_USERNAME` environment variable:

```bash
export DOCKER_USERNAME="yourusername"
./build-local.sh
./push-images.sh
```

### Build Subset of Images
Edit the loop ranges in `build-local.sh`:

```bash
# Only build CPU processes 1-10
for i in {1..10}; do
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive:$i" \
        -f cpu-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        cpu-intensive/
done
```

## Verification

### Check Built Images
```bash
# List all CloudAI images
docker images | grep cloudai

# Count total images
docker images | grep cloudai | wc -l
```

### Test Image
```bash
# Create results directory
mkdir -p results

# Run test
docker run -v $(pwd)/results:/results \
    moinvinchhi/cloudai-cpu-intensive:14

# Check results
ls -lh results/
cat results/cpu_stats.json
```

## Troubleshooting

### Build Fails
- Ensure all Python scripts exist in the directory
- Check that scripts have correct permissions (`chmod +x *.py`)
- Verify Dockerfile syntax

### Push Fails
- Ensure you're logged in: `docker login`
- Check your Docker Hub credentials
- Verify you have permission to push to the repository

### Image Too Large
- Consider using multi-stage builds
- Remove unnecessary dependencies
- Use `.dockerignore` to exclude files

## Performance Tips

### Parallel Builds
Build multiple images in parallel using GNU parallel or xargs:

```bash
# Build CPU images in parallel (4 at a time)
seq 1 32 | xargs -P 4 -I {} docker build \
    -t moinvinchhi/cloudai-cpu-intensive:{} \
    -f cpu-intensive/Dockerfile \
    --build-arg PROCESS_NUM={} \
    cpu-intensive/
```

### Docker BuildKit
Enable BuildKit for faster builds:

```bash
export DOCKER_BUILDKIT=1
./build-local.sh
```

### Layer Caching
The Dockerfiles are optimized for layer caching:
1. Base image and dependencies (cached between builds)
2. Copy scripts (only rebuilt when scripts change)
3. CMD with build arg (quick to build)

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Build and Push Docker Images
  run: |
    docker login -u ${{ secrets.DOCKER_USERNAME }} -p ${{ secrets.DOCKER_PASSWORD }}
    cd processes
    ./build-local.sh
    ./push-images.sh
```

### Jenkins Pipeline Example
```groovy
stage('Build Docker Images') {
    steps {
        sh 'cd processes && ./build-local.sh'
    }
}
stage('Push to Docker Hub') {
    steps {
        sh 'cd processes && ./push-images.sh'
    }
}
```

## Maintenance

### Update All Images
When you modify process scripts:

```bash
# Rebuild and push all
./build-local.sh
./push-images.sh
```

### Update Specific Image
```bash
# Update only CPU process 14
docker build -t moinvinchhi/cloudai-cpu-intensive:14 \
    -f cpu-intensive/Dockerfile \
    --build-arg PROCESS_NUM=14 \
    cpu-intensive/
    
docker push moinvinchhi/cloudai-cpu-intensive:14
```

### Clean Up Old Images
```bash
# Remove all CloudAI images
docker rmi $(docker images | grep cloudai | awk '{print $3}')

# Prune dangling images
docker image prune -f
```

## Docker Hub Links

After pushing, images will be available at:
- https://hub.docker.com/r/moinvinchhi/cloudai-io-intensive
- https://hub.docker.com/r/moinvinchhi/cloudai-cpu-intensive
- https://hub.docker.com/r/moinvinchhi/cloudai-mixed-intensive
- https://hub.docker.com/r/moinvinchhi/cloudai-gpu-intensive
