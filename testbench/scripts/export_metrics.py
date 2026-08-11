#!/usr/bin/env python3
"""Export Prometheus-backed observability artifacts for a CloudAI testbench run."""

from __future__ import annotations

import argparse
import csv
import json
import pathlib
import time
import urllib.parse
import urllib.request
from typing import Any


RANGE_QUERIES = {
    "master_queue_depth": "cloudai_master_queue_depth",
    "master_dequeue_rate": "sum by (outcome) (rate(cloudai_master_tasks_dequeued_total[1m]))",
    "worker_running_tasks": "cloudai_worker_running_tasks",
    "worker_cpu_usage_ratio": 'cloudai_worker_resource_usage_ratio{resource="cpu"}',
    "worker_memory_usage_ratio": 'cloudai_worker_resource_usage_ratio{resource="memory"}',
    "worker_storage_usage_ratio": 'cloudai_worker_resource_usage_ratio{resource="storage"}',
    "worker_container_cpu_seconds_rate": "sum by (task_type) (rate(cloudai_worker_container_cpu_seconds_total[1m]))",
    "worker_container_io_bytes_rate": "sum by (task_type) (rate(cloudai_worker_container_io_bytes_total[1m]))",
    "worker_container_memory_peak_p95": (
        "histogram_quantile(0.95, sum by (le, task_type) (rate(cloudai_worker_container_memory_peak_bytes_bucket[5m])))"
    ),
}


def request_json(url: str, timeout: float = 15.0) -> dict[str, Any]:
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
        return json.loads(body) if body else {}


def prom_query(prometheus_url: str, query: str, end_ts: float) -> dict[str, Any]:
    params = urllib.parse.urlencode({"query": query, "time": f"{end_ts:.3f}"})
    return request_json(f"{prometheus_url.rstrip('/')}/api/v1/query?{params}")


def prom_query_range(prometheus_url: str, query: str, start_ts: float, end_ts: float, step_seconds: int) -> dict[str, Any]:
    params = urllib.parse.urlencode(
        {
            "query": query,
            "start": f"{start_ts:.3f}",
            "end": f"{end_ts:.3f}",
            "step": str(step_seconds),
        }
    )
    return request_json(f"{prometheus_url.rstrip('/')}/api/v1/query_range?{params}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Export Prometheus metrics for a testbench run")
    parser.add_argument("--prometheus-url", default="http://localhost:9090", help="Prometheus base URL")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master API base URL")
    parser.add_argument("--summary", required=True, type=pathlib.Path, help="Path to workload summary JSON")
    parser.add_argument("--output-dir", required=True, type=pathlib.Path, help="Directory for observability artifacts")
    parser.add_argument("--step-seconds", type=int, default=5, help="Prometheus query_range step size")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    summary = json.loads(args.summary.read_text(encoding="utf-8"))

    started_at = float(summary["started_at_unix"])
    finished_at = float(summary["finished_at_unix"])
    start_ts = max(0.0, started_at - 5.0)
    end_ts = finished_at + 5.0
    window_seconds = max(1, int(end_ts - start_ts))

    output_dir = args.output_dir.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)

    # Check Prometheus connectivity before querying
    try:
        request_json(f"{args.prometheus_url.rstrip('/')}/api/v1/status/config", timeout=5.0)
    except Exception:
        print(f"Warning: Prometheus not reachable at {args.prometheus_url}, skipping metrics export")
        return 0

    range_results: dict[str, Any] = {}
    for name, query in RANGE_QUERIES.items():
        range_results[name] = prom_query_range(args.prometheus_url, query, start_ts, end_ts, args.step_seconds)

    instant_queries = {
        "max_queue_depth": f"max_over_time(cloudai_master_queue_depth[{window_seconds}s])",
        "task_requeues": f"sum by (failure_reason, task_type) (increase(cloudai_master_task_requeues_total[{window_seconds}s]))",
        "task_dequeues": f"sum by (outcome) (increase(cloudai_master_tasks_dequeued_total[{window_seconds}s]))",
        "stale_results": f"sum by (reason) (increase(cloudai_master_stale_results_total[{window_seconds}s]))",
        "worker_timeouts": f"sum by (worker_id) (increase(cloudai_master_worker_timeouts_total[{window_seconds}s]))",
        "task_terminals": f"sum by (status, task_type) (increase(cloudai_master_task_terminal_total[{window_seconds}s]))",
        "worker_container_cpu_seconds": (
            f"sum by (task_type) (increase(cloudai_worker_container_cpu_seconds_total[{window_seconds}s]))"
        ),
        "worker_container_io_bytes": (
            f"sum by (task_type) (increase(cloudai_worker_container_io_bytes_total[{window_seconds}s]))"
        ),
        "worker_container_memory_peak_p95": (
            f'histogram_quantile(0.95, sum by (le, task_type) (increase(cloudai_worker_container_memory_peak_bytes_bucket[{window_seconds}s])))'
        ),
        "p95_scheduling_latency": (
            f'histogram_quantile(0.95, sum by (le, scheduler) (increase(cloudai_master_scheduling_latency_seconds_bucket[{window_seconds}s])))'
        ),
        "p95_queue_wait": (
            f'histogram_quantile(0.95, sum by (le, scheduler, task_type) (increase(cloudai_master_task_queue_wait_seconds_bucket[{window_seconds}s])))'
        ),
    }

    instant_results = {
        name: prom_query(args.prometheus_url, query, end_ts) for name, query in instant_queries.items()
    }

    master_snapshot = {
        "workers": request_json(f"{args.master_url.rstrip('/')}/api/workers"),
        "tasks": request_json(f"{args.master_url.rstrip('/')}/api/tasks"),
    }

    (output_dir / "prometheus-range.json").write_text(json.dumps(range_results, indent=2), encoding="utf-8")
    (output_dir / "prometheus-instant.json").write_text(json.dumps(instant_results, indent=2), encoding="utf-8")
    (output_dir / "master-snapshot.json").write_text(json.dumps(master_snapshot, indent=2), encoding="utf-8")

    with (output_dir / "metrics-summary.csv").open("w", newline="", encoding="utf-8") as csv_file:
        writer = csv.writer(csv_file)
        writer.writerow(["metric", "labels", "value"])
        for metric_name, payload in instant_results.items():
            for series in payload.get("data", {}).get("result", []):
                labels = json.dumps(series.get("metric", {}), sort_keys=True)
                value = ""
                if "value" in series and len(series["value"]) >= 2:
                    value = series["value"][1]
                writer.writerow([metric_name, labels, value])

    print(f"Observability artifacts written to {output_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
