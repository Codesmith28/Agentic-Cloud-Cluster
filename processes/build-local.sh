#!/bin/bash

# Quick Build Script - Build all images locally without pushing
# Useful for testing before pushing to Docker Hub

set -e

DOCKER_USERNAME="${DOCKER_USERNAME:-moinvinchhi}"
IMAGE_PREFIX="cloudai"

echo "Building all CloudAI process images..."
echo "Docker username: $DOCKER_USERNAME"
echo ""

# Build IO-intensive images
echo "Building IO-intensive images..."
for i in {1..7}; do
    echo "  Building io-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-io-intensive:$i" \
        -f io-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        io-intensive/
done

# Build CPU-intensive images
echo "Building CPU-intensive images..."
for i in {1..12}; do
    echo "  Building cpu-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive:$i" \
        -f cpu-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        cpu-intensive/
done

# Build GPU-intensive images
echo "Building GPU-intensive images..."
for i in {1..6}; do
    echo "  Building gpu-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-intensive:$i" \
        -f gpu-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        gpu-intensive/
done

echo ""
echo "All images built successfully!"
echo ""
echo "List all images:"
docker images | grep "${DOCKER_USERNAME}/${IMAGE_PREFIX}"
