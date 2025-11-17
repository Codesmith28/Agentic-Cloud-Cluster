# CloudAI Testing Quick Reference

## One-Line Commands

### Setup
```bash
# Start services
./runMaster.sh                           # Terminal 1: Master node
cd database && docker-compose up -d      # MongoDB
./runWorker.sh                           # Terminals 2-5: Workers

# Register workers
bash test/register_workers.sh
```

### Testing
```bash
# Quick validation (2 tasks, ~30s)
bash test/smoke_test.sh

# Full test suite (30 tasks, 10-30min)
bash test/submit_all.sh

# Individual workload types
bash test/workloads/submit_cpu_light.sh
bash test/workloads/submit_cpu_heavy.sh
bash test/workloads/submit_memory_heavy.sh
bash test/workloads/submit_gpu_inference.sh
bash test/workloads/submit_gpu_training.sh
bash test/workloads/submit_mixed.sh
```

### Verification
```bash
bash test/verify/check_task_distribution.sh    # Worker assignments
bash test/verify/check_sla_violations.sh       # SLA rates
bash test/verify/check_ga_output.sh            # GA parameters
```

## Environment Variables

```bash
API=http://localhost:8080                      # Master endpoint
MONGO=mongodb://localhost:27017                # MongoDB connection
DB=cloudai                                     # Database name
POLL_INTERVAL=10                               # Status check interval (s)
TIMEOUT=1800                                   # Max wait time (s)
```

## Common Scenarios

### Remote Master
```bash
API=http://10.1.133.148:8080 bash test/submit_all.sh
```

### Extended Timeout
```bash
TIMEOUT=3600 bash test/submit_all.sh
```

### Custom MongoDB
```bash
MONGO=mongodb://remote:27017 DB=test bash test/submit_all.sh
```

## Master CLI Commands

```bash
# Inside master terminal
workers                  # List registered workers
queue                    # View task queue
list-tasks               # Show all tasks
monitor <task_id>        # Monitor specific task
internal-state           # View tau values and state
stats                    # System statistics
register <id> <addr>     # Register worker
task <img> <cpu> <mem> <gpu> -type <type>  # Submit task
```

## Quick Diagnostics

```bash
# Check master API
curl http://localhost:8080/health

# Check MongoDB
mongo mongodb://localhost:27017/cloudai --eval "db.runCommand({ping:1})"

# Check task counts
mongo cloudai --eval "db.results.count()"

# Check worker count
mongo cloudai --eval "db.workers.count()"

# View recent tasks
mongo cloudai --eval "db.results.find().sort({_id:-1}).limit(5).pretty()"
```

## File Locations

```
test/
├── smoke_test.sh              # Quick test
├── submit_all.sh              # Full test wrapper
├── register_workers.sh        # Worker registration
├── workloads/*.sh             # Individual submissions
├── verify/*.sh                # Verification scripts
└── logs/                      # Generated logs

docs/Scheduler/
└── TASK_5_1_CLI_TESTING_GUIDE.md  # Full guide
```

## Expected Results

| Metric | First Run | After GA Training |
|--------|-----------|-------------------|
| Submission Rate | 95-100% | 95-100% |
| Completion Rate | 95-100% | 95-100% |
| SLA Rate (Light) | 80-100% | 85-100% |
| SLA Rate (Heavy) | 60-80% | 70-90% |
| Task Distribution | Balanced ±2 | Optimized (uneven) |

## Troubleshooting

| Issue | Quick Fix |
|-------|-----------|
| API unreachable | `ps aux \| grep masterNode` |
| Tasks stuck | Check workers: `workers` in CLI |
| MongoDB error | `cd database && docker-compose up -d` |
| High SLA violations | Check `internal-state` for tau values |
| Worker not found | Re-register: `bash test/register_workers.sh` |

## Documentation

- Full Guide: `docs/Scheduler/TASK_5_1_CLI_TESTING_GUIDE.md`
- Test README: `test/README.md`
- Summary: `docs/Scheduler/TEST_INFRASTRUCTURE_SUMMARY.md`
- Sprint Plan: `docs/Scheduler/SPRINT_PLAN.md`
