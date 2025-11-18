# CLI Task Naming and Timestamp Support

**Status**: ✅ Implemented  
**Date**: 2025-01-XX  
**Components**: Master CLI, Database Schema

## Overview

Enhanced both CLI task submission commands (`task` and `dispatch`) with task naming and timestamp tracking support. This enables user-friendly task identification and proper file organization.

## What Was Changed

### 1. Proto Fields (Already Done)
```protobuf
message Task {
  string task_id = 1;
  string docker_image = 2;
  string command = 3;
  // ... resource requirements ...
  string user_id = 10;
  string task_name = 11;     // NEW: User-friendly task name
  int64 submitted_at = 12;   // NEW: Unix timestamp
}
```

### 2. Database Schema
**File**: `master/internal/db/tasks.go`

```go
type Task struct {
  TaskID      string    `bson:"task_id"`
  UserID      string    `bson:"user_id"`
  TaskName    string    `bson:"task_name"`    // NEW
  SubmittedAt int64     `bson:"submitted_at"` // NEW
  DockerImage string    `bson:"docker_image"`
  Command     string    `bson:"command"`
  // ... other fields ...
}
```

### 3. CLI Commands
**File**: `master/internal/cli/cli.go`

#### A. `task` Command (Scheduler-based submission)

**Updated Usage**:
```bash
task <docker_image> [-name <task_name>] [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]
```

**New Features**:
- `-name` flag: Optional custom task name
- Auto-generated task name: Extracted from docker image if not provided
  - Example: `docker.io/user/sample-task:latest` → `sample-task-1704067200`
- Automatic timestamp: Captured at submission time
- Enhanced display: Shows task name and formatted submission timestamp

**Examples**:
```bash
# With auto-generated task name
task docker.io/user/ml-training:latest

# With custom task name
task docker.io/user/ml-training:latest -name experiment-001

# With resources and custom name
task docker.io/user/ml-training:latest -name gpu-training -cpu_cores 4.0 -mem 8.0 -gpu_cores 2.0
```

**Display Output**:
```
═══════════════════════════════════════════════════════
  🚀 SUBMITTING TASK TO SCHEDULER
═══════════════════════════════════════════════════════
  Task ID:           task-1704067200
  Task Name:         ml-training-1704067200
  Docker Image:      docker.io/user/ml-training:latest
  Submitted At:      2025-01-01 12:00:00
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
```

#### B. `dispatch` Command (Direct worker assignment)

**Updated Usage**:
```bash
dispatch <worker_id> <docker_image> [-name <task_name>] [-cpu_cores <num>] [-mem <gb>] [-storage <gb>] [-gpu_cores <num>]
```

**New Features**:
- Same `-name` flag behavior as `task` command
- Same auto-generation logic
- Same timestamp tracking
- Enhanced display with task name and timestamp

**Examples**:
```bash
# With auto-generated task name
dispatch worker-1 docker.io/user/sample-task:latest

# With custom task name
dispatch worker-1 docker.io/user/sample-task:latest -name my-experiment

# With resources
dispatch worker-2 docker.io/user/ml-job:latest -name benchmark-test -cpu_cores 2.0 -mem 4.0
```

**Display Output**:
```
═══════════════════════════════════════════════════════
  🎯 DISPATCHING TASK DIRECTLY TO WORKER
═══════════════════════════════════════════════════════
  Task ID:           task-1704067200
  Task Name:         sample-task-1704067200
  Target Worker:     worker-1
  Docker Image:      docker.io/user/sample-task:latest
  Submitted At:      2025-01-01 12:00:00
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     1.00 cores
    • Memory:        0.50 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
───────────────────────────────────────────────────────
  ⚠️  NOTE: Bypassing scheduler - dispatching directly!
═══════════════════════════════════════════════════════
```

### 4. Server Methods
**File**: `master/internal/server/master_server.go`

Both `SubmitTask()` and `DispatchTaskToWorker()` now populate the database with task_name and submitted_at:

```go
dbTask := &db.Task{
  TaskID:      task.TaskId,
  UserID:      task.UserId,
  TaskName:    task.TaskName,    // NEW
  SubmittedAt: task.SubmittedAt, // NEW
  DockerImage: task.DockerImage,
  // ... other fields ...
}
```

## Implementation Details

### Task Name Auto-Generation Logic
```go
// If -name flag not provided:
if taskName == "" {
    // Extract image name from docker image
    // "user/image:tag" -> "image"
    imageParts := strings.Split(dockerImage, "/")
    imageName := imageParts[len(imageParts)-1]
    imageName = strings.Split(imageName, ":")[0] // Remove tag
    
    // Generate: "image-<timestamp>"
    taskName = fmt.Sprintf("%s-%d", imageName, time.Now().Unix())
}
```

**Examples**:
- `docker.io/user/ml-training:v1.0` → `ml-training-1704067200`
- `ubuntu:latest` → `ubuntu-1704067200`
- `gcr.io/project/custom-app:dev` → `custom-app-1704067200`

### Timestamp Format
- Captured: `time.Now().Unix()` (Unix timestamp)
- Stored in proto: `int64`
- Stored in database: `int64`
- Display format: `2006-01-02 15:04:05`

## Integration with File Storage

The task_name and submitted_at fields are **critical** for file organization:

**File Storage Path Pattern**:
```
/var/cloudai/files/<user_id>/<task_name>/<timestamp>/<task_id>/
```

**Example**:
```
/var/cloudai/files/admin/ml-training-1704067200/1704067200/task-1704067200/
├── output.txt
├── results.json
└── model.pkl
```

**Benefits**:
- Users can browse files by task name (human-readable)
- Tasks are organized chronologically by timestamp
- Multiple runs of the same task name are kept separate
- Task ID ensures uniqueness within a timestamp

## File Collection - Default Behavior

**IMPORTANT**: File collection is **automatic** for all tasks, not optional.

### How It Works

1. **Worker Mounts Volume**: 
   - Host path: `/var/cloudai/outputs/<task_id>/`
   - Container path: `/output`
   
2. **Task Writes Files**:
   ```bash
   # Inside container
   echo "Results" > /output/results.txt
   python train.py --output-dir /output
   ```

3. **Worker Collects Files**:
   - Automatically lists all files in `/var/cloudai/outputs/<task_id>/`
   - Creates tarball of entire directory

4. **Worker Uploads Files**:
   - Streams tarball to master via gRPC
   - Uses 1MB chunks for efficient transfer

5. **Master Stores Files**:
   - Extracts tarball to organized directory
   - Saves metadata to FILE_METADATA collection
   - Updates task with `output_files` list

### No Configuration Required

Users don't need to:
- ❌ Specify file collection in task submission
- ❌ Configure volume mounting
- ❌ Manually trigger file upload
- ❌ Request file transfer

System automatically:
- ✅ Mounts `/output` volume in every container
- ✅ Collects all files written to `/output`
- ✅ Uploads files to master after task completion
- ✅ Organizes files by user, task name, and timestamp

## Help Text Updates

Both commands now show:
```
Note: The scheduler will automatically select the best worker.
      Files generated in /output will be automatically collected and stored.
```

This clearly communicates:
1. File collection is automatic
2. Container should write to `/output`
3. No additional setup needed

## Testing the CLI

### Test 1: Auto-generated Task Name
```bash
# Start master
./master/masterNode

# In CLI:
task docker.io/library/alpine:latest

# Expected output should show:
# Task Name: alpine-<timestamp>
```

### Test 2: Custom Task Name
```bash
task docker.io/library/alpine:latest -name my-alpine-test

# Expected output should show:
# Task Name: my-alpine-test
```

### Test 3: Dispatch with Task Name
```bash
dispatch worker-1 ubuntu:latest -name ubuntu-experiment

# Expected output should show:
# Task Name: ubuntu-experiment
```

### Test 4: Database Verification
```bash
# Connect to MongoDB
mongosh mongodb://localhost:27017/cloudai

# Query tasks
db.TASKS.find().pretty()

# Should show task_name and submitted_at fields
```

## Benefits

### 1. User Experience
- Human-readable task identification
- No need to remember task IDs like `task-1704067200`
- Can reference tasks by meaningful names: "my-experiment", "benchmark-v1"

### 2. File Organization
- Files organized by task name and timestamp
- Easy to locate specific task outputs
- Chronological ordering within each task name

### 3. Future Features Enabled
- HTTP API: `GET /api/files?task_name=my-experiment`
- Web UI: Browse files by task name
- Analytics: Track task execution patterns over time
- Cleanup: Delete old files by timestamp

### 4. Database Queries
```javascript
// Find all tasks with a specific name
db.TASKS.find({task_name: "ml-training-experiment"})

// Find tasks submitted after a certain date
db.TASKS.find({submitted_at: {$gt: 1704067200}})

// Find tasks by name pattern
db.TASKS.find({task_name: /^ml-training/})
```

## Future HTTP API Endpoints

With task names in the database, we can add:

```
GET /api/tasks?task_name=<name>        # Query tasks by name
GET /api/files?task_name=<name>         # List files by task name
GET /api/files/:task_id                 # Get specific task files
GET /api/files/:task_id/:file_path      # Download specific file
DELETE /api/tasks/:task_id/files        # Delete task files
```

## Files Modified

1. ✅ `proto/master_worker.proto` - Added task_name, submitted_at fields
2. ✅ `master/internal/db/tasks.go` - Updated Task struct
3. ✅ `master/internal/cli/cli.go` - Enhanced both commands
4. ✅ `master/internal/server/master_server.go` - Updated SubmitTask, DispatchTaskToWorker

## Build Verification

```bash
make master  # ✅ Builds successfully
```

## Next Steps

1. **Testing** (Recommended Next):
   - Test `task` command with/without `-name` flag
   - Test `dispatch` command with/without `-name` flag
   - Verify database entries have correct fields
   - Verify file organization uses task names

2. **HTTP API** (Future Enhancement):
   - Implement file listing by task name
   - Add file download endpoints
   - Add task query by name endpoint

3. **Web UI** (Future Enhancement):
   - Add task name column to task list
   - Add submitted_at timestamp display
   - Add file browser grouped by task name

## Quick Reference

### Submission Commands
```bash
# Auto-generated task name
task <image>

# Custom task name
task <image> -name <custom-name>

# Direct dispatch with task name
dispatch <worker-id> <image> -name <custom-name>
```

### Task Name Format
- Auto-generated: `<image-name>-<unix-timestamp>`
- Custom: User-specified string (no validation)

### File Organization
```
/var/cloudai/files/
  └── <user_id>/
      └── <task_name>/
          └── <timestamp>/
              └── <task_id>/
                  └── [output files]
```

### Database Fields
```javascript
{
  task_id: "task-1704067200",
  user_id: "admin",
  task_name: "ml-training-experiment",  // NEW
  submitted_at: 1704067200,             // NEW (Unix timestamp)
  docker_image: "...",
  status: "pending",
  // ... other fields ...
}
```
