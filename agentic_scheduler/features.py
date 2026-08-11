from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable, List

import numpy as np

TASK_TYPE_TO_ID = {
    "cpu-light": 0,
    "cpu-heavy": 1,
    "memory-heavy": 2,
    "mixed": 3,
}

TASK_FEATURE_DIM = 5
WORKER_FEATURE_DIM = 9


def _safe_div(numerator: float, denominator: float) -> float:
    if denominator <= 0:
        return 0.0
    return numerator / denominator


def task_type_to_scalar(task_type: str) -> float:
    return float(TASK_TYPE_TO_ID.get(task_type or "mixed", TASK_TYPE_TO_ID["mixed"])) / 3.0


def extract_task_features(task) -> np.ndarray:
    return np.asarray(
        [
            float(getattr(task, "req_cpu", 0.0)),
            float(getattr(task, "req_memory", 0.0)),
            float(getattr(task, "req_storage", 0.0)),
            float(getattr(task, "sla_multiplier", 2.0)),
            task_type_to_scalar(getattr(task, "task_type", "mixed")),
        ],
        dtype=np.float32,
    )


def extract_worker_features(worker) -> np.ndarray:
    total_cpu = float(getattr(worker, "total_cpu", 0.0))
    total_memory = float(getattr(worker, "total_memory", 0.0))
    total_storage = float(getattr(worker, "total_storage", 0.0))
    available_cpu = float(getattr(worker, "available_cpu", 0.0))
    available_memory = float(getattr(worker, "available_memory", 0.0))
    available_storage = float(getattr(worker, "available_storage", 0.0))

    return np.asarray(
        [
            _safe_div(available_cpu, max(total_cpu, 1e-6)),
            _safe_div(available_memory, max(total_memory, 1e-6)),
            _safe_div(available_storage, max(total_storage, 1e-6)),
            total_cpu,
            total_memory,
            total_storage,
            float(getattr(worker, "current_cpu_usage", 0.0)),
            float(getattr(worker, "current_memory_usage", 0.0)),
            _safe_div(float(getattr(worker, "allocated_storage", 0.0)), max(total_storage, 1e-6)),
            1.0 if bool(getattr(worker, "is_active", False)) else 0.0,
        ],
        dtype=np.float32,
    )


def is_worker_feasible(task, worker) -> bool:
    if not bool(getattr(worker, "is_active", False)):
        return False

    return (
        float(getattr(worker, "available_cpu", 0.0)) >= float(getattr(task, "req_cpu", 0.0))
        and float(getattr(worker, "available_memory", 0.0)) >= float(getattr(task, "req_memory", 0.0))
        and float(getattr(worker, "available_storage", 0.0)) >= float(getattr(task, "req_storage", 0.0))
    )


@dataclass
class EncodedRequest:
    task_features: np.ndarray
    worker_features: np.ndarray
    action_mask: np.ndarray


def encode_request(task, workers: Iterable) -> EncodedRequest:
    worker_list: List = list(workers)
    if not worker_list:
        return EncodedRequest(
            task_features=np.zeros((TASK_FEATURE_DIM,), dtype=np.float32),
            worker_features=np.zeros((0, WORKER_FEATURE_DIM), dtype=np.float32),
            action_mask=np.zeros((0,), dtype=np.bool_),
        )

    task_features = extract_task_features(task)
    worker_features = np.stack([extract_worker_features(worker) for worker in worker_list], axis=0)
    action_mask = np.asarray([is_worker_feasible(task, worker) for worker in worker_list], dtype=np.bool_)

    return EncodedRequest(
        task_features=task_features,
        worker_features=worker_features,
        action_mask=action_mask,
    )
