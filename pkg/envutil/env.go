package envutil

import (
	"log"
	"os"
	"strconv"
)

// GetEnv retrieves an environment variable or returns fallback.
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GetEnvFloat parses a float64 environment variable or returns fallback.
func GetEnvFloat(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
		log.Printf("⚠️  Invalid float value for %s: %s, using fallback %.2f", key, value, fallback)
	}
	return fallback
}

// GetEnvInt parses an int environment variable or returns fallback.
func GetEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
		log.Printf("⚠️  Invalid int value for %s: %s, using fallback %d", key, value, fallback)
	}
	return fallback
}

// GetEnvBool parses a bool environment variable or returns fallback.
func GetEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
		log.Printf("⚠️  Invalid bool value for %s: %s, using fallback %t", key, value, fallback)
	}
	return fallback
}
