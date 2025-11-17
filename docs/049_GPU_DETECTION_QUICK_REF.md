# GPU Detection - Quick Reference

## What Changed

Workers now **detect and report GPU count** using `nvidia-smi`.

**Before**: Always reported `TotalGPU = 0.0`  
**After**: Reports actual GPU count (e.g., `TotalGPU = 1.0` for 1 GPU)

## Verification

### Check Your GPU

```bash
nvidia-smi --query-gpu=name --format=csv,noheader
```

**Your output:**
```
NVIDIA GeForce RTX 4060 Laptop GPU
```
→ Worker will report: **GPU = 1.0** ✓

### Test Detection

```bash
./test_gpu_detection.sh
```

### Verify in Master

```bash
master> workers

# Should show:
║ worker-1
║   Resources: CPU=8.0, Memory=16.0GB, GPU=1.0  ← GPU count
```

## How It Works

1. Worker calls `nvidia-smi --query-gpu=name --format=csv,noheader`
2. Counts output lines (1 line = 1 GPU)
3. Reports count to master during registration
4. Master uses count for task scheduling

## No GPU System

If you don't have NVIDIA GPU or nvidia-smi:
- Worker reports `TotalGPU = 0.0`
- Everything works normally
- Just can't run GPU tasks

## Requirements

- ✅ NVIDIA GPU
- ✅ NVIDIA drivers installed
- ✅ `nvidia-smi` in PATH
- ✅ Linux/Unix

## Logs

**With GPU:**
```
Detected 1 GPU(s)
Worker registered: CPU=8.0, Memory=16.0GB, GPU=1.0
```

**Without GPU:**
```
Info: No GPU detected or nvidia-smi not available
Worker registered: CPU=8.0, Memory=16.0GB, GPU=0.0
```

## Troubleshooting

### GPU not detected?

```bash
# Check nvidia-smi works
nvidia-smi

# If not found, install drivers
sudo apt install nvidia-driver-XXX

# Add to PATH if needed
export PATH=$PATH:/usr/bin
```

## Task Scheduling

Master now assigns GPU tasks correctly:
- Task requires GPU = 1.0
- Master only assigns to workers with GPU ≥ 1.0
- Your worker (GPU=1.0) is eligible ✓

## Files Changed

- `worker/internal/system/system.go` - Added GPU detection
- `test_gpu_detection.sh` - Test script
- `docs/049_GPU_DETECTION.md` - Full documentation

## See Also

- [Full Documentation](049_GPU_DETECTION.md)
