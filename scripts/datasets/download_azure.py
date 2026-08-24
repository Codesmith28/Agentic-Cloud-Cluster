#!/usr/bin/env python3
"""Download and stage Microsoft Azure Public Dataset for Agentic Cloud Cluster testbench."""

from __future__ import annotations

import argparse
import csv
import gzip
import io
import logging
import os
import sys
import urllib.request
from pathlib import Path

# Setup logging
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s", datefmt="%H:%M:%S")
LOGGER = logging.getLogger("download_azure")

REPO_ROOT = Path(__file__).resolve().parents[2]
RAW_DATA_DIR = REPO_ROOT / "testbench" / "data" / "raw"
AZURE_URL = "https://github.com/Azure/AzurePublicDataset/releases/download/dataset-v2/trace_data_vmtable_vmtable.csv.gz"
SCHEMA_URL = "https://github.com/Azure/AzurePublicDataset/releases/download/dataset-v2/schema.csv"


def download_and_stage_azure(sample_only: bool = False, max_sample_rows: int = 5000) -> Path:
    RAW_DATA_DIR.mkdir(parents=True, exist_ok=True)

    # 1. Download schema metadata
    schema_file = RAW_DATA_DIR / "azure_schema.csv"
    if not schema_file.exists():
        LOGGER.info("Downloading Azure schema definition from GitHub releases...")
        urllib.request.urlretrieve(SCHEMA_URL, schema_file)
        LOGGER.info("Saved schema metadata to %s", schema_file)

    dest_gz = RAW_DATA_DIR / "azure_vmtable_full.csv.gz"
    dest_sample = RAW_DATA_DIR / "azure_vmtable_sample.csv"

    # 2. If full download requested or already present
    if not sample_only:
        if dest_gz.exists() and dest_gz.stat().st_size > 10_000_000:
            LOGGER.info("Full Azure dataset already present at: %s (size: %.1f MB)", dest_gz, dest_gz.stat().st_size / (1024 * 1024))
            return dest_gz

        LOGGER.info("Downloading full Azure VM dataset (~418 MB compressed, 2.69M+ rows)...")
        urllib.request.urlretrieve(AZURE_URL, dest_gz)
        LOGGER.info("Successfully downloaded full Azure VM dataset to: %s", dest_gz)
        return dest_gz

    # 3. Stream sample if sample_only is requested
    LOGGER.info("Streaming %d sample records from Azure releases...", max_sample_rows)
    req = urllib.request.Request(AZURE_URL, headers={"User-Agent": "Mozilla/5.0"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        decompressor = gzip.GzipFile(fileobj=resp)
        text_wrapper = io.TextIOWrapper(decompressor, encoding="utf-8", errors="ignore")

        headers = [
            "vmid", "subscriptionid", "deploymentid", "vmcreated", "vmdeleted",
            "maxcpu", "avgcpu", "p95maxcpu", "vmcategory", "vmcorecountbucket", "vmmemorybucket"
        ]

        with open(dest_sample, "w", newline="", encoding="utf-8") as f:
            writer = csv.writer(f)
            writer.writerow(headers)
            count = 0
            for line in text_wrapper:
                row = line.strip().split(",")
                if len(row) >= 11:
                    writer.writerow(row[:11])
                    count += 1
                    if count >= max_sample_rows:
                        break

    LOGGER.info("Staged %d sample records to %s", count, dest_sample)
    return dest_sample


def main() -> None:
    parser = argparse.ArgumentParser(description="Download and stage Microsoft Azure Public Dataset")
    parser.add_argument("--sample-only", action="store_true", help="Download only a lightweight sample")
    parser.add_argument("--max-sample-rows", type=int, default=5000, help="Number of rows for sample staging")
    args = parser.parse_args()

    staged_path = download_and_stage_azure(sample_only=args.sample_only, max_sample_rows=args.max_sample_rows)
    print(f"\n✅ Azure dataset successfully staged at: {staged_path}")
    print(f"👉 Use with schema mapping: testbench/configs/azure_mapping.yaml")
    print(f"👉 Run benchmark: python3 testbench/runner.py --dataset {staged_path} --mapping testbench/configs/azure_mapping.yaml --profile all\n")


if __name__ == "__main__":
    main()
