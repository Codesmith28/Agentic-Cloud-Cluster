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

"""Gymnasium environment that replays real cluster traces for PPO training.

Unlike the synthetic ``SchedulingEnv`` which generates random tasks, this
environment reads a ``TraceCluster`` produced by the trace loaders and
replays the recorded task arrivals in chronological order, giving the PPO
agent a realistic distribution of resource requests and inter-arrival times.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from ..features import TASK_FEATURE_DIM, TASK_TYPE_TO_ID, WORKER_FEATURE_DIM
from .trace_loader import TraceCluster, TraceTask


@dataclass
class _WorkerState:
    worker_id: str
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


class TraceReplayEnv(gym.Env):
    """Replay a cluster trace as a Gymnasium RL environment.

    Each ``step()`` presents the next task from the trace.  The agent selects a
    worker (action) and receives a reward based on load balancing, feasibility,
    and SLA performance.  Active task loads decay over time proportional to the
    inter-arrival gap between consecutive tasks.

    Parameters
    ----------
    trace:
        A ``TraceCluster`` instance produced by the trace loaders.
    num_workers:
        How many workers from the trace to use.  If the trace has more
        machines, only the first ``num_workers`` are kept.  If fewer, extra
        default workers are added.
    loop:
        When ``True`` (default), the task sequence restarts from the beginning
        when exhausted, allowing longer training runs.  When ``False``, the
        episode terminates when the last trace task is scheduled.
    """

    metadata = {"render_modes": []}

    def __init__(
        self,
        trace: TraceCluster,
        num_workers: int = 4,
        loop: bool = True,
    ):
        super().__init__()
        self.trace = trace
        self.num_workers = num_workers
        self.loop = loop

        # Clamp/pad workers to num_workers
        raw_workers = list(trace.workers)
        while len(raw_workers) < num_workers:
            raw_workers.append({
                "worker_id": f"pad-{len(raw_workers)}",
                "total_cpu": 16,
                "total_memory": 48,
                "total_storage": 500,
            })
        self._worker_configs = raw_workers[:num_workers]

        self.tasks: List[TraceTask] = list(trace.tasks)
        if not self.tasks:
            raise ValueError("TraceCluster has no tasks")

        self._task_idx = 0
        self.workers: List[_WorkerState] = []
        self.current_task: Optional[TraceTask] = None
        self._prev_arrival = 0.0

        self.observation_space = spaces.Dict({
            "task": spaces.Box(low=0.0, high=256.0, shape=(TASK_FEATURE_DIM,), dtype=np.float32),
            "workers": spaces.Box(
                low=0.0, high=256.0,
                shape=(self.num_workers, WORKER_FEATURE_DIM),
                dtype=np.float32,
            ),
            "action_mask": spaces.Box(low=0.0, high=1.0, shape=(self.num_workers,), dtype=np.float32),
        })
        self.action_space = spaces.Discrete(self.num_workers)

    # ------------------------------------------------------------------
    # Gym API
    # ------------------------------------------------------------------

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        self._task_idx = 0
        self._prev_arrival = 0.0
        self.workers = [
            _WorkerState(
                worker_id=w["worker_id"],
                total_cpu=w["total_cpu"],
                total_memory=w["total_memory"],
                total_storage=w["total_storage"],
            )
            for w in self._worker_configs
        ]
        self.current_task = self.tasks[0]
        return self._observation(), {}

    def step(self, action: int):
        assert self.current_task is not None
        assert 0 <= action < self.num_workers

        selected = self.workers[action]
        feasible = self._is_feasible(self.current_task, selected)

        # Reward
        if feasible:
            self._apply_task(selected, self.current_task)
            load_penalty = self._normalised_load(selected)
            reward = 1.2 - load_penalty

            # Bonus for scheduling a task that matches worker strength
            if self.current_task.runtime_seconds > 0:
                runtime_penalty = min(self.current_task.runtime_seconds / 600.0, 0.3)
                reward -= runtime_penalty
        else:
            reward = -1.4

        # Advance to next task
        self._task_idx += 1
        terminated = False

        if self._task_idx >= len(self.tasks):
            if self.loop:
                self._task_idx = 0
                self._prev_arrival = 0.0
            else:
                terminated = True
                return self._observation(), float(reward), terminated, False, {"feasible": feasible}

        next_task = self.tasks[self._task_idx]

        # Time-based load decay between task arrivals
        dt = max(next_task.arrival_time - self._prev_arrival, 0.0)
        self._decay_loads(dt)
        self._prev_arrival = next_task.arrival_time

        self.current_task = next_task

        info = {"feasible": feasible, "task_idx": self._task_idx}
        return self._observation(), float(reward), terminated, False, info

    # ------------------------------------------------------------------
    # Observation helpers
    # ------------------------------------------------------------------

    def _observation(self):
        worker_features, mask = self._worker_features()
        return {
            "task": self._task_features(),
            "workers": worker_features,
            "action_mask": mask,
        }

    def _task_features(self) -> np.ndarray:
        assert self.current_task is not None
        tt = self.current_task.task_type
        task_type_scalar = TASK_TYPE_TO_ID.get(tt, TASK_TYPE_TO_ID["mixed"]) / max(len(TASK_TYPE_TO_ID) - 1, 1)
        return np.asarray([
            self.current_task.req_cpu,
            self.current_task.req_memory,
            self.current_task.req_storage,
            self.current_task.sla_multiplier,
            task_type_scalar,
        ], dtype=np.float32)

    def _worker_features(self) -> Tuple[np.ndarray, np.ndarray]:
        rows = []
        mask = []
        for w in self.workers:
            feasible = self._is_feasible(self.current_task, w)
            mask.append(1.0 if feasible else 0.0)
            rows.append(np.asarray([
                _safe_ratio(w.available_cpu, w.total_cpu),
                _safe_ratio(w.available_memory, w.total_memory),
                _safe_ratio(w.available_storage, w.total_storage),
                w.total_cpu,
                w.total_memory,
                w.total_storage,
                _safe_ratio(w.used_cpu, w.total_cpu),
                _safe_ratio(w.used_memory, w.total_memory),
                _safe_ratio(w.used_storage, w.total_storage),
            ], dtype=np.float32))
        return np.stack(rows, axis=0), np.asarray(mask, dtype=np.float32)

    # ------------------------------------------------------------------
    # Internals
    # ------------------------------------------------------------------

    @staticmethod
    def _is_feasible(task: TraceTask, worker: _WorkerState) -> bool:
        return (
            worker.available_cpu >= task.req_cpu
            and worker.available_memory >= task.req_memory
            and worker.available_storage >= task.req_storage
        )

    @staticmethod
    def _apply_task(worker: _WorkerState, task: TraceTask) -> None:
        worker.used_cpu += task.req_cpu
        worker.used_memory += task.req_memory
        worker.used_storage += task.req_storage

    @staticmethod
    def _normalised_load(worker: _WorkerState) -> float:
        cpu = _safe_ratio(worker.used_cpu, worker.total_cpu)
        mem = _safe_ratio(worker.used_memory, worker.total_memory)
        sto = _safe_ratio(worker.used_storage, worker.total_storage)
        return min((cpu + mem + sto) / 3.0, 1.5)

    def _decay_loads(self, dt: float) -> None:
        """Decay worker loads based on elapsed time.

        Uses exponential decay with a half-life of ~30 seconds so that
        long gaps between arrivals release more resources.
        """
        if dt <= 0:
            return
        # decay_factor ~ e^(-dt/tau), tau=30s so half-life ≈ 20.8s
        decay = np.exp(-dt / 30.0)
        for w in self.workers:
            w.used_cpu *= decay
            w.used_memory *= decay
            w.used_storage *= decay


def _safe_ratio(numer: float, denom: float) -> float:
    if denom <= 0:
        return 0.0
    return numer / denom
