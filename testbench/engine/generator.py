"""Streamlined test workload generator for Agentic Cloud Cluster."""

from __future__ import annotations

import copy
import logging
import random
from typing import List, Optional

from .schema import CanonicalTask, WorkloadSpec

LOGGER = logging.getLogger(__name__)


class TestWorkloadGenerator:
    """Derives 1-to-1 replicated and filtered test workloads from test split data."""

    def __init__(self, test_tasks: List[CanonicalTask], seed: int = 42):
        self.test_tasks = test_tasks
        self.seed = seed

    def generate_profile(
        self,
        profile: str,
        max_tasks: Optional[int] = None,
        burst_interval_sec: float = 0.2,
    ) -> WorkloadSpec:
        """Generate one of the 3 streamlined test profiles: 'default', 'bursty', or 'long-tail'."""
        profile = profile.lower().strip()

        if profile == "default":
            return self._build_default_workload(max_tasks)
        elif profile in ("bursty", "burst"):
            return self._build_bursty_workload(max_tasks, burst_interval_sec)
        elif profile in ("long-tail", "long_tail", "tail"):
            return self._build_long_tail_workload(max_tasks)
        else:
            raise ValueError(f"Unsupported test profile '{profile}'. Choose from: default, bursty, long-tail")

    def _build_default_workload(self, max_tasks: Optional[int] = None) -> WorkloadSpec:
        """Exact 1-to-1 replication of the test split in natural arrival order."""
        tasks = [copy.deepcopy(t) for t in self.test_tasks]
        tasks.sort(key=lambda t: t.arrival_offset_sec)

        if max_tasks and max_tasks > 0:
            tasks = tasks[:max_tasks]

        return WorkloadSpec(
            name=f"default-test-split-{len(tasks)}",
            profile="default",
            tasks=tasks,
            seed=self.seed,
            description="1-to-1 chronological replay of test dataset split",
            metadata={"task_count": len(tasks)},
        )

    def _build_bursty_workload(
        self,
        max_tasks: Optional[int] = None,
        burst_interval_sec: float = 0.2,
    ) -> WorkloadSpec:
        """Filters the most resource-intensive tasks (highest CPU & RAM) and batches them in burst arrivals."""
        if not self.test_tasks:
            return WorkloadSpec(name="bursty-empty", profile="bursty", tasks=[], seed=self.seed)

        # Find max CPU and RAM to normalize intensity
        max_cpu = max((t.req_cpu for t in self.test_tasks), default=1.0)
        max_mem = max((t.req_memory for t in self.test_tasks), default=1.0)

        # Calculate intensity score: (cpu / max_cpu) + (mem / max_mem)
        scored_tasks = []
        for t in self.test_tasks:
            intensity = (t.req_cpu / max(max_cpu, 1e-6)) + (t.req_memory / max(max_mem, 1e-6))
            scored_tasks.append((intensity, t))

        # Sort descending by resource intensity (heaviest tasks first)
        scored_tasks.sort(key=lambda item: item[0], reverse=True)

        selected_count = max_tasks if (max_tasks and max_tasks > 0) else max(len(scored_tasks) // 2, 1)
        selected = [copy.deepcopy(t) for _, t in scored_tasks[:selected_count]]

        # Compress arrival offsets to simulate sudden burst arrivals
        for idx, task in enumerate(selected):
            task.arrival_offset_sec = round(idx * burst_interval_sec, 2)
            task.task_id = f"burst-{idx+1:04d}-{task.task_id}"

        LOGGER.info(
            "Generated 'bursty' test workload with %d heaviest tasks (burst interval: %.2fs)",
            len(selected),
            burst_interval_sec,
        )

        return WorkloadSpec(
            name=f"bursty-heavy-{len(selected)}",
            profile="bursty",
            tasks=selected,
            seed=self.seed,
            description="High-intensity resource burst workload filtering heaviest CPU/RAM tasks",
            metadata={"task_count": len(selected), "burst_interval_sec": burst_interval_sec},
        )

    def _build_long_tail_workload(self, max_tasks: Optional[int] = None) -> WorkloadSpec:
        """Filters and organizes the longest-running tasks to stress tail latency and queueing."""
        if not self.test_tasks:
            return WorkloadSpec(name="long-tail-empty", profile="long-tail", tasks=[], seed=self.seed)

        # Sort tasks descending by execution duration
        sorted_by_duration = sorted(self.test_tasks, key=lambda t: t.duration_seconds, reverse=True)

        selected_count = max_tasks if (max_tasks and max_tasks > 0) else len(sorted_by_duration)
        top_long_tasks = [copy.deepcopy(t) for t in sorted_by_duration[:selected_count]]

        # Spread arrivals to create prolonged occupation followed by trailing arrivals
        for idx, task in enumerate(top_long_tasks):
            task.arrival_offset_sec = round(idx * 0.5, 2)
            task.task_id = f"tail-{idx+1:04d}-{task.task_id}"

        LOGGER.info("Generated 'long-tail' test workload with %d longest-running tasks", len(top_long_tasks))

        return WorkloadSpec(
            name=f"long-tail-{len(top_long_tasks)}",
            profile="long-tail",
            tasks=top_long_tasks,
            seed=self.seed,
            description="Long-tail execution workload to evaluate SLA attainment and tail latency",
            metadata={"task_count": len(top_long_tasks)},
        )


def generate_synthetic_dataset(task_count: int = 50, seed: int = 42) -> List[CanonicalTask]:
    """Generates deterministic synthetic canonical tasks when no external dataset is provided."""
    rng = random.Random(seed)
    tasks: List[CanonicalTask] = []

    profiles = [
        ("cpu-light", (0.2, 0.6), (0.2, 0.5), (2.0, 5.0)),
        ("cpu-heavy", (1.2, 2.5), (0.4, 0.8), (5.0, 15.0)),
        ("memory-heavy", (0.4, 0.8), (1.2, 2.5), (4.0, 12.0)),
        ("mixed", (0.8, 1.5), (0.8, 1.5), (3.0, 8.0)),
    ]

    current_time = 0.0
    for idx in range(task_count):
        profile_name, cpu_range, mem_range, dur_range = rng.choice(profiles)
        cpu = round(rng.uniform(*cpu_range), 2)
        mem = round(rng.uniform(*mem_range), 2)
        duration = round(rng.uniform(*dur_range), 2)
        storage = round(rng.uniform(1.0, 3.0), 2)
        sla_mult = round(rng.uniform(1.8, 2.5), 2)

        arrival = round(current_time, 2)
        current_time += rng.uniform(0.1, 1.0)

        tasks.append(
            CanonicalTask(
                task_id=f"synth-{idx+1:04d}",
                req_cpu=cpu,
                req_memory=mem,
                req_storage=storage,
                duration_seconds=duration,
                arrival_offset_sec=arrival,
                sla_multiplier=sla_mult,
                task_type=profile_name,
                docker_image="agentic/workflow-deterministic:v1",
                workflow_profile=profile_name,
                seed=1000 + idx,
            )
        )

    return tasks
