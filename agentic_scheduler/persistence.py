from __future__ import annotations

import hashlib
import logging
from datetime import datetime, timezone
from typing import Dict, Optional, Tuple

from bson import ObjectId
from pymongo import ASCENDING, DESCENDING, MongoClient
from pymongo.collection import Collection
from pymongo.database import Database
from pymongo.errors import PyMongoError
from gridfs import GridFSBucket


LOGGER = logging.getLogger(__name__)


def _sanitize_mongo_value(value: str) -> str:
    """Ensure a value used in MongoDB queries is a plain string, not operator injection."""
    if not isinstance(value, str):
        value = str(value)
    return value.strip()[:512]


class MongoSchedulerModelStore:
    """Stores scheduler checkpoints in GridFS with versioned metadata."""

    def __init__(self, mongo_uri: str, mongo_db: str):
        self.client: Optional[MongoClient] = None
        self.db: Optional[Database] = None
        self.collection: Optional[Collection] = None
        self.bucket: Optional[GridFSBucket] = None

        if not mongo_uri or not mongo_db:
            LOGGER.warning("Mongo configuration missing; model persistence disabled")
            return

        try:
            self.client = MongoClient(mongo_uri, serverSelectionTimeoutMS=2500)
            self.client.admin.command("ping")
            self.db = self.client[mongo_db]
            self.collection = self.db["SCHEDULER_MODELS"]
            self.bucket = GridFSBucket(self.db, bucket_name="scheduler_models")
            self._ensure_indexes()
            LOGGER.info("Mongo scheduler model persistence initialized")
        except Exception as exc:  # pylint: disable=broad-except
            LOGGER.warning("Failed to initialize Mongo scheduler persistence: %s", exc)
            self.client = None
            self.db = None
            self.collection = None
            self.bucket = None

    def is_available(self) -> bool:
        return self.collection is not None and self.bucket is not None

    def close(self) -> None:
        if self.client:
            self.client.close()

    def _ensure_indexes(self) -> None:
        if self.collection is None:
            return
        self.collection.create_index(
            [("scheduler_type", ASCENDING), ("fingerprint_hash", ASCENDING), ("version", DESCENDING)],
            name="sched_model_lookup_idx",
        )
        self.collection.create_index(
            [("scheduler_type", ASCENDING), ("fingerprint_hash", ASCENDING), ("active", ASCENDING)],
            unique=True,
            partialFilterExpression={"active": True},
            name="sched_model_one_active_idx",
        )
        self.collection.create_index(
            [("scheduler_type", ASCENDING), ("fingerprint_hash", ASCENDING), ("created_at", DESCENDING)],
            name="sched_model_created_idx",
        )

    def load_active_checkpoint(self, scheduler_type: str, fingerprint_hash: str) -> Optional[Tuple[Dict, bytes]]:
        if not self.is_available():
            return None
        assert self.collection is not None
        assert self.bucket is not None

        scheduler_type = _sanitize_mongo_value(scheduler_type)
        fingerprint_hash = _sanitize_mongo_value(fingerprint_hash)

        try:
            doc = self.collection.find_one(
                {
                    "scheduler_type": scheduler_type,
                    "fingerprint_hash": fingerprint_hash,
                    "active": True,
                },
                sort=[("version", DESCENDING)],
            )
            if not doc:
                return None

            file_id = doc.get("file_id")
            if not file_id:
                return None

            stream = self.bucket.open_download_stream(file_id)
            payload = stream.read()
            stream.close()

            return doc, payload
        except PyMongoError as exc:
            LOGGER.warning("Failed loading active checkpoint for %s/%s: %s", scheduler_type, fingerprint_hash, exc)
            return None

    def save_and_activate_checkpoint(
        self,
        scheduler_type: str,
        fingerprint_hash: str,
        fingerprint_payload: str,
        checkpoint_bytes: bytes,
        framework: str,
        extra_metadata: Optional[Dict] = None,
    ) -> Optional[Dict]:
        if not self.is_available():
            return None
        assert self.collection is not None
        assert self.bucket is not None

        scheduler_type = _sanitize_mongo_value(scheduler_type)
        fingerprint_hash = _sanitize_mongo_value(fingerprint_hash)
        fingerprint_payload = str(fingerprint_payload or "")[:4096]

        now = datetime.now(timezone.utc)
        sha256 = hashlib.sha256(checkpoint_bytes).hexdigest()

        try:
            latest = self.collection.find_one(
                {"scheduler_type": scheduler_type, "fingerprint_hash": fingerprint_hash},
                sort=[("version", DESCENDING)],
            )
            next_version = int(latest.get("version", 0)) + 1 if latest else 1

            filename = f"{scheduler_type.lower()}_{fingerprint_hash}_v{next_version}.ckpt"
            file_id = self.bucket.upload_from_stream(filename, checkpoint_bytes)

            doc = {
                "scheduler_type": scheduler_type,
                "fingerprint_hash": fingerprint_hash,
                "fingerprint_payload": fingerprint_payload,
                "version": next_version,
                "active": False,
                "file_id": file_id,
                "file_size": len(checkpoint_bytes),
                "file_sha256": sha256,
                "framework": framework,
                "metadata": extra_metadata or {},
                "created_at": now,
                "updated_at": now,
                "activated_at": None,
            }
            inserted = self.collection.insert_one(doc)
            doc_id = inserted.inserted_id

            self.collection.update_many(
                {
                    "scheduler_type": scheduler_type,
                    "fingerprint_hash": fingerprint_hash,
                    "active": True,
                    "_id": {"$ne": doc_id},
                },
                {"$set": {"active": False, "updated_at": now}},
            )
            self.collection.update_one(
                {"_id": doc_id},
                {
                    "$set": {
                        "active": True,
                        "updated_at": now,
                        "activated_at": now,
                    }
                },
            )

            saved = self.collection.find_one({"_id": doc_id}) or {}
            return saved
        except PyMongoError as exc:
            LOGGER.warning("Failed to save checkpoint for %s/%s: %s", scheduler_type, fingerprint_hash, exc)
            return None

    def activate_existing_version(
        self,
        scheduler_type: str,
        fingerprint_hash: str,
        version: int,
    ) -> bool:
        if not self.is_available():
            return False
        assert self.collection is not None
        now = datetime.now(timezone.utc)

        scheduler_type = _sanitize_mongo_value(scheduler_type)
        fingerprint_hash = _sanitize_mongo_value(fingerprint_hash)

        try:
            target = self.collection.find_one(
                {
                    "scheduler_type": scheduler_type,
                    "fingerprint_hash": fingerprint_hash,
                    "version": int(version),
                }
            )
            if not target:
                return False
            target_id = target["_id"]
            self.collection.update_many(
                {"scheduler_type": scheduler_type, "fingerprint_hash": fingerprint_hash, "active": True},
                {"$set": {"active": False, "updated_at": now}},
            )
            result = self.collection.update_one(
                {"_id": target_id},
                {"$set": {"active": True, "updated_at": now, "activated_at": now}},
            )
            return result.modified_count > 0
        except PyMongoError as exc:
            LOGGER.warning("Failed to activate existing checkpoint: %s", exc)
            return False


def coerce_object_id(value) -> Optional[ObjectId]:
    if isinstance(value, ObjectId):
        return value
    try:
        return ObjectId(str(value))
    except Exception:  # pylint: disable=broad-except
        return None

