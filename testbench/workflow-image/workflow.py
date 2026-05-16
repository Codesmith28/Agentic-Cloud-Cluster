#!/usr/bin/env python3
"""Deterministic workflow entrypoint for benchmark/testbench tasks."""

from __future__ import annotations

import argparse
import json
import signal
import sys
import time
from pathlib import Path


DEFAULT_RESULT_FILE = Path("/output/workflow-result.json")


def resolve_iterations(configured_iterations: int, seconds: float, fallback_iterations: int, per_second_iterations: int) -> int:
    if seconds > 0:
        return max(int(seconds * per_second_iterations), 1)
    if configured_iterations > 0:
        return configured_iterations
    return fallback_iterations


def resolve_memory_mib(memory_mib: int, memory_mb: int, fallback: int) -> int:
    if memory_mb > 0:
        return memory_mb
    if memory_mib > 0:
        return memory_mib
    return fallback


def deterministic_cpu(iterations: int, seed: int) -> int:
    accumulator = seed & 0xFFFFFFFF
    for step in range(max(iterations, 1)):
        accumulator = (accumulator * 1664525 + 1013904223 + step) & 0xFFFFFFFF
        accumulator ^= (accumulator >> 13)
    return accumulator & 0xFFFFFFFF


def deterministic_memory(memory_mib: int, passes: int, seed: int) -> int:
    size_bytes = max(memory_mib, 1) * 1024 * 1024
    stride = 4096
    memory = bytearray(size_bytes)
    checksum = seed & 0xFFFFFFFF

    for current_pass in range(max(passes, 1)):
        base = (seed + current_pass * 31) & 0xFF
        for offset in range(0, size_bytes, stride):
            value = (base + offset // stride) & 0xFF
            memory[offset] = value
            checksum = (checksum + value + offset) & 0xFFFFFFFF

        for offset in range(0, size_bytes, stride):
            checksum = ((checksum << 5) - checksum + memory[offset]) & 0xFFFFFFFF

    return checksum & 0xFFFFFFFF


def write_result(profile: str, args: argparse.Namespace, started_at: float, details: dict[str, object]) -> None:
    result_path = Path(args.result_file)
    result_path.parent.mkdir(parents=True, exist_ok=True)

    payload = {
        "profile": profile,
        "label": getattr(args, "label", ""),
        "seed": args.seed,
        "started_at_unix": round(started_at, 3),
        "finished_at_unix": round(time.time(), 3),
        "duration_seconds": round(time.time() - started_at, 3),
        "details": details,
    }

    result_path.write_text(json.dumps(payload, indent=2), encoding="utf-8")


def run_cpu_light(args: argparse.Namespace) -> int:
    started_at = time.time()
    iterations = resolve_iterations(args.iterations, args.seconds, 450000, 220000)
    checksum = deterministic_cpu(iterations, args.seed)
    print(f"cpu-light complete iterations={iterations} checksum={checksum}", flush=True)
    write_result(
        "cpu-light",
        args,
        started_at,
        {"iterations": iterations, "checksum": checksum},
    )
    return 0


def run_cpu_heavy(args: argparse.Namespace) -> int:
    started_at = time.time()
    iterations = resolve_iterations(args.iterations, args.seconds, 4200000, 650000)
    checksum = deterministic_cpu(iterations, args.seed)
    print(f"cpu-heavy complete iterations={iterations} checksum={checksum}", flush=True)
    write_result(
        "cpu-heavy",
        args,
        started_at,
        {"iterations": iterations, "checksum": checksum},
    )
    return 0


def run_memory_heavy(args: argparse.Namespace) -> int:
    started_at = time.time()
    memory_mib = resolve_memory_mib(args.memory_mib, args.memory_mb, 512)
    checksum = deterministic_memory(memory_mib, args.passes, args.seed)
    print(
        f"memory-heavy complete memory_mib={memory_mib} passes={args.passes} checksum={checksum}",
        flush=True,
    )
    write_result(
        "memory-heavy",
        args,
        started_at,
        {
            "memory_mib": memory_mib,
            "passes": args.passes,
            "checksum": checksum,
        },
    )
    return 0


def run_mixed(args: argparse.Namespace) -> int:
    started_at = time.time()
    iterations = resolve_iterations(args.iterations, args.seconds, 1800000, 320000)
    memory_mib = resolve_memory_mib(args.memory_mib, args.memory_mb, 256)
    cpu_checksum = deterministic_cpu(iterations, args.seed)
    mem_checksum = deterministic_memory(memory_mib, args.passes, args.seed)
    combined_checksum = (cpu_checksum ^ mem_checksum) & 0xFFFFFFFF
    print(
        "mixed complete "
        f"iterations={iterations} memory_mib={memory_mib} "
        f"passes={args.passes} checksum={combined_checksum}",
        flush=True,
    )
    write_result(
        "mixed",
        args,
        started_at,
        {
            "iterations": iterations,
            "memory_mib": memory_mib,
            "passes": args.passes,
            "cpu_checksum": cpu_checksum,
            "memory_checksum": mem_checksum,
            "combined_checksum": combined_checksum,
        },
    )
    return 0


def run_exit_nonzero(args: argparse.Namespace) -> int:
    started_at = time.time()
    exit_code = args.exit_code if args.exit_code is not None else args.code
    print(f"exit-nonzero requested code={exit_code}", flush=True)
    write_result("exit-nonzero", args, started_at, {"requested_exit_code": exit_code})
    return exit_code


def run_hang(args: argparse.Namespace) -> int:
    started_at = time.time()
    terminate_requested = False

    def _request_stop(_sig: int, _frame: object) -> None:
        nonlocal terminate_requested
        terminate_requested = True

    signal.signal(signal.SIGTERM, _request_stop)
    signal.signal(signal.SIGINT, _request_stop)

    tick = 0
    while True:
        tick += 1
        elapsed = tick * args.tick_seconds
        print(f"hang tick={tick} elapsed={elapsed:.1f}s", flush=True)

        if args.max_seconds > 0 and elapsed >= args.max_seconds:
            break
        if terminate_requested:
            break

        time.sleep(args.tick_seconds)

    write_result(
        "hang",
        args,
        started_at,
        {
            "ticks": tick,
            "tick_seconds": args.tick_seconds,
            "max_seconds": args.max_seconds,
            "terminated": terminate_requested,
        },
    )
    return 0


def run_slow_start(args: argparse.Namespace) -> int:
    started_at = time.time()
    delay_seconds = args.startup_delay if args.startup_delay > 0 else args.delay_seconds
    iterations = resolve_iterations(args.iterations, args.seconds, 700000, 220000)
    print(f"slow-start sleeping for {delay_seconds:.1f}s before cpu workload", flush=True)
    time.sleep(delay_seconds)

    checksum = deterministic_cpu(iterations, args.seed)
    print(
        f"slow-start complete delay={delay_seconds:.1f}s iterations={iterations} checksum={checksum}",
        flush=True,
    )
    write_result(
        "slow-start",
        args,
        started_at,
        {
            "delay_seconds": delay_seconds,
            "iterations": iterations,
            "checksum": checksum,
        },
    )
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Deterministic workflow image entrypoint")
    subparsers = parser.add_subparsers(dest="profile", required=True)

    def add_common(subparser: argparse.ArgumentParser) -> None:
        subparser.add_argument("--seed", type=int, default=1337, help="Seed for deterministic routines")
        subparser.add_argument("--label", default="", help="Optional label for easier trace/report correlation")
        subparser.add_argument(
            "--result-file",
            default=str(DEFAULT_RESULT_FILE),
            help="Path for workflow result JSON output",
        )

    cpu_light = subparsers.add_parser("cpu-light", help="Low-intensity deterministic CPU workload")
    add_common(cpu_light)
    cpu_light.add_argument("--iterations", type=int, default=450000)
    cpu_light.add_argument("--seconds", type=float, default=0.0)
    cpu_light.set_defaults(handler=run_cpu_light)

    cpu_heavy = subparsers.add_parser("cpu-heavy", help="High-intensity deterministic CPU workload")
    add_common(cpu_heavy)
    cpu_heavy.add_argument("--iterations", type=int, default=4200000)
    cpu_heavy.add_argument("--seconds", type=float, default=0.0)
    cpu_heavy.set_defaults(handler=run_cpu_heavy)

    memory_heavy = subparsers.add_parser("memory-heavy", help="Deterministic memory pressure workload")
    add_common(memory_heavy)
    memory_heavy.add_argument("--memory-mib", type=int, default=512)
    memory_heavy.add_argument("--memory-mb", type=int, default=0)
    memory_heavy.add_argument("--passes", type=int, default=3)
    memory_heavy.set_defaults(handler=run_memory_heavy)

    mixed = subparsers.add_parser("mixed", help="Combined deterministic CPU and memory workload")
    add_common(mixed)
    mixed.add_argument("--iterations", type=int, default=1800000)
    mixed.add_argument("--seconds", type=float, default=0.0)
    mixed.add_argument("--memory-mib", type=int, default=256)
    mixed.add_argument("--memory-mb", type=int, default=0)
    mixed.add_argument("--passes", type=int, default=2)
    mixed.set_defaults(handler=run_mixed)

    exit_nonzero = subparsers.add_parser("exit-nonzero", help="Exit immediately with a non-zero status")
    add_common(exit_nonzero)
    exit_nonzero.add_argument("--code", type=int, default=17)
    exit_nonzero.add_argument("--exit-code", type=int)
    exit_nonzero.set_defaults(handler=run_exit_nonzero)

    hang = subparsers.add_parser("hang", help="Keep running until terminated (or max-seconds is reached)")
    add_common(hang)
    hang.add_argument("--tick-seconds", type=float, default=5.0)
    hang.add_argument("--max-seconds", type=float, default=0.0)
    hang.set_defaults(handler=run_hang)

    slow_start = subparsers.add_parser("slow-start", help="Delay first, then execute deterministic CPU work")
    add_common(slow_start)
    slow_start.add_argument("--delay-seconds", type=float, default=20.0)
    slow_start.add_argument("--startup-delay", type=float, default=0.0)
    slow_start.add_argument("--seconds", type=float, default=0.0)
    slow_start.add_argument("--iterations", type=int, default=700000)
    slow_start.set_defaults(handler=run_slow_start)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return int(args.handler(args))


if __name__ == "__main__":
    sys.exit(main())
