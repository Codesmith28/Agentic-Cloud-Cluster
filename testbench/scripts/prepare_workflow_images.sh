#!/usr/bin/env bash


set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/testbench/docker-compose.yml"
IMAGE_TAG="${CLOUDAI_WORKFLOW_IMAGE:-cloudai-benchmark:1}"

mapfile -t DIND_SERVICES < <(docker compose -f "${COMPOSE_FILE}" ps --services | grep -- '-dind$' || true)
if [[ ${#DIND_SERVICES[@]} -eq 0 ]]; then
  echo "No DinD services are running; start the testbench stack first" >&2
  exit 1
fi

for service in "${DIND_SERVICES[@]}"; do
  echo "Building ${IMAGE_TAG} inside ${service}..."
  docker compose -f "${COMPOSE_FILE}" exec -T "${service}" \
    docker build -t "${IMAGE_TAG}" /opt/cloudai/workflow-image >/dev/null
  echo "Prepared ${IMAGE_TAG} in ${service}"
done
