#!/usr/bin/env python3


"""
Plot the training reward curve from a training_output.log file.

Usage:
    python plot_training_curve.py <log_file> <output_image_path>

Reads update lines from the log and produces a reward-vs-update PNG chart.
"""

import re
import sys
from pathlib import Path

import matplotlib
matplotlib.use("Agg")  # headless backend
import matplotlib.pyplot as plt
import numpy as np


def parse_updates(log_path: str) -> list[dict]:
    """Extract update records from the training log."""
    updates = []
    with open(log_path, "r") as f:
        for line in f:
            if "update=" not in line or "avg_reward=" not in line:
                continue
            # Try the new records_processed format first
            m = re.search(
                r"update=(\d+)\s+avg_reward=([-\d.]+)\s+records_processed=(\d+)/(\d+)",
                line,
            )
            if m:
                updates.append({
                    "update": int(m.group(1)),
                    "avg_reward": float(m.group(2)),
                    "records": int(m.group(3)),
                    "total": int(m.group(4)),
                })
                continue
            # Fall back to the old model_steps format
            m = re.search(
                r"update=(\d+)\s+avg_reward=([-\d.]+)\s+model_steps=(\d+)",
                line,
            )
            if m:
                updates.append({
                    "update": int(m.group(1)),
                    "avg_reward": float(m.group(2)),
                    "records": 0,
                    "total": 0,
                })
    return updates


def plot_curve(updates: list[dict], output_path: str) -> None:
    """Generate a styled reward curve and save as PNG."""
    if not updates:
        print("No update data found in log — nothing to plot.")
        sys.exit(1)

    xs = np.array([u["update"] for u in updates])
    ys = np.array([u["avg_reward"] for u in updates])

    # Compute a smoothed trend line (simple moving average, window=5 or less)
    window = min(5, len(ys))
    if window >= 2:
        kernel = np.ones(window) / window
        ys_smooth = np.convolve(ys, kernel, mode="valid")
        xs_smooth = xs[window - 1:]
    else:
        ys_smooth = ys
        xs_smooth = xs

    fig, ax = plt.subplots(figsize=(12, 6))

    # Style
    fig.patch.set_facecolor("#1a1a2e")
    ax.set_facecolor("#16213e")

    # Raw reward points
    ax.plot(xs, ys, color="#4cc9f0", alpha=0.4, linewidth=1, marker="o",
            markersize=4, label="Raw avg_reward")

    # Smoothed trend
    ax.plot(xs_smooth, ys_smooth, color="#f72585", linewidth=2.5,
            label=f"Smoothed (window={window})")

    # Zero line for reference
    ax.axhline(y=0, color="#adb5bd", linewidth=0.8, linestyle="--", alpha=0.5)

    # Labels & title
    ax.set_xlabel("PPO Update", fontsize=13, color="#e0e0e0")
    ax.set_ylabel("Average Reward", fontsize=13, color="#e0e0e0")
    ax.set_title("PPO Training Reward Curve — Alibaba Cluster Trace v2018",
                  fontsize=15, color="#e0e0e0", fontweight="bold")
    ax.legend(loc="lower right", fontsize=11, facecolor="#16213e",
              edgecolor="#4cc9f0", labelcolor="#e0e0e0")

    # Ticks
    ax.tick_params(colors="#adb5bd")
    for spine in ax.spines.values():
        spine.set_color("#4cc9f0")
        spine.set_linewidth(0.5)

    # Annotate start and end
    ax.annotate(f"Start: {ys[0]:.4f}", xy=(xs[0], ys[0]),
                fontsize=10, color="#4cc9f0",
                xytext=(xs[0] + (xs[-1] - xs[0]) * 0.05, ys[0] + 0.05),
                arrowprops=dict(arrowstyle="->", color="#4cc9f0"))
    ax.annotate(f"End: {ys[-1]:.4f}", xy=(xs[-1], ys[-1]),
                fontsize=10, color="#f72585",
                xytext=(xs[-1] - (xs[-1] - xs[0]) * 0.2, ys[-1] + 0.08),
                arrowprops=dict(arrowstyle="->", color="#f72585"))

    plt.tight_layout()
    plt.savefig(output_path, dpi=150, facecolor=fig.get_facecolor())
    plt.close()
    print(f"Training curve saved to {output_path}")


def main():
    if len(sys.argv) < 3:
        print("Usage: python plot_training_curve.py <log_file> <output_image>")
        sys.exit(1)

    log_file = sys.argv[1]
    out_file = sys.argv[2]

    if not Path(log_file).exists():
        print(f"Log file not found: {log_file}")
        sys.exit(1)

    updates = parse_updates(log_file)
    plot_curve(updates, out_file)


if __name__ == "__main__":
    main()
