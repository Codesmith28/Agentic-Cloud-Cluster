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

"""Register a single worker with the master node."""

import argparse
import json
import sys
import requests


def main():
    parser = argparse.ArgumentParser(description="Register worker with master")
    parser.add_argument("--worker-name", required=True, help="Worker ID")
    parser.add_argument("--worker-addr", required=True, help="Worker address (host:port)")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master URL")
    args = parser.parse_args()

    payload = {
        "worker_id": args.worker_name,
        "worker_ip": args.worker_addr,
    }

    try:
        resp = requests.post(
            f"{args.master_url}/api/workers",
            json=payload,
            timeout=5,
        )
        if resp.status_code in (200, 201):
            print(f"Registered {args.worker_name} at {args.worker_addr}")
            return 0
        else:
            print(f"Failed to register {args.worker_name}: {resp.status_code} {resp.text}", file=sys.stderr)
            return 1
    except Exception as e:
        print(f"Failed to register {args.worker_name}: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
