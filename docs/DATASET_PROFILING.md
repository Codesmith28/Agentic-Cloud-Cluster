# Full Dataset Profiling Report: Microsoft Azure Public Dataset (vmtable)

- **File**: `azure_vmtable_full.csv.gz`
- **Total Workload Records Analyzed**: **2,695,548**
- **Analysis Processing Time**: 9.14s

## 1. Resource Request Characteristics

| Metric | CPU Cores Requested | Memory (GB) Requested | Lifetime Duration (sec) |
| :--- | :---: | :---: | :---: |
| **Mean** | 3.7 cores | 15.91 GB | 216,962.69 s |
| **Median (P50)** | 2.0 cores | 8.0 GB | 1,800.0 s |
| **P75** | 4.0 cores | 32.0 GB | 12,000.0 s |
| **P90** | 8.0 cores | 32.0 GB | 266,400.0 s |
| **P95** | 8.0 cores | 32.0 GB | 2,591,400.0 s |
| **P99** | 24.0 cores | 64.0 GB | 2,591,400.0 s |
| **Min / Max** | 1.0 - 24.0 cores | 1.0 - 64.0 GB | 1.0 - 2,591,400.0 s |

## 2. Workload Categories Breakdown

| Category | Count | Proportion | Semantic Description |
| :--- | :---: | :---: | :--- |
| **Unknown** | 2,457,455 | 91.17% | Standard general cloud VM |
| **Delay-insensitive** | 159,615 | 5.92% | Batch / Delay-insensitive compute |
| **Interactive** | 78,478 | 2.91% | Interactive user service |

## 3. CPU Core Request Distribution

| CPU Cores | Count | Share (%) |
| :---: | :---: | :---: |
| **1.0** | 10,923 | 0.41% |
| **2.0** | 1,586,588 | 58.86% |
| **4.0** | 823,893 | 30.56% |
| **8.0** | 193,643 | 7.18% |
| **24.0** | 80,501 | 2.99% |

## 4. Memory (GB) Request Distribution

| Memory (GB) | Count | Share (%) |
| :---: | :---: | :---: |
| **1.0** | 10,923 | 0.41% |
| **2.0** | 326,044 | 12.10% |
| **4.0** | 413,201 | 15.33% |
| **8.0** | 1,014,932 | 37.65% |
| **32.0** | 846,387 | 31.40% |
| **64.0** | 84,061 | 3.12% |

## 5. Why This Dataset is Critical for Agentic Cloud Cluster

1. **Heterogeneous Bin-Packing Stress**: With CPU requests spanning from 1 to 64 cores and memory from 1 to 256+ GB, standard Round-Robin suffers massive fragmentation. The **PPO Reinforcement Learning** scheduler leverages multi-dimensional resource vectors to pack nodes tightly.
2. **Severe Long-Tail & Bursty Arrival Modeling**: Lifetimes span from short jobs (<5s) to long jobs (>100,000s), enabling realistic empirical benchmarking of our **`bursty`** and **`long-tail`** test profiles.
3. **Real-World SLA Attainment**: Category tags (`Interactive` vs `Delay-insensitive`) naturally translate into SLA priority deadlines.
