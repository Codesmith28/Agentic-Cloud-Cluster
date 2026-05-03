# Detailed Explanation: Reward, Loss, and Entropy in PPO Scheduler

## 1. REWARD FUNCTION - Task Scheduling Incentives

### Formula (trace_replay_env.py:330-337)
```
R(s,a) = 1.4 + 0.25·H - 0.35·Q_p - 0.55·T_p - 0.20·I_p - 0.40·ΔI - R_q
```

### 1.1 Base Reward (1.4)
**Meaning**: Positive baseline for all feasible placements
**Why**: Ensures that ANY valid placement (worker has enough CPU/memory/storage) yields positive reward. This shapes the learning signal so feasible = good, infeasible = -1.8 (strong negative).

**In Scheduling Context**: 
- If agent places task on worker with 2GB free memory, and task needs 1GB → valid → base +1.4
- Incentivizes the agent to explore feasible placements and refine them

---

### 1.2 Headroom Bonus (0.25·H)
**Formula**: H = 1 - L̄(w) where L̄(w) = min((C_used/C_tot + M_used/M_tot + S_used/S_tot)/3, 1.5)

**Meaning**: Rewards placing on less-loaded workers; penalizes overloaded placements

**How It Works**:
- Empty worker: L̄=0 → H=1.0 → Reward gain: +0.25 (full bonus)
- 50% loaded: L̄=0.5 → H=0.5 → Reward gain: +0.125 (half bonus)
- Overloaded (L̄=1.2): H=-0.2 → Reward loss: -0.05 (penalty)

**In Scheduling**: Encourages the agent to keep workers below capacity. If the cluster is idle, placing on empty workers is rewarded. If cluster is congested, spreads load to available workers.

---

### 1.3 Queue Pressure (0.35·Q_p)
**Formula**: Q_p = min((q_wait + t_run·L̄(w))/(k·t_run), 3.0)

**Breakdown**:
- q_wait: baseline queue time from Alibaba trace
- t_run: estimated task runtime
- L̄(w): worker normalized load
- k: SLA multiplier (1.5-2.5, e.g., k=2 means "2× runtime is acceptable")
- kSLA_budget = k·t_run (total time budget for the task)

**Meaning**: Ratio of estimated wait time to SLA budget. Capped at 3.0 for stability.

**Example**:
- Task: t_run=10s, k=2.0 → SLA_budget = 20s
- Worker: L̄=0.5, q_wait=3s (from trace)
- q_proxy = 3 + 10·0.5 = 8s (queue + running under load)
- Q_p = min(8/20, 3) = 0.4 → penalty: -0.35·0.4 = -0.14
- If we placed on L̄=0.9 worker: q_proxy=12s → Q_p=0.6 → penalty: -0.21 (worse)

**In Scheduling**: Directly targets average turnaround. Lower queue pressure → faster completion → agent learns to avoid congested paths.

---

### 1.4 Tail Pressure (0.55·T_p) — HIGHEST WEIGHT
**Formula**: T_p = max((q_wait + 2·t_run·L̄(w))/(k·t_run) - 1, 0)

**Breakdown**:
- Activated ONLY when turnaround > SLA_budget (when T̄AT/SLA > 1.0)
- Factor 2× on load (compounding: waiting + running under load)
- Subtracting 1 means penalties kick in when exceeding SLA

**Meaning**: Extreme penalty for SLA violations, especially for P95 tail latency

**Example**:
- Same task, same budget (20s), but worse placement
- q_proxy = 12s, t_run = 10s → turnaround = 22s (exceeds 20s budget!)
- T_p = max((22/20) - 1, 0) = max(0.1, 0) = 0.1
- Penalty: -0.55·0.1 = -0.055

**Why Weight 0.55 (Highest)**:
- This is what makes PPO beat RR and RTS!
- PPO learned that preventing P95 violations is more valuable than avg optimization
- RR/RTS optimize average; PPO learned to optimize tail

---

### 1.5 Imbalance Penalty (0.20·I_p)
**Formula**: I_p = max(L̄(w) - μ_cluster, 0)

where μ_cluster = (1/N)·Σ L̄(w_i) = average load across all workers

**Meaning**: Only penalizes placing on workers ABOVE cluster average. Rewards choosing below-average workers.

**Example**:
- Cluster has 3 workers: L̄=[0.3, 0.6, 0.9]
- μ_cluster = 0.6
- Placing on worker 1 (0.3): I_p = max(0.3-0.6, 0) = 0 (no penalty, reward!)
- Placing on worker 3 (0.9): I_p = max(0.9-0.6, 0) = 0.3 → penalty: -0.06

**In Scheduling**: Discourages hot-spotting. If worker-3 already has lots of load, don't pile on more.

---

### 1.6 Delta Imbalance (0.40·ΔI) — SECOND HIGHEST WEIGHT
**Formula**: ΔI = σ_loads^after - σ_loads^before

**Meaning**: Change in load distribution spread (standard deviation)

**Why Different from I_p**:
- I_p penalizes absolute imbalance (hot worker)
- ΔI penalizes CHANGE in imbalance (whether action worsens balance)
- Agent is only responsible for what IT causes, not pre-existing conditions

**Example**:
- Before: loads=[0.2, 0.5, 0.8] → σ=0.25
- Place task on worker-1: loads=[0.4, 0.5, 0.8] → σ=0.17 → ΔI=-0.08 (GOOD, reward +0.032)
- Place task on worker-3: loads=[0.2, 0.5, 0.95] → σ=0.32 → ΔI=+0.07 (BAD, penalty -0.028)

**Weight 0.40 means**: Cluster balance is second-most important objective after P95.

---

### 1.7 Requeue Penalty (R_q)
**Formula**: R_q = min(n_requeue, 4) × 0.05

**Meaning**: Small penalty for tasks already been rescheduled (n_requeue > 0)

**Example**:
- Fresh task: n_requeue=0 → R_q=0 (no penalty)
- Task already failed once: n_requeue=1 → R_q=0.05
- Task failed 3 times: n_requeue=3 → R_q=0.15
- Capped at 4×0.05=0.2 (don't over-penalize)

**Why**: Discourages the agent from placing on workers/task-combinations with history of failure.

---

### 1.8 Reward Range
- **Best case**: 1.4 + 0.25(1.0) = **1.65** (empty worker, no SLA risk, perfect balance)
- **Typical good**: 1.2 - 1.6 (what training report shows: avg_reward≈1.56)
- **Feasible but poor**: 0.3 - 0.8 (overloaded, risky)
- **Infeasible**: **-1.8** (hard constraint violation - worker can't fit task)

---

## 2. LOSS FUNCTION - What PPO Optimizes

### Total Loss (model.py:307)
```python
loss = policy_loss + (value_coeff * value_loss) - (entropy_coeff * entropy)
```

or expanded:
```
L(θ) = L^CLIP(θ) + c_1·L^VF(θ) - c_2·H[π_θ]
```

where c_1=0.5, c_2=0.01

---

### 2.1 Policy Loss: Clipped Surrogate Objective (model.py:294-297)

**Formula**:
```
L^CLIP = -E_t[ min(r_t(θ)·Â_t, clip(r_t(θ), 1-ε, 1+ε)·Â_t) ]
```

where:
- r_t(θ) = exp(log π_new - log π_old) = ratio of new/old policy
- Â_t = advantage (how much better than baseline?)
- ε = 0.2 (clip range: keep ratio in [0.8, 1.2])
- min() selects the worse outcome (conservative)

**Why Clipped Surrogate**?
Traditional policy gradient can overfit when taking large steps. PPO clips the ratio to prevent:
- Taking huge policy steps that destroy learning progress
- Overshooting good parameters

**Code Breakdown**:
```python
ratio = torch.exp(new_log_probs - old_log_probs)
# ratio ≈ π_new(a|s) / π_old(a|s)
# e.g., if new policy is 2× more likely → ratio=2.0

surrogate_1 = ratio * advantages
# Unclipped: if advantage=+0.5, surrogate_1 = 2.0 × 0.5 = 1.0

surrogate_2 = torch.clamp(ratio, 1.0 - 0.2, 1.0 + 0.2) * advantages
# Clipped: ratio clamped to [0.8, 1.2], so at most 1.2 × 0.5 = 0.6

policy_loss = -torch.min(surrogate_1, surrogate_2).mean()
# min(1.0, 0.6) = 0.6 (conservative choice)
# Negative because we're minimizing loss (ascending policy)
```

**In Scheduling**:
- If agent found a placement that works well (advantage > 0), increase its probability
- But don't increase TOO much (clipping prevents runaway)
- If placement was bad (advantage < 0), decrease its probability
- But don't decrease too aggressively either

---

### 2.2 Value Loss: Critic Learning (model.py:299-305)

**Formula**:
```
L^VF = max( (V_θ - V_target)², (V_clipped - V_target)² )
V_clipped = V_old + clip(V_θ - V_old, -ε_v, +ε_v)
```

where ε_v = 0.2

**Meaning**: Trains the value network V(s) to predict cumulative future reward

**Why Two Terms**?
- Unclipped: Direct MSE between prediction and target (return)
- Clipped: Conservative update, prevents large value estimate shifts

The max() chooses whichever loss is larger—ensures we don't over-update.

**Example**:
- State s: Cluster has 10 idle workers, incoming task
- Old V_old(s) = 0.5 (predicted return: 0.5)
- Actual returns during rollout: G_t = 1.8 (experienced actual reward ~1.8)
- New V_θ(s) = 1.7 (updated prediction)
- V_target = 1.8 (what we want to predict)

Unclipped loss: (1.7 - 1.8)² = 0.01
Clipped V: 0.5 + clip(1.7 - 0.5, -0.2, 0.2) = 0.5 + 0.2 = 0.7
Clipped loss: (0.7 - 1.8)² = 1.21
max(0.01, 1.21) = 1.21 (use clipped to prevent over-update)

**In Scheduling**: Value function learns baseline expectations. "If I see this cluster state, what's the average reward I'll get?" Helps compute advantages (actual_reward - baseline).

---

### 2.3 Entropy Regularization (model.py:292)

**Formula**:
```
H[π] = -E_a[π(a|s) log π(a|s)]
```

**Meaning**: Measure of policy randomness. Higher entropy = more uniform distribution = more exploration.

**Code**:
```python
distribution = Categorical(logits=logits)
# logits: [1.2, 0.3, -0.5] (3 workers)
# π ≈ softmax = [0.65, 0.30, 0.05] (mostly worker-0, some worker-1, little worker-2)

entropy = distribution.entropy().mean()
# H ≈ -[0.65·log(0.65) + 0.30·log(0.30) + 0.05·log(0.05)]
# H ≈ 0.85 (moderate randomness)

# If logits=[5.0, 0, 0] → π=[0.993, 0.0035, 0.0035] → H≈0.02 (deterministic)
# If logits=[0, 0, 0] → π=[0.33, 0.33, 0.33] → H≈1.10 (uniform, max randomness)
```

**Total Loss with Entropy**:
```
loss = policy_loss + 0.5·value_loss - 0.01·entropy
             ↑              ↑                  ↑
         maximize      minimize          maximize
         (negate)     gradients       (explore!)
```

---

## 3. WHY ENTROPY MATTERS FOR SCHEDULING

### 3.1 Without Entropy (entropy_coeff = 0)

Early training: Agent finds any placement that works → gets greedy
- Logits: [3.0, -2.0, -1.5] (heavily favor worker-0)
- π = [0.93, 0.05, 0.02] (almost always pick worker-0)
- Entropy ≈ 0.25 (very focused)

**Problem**: Agent never explores worker-1 or worker-2 unless reward explicitly guides it. If worker-0 happens to have a good average turnaround ONCE, agent locks in and ignores better placements later.

### 3.2 With Entropy (entropy_coeff = 0.01)

During training, the -0.01·entropy term becomes:
```
loss = ... - 0.01·(-entropy) = ... + 0.01·H
```

This ENCOURAGES higher entropy (more uniform distribution).

- Early: logits = [3.0, -2.0, -1.5], H=0.25 → loss += 0.01·0.25 = +0.0025
- Encourages: logits = [2.0, 0.5, 0.0], H=1.05 → loss += 0.01·1.05 = +0.0105 (higher entropy better)

**Result**: Agent explores all workers, finds which ones are good for different task types.

---

### 3.3 In the Scheduling Problem

**Task Types**: cpu-light, cpu-heavy, memory-heavy, mixed

Different workers have different capabilities:
- Worker-small: Good for cpu-light, bad for memory-heavy
- Worker-large: Good for memory-heavy, bad for thin tasks (wastes capacity)
- Worker-medium: OK for everything

**With Low Entropy**: Agent might decide "place everything on worker-large" (simple strategy, works OK).

**With High Entropy**: Agent explores:
- cpu-light tasks → worker-small (better)
- memory-heavy → worker-large (matches need)
- mixed → worker-medium (balanced)

Training report shows **entropy↓** and **loss↑** as training progresses = agent becomes more focused (converged policy).

---

## 4. COMPLETE TRAINING LOOP

```
Step 1: Rollout (collect experience)
  For each task, agent selects worker using π_old
    → Get placement, task starts/completes
    → Calculate reward R(s,a) from reward function
    → Store (s, a, R, V_old, log π_old)

Step 2: Compute Returns & Advantages
  G_t = R_t + γ·V(s_{t+1})  (discounted cumulative reward)
  Â_t = G_t - V(s_t)         (how much better than expected?)
  Normalize: Â_t ← (Â_t - mean) / (std + ε)

Step 3: PPO Update (repeat for 6 epochs)
  Mini-batch gradient descent:
    Forward: logits, V = π_θ(s), V_θ(s)
    Compute:
      r = exp(log π_new - log π_old)
      policy_loss = -min(r·Â, clip(r)·Â)
      value_loss = max((V - G)², (V_clip - G)²)
      entropy = -π·log(π)
      
    Total: L = policy_loss + 0.5·value_loss - 0.01·entropy
    
    Backprop: θ ← θ - α·∇L (α=3e-4, annealed)
    Grad clip: ||∇|| ≤ 1.0 (prevent exploding gradients)

Step 4: Repeat
  Continue for 200 updates (200 × 1024 steps ≈ 200K Alibaba tasks)
```

---

## 5. HYPERPARAMETER CHOICES & WHY

| Param | Value | Why |
|-------|-------|-----|
| γ | 0.99 | High discounting: future rewards matter (scheduling is long-term) |
| λ (GAE) | 0.95 | Trust value function estimates (critic is well-trained) |
| clip_ratio ε | 0.2 | Conservative: 80-120% of old policy (don't jump around) |
| value_coeff | 0.5 | Balances actor vs critic learning |
| entropy_coeff | 0.01 | SMALL: encourage exploration but don't go random |
| epochs | 6 | Multiple passes on same rollout data (efficient) |
| minibatch | 256 | Stable gradients (not too small) |
| LR | 3e-4 | Standard for PyTorch, annealed to 0 |

---

## 6. FINAL INSIGHT: How Reward + Loss + Entropy Align

**Reward function** encodes scheduling objectives:
- Tail pressure (0.55): "P95 is most important"
- Delta imbalance (0.40): "Balance matters"
- Queue pressure (0.35): "Latency matters"

**Loss function** optimizes policy to maximize expected reward:
- Clipping prevents overshooting
- Value function bootstraps learning (faster convergence)
- Entropy prevents premature convergence to suboptimal policy

**Together**: Agent learns to reliably pick placements that are:
1. Feasible (hard constraint)
2. Low tail-latency risk (high weight in reward)
3. Balancing cluster load (delta imbalance reward)
4. Not unnecessarily exploitative (entropy keeps exploring)

Result: **8.7-16.1% better than heuristics** ✓
