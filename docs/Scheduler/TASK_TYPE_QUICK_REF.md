# Task Type Quick Reference

## Available Task Types

| Type | Description | Resource Profile | Use Cases |
|------|-------------|------------------|-----------|
| `cpu-light` | Light CPU workloads | Low CPU, moderate memory | Web servers, APIs, simple scripts |
| `cpu-heavy` | Heavy CPU workloads | High CPU (>4 cores) | Video encoding, compilation, calculations |
| `memory-heavy` | Memory-intensive | High memory (>8GB) | Big data, in-memory DBs, caching |
| `gpu-inference` | GPU inference | GPU required | Model serving, predictions, image processing |
| `gpu-training` | GPU training | High GPU (>2), high CPU (>4) | Deep learning training, ML experiments |
| `mixed` | Mixed/unknown workloads | Variable | Pipelines, hybrid workloads |

## CLI Syntax

```bash
task <docker_image> -type <task_type> [other_options]
```

### Flags
- `-type` or `-task_type` : Specify task type
- `-cpu_cores` : CPU cores required
- `-mem` : Memory in GB
- `-gpu_cores` : GPU units
- `-storage` : Storage in GB
- `-k` or `-sla` : SLA multiplier (1.5-2.5)

## Quick Examples

```bash
# CPU Heavy
task encoder:latest -cpu_cores 8 -mem 4 -type cpu-heavy

# Memory Heavy  
task cache:latest -cpu_cores 4 -mem 32 -type memory-heavy

# GPU Inference
task api:latest -cpu_cores 2 -mem 4 -gpu_cores 1 -type gpu-inference

# GPU Training
task ml-train:latest -cpu_cores 8 -mem 16 -gpu_cores 4 -type gpu-training

# CPU Light
task webapp:latest -cpu_cores 2 -mem 2 -type cpu-light

# Mixed
task pipeline:latest -cpu_cores 4 -mem 8 -gpu_cores 1 -type mixed

# Auto-detect (no type specified)
task myapp:latest -cpu_cores 4 -mem 8
```

## Validation Rules

✅ **Valid**: Any of the 6 types above  
⚠️ **Invalid**: Any other string → Falls back to auto-inference  
🔄 **Empty**: Not specified → Auto-infers from resources

## Auto-Inference Rules

When `-type` is not specified or invalid:

1. GPU > 2 AND CPU > 4 → `gpu-training`
2. GPU > 0 → `gpu-inference`
3. Memory > 8 → `memory-heavy`
4. CPU > 4 → `cpu-heavy`
5. CPU > 0 → `cpu-light`
6. Otherwise → `mixed`

## Display Format

### With User-Specified Type
```
  Task Classification:
    • Type:          cpu-heavy (user-specified)
```

### Auto-Inferred Type
```
  Task Classification:
    • Type:          (will be inferred from resources)
```

## Verification

### Check in CLI
After submission, look for:
```
📋 Task task-1731753000 submitted and queued (position: 1, k=1.8, type=cpu-heavy)
```

### Check in Database
```javascript
db.TASKS.find({ task_id: "task-1731753000" }, { task_type: 1 })
```

## Help Command

```bash
help
```

Shows all task types and examples.

## Common Patterns

### Web Service
```bash
task nginx:latest -cpu_cores 2 -mem 2 -type cpu-light
```

### Data Processing
```bash
task spark:latest -cpu_cores 8 -mem 32 -type memory-heavy
```

### ML Inference API
```bash
task model-api:latest -cpu_cores 4 -mem 8 -gpu_cores 1 -k 1.5 -type gpu-inference
```

### Deep Learning Training
```bash
task pytorch-train:latest -cpu_cores 16 -mem 64 -gpu_cores 4 -k 2.5 -type gpu-training
```

### Video Processing
```bash
task ffmpeg:latest -cpu_cores 8 -mem 8 -type cpu-heavy
```

## Best Practices

1. ✅ **Always specify type when known**
2. ✅ **Use consistent types for similar tasks**
3. ✅ **Match type with actual resource usage**
4. ✅ **Monitor scheduler behavior**
5. ⚠️ **Don't override if unsure** - let system infer

## Troubleshooting

### Warning: Invalid task type
```
⚠️  Warning: Invalid task type 'my-type'. Must be one of: [cpu-light cpu-heavy ...]
    Task type will be automatically inferred from resources.
```
**Solution**: Use one of the 6 valid types

### Type not showing in logs
**Check**: Proto regenerated? Server restarted?

### Auto-inference giving wrong type
**Solution**: Specify explicit `-type` flag

## Integration Points

- ✅ Proto: `task.task_type`
- ✅ Database: `task_type` field
- ✅ CLI: `-type` flag
- ✅ Server: Validation + logging
- ✅ Scheduler: `TaskView.Type`
- 🔜 Web UI: Dropdown menu

## Files Modified

1. `proto/master_worker.proto` - Added task_type field
2. `master/internal/db/tasks.go` - Added TaskType field
3. `master/internal/cli/cli.go` - Added -type flag
4. `master/internal/server/master_server.go` - Added validation
5. `master/internal/scheduler/rts_models.go` - Updated NewTaskViewFromProto

## Testing Checklist

- [ ] Submit task with each type
- [ ] Submit task with invalid type
- [ ] Submit task without type
- [ ] Verify in database
- [ ] Check server logs
- [ ] Test with scheduler
