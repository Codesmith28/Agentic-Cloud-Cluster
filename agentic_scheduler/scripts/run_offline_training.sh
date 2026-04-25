#!/bin/bash

# Get the directory where the script is located (agentic_scheduler/scripts)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
# The agentic_scheduler directory is one level up
SCHEDULER_DIR="$(dirname "$SCRIPT_DIR")"
# The project root is two levels up
PROJECT_ROOT="$(dirname "$SCHEDULER_DIR")"

# Navigate to project root to correctly resolve the python module
cd "$PROJECT_ROOT" || exit 1

# Create directories for isolation within agentic_scheduler
LOG_DIR="${SCHEDULER_DIR}/logs"
RESULTS_DIR="${SCHEDULER_DIR}/results"
CHECKPOINT_DIR="${RESULTS_DIR}/checkpoints"

mkdir -p "$LOG_DIR"
mkdir -p "$RESULTS_DIR"
mkdir -p "$CHECKPOINT_DIR"

LOG_FILE="${LOG_DIR}/training_output.log"
REPORT_FILE="${RESULTS_DIR}/training_report.md"
REPORT_SCRIPT="${SCRIPT_DIR}/generate_report.py"

echo "======================================================="
echo "Starting Offline PPO Training on Alibaba Trace Data"
echo "======================================================="
echo "Logs directory:     $LOG_DIR"
echo "Results directory:  $RESULTS_DIR"
echo "Checkpoints:        $CHECKPOINT_DIR"
echo "======================================================="

# Run the training command
# Uses the curated train/ split (200K tasks with headers) instead of raw/ (14.3M rows, no header).
# The test/ split (50K tasks) is held out for post-training evaluation.
python -m agentic_scheduler.train_ppo \
  --trace-source alibaba \
  --trace-path agentic_scheduler/data/alibaba_v2018/train \
  --max-trace-tasks 200000 \
  --rollout-steps 16384 \
  --minibatch-size 4096 \
  --ppo-epochs 15 \
  --updates 1000 \
  --checkpoint-every 50 \
  --output "$RESULTS_DIR/ppo_trained_final.pt" \
  --checkpoint-dir "$CHECKPOINT_DIR" \
  --num-workers 64 \
  --log-every 1 \
  2>&1 | tee "$LOG_FILE"

echo ""
echo "Training finished!"

# Generate the final report once the training is done
if [ -f "$REPORT_SCRIPT" ]; then
    echo "Generating final training report..."
    python "$REPORT_SCRIPT" "$LOG_FILE" "$REPORT_FILE"
    echo "A compiled training report has been generated at: $REPORT_FILE"
else
    echo "Warning: Report script not found at $REPORT_SCRIPT"
fi

# Generate the training reward curve plot
PLOT_SCRIPT="${SCRIPT_DIR}/plot_training_curve.py"
PLOT_FILE="${RESULTS_DIR}/training_reward_curve.png"
if [ -f "$PLOT_SCRIPT" ]; then
    echo "Generating training reward curve..."
    python "$PLOT_SCRIPT" "$LOG_FILE" "$PLOT_FILE"
else
    echo "Warning: Plot script not found at $PLOT_SCRIPT"
fi
