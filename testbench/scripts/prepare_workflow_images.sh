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
