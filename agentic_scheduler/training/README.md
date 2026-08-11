# Alibaba Cluster Trace 2018 Dataset Documentation

## Overview

The data stored in this directory is derived from the **Alibaba Cluster Trace v2018** dataset. It represents 8 days of production cluster data collected from a cluster of approximately 4,000 machines. The dataset captures a co-located workload environment, which includes:
- **Online Services:** Long-running services (managed by the "Sigma" scheduler) housed in containers.
- **Batch Workloads:** Short-lived computing jobs (managed by the "Fuxi" scheduler).

This dataset is widely used for research in resource scheduling, cluster management, and machine learning-based workload optimization. In our framework, it serves as a source of real-world traces to train and evaluate our Reinforcement Learning (PPO) agent under realistic arrival patterns and resource constraints.

---

## Dataset Structure & Files

The `raw/` directory contains the compressed archives and extracted CSVs of the trace. The key files include:

### 1. `machine_meta.csv`
Contains hardware metadata and lifecycle events for the physical machines in the cluster.
- **Key Columns:** `machine_id`, `time_stamp`, `cpu_num`, `mem_size`, `disk_size`
- **Details:** Resource values (CPU, memory) are typically normalized to obscure sensitive hardware details. `cpu_num` generally represents the number of cores, and `mem_size` is normalized to a fraction of a baseline (often `[0, 100]`).

### 2. `batch_task.csv`
Defines the DAGs of batch jobs, their task configurations, and actual runtime metrics.
- **Key Columns:** `task_name`, `instance_num`, `job_name`, `task_type`, `status`, `start_time`, `end_time`, `plan_cpu`, `plan_mem`
- **Details:** Batch workloads are modeled as a hierarchy of Job -> Task -> Instance. `plan_cpu` and `plan_mem` describe the resource requests for instances within the task.

### 3. `container_meta.csv`
Contains metadata and events for containers running the online service workloads.
- **Key Columns:** `container_id`, `machine_id`, `time_stamp`, `app_du`, `status`, `cpu_request`, `mem_size`

### 4. `batch_instance.tar.gz` & `machine_usage.tar.gz`
- **Batch Instance:** Finer-grained data detailing the specific executions (instances) of each task, including granular start and end times.
- **Machine Usage:** Time-series telemetry data of the machines' actual resource utilization (CPU, memory, disk I/O) measured at regular intervals.

---

## Usage in the Agentic Scheduler Framework

Our framework leverages this data primarily to simulate realistic cluster environments for offline training of our PPO scheduler. The integration is handled through a trace ingestion pipeline and an environment replay mechanism.

### 1. Data Ingestion & Normalization (`training/trace_loader.py`)

The `load_alibaba_trace()` function is responsible for parsing the CSV files and converting them into a unified, framework-agnostic format called `TraceCluster`. 

**What is the `TraceCluster` format?**
`TraceCluster` is an internal data class that standardizes trace data from different sources (such as Alibaba, Google, or internal records) so that the Reinforcement Learning environment can interact with a consistent API. It is primarily composed of:
- **`workers` (List of Dictionaries):** Represents the available physical machines in the cluster. Each dictionary standardizes a machine's hardware capacity with keys like `worker_id`, `total_cpu`, `total_memory`, and `total_storage`.
- **`tasks` (List of `TraceTask` Objects):** A chronologically sorted list of all jobs to be simulated. The `TraceTask` class normalizes individual task requirements into standard fields, including:
  - `task_id`: Unique identifier for the task.
  - `arrival_time`: Seconds elapsed from the start of the trace, used by the replay environment to simulate time passing and task submission.
  - `req_cpu` & `req_memory`: Standardized resource requests (cores and GBs).
  - `runtime_seconds`: The duration the task needs to run to completion, which dictates how long resources are occupied during simulation.
  - `task_type`: Categorization of the workload (e.g., `cpu-heavy`, `mixed`).

By mapping Alibaba's raw format into `TraceCluster` and `TraceTask` objects, the training environment does not need to handle any Alibaba-specific parsing logic. The ingestion process includes:

- **Machine Parsing:** 
  The loader reads `machine_meta.csv` to build a list of worker nodes, parsing `cpu_num`, `mem_size`, and `disk_size` to establish the initial cluster capacity.
- **Task Parsing:** 
  The loader processes `batch_task.csv` to instantiate `TraceTask` objects. Invalid tasks (e.g., negative start times) are filtered out.
- **Resource Normalization:**
  - **CPU:** Alibaba's batch traces often represent CPU in centi-cores (where 100 = 1 core). The loader's `_normalize_alibaba_cpu()` function converts `plan_cpu` into actual standard core units.
  - **Memory:** Memory requests (`plan_mem`) are often normalized as a fraction of a host's budget. The `_normalize_alibaba_memory()` function scales values $\le 1.0$ by multiplying them by 64 (approximating GBs) to match standard cluster logic.
- **Task Classification:**
  Based on the ratio of `req_cpu` to `req_memory`, tasks are algorithmically categorized via `_classify_task_type()` into profiles such as `cpu-heavy`, `memory-heavy`, `cpu-light`, or `mixed`. The raw `task_type` from the CSV is also mapped through a predefined dictionary (`ALIBABA_TASK_TYPE_MAP`).

### 2. Environment Simulation (`train_ppo.py`)

During PPO training, the loaded `TraceCluster` object is passed into the `TraceReplayEnv`. 

- **Trace Replay:** Rather than generating synthetic tasks at random, the `TraceReplayEnv` uses the exact sequence, arrival times (`start_time`), and resource constraints found in the Alibaba trace. 
- **Offline RL Training:** The PPO agent (in `train_ppo.py`) learns to schedule these real-world workloads by choosing which task to assign to which worker node at specific timesteps. The environment issues rewards based on how well the agent respects the machine capacities and minimizes queue wait times for the real Alibaba trace distribution.
- **Trace Windows:** The framework allows filtering the trace into specific time windows (using arguments like `--trace-window-start` and `--trace-window-end`) to evaluate the scheduler's performance during periods of low or high cluster contention.

By using the Alibaba v2018 dataset, the agentic scheduler can be validated against highly complex, co-located production workloads, ensuring that the learned policies are robust enough for real-world cloud data centers.

---

## Frequent Questions & Critical Concepts

### Offline Training on Alibaba vs. Live Inference

**Can I train the model offline on Alibaba's dataset explicitly and then use that training when I run my scheduler?**
Yes, absolutely. The framework is designed so you can "pre-train" the agent on the Alibaba dataset offline, allowing it to learn efficient scheduling policies without risking live cluster resources or starting from a "blank slate." This offline training process outputs a `.pt` model checkpoint (e.g., `ppo_trained.pt`) which the live scheduler can then load when deployed.

**How to do it and does it train on the entire data?**
To initiate offline training, you can run a command similar to:
```bash
python -m agentic_scheduler.train_ppo --trace-source alibaba --trace-path agentic_scheduler/data/alibaba_v2018/raw --max-trace-tasks 5000
```

By default, the script does not train on the entire 8-day dataset simultaneously due to memory constraints and training efficiency. Instead, it bounds the training to a subset of tasks (using `--max-trace-tasks`, which defaults to `5000`) or within a targeted time frame (using `--trace-window-start`). The PPO algorithm then iterates over this subset for a specified number of epochs (`--updates`), continually refining the neural network's policy until it converges on an optimal scheduling strategy for that workload profile.

### Alibaba Dataset vs. Live Go Cluster Framework Data

When transitioning from the static offline dataset to live inference on the Go cluster framework, several key differences arise in how the data is handled:

**1. Data Dynamics (Historical vs. Live):**
- **Alibaba Data:** The data is historical and static. The exact `runtime_seconds` and arrival behaviors are known upfront, allowing the `TraceReplayEnv` to simulate task durations perfectly.
- **Go Framework Data:** The data is dynamic and live. Tasks arrive via RPCs or database inserts, and the scheduler must make decisions in real-time without knowing the exact future execution duration of the job.

**2. Schema and Formatting:**
- **Alibaba Data:** The raw CSVs use abstract or exotic units (e.g., CPU in centi-cores, memory as a normalized ratio between `[0, 100]`).
- **Go Framework Data:** The Go cluster communicates absolute hardware limits (e.g., explicit CPU cores, exact GBs of memory).

**How we handle these differences:**
The disparity between the two data sources is precisely why the `trace_loader.py` adapter exists. Before the neural network sees any data, the loader forces the exotic Alibaba values into the canonical `TraceCluster` and `TraceTask` formats, mathematically converting centi-cores to absolute cores, and normalizing budget fractions to GB estimates.

Because the Reinforcement Learning agent only ever interacts with numerical feature arrays (`task_features`, `worker_features`) derived from this canonical structure, the Neural Network receives inputs that are already normalized against the absolute maximum capacity of the cluster. Consequently, the trained PPO model becomes completely agnostic to the source of the data. As long as the Go framework and the offline environment construct their feature arrays identically, the offline-trained model performs seamlessly during online inference.
