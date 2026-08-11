#!/usr/bin/env bash

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
