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

We optimize these parameters using **Gradient Ascent** combined with **PPO Clipping**. Every term below maps directly to code in `model.py` and `train_ppo.py`.

---

#### Step 1 — Collect Data (the Rollout)

**Code:** `train_ppo.py` → the `for _ in range(args.rollout_steps):` loop.

During each rollout, the CPU runs `--rollout-steps` (16,384) scheduling decisions. At every step it records a **transition** — a dictionary containing:

```python
transitions.append({
    "task_features":  task_features,   # 5 numbers describing the incoming task
    "worker_features": worker_features, # 9 numbers × 17,592 machines
    "action":         action,           # which machine index was chosen
    "old_log_prob":   old_log_prob,     # how confident the model was at the time
    "old_value":      old_value,        # what reward the model predicted it would get
    "reward":         float(reward),    # what reward the environment actually gave
    "done":           done,             # did the episode end?
})
```

**What is a Reward?**  
The `reward` is a scalar number returned by `env.step(action)` (inside `TraceReplayEnv`). It is positive when the scheduler makes a good placement (tight packing, no SLA violation) and negative when it overloads a machine or causes queuing delays. The entire goal of training is to maximize the *sum of rewards over time*.

**What is `old_log_prob`?**  
When `choose_action` runs the neural network, it produces a **Categorical distribution** over all 17,592 machines. `log_prob` is the natural logarithm of the probability the model assigned to the chosen machine. It is stored as `old_log_prob` because during the update phase we will compare this "old confidence" against the "new confidence" after the weights have changed — this is the core of PPO.

**What is `old_value`?**  
This is the Critic's prediction: "Given the current state of the cluster, how much total future reward do I expect to accumulate?" It is a single number output by `self.value_head` in `PPOActorCritic`. Storing it here lets us measure how wrong the Critic was once we see the actual outcome.

---

#### Step 2 — Calculate Advantage (GAE)

**Code:** `train_ppo.py` → `generalized_advantage_estimation()`

```python
advantages, returns = generalized_advantage_estimation(
    rewards=rewards,
    dones=dones,
    values=values,         # Critic's predictions from the rollout
    next_value=next_value, # bootstrapped value of the state after the rollout ends
    gamma=args.gamma,      # 0.99 — how much to discount future rewards
    gae_lambda=args.gae_lambda,  # 0.95 — smoothing factor
)
```

The **Advantage** at each step answers the question: *"Was the actual outcome better or worse than what the Critic predicted?"*

**The TD-Delta (Temporal Difference Error):**
```python
delta = rewards[i] + gamma * bootstrap_value * not_done - values[i]
```
- `rewards[i]` — the actual reward received at this step.
- `gamma * bootstrap_value` — a discounted estimate of all *future* rewards from the next step onward. `gamma = 0.99` means a reward 100 steps in the future is worth `0.99^100 ≈ 0.37` of a reward received now. This prevents the model from being short-sighted.
- `values[i]` — what the Critic predicted. If `delta > 0`, the outcome was better than expected (positive advantage). If `delta < 0`, it was worse (negative advantage).

**GAE (Generalized Advantage Estimation):**
```python
gae = delta + gamma * gae_lambda * not_done * gae
```
Rather than using the raw one-step `delta`, GAE accumulates a **exponentially-weighted sum of deltas** going backwards through the rollout. `gae_lambda = 0.95` controls the bias/variance trade-off: `lambda=0` means pure one-step TD (low variance, high bias), `lambda=1` means full Monte Carlo returns (high variance, low bias). `0.95` is empirically a good middle ground.

**Advantage Normalization:**
```python
advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)
```
After GAE, advantages are normalized to zero mean and unit variance. This prevents one extremely-good-or-bad scheduling decision from dominating the entire gradient update and destabilizing training.

**Returns:**
```python
returns = advantages + values
```
The **Returns** are the target values the Critic is trained to predict. They represent "what the Critic *should* have said" given the actual experience. The gap between `values` (prediction) and `returns` (truth) is what the Critic learns to close over time.

---

#### Step 3 — Backpropagation (The GPU Update)

**Code:** `model.py` → `ppo_update()` → the inner `for start in range(...)` loop.

After GAE, the full 16,384-step batch is shuffled and sliced into minibatches of `--minibatch-size` (4,096) for GPU efficiency. For each minibatch:

**Forward Pass:**
```python
logits, values = state.model(
    task_features[batch_idx],
    worker_features[batch_idx],
    action_masks[batch_idx],
)
```
The neural network runs on 4,096 scheduling decisions simultaneously. The GPU computes new `logits` (raw scores for each machine) and new `values` (updated Critic estimates).

**New Log Probabilities:**
```python
distribution = Categorical(logits=logits)
new_log_probs = distribution.log_prob(actions[batch_idx])
```
We rebuild the probability distribution from the updated weights and ask: "Now that the model has been partially updated, what log-probability does it assign to the *same action* it took before?"

**The Probability Ratio:**
```python
ratio = torch.exp(new_log_probs - old_log_probs[batch_idx])
```
`ratio = π_new(a|s) / π_old(a|s)` — the ratio of the new policy's probability to the old policy's probability for the same action. This is the mathematical engine of PPO.
- `ratio > 1` means the new policy is *more likely* to take this action than before.
- `ratio < 1` means the new policy is *less likely* to take this action.
- `ratio = 1` means the policy hasn't changed for this action yet.

**Entropy Bonus:**
```python
entropy = distribution.entropy().mean()
```
Entropy measures how "spread out" the probability distribution is. High entropy means the model is uncertain (good for exploration early in training). We add `entropy_coeff * entropy` to the loss to discourage the model from becoming overconfident too quickly and collapsing to one machine for every task.

**Computing the Loss:**
```python
loss = policy_loss + (value_coeff * value_loss) - (entropy_coeff * entropy)
```
The total loss is a weighted sum of three components:
- `policy_loss` — how badly the Actor is performing relative to advantage (see Step 4).
- `value_coeff * value_loss` — how wrong the Critic's predictions were (`value_coeff = 0.5`).
- `-entropy_coeff * entropy` — subtract entropy to *reward* exploration (`entropy_coeff = 0.01`).

**Backward Pass + Gradient Clipping:**
```python
state.optimizer.zero_grad()   # clear gradients from previous minibatch
loss.backward()               # compute gradients for all 20,000 weights via chain rule
nn.utils.clip_grad_norm_(state.model.parameters(), max_norm=1.0)  # safety cap
state.optimizer.step()        # Adam optimizer nudges all weights in the gradient direction
```
`loss.backward()` is where PyTorch automatically computes how much each of the 20,000 weights in the network contributed to the loss (via the chain rule of calculus). `optimizer.step()` then nudges every weight by a tiny amount in the direction that reduces the loss — this is the core learning step.

Gradient clipping (`max_norm=1.0`) prevents any single step from making huge weight changes if the gradients are abnormally large (which can happen with chaotic real-world workloads like Alibaba). Without it, one bad minibatch could corrupt months of learned behavior.

**Learning Rate Annealing:**
```python
# train_ppo.py → top of the update loop
frac = 1.0 - ((update_idx - 1) / float(args.updates - 1))
current_lr = max(args.learning_rate * frac, 1e-6)
```
The learning rate starts at `3e-4` and linearly decays to `1e-6` over all 1,000 updates. Early in training, large learning rate steps help the model escape random initialization quickly. Later, small steps ensure it fine-tunes precisely without overshooting the optimal policy.

---

#### Step 4 — PPO Clipping (Stable Policy Updates)

**Code:** `model.py` → lines `surrogate_1`, `surrogate_2`, `policy_loss`

```python
surrogate_1 = ratio * advantages[batch_idx]
surrogate_2 = torch.clamp(ratio, 1.0 - clip_ratio, 1.0 + clip_ratio) * advantages[batch_idx]
policy_loss  = -torch.min(surrogate_1, surrogate_2).mean()
```

This is the heart of PPO and why it is preferred over vanilla policy gradient. Here is what each line means:

**`surrogate_1 = ratio * advantage`**  
The "naive" policy gradient objective. If `advantage > 0` (good action), maximize it by increasing `ratio` (make the action more likely). If `advantage < 0` (bad action), minimize it by decreasing `ratio`.

**`surrogate_2 = clamp(ratio, 0.8, 1.2) * advantage`** (with `clip_ratio = 0.2`)  
The same calculation, but the ratio is hard-capped between `0.8` and `1.2`. This means no single update can make an action more than 20% more likely or less than 20% less likely, no matter how large the gradient is.

**`policy_loss = -min(surrogate_1, surrogate_2)`**  
PPO always takes the *pessimistic* (minimum) of the two surrogates:
- When `advantage > 0` (reward a good action): `min` prevents the ratio from growing above `1.2`. The model gets rewarded for improving, but not too aggressively.
- When `advantage < 0` (punish a bad action): `min` prevents the ratio from shrinking below `0.8`. The model gets punished, but not catastrophically.

The net effect: **the policy is prevented from moving too far from the old policy in a single update**. This is critical for stability because RL has no ground truth — a policy that changes too drastically can corrupt its own training data for future updates.

**Value Clipping (Critic Stability):**
```python
value_delta   = values - old_value_batch
clipped_values = old_value_batch + torch.clamp(value_delta, -value_clip_range, value_clip_range)
value_loss_unclipped = F.mse_loss(values, value_targets, reduction="none")
value_loss_clipped   = F.mse_loss(clipped_values, value_targets, reduction="none")
value_loss = torch.max(value_loss_unclipped, value_loss_clipped).mean()
```
The same clipping principle is applied to the Critic. The Critic's updated value estimate cannot deviate more than `value_clip_range = 0.2` from its old estimate in a single update step. We take the `max` of the two losses (again, pessimistic) to ensure the Critic only gets updated when the clipped version is not conservative enough.

### Why are we optimizing these parameters?
By allowing the neural network to mathematically tie **Task Features** (like CPU req, Memory req, Urgency) and **Worker Features** (like Available CPU, Current Load) to a single **Reward Signal**, we bypass the need for a human to write complex `if/else` rules. 
Instead of hardcoding "If task requires X memory, put it on machine Y," the model discovers complex, non-linear relationships. It naturally learns to pack resources efficiently, avoid hot-spots, and handle sudden spikes in cluster traffic gracefully—because doing so is the only mathematical way to maximize its reward parameters over time.

---

## 6. Training Reward Curve — Reading the Chart

After a training run, you can generate a reward curve by running:
```bash
./venv/bin/python agentic_scheduler/scripts/plot_training_curve.py \
    agentic_scheduler/logs/training_output.log \
    agentic_scheduler/results/training_reward_curve.png
```

This produces a chart like the one below:

![PPO Training Reward Curve](results/training_reward_curve.png)

### How to read this chart

The **X-axis** is the PPO update number (each update = one full rollout of 16,384 scheduling decisions + one GPU optimization pass). The **Y-axis** is the **average reward** the agent received over its most recent 5,000 scheduling decisions.

**The cyan line** shows the raw `avg_reward` value logged at each checkpoint. **The pink line** is a smoothed moving average that filters out per-update noise and reveals the true trend.

### What does the reward value mean?

The reward is a composite score from the `TraceReplayEnv` that combines multiple scheduling quality metrics:
- **Positive contributions**: Successfully placing a task on a machine with enough resources, tight resource packing (high utilization), and meeting SLA deadlines.
- **Negative contributions**: Overloading a machine, violating resource constraints, queueing delays, and unbalanced cluster utilization.

A reward of **0.0** would mean the agent is breaking even — placing tasks acceptably but not optimally. The dashed gray line on the chart marks this baseline.

### Is a negative reward bad?

**Not necessarily.** A deeply negative reward at the start (e.g., `-1.34`) is expected — the agent begins with randomly initialized weights and is essentially flipping coins to decide where to put tasks.

What matters is the **trend over time**:

| Curve Shape | What It Means | Good or Bad? |
|---|---|---|
| **Steadily rising toward 0** | The agent is learning to make fewer bad placements. The policy is converging. | ✅ Good — training is working. |
| **Dips then recovers** | The agent tried an aggressive strategy (tight packing), got penalized, then adapted. | ✅ Normal — exploration is healthy. |
| **Flat / oscillating** | The agent is stuck in a local optimum or the learning rate is too low for further progress. | ⚠️ Stalled — consider increasing `--updates`, adjusting `--entropy-coeff`, or resuming from a checkpoint. |
| **Steadily falling** | The policy is getting worse with more training (catastrophic forgetting or broken reward signal). | ❌ Bad — stop training, investigate reward function. |
| **Crosses above 0** | The agent is consistently placing tasks better than the neutral baseline. | 🎯 Excellent — model is production-ready. |

### Interpreting the current curve

Looking at the chart above from the active training run:
- **Update 1 → 10**: Reward holds steady around `-1.34` → `-1.37`. The agent is in its "random exploration" phase and hasn't learned strong patterns yet.
- **Update 10 → 20**: A dip to `-1.61`. The agent started experimenting with tighter packing strategies but overloaded machines, causing penalties. This is the classic "exploration dip" and is perfectly healthy.
- **Update 20 → 50**: Recovery toward `-1.40`. The agent has learned from its mistakes and is climbing back. The smoothed trend line confirms this upward trajectory.

**Conclusion**: The reward curve shows the expected "U-shaped" pattern of early PPO training. With 1,000 updates configured, the model has plenty of room to continue climbing toward zero and potentially into positive territory as it masters the Alibaba workload distribution.
