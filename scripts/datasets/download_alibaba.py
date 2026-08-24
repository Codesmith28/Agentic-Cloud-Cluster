#!/usr/bin/env python3
"""Download and stage official Alibaba Cluster Trace v2018 dataset for Agentic Cloud Cluster.

Downloads the genuine production cluster trace directly from the official Alibaba Cloud Open Trace repository:
http://aliopentrace.oss-cn-beijing.aliyuncs.com/v2018Traces/batch_task.tar.gz
"""

from __future__ import annotations

import argparse
import logging
import os
import tarfile
import urllib.request
from pathlib import Path

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s", datefmt="%H:%M:%S")
LOGGER = logging.getLogger("download_alibaba")

REPO_ROOT = Path(__file__).resolve().parents[2]
RAW_DATA_DIR = REPO_ROOT / "testbench" / "data" / "raw"

ALIBABA_BASE_URL = "http://aliopentrace.oss-cn-beijing.aliyuncs.com/v2018Traces"
BATCH_TASK_URL = f"{ALIBABA_BASE_URL}/batch_task.tar.gz"
MACHINE_META_URL = f"{ALIBABA_BASE_URL}/machine_meta.tar.gz"


def download_and_extract_alibaba(dest_dir: Path) -> Path:
    dest_dir.mkdir(parents=True, exist_ok=True)
    task_csv = dest_dir / "alibaba_batch_task.csv"
    machine_csv = dest_dir / "machine_meta.csv"

    # 1. Download & Extract Machine Meta
    if not machine_csv.exists():
        machine_tar = dest_dir / "machine_meta.tar.gz"
        LOGGER.info("Downloading official Alibaba machine_meta.tar.gz from Aliyun OSS...")
        urllib.request.urlretrieve(MACHINE_META_URL, machine_tar)
        LOGGER.info("Extracting %s...", machine_tar.name)
        with tarfile.open(machine_tar, "r:gz") as tar:
            tar.extractall(path=dest_dir)
        if (dest_dir / "machine_meta.csv").exists():
            LOGGER.info("Successfully extracted machine_meta.csv (4,034 machines)")

    # 2. Download & Extract Batch Task
    if task_csv.exists() and task_csv.stat().st_size > 50_000_000:
        LOGGER.info("Full official Alibaba batch_task.csv already present at: %s (size: %.1f MB)", task_csv, task_csv.stat().st_size / (1024 * 1024))
        return task_csv

    task_tar = dest_dir / "batch_task.tar.gz"
    LOGGER.info("Downloading official Alibaba batch_task.tar.gz (130 MB compressed, ~1.1 GB raw)...")
    urllib.request.urlretrieve(BATCH_TASK_URL, task_tar)
    LOGGER.info("Extracting batch_task.tar.gz...")
    with tarfile.open(task_tar, "r:gz") as tar:
        tar.extractall(path=dest_dir)

    # Rename extracted batch_task.csv to alibaba_batch_task.csv if needed
    extracted_csv = dest_dir / "batch_task.csv"
    if extracted_csv.exists():
        if extracted_csv != task_csv:
            extracted_csv.replace(task_csv)

    LOGGER.info("Successfully staged full official Alibaba Cluster Trace to: %s (size: %.1f MB)", task_csv, task_csv.stat().st_size / (1024 * 1024))
    return task_csv


def main() -> None:
    parser = argparse.ArgumentParser(description="Download official Alibaba Cluster Trace v2018")
    parser.add_argument("--output-dir", type=str, default=str(RAW_DATA_DIR), help="Output raw directory")
    args = parser.parse_args()

    staged_path = download_and_extract_alibaba(Path(args.output_dir))
    print(f"\n✅ Official Alibaba dataset successfully downloaded & staged at: {staged_path}")
    print(f"👉 Schema mapping: testbench/configs/alibaba_mapping.yaml")
    print(f"👉 Run benchmark: python3 testbench/runner.py --dataset {staged_path} --mapping testbench/configs/alibaba_mapping.yaml --profile all\n")


if __name__ == "__main__":
    main()
