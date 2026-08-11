#!/usr/bin/env python3

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

"""Set the active scheduler algorithm on the master node."""

import argparse
import sys
import requests


def main():
    parser = argparse.ArgumentParser(description="Set scheduler algorithm")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master URL")
    parser.add_argument("--scheduler", required=True, choices=["RR", "RTS", "PPO"],
                        help="Scheduler algorithm")
    parser.add_argument("--model", default="", help="Path to PPO model (required for PPO)")
    parser.add_argument("--online-mode", action="store_true", help="Enable online learning mode")
    parser.add_argument("--replay-batch-size", type=int, default=100, help="Online replay batch size")
    parser.add_argument("--online-lr-scale", type=float, default=0.1, help="Online learning rate scale")
    args = parser.parse_args()

    payload = {
        "algorithm": args.scheduler,
    }
    if args.model:
        payload["model_path"] = args.model
    if args.online_mode:
        payload["online_mode"] = True
        payload["replay_batch_size"] = args.replay_batch_size
        payload["online_lr_scale"] = args.online_lr_scale

    try:
        resp = requests.post(
            f"{args.master_url}/api/config/scheduler",
            json=payload,
            timeout=5,
        )
        if resp.status_code in (200, 201):
            mode = "online" if args.online_mode else "offline"
            print(f"Scheduler set to {args.scheduler} ({mode})")
            return 0
        else:
            print(f"Failed to set scheduler: {resp.status_code} {resp.text}", file=sys.stderr)
            return 1
    except Exception as e:
        print(f"Failed to set scheduler: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
