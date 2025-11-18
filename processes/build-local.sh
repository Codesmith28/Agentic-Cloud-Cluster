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
for i in {1..27}; do
    echo "  Building io-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-io-intensive:$i" \
        -f io-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        io-intensive/
done
echo "  Building io-intensive:latest..."
docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-io-intensive:latest" \
    -f io-intensive/Dockerfile \
    --build-arg PROCESS_NUM=1 \
    io-intensive/

# Build CPU-intensive images
echo "Building CPU-intensive images..."
for i in {1..32}; do
    echo "  Building cpu-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive:$i" \
        -f cpu-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        cpu-intensive/
done
echo "  Building cpu-intensive:latest..."
docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive:latest" \
    -f cpu-intensive/Dockerfile \
    --build-arg PROCESS_NUM=1 \
    cpu-intensive/

# Build Mixed-intensive images
echo "Building Mixed-intensive images..."
for i in {1..20}; do
    echo "  Building mixed-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed-intensive:$i" \
        -f mixed-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        mixed-intensive/
done
echo "  Building mixed-intensive:latest..."
docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed-intensive:latest" \
    -f mixed-intensive/Dockerfile \
    --build-arg PROCESS_NUM=1 \
    mixed-intensive/

# Build GPU-intensive images
echo "Building GPU-intensive images..."
for i in {1..6}; do
    echo "  Building gpu-intensive process $i..."
    docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-intensive:$i" \
        -f gpu-intensive/Dockerfile \
        --build-arg PROCESS_NUM=$i \
        gpu-intensive/
done
echo "  Building gpu-intensive:latest..."
docker build -t "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-intensive:latest" \
    -f gpu-intensive/Dockerfile \
    --build-arg PROCESS_NUM=1 \
    gpu-intensive/

echo ""
echo "All images built successfully!"
echo ""
echo "List all images:"
docker images | grep "${DOCKER_USERNAME}/${IMAGE_PREFIX}"
