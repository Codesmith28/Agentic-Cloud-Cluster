package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
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
	loadDotEnv()

	// Port allocation scheme:
	//   50050  PPO scheduler gRPC  (colocated with master)
	//   50051  Master gRPC
	//   50052+ Worker gRPC         (auto-increment per host)
	//   8080   Master HTTP API
	//   9101+  Worker metrics
	username := getEnv("MONGODB_USERNAME", getEnv("MONGO_USER", ""))
	password := getEnv("MONGODB_PASSWORD", getEnv("MONGO_PASSWORD", ""))
	host := getEnv("MONGODB_HOST", getEnv("MONGO_HOST", "localhost:27017"))
	database := getEnv("MONGODB_DATABASE", getEnv("MONGO_DATABASE", "cluster_db"))
	port := getEnv("GRPC_PORT", ":50051")
	httpPort := getEnv("HTTP_PORT", ":8080") // Default HTTP port for telemetry API
	gaParamsPath := getEnv("SCHED_GA_PARAMS_PATH", "config/ga_output.json")
	schedulerAlgo := getEnv("SCHED_ALGO", "RTS")
	ppoAddr := getEnv("PPO_GRPC_ADDR", "127.0.0.1:50050")
	ppoRequestTimeout := getEnvInt("PPO_REQUEST_TIMEOUT_MS", 1500)
	ppoAutostart := getEnvBool("PPO_AUTOSTART", true)
	ppoModelPath := getEnv("PPO_MODEL_PATH", "latest")
	ppoDeploymentMode := normalizePPODeploymentMode(getEnv("PPO_DEPLOYMENT_MODE", "active"))
	ppoOnlineUpdates := getEnvBool("PPO_ONLINE_UPDATES_ENABLED", true)
	headless := getEnvBool("AGENTIC_HEADLESS", getEnvBool("CLOUDAI_HEADLESS", false))
	uiMode := strings.ToLower(strings.TrimSpace(getEnv("AGENTIC_UI_MODE", getEnv("CLOUDAI_UI_MODE", "cli"))))
	if uiMode != "cli" && uiMode != "tui" {
		log.Printf("⚠️  Invalid UI mode %q, using default 'cli'", uiMode)
		uiMode = "cli"
	}
	masterBindAddr := strings.TrimSpace(getEnv("MASTER_BIND_ADDR", ""))
	masterAdvAddr := strings.TrimSpace(getEnv("MASTER_ADVERTISE_ADDR", ""))

	// Load SLA multiplier with validation
	slaMultiplier := getEnvFloat("SCHED_SLA_MULTIPLIER", 2.0)
	if slaMultiplier < 1.5 || slaMultiplier > 2.5 {
		log.Printf("⚠️  Invalid SLA multiplier %.2f from env, using default 2.0", slaMultiplier)
		slaMultiplier = 2.0
	}
	if ppoRequestTimeout <= 0 {
		log.Printf("⚠️  Invalid PPO request timeout %dms from env, using default 1500ms", ppoRequestTimeout)
		ppoRequestTimeout = 1500
	}

	var mongoURI string
	if username != "" && password != "" {
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

// loadDotEnv loads environment variables from .env file
func loadDotEnv() {
	paths := []string{".env", "../.env", "../../.env"}
	for _, path := range paths {
		if err := godotenv.Load(path); err == nil {
			log.Printf("Loaded .env from %s", path)
			return
		}
	}
	log.Println("No .env file found, using environment variables")
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvFloat gets a float environment variable with a fallback value
func getEnvFloat(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
		log.Printf("⚠️  Invalid float value for %s: %s, using fallback %.2f", key, value, fallback)
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
		log.Printf("⚠️  Invalid int value for %s: %s, using fallback %d", key, value, fallback)
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
		log.Printf("⚠️  Invalid bool value for %s: %s, using fallback %t", key, value, fallback)
	}
	return fallback
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
