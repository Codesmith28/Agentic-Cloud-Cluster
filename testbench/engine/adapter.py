"""Universal dataset ingestion and schema mapping adapter for Agentic Cloud Cluster."""

from __future__ import annotations

import csv
import json
import logging
import math
import random
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple, Union

try:
    import yaml
except ImportError:
    yaml = None

from .schema import CanonicalTask, DatasetSplit

LOGGER = logging.getLogger(__name__)


def _simple_yaml_parse(text: str) -> Dict[str, Any]:
    """Lightweight fallback parser for basic YAML key-value and nested dict structures."""
    result: Dict[str, Any] = {}
    current_section = None

    for raw_line in text.splitlines():
        line = raw_line.split("#", 1)[0].rstrip()
        if not line.strip():
            continue

        indent = len(line) - len(line.lstrip())
        stripped = line.strip()

        if ":" in stripped:
            key, val = stripped.split(":", 1)
            key = key.strip()
            val = val.strip().strip("\"'")

            if not val:
                # Section header
                current_section = key
                result[current_section] = {}
            else:
                parsed_val: Any = val
                try:
                    if "." in val:
                        parsed_val = float(val)
                    else:
                        parsed_val = int(val)
                except ValueError:
                    if val.lower() == "true":
                        parsed_val = True
                    elif val.lower() == "false":
                        parsed_val = False

                if indent > 0 and current_section:
                    result[current_section][key] = parsed_val
                else:
                    result[key] = parsed_val
                    current_section = None
    return result


def load_mapping_config(mapping_input: Union[str, Path, Dict[str, Any]]) -> Dict[str, Any]:
    """Load a YAML/JSON field mapping configuration."""
    if isinstance(mapping_input, dict):
        return mapping_input

    path = Path(mapping_input)
    if not path.exists():
        raise FileNotFoundError(f"Mapping configuration not found: {path}")

    text = path.read_text(encoding="utf-8")
    if path.suffix in (".yaml", ".yml"):
        if yaml is not None:
            return yaml.safe_load(text) or {}
        return _simple_yaml_parse(text)
    return json.loads(text)


class DatasetAdapter:
    """Ingests arbitrary datasets and maps them into CanonicalTask objects."""

    def __init__(self, mapping_config: Union[str, Path, Dict[str, Any]]):
        self.config = load_mapping_config(mapping_input=mapping_config)
        self.fields = self.config.get("fields", {})
        self.transforms = self.config.get("transforms", {})
        self.defaults = self.config.get("defaults", {})

    def load_dataset(self, dataset_path: Union[str, Path], max_records: Optional[int] = None) -> List[CanonicalTask]:
        """Load and parse an external dataset (CSV, JSON, or JSONL)."""
        path = Path(dataset_path)
        if not path.exists():
            raise FileNotFoundError(f"Dataset file not found: {path}")

        records: List[Dict[str, Any]] = []
        if path.suffix == ".csv":
            records = self._read_csv(path)
        elif path.suffix in (".json",):
            records = self._read_json(path)
        elif path.suffix in (".jsonl", ".ndjson"):
            records = self._read_jsonl(path)
        else:
            # Attempt CSV then JSON fallback
            try:
                records = self._read_csv(path)
            except Exception:
                records = self._read_json(path)

        if max_records and max_records > 0:
            records = records[:max_records]

        canonical_tasks: List[CanonicalTask] = []
        for idx, row in enumerate(records):
            task = self._map_row_to_task(row, idx)
            if task is not None:
                canonical_tasks.append(task)

        LOGGER.info("Ingested %d canonical tasks from %s", len(canonical_tasks), path.name)
        return canonical_tasks

    def _read_csv(self, path: Path) -> List[Dict[str, Any]]:
        records = []
        with open(path, mode="r", encoding="utf-8-sig") as f:
            reader = csv.DictReader(f)
            for row in reader:
                records.append(dict(row))
        return records

    def _read_json(self, path: Path) -> List[Dict[str, Any]]:
        with open(path, mode="r", encoding="utf-8") as f:
            data = json.load(f)
        if isinstance(data, list):
            return data
        if isinstance(data, dict):
            # Check for common array keys
            for key in ("tasks", "records", "data", "jobs"):
                if key in data and isinstance(data[key], list):
                    return data[key]
        return [data]

    def _read_jsonl(self, path: Path) -> List[Dict[str, Any]]:
        records = []
        with open(path, mode="r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line:
                    records.append(json.loads(line))
        return records

    def _get_field_value(self, row: Dict[str, Any], canonical_name: str, fallback_default: Any = None) -> Any:
        dataset_field = self.fields.get(canonical_name)
        if dataset_field and dataset_field in row:
            val = row[dataset_field]
            if val is not None and str(val).strip() != "":
                return val

        # Fallback to direct field name matching
        if canonical_name in row and row[canonical_name] is not None:
            return row[canonical_name]

        return self.defaults.get(canonical_name, fallback_default)

    def _apply_transform(self, value: Any, field_name: str, default_val: float) -> float:
        try:
            val = float(value)
        except (ValueError, TypeError):
            val = default_val

        tf = self.transforms.get(field_name, {})
        scale = float(tf.get("scale", 1.0))
        offset = float(tf.get("offset", 0.0))
        min_val = tf.get("min_val")
        max_val = tf.get("max_val")

        transformed = (val * scale) + offset
        if min_val is not None:
            transformed = max(transformed, float(min_val))
        if max_val is not None:
            transformed = min(transformed, float(max_val))

        return float(transformed)

    def _map_row_to_task(self, row: Dict[str, Any], idx: int) -> Optional[CanonicalTask]:
        task_id = str(self._get_field_value(row, "task_id", f"task-{idx+1:05d}"))

        raw_cpu = self._get_field_value(row, "req_cpu", 1.0)
        raw_mem = self._get_field_value(row, "req_memory", 0.5)
        raw_storage = self._get_field_value(row, "req_storage", 1.0)
        raw_duration = self._get_field_value(row, "duration_seconds", 5.0)
        raw_arrival = self._get_field_value(row, "arrival_offset_sec", 0.0)
        raw_sla = self._get_field_value(row, "sla_multiplier", 2.0)

        req_cpu = self._apply_transform(raw_cpu, "req_cpu", 1.0)
        req_mem = self._apply_transform(raw_mem, "req_memory", 0.5)
        req_storage = self._apply_transform(raw_storage, "req_storage", 1.0)
        duration = self._apply_transform(raw_duration, "duration_seconds", 5.0)
        arrival = self._apply_transform(raw_arrival, "arrival_offset_sec", 0.0)
        sla_mult = self._apply_transform(raw_sla, "sla_multiplier", 2.0)

        task_type = str(self._get_field_value(row, "task_type", "")).strip().lower()
        if not task_type or task_type not in ("cpu-light", "cpu-heavy", "memory-heavy", "mixed"):
            task_type = self._infer_task_type(req_cpu, req_mem)

        docker_image = str(self._get_field_value(row, "docker_image", "agentic/workflow-deterministic:v1"))
        workflow_profile = task_type
        seed = int(self._get_field_value(row, "seed", 100 + idx))

        return CanonicalTask(
            task_id=task_id,
            req_cpu=round(req_cpu, 2),
            req_memory=round(req_mem, 2),
            req_storage=round(req_storage, 2),
            duration_seconds=round(duration, 2),
            arrival_offset_sec=round(arrival, 2),
            sla_multiplier=round(sla_mult, 2),
            task_type=task_type,
            docker_image=docker_image,
            workflow_profile=workflow_profile,
            seed=seed,
        )

    def _infer_task_type(self, cpu: float, memory: float) -> str:
        """Infer task classification based on relative CPU and memory demands."""
        if cpu >= 1.5 and memory < 1.0:
            return "cpu-heavy"
        if memory >= 1.5 and cpu < 1.0:
            return "memory-heavy"
        if cpu <= 0.6 and memory <= 0.6:
            return "cpu-light"
        return "mixed"


def split_dataset(
    tasks: List[CanonicalTask],
    train_ratio: float = 0.8,
    seed: int = 42,
    source_name: str = "",
) -> DatasetSplit:
    """Deterministically partition canonical tasks into train and test splits."""
    if not tasks:
        raise ValueError("Cannot split empty task list")

    train_ratio = max(min(train_ratio, 0.95), 0.05)
    total = len(tasks)

    # Use a deterministic copy and shuffle with fixed seed
    shuffled = list(tasks)
    rng = random.Random(seed)
    rng.shuffle(shuffled)

    split_idx = int(math.ceil(total * train_ratio))
    train_tasks = shuffled[:split_idx]
    test_tasks = shuffled[split_idx:]

    # Re-sort test tasks by arrival_offset_sec to preserve natural trace chronological flow
    test_tasks.sort(key=lambda t: t.arrival_offset_sec)

    LOGGER.info(
        "Dataset split (seed=%d, ratio=%.2f): %d train tasks, %d test tasks",
        seed,
        train_ratio,
        len(train_tasks),
        len(test_tasks),
    )

    return DatasetSplit(
        train_tasks=train_tasks,
        test_tasks=test_tasks,
        source_name=source_name,
        train_ratio=train_ratio,
        seed=seed,
        metadata={
            "total_tasks": total,
            "train_count": len(train_tasks),
            "test_count": len(test_tasks),
        },
    )


def export_split_for_training(split: DatasetSplit, output_path: Path) -> Path:
    """Export train split to JSON format ready for PPO model training."""
    output_path.parent.mkdir(parents=True, exist_ok=True)
    records = []
    for t in split.train_tasks:
        records.append({
            "task_id": t.task_id,
            "req_cpu": t.req_cpu,
            "req_memory": t.req_memory,
            "req_storage": t.req_storage,
            "duration_seconds": t.duration_seconds,
            "arrival_offset_sec": t.arrival_offset_sec,
            "sla_multiplier": t.sla_multiplier,
            "task_type": t.task_type,
        })
    output_path.write_text(json.dumps(records, indent=2), encoding="utf-8")
    LOGGER.info("Exported %d training tasks to %s", len(records), output_path)
    return output_path
