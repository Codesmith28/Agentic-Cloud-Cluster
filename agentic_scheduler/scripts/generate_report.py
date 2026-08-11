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

import re
import sys
from pathlib import Path

def parse_log(log_path):
    with open(log_path, 'r') as f:
        lines = f.readlines()

    gpu_info = "Unknown"
    machines_loaded = 0
    tasks_loaded = 0
    updates = []

    for line in lines:
        if "Using GPU for offline PPO training:" in line:
            gpu_info = line.split("Using GPU for offline PPO training:")[1].strip()
        elif "Loaded" in line and "machines from" in line:
            match = re.search(r'Loaded (\d+)', line)
            if match:
                machines_loaded = int(match.group(1))
        elif "Loaded" in line and "tasks" in line and "capped at" in line:
            match = re.search(r'Loaded (\d+) .*tasks', line)
            if match:
                tasks_loaded = int(match.group(1))
        elif "update=" in line and "avg_reward=" in line:
            # New format: update=N avg_reward=X.XXXX steps=S epoch=E.EE
            match = re.search(
                r'update=(\d+)\s+avg_reward=([-\d.]+)\s+steps=(\d+)\s+epoch=([\d.]+)',
                line,
            )
            if match:
                updates.append({
                    'update': int(match.group(1)),
                    'avg_reward': float(match.group(2)),
                    'steps': int(match.group(3)),
                    'epoch': float(match.group(4)),
                })
                continue
            # New format without epoch (synthetic env)
            match = re.search(
                r'update=(\d+)\s+avg_reward=([-\d.]+)\s+steps=(\d+)',
                line,
            )
            if match:
                updates.append({
                    'update': int(match.group(1)),
                    'avg_reward': float(match.group(2)),
                    'steps': int(match.group(3)),
                    'epoch': 0.0,
                })
                continue
            # Legacy format: update=N avg_reward=X records_processed=R/T (P%)
            match = re.search(
                r'update=(\d+) avg_reward=([-\d.]+) records_processed=(\d+)/(\d+) \(([\d.]+)%\)',
                line,
            )
            if match:
                updates.append({
                    'update': int(match.group(1)),
                    'avg_reward': float(match.group(2)),
                    'steps': int(match.group(3)),
                    'epoch': float(match.group(3)) / max(float(match.group(4)), 1),
                })

    return gpu_info, machines_loaded, tasks_loaded, updates

def generate_markdown(gpu_info, machines_loaded, tasks_loaded, updates, output_path):
    md = f"# Agentic Scheduler - Training Report\n\n"
    md += "## Hardware & Dataset Setup\n"
    md += f"- **Accelerator**: {gpu_info}\n"
    md += f"- **Cluster Size**: {machines_loaded} machines\n"
    md += f"- **Workload**: {tasks_loaded} tasks (Alibaba v2018 Trace)\n\n"

    md += "## Training Progress\n"
    if not updates:
        md += "Training is still initializing (loading dataset) or no updates have been logged yet.\n"
    else:
        md += "| Update | Avg Reward | Steps | Epoch |\n"
        md += "|---|---|---|---|\n"
        for u in updates:
            epoch_str = f"{u['epoch']:.2f}" if u['epoch'] > 0 else "—"
            md += f"| {u['update']} | {u['avg_reward']:.4f} | {u['steps']} | {epoch_str} |\n"

        md += "\n### Summary\n"
        rewards = [u['avg_reward'] for u in updates]
        md += f"The model has completed **{updates[-1]['update']}** PPO updates.\n"
        md += f"Final average reward: **{updates[-1]['avg_reward']:.4f}**\n"
        md += f"Best average reward:  **{max(rewards):.4f}** (update {updates[rewards.index(max(rewards))]['update']})\n"
        md += f"Worst average reward: **{min(rewards):.4f}** (update {updates[rewards.index(min(rewards))]['update']})\n"

    with open(output_path, 'w') as f:
        f.write(md)
    print(f"Report compiled successfully: {output_path}")

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python generate_report.py <log_file> <output_file>")
        sys.exit(1)

    log_file = sys.argv[1]
    out_file = sys.argv[2]

    if not Path(log_file).exists():
        print(f"Log file {log_file} not found. Has training started?")
        sys.exit(1)

    gpu_info, m_loaded, t_loaded, updates = parse_log(log_file)
    generate_markdown(gpu_info, m_loaded, t_loaded, updates, out_file)
