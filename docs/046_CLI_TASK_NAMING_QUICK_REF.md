# CLI Task Naming Quick Reference

## Commands

### Submit Task (Scheduler-based)
```bash
task <image> [-name <task_name>] [resource flags]
```

### Dispatch Task (Direct to worker)
```bash
dispatch <worker_id> <image> [-name <task_name>] [resource flags]
```

## Examples

### Basic Usage
```bash
# Auto-generated task name
task docker.io/user/ml-training:latest

# Custom task name
task docker.io/user/ml-training:latest -name experiment-001

# Dispatch with task name
dispatch worker-1 ubuntu:latest -name my-test
```

### With Resources
```bash
task alpine:latest -name alpine-test -cpu_cores 2.0 -mem 4.0
dispatch worker-2 python:3.9 -name data-processing -cpu_cores 4.0 -mem 8.0 -gpu_cores 1.0
```

## Task Name Auto-Generation

| Docker Image | Generated Task Name |
|-------------|-------------------|
| `ubuntu:latest` | `ubuntu-1704067200` |
| `docker.io/user/ml-training:v1.0` | `ml-training-1704067200` |
| `gcr.io/project/app:dev` | `app-1704067200` |
| `alpine` | `alpine-1704067200` |

**Pattern**: `<image-name>-<unix-timestamp>`

## File Organization

Files are automatically organized by task name:

```
/var/cloudai/files/
  └── admin/
      └── experiment-001/
          └── 1704067200/
              └── task-1704067200/
                  ├── output.txt
                  ├── results.json
                  └── model.pkl
```

**Path Pattern**: `<user>/<task_name>/<timestamp>/<task_id>/`

## Database Schema

```javascript
{
  task_id: "task-1704067200",
  user_id: "admin",
  task_name: "experiment-001",      // NEW
  submitted_at: 1704067200,         // NEW
  docker_image: "...",
  status: "pending",
  // ...
}
```

## File Collection (Automatic)

✅ **Default Behavior** - No configuration needed:
- Volume mounted: `/output` in container
- Worker collects: All files in `/output`
- Worker uploads: To master via gRPC
- Master stores: In organized directory structure

**Container Usage**:
```bash
# Inside your container
echo "Results" > /output/results.txt
python train.py --output-dir /output
./app --log-dir /output/logs
```

## Key Benefits

- 🏷️ **Human-readable task names** instead of just IDs
- 📁 **Organized file storage** by task name and timestamp
- 🔍 **Easy task lookup** by name in database
- 📊 **Chronological tracking** via timestamp
- 🚀 **Auto-generation** when name not provided
- 💾 **Automatic file collection** for all tasks

## Testing

```bash
# 1. Start master
./master/masterNode

# 2. Submit task with custom name
task alpine:latest -name test-001

# 3. Verify in MongoDB
mongosh mongodb://localhost:27017/cloudai
db.TASKS.find({task_name: "test-001"}).pretty()

# 4. Check files (after task completion)
ls -la /var/cloudai/files/admin/test-001/
```

## Display Output

```
═══════════════════════════════════════════════════════
  🚀 SUBMITTING TASK TO SCHEDULER
═══════════════════════════════════════════════════════
  Task ID:           task-1704067200
  Task Name:         experiment-001
  Docker Image:      docker.io/user/ml-training:latest
  Submitted At:      2025-01-01 12:00:00
───────────────────────────────────────────────────────
  Resource Requirements:
    • CPU Cores:     2.00 cores
    • Memory:        4.00 GB
    • Storage:       1.00 GB
    • GPU Cores:     0.00 cores
═══════════════════════════════════════════════════════
```

## Future API Endpoints

```
GET /api/tasks?task_name=experiment-001
GET /api/files?task_name=experiment-001
GET /api/files/:task_id
GET /api/files/:task_id/:file_path
```
