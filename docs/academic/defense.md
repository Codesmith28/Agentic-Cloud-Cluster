# Viva Voce Defense — Agentic Cloud Cluster Project

This document answers all 27 viva questions using current repository code and project artifacts.  
Where evidence is missing, I state that directly.

---

## Section 1: High-Level Project & Architecture (Systems View)

### 1) Core Claim: Why is this a framework, not just an application? Extension points for new scheduler/executor backend.

**Answer**

It is a **framework** because it exposes reusable control-plane abstractions and pluggable scheduling/runtime pipelines rather than one fixed execution path:

1. **Scheduler plugin contract is explicit** via `Scheduler` (`SelectWorker`, `GetName`, `Reset`) plus optional `OutcomeReporter` for online feedback (`ReportOutcome`) in `master/internal/scheduler/scheduler.go`.
2. **Runtime wiring is compositional** in `master/main.go`: RR → RTS → PPO stack is assembled and selected by config (`SCHED_ALGO`, PPO mode).
3. **Multiple ingress surfaces** (CLI, HTTP API, Web UI) all converge to the same master queue/scheduler path (`SubmitTask`), so scheduling logic is reusable regardless of client.

**Adding a new scheduler (exact extension path)**

1. Implement `Scheduler` in `master/internal/scheduler/<new>.go`.
2. Optionally implement `OutcomeReporter` if it needs post-execution learning.
3. Register it in `master/main.go` scheduler selection and optional HTTP scheduler registry.
4. Master uses it automatically through `MasterServer.SetScheduler(...)` and `selectWorkerForTask(...)`.

**Adding a new executor backend**

There is **no first-class executor interface yet**. Worker server is tightly coupled to concrete `*executor.TaskExecutor` (`worker/internal/server/worker_server.go`).  
So adding a non-Docker backend currently requires refactoring worker server to depend on an executor interface (execute/cancel/stream logs/status) and then providing backend implementations.

**References**  
`master/internal/scheduler/scheduler.go:10-40`  
`master/main.go:235-353`  
`master/internal/server/master_server.go:2330-2357`  
`worker/internal/server/worker_server.go:23-49`

---

### 2) Exact task data flow from submission to result + online PPO update, protocols, persistence, SPOFs.

**Answer**

**End-to-end flow**

1. **Submission**
   - CLI: `controlplane.Executor cmdSubmitTask` → `MasterServer.SubmitTask`.
   - HTTP/Web UI: `POST /api/tasks` (`TaskAPIHandler.HandleCreateTask`) → `MasterServer.SubmitTask`.
2. **Queueing**
   - `SubmitTask` persists task (if DB enabled), enriches SLA metadata, enqueues task in in-memory queue.
3. **Scheduling decision**
   - Background queue loop (`processQueue`, every 5s) snapshots queued tasks.
   - `selectWorkerForTask` converts master worker states to scheduler `WorkerInfo` and calls active scheduler (RR/RTS/PPO).
4. **Assignment**
   - `assignTaskToWorker` checks active worker + resource feasibility, **reserves resources under lock**, persists attempt/assignment, dials worker gRPC and calls `AssignTask`.
5. **Worker execution**
   - Worker validates task, tracks it in monitor, executes container via Docker executor, streams logs, collects outputs.
6. **Result reporting**
   - Worker sends `ReportTaskCompletion` (gRPC), optionally uploads output files via streaming `UploadTaskFiles`.
   - Master processes completion idempotently, handles stale/late attempts, releases resources, updates task/attempt/result DB state.
7. **Online PPO update**
   - Master asynchronously computes outcome reward and calls scheduler `ReportOutcome`.
   - PPO scheduler forwards `ReportOutcome` gRPC to Python service.
   - PPO service matches pending decision, appends replay sample; when `replay_buffer >= update_batch_size` (default 32), runs PPO mini-update and persists model.

**Protocols**

- **Master ↔ Worker:** gRPC (`RegisterWorker`, `SendHeartbeat`, `AssignTask`, `ReportTaskCompletion`, `UploadTaskFiles`, log streaming) via `proto/master_worker.proto`.
- **Master ↔ PPO service:** gRPC (`Ping`, `LoadModelForFingerprint`, `SelectWorker`, `ReportOutcome`) via `proto/ppo_scheduler.proto`.
- **Client ↔ Master:** HTTP REST + WebSocket + CLI.

**Persistence layers**

- Task/worker/attempt/assignment/result/file metadata collections via master DB handlers.
- PPO model metadata + binaries in Mongo `SCHEDULER_MODELS` + GridFS `scheduler_models`.
- File artifacts on master filesystem storage service.

**Single points of failure**

1. **Master is a single stateful coordinator** (no leader replica in current design).
2. **Active PPO service** can fail, but scheduler has fallback; so it is not a hard SPOF for dispatch when fallback mode is available.
3. MongoDB is optional for runtime, but persistence/recovery quality degrades without it.

**References**  
`master/internal/controlplane/executor.go:247-252,716-852`  
`master/internal/http/task_handler.go:77-165`  
`master/internal/server/master_server.go:1703-1758,2193-2327,2330-2357,2553-2792,908-1190,1233-1288`  
`worker/internal/server/worker_server.go:137-249,466-543`  
`proto/master_worker.proto:7-19,83-137`  
`proto/ppo_scheduler.proto:9-22,50-95`  
`agentic_scheduler/service.py:281-330,373-456`  
`ARCHITECTURE.md:249-251`

---

### 3) Host-master + Docker workers topology: discovery/registration in production, NAT/firewalls, master restart behavior.

**Answer**

**Discovery/registration model (current)**

- Workers are **not auto-discovered**.
- Admin pre-registers worker endpoint (`worker_id`, `ip:port`) via CLI/API.
- Master calls worker `MasterRegister`; worker then calls back `RegisterWorker` with capacities and starts heartbeats.
- Master intentionally preserves admin-configured endpoint if worker reports a different address.

So across NAT/firewalls, deployment depends on **operator-provided routable addresses and open gRPC ports**; there is no built-in NAT traversal, mTLS identity exchange, service discovery bus, or reverse-tunnel mechanism.

**Master restart behavior**

1. On startup, master loads workers from DB and restores queued/pending/running tasks (`RestoreQueuedTasks`).
2. It starts queue processor and reconnection monitor.
3. It broadcasts `MasterRegister` to known workers (`BroadcastMasterRegistration`).
4. If workers are inactive/unreachable, reconnection monitor retries every 5s.
5. For stranded running tasks, recovery marks attempts lost, releases resources, deletes assignment, and requeues logical task.

**References**  
`master/internal/server/master_server.go:483-515,766-851,793-807,2378-2466,517-535,684-735,611-664`  
`master/main.go:434-453,598-600`  
`worker/internal/server/worker_server.go:51-69,72-135`  
`master/README.md:74-85`

---

### 4) Resource accounting: CPU/memory/storage tracking, source of truth, double-booking/leak prevention, benchmark bug fixes.

**Answer**

Resource accounting is a **hybrid authoritative model**:

1. **Master in-memory worker state** is the scheduling fast-path (`Allocated*`, `Available*`).
2. **WorkerDB** is durable state and reconciliation source.
3. **Worker heartbeat telemetry** contributes live usage signals (`LatestCPU/Memory/Storage`) but does not directly bypass reservation logic.

**Double-booking prevention**

- In `assignTaskToWorker`, master checks availability then **reserves resources under lock before RPC** and rolls back reservation on failure/negative ack.

**Leak prevention**

- On completion: release resources in-memory + DB.
- Fallback cache (`taskResourceCache`) exists so release still works if taskDB lookup is unavailable.
- Reconciliation routines (`reconcileSingleWorker`, `ReconcileWorkerResources`) and recovery path (`recoverWorkerTasksLocked`) correct stale allocations after failures.

**Benchmark-related bugs fixed (documented)**

- Missing `taskResourceCache` caused resource leaks without DB (`BENCHMARK_RESULTS` bug #7).
- Worker visibility/registration bugs and typed-nil DB bug also impacted scheduling correctness.

**References**  
`master/internal/server/master_server.go:2553-2671,2699-2758,970-1007,926-931,440-481,569-609,611-664`  
`docs/BENCHMARK_RESULTS.md:173-193`

---

## Section 2: Systems Programming & Concurrency

### 5) Concurrency model in master: goroutines and synchronization primitives.

**Answer**

**Typical goroutine structure (master runtime)**

Base always-on goroutines:

1. main/CLI or TUI loop
2. gRPC server serve loop
3. HTTP server loop
4. queue processor goroutine (5s ticker)
5. worker reconnection monitor goroutine (5s ticker)
6. telemetry inactivity checker goroutine
7. signal/shutdown handler goroutine
8. optional AOD training ticker goroutine

Per-worker:

9. **one telemetry processing goroutine per worker** (`TelemetryManager.RegisterWorker`).

Per-event ephemeral goroutines:

- async scheduler outcome report
- async master-notify register/reconnect calls

So active goroutines scale roughly as: **O(workers) + base services**.

**Synchronization primitives used**

- `sync.RWMutex` and `sync.Mutex` heavily in master state maps and queues.
- Channels for queue stop/reconnect stop and telemetry heartbeat fan-in.
- `sync.WaitGroup` for queue worker and telemetry manager lifecycle.
- `sync.Once` in log broadcaster startup.

Concrete examples:

- Master worker map lock: `MasterServer.mu`.
- Queue lock + in-flight cancellation maps: `queueMu`, `processingTasks`, `cancellationRequests`.
- Telemetry manager uses `mu` + per-worker buffered channels (non-blocking drop on full).
- Round-robin scheduler uses mutex around rotating index.

**References**  
`master/internal/server/master_server.go:39-79,2158-2190,2193-2327`  
`master/internal/telemetry/telemetry_manager.go:23-47,74-101,125-149,151-179,259-318`  
`master/internal/scheduler/round_robin.go:26-43`  
`worker/internal/logstream/log_broadcaster.go:39-44,80-82`

---

### 6) Memory safety/buffers/log streaming, truncation/upload behavior, race detector/profiler findings.

**Answer**

**Log/output buffering safeguards**

- Final collected container logs are hard-capped at **10 MB** (`maxLogBytes`), with truncation marker.
- Live log broadcaster keeps only last **1000 lines** per task.
- Subscriber channels are buffered (100) and slow subscribers are skipped (non-blocking broadcast) to avoid producer stalls.
- Telemetry heartbeat channels are buffered (10); extra heartbeats are dropped when full.

**File upload**

- Worker uploads outputs in **1 MB chunks** over gRPC stream.

**Memory growth risk**

- Unbounded growth is largely constrained for logs/telemetry; main remaining risk is high concurrency with many active tasks/subscribers, but ring buffers and caps reduce worst-case behavior.

**Race detector/profiler evidence**

- I found **commands documented** (e.g., `go test -race`) but no committed run evidence/results proving race detector or memory profiler findings in repo artifacts.
- So the precise finding is: **not evidenced in artifacts I reviewed**.

**References**  
`worker/internal/executor/executor.go:29-32,352-381`  
`worker/internal/logstream/log_broadcaster.go:54-56,68-70,242-247,257-266`  
`master/internal/telemetry/telemetry_manager.go:86-87,141-147`  
`worker/internal/server/worker_server.go:504-527`  
`docs/USER_MANUAL.md:1162,1302`

---

### 7) gRPC implementation: TLS, connection/retry/backpressure/timeout behavior, partition/crash handling.

**Answer**

**TLS/security**

- gRPC links are currently **insecure/plaintext** (`insecure.NewCredentials()` on Go side; `add_insecure_port` on PPO server). No mTLS/TLS is configured in code.

**Connection strategy**

- Mostly **new dial per operation** (assignment/cancel/heartbeat/result/reporting), not long-lived pooled channels.

**Timeouts/retries**

- Timeouts exist on many paths (`context.WithTimeout`) for assignment, registration, cancel, PPO RPCs.
- Retries are limited/specific (e.g., worker cancellation confirmation retry with exponential backoff); no centralized retry/backoff framework across all RPC types.

**Backpressure**

- Telemetry: drop heartbeats when per-worker channel is full.
- Log streaming: non-blocking fanout drops slow-subscriber messages.

**Network partition / worker crash mid-task**

- Master marks worker inactive after heartbeat timeout (30s), recovers assigned/running tasks: mark attempt lost, release resources, delete assignment, requeue logical task.
- Late stale results are accepted for audit but ignored for state overwrite.

**References**  
`master/internal/scheduler/ppo_scheduler.go:77-82`  
`master/internal/server/master_server.go:1875,1978,2112,2700,549-564,611-664,941-968`  
`worker/internal/server/worker_server.go:88-91,479-482`  
`worker/internal/telemetry/telemetry.go:128-133,238-243`  
`agentic_scheduler/server.py:191-194`  
`master/internal/telemetry/telemetry_manager.go:141-147`

---

### 8) Docker executor security: hardening flags, fork bombs/escape, resource enforcement.

**Answer**

**Configured hardening**

- `PidsLimit=512` (fork-bomb guard)
- `SecurityOpt: no-new-privileges`
- `CapDrop: ALL`
- Minimal `CapAdd`: `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID`
- Container network mode configurable (`bridge|host|none`)

**Resource enforcement**

- CPU: `HostConfig.Resources.NanoCPUs = reqCPU * 1e9`
- Memory: `HostConfig.Resources.Memory = reqMemory * GiB`
- PIDs: `PidsLimit`

**Important limitation**

- **No per-container storage quota flag is set** in Docker host config; storage is tracked/scheduled at master level but not kernel-enforced in container runtime.
- Root FS is currently **not read-only** (`ReadonlyRootfs: false`), so hardening is partial.

So fork-bomb resistance is present, but container escape/malicious behavior hardening is not maximal yet (no explicit seccomp/apparmor policy tuning, read-only FS off).

**References**  
`worker/internal/executor/executor.go:30-32,306-325,327-335,315`  
`worker/internal/system/runtime_config.go:11-67`

---

## Section 3: Deep Reinforcement Learning — PPO Core

### 9) PPO fundamentals: who proposed it, what vs TRPO, clipped objective math.

**Answer**

- PPO was proposed by **Schulman et al.**, paper: **“Proximal Policy Optimization Algorithms” (2017, arXiv:1707.06347)**.
- Compared to TRPO, PPO keeps the trust-region idea but replaces constrained second-order optimization with a simpler first-order clipped surrogate objective.

\[
L^{CLIP}(\theta)=\mathbb{E}_t\left[\min\left(r_t(\theta)\hat A_t,\ \text{clip}(r_t(\theta),1-\epsilon,1+\epsilon)\hat A_t\right)\right]
\]
\[
r_t(\theta)=\frac{\pi_\theta(a_t|s_t)}{\pi_{\theta old}(a_t|s_t)}
\]

Clipping is needed to prevent overly large policy updates in one step (stability/safety).

**References**  
`final_BTEP_report/Chapters/5. Literature Survey.tex:38`  
`final_BTEP_report/Chapters/7. Methodology.tex:61-81`  
`docs/BENCHMARK_RESULTS.md:346-348`

---

### 10) Your architecture: exact network, actor/critic split, action space, infeasible masking.

**Answer**

**Network (implemented)**

- Input is **pairwise 14-dim** = task(5) + worker(9) **per candidate worker**.
- Shared encoder: `Linear(14→128) + ReLU + Linear(128→128) + ReLU`.
- Policy head: `Linear(128→1)` per worker logit.
- Value head: `Linear(128→1)` on pooled (mean-over-workers) hidden state.

**Actor vs Critic**

- They **share encoder parameters**.
- Actor outputs masked categorical logits over workers.
- Critic outputs scalar state value used for advantage estimation and value loss.

**Action space**

- Discrete action = **worker index** in candidate list.
- Inference maps index back to worker ID.

**Masking**

- Feasibility mask from active + CPU/memory/storage checks.
- Infeasible workers are suppressed by setting logits to large negative (`-1e4`).

**References**  
`agentic_scheduler/features.py:15-17,83-91,101-118`  
`agentic_scheduler/model.py:81-113,192-249`  
`agentic_scheduler/service.py:238-264`

---

### 11) Reward function: full equation, each term/formula, why coefficients, ablation.

**Answer**

**Implemented trace-replay reward**

\[
R = 1.4 + 0.25H - 0.35Q_p - 0.55T_p - 0.20I_p - 0.40\Delta I - R_q
\]

With:

1. \(H = 1 - L_{selected}\) (headroom bonus)
2. \(Q_p = \min(queueWaitProxy/slaBudget, 3.0)\)
3. \(turnaroundProxy = queueWaitProxy + runtime\), \(T_p=\max(turnaroundProxy/slaBudget - 1, 0)\)
4. \(I_p=\max(L_{selected}-L_{cluster},0)\)
5. \(\Delta I=\sigma(loads_{after})-\sigma(loads_{before})\)
6. \(R_q = 0.05 \times \min(requeueCount,4)\)

If infeasible action: reward = **-1.8**.

**Coefficient choice**

- Repository documentation states coefficients were **empirically tuned** for a smooth, non-degenerate reward landscape.
- I found **no formal ablation study with statistical tests** in checked artifacts; tuning rationale is documented qualitatively.

**References**  
`agentic_scheduler/training/trace_replay_env.py:311-338,330-346`  
`final_BTEP_report/Chapters/7. Methodology.tex:221-263,270`  
`agentic_scheduler/TRAINING_ARCHITECTURE.md:705-740`  
`agentic_scheduler/TRAINING_DECISIONS.md:104-114`

---

### 12) Full PPO loss: policy/value/entropy terms, coefficients and why.

**Answer**

Implemented total loss:

\[
L = L^{CLIP} + c_1 L^{VF} - c_2 H[\pi_\theta]
\]

Where in code:

1. **Policy loss:** clipped surrogate with ratio \(r_t\).
2. **Value loss:** clipped value objective (max of unclipped/clipped MSE).
3. **Entropy term:** categorical entropy bonus.

**Configured coefficients**

- `value_coeff (c1) = 0.5`
- `entropy_coeff (c2) = 0.01`
- `clip_ratio = 0.2`

Why: these are the project’s PPO defaults (offline and online) balancing stable critic learning with controlled exploration; documented as tuned around conventional PPO ranges.

**References**  
`agentic_scheduler/model.py:252-320`  
`agentic_scheduler/train_ppo.py:31-39,570-580`  
`agentic_scheduler/service.py:446-453`  
`final_BTEP_report/Chapters/A. Appendix A.tex:35-38`

---

### 13) Hyperparameters vs parameters, full hyperparameter list + values, offline/online details, C5 issue.

**Answer**

**Definitions**

- **Parameters**: learned weights/biases of actor-critic network.
- **Hyperparameters**: externally set training controls (e.g., \(\gamma\), LR, epochs, clip).

**Core PPO hyperparameters (defaults in trainer)**

| Hyperparameter | Value |
|---|---:|
| `gamma` | 0.99 |
| `gae_lambda` | 0.95 |
| `learning_rate` | 3e-4 |
| `clip_ratio` | 0.2 |
| `entropy_coeff` | 0.01 |
| `value_coeff` | 0.5 |
| `value_clip_range` | 0.2 |
| `ppo_epochs` | 6 |
| `minibatch_size` | 256 |
| `rollout_steps` | 1024 |
| `updates` | 200 |

**Offline training context**

- Campaign artifacts describe offline PPO trained on Alibaba trace with **~199,614 tasks** and **200 updates** for campaign baseline model.

**Online adaptation**

- Trigger: each reported outcome appends replay sample; train when buffer reaches `update_batch_size` (default **32**).
- Online update settings: `gamma=0.97`, `gae_lambda=0.92`, `epochs=4`, clip 0.2, entropy 0.01, value coeff 0.5.

**What went wrong in C5**

- Online updates were active and produced new version, but benchmark shows tied success and slower PPO average duration.
- Documented reasons: short horizon, mixed/conflicting synthetic workloads, policy oscillation/drift from offline optimum.

**References**  
`agentic_scheduler/train_ppo.py:27-39,89-97`  
`docs/BENCHMARK_RESULTS.md:6,34`  
`agentic_scheduler/service.py:47-54,327-331,425-453`  
`docs/ONLINE_ADAPTATION_ANALYSIS.md:30-37,90-99,147-156`

---

### 14) Learning dynamics: one full PPO update path, entropy interpretation, best average reward & feasible action rate.

**Answer**

**One update cycle**

1. Rollout collects transitions: features, mask, action, old log-prob, old value, reward, done.
2. Compute GAE advantages + returns (bootstrapped with next value).
3. Normalize advantages.
4. Build tensors and run PPO update across epochs/minibatches.
5. Compute clipped policy loss + clipped value loss − entropy bonus.
6. Backprop + gradient clip (`max_norm=1.0`) + optimizer step.

**Entropy interpretation**

- High entropy = exploratory policy spread across workers.
- Lower entropy over training = more confident/specialized scheduling decisions.

**Best model metrics (reported)**

- Mean reward (best PPO model): **0.8801**
- Feasible action rate: **94.86%**

**References**  
`agentic_scheduler/train_ppo.py:457-580,295-324,529-537`  
`agentic_scheduler/model.py:294-320`  
`final_BTEP_report/Chapters/8. Results.tex:69-83`  
`final_BTEP_report/Chapters/8. Results.tex:59-63`

---

## Section 4: PPO Implementation & Integration

### 15) State representation: exact 14 features and normalization details.

**Answer**

Implemented state is **pairwise 14 features per (task, worker)**:

**Task (5):**

1. `req_cpu`
2. `req_memory`
3. `req_storage`
4. `sla_multiplier`
5. `task_type_scalar` (`cpu-light=0`, `cpu-heavy=1/3`, `memory-heavy=2/3`, `mixed=1`)

**Worker (9):**

6. `available_cpu / total_cpu`
7. `available_memory / total_memory`
8. `available_storage / total_storage`
9. `total_cpu`
10. `total_memory`
11. `total_storage`
12. `used_cpu / total_cpu`
13. `used_memory / total_memory`
14. `used_storage / total_storage`

**Normalization**

- `RunningNormalizer` (Welford-style online mean/variance):
  - tracks `count`, `mean`, `m2`
  - normalizes as \((x-\mu)/\sigma\), floor variance at `1e-6`
  - state persisted in checkpoints for consistency.

**References**  
`agentic_scheduler/features.py:15-17,29-39,42-80`  
`agentic_scheduler/model.py:20-59,60-77,205-214`

---

### 16) Inference vs training: `choose_action` in production, deterministic bias/headroom reranking, why 0.20/0.25.

**Answer**

**Production inference (`choose_action`)**

1. Build pairwise rows, update+apply normalizer.
2. Run policy/value forward with action mask.
3. In deterministic mode, add headroom prior (`headroom_bias * projected_headroom_scores`) and rerank top-k feasible logits.
4. Return action index + log-prob + value.

**Training**

- Offline training uses `deterministic=False` rollout sampling for exploration.
- Online service inference is deterministic by default with configurable bias.

**0.25 vs 0.20**

- Runtime default deterministic bias is **0.25**.
- Optimization campaign reported better repeated-cluster behavior with **0.20**, so this was used in optimized runs.
- So: 0.25 = safe default, 0.20 = empirically tuned deployment setting for that workload.

**References**  
`agentic_scheduler/model.py:192-241,324-379`  
`agentic_scheduler/service.py:51,252-253`  
`agentic_scheduler/server.py:157-161`  
`docs/PPO_PERFORMANCE_OPTIMIZATION.md:28-33,79-83`

---

### 17) Model persistence: local files, Mongo/GridFS, versioning, topology fingerprinting.

**Answer**

- **Local format:** PyTorch checkpoint bytes (`.pt`), includes model/optimizer/normalizer/version/fingerprint/training steps.
- **Mongo persistence:** `SCHEDULER_MODELS` metadata + GridFS bucket `scheduler_models`.
- **Versioning:** monotonically incremented per `(scheduler_type, fingerprint_hash)`, exactly one active version (partial unique index).
- **Fingerprinting:** strict cluster fingerprint hash built from sorted worker IDs and capacities (`total_cpu/memory/storage` + schema version). PPO model is loaded/activated per fingerprint.

**References**  
`agentic_scheduler/model.py:126-139`  
`agentic_scheduler/persistence.py:43-79,115-186`  
`master/internal/scheduler/fingerprint.go:24-64`  
`agentic_scheduler/service.py:108-152,457-473,482-519`  
`proto/ppo_scheduler.proto:65-78`

---

### 18) Shadow / Active / Fallback modes: trade-offs and usage.

**Answer**

1. **Shadow**
   - PPO queried, but fallback scheduler decision is enforced.
   - Use for safe evaluation/divergence analysis.
2. **Active**
   - PPO decision used; on RPC/model/validation failure, fallback scheduler used.
   - Use for production with guardrails.
3. **Fallback**
   - PPO RPC bypassed entirely; always use fallback scheduler.
   - Use as safety/dependency-isolation mode.

**Trade-off summary**

- Safety: `fallback > shadow > active`
- Learning impact: `active > shadow > fallback`
- Operational risk: `active` highest without robust PPO SLOs, mitigated by fallback path.

**References**  
`master/internal/scheduler/ppo_scheduler.go:18-22,66-69,139-223,225-230`  
`ARCHITECTURE.md:69-75`

---

### 19) Performance: why online PPO underperformed frozen in C5, production changes.

**Answer**

From campaign analysis:

1. C5 used short mixed-workload benchmark; signals were conflicting.
2. Same success rate ceiling (resource-limited scenarios), so gains must come from duration optimization only.
3. Online updates appeared to oscillate/drift policy in that short horizon.

**What to change for production**

1. Start with frozen model; enable online adaptation only after stable long-lived workload.
2. Gate updates by data quality/stationarity windows, not immediate mixed benchmark feedback.
3. Add convergence telemetry (loss/entropy/reward drift) and rollback guardrails.

**References**  
`docs/ONLINE_ADAPTATION_ANALYSIS.md:30-37,90-99,109-117,147-156,193-201`

---

## Section 5: Benchmarking, Results & Claims

### 20) Methodology (C2/C4/C5), burst dominance, tie in success, significance.

**Answer**

**Campaign structures**

1. **C2**: 6 workloads × 3 scenarios × 4 scheduler variants = **72 runs** (includes offline + online PPO variants).
2. **C4**: 4 workloads × 3 scenarios × 3 schedulers = **36 runs**.
3. **C5**: 4 workloads × 3 scenarios × 3 schedulers = **36 runs**, online updates active.

**Why PPO dominated burst in C2**

- RR and RTS hit cliff-edge failures/timeouts in burst scenarios (0% in listed burst workloads), while PPO remained feasible in many burst cases (75–100% bands in report).
- This aligns with learned policy adapting to load interactions vs brittle rule thresholds/cycling.

**Why tie in success rate in C4/C5**

- In those runs, all schedulers often hit same hardware feasibility limits (especially memory-pressure), yielding equal completion ratios; differentiation moved to duration/queue/tail metrics.

**Statistical significance**

- There are multiple campaign runs and scenario matrices, but I found **no formal significance tests** (e.g., p-values/confidence intervals) reported.  
- So claims are empirical comparative results, not statistically certified inference.

**References**  
`docs/BENCHMARK_RESULTS.md:47-53,60-71,82-91,113-126`  
`docs/ONLINE_ADAPTATION_ANALYSIS.md:9-10,30-37,81-87`

---

### 21) Comparison with SAC-CS paper: key differences, PPO over SAC.

**Answer**

Key differences documented in project artifacts:

1. **Algorithm**: SAC (paper) vs PPO (this project).
2. **Training/eval setting**: SAC-CS reported in simulation; this project emphasizes live Docker cluster execution with real control plane.
3. **Data source**: project trains with Alibaba trace replay and deploys into live cluster flow.
4. **Online adaptation path** exists in this PPO service (outcome reporting + replay updates).

Why PPO here:

- Simpler clipped-policy updates, easier integration in this Go↔Python gRPC service model, and strong empirical burst resilience against project baselines.

**References**  
`docs/BENCHMARK_RESULTS.md:253-281`  
`final_BTEP_report/Chapters/5. Literature Survey.tex:38-48,62`

---

### 22) Novelty claim: what is truly novel here?

**Answer (brutally honest)**

This project is **not novel at RL-theory level** (PPO for scheduling is established).  
Its real contribution is **systems integration + evidence**:

1. A complete Go distributed cluster control plane with scheduler abstraction, recovery semantics, and live Docker execution.
2. Practical PPO integration (fingerprint-aware model lifecycle, shadow/active/fallback deployment modes, online feedback loop).
3. End-to-end comparative campaign evidence across realistic stress scenarios, including documented failure modes and engineering bug fixes.

So novelty is primarily **engineering architecture and operational evaluation rigor in this codebase**, not a new PPO algorithm.

**References**  
`master/internal/scheduler/scheduler.go:10-40`  
`master/internal/scheduler/ppo_scheduler.go:18-27,139-223`  
`docs/BENCHMARK_RESULTS.md:173-193`

---

## Section 6: Edge Cases, Reliability, Future Work

### 23) Failure handling: worker task failure, retry/reschedule, PPO reward signal.

**Answer**

Failure path:

1. Assignment failure or worker timeout → task stays/re-enters queue.
2. Worker crash/partition → attempt marked lost, resources released, assignment deleted, task marked for requeue.
3. Late stale completion reports are recorded but ignored for active logical state.

Retry model:

- Queue processor retries on subsequent cycles (5s ticker), preserving FIFO for retries.

PPO signal:

- Master computes outcome reward (`success +1`, `cancel -0.5`, else `-1`, SLA and runtime adjustments) and sends async `ReportOutcome`.
- PPO service stores terminal outcome in replay buffer and updates when batch threshold reached.

**References**  
`master/internal/server/master_server.go:2193-2327,611-664,941-968,1203-1223,1125-1190`  
`agentic_scheduler/service.py:281-331,332-349,373-456`

---

### 24) Scalability: behavior at 10 workers vs 100 workers, bottlenecks.

**Answer**

At **10 workers**, architecture should remain comfortable: per-worker telemetry goroutine model and current queue loop are manageable.

At **100 workers**, likely bottlenecks:

1. **Single master coordinator** (stateful SPOF + throughput bottleneck).
2. **Queue cycle granularity (5s) + O(queue×workers) decision work** can increase latency.
3. **Per-RPC dial pattern** (no pooling) adds overhead under high request rates.
4. Global locks (`mu`, `queueMu`) become hotter with more concurrent events.
5. Recovery/reconciliation DB operations scale up during failures.

So practical scaling to 100 requires connection reuse, lock/queue refinement, and likely control-plane sharding/HA.

**References**  
`ARCHITECTURE.md:249-252`  
`master/internal/server/master_server.go:2167,2193-2327,2553-2792`  
`master/internal/telemetry/telemetry_manager.go:24-25,74-101`  
`worker/internal/telemetry/telemetry.go:128-137`

---

### 25) CRIU mention in journal: implemented or not?

**Answer**

**Not implemented in current runtime code.**  
CRIU appears in the report as **future work** proposal (checkpoint/restore, migration, warm start), not as implemented executor path.

**References**  
`final_BTEP_report/Chapters/9. Conclusion.tex:20-29`

---

### 26) Top 3–5 known issues / technical debt now.

**Answer**

1. **gRPC transport security gap**: plaintext/insecure credentials across links.
2. **Executor backend coupling**: worker server depends on concrete Docker executor (no backend interface abstraction).
3. **Storage quota enforcement gap**: storage requested/scheduled, but no per-container Docker storage limit flag enforced.
4. **Single-master architecture**: SPOF and scaling bottleneck.
5. **Online adaptation stability controls**: current online PPO can oscillate under short mixed workloads; limited convergence guardrails.

**References**  
`master/internal/scheduler/ppo_scheduler.go:80`  
`worker/internal/server/worker_server.go:27`  
`worker/internal/executor/executor.go:309-335`  
`ARCHITECTURE.md:249-252`  
`docs/ONLINE_ADAPTATION_ANALYSIS.md:94-99,147-156`

---

### 27) Future work: top 3 concrete improvements with effort and impact.

**Answer**

| Priority | Improvement | Effort | Expected Impact |
|---|---|---|---|
| 1 | **Secure control plane** (mTLS for master↔worker and master↔PPO, cert rotation, authN/authZ between services) | Medium | High (security + production readiness) |
| 2 | **Scalable execution/control path** (gRPC connection pooling/reuse, queue scheduling cadence optimization, lock contention reduction, master HA path) | High | High (10→100+ worker scalability, lower latency) |
| 3 | **Stabilized online RL operations** (drift detection, update gating by workload stationarity, canary model promotion/rollback, better online diagnostics) | Medium | High (prevents adaptation regressions while preserving long-run gains) |

---

## Notes on Evidence Quality

- All answers are mapped to current repo code/docs where available.
- One explicit gap: no artifact-backed race-detector/profiler run findings were found in reviewed materials.
