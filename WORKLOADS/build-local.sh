#!/bin/bash

# Quick Build Script - Build all workload images locally without pushing
# Useful for testing before pushing to Docker Hub

set -e

DOCKER_USERNAME="${DOCKER_USERNAME:-moinvinchhi}"
IMAGE_PREFIX="cloudai"

echo "Building all CloudAI workload images..."
echo "Docker username: $DOCKER_USERNAME"
echo ""

# Build CPU-Light images (01-05)
echo "Building CPU-Light images..."
for i in {1..5}; do
    echo "  Building cpu-light process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-light:$i" \
        -f Dockerfile \
        --build-arg WORKLOAD_FILE="$(printf "%02d" $i)_cpu-light_*.py" \
        .
done

# Build CPU-Heavy images (06-10)
echo "Building CPU-Heavy images..."
for i in {1..5}; do
    num=$((i + 5))
    echo "  Building cpu-heavy process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-heavy:$i" \
        -f Dockerfile \
        --build-arg WORKLOAD_FILE="$(printf "%02d" $num)_cpu-heavy_*.py" \
        .
done

# Build Memory-Heavy images (11-15)
echo "Building Memory-Heavy images..."
for i in {1..5}; do
    num=$((i + 10))
    echo "  Building memory-heavy process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-memory-heavy:$i" \
        -f Dockerfile \
        --build-arg WORKLOAD_FILE="$(printf "%02d" $num)_memory-heavy_*.py" \
        .
done

# Build GPU-Inference images (16-20)
echo "Building GPU-Inference images..."
for i in {1..5}; do
    num=$((i + 15))
    echo "  Building gpu-inference process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-inference:$i" \
        -f Dockerfile \
        --build-arg WORKLOAD_FILE="$(printf "%02d" $num)_gpu-inference_*.py" \
        .
done

# Build GPU-Training images (21-25)
echo "Building GPU-Training images..."
for i in {1..5}; do
    num=$((i + 20))
    echo "  Building gpu-training process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-training:$i" \
        -f Dockerfile \
        --build-arg WORKLOAD_FILE="$(printf "%02d" $num)_gpu-training_*.py" \
        .
done

# Build Mixed images (26-30)
echo "Building Mixed images..."
for i in {1..5}; do
    num=$((i + 25))
    echo "  Building mixed process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed:$i" \
        -f Dockerfile \
        --build-arg WORKLOAD_FILE="$(printf "%02d" $num)_mixed_*.py" \
        .
done

echo ""
echo "All images built successfully!"
echo ""
echo "List all images:"
docker images | grep "${DOCKER_USERNAME}/${IMAGE_PREFIX}"
