package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/constants"
	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/envutil"
)

// Config holds all configuration for the worker node.
type Config struct {
	WorkerID           string
	WorkerIP           string
	DefaultPort        int
	DefaultMetricsPort int
	OutputBaseDir      string
	StateDir           string
}

// LoadConfig initializes worker configuration from environment variables and defaults.
func LoadConfig() *Config {
	envutil.LoadDotEnv()

	outputDir := envutil.GetEnv(constants.EnvOutputDir, envutil.GetEnv(constants.EnvLegacyOutputDir, constants.DefaultOutputDir))
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, ".agentic-cloud-cluster", "outputs")
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			log.Fatalf("Failed to create output directory %s: %v", outputDir, err)
		}
		log.Printf("⚠️  Using user directory (no root access): %s", outputDir)
	} else {
		log.Printf("✓ Output directory ready (secure): %s", outputDir)
	}

	stateDir := envutil.GetEnv(constants.EnvWorkerStateDir, envutil.GetEnv(constants.EnvLegacyWorkerStateDir, constants.DefaultWorkerStateDir))

	return &Config{
		WorkerID:           envutil.GetEnv(constants.EnvWorkerID, ""),
		WorkerIP:           envutil.GetEnv(constants.EnvWorkerIP, ""),
		DefaultPort:        50052,
		DefaultMetricsPort: 9101,
		OutputBaseDir:      outputDir,
		StateDir:           stateDir,
	}
}
