package config

import (
	"log"
	"strings"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/constants"
	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/envutil"
)

// Config holds all configuration for the master node
type Config struct {
	MongoDBUsername string
	MongoDBPassword string
	GRPCPort        string
	MongoDBURI      string
	MongoDBDatabase string
	HTTPPort        string // HTTP port for telemetry API
	Headless        bool   // Run without interactive CLI (for containerized/testbench mode)
	UIMode          string // "cli" (default) or "tui" - resolved from --mode flag or AGENTIC_UI_MODE env
	MasterBindAddr  string // Optional gRPC bind address override
	MasterAdvAddr   string // Optional gRPC advertise address override

	// Scheduler selection/configuration
	SchedulerAlgo       string // RR / RTS / PPO
	SLAMultiplier       float64
	GAParamsPath        string
	PPOGRPCAddr         string
	PPORequestTimeoutMS int
	PPOAutostart        bool
	PPOModelPath        string
	PPODeploymentMode   string // shadow / active / fallback
	PPOOnlineUpdates    bool
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() *Config {
	envutil.LoadDotEnv()

	// Port allocation scheme:
	//   50050  PPO scheduler gRPC  (colocated with master)
	//   50051  Master gRPC
	//   50052+ Worker gRPC         (auto-increment per host)
	//   8080   Master HTTP API
	//   9101+  Worker metrics
	username := envutil.GetEnv(constants.EnvMongoDBUsername, envutil.GetEnv(constants.EnvLegacyMongoUser, ""))
	password := envutil.GetEnv(constants.EnvMongoDBPassword, envutil.GetEnv(constants.EnvLegacyMongoPass, ""))
	host := envutil.GetEnv(constants.EnvMongoDBHost, envutil.GetEnv(constants.EnvLegacyMongoHost, constants.DefaultMongoHost))
	database := envutil.GetEnv(constants.EnvMongoDBDatabase, envutil.GetEnv(constants.EnvLegacyMongoDB, constants.DefaultMongoDatabase))
	port := envutil.GetEnv(constants.EnvGRPCPort, constants.DefaultGRPCPort)
	httpPort := envutil.GetEnv(constants.EnvHTTPPort, constants.DefaultHTTPPort)
	gaParamsPath := envutil.GetEnv(constants.EnvGAParamsPath, constants.DefaultGAParamsPath)
	schedulerAlgo := envutil.GetEnv(constants.EnvSchedulerAlgo, constants.DefaultSchedulerAlgo)
	ppoAddr := envutil.GetEnv(constants.EnvPPOGRPCAddr, constants.DefaultPPOGRPCAddr)
	ppoRequestTimeout := envutil.GetEnvInt(constants.EnvPPORequestTimeoutMS, constants.DefaultPPORequestTimeoutMS)
	ppoAutostart := envutil.GetEnvBool(constants.EnvPPOAutostart, true)
	ppoModelPath := envutil.GetEnv(constants.EnvPPOModelPath, constants.DefaultPPOModelPath)
	ppoDeploymentMode := normalizePPODeploymentMode(envutil.GetEnv(constants.EnvPPODeploymentMode, constants.DefaultPPODeploymentMode))
	ppoOnlineUpdates := envutil.GetEnvBool(constants.EnvPPOOnlineUpdates, true)
	headless := envutil.GetEnvBool(constants.EnvAgenticHeadless, envutil.GetEnvBool(constants.EnvLegacyHeadless, false))
	uiMode := strings.ToLower(strings.TrimSpace(envutil.GetEnv(constants.EnvAgenticUIMode, envutil.GetEnv(constants.EnvLegacyUIMode, constants.DefaultUIMode))))
	if uiMode != "cli" && uiMode != "tui" {
		log.Printf("⚠️  Invalid UI mode %q, using default 'cli'", uiMode)
		uiMode = constants.DefaultUIMode
	}
	masterBindAddr := strings.TrimSpace(envutil.GetEnv(constants.EnvMasterBindAddr, ""))
	masterAdvAddr := strings.TrimSpace(envutil.GetEnv(constants.EnvMasterAdvAddr, ""))

	// Load SLA multiplier with validation
	slaMultiplier := envutil.GetEnvFloat(constants.EnvSLAMultiplier, constants.DefaultSLAMultiplier)
	if slaMultiplier < constants.DefaultMinSLAMultiplier || slaMultiplier > constants.DefaultMaxSLAMultiplier {
		log.Printf("⚠️  Invalid SLA multiplier %.2f from env, using default %.1f", slaMultiplier, constants.DefaultSLAMultiplier)
		slaMultiplier = constants.DefaultSLAMultiplier
	}
	if ppoRequestTimeout <= 0 {
		log.Printf("⚠️  Invalid PPO request timeout %dms from env, using default %dms", ppoRequestTimeout, constants.DefaultPPORequestTimeoutMS)
		ppoRequestTimeout = constants.DefaultPPORequestTimeoutMS
	}

	var mongoURI string
	if explicitURI := envutil.GetEnv(constants.EnvMongoDBURI, ""); explicitURI != "" {
		mongoURI = explicitURI
	} else if username != "" && password != "" {
		mongoURI = "mongodb://" + username + ":" + password + "@" + host
	} else {
		mongoURI = "mongodb://" + host
	}

	config := &Config{
		MongoDBUsername:     username,
		MongoDBPassword:     password,
		GRPCPort:            port,
		MongoDBURI:          mongoURI,
		MongoDBDatabase:     database,
		HTTPPort:            httpPort,
		Headless:            headless,
		UIMode:              uiMode,
		MasterBindAddr:      masterBindAddr,
		MasterAdvAddr:       masterAdvAddr,
		SchedulerAlgo:       schedulerAlgo,
		SLAMultiplier:       slaMultiplier,
		GAParamsPath:        gaParamsPath,
		PPOGRPCAddr:         ppoAddr,
		PPORequestTimeoutMS: ppoRequestTimeout,
		PPOAutostart:        ppoAutostart,
		PPOModelPath:        ppoModelPath,
		PPODeploymentMode:   ppoDeploymentMode,
		PPOOnlineUpdates:    ppoOnlineUpdates,
	}

	return config
}

// ResolveUIMode determines the final UI mode given CLI flag override.
// Precedence: Headless=true (highest) > explicit flag > env var > default "cli"
func (c *Config) ResolveUIMode(flagMode string) string {
	if c.Headless {
		return "headless"
	}
	if flagMode != "" {
		mode := strings.ToLower(strings.TrimSpace(flagMode))
		if mode == "cli" || mode == "tui" {
			return mode
		}
		log.Printf("⚠️  Invalid --mode value %q, falling back to config", flagMode)
	}
	return c.UIMode
}

func normalizePPODeploymentMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "shadow":
		return "shadow"
	case "fallback":
		return "fallback"
	case "", "active":
		return "active"
	default:
		log.Printf("⚠️  Invalid PPO deployment mode %q, using default active", value)
		return "active"
	}
}
