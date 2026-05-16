# Scheduling Benchmark Report

Generated: 2026-04-11T18:36:03+05:30

## steady

Steady-state stream emphasizing queue latency and throughput consistency over long horizons.

| Scheduler | SLA % | P95 Wait (s) | Throughput (tasks/min) | Makespan (s) | CPU Util % | Balance |
|---|---:|---:|---:|---:|---:|---:|
| RTS | 100.00 | 0.00 | 15.26 | 361.63 | 9.03 | 0.766 |
| Round-Robin | 100.00 | 0.00 | 15.20 | 363.12 | 9.75 | 1.000 |

Winner: **Round-Robin**

- SLA improvement (RTS vs RR): 0.00%
- P95 queue wait reduction: 0.00%
- Makespan reduction: 0.41%
- Throughput gain: 0.41%

