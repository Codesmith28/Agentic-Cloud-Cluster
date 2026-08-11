// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package system

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	workerPortEnv           = "WORKER_PORT"
	workerMetricsPortEnv    = "WORKER_METRICS_PORT"
	workerBindIPEnv         = "WORKER_BIND_IP"
	workerTotalCPUEnv       = "WORKER_TOTAL_CPU"
	workerTotalMemoryGBEnv  = "WORKER_TOTAL_MEMORY_GB"
	workerTotalStorageGBEnv = "WORKER_TOTAL_STORAGE_GB"
)

// ResolveWorkerPort resolves worker gRPC port from environment or picks the first available port.
func ResolveWorkerPort(defaultPort int) (int, error) {
	if raw := strings.TrimSpace(os.Getenv(workerPortEnv)); raw != "" {
		parsed, err := parsePort(raw)
		if err != nil {
			return 0, fmt.Errorf("invalid %s value %q: %w", workerPortEnv, raw, err)
		}
		return parsed, nil
	}

	return FindAvailablePort(defaultPort)
}

// ResolveWorkerMetricsPort resolves the worker metrics HTTP port from environment or defaults to a fixed port.
func ResolveWorkerMetricsPort(defaultPort int) (int, error) {
	if raw := strings.TrimSpace(os.Getenv(workerMetricsPortEnv)); raw != "" {
		parsed, err := parsePort(raw)
		if err != nil {
			return 0, fmt.Errorf("invalid %s value %q: %w", workerMetricsPortEnv, raw, err)
		}
		return parsed, nil
	}
	return defaultPort, nil
}

// ResolveWorkerBindIP resolves worker bind IP from environment or uses the detected default.
func ResolveWorkerBindIP(detected string) string {
	if override := strings.TrimSpace(os.Getenv(workerBindIPEnv)); override != "" {
		return override
	}
	return detected
}

func parsePort(raw string) (int, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), ":")
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("port must be in range 1-65535")
	}
	return port, nil
}

func applyResourceOverrides(resources *ResourceInfo) {
	if resources == nil {
		return
	}

	if value, ok, err := parseOptionalFloat(workerTotalCPUEnv); err != nil {
		log.Printf("⚠️  Ignoring %s: %v", workerTotalCPUEnv, err)
	} else if ok {
		if value <= 0 {
			log.Printf("⚠️  Ignoring %s: value must be > 0", workerTotalCPUEnv)
		} else {
			resources.TotalCPU = value
			log.Printf("✓ Worker CPU override applied: %.2f cores", value)
		}
	}

	if value, ok, err := parseOptionalFloat(workerTotalMemoryGBEnv); err != nil {
		log.Printf("⚠️  Ignoring %s: %v", workerTotalMemoryGBEnv, err)
	} else if ok {
		if value <= 0 {
			log.Printf("⚠️  Ignoring %s: value must be > 0", workerTotalMemoryGBEnv)
		} else {
			resources.TotalMemory = value
			log.Printf("✓ Worker memory override applied: %.2f GB", value)
		}
	}

	if value, ok, err := parseOptionalFloat(workerTotalStorageGBEnv); err != nil {
		log.Printf("⚠️  Ignoring %s: %v", workerTotalStorageGBEnv, err)
	} else if ok {
		if value <= 0 {
			log.Printf("⚠️  Ignoring %s: value must be > 0", workerTotalStorageGBEnv)
		} else {
			resources.TotalStorage = value
			log.Printf("✓ Worker storage override applied: %.2f GB", value)
		}
	}
}

func parseOptionalFloat(key string) (float64, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, true, err
	}
	return value, true, nil
}
