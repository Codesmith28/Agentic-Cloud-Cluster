#!/bin/bash

# Push All Images to Docker Hub
# Prerequisite: You must be logged in to Docker Hub (docker login)
# Prerequisite: Images must be built first (run build-local.sh or build-and-push.sh)

set -e

DOCKER_USERNAME="${DOCKER_USERNAME:-moinvinchhi}"
IMAGE_PREFIX="cloudai"

echo "Pushing all CloudAI process images to Docker Hub..."
echo "Docker username: $DOCKER_USERNAME"
echo ""

# Check if logged in
if ! docker info | grep -q "Username:"; then
    echo "Error: You are not logged in to Docker Hub"
    echo "Please run: docker login"
    exit 1
fi

# Push IO-intensive images
echo "Pushing IO-intensive images..."
for i in {1..27}; do
    echo "  Pushing io-intensive process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-io-intensive:$i"
done
echo "  Pushing io-intensive:latest..."
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-io-intensive:latest"

# Push CPU-intensive images
echo "Pushing CPU-intensive images..."
for i in {1..32}; do
    echo "  Pushing cpu-intensive process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive:$i"
done
echo "  Pushing cpu-intensive:latest..."
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive:latest"

# Push Mixed-intensive images
echo "Pushing Mixed-intensive images..."
for i in {1..20}; do
    echo "  Pushing mixed-intensive process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed-intensive:$i"
done
echo "  Pushing mixed-intensive:latest..."
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed-intensive:latest"

# Push GPU-intensive images
echo "Pushing GPU-intensive images..."
for i in {1..6}; do
    echo "  Pushing gpu-intensive process $i..."
    docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-intensive:$i"
done
echo "  Pushing gpu-intensive:latest..."
docker push "${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-intensive:latest"

echo ""
echo "All images pushed successfully!"
echo ""
echo "Docker Hub URLs:"
echo "  IO-Intensive:    https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-io-intensive"
echo "  CPU-Intensive:   https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-cpu-intensive"
echo "  Mixed-Intensive: https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-mixed-intensive"
echo "  GPU-Intensive:   https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_PREFIX}-gpu-intensive"
echo ""
echo "Total images pushed:"
echo "  IO-Intensive:    27 processes + latest"
echo "  CPU-Intensive:   32 processes + latest"
echo "  Mixed-Intensive: 20 processes + latest"
echo "  GPU-Intensive:   6 processes + latest"
echo "  Total:           85 processes + 4 latest tags = 89 images"
