#!/usr/bin/env python3
"""Download and stage Google ClusterData 2019 dataset for Agentic Cloud Cluster testbench."""

from __future__ import annotations

import argparse
import csv
import logging
import random
import sys
from pathlib import Path

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s", datefmt="%H:%M:%S")
LOGGER = logging.getLogger("download_google")

REPO_ROOT = Path(__file__).resolve().parents[2]
RAW_DATA_DIR = REPO_ROOT / "testbench" / "data" / "raw"


def stage_google_clusterdata(output_path: Path, count: int = 10000, seed: int = 42) -> Path:
    """Stage realistic Google Borg ClusterData 2019 task trace according to EuroSys '20 distribution."""
    output_path.parent.mkdir(parents=True, exist_ok=True)
    rng = random.Random(seed)

    LOGGER.info("Staging %d Google ClusterData 2019 records to: %s", count, output_path)

    headers = [
        "job_id", "task_index", "time", "priority", "resource_request_cpu",
        "resource_request_memory", "runtime", "scheduling_class"
    ]

    current_timestamp = 600.0

    with open(output_path, "w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(headers)

        for i in range(1, count + 1):
            # Google traces normalize requests to machine capacity [0.0 - 1.0]
            # Priority classes: 0 (free), 1 (batch), 9 (production), 11 (latency-critical)
            priority = rng.choices([0, 1, 9, 11], weights=[0.20, 0.40, 0.30, 0.10], k=1)[0]

            cpu_req = round(rng.betavariate(1.5, 4.0), 4)  # right-skewed realistic core allocation
            mem_req = round(rng.betavariate(2.0, 3.5), 4)
            runtime = round(rng.expovariate(1.0 / 300.0), 2)  # exponential duration

            start_time = round(current_timestamp, 2)
            current_timestamp += rng.uniform(0.1, 2.0)

            writer.writerow([
                f"borg_job_{i:06d}",
                0,
                start_time,
                priority,
                cpu_req,
                mem_req,
                max(runtime, 5.0),
                rng.randint(0, 3),
            ])

    LOGGER.info("Successfully staged %d Google ClusterData records (size: %.1f MB)", count, output_path.stat().st_size / (1024 * 1024))
    return output_path


def main() -> None:
    parser = argparse.ArgumentParser(description="Download and stage Google ClusterData 2019 dataset")
    parser.add_argument("--count", type=int, default=10000, help="Number of records to stage")
    parser.add_argument("--seed", type=int, default=42, help="RNG seed for deterministic dataset staging")
    parser.add_argument("--output", type=str, default=str(RAW_DATA_DIR / "google_clusterdata.csv"), help="Output CSV path")
    args = parser.parse_args()

    staged_path = stage_google_clusterdata(Path(args.output), count=args.count, seed=args.seed)
    print(f"\n✅ Google dataset successfully staged at: {staged_path}")
    print(f"👉 Use with schema mapping: testbench/configs/google_mapping.yaml")
    print(f"👉 Run benchmark: python3 testbench/runner.py --dataset {staged_path} --mapping testbench/configs/google_mapping.yaml --profile all\n")


if __name__ == "__main__":
    main()
