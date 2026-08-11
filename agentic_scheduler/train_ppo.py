from __future__ import annotations

import argparse
import logging
from pathlib import Path

import numpy as np
import torch

from .model import build_fresh_state, choose_action, ppo_update
from .persistence import MongoSchedulerModelStore
from .training.scheduler_env import SchedulingEnv


LOGGER = logging.getLogger(__name__)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train PPO scheduler with Gymnasium + PyTorch")
    parser.add_argument("--num-workers", type=int, default=4)
    parser.add_argument("--episode-length", type=int, default=96)
    parser.add_argument("--rollout-steps", type=int, default=1024)
    parser.add_argument("--updates", type=int, default=200)
    parser.add_argument("--gamma", type=float, default=0.99)
    parser.add_argument("--learning-rate", type=float, default=3e-4)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--output", default="agentic_scheduler/models/ppo_trained.pt")
    parser.add_argument("--log-every", type=int, default=10)
    parser.add_argument("--mongo-uri", default="")
    parser.add_argument("--mongo-db", default="cluster_db")
    parser.add_argument("--fingerprint-hash", default="")
    parser.add_argument("--fingerprint-payload", default="")
    return parser.parse_args()


def discounted_returns(rewards: np.ndarray, dones: np.ndarray, gamma: float) -> np.ndarray:
    out = np.zeros_like(rewards, dtype=np.float32)
    running = 0.0
    for i in reversed(range(len(rewards))):
        if dones[i]:
            running = 0.0
        running = rewards[i] + gamma * running
        out[i] = running
    return out


def main() -> None:
    args = parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
    np.random.seed(args.seed)
    torch.manual_seed(args.seed)

    env = SchedulingEnv(num_workers=args.num_workers, episode_length=args.episode_length, seed=args.seed)
    state = build_fresh_state(args.learning_rate, torch.device("cpu"))

    observation, _ = env.reset(seed=args.seed)
    recent_rewards = []

    for update_idx in range(1, args.updates + 1):
        transitions = []
        step_rewards = []

        for _ in range(args.rollout_steps):
            task_features = observation["task"]
            worker_features = observation["workers"]
            action_mask = observation["action_mask"].astype(bool)

            if not action_mask.any():
                action = int(np.random.randint(0, env.num_workers))
                old_log_prob = 0.0
                old_value = 0.0
            else:
                action_info = choose_action(
                    state,
                    task_features=task_features,
                    worker_features=worker_features,
                    action_mask=action_mask,
                    device=torch.device("cpu"),
                )
                if action_info is None:
                    feasible_ids = np.where(action_mask)[0]
                    action = int(feasible_ids[0]) if feasible_ids.size else 0
                    old_log_prob = 0.0
                    old_value = 0.0
                else:
                    action = int(action_info["action_index"])
                    old_log_prob = float(action_info["log_prob"])
                    old_value = float(action_info["value"])
                    worker_features = np.asarray(action_info["normalized_worker_features"], dtype=np.float32)

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
        returns = discounted_returns(rewards, dones, args.gamma)
        old_values = np.asarray([x["old_value"] for x in transitions], dtype=np.float32)
        advantages = returns - old_values
        advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)

        batch = {
            "task_features": torch.as_tensor(
                np.stack([x["task_features"] for x in transitions], axis=0),
                dtype=torch.float32,
            ),
            "worker_features": torch.as_tensor(
                np.stack([x["worker_features"] for x in transitions], axis=0),
                dtype=torch.float32,
            ),
            "action_masks": torch.as_tensor(
                np.stack([x["action_mask"] for x in transitions], axis=0),
                dtype=torch.bool,
            ),
            "actions": torch.as_tensor(np.asarray([x["action"] for x in transitions], dtype=np.int64), dtype=torch.long),
            "old_log_probs": torch.as_tensor(
                np.asarray([x["old_log_prob"] for x in transitions], dtype=np.float32),
                dtype=torch.float32,
            ),
            "returns": torch.as_tensor(returns, dtype=torch.float32),
            "advantages": torch.as_tensor(advantages, dtype=torch.float32),
        }

        ppo_update(
            state,
            batch=batch,
            clip_ratio=0.2,
            entropy_coeff=0.01,
            value_coeff=0.5,
            epochs=6,
        )

        recent_rewards.extend(step_rewards)
        if len(recent_rewards) > 5000:
            recent_rewards = recent_rewards[-5000:]

        if update_idx % args.log_every == 0 or update_idx == 1:
            LOGGER.info(
                "update=%d avg_reward=%.4f model_steps=%d",
                update_idx,
                float(np.mean(recent_rewards) if recent_rewards else 0.0),
                state.training_steps,
            )

    output_path = Path(args.output).expanduser()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(state.checkpoint_payload())
    LOGGER.info("Saved trained checkpoint to %s", output_path)

    if args.mongo_uri and args.fingerprint_hash:
        store = MongoSchedulerModelStore(args.mongo_uri, args.mongo_db)
        try:
            saved = store.save_and_activate_checkpoint(
                scheduler_type="PPO",
                fingerprint_hash=args.fingerprint_hash,
                fingerprint_payload=args.fingerprint_payload,
                checkpoint_bytes=output_path.read_bytes(),
                framework="pytorch-ppo",
                extra_metadata={"source": "offline-training", "updates": args.updates},
            )
            if saved:
                LOGGER.info("Persisted checkpoint to Mongo version v%s", saved.get("version"))
        finally:
            store.close()


if __name__ == "__main__":
    main()

