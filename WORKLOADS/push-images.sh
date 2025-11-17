#!/bin/bash

# Push All Workload Images to Docker Hub
# Prerequisite: You must be logged in to Docker Hub (docker login)
# Prerequisite: Images must be built first (run build-local.sh)

set -e

DOCKER_USERNAME="${DOCKER_USERNAME:-moinvinchhi}"
IMAGE_PREFIX="cloudai"

echo "Pushing all CloudAI workload images to Docker Hub..."
echo "Docker username: $DOCKER_USERNAME"
echo ""

# Check if logged in
if ! docker info | grep -q "Username:"; then
    echo "Error: You are not logged in to Docker Hub"
    echo "Please run: docker login"
    exit 1
fi

# Push CPU-Light images (1-5)
echo "Pushing CPU-Light images..."
for i in {1..5}; do
    echo "  Pushing cpu-light process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-light:$i"
done
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-light:latest" || true

# Push CPU-Heavy images (1-5)
echo "Pushing CPU-Heavy images..."
for i in {1..5}; do
    echo "  Pushing cpu-heavy process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-heavy:$i"
done
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-heavy:latest" || true

# Push Memory-Heavy images (1-5)
echo "Pushing Memory-Heavy images..."
for i in {1..5}; do
    echo "  Pushing memory-heavy process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-memory-heavy:$i"
done
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-memory-heavy:latest" || true

# Push GPU-Inference images (1-5)
echo "Pushing GPU-Inference images..."
for i in {1..5}; do
    echo "  Pushing gpu-inference process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-inference:$i"
done
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-inference:latest" || true

# Push GPU-Training images (1-5)
echo "Pushing GPU-Training images..."
for i in {1..5}; do
    echo "  Pushing gpu-training process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-training:$i"
done
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-training:latest" || true

# Push Mixed images (1-5)
echo "Pushing Mixed images..."
for i in {1..5}; do
    echo "  Pushing mixed process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed:$i"
done
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed:latest" || true

echo ""
echo "All images pushed successfully!"
echo ""
echo "Docker Hub URLs:"
echo "  CPU-Light:     https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-light"
echo "  CPU-Heavy:     https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-heavy"
echo "  Memory-Heavy:  https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-memory-heavy"
echo "  GPU-Inference: https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-inference"
echo "  GPU-Training:  https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-training"
echo "  Mixed:         https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed"
