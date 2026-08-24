package constants

import "time"

// Default system configurations and timeouts.
const (
	// Network Defaults
	DefaultGRPCPort    = ":50051"
	DefaultHTTPPort    = ":8080"
	DefaultPPOGRPCAddr = "127.0.0.1:50050"
	DefaultWorkerPort  = "50052"
	DefaultWebUIPort   = 3001

	// Database Defaults
	DefaultMongoHost     = "localhost:27017"
	DefaultMongoDatabase = "cluster_db"
	DefaultMinPoolSize   = 5
	DefaultMaxPoolSize   = 50
	DefaultDBTimeout     = 10 * time.Second

	// Scheduler Defaults
	DefaultSchedulerAlgo       = "RTS"
	DefaultSLAMultiplier       = 2.0
	DefaultMinSLAMultiplier    = 1.5
	DefaultMaxSLAMultiplier    = 2.5
	DefaultGAParamsPath        = "config/ga_output.json"
	DefaultPPORequestTimeoutMS = 1500
	DefaultPPOModelPath        = "latest"
	DefaultPPODeploymentMode   = "active"

	// Worker & Heartbeat Defaults
	DefaultHeartbeatInterval = 5 * time.Second
	DefaultWorkerTimeout     = 15 * time.Second
	DefaultWorkerStateDir    = "/var/lib/agentic/worker"
	DefaultOutputDir         = "/var/agentic-cloud-cluster/outputs"
	DefaultFilesDir          = "/var/agentic-cloud-cluster/files"
	DefaultContainerPIDLimit = 512

	// Master Lifecycle Defaults
	DefaultQueueDrainInterval = 1 * time.Second
	DefaultMaxTaskRetries     = 3
	DefaultUIMode             = "cli"
)
