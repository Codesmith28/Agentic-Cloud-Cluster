"""Constants and environment variable definitions for the Agentic PPO Scheduler."""

# Feature Dimensions
TASK_FEATURE_DIM = 5
WORKER_FEATURE_DIM = 9

# Task Types
TASK_TYPE_TO_ID = {
    "cpu-light": 0,
    "cpu-heavy": 1,
    "memory-heavy": 2,
    "mixed": 3,
}

# Network & gRPC Defaults
DEFAULT_GRPC_PORT = 50050
DEFAULT_GRPC_ADDR = "127.0.0.1:50050"

# Model & Persistence Defaults
DEFAULT_MODEL_DIR = "agentic_scheduler/models"
DEFAULT_LATEST_MODEL = "ppo_latest.pt"
DEFAULT_DEPLOYMENT_MODE = "active"  # "shadow", "active", "fallback"
MASK_VALUE = -1e4

# MongoDB Collections
COLLECTION_SCHEDULER_MODELS = "SCHEDULER_MODELS"
DEFAULT_MONGO_DB = "cluster_db"
DEFAULT_MONGO_HOST = "localhost:27017"

# Environment Variable Keys
ENV_PPO_GRPC_ADDR = "PPO_GRPC_ADDR"
ENV_PPO_MODEL_PATH = "PPO_MODEL_PATH"
ENV_PPO_DEPLOYMENT_MODE = "PPO_DEPLOYMENT_MODE"
ENV_PPO_ONLINE_UPDATES = "PPO_ONLINE_UPDATES_ENABLED"
ENV_PPO_AUTOSTART = "PPO_AUTOSTART"
ENV_MONGODB_URI = "MONGODB_URI"
ENV_MONGODB_DATABASE = "MONGODB_DATABASE"
ENV_MONGODB_HOST = "MONGODB_HOST"
ENV_MONGODB_USERNAME = "MONGODB_USERNAME"
ENV_MONGODB_PASSWORD = "MONGODB_PASSWORD"
