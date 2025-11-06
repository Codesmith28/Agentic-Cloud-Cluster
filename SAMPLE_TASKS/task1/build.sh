#!/bin/bash

# Build and push script for CloudAI sample task

set -e

# Configuration
IMAGE_NAME="cloudai-sample-task"
IMAGE_TAG="latest"

echo "═══════════════════════════════════════════════════════"
echo "  CloudAI Sample Task - Build & Push"
echo "═══════════════════════════════════════════════════════"
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed or not in PATH"
    exit 1
fi

# Ask for Docker Hub username
echo "To use this task with CloudAI workers, you need to push it to Docker Hub."
echo ""
read -p "Enter your Docker Hub username: " DOCKERHUB_USERNAME

if [ -z "$DOCKERHUB_USERNAME" ]; then
    echo "❌ Docker Hub username cannot be empty"
    exit 1
fi

FULL_IMAGE_NAME="$DOCKERHUB_USERNAME/$IMAGE_NAME:$IMAGE_TAG"

echo ""
echo "Building Docker image: $FULL_IMAGE_NAME"
echo ""

# Build the image
docker build -t "$FULL_IMAGE_NAME" .

if [ $? -ne 0 ]; then
    echo ""
    echo "❌ Build failed!"
    exit 1
fi

echo ""
echo "✅ Build successful!"
echo ""

# Ask if user wants to push
read -p "Push to Docker Hub? (y/n): " PUSH_CHOICE

if [ "$PUSH_CHOICE" = "y" ] || [ "$PUSH_CHOICE" = "Y" ]; then
    echo ""
    echo "Logging in to Docker Hub..."
    docker login
    
    if [ $? -ne 0 ]; then
        echo "❌ Docker login failed!"
        exit 1
    fi
    
    echo ""
    echo "Pushing image to Docker Hub..."
    docker push "$FULL_IMAGE_NAME"
    
    if [ $? -eq 0 ]; then
        echo ""
        echo "═══════════════════════════════════════════════════════"
        echo "  ✅ Push Successful!"
        echo "═══════════════════════════════════════════════════════"
        echo ""
        echo "Image: $FULL_IMAGE_NAME"
        echo ""
        echo "✅ Your workers can now pull this image!"
        echo ""
        echo "To use in CloudAI master CLI:"
        echo "  task <worker_id> $FULL_IMAGE_NAME"
        echo ""
        echo "Examples:"
        echo "  task worker-1 $FULL_IMAGE_NAME"
        echo "  task worker-1 $FULL_IMAGE_NAME -cpu_cores 2.0 -mem 1.0"
        echo ""
    else
        echo ""
        echo "❌ Push failed!"
        exit 1
    fi
else
    echo ""
    echo "Image built locally as: $FULL_IMAGE_NAME"
    echo ""
    echo "⚠️  WARNING: This image is only available locally."
    echo "   Workers on different machines cannot pull it."
    echo ""
    echo "To test locally:"
    echo "  docker run --rm $FULL_IMAGE_NAME"
    echo ""
    echo "To push later:"
    echo "  docker push $FULL_IMAGE_NAME"
fi
