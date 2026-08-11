# Copyright 2025-2026 Sarthak Siddhpura
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from __future__ import annotations

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
        for row in data:
            self.count += 1
            delta = row - self.mean
            self.mean += delta / self.count
            delta2 = row - self.mean
            self.m2 += delta * delta2

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
    task_tensor = torch.as_tensor(task_features[None, :], dtype=torch.float32, device=device)
    worker_tensor = torch.as_tensor(normalized_worker[None, :, :], dtype=torch.float32, device=device)
    mask_tensor = torch.as_tensor(action_mask[None, :], dtype=torch.bool, device=device)

    with torch.no_grad():
        logits, value = state.model(task_tensor, worker_tensor, mask_tensor)
        distribution = Categorical(logits=logits)
        action = torch.argmax(logits, dim=-1)
        log_prob = distribution.log_prob(action)

    return {
        "action_index": int(action.item()),
        "log_prob": float(log_prob.item()),
        "value": float(value.item()),
        "normalized_worker_features": normalized_worker,
    }


def ppo_update(
    state: PPOState,
    batch: Dict[str, torch.Tensor],
    clip_ratio: float,
    entropy_coeff: float,
    value_coeff: float,
    epochs: int,
):
    actions = batch["actions"]
    old_log_probs = batch["old_log_probs"]
    returns = batch["returns"]
    advantages = batch["advantages"]
    task_features = batch["task_features"]
    worker_features = batch["worker_features"]
    action_masks = batch["action_masks"]

    for _ in range(epochs):
        logits, values = state.model(task_features, worker_features, action_masks)
        distribution = Categorical(logits=logits)
        new_log_probs = distribution.log_prob(actions)
        entropy = distribution.entropy().mean()

        ratio = torch.exp(new_log_probs - old_log_probs)
        surrogate_1 = ratio * advantages
        surrogate_2 = torch.clamp(ratio, 1.0 - clip_ratio, 1.0 + clip_ratio) * advantages
        policy_loss = -torch.min(surrogate_1, surrogate_2).mean()
        value_loss = F.mse_loss(values, returns)

        loss = policy_loss + (value_coeff * value_loss) - (entropy_coeff * entropy)

        state.optimizer.zero_grad()
        loss.backward()
        nn.utils.clip_grad_norm_(state.model.parameters(), max_norm=1.0)
        state.optimizer.step()

    state.training_steps += 1
