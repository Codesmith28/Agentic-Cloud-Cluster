#!/bin/bash

# Test the Docker image locally before using it in CloudAI

echo "═══════════════════════════════════════════════════════"
echo "  Testing CloudAI Sample Task Locally"
echo "═══════════════════════════════════════════════════════"
echo ""

# Find available local images
echo "Looking for local cloudai-sample-task images..."
IMAGES=$(docker images --format "{{.Repository}}:{{.Tag}}" | grep cloudai-sample-task || true)

if [ -z "$IMAGES" ]; then
    echo "❌ No cloudai-sample-task images found locally"
    echo ""
    echo "Please build the image first:"
    echo "  ./build.sh"
    exit 1
fi

echo "Found images:"
echo "$IMAGES" | nl
echo ""

# If multiple images, let user choose
IMAGE_COUNT=$(echo "$IMAGES" | wc -l)

if [ "$IMAGE_COUNT" -eq 1 ]; then
    IMAGE_NAME=$(echo "$IMAGES" | head -1)
else
    read -p "Select image number (1-$IMAGE_COUNT): " SELECTION
    IMAGE_NAME=$(echo "$IMAGES" | sed -n "${SELECTION}p")
fi

echo ""
echo "Testing image: $IMAGE_NAME"
echo ""
echo "─────────────────────────────────────────────────────"

# Run the container
docker run --rm "$IMAGE_NAME"

echo "─────────────────────────────────────────────────────"
echo ""
echo "✅ Local test complete!"
