from __future__ import annotations

import logging
import io
import time
from dataclasses import dataclass
from typing import Dict, Optional

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.distributions import Categorical

from .features import TASK_FEATURE_DIM, WORKER_FEATURE_DIM

LOGGER = logging.getLogger(__name__)


class RunningNormalizer:
    """Online normalizer for pairwise (task+worker) features."""

    def __init__(self, size: int):
        self.size = size
        self.count = 0
        self.mean = np.zeros((size,), dtype=np.float64)
        self.m2 = np.zeros((size,), dtype=np.float64)

    def update(self, samples: np.ndarray) -> None:
        if samples.size == 0:
            return
        data = np.asarray(samples, dtype=np.float64)
        if data.ndim == 1:
            data = data[None, :]
        n = data.shape[0]
        if n == 0:
            return
        batch_mean = data.mean(axis=0)
        batch_var = data.var(axis=0) if n > 1 else np.zeros_like(batch_mean)
        batch_count = n

        old_count = self.count
        new_count = old_count + batch_count
        if new_count == 0:
            return
        delta = batch_mean - self.mean
        new_mean = self.mean + delta * (batch_count / new_count)
        m2_batch = batch_var * batch_count
        self.m2 = self.m2 + m2_batch + (delta ** 2) * (old_count * batch_count / new_count)
        self.mean = new_mean
        self.count = new_count

    def normalize(self, samples: np.ndarray) -> np.ndarray:
        if self.count < 2:
            return samples.astype(np.float32)
        variance = self.m2 / max(self.count - 1, 1)
        std = np.sqrt(np.maximum(variance, 1e-6))
        return ((samples - self.mean) / std).astype(np.float32)

    def state_dict(self) -> Dict:
        return {
            "size": self.size,
            "count": self.count,
            "mean": self.mean.tolist(),
            "m2": self.m2.tolist(),
        }

    @classmethod
    def from_state_dict(cls, state: Optional[Dict], size: int) -> "RunningNormalizer":
        normalizer = cls(size=size)
        if not state:
            return normalizer
        normalizer.size = int(state.get("size", size))
        normalizer.count = int(state.get("count", 0))
        normalizer.mean = np.asarray(state.get("mean", [0.0] * size), dtype=np.float64)
        normalizer.m2 = np.asarray(state.get("m2", [0.0] * size), dtype=np.float64)
        return normalizer


class PPOActorCritic(nn.Module):
    def __init__(self, hidden_dim: int = 128):
        super().__init__()
        input_dim = TASK_FEATURE_DIM + WORKER_FEATURE_DIM
        self.encoder = nn.Sequential(
            nn.Linear(input_dim, hidden_dim),
            nn.ReLU(),
            nn.Linear(hidden_dim, hidden_dim),
            nn.ReLU(),
        )
        self.policy_head = nn.Linear(hidden_dim, 1)
        self.value_head = nn.Linear(hidden_dim, 1)

    def forward(
        self,
        task_features: torch.Tensor,
        worker_features: torch.Tensor,
        action_mask: Optional[torch.Tensor] = None,
    ):
        # Shapes:
        # task_features: [B, task_dim]
        # worker_features: [B, W, worker_dim]
        # action_mask: [B, W]
        batch_size, worker_count, _ = worker_features.shape
        expanded_task = task_features.unsqueeze(1).expand(batch_size, worker_count, task_features.shape[-1])
        pairwise = torch.cat([expanded_task, worker_features], dim=-1)
        hidden = self.encoder(pairwise)
        logits = self.policy_head(hidden).squeeze(-1)
        pooled = hidden.mean(dim=1)
        values = self.value_head(pooled).squeeze(-1)

        if action_mask is not None:
            logits = logits.masked_fill(~action_mask.bool(), -1e9)
        return logits, values


@dataclass
class PPOState:
    model: PPOActorCritic
    optimizer: torch.optim.Optimizer
    normalizer: RunningNormalizer
    model_version: str
    fingerprint_hash: str
    training_steps: int
    lineage_metadata: Dict[str, str]

    def checkpoint_payload(self) -> bytes:
        payload = {
            "model_state_dict": self.model.state_dict(),
            "optimizer_state_dict": self.optimizer.state_dict(),
            "normalizer": self.normalizer.state_dict(),
            "model_version": self.model_version,
            "fingerprint_hash": self.fingerprint_hash,
            "training_steps": self.training_steps,
            "lineage_metadata": self.lineage_metadata,
            "saved_at_unix": time.time(),
        }
        buffer = io.BytesIO()
        torch.save(payload, buffer)
        return buffer.getvalue()

    @classmethod
    def from_checkpoint_bytes(
        cls,
        checkpoint_bytes: bytes,
        learning_rate: float,
        device: torch.device,
    ) -> "PPOState":
        model = PPOActorCritic().to(device)
        optimizer = torch.optim.Adam(model.parameters(), lr=learning_rate)
        normalizer = RunningNormalizer(TASK_FEATURE_DIM + WORKER_FEATURE_DIM)

        payload = torch.load(io.BytesIO(checkpoint_bytes), map_location=device, weights_only=True)
        model.load_state_dict(payload["model_state_dict"])
        optimizer_state = payload.get("optimizer_state_dict")
        if optimizer_state:
            optimizer.load_state_dict(optimizer_state)
        normalizer = RunningNormalizer.from_state_dict(
            payload.get("normalizer"),
            TASK_FEATURE_DIM + WORKER_FEATURE_DIM,
        )

        lineage_payload = payload.get("lineage_metadata") or {}
        if not isinstance(lineage_payload, dict):
            lineage_payload = {}

        return cls(
            model=model,
            optimizer=optimizer,
            normalizer=normalizer,
            model_version=str(payload.get("model_version", "v1")),
            fingerprint_hash=str(payload.get("fingerprint_hash", "")),
            training_steps=int(payload.get("training_steps", 0)),
            lineage_metadata={str(k): str(v) for k, v in lineage_payload.items()},
        )


def build_fresh_state(learning_rate: float, device: torch.device) -> PPOState:
    model = PPOActorCritic().to(device)
    optimizer = torch.optim.Adam(model.parameters(), lr=learning_rate)
    normalizer = RunningNormalizer(TASK_FEATURE_DIM + WORKER_FEATURE_DIM)
    return PPOState(
        model=model,
        optimizer=optimizer,
        normalizer=normalizer,
        model_version="v0",
        fingerprint_hash="",
        training_steps=0,
        lineage_metadata={},
    )


def choose_action(
    state: PPOState,
    task_features: np.ndarray,
    worker_features: np.ndarray,
    action_mask: np.ndarray,
    device: torch.device,
    deterministic: bool = True,
    headroom_bias: float = 0.15,
    inference_model: Optional[nn.Module] = None,
):
    if worker_features.size == 0:
        return None

    pairwise_rows = np.concatenate(
        [np.repeat(task_features[None, :], worker_features.shape[0], axis=0), worker_features],
        axis=1,
    )
    state.normalizer.update(pairwise_rows)
    normalized_rows = state.normalizer.normalize(pairwise_rows)

    normalized_worker = normalized_rows[:, TASK_FEATURE_DIM:]
    normalized_task = normalized_rows[0, :TASK_FEATURE_DIM]
    task_tensor = torch.as_tensor(normalized_task[None, :], dtype=torch.float32, device=device)
    worker_tensor = torch.as_tensor(normalized_worker[None, :, :], dtype=torch.float32, device=device)
    mask_tensor = torch.as_tensor(action_mask[None, :], dtype=torch.bool, device=device)
    headroom_bias = max(float(headroom_bias), 0.0)

    model = inference_model if inference_model is not None else state.model
    with torch.no_grad():
        policy_logits, value = model(task_tensor, worker_tensor, mask_tensor)
        selection_logits = policy_logits
        if deterministic and headroom_bias > 0.0:
            headroom_scores = _projected_headroom_scores(task_features, worker_features, action_mask)
            headroom_tensor = torch.as_tensor(headroom_scores[None, :], dtype=torch.float32, device=device)
            selection_logits = selection_logits + (headroom_bias * headroom_tensor)
        distribution = Categorical(logits=policy_logits)
        if deterministic:
            feasible_count = int(mask_tensor.sum().item())
            if headroom_bias > 0.0 and feasible_count > 1:
                rerank_k = min(3, feasible_count)
                candidate_source = policy_logits.masked_fill(~mask_tensor, float("-inf"))
                _, candidate_indices = torch.topk(candidate_source, k=rerank_k, dim=-1)
                rerank_logits = torch.full_like(selection_logits, fill_value=-1e9)
                rerank_logits.scatter_(1, candidate_indices, selection_logits.gather(1, candidate_indices))
                action = torch.argmax(rerank_logits, dim=-1)
            else:
                action = torch.argmax(selection_logits, dim=-1)
        else:
            action = distribution.sample()
        log_prob = distribution.log_prob(action)

    return {
        "action_index": int(action.item()),
        "log_prob": float(log_prob.item()),
        "value": float(value.item()),
        "normalized_worker_features": normalized_worker,
        "normalized_task_features": normalized_task,
    }


def ppo_update(
    state: PPOState,
    batch: Dict[str, torch.Tensor],
    clip_ratio: float,
    entropy_coeff: float,
    value_coeff: float,
    epochs: int,
    minibatch_size: int = 0,
    value_clip_range: float = 0.2,
    grad_scaler: Optional[torch.amp.GradScaler] = None,
):
    actions = batch["actions"]
    old_log_probs = batch["old_log_probs"]
    returns = batch["returns"]
    advantages = batch["advantages"]
    old_values = batch["old_values"]
    task_features = batch["task_features"]
    worker_features = batch["worker_features"]
    action_masks = batch["action_masks"]
    total_samples = int(actions.shape[0])
    effective_minibatch = int(minibatch_size) if int(minibatch_size) > 0 else total_samples
    effective_minibatch = max(min(effective_minibatch, total_samples), 1)

    for _ in range(epochs):
        permutation = torch.randperm(total_samples, device=actions.device)
        for start in range(0, total_samples, effective_minibatch):
            end = start + effective_minibatch
            batch_idx = permutation[start:end]

            use_amp = grad_scaler is not None
            device_type = task_features.device.type

            with torch.amp.autocast(device_type=device_type, enabled=use_amp):
                logits, values = state.model(
                    task_features[batch_idx],
                    worker_features[batch_idx],
                    action_masks[batch_idx],
                )
                distribution = Categorical(logits=logits)
                new_log_probs = distribution.log_prob(actions[batch_idx])
                entropy = distribution.entropy().mean()

                ratio = torch.exp(new_log_probs - old_log_probs[batch_idx])
                surrogate_1 = ratio * advantages[batch_idx]
                surrogate_2 = torch.clamp(ratio, 1.0 - clip_ratio, 1.0 + clip_ratio) * advantages[batch_idx]
                policy_loss = -torch.min(surrogate_1, surrogate_2).mean()

                value_targets = returns[batch_idx]
                old_value_batch = old_values[batch_idx]
                value_delta = values - old_value_batch
                clipped_values = old_value_batch + torch.clamp(value_delta, -value_clip_range, value_clip_range)
                value_loss_unclipped = F.mse_loss(values, value_targets, reduction="none")
                value_loss_clipped = F.mse_loss(clipped_values, value_targets, reduction="none")
                value_loss = torch.max(value_loss_unclipped, value_loss_clipped).mean()

                loss = policy_loss + (value_coeff * value_loss) - (entropy_coeff * entropy)

            state.optimizer.zero_grad()
            if grad_scaler is not None:
                grad_scaler.scale(loss).backward()
                grad_scaler.unscale_(state.optimizer)
                nn.utils.clip_grad_norm_(state.model.parameters(), max_norm=1.0)
                grad_scaler.step(state.optimizer)
                grad_scaler.update()
            else:
                loss.backward()
                nn.utils.clip_grad_norm_(state.model.parameters(), max_norm=1.0)
                state.optimizer.step()

    state.training_steps += 1


def _projected_headroom_scores(
    task_features: np.ndarray,
    worker_features: np.ndarray,
    action_mask: np.ndarray,
) -> np.ndarray:
    """Deterministic prior blending urgency and capacity trade-offs.

    Higher scores represent better placements under a mix of:
      - queue/tail risk control for urgent or large tasks
      - tighter packing for less urgent tasks
    Infeasible workers receive a large negative score so they stay suppressed by
    action masking.
    """
    req_cpu = float(task_features[0]) if task_features.size > 0 else 0.0
    req_memory = float(task_features[1]) if task_features.size > 1 else 0.0
    req_storage = float(task_features[2]) if task_features.size > 2 else 0.0
    sla_multiplier = float(task_features[3]) if task_features.size > 3 else 2.0

    total_cpu = np.maximum(worker_features[:, 3], 1e-6)
    total_memory = np.maximum(worker_features[:, 4], 1e-6)
    total_storage = np.maximum(worker_features[:, 5], 1e-6)

    available_cpu = worker_features[:, 0] * total_cpu
    available_memory = worker_features[:, 1] * total_memory
    available_storage = worker_features[:, 2] * total_storage

    residual_cpu = (available_cpu - req_cpu) / total_cpu
    residual_memory = (available_memory - req_memory) / total_memory
    residual_storage = (available_storage - req_storage) / total_storage

    residuals = np.stack([residual_cpu, residual_memory, residual_storage], axis=1)
    min_residual = residuals.min(axis=1)
    mean_residual = residuals.mean(axis=1)

    projected_cpu = worker_features[:, 6] + (req_cpu / total_cpu)
    projected_memory = worker_features[:, 7] + (req_memory / total_memory)
    projected_storage = worker_features[:, 8] + (req_storage / total_storage)
    projected = np.stack([projected_cpu, projected_memory, projected_storage], axis=1)
    projected_peak = projected.max(axis=1)
    projected_spread = projected.std(axis=1)

    median_cpu = max(float(np.median(total_cpu)), 1e-6)
    median_memory = max(float(np.median(total_memory)), 1e-6)
    median_storage = max(float(np.median(total_storage)), 1e-6)
    task_size = max(req_cpu / median_cpu, req_memory / median_memory, req_storage / median_storage)
    task_size = float(np.clip(task_size, 0.0, 1.5))

    sla_urgency = float(np.clip((sla_multiplier - 1.0) / 2.0, 0.0, 1.0))
    urgency = float(np.clip((0.7 * sla_urgency) + (0.3 * (task_size / 1.5)), 0.0, 1.0))

    risk_aware_score = (1.25 * min_residual) - (1.0 * projected_peak) - (0.35 * projected_spread)
    packing_score = (-0.9 * mean_residual) - (0.6 * projected_peak) - (0.2 * projected_spread)
    scores = ((urgency * risk_aware_score) + ((1.0 - urgency) * packing_score)).astype(np.float32)
    infeasible = ~np.asarray(action_mask, dtype=np.bool_)
    scores[infeasible] = -1e6
    return scores
