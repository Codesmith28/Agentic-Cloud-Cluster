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
        loaded, cold_start, version, message = self.core.ensure_fingerprint_loaded(
            fingerprint_hash=request.fingerprint_hash,
            fingerprint_payload=request.fingerprint_payload,
            create_if_missing=bool(request.create_if_missing),
        )
        return ppo_scheduler_pb2.LoadModelForFingerprintResponse(
            loaded=loaded,
            cold_start=cold_start,
            model_version=version,
            message=message,
        )

    def SelectWorker(self, request, context):  # noqa: N802
        if request.cluster_fingerprint_hash:
            self.core.ensure_fingerprint_loaded(
                fingerprint_hash=request.cluster_fingerprint_hash,
                fingerprint_payload=request.cluster_fingerprint_payload,
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
        accepted, message = self.core.report_outcome(
            task_id=request.task_id,
            worker_id=request.worker_id,
            status=request.status,
            reward=request.reward,
            runtime_seconds=request.runtime_seconds,
            sla_success=request.sla_success,
            fingerprint_hash=request.fingerprint_hash,
        )
        return ppo_scheduler_pb2.ReportOutcomeResponse(
            accepted=accepted,
            message=message,
        )


def _configure_logging(level: str) -> None:
    logging.basicConfig(
        level=getattr(logging, level.upper(), logging.INFO),
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
    parser.add_argument("--grpc-addr", default=os.getenv("PPO_GRPC_ADDR", "127.0.0.1:50061"))
    parser.add_argument("--mongo-uri", default=os.getenv("MONGODB_URI", ""))
    parser.add_argument("--mongo-db", default=os.getenv("MONGODB_DATABASE", "cluster_db"))
    parser.add_argument("--model-path", default=os.getenv("PPO_MODEL_PATH", "agentic_scheduler/models/ppo_latest.pt"))
    parser.add_argument("--learning-rate", type=float, default=float(os.getenv("PPO_LEARNING_RATE", "0.0003")))
    parser.add_argument("--update-batch-size", type=int, default=int(os.getenv("PPO_UPDATE_BATCH_SIZE", "32")))
    parser.add_argument(
        "--online-updates",
        type=_parse_bool,
        default=_parse_bool(os.getenv("PPO_ONLINE_UPDATES_ENABLED", "true")),
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
        online_updates_enabled=args.online_updates,
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
