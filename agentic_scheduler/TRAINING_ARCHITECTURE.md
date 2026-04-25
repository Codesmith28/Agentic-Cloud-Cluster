# Agentic Scheduler: PPO Training Architecture

This document provides a comprehensive, deep-dive explanation of the **Whats, Hows, and Whys** behind the Reinforcement Learning (RL) training architecture used by the `agentic_scheduler`. 

---

## 1. The "What": Reinforcement Learning for Cluster Scheduling

At its core, the goal of the `agentic_scheduler` is to learn how to efficiently assign incoming computational tasks (containers, batch jobs) to a cluster of physical machines (workers). 

To achieve this, we use **Proximal Policy Optimization (PPO)**—a state-of-the-art Reinforcement Learning algorithm. 

* **The Environment**: A simulated cluster (`TraceReplayEnv`) that faithfully replays historical workloads (e.g., the Alibaba v2018 Trace).
* **The State**: What the neural network "sees". This includes the requirements of the incoming task (CPU, Memory, Urgency) and the current state of every machine in the cluster (Available CPU/Memory, Active Jobs).
* **The Action**: The neural network outputs a probability distribution across all available machines, choosing the best specific machine to assign the task to.
* **The Reward**: A mathematical score given to the model after a decision. If the model packs machines tightly and avoids queuing delays, it gets a positive reward. If it overloads a machine or violates a constraint, it gets penalized.

---

## 2. The "Why": Why Train Offline on Alibaba Data?

Cloud infrastructure is expensive and mission-critical. If we let an untrained Reinforcement Learning agent make decisions on a live, production cluster, it would make catastrophic mistakes (e.g., assigning 500 tasks to a single machine while leaving 10,000 machines idle) while it learns via "trial and error".

**Offline Trace-Driven Training** solves this. By using the Alibaba Trace (which records the exact arrival times, durations, and resource requests of millions of tasks over 8 days), we can build a `TraceReplayEnv` that simulates the real world perfectly.
1. The model can make millions of mistakes safely in the simulator.
2. It explores complex scheduling strategies risk-free.
3. Once the neural network converges on an optimal policy, the resulting model checkpoint (`.pt`) is deployed to the live cluster where it operates with pre-learned expertise.

---

## 3. The "How": The Two-Phase Training Loop

The training process (executed by `train_ppo.py`) operates in a continuous, cyclic loop comprising two distinct phases. This cycle repeats for a configured number of updates (e.g., 200 times).

### Phase 1: The Rollout Phase (Data Collection)
**Hardware Profile:** Extremely CPU-bound. GPU is mostly idle.

During this phase, the algorithm needs to gather "experiences" (States, Actions, and Rewards) to learn from. 
1. The `TraceReplayEnv` reads the next task from the Alibaba dataset.
2. It asks the Neural Network (GPU) to make a prediction (`choose_action`). Because it is asking for a prediction for only **one task** at a time, the GPU processes a batch size of `1`.
3. The CPU calculates the consequences of that action (updating the simulated cluster capacity, calculating the reward).
4. This step repeats sequentially for a specified number of `--rollout-steps` (e.g., 16,384 times).

**The Bottleneck:** The GPU is lightning fast, but the Python code simulating the cluster environment is single-threaded and sequential. The GPU spends 90% of its time waiting for the CPU to finish calculating the cluster state. This is why tools like `nvidia-smi` report low average GPU utilization (~20%) and low memory usage (~200 MiB).

### Phase 2: The PPO Update Phase (Optimization)
**Hardware Profile:** Extremely GPU-bound. CPU is mostly idle.

Once the CPU has collected all 16,384 experiences, Phase 1 stops. The algorithm now enters the optimization phase.
1. The 16,384 experiences are loaded into GPU VRAM.
2. The data is sliced into massive chunks (`--minibatch-size`, e.g., 4096).
3. The GPU performs a forward pass and backpropagation on these 4,096 tasks simultaneously, calculating the gradients and updating the neural network weights to make better decisions in the future.
4. The GPU sweeps over the entire 16,384 experiences multiple times (`--ppo-epochs`, e.g., 15 times).

Because we feed the GPU huge matrices (4,096 tasks × 17,592 machines), the CUDA cores are fully saturated. However, because modern GPUs like the RTX 4060 are so powerful, it finishes these heavy calculations in just 5-10 seconds.

Once Phase 2 finishes, the updated Neural Network is handed back to Phase 1, and the cycle repeats.

---

## 4. Hyperparameter Tuning for Hardware Maximization

To ensure we don't waste the potential of a dedicated GPU, the parameters in our offline training scripts are intentionally pushed beyond standard defaults:

* **`--rollout-steps`**: Scaled up to `16,384`. This forces the CPU to collect a massive dataset before triggering Phase 2, ensuring that when the GPU finally does wake up to optimize, it has a massive payload to chew on.
* **`--minibatch-size`**: Scaled to `4096`. In Deep Learning, larger batches mean higher GPU core saturation. We set this as high as possible without exceeding the 8GB VRAM limit of the RTX 4060.
* **`--ppo-epochs`**: Scaled to `15`. This forces the GPU to iterate over the gathered data more times per cycle, squeezing more learning out of the data before reverting back to the slow CPU data-gathering phase.

### Summary of a Single Update Cycle
1. **CPU Simulation:** ~30-40 seconds to process 16k tasks.
2. **GPU Optimization:** ~5-10 seconds to update weights.
3. **Log Output:** `update=X avg_reward=...` is printed.
4. **Repeat.**

---

## 5. What Parameters Are We Optimizing, How, and Why?

At the core of the PPO (Proximal Policy Optimization) algorithm, we are training a tiny **Neural Network** (less than 20,000 parameters) to serve as our "Scheduling Policy."

### What exactly are we optimizing?
We are optimizing the **weights and biases** of two specific sub-networks:
1. **The Policy Head (Actor):** This outputs the probabilities for picking a specific machine. We are optimizing it so the probabilities of "good" machines increase and "bad" machines decrease.
2. **The Value Head (Critic):** This predicts *how good* the current cluster state is. We optimize it so it can accurately guess whether the cluster is in a healthy or congested state, providing a baseline for the Actor.

### How are we optimizing them?
We optimize these parameters using **Gradient Ascent** combined with **PPO Clipping**.
1. **Collect Data:** As the CPU runs the rollout, we record every action taken and the **Reward** received from the environment.
2. **Calculate Advantage:** The Critic compares the reward received to what it *expected* to receive. If the reward is better than expected, the "Advantage" is positive.
3. **Backpropagation:** The GPU feeds the data through the network. If the Advantage was positive, the Optimizer adjusts the network parameters (weights) to make that specific machine choice *more likely* in the future.
4. **PPO Clipping:** To prevent the model from drastically breaking itself by updating too fast, PPO mathematically "clips" the updates (`--clip-ratio`). This ensures the model learns smoothly and stably.

### Why are we optimizing these parameters?
By allowing the neural network to mathematically tie **Task Features** (like CPU req, Memory req, Urgency) and **Worker Features** (like Available CPU, Current Load) to a single **Reward Signal**, we bypass the need for a human to write complex `if/else` rules. 
Instead of hardcoding "If task requires X memory, put it on machine Y," the model discovers complex, non-linear relationships. It naturally learns to pack resources efficiently, avoid hot-spots, and handle sudden spikes in cluster traffic gracefully—because doing so is the only mathematical way to maximize its reward parameters over time.
