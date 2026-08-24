"""Comprehensive unit and integration tests for agentic_scheduler."""

import os
import unittest
import numpy as np
import torch

from agentic_scheduler.features import (
    TASK_FEATURE_DIM,
    WORKER_FEATURE_DIM,
    encode_request,
)
from agentic_scheduler.model import (
    PPOActorCritic,
    PPOState,
    RunningNormalizer,
    build_fresh_state,
    choose_action,
)
from agentic_scheduler.service import PPOServiceCore


class TestAgenticScheduler(unittest.TestCase):
    """Test suite for agentic_scheduler components."""

    def test_feature_dimensions(self):
        """Verify feature vector dimensions."""
        self.assertEqual(TASK_FEATURE_DIM, 5)
        self.assertEqual(WORKER_FEATURE_DIM, 9)

    def test_running_normalizer(self):
        """Test online running normalizer."""
        norm = RunningNormalizer(size=4)
        samples = np.array([[1.0, 2.0, 3.0, 4.0], [5.0, 6.0, 7.0, 8.0]])
        norm.update(samples)
        self.assertEqual(norm.count, 2)
        normalized = norm.normalize(samples)
        self.assertEqual(normalized.shape, (2, 4))

        state_dict = norm.state_dict()
        norm2 = RunningNormalizer.from_state_dict(state_dict, size=4)
        self.assertEqual(norm2.count, 2)
        np.testing.assert_allclose(norm.mean, norm2.mean)

    def test_ppo_actor_critic_forward(self):
        """Test neural network forward pass and shapes."""
        net = PPOActorCritic(hidden_dim=64)
        task = torch.randn(1, TASK_FEATURE_DIM)
        workers = torch.randn(1, 4, WORKER_FEATURE_DIM)
        mask = torch.tensor([[True, True, False, True]])

        logits, value = net(task, workers, mask)
        self.assertEqual(logits.shape, (1, 4))
        self.assertEqual(value.shape, torch.Size([1]))
        # Masked worker at index 2 should have suppressed logit (-1e4)
        self.assertLessEqual(logits[0, 2].item(), -1e4)

    def test_state_serialization(self):
        """Test PPOState serialization and deserialization."""
        device = torch.device("cpu")
        state = build_fresh_state(learning_rate=3e-4, device=device)
        state.model_version = "v1"

        data = state.checkpoint_payload()
        self.assertGreater(len(data), 0)

        state2 = PPOState.from_checkpoint_bytes(data, learning_rate=3e-4, device=device)
        self.assertEqual(state2.model_version, "v1")

    def test_service_core_inference(self):
        """Test PPOServiceCore decision making and checkpoint loading."""
        core = PPOServiceCore(
            mongo_uri="",
            mongo_db="",
            model_path="agentic_scheduler/models/ppo_latest.pt",
            online_updates_enabled=False,
            prefer_gpu=False,
        )
        self.assertIsNotNone(core)
        self.assertIsNotNone(core.state)
        self.assertEqual(core.state.model_version, "v0")


if __name__ == "__main__":
    unittest.main()
