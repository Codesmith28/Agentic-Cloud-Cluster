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

import logging
import os
import tempfile
import threading
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional, Tuple

import numpy as np
import torch

from .features import encode_request
from .model import PPOState, build_fresh_state, choose_action, ppo_update
from .persistence import MongoSchedulerModelStore


LOGGER = logging.getLogger(__name__)


@dataclass
class DecisionRecord:
    task_features: np.ndarray
    worker_features: np.ndarray
    action_mask: np.ndarray
    action_index: int
    old_log_prob: float
    old_value: float
    worker_id: str


class PPOServiceCore:
    """Holds PPO policy lifecycle + persistence + online updates."""

    def __init__(
        self,
        mongo_uri: str,
        mongo_db: str,
        model_path: str,
        learning_rate: float = 3e-4,
        update_batch_size: int = 32,
        online_updates_enabled: bool = True,
    ):
        self.learning_rate = float(learning_rate)
        self.update_batch_size = max(int(update_batch_size), 4)
        self.online_updates_enabled = bool(online_updates_enabled)
        self.model_path = Path(model_path).expanduser() if model_path else None
        self.device = torch.device("cpu")
        self.scheduler_type = "PPO"

        self.store = MongoSchedulerModelStore(mongo_uri, mongo_db)
        self.lock = threading.RLock()
        self.state: PPOState = build_fresh_state(self.learning_rate, self.device)
        self.current_fingerprint_hash = ""
        self.current_fingerprint_payload = ""
        self.pending_decisions: Dict[str, DecisionRecord] = {}
        self.replay_buffer: List[Dict] = []

        if self.model_path and self.model_path.exists():
            self._load_from_local_path_locked(self.model_path)

        LOGGER.info(
            "PPO service initialized (online_updates_enabled=%s, update_batch_size=%d)",
            self.online_updates_enabled,
            self.update_batch_size,
        )

    def close(self) -> None:
        self.store.close()

    def ping(self) -> Tuple[bool, str, str, str]:
        with self.lock:
            return True, "ppo-service-ok", self.current_fingerprint_hash, self.state.model_version

    def ensure_fingerprint_loaded(
        self,
        fingerprint_hash: str,
        fingerprint_payload: str,
        create_if_missing: bool,
    ) -> Tuple[bool, bool, str, str]:
        with self.lock:
            if fingerprint_hash and fingerprint_hash == self.current_fingerprint_hash:
                return True, False, self.state.model_version, "fingerprint already loaded"

            loaded_from_store = self._load_from_store_locked(fingerprint_hash)
            if loaded_from_store is not None:
                version, message = loaded_from_store
                self.current_fingerprint_hash = fingerprint_hash
                self.current_fingerprint_payload = fingerprint_payload
                return True, False, version, message

            if self.model_path and self.model_path.exists():
                self._load_from_local_path_locked(self.model_path)
                self.current_fingerprint_hash = fingerprint_hash
                self.current_fingerprint_payload = fingerprint_payload

                if create_if_missing:
                    self._persist_current_state_locked("import-local")
                return True, False, self.state.model_version, f"loaded local checkpoint {self.model_path}"

            self.state = build_fresh_state(self.learning_rate, self.device)
            self.state.fingerprint_hash = fingerprint_hash
            self.current_fingerprint_hash = fingerprint_hash
            self.current_fingerprint_payload = fingerprint_payload
            self.pending_decisions.clear()
            self.replay_buffer.clear()

            if create_if_missing:
                self._persist_current_state_locked("cold-start")

            return True, True, self.state.model_version, "cold-started fresh PPO policy"

    def select_worker(self, task, workers, fallback_scheduler: str):
        with self.lock:
            encoded = encode_request(task, workers)
            if encoded.worker_features.shape[0] == 0:
                return "", True, "no workers provided", self.state.model_version
            if not encoded.action_mask.any():
                return "", True, "no feasible workers", self.state.model_version

            action = choose_action(
                self.state,
                encoded.task_features,
                encoded.worker_features,
                encoded.action_mask,
                self.device,
            )
            if action is None:
                return "", True, "policy produced no action", self.state.model_version

            index = int(action["action_index"])
            if index < 0 or index >= len(workers):
                return "", True, "policy returned invalid index", self.state.model_version
            if not bool(encoded.action_mask[index]):
                return "", True, "policy selected infeasible worker", self.state.model_version

            worker_id = workers[index].worker_id
            task_id = getattr(task, "task_id", "")
            if task_id:
                self.pending_decisions[task_id] = DecisionRecord(
                    task_features=encoded.task_features.copy(),
                    worker_features=np.asarray(action["normalized_worker_features"], dtype=np.float32).copy(),
                    action_mask=encoded.action_mask.copy(),
                    action_index=index,
                    old_log_prob=float(action["log_prob"]),
                    old_value=float(action["value"]),
                    worker_id=worker_id,
                )

            return worker_id, False, f"selected-by-ppo (fallback={fallback_scheduler})", self.state.model_version

    def report_outcome(
        self,
        task_id: str,
        worker_id: str,
        status: str,
        reward: float,
        runtime_seconds: float,
        sla_success: bool,
        fingerprint_hash: str,
    ) -> Tuple[bool, str]:
        with self.lock:
            if fingerprint_hash and fingerprint_hash != self.current_fingerprint_hash:
                return False, "fingerprint mismatch for reported outcome"

            decision = self.pending_decisions.pop(task_id, None)
            if decision is None:
                return False, "no matching pending decision for task"
            if worker_id and decision.worker_id != worker_id:
                return False, "worker mismatch for task outcome"

            resolved_reward = float(reward)
            if resolved_reward == 0.0:
                resolved_reward = self._derive_reward(status, runtime_seconds, sla_success)

            if not self.online_updates_enabled:
                return True, "outcome accepted (online updates disabled)"

            self.replay_buffer.append(
                {
                    "task_features": decision.task_features,
                    "worker_features": decision.worker_features,
                    "action_mask": decision.action_mask,
                    "action_index": decision.action_index,
                    "old_log_prob": decision.old_log_prob,
                    "old_value": decision.old_value,
                    "reward": resolved_reward,
                }
            )

            if len(self.replay_buffer) >= self.update_batch_size:
                self._train_from_replay_locked()

            return True, "outcome accepted"

    def _derive_reward(self, status: str, runtime_seconds: float, sla_success: bool) -> float:
        reward = 0.0
        normalized_status = (status or "").lower()
        if normalized_status in {"success", "completed"}:
            reward += 1.0
        elif normalized_status == "cancelled":
            reward -= 0.5
        else:
            reward -= 1.0

        if sla_success:
            reward += 0.5
        else:
            reward -= 0.25

        if runtime_seconds > 0:
            reward -= min(runtime_seconds / 600.0, 0.5)
        return reward

    def _train_from_replay_locked(self) -> None:
        if not self.replay_buffer:
            return
        batch_size = len(self.replay_buffer)
        worker_count = self.replay_buffer[0]["worker_features"].shape[0]

        task_features = torch.as_tensor(
            np.stack([item["task_features"] for item in self.replay_buffer], axis=0),
            dtype=torch.float32,
            device=self.device,
        )
        worker_features = torch.as_tensor(
            np.stack([item["worker_features"] for item in self.replay_buffer], axis=0),
            dtype=torch.float32,
            device=self.device,
        )
        action_masks = torch.as_tensor(
            np.stack([item["action_mask"] for item in self.replay_buffer], axis=0),
            dtype=torch.bool,
            device=self.device,
        )
        actions = torch.as_tensor(
            np.asarray([item["action_index"] for item in self.replay_buffer], dtype=np.int64),
            dtype=torch.long,
            device=self.device,
        )
        old_log_probs = torch.as_tensor(
            np.asarray([item["old_log_prob"] for item in self.replay_buffer], dtype=np.float32),
            dtype=torch.float32,
            device=self.device,
        )
        old_values = torch.as_tensor(
            np.asarray([item["old_value"] for item in self.replay_buffer], dtype=np.float32),
            dtype=torch.float32,
            device=self.device,
        )
        rewards = torch.as_tensor(
            np.asarray([item["reward"] for item in self.replay_buffer], dtype=np.float32),
            dtype=torch.float32,
            device=self.device,
        )

        returns = rewards
        advantages = returns - old_values
        advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)

        batch = {
            "task_features": task_features,
            "worker_features": worker_features.reshape(batch_size, worker_count, -1),
            "action_masks": action_masks,
            "actions": actions,
            "old_log_probs": old_log_probs,
            "returns": returns,
            "advantages": advantages,
        }

        ppo_update(
            self.state,
            batch=batch,
            clip_ratio=0.2,
            entropy_coeff=0.01,
            value_coeff=0.5,
            epochs=4,
        )
        self.replay_buffer.clear()
        self._persist_current_state_locked("online-update")

    def _load_from_store_locked(self, fingerprint_hash: str) -> Optional[Tuple[str, str]]:
        if not fingerprint_hash:
            return None
        loaded = self.store.load_active_checkpoint(self.scheduler_type, fingerprint_hash)
        if not loaded:
            return None

        metadata, payload = loaded
        self.state = PPOState.from_checkpoint_bytes(payload, self.learning_rate, self.device)
        version_number = int(metadata.get("version", 0))
        self.state.model_version = f"v{version_number}" if version_number > 0 else self.state.model_version
        self.state.fingerprint_hash = fingerprint_hash
        self.pending_decisions.clear()
        self.replay_buffer.clear()
        return self.state.model_version, "loaded checkpoint from mongo"

    def _load_from_local_path_locked(self, checkpoint_path: Path) -> None:
        payload = checkpoint_path.read_bytes()
        self.state = PPOState.from_checkpoint_bytes(payload, self.learning_rate, self.device)
        self.pending_decisions.clear()
        self.replay_buffer.clear()
        LOGGER.info("Loaded PPO checkpoint from local path %s", checkpoint_path)

    def _persist_current_state_locked(self, reason: str) -> None:
        payload = self.state.checkpoint_payload()

        if self.model_path:
            self.model_path.parent.mkdir(parents=True, exist_ok=True)
            with tempfile.NamedTemporaryFile(
                mode="wb",
                prefix="ppo_ckpt_",
                suffix=".tmp",
                dir=str(self.model_path.parent),
                delete=False,
            ) as handle:
                handle.write(payload)
                temp_name = handle.name
            os.replace(temp_name, self.model_path)

        if not self.current_fingerprint_hash:
            return

        doc = self.store.save_and_activate_checkpoint(
            scheduler_type=self.scheduler_type,
            fingerprint_hash=self.current_fingerprint_hash,
            fingerprint_payload=self.current_fingerprint_payload,
            checkpoint_bytes=payload,
            framework="pytorch-ppo",
            extra_metadata=self._build_persistence_metadata(reason),
        )
        if doc:
            version = int(doc.get("version", 0))
            if version > 0:
                self.state.model_version = f"v{version}"

    def _build_persistence_metadata(self, reason: str) -> Dict[str, object]:
        return {
            "reason": reason,
            "training_steps": self.state.training_steps,
            "model_source": "live-online-adaptation" if self.online_updates_enabled else "runtime-checkpoint",
            "training_corpus": "cloudai-live-outcomes",
            "cluster_fingerprint": self.current_fingerprint_hash,
            "train_timestamp": datetime.now(timezone.utc).isoformat(),
            "online_updates_enabled": self.online_updates_enabled,
        }
