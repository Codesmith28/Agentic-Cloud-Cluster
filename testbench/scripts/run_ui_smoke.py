#!/usr/bin/env python3


"""Light UI smoke checks for CloudAI."""

from __future__ import annotations

import argparse
import json
import pathlib
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, asdict
from http.cookiejar import CookieJar
from typing import Any

from shared_polling import request_json


@dataclass
class CheckResult:
    name: str
    success: bool
    detail: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run light CloudAI UI smoke checks")
    parser.add_argument("--master-url", default="http://localhost:8080", help="Master API URL")
    parser.add_argument("--ui-url", default="http://localhost:3000", help="UI base URL")
    parser.add_argument("--output", required=True, type=pathlib.Path, help="Output summary JSON path")
    parser.add_argument("--user-name", default="ui-smoke", help="Test user display name")
    parser.add_argument("--user-email", default="ui-smoke@example.com", help="Test user email")
    parser.add_argument("--user-password", default="ui-smoke-password-123", help="Test user password")
    return parser.parse_args()


def http_get_text(url: str, timeout: float = 10.0) -> tuple[int, str]:
    req = urllib.request.Request(url=url, method="GET")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return resp.getcode(), resp.read().decode("utf-8", errors="replace")


def auth_open(opener: urllib.request.OpenerDirector, method: str, url: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url=url, method=method, data=data, headers=headers)
    with opener.open(req, timeout=12.0) as resp:
        body = resp.read().decode("utf-8")
        return json.loads(body) if body else {}


def run_checks(args: argparse.Namespace) -> tuple[bool, list[CheckResult]]:
    checks: list[CheckResult] = []
    master_url = args.master_url.rstrip("/")
    ui_url = args.ui_url.rstrip("/")

    # API health
    try:
        health = request_json("GET", f"{master_url}/health")
        ok = str(health.get("status", "")).lower() == "healthy"
        checks.append(CheckResult("api-health", ok, json.dumps(health)))
    except Exception as exc:  # pylint: disable=broad-except
        checks.append(CheckResult("api-health", False, f"{type(exc).__name__}: {exc}"))

    # Worker + task endpoints
    for name, endpoint in [("api-workers", "/api/workers"), ("api-tasks", "/api/tasks")]:
        try:
            payload = request_json("GET", f"{master_url}{endpoint}")
            checks.append(CheckResult(name, True, f"keys={sorted(payload.keys())}"))
        except Exception as exc:  # pylint: disable=broad-except
            checks.append(CheckResult(name, False, f"{type(exc).__name__}: {exc}"))

    # UI root page
    try:
        status, body = http_get_text(f"{ui_url}/")
        body_lower = body.lower()
        ok = status == 200 and ("<html" in body_lower or "vite" in body_lower or "react" in body_lower)
        checks.append(CheckResult("ui-root", ok, f"status={status} length={len(body)}"))
    except Exception as exc:  # pylint: disable=broad-except
        checks.append(CheckResult("ui-root", False, f"{type(exc).__name__}: {exc}"))

    # Auth flow smoke (register/login/me)
    jar = CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    register_payload = {
        "name": args.user_name,
        "email": args.user_email,
        "password": args.user_password,
    }
    login_payload = {
        "email": args.user_email,
        "password": args.user_password,
    }

    try:
        register_resp = auth_open(opener, "POST", f"{master_url}/api/auth/register", register_payload)
        registered = bool(register_resp.get("success")) or "already" in str(register_resp.get("message", "")).lower()
        checks.append(CheckResult("auth-register", registered, json.dumps(register_resp)))
    except urllib.error.HTTPError as exc:
        # Existing user can return 4xx depending deployment; login is the authoritative check.
        checks.append(CheckResult("auth-register", True, f"ignored-http-{exc.code}"))
    except Exception as exc:  # pylint: disable=broad-except
        checks.append(CheckResult("auth-register", False, f"{type(exc).__name__}: {exc}"))

    try:
        login_resp = auth_open(opener, "POST", f"{master_url}/api/auth/login", login_payload)
        checks.append(CheckResult("auth-login", bool(login_resp.get("success")), json.dumps(login_resp)))
    except Exception as exc:  # pylint: disable=broad-except
        checks.append(CheckResult("auth-login", False, f"{type(exc).__name__}: {exc}"))

    try:
        me_resp = auth_open(opener, "GET", f"{master_url}/api/auth/me")
        email = str(me_resp.get("user", {}).get("email", ""))
        checks.append(CheckResult("auth-me", email == args.user_email, json.dumps(me_resp)))
    except Exception as exc:  # pylint: disable=broad-except
        checks.append(CheckResult("auth-me", False, f"{type(exc).__name__}: {exc}"))

    return all(item.success for item in checks), checks


def main() -> int:
    args = parse_args()
    started = time.time()
    ok, checks = run_checks(args)
    finished = time.time()

    summary = {
        "suite": "ui-smoke",
        "started_at_unix": started,
        "finished_at_unix": finished,
        "duration_seconds": round(finished - started, 3),
        "success": ok,
        "checks": [asdict(item) for item in checks],
    }

    output_path = args.output.resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(summary, indent=2), encoding="utf-8")
    print(f"UI smoke summary written to {output_path}")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
