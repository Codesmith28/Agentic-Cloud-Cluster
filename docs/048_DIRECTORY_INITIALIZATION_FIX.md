# Directory Initialization Fix

**Status**: ✅ Fixed  
**Date**: 2025-11-17  
**Issue**: Bind mount failure - `/var/cloudai/outputs/<task-id>` path does not exist

## Problem

When dispatching tasks to workers, the following error occurred:

```
Error creating container: Error response from daemon: invalid mount config for type "bind": 
bind source path does not exist: /var/cloudai/outputs/task-1763368091
```

## Root Cause

Docker requires bind mount source paths to **exist before** creating the container. The issue had two parts:

1. **Worker**: The base directory `/var/cloudai/outputs/` was not created at worker startup
2. **Master**: The base directory `/var/cloudai/files/` was not created at master startup
3. **Error Handling**: Directory creation errors were logged as warnings instead of failing fast

## Solution

### 1. Worker Initialization (`worker/main.go`)

Added base directory creation at worker startup:

```go
func main() {
    log.Println("CloudAI Worker Node - Starting...")
    
    // Create base output directory for task files
    outputBaseDir := "/var/cloudai/outputs"
    if err := os.MkdirAll(outputBaseDir, 0755); err != nil {
        log.Fatalf("Failed to create base output directory %s: %v", outputBaseDir, err)
    }
    log.Printf("✓ Output directory ready: %s", outputBaseDir)
    
    // ... rest of initialization ...
}
```

**Benefits**:
- ✅ Base directory created once at startup
- ✅ Worker fails fast if directory can't be created (permissions issue)
- ✅ Per-task subdirectories can be created reliably

### 2. Master Initialization (`master/main.go`)

Added base directory creation at master startup:

```go
func main() {
    cfg := config.LoadConfig()
    
    // Create base file storage directory
    fileStorageBaseDir := "/var/cloudai/files"
    if err := os.MkdirAll(fileStorageBaseDir, 0755); err != nil {
        log.Fatalf("Failed to create base file storage directory %s: %v", fileStorageBaseDir, err)
    }
    log.Printf("✓ File storage directory ready: %s", fileStorageBaseDir)
    
    // ... rest of initialization ...
}
```

**Benefits**:
- ✅ File storage directory ready for uploads
- ✅ Master fails fast if directory can't be created
- ✅ Prevents runtime errors when receiving files

### 3. Improved Error Handling (`worker/internal/executor/executor.go`)

Changed directory creation error from warning to fatal error:

**Before** (incorrect):
```go
if err := os.MkdirAll(outputDir, 0755); err != nil {
    log.Printf("[Task %s] Warning: failed to create output directory: %v", taskID, err)
} else {
    log.Printf("[Task %s] Created output directory: %s", taskID, outputDir)
}
```

**After** (correct):
```go
if err := os.MkdirAll(outputDir, 0755); err != nil {
    return "", fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
}
log.Printf("[Task %s] ✓ Created output directory: %s", taskID, outputDir)
```

**Benefits**:
- ✅ Task fails immediately if directory can't be created
- ✅ Error is propagated to master with clear message
- ✅ No silent failures or confusing container errors

## Directory Structure

### Worker (`/var/cloudai/outputs/`)

```
/var/cloudai/outputs/
├── task-1763368091/
│   ├── output.txt
│   └── results.json
├── task-1763368092/
│   └── model.pkl
└── task-1763368093/
    └── data.csv
```

**Permissions**: `0755` (drwxr-xr-x)
- Created at worker startup
- Per-task subdirectories created before container launch
- Mounted to `/output` in containers

### Master (`/var/cloudai/files/`)

```
/var/cloudai/files/
├── admin/
│   ├── ml-training-experiment/
│   │   └── 1704067200/
│   │       └── task-1704067200/
│   │           └── model.pkl
│   └── data-processing/
│       └── 1704153600/
│           └── task-1704153600/
│               └── output.csv
└── user123/
    └── benchmark-v1/
        └── 1704240000/
            └── task-1704240000/
                └── results.txt
```

**Permissions**: `0755` (drwxr-xr-x)
- Created at master startup
- Subdirectories created dynamically based on user/task/timestamp
- Populated by file uploads from workers

## Testing

### Test 1: Worker Startup
```bash
# Start worker
./worker/workerNode

# Expected output:
# ✓ Output directory ready: /var/cloudai/outputs
```

### Test 2: Master Startup
```bash
# Start master
./master/masterNode

# Expected output:
# ✓ File storage directory ready: /var/cloudai/files
```

### Test 3: Task Dispatch
```bash
# In master CLI
master> dispatch <worker-id> alpine:latest

# Should succeed without bind mount errors
# Worker creates: /var/cloudai/outputs/task-<timestamp>/
```

### Test 4: Directory Verification
```bash
# On worker machine
ls -la /var/cloudai/outputs/

# On master machine
ls -la /var/cloudai/files/
```

## Permission Requirements

Both master and worker need write permissions to `/var/cloudai/`:

### Option 1: Run as root (simple but not recommended)
```bash
sudo ./worker/workerNode
sudo ./master/masterNode
```

### Option 2: Create directories with proper ownership (recommended)
```bash
# On worker
sudo mkdir -p /var/cloudai/outputs
sudo chown $(whoami):$(whoami) /var/cloudai/outputs
sudo chmod 755 /var/cloudai/outputs

# On master
sudo mkdir -p /var/cloudai/files
sudo chown $(whoami):$(whoami) /var/cloudai/files
sudo chmod 755 /var/cloudai/files
```

### Option 3: Use different base directories (alternative)
Modify the code to use user-accessible directories:
- Worker: `~/cloudai/outputs/`
- Master: `~/cloudai/files/`

## Files Modified

1. ✅ `worker/main.go` - Added base directory creation
2. ✅ `master/main.go` - Added base directory creation
3. ✅ `worker/internal/executor/executor.go` - Improved error handling

## Build Verification

```bash
make worker  # ✅ Builds successfully
make master  # ✅ Builds successfully
```

## Startup Logs

### Worker
```
═══════════════════════════════════════════════════════
  CloudAI Worker Node - Starting...
═══════════════════════════════════════════════════════
✓ Output directory ready: /var/cloudai/outputs
✓ System information collected
...
```

### Master
```
✓ File storage directory ready: /var/cloudai/files
✓ MongoDB collections ensured
✓ WorkerDB initialized
...
```

## Related Documentation

- **043_FILE_TRANSFER_AND_STORAGE.md** - File transfer implementation
- **044_FILE_STORAGE_QUICK_REF.md** - File storage quick reference
- **047_COMPLETE_IMPLEMENTATION_SUMMARY.md** - Complete implementation overview

## Quick Reference

| Component | Directory | Purpose | Created |
|-----------|-----------|---------|---------|
| Worker | `/var/cloudai/outputs/` | Task output files | Startup |
| Worker | `/var/cloudai/outputs/<task-id>/` | Per-task files | Task execution |
| Master | `/var/cloudai/files/` | Uploaded file storage | Startup |
| Master | `/var/cloudai/files/<user>/<task>/<ts>/<id>/` | Organized files | File upload |

## Error Messages

### Before Fix
```
Error creating container: Error response from daemon: invalid mount config for type "bind": 
bind source path does not exist: /var/cloudai/outputs/task-1763368091
```

### After Fix (if permissions denied)
```
Failed to create base output directory /var/cloudai/outputs: permission denied
```

**Solution**: Grant write permissions to `/var/cloudai/` or run with appropriate privileges.
