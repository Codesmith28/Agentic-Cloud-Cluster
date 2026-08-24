"""Canonical data models for Agentic Cloud Cluster testing and benchmarking engine."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional


@dataclass
class CanonicalTask:
    """Standardized task specification matching Agentic Cloud Cluster requirements."""

    task_id: str
    req_cpu: float
    req_memory: float
    req_storage: float
    duration_seconds: float
    arrival_offset_sec: float = 0.0
    sla_multiplier: float = 2.0
    task_type: str = "mixed"  # "cpu-light", "cpu-heavy", "memory-heavy", "mixed"
    docker_image: str = "agentic/workflow-deterministic:v1"
    workflow_profile: str = "mixed"
    workflow_args: Dict[str, Any] = field(default_factory=dict)
    seed: int = 42

    def to_cluster_request(self) -> Dict[str, Any]:
        """Convert canonical task to Master API /api/tasks submission payload."""
        payload: Dict[str, Any] = {
            "task_id": self.task_id,
            "docker_image": self.docker_image,
            "cpu_required": float(self.req_cpu),
            "memory_required": float(self.req_memory),
            "storage_required": float(self.req_storage),
            "sla_multiplier": float(self.sla_multiplier),
            "task_type": self.task_type,
            "tag": self.task_type,
            "k_value": float(self.sla_multiplier),
        }

        # Format workflow command if using deterministic workflow image
        if self.workflow_profile:
            payload["workflow"] = {
                "profile": self.workflow_profile,
                "args": self.workflow_args or {
                    "iterations": int(max(self.duration_seconds * 100000, 10000)),
                    "seed": self.seed,
                },
            }
        return payload


@dataclass
class DatasetSplit:
    """Holds train and test splits derived from an ingested dataset."""

    train_tasks: List[CanonicalTask]
    test_tasks: List[CanonicalTask]
    source_name: str = ""
    train_ratio: float = 0.8
    seed: int = 42
    metadata: Dict[str, Any] = field(default_factory=dict)


@dataclass
class WorkloadSpec:
    """Workload specification generated for a specific test profile."""

    name: str
    profile: str  # "default", "bursty", "long-tail"
    tasks: List[CanonicalTask]
    seed: int = 42
    description: str = ""
    metadata: Dict[str, Any] = field(default_factory=dict)


@dataclass
class TaskExecutionRecord:
    """Execution lifecycle record of a single task run on the cluster."""

    task_id: str
    scheduler: str
    worker_id: str = ""
    status: str = "UNKNOWN"  # "SUCCESS", "FAILED", "TIMEOUT"
    submitted_at: float = 0.0
    scheduled_at: float = 0.0
    completed_at: float = 0.0
    wait_duration_sec: float = 0.0
    execution_duration_sec: float = 0.0
    turnaround_sec: float = 0.0
    sla_target_sec: float = 0.0
    sla_met: bool = False
    exit_code: int = 0
    error: str = ""


@dataclass
class SchedulerTrialResult:
    """Benchmark trial outcome for a single scheduler on a workload profile."""

    scheduler: str  # "RR", "RTS", "PPO"
    profile: str    # "default", "bursty", "long-tail"
    tasks_submitted: int = 0
    tasks_completed: int = 0
    tasks_failed: int = 0
    success_rate: float = 0.0
    sla_attainment_rate: float = 0.0
    total_duration_sec: float = 0.0
    avg_turnaround_sec: float = 0.0
    p50_turnaround_sec: float = 0.0
    p90_turnaround_sec: float = 0.0
    p95_turnaround_sec: float = 0.0
    p99_turnaround_sec: float = 0.0
    avg_wait_sec: float = 0.0
    p95_wait_sec: float = 0.0
    makespan_sec: float = 0.0
    worker_load_variance: float = 0.0
    task_records: List[TaskExecutionRecord] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)


@dataclass
class BenchmarkSummary:
    """Complete comparative benchmark report across all schedulers and profiles."""

    title: str
    started_at: str
    finished_at: str
    dataset_name: str
    seed: int
    trials: List[SchedulerTrialResult] = field(default_factory=list)
    summary_matrix: Dict[str, Any] = field(default_factory=dict)
