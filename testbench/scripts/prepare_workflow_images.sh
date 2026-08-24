#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${REPO_ROOT}/testbench/docker-compose.yml}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-${REPO_ROOT}/.env}"
IMAGE_CONTEXT="${IMAGE_CONTEXT:-${REPO_ROOT}/testbench/workflow-image}"
IMAGE_TAG="${AGENTIC_WORKFLOW_IMAGE_TAG:-${CLOUDAI_WORKFLOW_IMAGE_TAG:-agentic/workflow-deterministic:v1}}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-30}"
SLEEP_SECONDS="${SLEEP_SECONDS:-2}"
SKIP_BUILD="${SKIP_BUILD:-false}"

IFS=',' read -r -a DIND_SERVICES <<< "${DIND_SERVICES:-worker-small-dind,worker-medium-dind,worker-large-dind}"

docker_compose() {
  local -a args=(-f "${COMPOSE_FILE}")
  if [[ -f "${COMPOSE_ENV_FILE}" ]]; then
    args+=(--env-file "${COMPOSE_ENV_FILE}")
  fi
  docker compose "${args[@]}" "$@"
}

wait_for_dind() {
  local container_id="$1"
  local service_name="$2"
  local attempt

  for attempt in $(seq 1 "${MAX_ATTEMPTS}"); do
    if docker exec "${container_id}" docker info >/dev/null 2>&1; then
      return 0
    fi
    sleep "${SLEEP_SECONDS}"
  done

  echo "DinD daemon for ${service_name} did not become ready" >&2
  return 1
}

ensure_running_container() {
  local service_name="$1"
  local container_id

  container_id="$(docker_compose ps -q "${service_name}")"
  if [[ -z "${container_id}" ]]; then
    echo "Service ${service_name} is not running. Start the testbench stack first." >&2
    return 1
  fi

  wait_for_dind "${container_id}" "${service_name}"
  echo "${container_id}"
}

build_image() {
  if [[ "${SKIP_BUILD}" == "true" ]]; then
    echo "Skipping deterministic workflow image build (SKIP_BUILD=true)"
    return
  fi

  echo "Building deterministic workflow image ${IMAGE_TAG}..."
  docker build --pull -t "${IMAGE_TAG}" "${IMAGE_CONTEXT}" >/dev/null
}

load_image_into_dind() {
  local container_id="$1"
  local service_name="$2"

  echo "Loading ${IMAGE_TAG} into ${service_name} (${container_id:0:12})..."
  docker image save "${IMAGE_TAG}" | docker exec -i "${container_id}" docker image load >/dev/null
  docker exec "${container_id}" docker image inspect "${IMAGE_TAG}" >/dev/null
}

main() {
  build_image

  local service_name
  local container_id
  for service_name in "${DIND_SERVICES[@]}"; do
    container_id="$(ensure_running_container "${service_name}")"
    load_image_into_dind "${container_id}" "${service_name}"
  done

  echo "Deterministic workflow image prepared in all worker DinD daemons"
}

main "$@"
