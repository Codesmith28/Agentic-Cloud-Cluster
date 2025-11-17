# GPU Detection Implementation

## Problem

Workers were always reporting `TotalGPU = 0.0` even when NVIDIA GPUs were available on the system. This was because:

1. GPU count detection was not implemented in `system.go`
2. The code had a hardcoded value: `TotalGPU: 0.0` with a comment saying GPU detection requires additional libraries
3. While GPU **usage** was being reported in heartbeats (via `nvidia-smi`), GPU **capacity** was never detected during registration

## Solution

Implemented GPU detection using `nvidia-smi` to count the number of NVIDIA GPUs available on the worker system.

## What Was Changed

### 1. `worker/internal/system/system.go`

#### Added Imports
```go
import (
    // ... existing imports ...
    "bytes"      // For command output buffering
    "os/exec"    // For running nvidia-smi
)
```

#### Updated `GetSystemResources()`
```go
// Before: Hardcoded to 0.0
resources := &ResourceInfo{
    TotalCPU: float64(runtime.NumCPU()),
    TotalGPU: 0.0, // GPU detection requires additional libraries
}

// After: Detects GPU count
resources := &ResourceInfo{
    TotalCPU: float64(runtime.NumCPU()),
}

// Detect GPU count
gpuCount, err := detectGPUCount()
if err != nil {
    log.Printf("Info: No GPU detected or nvidia-smi not available: %v", err)
    resources.TotalGPU = 0.0
} else {
    resources.TotalGPU = gpuCount
    log.Printf("Detected %d GPU(s)", int(gpuCount))
}
```

#### Added New Function `detectGPUCount()`
```go
// detectGPUCount detects the number of NVIDIA GPUs using nvidia-smi
func detectGPUCount() (float64, error) {
    cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
    var out bytes.Buffer
    cmd.Stdout = &out

    if err := cmd.Run(); err != nil {
        return 0.0, fmt.Errorf("nvidia-smi not available: %w", err)
    }

    output := strings.TrimSpace(out.String())
    if output == "" {
        return 0.0, fmt.Errorf("no GPUs found")
    }

    // Count lines (each line = one GPU)
    lines := strings.Split(output, "\n")
    gpuCount := len(lines)

    return float64(gpuCount), nil
}
```

## How It Works

### Detection Flow

```
Worker Startup
     │
     ↓
GetSystemResources()
     │
     ├─→ Detect CPU: runtime.NumCPU()
     ├─→ Detect Memory: /proc/meminfo
     ├─→ Detect Storage: syscall.Statfs
     │
     └─→ Detect GPU: detectGPUCount()
              │
              ↓
         Run: nvidia-smi --query-gpu=name --format=csv,noheader
              │
              ↓
         Output: "NVIDIA GeForce RTX 4060 Laptop GPU"
              │
              ↓
         Count lines: 1 GPU
              │
              ↓
         Return: 1.0
```

### nvidia-smi Command

```bash
# Command used
nvidia-smi --query-gpu=name --format=csv,noheader

# Example output with 1 GPU:
NVIDIA GeForce RTX 4060 Laptop GPU

# Example output with 2 GPUs:
NVIDIA GeForce RTX 4060 Laptop GPU
NVIDIA GeForce RTX 3080

# Example output with no GPUs or nvidia-smi not installed:
(command fails, returns error)
```

### Worker Registration with GPU

**Before:**
```
Worker → Master: RegisterWorker()
{
    worker_id: "worker-1"
    total_cpu: 8.0
    total_memory: 16.0
    total_storage: 500.0
    total_gpu: 0.0          ← Always zero!
}
```

**After:**
```
Worker → Master: RegisterWorker()
{
    worker_id: "worker-1"
    total_cpu: 8.0
    total_memory: 16.0
    total_storage: 500.0
    total_gpu: 1.0          ← Correctly detected!
}
```

## GPU Information Reported

### During Registration
- **TotalGPU**: Number of GPUs (now detected correctly)
- Reported once when worker registers with master
- Used for task scheduling decisions

### During Heartbeats
- **GPUUsage**: Current GPU utilization percentage (0-100%)
- Already implemented, continues to work
- Reported every 5 seconds in heartbeats

## Testing

### Manual Test with nvidia-smi

```bash
# Check GPU count
nvidia-smi --query-gpu=name --format=csv,noheader

# Output (your system):
NVIDIA GeForce RTX 4060 Laptop GPU

# Expected: Worker will report TotalGPU = 1.0
```

### Automated Test

Run the test script:
```bash
./test_gpu_detection.sh
```

**Expected Output:**
```
═══════════════════════════════════════════
  GPU Detection Test
═══════════════════════════════════════════

✓ nvidia-smi found

GPU Information from nvidia-smi:
─────────────────────────────────────────
index, name, memory.total [MiB]
0, NVIDIA GeForce RTX 4060 Laptop GPU, 8188 MiB
─────────────────────────────────────────

Total GPUs detected: 1

Testing worker resource detection...

Worker will report to master:
  GPU:     1

═══════════════════════════════════════════
Test complete!
═══════════════════════════════════════════
```

### Verify in Master CLI

```bash
# Start worker
cd worker && ./workerNode

# In master CLI
master> workers

# Expected output:
╔═══ Registered Workers 
║ worker-1
║   Status: 🟢 Active
║   IP: localhost:50052
║   Resources: CPU=8.0, Memory=16.0GB, GPU=1.0  ← GPU count now shown!
║   Running Tasks: 0
╚═══════════════════════
```

## Error Handling

### GPU Not Available

**Scenario**: System has no NVIDIA GPU or nvidia-smi is not installed

**Behavior**:
```
Info: No GPU detected or nvidia-smi not available: nvidia-smi not available: ...
Worker will report: TotalGPU = 0.0
```

**Impact**: Worker can still function normally, just without GPU support

### nvidia-smi Fails

**Possible reasons**:
1. NVIDIA drivers not installed
2. nvidia-smi not in PATH
3. Permission issues
4. GPU hardware failure

**Behavior**: Falls back to `TotalGPU = 0.0` gracefully

### Multiple GPUs

**System with 2+ GPUs**:
```bash
nvidia-smi --query-gpu=name --format=csv,noheader

# Output:
NVIDIA GeForce RTX 4090
NVIDIA GeForce RTX 3080

# Detection result: TotalGPU = 2.0
```

## Compatibility

### Supported GPU Types
- ✅ NVIDIA GPUs (via nvidia-smi)
- ❌ AMD GPUs (not currently supported)
- ❌ Intel GPUs (not currently supported)

### Requirements
- NVIDIA drivers installed
- nvidia-smi available in PATH
- Linux/Unix system

### Fallback Behavior
If nvidia-smi is not available:
- Worker reports 0 GPUs
- System continues to work normally
- No crashes or errors
- Tasks requiring GPU won't be assigned to this worker

## Scheduler Integration

### Task Scheduling with GPU

**Master's scheduling decision now considers GPU:**

```
Task requires: GPU = 1.0

Available workers:
  worker-1: GPU = 0.0  ← Not eligible
  worker-2: GPU = 1.0  ← Eligible ✓
  worker-3: GPU = 2.0  ← Eligible ✓

Master assigns task to worker-2 or worker-3
```

### GPU Resource Tracking

**Before task assignment:**
```
Worker: total_gpu=1.0, allocated_gpu=0.0, available_gpu=1.0
```

**After task assignment (task requires 1 GPU):**
```
Worker: total_gpu=1.0, allocated_gpu=1.0, available_gpu=0.0
```

**After task completion:**
```
Worker: total_gpu=1.0, allocated_gpu=0.0, available_gpu=1.0
```

## Logs

### Worker Startup with GPU

```
=== Worker System Information ===
Hostname: laptop
IP Addresses: [192.168.1.100]
OS: linux
Architecture: amd64
CPU Cores: 8
...
===============================

Detected System Resources:
  CPU:     8.00 cores
  Memory:  15.50 GB
  Storage: 476.94 GB
  GPU:     1.00 cores          ← GPU detected!

Detected 1 GPU(s)                ← Confirmation log
```

### Worker Startup without GPU

```
=== Worker System Information ===
...
===============================

Detected System Resources:
  CPU:     8.00 cores
  Memory:  15.50 GB
  Storage: 476.94 GB
  GPU:     0.00 cores          ← No GPU

Info: No GPU detected or nvidia-smi not available: nvidia-smi not available: exec: "nvidia-smi": executable file not found in $PATH
```

## Troubleshooting

### GPU Not Detected (But You Have One)

**Check nvidia-smi:**
```bash
nvidia-smi
```

**If not found:**
```bash
# Install NVIDIA drivers (Ubuntu/Debian)
sudo apt install nvidia-driver-XXX

# Or check PATH
which nvidia-smi
```

### GPU Detected but Wrong Count

**Verify actual GPU count:**
```bash
nvidia-smi --list-gpus
```

**Check detection:**
```bash
nvidia-smi --query-gpu=name --format=csv,noheader | wc -l
```

### Permissions Issues

**Error**: `nvidia-smi: command not found` (but it exists)

**Solution**: Add to PATH or run with full path
```bash
export PATH=$PATH:/usr/bin
```

## Future Enhancements

Possible improvements:
1. **AMD GPU support** - Use rocm-smi for AMD GPUs
2. **Intel GPU support** - Use intel_gpu_top
3. **GPU memory detection** - Report total GPU memory
4. **Multiple GPU types** - Support mixed NVIDIA/AMD systems
5. **GPU capabilities** - Report compute capability, CUDA version

## Files Modified

1. **`worker/internal/system/system.go`** (+35 lines)
   - Added `bytes` and `os/exec` imports
   - Modified `GetSystemResources()` to detect GPU
   - Added `detectGPUCount()` function

## Files Created

1. **`test_gpu_detection.sh`**
   - Automated test script for GPU detection
   
2. **`docs/049_GPU_DETECTION.md`**
   - This documentation

## Summary

✅ **GPU detection implemented** - Workers now correctly report GPU count  
✅ **Uses nvidia-smi** - Standard NVIDIA tool for GPU information  
✅ **Graceful fallback** - Works fine without GPUs (reports 0)  
✅ **Multiple GPU support** - Correctly counts multiple GPUs  
✅ **Tested** - Verified on system with RTX 4060 (reports 1 GPU)  
✅ **Scheduler-ready** - Master can now assign GPU-requiring tasks correctly  

Your worker will now report:
```
GPU: 1.0 (NVIDIA GeForce RTX 4060 Laptop GPU)
```

Instead of:
```
GPU: 0.0 (always)
```
