package constants

// Environment variable keys used across Agentic Cloud Cluster.
const (
	// Master Networking & HTTP
	EnvGRPCPort        = "GRPC_PORT"
	EnvHTTPPort        = "HTTP_PORT"
	EnvMasterBindAddr  = "MASTER_BIND_ADDR"
	EnvMasterAdvAddr   = "MASTER_ADVERTISE_ADDR"
	EnvMasterURL       = "MASTER_URL"
	EnvAgenticHeadless = "AGENTIC_HEADLESS"
	EnvLegacyHeadless  = "CLOUDAI_HEADLESS"
	EnvAgenticUIMode   = "AGENTIC_UI_MODE"
	EnvLegacyUIMode    = "CLOUDAI_UI_MODE"
	EnvWebUIPort       = "WEBUI_PORT"

	// MongoDB Configuration
	EnvMongoDBURI      = "MONGODB_URI"
	EnvMongoDBHost     = "MONGODB_HOST"
	EnvMongoDBPort     = "MONGODB_PORT"
	EnvMongoDBUsername = "MONGODB_USERNAME"
	EnvMongoDBPassword = "MONGODB_PASSWORD"
	EnvMongoDBDatabase = "MONGODB_DATABASE"
	EnvLegacyMongoUser = "MONGO_USER"
	EnvLegacyMongoPass = "MONGO_PASSWORD"
	EnvLegacyMongoHost = "MONGO_HOST"
	EnvLegacyMongoDB   = "MONGO_DATABASE"

	// Scheduler Configuration
	EnvSchedulerAlgo       = "SCHED_ALGO"
	EnvSLAMultiplier       = "SCHED_SLA_MULTIPLIER"
	EnvGAParamsPath        = "SCHED_GA_PARAMS_PATH"
	EnvPPOGRPCAddr         = "PPO_GRPC_ADDR"
	EnvPPORequestTimeoutMS = "PPO_REQUEST_TIMEOUT_MS"
	EnvPPOAutostart        = "PPO_AUTOSTART"
	EnvPPOModelPath        = "PPO_MODEL_PATH"
	EnvPPODeploymentMode   = "PPO_DEPLOYMENT_MODE"
	EnvPPOOnlineUpdates    = "PPO_ONLINE_UPDATES_ENABLED"

	// Worker Configuration
	EnvWorkerID             = "WORKER_ID"
	EnvWorkerIP             = "WORKER_IP"
	EnvWorkerPort           = "WORKER_PORT"
	EnvWorkerStateDir       = "AGENTIC_WORKER_STATE_DIR"
	EnvLegacyWorkerStateDir = "WORKER_STATE_DIR"
	EnvOutputDir            = "AGENTIC_OUTPUT_DIR"
	EnvLegacyOutputDir      = "CLOUDAI_OUTPUT_DIR"
	EnvHeartbeatIntervalSec = "HEARTBEAT_INTERVAL_SEC"
	EnvWorkerTimeoutSec     = "WORKER_TIMEOUT_SEC"

	// Storage & Security
	EnvFilesDir       = "AGENTIC_FILES_DIR"
	EnvLegacyFilesDir = "CLOUDAI_FILES_DIR"
	EnvJWTSecret      = "JWT_SECRET"
	EnvAdminUser      = "ADMIN_USER"
	EnvAdminPassword  = "ADMIN_PASSWORD"
)
