"""Cluster execution and lifecycle orchestrator for Agentic Cloud Cluster benchmarks."""

from __future__ import annotations

import json
import logging
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Dict, List, Optional
from urllib.error import URLError
from urllib.request import Request, urlopen

from .adapter import DatasetSplit, export_split_for_training
from .metrics import compute_trial_metrics
from .schema import CanonicalTask, SchedulerTrialResult, TaskExecutionRecord, WorkloadSpec

LOGGER = logging.getLogger(__name__)

TERMINAL_STATUSES = {"SUCCESS", "FAILED", "TIMEOUT", "CANCELLED", "LOST"}


def _http_request(
    method: str,
    url: str,
    payload: Optional[Dict[str, Any]] = None,
    timeout: float = 15.0,
) -> Dict[str, Any]:
    """Execute HTTP JSON request to Master REST API."""
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    req = Request(url, data=data, headers=headers, method=method.upper())

    try:
        with urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8")
            if not raw:
                return {}
            return json.loads(raw)
    except Exception as exc:
        LOGGER.debug("HTTP %s %s failed: %s", method, url, exc)
        raise


class ClusterOrchestrator:
    """Orchestrates model training, scheduler switching, task dispatch, and status polling."""

    def __init__(self, master_url: str = "http://localhost:8080", python_bin: Optional[str] = None):
        self.master_url = master_url.rstrip("/")
        self.python_bin = python_bin or sys.executable

    def check_health(self, timeout_sec: int = 15) -> bool:
        """Verify master node HTTP API is accessible."""
        deadline = time.time() + timeout_sec
        while time.time() < deadline:
            try:
                data = _http_request("GET", f"{self.master_url}/api/workers", timeout=3.0)
                if isinstance(data, dict):
                    return True
            except Exception:
                time.sleep(1.0)
        return False

    def switch_scheduler(self, algorithm: str) -> bool:
        """Switch active scheduler on the master node (RR, RTS, or PPO)."""
        algo = algorithm.upper().strip()
        try:
            resp = _http_request(
                "POST",
                f"{self.master_url}/api/config/scheduler",
                {"algorithm": algo},
                timeout=5.0,
            )
            if not resp.get("success", False):
                LOGGER.warning("Scheduler switch to %s returned unsuccessful: %s", algo, resp)
                return False

            # Verify active scheduler
            status_resp = _http_request("GET", f"{self.master_url}/api/config/scheduler", timeout=5.0)
            current = str(status_resp.get("algorithm", status_resp.get("current", ""))).upper()
            if algo in current:
                LOGGER.info("Active cluster scheduler verified: %s", algo)
                return True
            return True
        except Exception as exc:
            LOGGER.error("Failed to switch scheduler to %s: %s", algo, exc)
            return False

    def drain_cluster(self, timeout_sec: int = 60) -> bool:
        """Wait until all active tasks are in terminal states."""
        deadline = time.time() + timeout_sec
        while time.time() < deadline:
            try:
                resp = _http_request("GET", f"{self.master_url}/api/tasks", timeout=5.0)
                tasks = resp.get("tasks", [])
                active = [
                    t for t in tasks
                    if str(t.get("status", "")).upper() not in TERMINAL_STATUSES
                ]
                if not active:
                    return True
                LOGGER.debug("Waiting for %d active tasks to drain...", len(active))
            except Exception:
                pass
            time.sleep(1.5)
        LOGGER.warning("Drain cluster timed out after %ds", timeout_sec)
        return False

    def train_ppo_model(
        self,
        split: DatasetSplit,
        model_output_path: Path,
        epochs: int = 10,
        batch_size: int = 64,
    ) -> bool:
        """Train PPO neural network policy on the dataset training split."""
        LOGGER.info("Starting PPO training phase on %d training tasks...", len(split.train_tasks))
        temp_trace = Path("/tmp/agentic_train_trace.json")
        export_split_for_training(split, temp_trace)

        model_output_path.parent.mkdir(parents=True, exist_ok=True)

        cmd = [
            self.python_bin,
            "-m",
            "agentic_scheduler.train_ppo",
            "--trace-source",
            "agentic",
            "--trace-path",
            str(temp_trace),
            "--output",
            str(model_output_path),
            "--updates",
            str(epochs),
            "--minibatch-size",
            str(batch_size),
        ]

        LOGGER.info("Executing PPO trainer: %s", " ".join(cmd))
        proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
        if proc.returncode != 0:
            LOGGER.error("PPO model training failed with code %d:\n%s", proc.returncode, proc.stderr)
            return False

        LOGGER.info("PPO model training complete! Output saved to: %s", model_output_path)
        return model_output_path.exists()

    def execute_workload(
        self,
        workload: WorkloadSpec,
        scheduler: str,
        poll_interval_sec: float = 1.0,
        max_timeout_sec: int = 300,
    ) -> SchedulerTrialResult:
        """Execute a workload specification against the cluster and collect metrics."""
        LOGGER.info(
            "═══════════════════════════════════════════════════════════════════\n"
            "  Starting Benchmark Trial: Profile='%s' | Scheduler='%s' | Tasks=%d\n"
            "═══════════════════════════════════════════════════════════════════",
            workload.profile,
            scheduler,
            len(workload.tasks),
        )

        # 1. Switch scheduler and drain
        self.switch_scheduler(scheduler)
        self.drain_cluster(timeout_sec=30)

        trial_start = time.time()
        submitted_task_ids: List[str] = []

        # 2. Dispatch tasks respecting arrival offsets
        for task in workload.tasks:
            # Sleep if arrival offset has not arrived yet
            target_arrival = trial_start + task.arrival_offset_sec
            sleep_time = target_arrival - time.time()
            if sleep_time > 0.05:
                time.sleep(sleep_time)

            payload = task.to_cluster_request()
            try:
                resp = _http_request("POST", f"{self.master_url}/api/tasks", payload, timeout=10.0)
                submitted_id = str(resp.get("task_id", task.task_id))
                submitted_task_ids.append(submitted_id)
            except Exception as exc:
                LOGGER.error("Failed to submit task %s: %s", task.task_id, exc)
                submitted_task_ids.append(task.task_id)

        LOGGER.info("Submitted %d tasks to cluster. Polling for completion...", len(submitted_task_ids))

        # 3. Poll for completion of all submitted tasks
        submitted_set = set(submitted_task_ids)
        deadline = time.time() + max_timeout_sec
        finished_tasks: Dict[str, Dict[str, Any]] = {}

        while time.time() < deadline:
            try:
                resp = _http_request("GET", f"{self.master_url}/api/tasks", timeout=10.0)
                tasks_list = resp.get("tasks", [])
                for t in tasks_list:
                    tid = str(t.get("task_id", t.get("id", "")))
                    if tid in submitted_set:
                        status = str(t.get("status", "")).upper()
                        if status in TERMINAL_STATUSES:
                            finished_tasks[tid] = t

                if len(finished_tasks) >= len(submitted_set):
                    break
            except Exception as exc:
                LOGGER.debug("Polling error: %s", exc)

            time.sleep(poll_interval_sec)

        trial_end = time.time()
        total_duration = trial_end - trial_start

        # 4. Construct execution records
        records: List[TaskExecutionRecord] = []
        for task in workload.tasks:
            raw = finished_tasks.get(task.task_id, {})
            status = str(raw.get("status", "TIMEOUT")).upper()
            worker_id = str(raw.get("worker_id", raw.get("assigned_worker", "")))

            # Extract duration and turnaround
            duration = float(raw.get("duration", raw.get("execution_duration", task.duration_seconds)))
            turnaround = float(raw.get("turnaround_time", raw.get("total_duration", duration)))
            wait_time = float(raw.get("wait_time", max(turnaround - duration, 0.0)))
            sla_target = round(duration * float(task.sla_multiplier), 3)
            sla_met = (turnaround <= sla_target) if status == "SUCCESS" else False

            records.append(
                TaskExecutionRecord(
                    task_id=task.task_id,
                    scheduler=scheduler,
                    worker_id=worker_id,
                    status=status,
                    submitted_at=trial_start + task.arrival_offset_sec,
                    completed_at=trial_end,
                    wait_duration_sec=round(wait_time, 3),
                    execution_duration_sec=round(duration, 3),
                    turnaround_sec=round(turnaround, 3),
                    sla_target_sec=sla_target,
                    sla_met=sla_met,
                    exit_code=int(raw.get("exit_code", 0)),
                    error=str(raw.get("error", "")),
                )
            )

        trial_result = compute_trial_metrics(
            records=records,
            scheduler=scheduler,
            profile=workload.profile,
            total_duration_sec=total_duration,
        )

        LOGGER.info(
            "Trial Complete [%s | %s]: Success=%.1f%% | SLA Attainment=%.1f%% | P95 Turnaround=%.2fs | Makespan=%.1fs",
            scheduler,
            workload.profile,
            trial_result.success_rate,
            trial_result.sla_attainment_rate,
            trial_result.p95_turnaround_sec,
            trial_result.makespan_sec,
        )

        return trial_result
