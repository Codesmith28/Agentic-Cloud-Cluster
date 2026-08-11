#!/usr/bin/env python3

# Copyright 2025-2026 Sarthak Siddhpura
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Generate a comparison report from benchmark results."""

import argparse
import json
import sys
from pathlib import Path
from collections import defaultdict
from statistics import mean, stdev


def load_results(results_dir: Path) -> dict:
    """Load all result files from directory."""
    results = defaultdict(lambda: defaultdict(list))
    
    for file in results_dir.glob("*.json"):
        try:
            with open(file) as f:
                data = json.load(f)
                # Parse filename: scheduler-workload-scenario.json
                parts = file.stem.split("-")
                if len(parts) >= 3:
                    scheduler = parts[0]
                    scenario = parts[-1]
                    workload = "-".join(parts[1:-1])
                    
                    if isinstance(data, dict) and "results" in data:
                        for result in data.get("results", []):
                            results[scheduler][(workload, scenario)].append(result)
        except Exception as e:
            print(f"Warning: Could not load {file}: {e}", file=sys.stderr)
    
    return results


def compute_stats(results_list) -> dict:
    """Compute statistics from result list."""
    if not results_list:
        return {}
    
    success_rates = [r.get("success_rate", 0) for r in results_list]
    avg_waits = [r.get("avg_wait_seconds", 0) for r in results_list]
    avg_turnarounds = [r.get("avg_turnaround_seconds", 0) for r in results_list]
    
    return {
        "success_rate_mean": mean(success_rates) if success_rates else 0,
        "success_rate_stdev": stdev(success_rates) if len(success_rates) > 1 else 0,
        "avg_wait_mean": mean(avg_waits) if avg_waits else 0,
        "avg_turnaround_mean": mean(avg_turnarounds) if avg_turnarounds else 0,
        "count": len(results_list),
    }


def generate_report(results_dir: Path, output_file: Path):
    """Generate comparison markdown report."""
    results = load_results(results_dir)
    
    if not results:
        print("No results found", file=sys.stderr)
        return
    
    # Compute aggregate stats per scheduler
    scheduler_stats = {}
    for scheduler, data in results.items():
        all_results = []
        for result_list in data.values():
            all_results.extend(result_list)
        scheduler_stats[scheduler] = compute_stats(all_results)
    
    # Generate markdown
    with open(output_file, "w") as f:
        f.write("# Alibaba Test Benchmark Report\n\n")
        
        f.write("## Summary Statistics\n\n")
        f.write("| Scheduler | Success Rate | Avg Turnaround (s) | Avg Wait (s) | Runs |\n")
        f.write("|-----------|--------------|--------------------|--------------|-----------|\n")
        
        for scheduler in sorted(scheduler_stats.keys()):
            stats = scheduler_stats[scheduler]
            f.write(
                f"| {scheduler} | "
                f"{stats.get('success_rate_mean', 0):.1%} | "
                f"{stats.get('avg_turnaround_mean', 0):.2f} | "
                f"{stats.get('avg_wait_mean', 0):.2f} | "
                f"{stats.get('count', 0)} |\n"
            )
        
        f.write("\n## Detailed Results by Workload\n\n")
        
        for scheduler in sorted(results.keys()):
            f.write(f"### {scheduler}\n\n")
            f.write("| Workload | Scenario | Success Rate | Turnaround (s) |\n")
            f.write("|----------|----------|--------------|----------------|\n")
            
            for (workload, scenario), result_list in sorted(results[scheduler].items()):
                if result_list:
                    latest = result_list[0]  # Take first/latest result
                    f.write(
                        f"| {workload} | {scenario} | "
                        f"{latest.get('success_rate', 0):.1%} | "
                        f"{latest.get('avg_turnaround_seconds', 0):.2f} |\n"
                    )
            
            f.write("\n")
        
        f.write("## Scheduler Comparison\n\n")
        
        # Find best and worst for each metric
        metrics = ["success_rate_mean", "avg_turnaround_mean"]
        for metric in metrics:
            f.write(f"**Best {metric}:**\n\n")
            best_scheduler = max(scheduler_stats.items(), 
                                key=lambda x: x[1].get(metric, 0),
                                default=(None, {}))
            if best_scheduler[0]:
                f.write(f"- {best_scheduler[0]}: {best_scheduler[1].get(metric, 0):.2f}\n")
            f.write("\n")


def main():
    parser = argparse.ArgumentParser(description="Generate comparison report")
    parser.add_argument("--results-dir", type=Path, required=True, help="Results directory")
    parser.add_argument("--output", type=Path, required=True, help="Output markdown file")
    args = parser.parse_args()
    
    args.results_dir.mkdir(parents=True, exist_ok=True)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    
    generate_report(args.results_dir, args.output)
    print(f"Report generated: {args.output}")


if __name__ == "__main__":
    main()
