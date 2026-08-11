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

from __future__ import annotations

import argparse
import logging
import os
import signal
import sys
from concurrent import futures
from pathlib import Path

import grpc

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from proto.py import ppo_scheduler_pb2, ppo_scheduler_pb2_grpc  # pylint: disable=wrong-import-position

from .service import PPOServiceCore  # pylint: disable=wrong-import-position


LOGGER = logging.getLogger(__name__)


class PPOSchedulerServicer(ppo_scheduler_pb2_grpc.PPOSchedulerServicer):
    def __init__(self, core: PPOServiceCore):
        self.core = core

    def Ping(self, request, context):  # noqa: N802 (gRPC naming)
        healthy, message, fingerprint_hash, model_version = self.core.ping()
        return ppo_scheduler_pb2.PingResponse(
            healthy=healthy,
            message=message,
            fingerprint_hash=fingerprint_hash,
            model_version=model_version,
        )

    def LoadModelForFingerprint(self, request, context):  # noqa: N802
        fingerprint_hash = _sanitize_string(request.fingerprint_hash, max_length=256)
        fingerprint_payload = _sanitize_string(request.fingerprint_payload, max_length=4096)
        if not fingerprint_hash:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("fingerprint_hash is required")
            return ppo_scheduler_pb2.LoadModelForFingerprintResponse(
                loaded=False, message="fingerprint_hash is required"
            )
        loaded, cold_start, version, message = self.core.ensure_fingerprint_loaded(
            fingerprint_hash=fingerprint_hash,
            fingerprint_payload=fingerprint_payload,
            create_if_missing=bool(request.create_if_missing),
        )
        return ppo_scheduler_pb2.LoadModelForFingerprintResponse(
            loaded=loaded,
            cold_start=cold_start,
            model_version=version,
            message=message,
        )

    def SelectWorker(self, request, context):  # noqa: N802
        if len(request.workers) > _MAX_WORKERS_PER_REQUEST:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details(f"too many workers (max {_MAX_WORKERS_PER_REQUEST})")
            return ppo_scheduler_pb2.SelectWorkerResponse(
                used_fallback_policy=True, reason="too many workers"
            )
        cluster_fp = _sanitize_string(request.cluster_fingerprint_hash, max_length=256)
        if cluster_fp:
            self.core.ensure_fingerprint_loaded(
                fingerprint_hash=cluster_fp,
                fingerprint_payload=_sanitize_string(
                    request.cluster_fingerprint_payload, max_length=4096
                ),
                create_if_missing=True,
            )
        worker_id, used_fallback, reason, model_version = self.core.select_worker(
            task=request.task,
            workers=request.workers,
            fallback_scheduler=request.fallback_scheduler,
        )
        return ppo_scheduler_pb2.SelectWorkerResponse(
            worker_id=worker_id,
            used_fallback_policy=used_fallback,
            reason=reason,
            model_version=model_version,
        )

    def ReportOutcome(self, request, context):  # noqa: N802
        task_id = _sanitize_string(request.task_id, max_length=512)
        if not task_id:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("task_id is required")
            return ppo_scheduler_pb2.ReportOutcomeResponse(
                accepted=False, message="task_id is required"
            )
        reward = _clamp_float(request.reward, -10.0, 10.0)
        runtime_seconds = max(float(request.runtime_seconds), 0.0)
        accepted, message = self.core.report_outcome(
            task_id=task_id,
            worker_id=_sanitize_string(request.worker_id, max_length=256),
            status=_sanitize_string(request.status, max_length=64),
            reward=reward,
            runtime_seconds=runtime_seconds,
            sla_success=request.sla_success,
            fingerprint_hash=_sanitize_string(request.fingerprint_hash, max_length=256),
        )
        return ppo_scheduler_pb2.ReportOutcomeResponse(
            accepted=accepted,
            message=message,
        )


_MAX_WORKERS_PER_REQUEST = 512


def _sanitize_string(value: str, max_length: int = 256) -> str:
    """Strip and truncate a string input to prevent oversized data."""
    return str(value or "").strip()[:max_length]


def _clamp_float(value: float, lo: float, hi: float) -> float:
    """Clamp a float to a safe range, treating NaN/Inf as 0."""
    v = float(value)
    if v != v or v == float("inf") or v == float("-inf"):
        return 0.0
    return max(lo, min(v, hi))


def _configure_logging(level: str) -> None:
    _ALLOWED_LOG_LEVELS = {"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}
    normalized = str(level).upper().strip()
    resolved = normalized if normalized in _ALLOWED_LOG_LEVELS else "INFO"
    logging.basicConfig(
        level=getattr(logging, resolved),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )


def _parse_bool(value: str) -> bool:
    if isinstance(value, bool):
        return value
    normalized = str(value).strip().lower()
    if normalized in {"1", "true", "t", "yes", "y", "on"}:
        return True
    if normalized in {"0", "false", "f", "no", "n", "off"}:
        return False
    raise argparse.ArgumentTypeError(f"Invalid boolean value: {value}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="PPO scheduler gRPC service")
    parser.add_argument("--grpc-addr", default=os.getenv("PPO_GRPC_ADDR", "127.0.0.1:50050"))
    parser.add_argument("--mongo-uri", default=os.getenv("MONGODB_URI", ""))
    parser.add_argument("--mongo-db", default=os.getenv("MONGODB_DATABASE", "cluster_db"))
    parser.add_argument("--model-path", default=os.getenv("PPO_MODEL_PATH", "latest"))
    parser.add_argument("--learning-rate", type=float, default=float(os.getenv("PPO_LEARNING_RATE", "0.0003")))
    parser.add_argument("--update-batch-size", type=int, default=int(os.getenv("PPO_UPDATE_BATCH_SIZE", "32")))
    parser.add_argument(
        "--deterministic-bias",
        type=float,
        default=float(os.getenv("PPO_DETERMINISTIC_BIAS", "0.25")),
    )
    parser.add_argument(
        "--online-updates",
        type=_parse_bool,
        default=_parse_bool(os.getenv("PPO_ONLINE_UPDATES_ENABLED", "true")),
    )
    parser.add_argument(
        "--prefer-gpu",
        type=_parse_bool,
        default=_parse_bool(os.getenv("PPO_PREFER_GPU", "true")),
    )
    parser.add_argument("--log-level", default=os.getenv("PPO_LOG_LEVEL", "INFO"))
    return parser.parse_args()


def serve() -> None:
    args = parse_args()
    _configure_logging(args.log_level)

    core = PPOServiceCore(
        mongo_uri=args.mongo_uri,
        mongo_db=args.mongo_db,
        model_path=args.model_path,
        learning_rate=args.learning_rate,
        update_batch_size=args.update_batch_size,
        deterministic_bias=args.deterministic_bias,
        online_updates_enabled=args.online_updates,
        prefer_gpu=args.prefer_gpu,
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=16))
    ppo_scheduler_pb2_grpc.add_PPOSchedulerServicer_to_server(PPOSchedulerServicer(core), server)
    server.add_insecure_port(args.grpc_addr)
    server.start()
    LOGGER.info("PPO scheduler service started on %s", args.grpc_addr)

    stop_event = {"stopped": False}

    def _stop(signum, frame):  # noqa: ARG001
        if stop_event["stopped"]:
            return
        stop_event["stopped"] = True
        LOGGER.info("Stopping PPO scheduler service")
        core.close()
        server.stop(grace=5)

    signal.signal(signal.SIGINT, _stop)
    signal.signal(signal.SIGTERM, _stop)

    server.wait_for_termination()


if __name__ == "__main__":
    serve()
