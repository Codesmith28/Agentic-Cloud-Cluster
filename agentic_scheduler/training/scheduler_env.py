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
    used_cpu: float = 0.0
    used_memory: float = 0.0
    used_storage: float = 0.0

    @property
    def available_cpu(self) -> float:
        return max(self.total_cpu - self.used_cpu, 0.0)

    @property
    def available_memory(self) -> float:
        return max(self.total_memory - self.used_memory, 0.0)

    @property
    def available_storage(self) -> float:
        return max(self.total_storage - self.used_storage, 0.0)


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

        if feasible:
            # Snapshot cluster load std BEFORE placement for delta-balance reward
            loads_before = [self._normalized_load(w) for w in self.workers]
            loads_before_std = float(np.std(loads_before))

            self._apply_task(selected, self.current_task)

            # Post-placement metrics
            post_load = self._normalized_load(selected)
            all_loads_after = [self._normalized_load(w) for w in self.workers]
            cluster_load = float(np.mean(all_loads_after))
            loads_after_std = float(np.std(all_loads_after))

            headroom = max(1.0 - post_load, 0.0)
            # Penalize if selected worker is more loaded than cluster average
            imbalance = max(post_load - cluster_load, 0.0)
            # Penalize actions that worsen overall load spread across the cluster
            delta_imbalance = loads_after_std - loads_before_std

            reward = (
                0.8
                + 0.30 * headroom
                - 0.40 * imbalance
                - 0.50 * delta_imbalance
            )
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
        # Realistic worker sizes matching the actual 3-tier test cluster
        worker_tier = int(self.rng.integers(0, 3))  # 0=small, 1=medium, 2=large
        if worker_tier == 0:
            return WorkerState(total_cpu=1.0, total_memory=1.5, total_storage=10.0)
        elif worker_tier == 1:
            return WorkerState(total_cpu=2.0, total_memory=3.0, total_storage=20.0)
        else:
            return WorkerState(total_cpu=3.0, total_memory=5.0, total_storage=30.0)

    def _sample_task(self) -> Dict:
        task_type = TASK_TYPES[int(self.rng.integers(0, len(TASK_TYPES)))]
        if task_type == "cpu-light":
            req_cpu = float(self.rng.uniform(0.3, 0.7))
            req_memory = float(self.rng.uniform(0.2, 0.5))
        elif task_type == "cpu-heavy":
            req_cpu = float(self.rng.uniform(1.5, 2.8))
            req_memory = float(self.rng.uniform(0.5, 1.0))
        elif task_type == "memory-heavy":
            req_cpu = float(self.rng.uniform(0.5, 1.0))
            req_memory = float(self.rng.uniform(1.5, 2.5))
        else:  # mixed
            req_cpu = float(self.rng.uniform(1.0, 1.8))
            req_memory = float(self.rng.uniform(0.8, 1.5))
        req_storage = float(self.rng.uniform(1.0, 5.0))
        return {
            "req_cpu": req_cpu,
            "req_memory": req_memory,
            "req_storage": req_storage,
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
            storage_usage = self._safe_ratio(worker.used_storage, worker.total_storage)

            rows.append(
                np.asarray(
                    [
                        self._safe_ratio(worker.available_cpu, worker.total_cpu),
                        self._safe_ratio(worker.available_memory, worker.total_memory),
                        self._safe_ratio(worker.available_storage, worker.total_storage),
                        worker.total_cpu,
                        worker.total_memory,
                        worker.total_storage,
                        cpu_usage,
                        mem_usage,
                        storage_usage,
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
        storage = SchedulingEnv._safe_ratio(worker.used_storage, worker.total_storage)
        return min((cpu + memory + storage) / 3.0, 1.5)

    @staticmethod
    def _is_feasible(task: Dict, worker: WorkerState) -> bool:
        return (
            worker.available_cpu >= task["req_cpu"]
            and worker.available_memory >= task["req_memory"]
            and worker.available_storage >= task["req_storage"]
        )

    @staticmethod
    def _apply_task(worker: WorkerState, task: Dict) -> None:
        worker.used_cpu += task["req_cpu"]
        worker.used_memory += task["req_memory"]
        worker.used_storage += task["req_storage"]

    def _decay_loads(self) -> None:
        for worker in self.workers:
            worker.used_cpu *= 0.85
            worker.used_memory *= 0.85
            worker.used_storage *= 0.90
