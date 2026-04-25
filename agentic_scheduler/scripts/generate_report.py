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
            match = re.search(r'Loaded (\d+) machines', line)
            if match:
                machines_loaded = int(match.group(1))
        elif "Loaded" in line and "tasks" in line and "capped at" in line:
            match = re.search(r'Loaded (\d+) .*tasks', line)
            if match:
                tasks_loaded = int(match.group(1))
        elif "update=" in line and "avg_reward=" in line:
            match = re.search(r'update=(\d+) avg_reward=([-\d.]+) records_processed=(\d+)/(\d+) \(([\d.]+)%\)', line)
            if match:
                updates.append({
                    'update': int(match.group(1)),
                    'avg_reward': float(match.group(2)),
                    'records': int(match.group(3)),
                    'total': int(match.group(4)),
                    'percent': float(match.group(5))
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
        md += "| Update | Avg Reward | Records Processed |\n"
        md += "|---|---|---|\n"
        for u in updates:
            md += f"| {u['update']} | {u['avg_reward']:.4f} | {u['records']}/{u['total']} ({u['percent']}%) |\n"
            
        md += "\n### Summary\n"
        md += f"The model has completed **{updates[-1]['update']}** PPO updates.\n"
        md += f"Current average reward: **{updates[-1]['avg_reward']:.4f}**\n"
        
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
