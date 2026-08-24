"""Unit tests for the Agentic Cloud Cluster testing and benchmarking engine."""

import json
import tempfile
import unittest
from pathlib import Path

from testbench.engine.adapter import DatasetAdapter, split_dataset
from testbench.engine.generator import TestWorkloadGenerator, generate_synthetic_dataset
from testbench.engine.metrics import compute_comparative_summary, compute_trial_metrics
from testbench.engine.reporter import BenchmarkReporter
from testbench.engine.schema import (
    BenchmarkSummary,
    CanonicalTask,
    TaskExecutionRecord,
)


class TestEngineComponents(unittest.TestCase):
    """Verify schema mapping, train/test splitting, profile generation, and metrics calculation."""

    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.work_dir = Path(self.temp_dir.name)

        # Create dummy sample CSV
        self.csv_path = self.work_dir / "sample.csv"
        self.csv_path.write_text(
            "task_id,cpu,memory,disk,duration,timestamp,sla\n"
            "t-01,1.0,0.5,1.0,10.0,0.0,2.0\n"
            "t-02,2.0,1.5,2.0,20.0,1.0,2.0\n"
            "t-03,0.5,0.25,1.0,5.0,2.0,2.0\n"
            "t-04,3.0,2.0,3.0,30.0,3.0,2.0\n"
            "t-05,0.8,0.8,1.5,8.0,4.0,2.0\n",
            encoding="utf-8",
        )

        self.mapping_config = {
            "fields": {
                "task_id": "task_id",
                "req_cpu": "cpu",
                "req_memory": "memory",
                "req_storage": "disk",
                "duration_seconds": "duration",
                "arrival_offset_sec": "timestamp",
                "sla_multiplier": "sla",
            },
            "transforms": {
                "req_cpu": {"scale": 1.0},
                "req_memory": {"scale": 1.0},
                "duration_seconds": {"scale": 1.0},
            },
            "defaults": {
                "docker_image": "agentic/workflow-deterministic:v1",
            },
        }

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_dataset_adapter_and_mapping(self):
        adapter = DatasetAdapter(mapping_config=self.mapping_config)
        tasks = adapter.load_dataset(self.csv_path)

        self.assertEqual(len(tasks), 5)
        self.assertEqual(tasks[0].task_id, "t-01")
        self.assertEqual(tasks[0].req_cpu, 1.0)
        self.assertEqual(tasks[0].duration_seconds, 10.0)
        self.assertEqual(tasks[3].req_cpu, 3.0)

    def test_train_test_split_deterministic(self):
        tasks = generate_synthetic_dataset(task_count=20, seed=123)

        split1 = split_dataset(tasks, train_ratio=0.8, seed=42)
        split2 = split_dataset(tasks, train_ratio=0.8, seed=42)

        self.assertEqual(len(split1.train_tasks), 16)
        self.assertEqual(len(split1.test_tasks), 4)

        # Same seed must produce exact same partition
        self.assertEqual([t.task_id for t in split1.train_tasks], [t.task_id for t in split2.train_tasks])
        self.assertEqual([t.task_id for t in split1.test_tasks], [t.task_id for t in split2.test_tasks])

    def test_test_workload_generator_profiles(self):
        tasks = [
            CanonicalTask("task-1", req_cpu=0.5, req_memory=0.25, req_storage=1.0, duration_seconds=5.0, arrival_offset_sec=0.0),
            CanonicalTask("task-2", req_cpu=2.5, req_memory=2.0, req_storage=2.0, duration_seconds=15.0, arrival_offset_sec=1.0),
            CanonicalTask("task-3", req_cpu=1.0, req_memory=0.8, req_storage=1.0, duration_seconds=8.0, arrival_offset_sec=2.0),
            CanonicalTask("task-4", req_cpu=3.0, req_memory=2.5, req_storage=3.0, duration_seconds=40.0, arrival_offset_sec=3.0),
        ]

        generator = TestWorkloadGenerator(test_tasks=tasks, seed=42)

        # 1. Default Profile (1-to-1 exact)
        default_workload = generator.generate_profile("default")
        self.assertEqual(len(default_workload.tasks), 4)
        self.assertEqual(default_workload.tasks[0].task_id, "task-1")

        # 2. Bursty Profile (heaviest tasks)
        bursty_workload = generator.generate_profile("bursty", max_tasks=2, burst_interval_sec=0.1)
        self.assertEqual(len(bursty_workload.tasks), 2)
        # Heaviest CPU+RAM task is task-4 (3.0 CPU, 2.5 RAM)
        self.assertIn("task-4", bursty_workload.tasks[0].task_id)
        self.assertEqual(bursty_workload.tasks[0].arrival_offset_sec, 0.0)
        self.assertEqual(bursty_workload.tasks[1].arrival_offset_sec, 0.1)

        # 3. Long-tail Profile (longest duration tasks)
        tail_workload = generator.generate_profile("long-tail", max_tasks=2)
        self.assertEqual(len(tail_workload.tasks), 2)
        # Longest task is task-4 (40s), followed by task-2 (15s)
        self.assertIn("task-4", tail_workload.tasks[0].task_id)
        self.assertIn("task-2", tail_workload.tasks[1].task_id)

    def test_metrics_computation(self):
        records = [
            TaskExecutionRecord(
                task_id="t1",
                scheduler="PPO",
                worker_id="w1",
                status="SUCCESS",
                wait_duration_sec=0.5,
                execution_duration_sec=10.0,
                turnaround_sec=10.5,
                sla_target_sec=20.0,
                sla_met=True,
            ),
            TaskExecutionRecord(
                task_id="t2",
                scheduler="PPO",
                worker_id="w2",
                status="SUCCESS",
                wait_duration_sec=1.0,
                execution_duration_sec=5.0,
                turnaround_sec=12.0,
                sla_target_sec=10.0,
                sla_met=False,  # Breached SLA (12s > 10s)
            ),
        ]

        result = compute_trial_metrics(records, scheduler="PPO", profile="default", total_duration_sec=15.0)

        self.assertEqual(result.tasks_submitted, 2)
        self.assertEqual(result.tasks_completed, 2)
        self.assertEqual(result.success_rate, 100.0)
        self.assertEqual(result.sla_attainment_rate, 50.0)  # 1 out of 2 met SLA
        self.assertAlmostEqual(result.avg_turnaround_sec, 11.25, places=2)

    def test_reporter_exports(self):
        records = [
            TaskExecutionRecord(
                task_id="t1",
                scheduler="PPO",
                worker_id="w1",
                status="SUCCESS",
                wait_duration_sec=0.2,
                execution_duration_sec=5.0,
                turnaround_sec=5.2,
                sla_target_sec=10.0,
                sla_met=True,
            )
        ]
        trial = compute_trial_metrics(records, "PPO", "default", 6.0)
        summary = BenchmarkSummary(
            title="Test Benchmark",
            started_at="2026-08-24 10:00:00",
            finished_at="2026-08-24 10:05:00",
            dataset_name="sample.csv",
            seed=42,
            trials=[trial],
            summary_matrix=compute_comparative_summary([trial]),
        )

        reporter = BenchmarkReporter(self.work_dir)
        reporter.export_all(summary)

        self.assertTrue((self.work_dir / "summary.json").exists())
        self.assertTrue((self.work_dir / "summary.md").exists())
        self.assertTrue((self.work_dir / "tasks.csv").exists())

        md_text = (self.work_dir / "summary.md").read_text(encoding="utf-8")
        self.assertIn("SLA Attainment", md_text)
        self.assertIn("PPO", md_text)


if __name__ == "__main__":
    unittest.main()
