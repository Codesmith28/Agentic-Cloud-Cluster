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

from dataclasses import dataclass
from typing import Dict, Tuple

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from ..features import TASK_FEATURE_DIM, TASK_TYPE_TO_ID, WORKER_FEATURE_DIM


TASK_TYPES = list(TASK_TYPE_TO_ID.keys())


@dataclass
class WorkerState:
    total_cpu: float
    total_memory: float
    total_storage: float
    total_gpu: float
    used_cpu: float = 0.0
    used_memory: float = 0.0
    used_storage: float = 0.0
    used_gpu: float = 0.0

    @property
    def available_cpu(self) -> float:
        return max(self.total_cpu - self.used_cpu, 0.0)

    @property
    def available_memory(self) -> float:
        return max(self.total_memory - self.used_memory, 0.0)

    @property
    def available_storage(self) -> float:
        return max(self.total_storage - self.used_storage, 0.0)

    @property
    def available_gpu(self) -> float:
        return max(self.total_gpu - self.used_gpu, 0.0)


class SchedulingEnv(gym.Env):
    """Synthetic scheduling environment for PPO training."""

    metadata = {"render_modes": []}

    def __init__(self, num_workers: int = 4, episode_length: int = 96, seed: int | None = None):
        super().__init__()
        self.num_workers = num_workers
        self.episode_length = episode_length
        self.current_step = 0
        self.rng = np.random.default_rng(seed)
        self.workers = []
        self.current_task = None

        self.observation_space = spaces.Dict(
            {
                "task": spaces.Box(low=0.0, high=256.0, shape=(TASK_FEATURE_DIM,), dtype=np.float32),
                "workers": spaces.Box(
                    low=0.0,
                    high=256.0,
                    shape=(self.num_workers, WORKER_FEATURE_DIM),
                    dtype=np.float32,
                ),
                "action_mask": spaces.Box(low=0.0, high=1.0, shape=(self.num_workers,), dtype=np.float32),
            }
        )
        self.action_space = spaces.Discrete(self.num_workers)

    def reset(self, seed=None, options=None):  # noqa: D401
        super().reset(seed=seed)
        if seed is not None:
            self.rng = np.random.default_rng(seed)
        self.current_step = 0
        self.workers = [self._sample_worker() for _ in range(self.num_workers)]
        self.current_task = self._sample_task()
        return self._observation(), {}

    def step(self, action: int):
        assert self.current_task is not None
        assert 0 <= action < self.num_workers

        selected = self.workers[action]
        feasible = self._is_feasible(self.current_task, selected)

        reward = -1.0
        if feasible:
            self._apply_task(selected, self.current_task)
            load_penalty = self._normalized_load(selected)
            reward = 1.2 - load_penalty
            if self.current_task["task_type"] in {"gpu-training", "gpu-inference"} and selected.total_gpu <= 0:
                reward -= 0.4
        else:
            reward = -1.4

        self._decay_loads()
        self.current_task = self._sample_task()
        self.current_step += 1
        terminated = self.current_step >= self.episode_length
        truncated = False

        info = {"feasible": feasible}
        return self._observation(), float(reward), terminated, truncated, info

    def _sample_worker(self) -> WorkerState:
        has_gpu = bool(self.rng.random() > 0.35)
        return WorkerState(
            total_cpu=float(self.rng.uniform(4.0, 32.0)),
            total_memory=float(self.rng.uniform(8.0, 128.0)),
            total_storage=float(self.rng.uniform(100.0, 1500.0)),
            total_gpu=float(self.rng.choice([0.0, 1.0, 2.0, 4.0]) if has_gpu else 0.0),
        )

    def _sample_task(self) -> Dict:
        task_type = TASK_TYPES[int(self.rng.integers(0, len(TASK_TYPES)))]
        req_cpu = float(self.rng.uniform(0.5, 12.0))
        req_memory = float(self.rng.uniform(0.5, 32.0))
        req_storage = float(self.rng.uniform(0.5, 80.0))
        req_gpu = 0.0
        if task_type in {"gpu-inference", "gpu-training"}:
            req_gpu = float(self.rng.choice([0.5, 1.0, 2.0, 4.0]))
            req_cpu = max(req_cpu, float(self.rng.uniform(2.0, 16.0)))
        if task_type == "memory-heavy":
            req_memory = max(req_memory, float(self.rng.uniform(16.0, 64.0)))
        if task_type == "cpu-heavy":
            req_cpu = max(req_cpu, float(self.rng.uniform(6.0, 20.0)))

        return {
            "req_cpu": req_cpu,
            "req_memory": req_memory,
            "req_storage": req_storage,
            "req_gpu": req_gpu,
            "sla_multiplier": float(self.rng.uniform(1.5, 2.5)),
            "task_type": task_type,
        }

    def _task_features(self) -> np.ndarray:
        assert self.current_task is not None
        task_type_scalar = TASK_TYPE_TO_ID[self.current_task["task_type"]] / max(len(TASK_TYPE_TO_ID) - 1, 1)
        return np.asarray(
            [
                self.current_task["req_cpu"],
                self.current_task["req_memory"],
                self.current_task["req_storage"],
                self.current_task["req_gpu"],
                self.current_task["sla_multiplier"],
                task_type_scalar,
            ],
            dtype=np.float32,
        )

    def _worker_features(self) -> Tuple[np.ndarray, np.ndarray]:
        rows = []
        mask = []
        for worker in self.workers:
            feasible = self._is_feasible(self.current_task, worker)
            mask.append(1.0 if feasible else 0.0)

            cpu_usage = self._safe_ratio(worker.used_cpu, worker.total_cpu)
            mem_usage = self._safe_ratio(worker.used_memory, worker.total_memory)
            gpu_usage = self._safe_ratio(worker.used_gpu, worker.total_gpu if worker.total_gpu > 0 else 1.0)

            rows.append(
                np.asarray(
                    [
                        self._safe_ratio(worker.available_cpu, worker.total_cpu),
                        self._safe_ratio(worker.available_memory, worker.total_memory),
                        self._safe_ratio(worker.available_storage, worker.total_storage),
                        self._safe_ratio(worker.available_gpu, worker.total_gpu if worker.total_gpu > 0 else 1.0),
                        worker.total_cpu,
                        worker.total_memory,
                        worker.total_storage,
                        worker.total_gpu,
                        cpu_usage,
                        mem_usage,
                        gpu_usage,
                        1.0,
                    ],
                    dtype=np.float32,
                )
            )

        return np.stack(rows, axis=0), np.asarray(mask, dtype=np.float32)

    def _observation(self):
        worker_features, mask = self._worker_features()
        return {
            "task": self._task_features(),
            "workers": worker_features,
            "action_mask": mask,
        }

    @staticmethod
    def _safe_ratio(numerator: float, denominator: float) -> float:
        if denominator <= 0:
            return 0.0
        return numerator / denominator

    @staticmethod
    def _normalized_load(worker: WorkerState) -> float:
        cpu = SchedulingEnv._safe_ratio(worker.used_cpu, worker.total_cpu)
        memory = SchedulingEnv._safe_ratio(worker.used_memory, worker.total_memory)
        gpu = SchedulingEnv._safe_ratio(worker.used_gpu, worker.total_gpu if worker.total_gpu > 0 else 1.0)
        return min((cpu + memory + gpu) / 3.0, 1.5)

    @staticmethod
    def _is_feasible(task: Dict, worker: WorkerState) -> bool:
        return (
            worker.available_cpu >= task["req_cpu"]
            and worker.available_memory >= task["req_memory"]
            and worker.available_storage >= task["req_storage"]
            and worker.available_gpu >= task["req_gpu"]
        )

    @staticmethod
    def _apply_task(worker: WorkerState, task: Dict) -> None:
        worker.used_cpu += task["req_cpu"]
        worker.used_memory += task["req_memory"]
        worker.used_storage += task["req_storage"]
        worker.used_gpu += task["req_gpu"]

    def _decay_loads(self) -> None:
        for worker in self.workers:
            worker.used_cpu *= 0.85
            worker.used_memory *= 0.85
            worker.used_storage *= 0.90
            worker.used_gpu *= 0.80

