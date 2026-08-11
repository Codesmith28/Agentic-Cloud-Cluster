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

Resource tracking uses **lifecycle-based accounting**: when a task is placed
on a worker its resource demands are added, and when the simulated runtime
elapses the resources are released.  This replaces the earlier exponential-
decay approximation which could not model simultaneous arrivals (65 % of the
Alibaba trace) and released resources either too fast or too slow depending
on the chosen time constant.
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
class _ActiveTask:
    """A task currently occupying resources on a worker."""

    worker_idx: int
    req_cpu: float
    req_memory: float
    req_storage: float
    end_time: float  # arrival_time + runtime_seconds


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

    Each ``step()`` presents the next task from the trace.  The agent selects
    a worker (action) and receives a reward based on feasibility,
    queue/turnaround proxies, tail-risk pressure, and load balance.

    Resource occupation is tracked via explicit task lifecycles: resources are
    reserved on placement and released when the task's simulated runtime
    expires.

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
        # Use realistic small-cluster worker sizes for padding (matching 3-tier test cluster)
        _pad_tiers = [(1.0, 1.5, 10.0), (2.0, 3.0, 20.0), (3.0, 5.0, 30.0)]
        while len(raw_workers) < num_workers:
            idx = len(raw_workers) % len(_pad_tiers)
            pcpu, pmem, psto = _pad_tiers[idx]
            raw_workers.append({
                "worker_id": f"pad-{len(raw_workers)}",
                "total_cpu": pcpu,
                "total_memory": pmem,
                "total_storage": psto,
            })
        self._worker_configs = raw_workers[:num_workers]

        self.tasks: List[TraceTask] = list(trace.tasks)
        if not self.tasks:
            raise ValueError("TraceCluster has no tasks")

        # Pre-compute task features for all trace tasks
        self._task_features_cache = np.stack(
            [self._compute_task_features(t) for t in self.tasks], axis=0
        )

        self._task_idx = 0
        self.workers: List[_WorkerState] = []
        self.current_task: Optional[TraceTask] = None
        self._active_tasks: List[_ActiveTask] = []

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
        self._active_tasks.clear()
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

        current_time = self.current_task.arrival_time

        # Release resources from tasks that have completed by now
        self._complete_tasks(current_time)

        selected = self.workers[action]
        feasible = self._is_feasible(self.current_task, selected)
        reward_details = {
            "queue_pressure": 0.0,
            "turnaround_pressure": 0.0,
            "tail_pressure": 0.0,
            "imbalance_penalty": 0.0,
            "delta_imbalance": 0.0,
        }

        if feasible:
            # Snapshot cluster balance BEFORE placement
            loads_before_std = float(np.std(
                [self._normalised_load(w) for w in self.workers]
            ))

            self._apply_task(selected, self.current_task)

            # Track this task for lifecycle-based resource release
            runtime = max(self.current_task.runtime_seconds, 1.0)
            self._active_tasks.append(_ActiveTask(
                worker_idx=action,
                req_cpu=self.current_task.req_cpu,
                req_memory=self.current_task.req_memory,
                req_storage=self.current_task.req_storage,
                end_time=current_time + runtime,
            ))

            reward, reward_details = self._quality_reward(
                selected, self.current_task, loads_before_std,
            )
        else:
            reward = -1.8

        # Advance to next task
        self._task_idx += 1
        terminated = False

        if self._task_idx >= len(self.tasks):
            if self.loop:
                self._task_idx = 0
                self._active_tasks.clear()
                for w in self.workers:
                    w.used_cpu = 0.0
                    w.used_memory = 0.0
                    w.used_storage = 0.0
            else:
                terminated = True
                terminal_info = {"feasible": feasible, **reward_details}
                return self._observation(), float(reward), terminated, False, terminal_info

        self.current_task = self.tasks[self._task_idx]

        info = {"feasible": feasible, "task_idx": self._task_idx, **reward_details}
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

    @staticmethod
    def _compute_task_features(task: TraceTask) -> np.ndarray:
        tt = task.task_type
        task_type_scalar = TASK_TYPE_TO_ID.get(tt, TASK_TYPE_TO_ID["mixed"]) / max(len(TASK_TYPE_TO_ID) - 1, 1)
        return np.asarray([
            task.req_cpu,
            task.req_memory,
            task.req_storage,
            task.sla_multiplier,
            task_type_scalar,
        ], dtype=np.float32)

    def _task_features(self) -> np.ndarray:
        # Clamp index for terminal observation when _task_idx overshoots
        idx = min(self._task_idx, len(self.tasks) - 1)
        return self._task_features_cache[idx]

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

    def _quality_reward(
        self,
        selected: _WorkerState,
        task: TraceTask,
        loads_before_std: float,
    ) -> Tuple[float, Dict[str, float]]:
        runtime_seconds = max(float(task.runtime_seconds), 1.0)
        sla_multiplier = max(float(task.sla_multiplier), 1.0)
        projected_load = self._normalised_load(selected)
        loads = [self._normalised_load(w) for w in self.workers]
        cluster_load = float(np.mean(loads))

        trace_queue_wait = max(float(task.queue_wait_seconds), 0.0)
        queue_wait_proxy = trace_queue_wait + (runtime_seconds * projected_load)
        turnaround_proxy = queue_wait_proxy + runtime_seconds
        sla_budget = max(runtime_seconds * sla_multiplier, 1.0)

        queue_pressure = min(queue_wait_proxy / sla_budget, 3.0)
        turnaround_pressure = min(turnaround_proxy / sla_budget, 4.0)
        tail_pressure = max(turnaround_pressure - 1.0, 0.0)
        imbalance_penalty = max(projected_load - cluster_load, 0.0)
        headroom_bonus = 1.0 - projected_load
        requeue_penalty = min(float(task.requeue_count), 4.0) * 0.05

        # Delta-imbalance: penalise actions that *worsen* cluster balance.
        # Using the change in std(loads) rather than the absolute std avoids
        # rewarding/penalising based on pre-existing state the agent did not
        # create.
        loads_after_std = float(np.std(loads))
        delta_imbalance = loads_after_std - loads_before_std

        reward = (
            1.4
            + (0.25 * headroom_bonus)
            - (0.35 * queue_pressure)
            - (0.55 * tail_pressure)
            - (0.20 * imbalance_penalty)
            - (0.40 * delta_imbalance)
            - requeue_penalty
        )

        return float(reward), {
            "queue_pressure": float(queue_pressure),
            "turnaround_pressure": float(turnaround_pressure),
            "tail_pressure": float(tail_pressure),
            "imbalance_penalty": float(imbalance_penalty),
            "delta_imbalance": float(delta_imbalance),
        }

    def _complete_tasks(self, current_time: float) -> None:
        """Release resources from tasks whose runtime has elapsed."""
        still_active: List[_ActiveTask] = []
        for at in self._active_tasks:
            if current_time >= at.end_time:
                w = self.workers[at.worker_idx]
                w.used_cpu = max(w.used_cpu - at.req_cpu, 0.0)
                w.used_memory = max(w.used_memory - at.req_memory, 0.0)
                w.used_storage = max(w.used_storage - at.req_storage, 0.0)
            else:
                still_active.append(at)
        self._active_tasks = still_active


def _safe_ratio(numer: float, denom: float) -> float:
    if denom <= 0:
        return 0.0
    return numer / denom
