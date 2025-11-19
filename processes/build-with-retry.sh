#!/bin/bash

# Build Script with Retry Logic and Smaller Batches
# This helps handle network timeouts

set -e

DOCKER_USERNAME="${DOCKER_USERNAME:-moinvinchhi}"
IMAGE_PREFIX="cloudai"
MAX_RETRIES=3

build_image() {
    local category=$1
    local num=$2
    local dockerfile=$3
    local context=$4
    
    local image_name="${DOCKER_USERNAME}/${IMAGE_PREFIX}-${category}:${num}"
    local retry_count=0
    
    while [ $retry_count -lt $MAX_RETRIES ]; do
        echo "  Building ${category} process ${num} (attempt $((retry_count + 1))/${MAX_RETRIES})..."
        
        if docker build -t "${image_name}" \
            -f "${dockerfile}" \
            --build-arg PROCESS_NUM=${num} \
            "${context}"; then
            echo "  ✓ Successfully built ${image_name}"
            return 0
        else
            retry_count=$((retry_count + 1))
            if [ $retry_count -lt $MAX_RETRIES ]; then
                echo "  ✗ Build failed, retrying in 5 seconds..."
                sleep 5
            fi
        fi
    done
    
    echo "  ✗ Failed to build ${image_name} after ${MAX_RETRIES} attempts"
    return 1
}

echo "Building CloudAI process images with retry logic..."
echo "Docker username: $DOCKER_USERNAME"
echo ""

# Build IO-intensive images
echo "Building IO-intensive images (27 total)..."
for i in {1..27}; do
    build_image "io-intensive" $i "io-intensive/Dockerfile" "io-intensive/"
done
build_image "io-intensive" "latest" "io-intensive/Dockerfile" "io-intensive/"

echo ""

# Build CPU-intensive images
echo "Building CPU-intensive images (32 total)..."
for i in {1..32}; do
    build_image "cpu-intensive" $i "cpu-intensive/Dockerfile" "cpu-intensive/"
done
build_image "cpu-intensive" "latest" "cpu-intensive/Dockerfile" "cpu-intensive/"

echo ""

# Build Mixed-intensive images
echo "Building Mixed-intensive images (20 total)..."
for i in {1..20}; do
    build_image "mixed-intensive" $i "mixed-intensive/Dockerfile" "mixed-intensive/"
done
build_image "mixed-intensive" "latest" "mixed-intensive/Dockerfile" "mixed-intensive/"

echo ""

# Build GPU-intensive images (if directory exists)
if [ -d "gpu-intensive" ]; then
    echo "Building GPU-intensive images (6 total)..."
    for i in {1..6}; do
        build_image "gpu-intensive" $i "gpu-intensive/Dockerfile" "gpu-intensive/"
    done
    build_image "gpu-intensive" "latest" "gpu-intensive/Dockerfile" "gpu-intensive/"
fi

echo ""
echo "Build process completed!"
echo ""
echo "List all images:"
docker images | grep "${DOCKER_USERNAME}/${IMAGE_PREFIX}"
