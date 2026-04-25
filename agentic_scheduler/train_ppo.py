from __future__ import annotations

import argparse
import logging
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, Optional

import numpy as np
import torch

from .features import TASK_FEATURE_DIM
from .model import PPOState, build_fresh_state, choose_action, ppo_update
from .persistence import MongoSchedulerModelStore
from .training.scheduler_env import SchedulingEnv
from .training.trace_loader import load_trace
from .training.trace_replay_env import TraceReplayEnv


LOGGER = logging.getLogger(__name__)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train PPO scheduler with Gymnasium + PyTorch")
    parser.add_argument("--num-workers", type=int, default=4)
    parser.add_argument("--episode-length", type=int, default=96)
    parser.add_argument("--rollout-steps", type=int, default=1024)
    parser.add_argument("--updates", type=int, default=200)
    parser.add_argument("--gamma", type=float, default=0.99)
    parser.add_argument("--gae-lambda", type=float, default=0.95)
    parser.add_argument("--learning-rate", type=float, default=3e-4)
    parser.add_argument("--clip-ratio", type=float, default=0.2)
    parser.add_argument("--entropy-coeff", type=float, default=0.01)
    parser.add_argument("--value-coeff", type=float, default=0.5)
    parser.add_argument("--value-clip-range", type=float, default=0.2)
    parser.add_argument("--ppo-epochs", type=int, default=6)
    parser.add_argument("--minibatch-size", type=int, default=256)
    parser.add_argument(
        "--lr-anneal",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="Linearly anneal learning rate to 0 over updates",
    )
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--output", default="agentic_scheduler/models/ppo_trained.pt")
    parser.add_argument("--checkpoint-dir", default="agentic_scheduler/models/checkpoints")
    parser.add_argument("--checkpoint-prefix", default="ppo_offline")
    parser.add_argument(
        "--checkpoint-every",
        type=int,
        default=10,
        help="Save local checkpoint every N PPO updates (<=0 disables periodic checkpoints)",
    )
    parser.add_argument("--resume-from", default="", help="Resume from an explicit checkpoint path")
    parser.add_argument(
        "--resume-latest",
        action="store_true",
        help="Resume from the latest .pt checkpoint in --checkpoint-dir",
    )
    parser.add_argument("--log-every", type=int, default=10)
    parser.add_argument("--mongo-uri", default="")
    parser.add_argument("--mongo-db", default="cluster_db")
    parser.add_argument("--fingerprint-hash", default="")
    parser.add_argument("--fingerprint-payload", default="")
    parser.add_argument(
        "--mongo-checkpoint-every",
        type=int,
        default=0,
        help="Save active checkpoint to Mongo every N PPO updates (<=0 disables periodic Mongo checkpoints)",
    )
    parser.add_argument(
        "--mongo-save-final",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="Persist final checkpoint to Mongo when --mongo-uri and --fingerprint-hash are provided",
    )
    parser.add_argument(
        "--resume-mongo",
        action="store_true",
        help="Resume from active Mongo checkpoint when local resume is not used",
    )
    # Trace replay options
    parser.add_argument("--trace-source", default="", choices=["", "alibaba", "google", "cloudai"],
                        help="Use real cluster trace instead of synthetic env")
    parser.add_argument("--trace-path", default="",
                        help="Path to trace data directory (optional for cloudai when mongo-uri is set)")
    parser.add_argument("--max-trace-tasks", type=int, default=5000,
                        help="Maximum tasks to load from trace")
    parser.add_argument("--trace-window", default="",
                        help="Optional trace window label (used by cloudai replay lineage/filter)")
    parser.add_argument("--trace-window-start", default="",
                        help="Optional trace window start timestamp (unix or ISO-8601)")
    parser.add_argument("--trace-window-end", default="",
                        help="Optional trace window end timestamp (unix or ISO-8601)")
    return parser.parse_args()


_MAX_NUM_WORKERS = 256
_MAX_ROLLOUT_STEPS = 65536
_MAX_UPDATES = 100000
_MAX_EPISODE_LENGTH = 10000


def _validate_training_args(args: argparse.Namespace) -> None:
    """Enforce upper bounds on resource-controlling parameters."""
    if args.num_workers > _MAX_NUM_WORKERS:
        raise ValueError(f"--num-workers must be <= {_MAX_NUM_WORKERS}")
    if args.rollout_steps > _MAX_ROLLOUT_STEPS:
        raise ValueError(f"--rollout-steps must be <= {_MAX_ROLLOUT_STEPS}")
    if args.updates > _MAX_UPDATES:
        raise ValueError(f"--updates must be <= {_MAX_UPDATES}")
    if args.episode_length > _MAX_EPISODE_LENGTH:
        raise ValueError(f"--episode-length must be <= {_MAX_EPISODE_LENGTH}")
    if args.num_workers < 1:
        raise ValueError("--num-workers must be >= 1")
    if args.rollout_steps < 1:
        raise ValueError("--rollout-steps must be >= 1")
    if args.learning_rate <= 0:
        raise ValueError("--learning-rate must be > 0")


def resolve_training_device() -> torch.device:
    if torch.cuda.is_available():
        return torch.device("cuda")
    return torch.device("cpu")


def atomic_write_bytes(path: Path, payload: bytes) -> None:
    destination = path.expanduser()
    destination.parent.mkdir(parents=True, exist_ok=True)
    temp_name: Optional[str] = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="wb",
            prefix=f".{destination.name}.",
            suffix=".tmp",
            dir=str(destination.parent),
            delete=False,
        ) as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
            temp_name = handle.name
        os.replace(temp_name, destination)
    finally:
        if temp_name:
            temp_path = Path(temp_name)
            if temp_path.exists():
                try:
                    temp_path.unlink()
                except OSError:
                    LOGGER.warning("Failed cleaning temp checkpoint %s", temp_path)


def find_latest_checkpoint(checkpoint_dir: Path) -> Optional[Path]:
    directory = checkpoint_dir.expanduser()
    if not directory.exists():
        return None
    candidates: list[tuple[int, str, Path]] = []
    for candidate in directory.glob("*.pt"):
        if not candidate.is_file():
            continue
        try:
            mtime = candidate.stat().st_mtime_ns
        except OSError:
            continue
        candidates.append((mtime, candidate.name, candidate))
    # Also check legacy .pkl files for backward compatibility
    for candidate in directory.glob("*.pkl"):
        if not candidate.is_file():
            continue
        try:
            mtime = candidate.stat().st_mtime_ns
        except OSError:
            continue
        candidates.append((mtime, candidate.name, candidate))
    if not candidates:
        return None
    return max(candidates, key=lambda item: (item[0], item[1]))[2]


def resolve_resume_path(args: argparse.Namespace) -> Optional[Path]:
    explicit = Path(args.resume_from).expanduser() if args.resume_from else None
    checkpoint_dir = Path(args.checkpoint_dir).expanduser()

    if explicit:
        if explicit.exists():
            return explicit
        if not args.resume_latest:
            raise FileNotFoundError(f"Resume checkpoint not found: {explicit}")
        LOGGER.warning(
            "--resume-from path %s does not exist; falling back to latest checkpoint in %s",
            explicit,
            checkpoint_dir,
        )

    if args.resume_latest:
        latest = find_latest_checkpoint(checkpoint_dir)
        if latest is None:
            raise FileNotFoundError(f"No .pt checkpoints found in {checkpoint_dir}")
        return latest
    return None


def periodic_checkpoint_path(
    checkpoint_dir: Path,
    checkpoint_prefix: str,
    update_idx: int,
    training_steps: int,
) -> Path:
    return checkpoint_dir / f"{checkpoint_prefix}_u{update_idx:06d}_s{training_steps:06d}.pt"


def create_mongo_store(args: argparse.Namespace, purpose: str) -> MongoSchedulerModelStore:
    if not args.mongo_uri:
        raise ValueError(f"{purpose} requires --mongo-uri")
    if not args.fingerprint_hash:
        raise ValueError(f"{purpose} requires --fingerprint-hash")
    store = MongoSchedulerModelStore(args.mongo_uri, args.mongo_db)
    if not store.is_available():
        store.close()
        raise RuntimeError(f"Mongo scheduler model store unavailable for {purpose}")
    return store


def update_state_model_version_from_mongo(state: PPOState, metadata: Dict) -> None:
    raw_version = metadata.get("version")
    try:
        version = int(raw_version)
    except (TypeError, ValueError):
        LOGGER.warning("Mongo checkpoint metadata has non-integer version: %r", raw_version)
        return
    if version > 0:
        state.model_version = f"v{version}"


def persist_checkpoint_to_mongo(
    store: MongoSchedulerModelStore,
    args: argparse.Namespace,
    checkpoint_bytes: bytes,
    *,
    checkpoint_kind: str,
    update_idx: int,
    training_steps: int,
    lineage_metadata: Dict[str, str],
    local_checkpoint_path: Optional[Path] = None,
) -> Dict:
    extra_metadata: Dict[str, object] = {
        "source": "offline-training",
        "checkpoint_kind": checkpoint_kind,
        "update_idx": update_idx,
        "configured_updates": args.updates,
        "training_steps": training_steps,
        "trace_source": args.trace_source or "synthetic",
        **lineage_metadata,
    }
    if local_checkpoint_path is not None:
        extra_metadata["local_checkpoint_path"] = str(local_checkpoint_path)

    saved = store.save_and_activate_checkpoint(
        scheduler_type="PPO",
        fingerprint_hash=args.fingerprint_hash,
        fingerprint_payload=args.fingerprint_payload,
        checkpoint_bytes=checkpoint_bytes,
        framework="pytorch-ppo",
        extra_metadata=extra_metadata,
    )
    if not saved:
        raise RuntimeError(
            f"Failed to persist {checkpoint_kind} checkpoint to Mongo for fingerprint {args.fingerprint_hash}"
        )
    return saved


def resume_state_from_mongo(
    store: MongoSchedulerModelStore,
    args: argparse.Namespace,
    learning_rate: float,
    device: torch.device,
) -> PPOState:
    loaded = store.load_active_checkpoint("PPO", args.fingerprint_hash)
    if not loaded:
        raise FileNotFoundError(
            f"No active Mongo checkpoint found for scheduler=PPO fingerprint={args.fingerprint_hash}"
        )
    metadata, checkpoint_bytes = loaded
    state = PPOState.from_checkpoint_bytes(checkpoint_bytes, learning_rate, device)
    update_state_model_version_from_mongo(state, metadata)
    state.fingerprint_hash = args.fingerprint_hash
    return state


def generalized_advantage_estimation(
    rewards: np.ndarray,
    dones: np.ndarray,
    values: np.ndarray,
    next_value: float,
    gamma: float,
    gae_lambda: float,
) -> tuple[np.ndarray, np.ndarray]:
    advantages = np.zeros_like(rewards, dtype=np.float32)
    gae = 0.0
    for i in reversed(range(len(rewards))):
        not_done = 1.0 - float(dones[i])
        bootstrap_value = float(next_value) if i == (len(rewards) - 1) else float(values[i + 1])
        delta = rewards[i] + gamma * bootstrap_value * not_done - values[i]
        gae = delta + gamma * gae_lambda * not_done * gae
        advantages[i] = gae

    returns = advantages + values
    return advantages.astype(np.float32), returns.astype(np.float32)


def build_lineage_metadata(args: argparse.Namespace, trace) -> dict:
    if trace is not None:
        model_source = "offline-trace-replay"
        training_corpus = trace.source or (args.trace_source or "trace")
        trace_window = args.trace_window or getattr(trace, "trace_window", "") or "full"
    else:
        model_source = "synthetic-training"
        training_corpus = "synthetic-smoke"
        trace_window = args.trace_window or "synthetic"

    return {
        "model_source": model_source,
        "training_corpus": training_corpus,
        "trace_window": trace_window,
        "cluster_fingerprint": args.fingerprint_hash,
        "train_timestamp": datetime.now(timezone.utc).isoformat(),
    }


def main() -> None:
    args = parse_args()
    _validate_training_args(args)
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
    np.random.seed(args.seed)
    torch.manual_seed(args.seed)
    device = resolve_training_device()
    if device.type == "cuda":
        torch.cuda.manual_seed_all(args.seed)
        LOGGER.info("Using GPU for offline PPO training: %s", torch.cuda.get_device_name(device))
    else:
        LOGGER.info("Using CPU for offline PPO training")

    if device.type == "cuda":
        torch.backends.cudnn.benchmark = True

    resume_path: Optional[Path]
    try:
        resume_path = resolve_resume_path(args)
    except FileNotFoundError:
        if args.resume_mongo:
            LOGGER.warning("Local resume checkpoint not found; falling back to --resume-mongo")
            resume_path = None
        else:
            raise

    if args.resume_mongo and resume_path:
        LOGGER.info("Using local resume checkpoint %s; ignoring --resume-mongo", resume_path)

    if args.mongo_checkpoint_every > 0 and (not args.mongo_uri or not args.fingerprint_hash):
        raise ValueError("--mongo-checkpoint-every requires --mongo-uri and --fingerprint-hash")

    mongo_resume_enabled = args.resume_mongo and resume_path is None
    if mongo_resume_enabled and (not args.mongo_uri or not args.fingerprint_hash):
        raise ValueError("--resume-mongo requires --mongo-uri and --fingerprint-hash")

    mongo_persist_enabled = bool(
        args.mongo_uri
        and args.fingerprint_hash
        and (args.mongo_checkpoint_every > 0 or args.mongo_save_final)
    )
    mongo_store_required = mongo_resume_enabled or mongo_persist_enabled
    mongo_store: Optional[MongoSchedulerModelStore] = None
    if mongo_store_required:
        mongo_store = create_mongo_store(args, "offline PPO checkpoint persistence/resume")

    trace = None
    # Choose environment: trace replay or synthetic
    if args.trace_source:
        if args.trace_source in {"alibaba", "google"} and not args.trace_path:
            raise ValueError(f"--trace-path is required for trace source {args.trace_source}")
        if args.trace_source == "cloudai" and not args.trace_path and not args.mongo_uri:
            raise ValueError("--trace-source cloudai requires --trace-path or --mongo-uri")

        trace_path = args.trace_path or ""
        LOGGER.info("Loading %s trace from %s (max %d tasks)",
                     args.trace_source, trace_path or "<mongo>", args.max_trace_tasks)
        trace = load_trace(
            trace_path,
            args.trace_source,
            max_tasks=args.max_trace_tasks,
            trace_window=args.trace_window,
            trace_window_start=args.trace_window_start,
            trace_window_end=args.trace_window_end,
            mongo_uri=args.mongo_uri,
            mongo_db=args.mongo_db,
        )
        env = TraceReplayEnv(trace, num_workers=args.num_workers, loop=True)
        LOGGER.info("Trace replay env: %s (window=%s)", trace.description, getattr(trace, "trace_window", "full"))
    else:
        env = SchedulingEnv(num_workers=args.num_workers, episode_length=args.episode_length, seed=args.seed)
    try:
        if resume_path:
            state = PPOState.from_checkpoint_bytes(resume_path.read_bytes(), args.learning_rate, device)
            LOGGER.info("Resumed PPO training from %s (training_steps=%d)", resume_path, state.training_steps)
        elif mongo_resume_enabled:
            if mongo_store is None:
                raise RuntimeError("Mongo resume requested but Mongo store is not initialized")
            state = resume_state_from_mongo(mongo_store, args, args.learning_rate, device)
            LOGGER.info(
                "Resumed PPO training from Mongo fingerprint=%s version=%s (training_steps=%d)",
                args.fingerprint_hash,
                state.model_version,
                state.training_steps,
            )
        else:
            state = build_fresh_state(args.learning_rate, device)

        if args.fingerprint_hash:
            state.fingerprint_hash = args.fingerprint_hash

        observation, _ = env.reset(seed=args.seed)
        recent_rewards = []

        for update_idx in range(1, args.updates + 1):
            if args.lr_anneal and args.updates > 1:
                frac = 1.0 - ((update_idx - 1) / float(args.updates - 1))
                current_lr = max(args.learning_rate * frac, 1e-6)
                for param_group in state.optimizer.param_groups:
                    param_group["lr"] = current_lr

            transitions = []
            step_rewards = []

            for _ in range(args.rollout_steps):
                task_features = observation["task"]
                worker_features = observation["workers"]
                action_mask = observation["action_mask"].astype(bool)

                action_info = choose_action(
                    state,
                    task_features=task_features,
                    worker_features=worker_features,
                    action_mask=action_mask,
                    device=device,
                    deterministic=False,
                )
                if action_info is None:
                    feasible_ids = np.where(action_mask)[0]
                    if feasible_ids.size:
                        action = int(np.random.choice(feasible_ids))
                    else:
                        action = int(np.random.randint(0, env.num_workers))
                    old_log_prob = 0.0
                    old_value = 0.0
                    # Normalize features for consistency even in fallback path
                    pairwise = np.concatenate(
                        [np.repeat(task_features[None, :], worker_features.shape[0], axis=0), worker_features],
                        axis=1,
                    )
                    normalized = state.normalizer.normalize(pairwise)
                    task_features = normalized[0, :TASK_FEATURE_DIM].astype(np.float32)
                    worker_features = normalized[:, TASK_FEATURE_DIM:].astype(np.float32)
                else:
                    action = int(action_info["action_index"])
                    old_log_prob = float(action_info["log_prob"])
                    old_value = float(action_info["value"])
                    worker_features = np.asarray(action_info["normalized_worker_features"], dtype=np.float32)
                    task_features = np.asarray(action_info["normalized_task_features"], dtype=np.float32)

                next_observation, reward, terminated, truncated, _ = env.step(action)
                done = bool(terminated or truncated)

                transitions.append(
                    {
                        "task_features": task_features.astype(np.float32),
                        "worker_features": worker_features.astype(np.float32),
                        "action_mask": action_mask.astype(np.bool_),
                        "action": action,
                        "old_log_prob": old_log_prob,
                        "old_value": old_value,
                        "reward": float(reward),
                        "done": done,
                    }
                )
                step_rewards.append(float(reward))

                observation = next_observation
                if done:
                    observation, _ = env.reset()

            rewards = np.asarray([x["reward"] for x in transitions], dtype=np.float32)
            dones = np.asarray([x["done"] for x in transitions], dtype=np.bool_)
            values = np.asarray([x["old_value"] for x in transitions], dtype=np.float32)
            bootstrap_info = choose_action(
                state,
                task_features=observation["task"],
                worker_features=observation["workers"],
                action_mask=observation["action_mask"].astype(bool),
                device=device,
                deterministic=True,
                headroom_bias=0.0,
            )
            next_value = float(bootstrap_info["value"]) if bootstrap_info is not None else 0.0
            advantages, returns = generalized_advantage_estimation(
                rewards=rewards,
                dones=dones,
                values=values,
                next_value=next_value,
                gamma=args.gamma,
                gae_lambda=args.gae_lambda,
            )
            advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)

            batch = {
                "task_features": torch.as_tensor(
                    np.stack([x["task_features"] for x in transitions], axis=0),
                    dtype=torch.float32,
                    device=device,
                ),
                "worker_features": torch.as_tensor(
                    np.stack([x["worker_features"] for x in transitions], axis=0),
                    dtype=torch.float32,
                    device=device,
                ),
                "action_masks": torch.as_tensor(
                    np.stack([x["action_mask"] for x in transitions], axis=0),
                    dtype=torch.bool,
                    device=device,
                ),
                "actions": torch.as_tensor(
                    np.asarray([x["action"] for x in transitions], dtype=np.int64),
                    dtype=torch.long,
                    device=device,
                ),
                "old_log_probs": torch.as_tensor(
                    np.asarray([x["old_log_prob"] for x in transitions], dtype=np.float32),
                    dtype=torch.float32,
                    device=device,
                ),
                "old_values": torch.as_tensor(values, dtype=torch.float32, device=device),
                "returns": torch.as_tensor(returns, dtype=torch.float32, device=device),
                "advantages": torch.as_tensor(advantages, dtype=torch.float32, device=device),
            }

            ppo_update(
                state,
                batch=batch,
                clip_ratio=args.clip_ratio,
                entropy_coeff=args.entropy_coeff,
                value_coeff=args.value_coeff,
                epochs=args.ppo_epochs,
                minibatch_size=args.minibatch_size,
                value_clip_range=args.value_clip_range,
            )

            recent_rewards.extend(step_rewards)
            if len(recent_rewards) > 5000:
                recent_rewards = recent_rewards[-5000:]

            if update_idx % args.log_every == 0 or update_idx == 1:
                processed = update_idx * args.rollout_steps
                if trace is not None:
                    total_tasks = len(trace.tasks)
                    epochs = processed / total_tasks if total_tasks > 0 else 0.0
                    LOGGER.info(
                        "update=%d avg_reward=%.4f steps=%d epoch=%.2f",
                        update_idx,
                        float(np.mean(recent_rewards) if recent_rewards else 0.0),
                        processed,
                        epochs,
                    )
                else:
                    LOGGER.info(
                        "update=%d avg_reward=%.4f steps=%d",
                        update_idx,
                        float(np.mean(recent_rewards) if recent_rewards else 0.0),
                        processed,
                    )

            save_local_checkpoint = args.checkpoint_every > 0 and update_idx % args.checkpoint_every == 0
            save_mongo_checkpoint = args.mongo_checkpoint_every > 0 and update_idx % args.mongo_checkpoint_every == 0
            if save_local_checkpoint or save_mongo_checkpoint:
                lineage_metadata = build_lineage_metadata(args, trace)
                state.lineage_metadata = dict(lineage_metadata)
                payload = state.checkpoint_payload()
                local_checkpoint_path: Optional[Path] = None

                if save_local_checkpoint:
                    checkpoint_dir = Path(args.checkpoint_dir).expanduser()
                    local_checkpoint_path = periodic_checkpoint_path(
                        checkpoint_dir,
                        args.checkpoint_prefix,
                        update_idx,
                        state.training_steps,
                    )
                    atomic_write_bytes(local_checkpoint_path, payload)
                    atomic_write_bytes(checkpoint_dir / f"{args.checkpoint_prefix}_latest.pt", payload)
                    LOGGER.info("Saved periodic local checkpoint to %s", local_checkpoint_path)

                if save_mongo_checkpoint:
                    if mongo_store is None:
                        raise RuntimeError("Mongo periodic checkpoint requested but Mongo store is not initialized")
                    saved = persist_checkpoint_to_mongo(
                        mongo_store,
                        args,
                        payload,
                        checkpoint_kind="periodic",
                        update_idx=update_idx,
                        training_steps=state.training_steps,
                        lineage_metadata=lineage_metadata,
                        local_checkpoint_path=local_checkpoint_path,
                    )
                    update_state_model_version_from_mongo(state, saved)
                    LOGGER.info(
                        "Persisted periodic checkpoint to Mongo version v%s (update=%d)",
                        saved.get("version"),
                        update_idx,
                    )

        final_lineage_metadata = build_lineage_metadata(args, trace)
        state.lineage_metadata = dict(final_lineage_metadata)

        output_path = Path(args.output).expanduser()
        final_payload = state.checkpoint_payload()
        atomic_write_bytes(output_path, final_payload)
        LOGGER.info("Saved trained checkpoint to %s", output_path)

        if args.mongo_save_final:
            if args.mongo_uri and args.fingerprint_hash:
                if mongo_store is None:
                    raise RuntimeError("Mongo final save requested but Mongo store is not initialized")
                saved = persist_checkpoint_to_mongo(
                    mongo_store,
                    args,
                    final_payload,
                    checkpoint_kind="final",
                    update_idx=args.updates,
                    training_steps=state.training_steps,
                    lineage_metadata=final_lineage_metadata,
                    local_checkpoint_path=output_path,
                )
                update_state_model_version_from_mongo(state, saved)
                LOGGER.info("Persisted final checkpoint to Mongo version v%s", saved.get("version"))
            elif args.mongo_uri or args.fingerprint_hash:
                LOGGER.warning(
                    "Skipping final Mongo checkpoint persistence; both --mongo-uri and --fingerprint-hash are required"
                )
    finally:
        if mongo_store is not None:
            mongo_store.close()


if __name__ == "__main__":
    main()
