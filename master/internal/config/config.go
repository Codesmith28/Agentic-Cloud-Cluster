package config

import (
	"log"
	"os"

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

	var mongoURI string
	if username != "" && password != "" {
		mongoURI = "mongodb://" + username + ":" + password + "@" + host
	} else {
		mongoURI = "mongodb://" + host
	}

	config := &Config{
		MongoDBUsername: username,
		MongoDBPassword: password,
		GRPCPort:        port,
		MongoDBURI:      mongoURI,
		MongoDBDatabase: database,
		HTTPPort:        httpPort,
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
