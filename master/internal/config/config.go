package config

import (
	"log"
	"os"
	"strconv"

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

	// Scheduler selection/configuration
	SchedulerAlgo       string // RR / RTS / PPO
	SLAMultiplier       float64
	GAParamsPath        string
	PPOGRPCAddr         string
	PPORequestTimeoutMS int
	PPOAutostart        bool
	PPOModelPath        string
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() *Config {
	loadDotEnv()

	username := getEnv("MONGODB_USERNAME", "")
	password := getEnv("MONGODB_PASSWORD", "")
	host := getEnv("MONGODB_HOST", "localhost:27017")
	database := getEnv("MONGODB_DATABASE", "cluster_db")
	port := getEnv("GRPC_PORT", ":50051")
	httpPort := getEnv("HTTP_PORT", ":8080") // Default HTTP port for telemetry API
	gaParamsPath := getEnv("SCHED_GA_PARAMS_PATH", "config/ga_output.json")
	schedulerAlgo := getEnv("SCHED_ALGO", "RTS")
	ppoAddr := getEnv("PPO_GRPC_ADDR", "127.0.0.1:50061")
	ppoRequestTimeout := getEnvInt("PPO_REQUEST_TIMEOUT_MS", 1500)
	ppoAutostart := getEnvBool("PPO_AUTOSTART", true)
	ppoModelPath := getEnv("PPO_MODEL_PATH", "agentic_scheduler/models/ppo_latest.pt")

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
		SchedulerAlgo:       schedulerAlgo,
		SLAMultiplier:       slaMultiplier,
		GAParamsPath:        gaParamsPath,
		PPOGRPCAddr:         ppoAddr,
		PPORequestTimeoutMS: ppoRequestTimeout,
		PPOAutostart:        ppoAutostart,
		PPOModelPath:        ppoModelPath,
	}

	return config
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
