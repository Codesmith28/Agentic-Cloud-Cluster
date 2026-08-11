#!/bin/bash

# Copyright 2025-2026 Sarthak Siddhpura
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
