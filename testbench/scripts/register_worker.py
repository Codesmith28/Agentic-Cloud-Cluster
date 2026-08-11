#!/usr/bin/env python3


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
