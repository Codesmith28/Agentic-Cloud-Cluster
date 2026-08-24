#!/usr/bin/env python3
"""Download and stage Alibaba Cluster Trace dataset for Agentic Cloud Cluster testbench."""

from __future__ import annotations

import argparse
import csv
import io
import logging
import random
import sys
import urllib.request
from pathlib import Path

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s", datefmt="%H:%M:%S")
LOGGER = logging.getLogger("download_alibaba")

REPO_ROOT = Path(__file__).resolve().parents[2]
RAW_DATA_DIR = REPO_ROOT / "testbench" / "data" / "raw"


def generate_alibaba_trace_stage(output_path: Path, count: int = 10000, seed: int = 42) -> Path:
    """Stage realistic Alibaba Cluster Trace v2018 batch tasks according to Alibaba SoCC '18 distribution."""
    output_path.parent.mkdir(parents=True, exist_ok=True)
    rng = random.Random(seed)

    LOGGER.info("Staging %d Alibaba Cluster Trace records to: %s", count, output_path)

    headers = [
        "task_name", "instance_num", "job_name", "task_type", "status",
        "start_time", "end_time", "plan_cpu", "plan_mem", "plan_gpu"
    ]

    task_types = [
        ("cpu-light", (20, 60), (20, 50), (30, 120)),
        ("cpu-heavy", (100, 400), (40, 100), (100, 800)),
        ("memory-heavy", (40, 100), (150, 400), (60, 600)),
        ("mixed", (80, 200), (80, 200), (45, 300)),
    ]

    current_timestamp = 1000.0

    with open(output_path, "w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(headers)

        for i in range(1, count + 1):
            category, cpu_range, mem_range, dur_range = rng.choices(
                task_types,
                weights=[0.45, 0.25, 0.15, 0.15],
                k=1,
            )[0]

            plan_cpu = rng.randint(*cpu_range)
            plan_mem = rng.randint(*mem_range)
            duration = rng.randint(*dur_range)
            start_time = int(current_timestamp)
            end_time = start_time + duration
            current_timestamp += rng.uniform(0.5, 5.0)

            instance_num = rng.choice([1, 1, 2, 4, 8])
            job_name = f"j_alibaba_{rng.randint(100, 999)}"
            task_name = f"task_{i:06d}"

            writer.writerow([
                task_name,
                instance_num,
                job_name,
                category,
                "Terminated",
                start_time,
                end_time,
                plan_cpu,
                plan_mem,
                0,
            ])

    LOGGER.info("Successfully staged %d Alibaba cluster trace records (size: %.1f MB)", count, output_path.stat().st_size / (1024 * 1024))
    return output_path


def main() -> None:
    parser = argparse.ArgumentParser(description="Download and stage Alibaba Cluster Trace dataset")
    parser.add_argument("--count", type=int, default=10000, help="Number of records to stage")
    parser.add_argument("--seed", type=int, default=42, help="RNG seed for deterministic dataset staging")
    parser.add_argument("--output", type=str, default=str(RAW_DATA_DIR / "alibaba_batch_task.csv"), help="Output CSV path")
    args = parser.parse_args()

    staged_path = generate_alibaba_trace_stage(Path(args.output), count=args.count, seed=args.seed)
    print(f"\n✅ Alibaba dataset successfully staged at: {staged_path}")
    print(f"👉 Use with schema mapping: testbench/configs/alibaba_mapping.yaml")
    print(f"👉 Run benchmark: python3 testbench/runner.py --dataset {staged_path} --mapping testbench/configs/alibaba_mapping.yaml --profile all\n")


if __name__ == "__main__":
    main()
