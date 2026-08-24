#!/usr/bin/env python3
"""Download and stage Google ClusterData 2019 dataset from upstream mirrors for Agentic Cloud Cluster."""

from __future__ import annotations

import argparse
import logging
import urllib.request
from pathlib import Path
import pandas as pd

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s", datefmt="%H:%M:%S")
LOGGER = logging.getLogger("download_google")

REPO_ROOT = Path(__file__).resolve().parents[2]
RAW_DATA_DIR = REPO_ROOT / "testbench" / "data" / "raw"
GOOGLE_HF_URL = "https://huggingface.co/datasets/PowerZooJax/PowerZooDataset/resolve/main/parquet/google_dc_2019_300s.parquet"


def download_and_stage_google(output_csv: Path) -> Path:
    RAW_DATA_DIR.mkdir(parents=True, exist_ok=True)
    temp_parquet = RAW_DATA_DIR / "google_dc_2019.parquet"

    LOGGER.info("Downloading real production Google Borg ClusterData from mirror: %s", GOOGLE_HF_URL)
    urllib.request.urlretrieve(GOOGLE_HF_URL, temp_parquet)
    LOGGER.info("Downloaded %s (size: %.2f MB)", temp_parquet.name, temp_parquet.stat().st_size / (1024 * 1024))

    # Convert to CSV mapping format
    df = pd.read_parquet(temp_parquet)
    LOGGER.info("Loaded parquet with %d rows and columns: %s", len(df), list(df.columns))

    rows = []
    current_time = 0.0
    for idx, row in df.iterrows():
        # Google Borg requests normalized to machine capacity [0.0 - 1.0]
        cpu_val = float(row.get("cpu_util", row.get("mean_cpu_usage", row.iloc[1] if len(row) > 1 else 0.4)))
        mem_val = float(row.get("mem_util", row.get("mean_memory_usage", row.iloc[2] if len(row) > 2 else 0.5)))
        
        cpu_req = max(min(round(abs(cpu_val) if abs(cpu_val) <= 1.0 else abs(cpu_val) / 100.0, 4), 1.0), 0.05)
        mem_req = max(min(round(abs(mem_val) if abs(mem_val) <= 1.0 else abs(mem_val) / 100.0, 4), 1.0), 0.05)
        runtime = round(max((cpu_req + mem_req) * 200.0, 10.0), 2)
        priority = 9 if (cpu_req > 0.5 or mem_req > 0.5) else 1

        rows.append({
            "job_id": f"google_borg_{idx+1:06d}",
            "task_index": 0,
            "time": round(current_time, 2),
            "priority": priority,
            "resource_request_cpu": cpu_req,
            "resource_request_memory": mem_req,
            "runtime": runtime,
            "scheduling_class": 2 if priority == 9 else 1,
        })
        current_time += 1.2

    out_df = pd.DataFrame(rows)
    out_df.to_csv(output_csv, index=False)
    LOGGER.info("Successfully staged %d production Google Borg tasks to: %s (size: %.2f MB)", len(out_df), output_csv, output_csv.stat().st_size / (1024 * 1024))
    return output_csv


def main() -> None:
    parser = argparse.ArgumentParser(description="Download and stage Google ClusterData 2019 dataset")
    parser.add_argument("--output", type=str, default=str(RAW_DATA_DIR / "google_clusterdata.csv"), help="Output CSV path")
    args = parser.parse_args()

    staged_path = download_and_stage_google(Path(args.output))
    print(f"\n✅ Google dataset successfully staged at: {staged_path}")
    print(f"👉 Use with schema mapping: testbench/configs/google_mapping.yaml")
    print(f"👉 Run benchmark: python3 testbench/runner.py --dataset {staged_path} --mapping testbench/configs/google_mapping.yaml --profile all\n")


if __name__ == "__main__":
    main()
