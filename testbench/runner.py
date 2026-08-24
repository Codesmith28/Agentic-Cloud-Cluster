#!/usr/bin/env python3
"""Unified Dataset-Driven Testing & Benchmarking CLI for Agentic Cloud Cluster."""

from __future__ import annotations

import argparse
import datetime
import logging
import os
import sys
import time
from pathlib import Path
from typing import List

# Ensure repository root is in python module path
REPO_ROOT = Path(__file__).resolve().parents[1]
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from testbench.engine.adapter import DatasetAdapter, split_dataset
from testbench.engine.generator import TestWorkloadGenerator, generate_synthetic_dataset
from testbench.engine.metrics import compute_comparative_summary
from testbench.engine.orchestrator import ClusterOrchestrator
from testbench.engine.reporter import BenchmarkReporter
from testbench.engine.schema import BenchmarkSummary, SchedulerTrialResult

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
LOGGER = logging.getLogger("testbench")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Agentic Cloud Cluster — Dataset-Driven Testing & Benchmarking Runner",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )

    parser.add_argument(
        "--dataset",
        type=str,
        default=str(REPO_ROOT / "testbench" / "data" / "sample_trace.csv"),
        help="Path to input dataset file (CSV, JSON, JSONL)",
    )
    parser.add_argument(
        "--mapping",
        type=str,
        default=str(REPO_ROOT / "testbench" / "configs" / "default_mapping.yaml"),
        help="Path to YAML/JSON schema mapping configuration",
    )
    parser.add_argument(
        "--profile",
        type=str,
        default="all",
        choices=["default", "bursty", "long-tail", "all"],
        help="Test profile to execute (or 'all' for sequential suite)",
    )
    parser.add_argument(
        "--schedulers",
        type=str,
        default="RR,RTS,PPO",
        help="Comma-separated schedulers to evaluate (e.g. 'RR,RTS,PPO')",
    )
    parser.add_argument(
        "--train-ratio",
        type=float,
        default=0.8,
        help="Proportion of dataset to reserve for training split (0.1 - 0.9)",
    )
    parser.add_argument(
        "--seed",
        type=int,
        default=42,
        help="Random number generator seed for deterministic reproducibility",
    )
    parser.add_argument(
        "--skip-training",
        action="store_true",
        help="Skip PPO model training on training split and use existing model",
    )
    parser.add_argument(
        "--model-output",
        type=str,
        default=str(REPO_ROOT / "agentic_scheduler" / "models" / "ppo_dataset_trained.pt"),
        help="Destination path for newly trained PPO model checkpoint",
    )
    parser.add_argument(
        "--synthetic",
        action="store_true",
        help="Generate synthetic dataset instead of reading external dataset file",
    )
    parser.add_argument(
        "--max-tasks",
        type=int,
        default=0,
        help="Maximum tasks to load/evaluate (0 for all available)",
    )
    parser.add_argument(
        "--master-url",
        type=str,
        default=os.getenv("MASTER_URL", "http://localhost:8080"),
        help="Master node REST API URL",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default="",
        help="Directory to save benchmark reports (default: results/benchmarks/<timestamp>)",
    )
    parser.add_argument(
        "--mock-dry-run",
        action="store_true",
        help="Perform dry-run split and test generation without connecting to live cluster",
    )

    return parser.parse_args()


def main() -> None:
    args = parse_args()

    started_at_str = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    timestamp_slug = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")

    output_dir = Path(args.output_dir) if args.output_dir else (REPO_ROOT / "results" / "benchmarks" / timestamp_slug)
    output_dir.mkdir(parents=True, exist_ok=True)

    print("═══════════════════════════════════════════════════════════════════════")
    print("  Agentic Cloud Cluster — Dataset-Driven Testing & Benchmark Engine")
    print("═══════════════════════════════════════════════════════════════════════")
    print(f"  Dataset:      {args.dataset if not args.synthetic else 'SYNTHETIC'}")
    print(f"  Mapping:      {args.mapping}")
    print(f"  Profile:      {args.profile}")
    print(f"  Schedulers:   {args.schedulers}")
    print(f"  Train Ratio:  {args.train_ratio * 100:.0f}%")
    print(f"  Seed:         {args.seed}")
    print(f"  Output Dir:   {output_dir}")
    print("═══════════════════════════════════════════════════════════════════════\n")

    # 1. Dataset Ingestion & Schema Adaptation
    if args.synthetic:
        LOGGER.info("Generating synthetic canonical tasks (count=%d, seed=%d)...", args.max_tasks or 50, args.seed)
        tasks = generate_synthetic_dataset(task_count=args.max_tasks or 50, seed=args.seed)
        dataset_name = "synthetic"
    else:
        adapter = DatasetAdapter(mapping_config=args.mapping)
        tasks = adapter.load_dataset(dataset_path=args.dataset, max_records=args.max_tasks if args.max_tasks > 0 else None)
        dataset_name = Path(args.dataset).name

    if not tasks:
        LOGGER.error("No canonical tasks available to execute. Exiting.")
        sys.exit(1)

    # 2. Deterministic Train/Test Split
    split = split_dataset(
        tasks=tasks,
        train_ratio=args.train_ratio,
        seed=args.seed,
        source_name=dataset_name,
    )

    orchestrator = ClusterOrchestrator(master_url=args.master_url)

    # 3. Model Training Phase (if PPO is in schedulers and not skipped)
    target_schedulers = [s.strip().upper() for s in args.schedulers.split(",") if s.strip()]
    if "PPO" in target_schedulers and not args.skip_training and not args.mock_dry_run:
        LOGGER.info("Executing Pre-Test Model Training Phase...")
        ok = orchestrator.train_ppo_model(split=split, model_output_path=Path(args.model_output))
        if not ok:
            LOGGER.warning("PPO training did not produce expected model. Proceeding with existing weights...")

    # 4. Workload Generation
    generator = TestWorkloadGenerator(test_tasks=split.test_tasks, seed=args.seed)

    profiles_to_run: List[str] = []
    if args.profile == "all":
        profiles_to_run = ["default", "bursty", "long-tail"]
    else:
        profiles_to_run = [args.profile]

    if args.mock_dry_run:
        LOGGER.info("Dry-run requested: generated test split with %d tasks.", len(split.test_tasks))
        for p in profiles_to_run:
            w = generator.generate_profile(profile=p)
            LOGGER.info("Generated profile '%s' with %d tasks", p, len(w.tasks))
        print("\n✅ Dry run complete!")
        return

    # 5. Cluster Health Verification
    LOGGER.info("Verifying Master node connectivity at %s...", args.master_url)
    if not orchestrator.check_health(timeout_sec=10):
        LOGGER.error(
            "Master node is not accessible at %s.\n"
            "Please ensure MongoDB and masterNode are running:\n"
            "  1. docker compose -f database/docker-compose.yml up -d\n"
            "  2. ./master/masterNode --mode headless &\n",
            args.master_url,
        )
        sys.exit(1)

    # 6. Execute Benchmark Suite
    all_trial_results: List[SchedulerTrialResult] = []

    for profile_name in profiles_to_run:
        workload = generator.generate_profile(profile=profile_name)

        for sched in target_schedulers:
            trial_result = orchestrator.execute_workload(
                workload=workload,
                scheduler=sched,
                poll_interval_sec=1.0,
            )
            all_trial_results.append(trial_result)

    finished_at_str = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    # 7. Metrics Aggregation & Reporting
    summary_matrix = compute_comparative_summary(all_trial_results)
    benchmark_summary = BenchmarkSummary(
        title="Agentic Cloud Cluster — Benchmark Evaluation Report",
        started_at=started_at_str,
        finished_at=finished_at_str,
        dataset_name=dataset_name,
        seed=args.seed,
        trials=all_trial_results,
        summary_matrix=summary_matrix,
    )

    reporter = BenchmarkReporter(output_dir=output_dir)
    reporter.export_all(benchmark_summary)

    # 8. Print Final Markdown Summary to Terminal
    md_content = (output_dir / "summary.md").read_text(encoding="utf-8")
    print("\n" + md_content)
    print(f"📊 Detailed evidence exported to: {output_dir}\n")


if __name__ == "__main__":
    main()
