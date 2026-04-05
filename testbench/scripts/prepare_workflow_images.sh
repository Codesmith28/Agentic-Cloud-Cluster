#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/testbench/docker-compose.yml"
IMAGE_TAG="${CLOUDAI_WORKFLOW_IMAGE:-cloudai-benchmark:1}"

services=(
  "worker-small-dind"
  "worker-medium-dind"
  "worker-large-dind"
)

for service in "${services[@]}"; do
  echo "Building ${IMAGE_TAG} in ${service}..."
  docker compose -f "${COMPOSE_FILE}" exec -T "${service}" \
    docker build -t "${IMAGE_TAG}" /opt/cloudai/workflow-image >/dev/null
done

echo "Deterministic workflow image ready in all worker daemons: ${IMAGE_TAG}"
