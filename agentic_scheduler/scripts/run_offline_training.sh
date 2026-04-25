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
# Output is saved to the logs folder within agentic_scheduler
python -m agentic_scheduler.train_ppo \
  --trace-source alibaba \
  --trace-path agentic_scheduler/data/alibaba_v2018/raw \
  --max-trace-tasks 10000000 \
  --rollout-steps 16384 \
  --minibatch-size 4096 \
  --ppo-epochs 15 \
  --updates 1000 \
  --checkpoint-every 50 \
  --output "$RESULTS_DIR/ppo_trained_final.pt" \
  --checkpoint-dir "$CHECKPOINT_DIR" \
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
