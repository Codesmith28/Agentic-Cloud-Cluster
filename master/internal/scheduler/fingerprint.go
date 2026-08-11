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

package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

const clusterFingerprintSchemaVersion = "cluster-fingerprint/v1"

type fingerprintWorker struct {
	WorkerID     string  `json:"worker_id"`
	TotalCPU     float64 `json:"total_cpu"`
	TotalMemory  float64 `json:"total_memory"`
	TotalStorage float64 `json:"total_storage"`
}

type fingerprintPayload struct {
	SchemaVersion string              `json:"schema_version"`
	Workers       []fingerprintWorker `json:"workers"`
}

// BuildClusterFingerprint returns a strict hash + canonical payload for the
// current worker set (IDs + capacities + schema version).
func BuildClusterFingerprint(workers map[string]*WorkerInfo) (string, string) {
	if len(workers) == 0 {
		payload := fingerprintPayload{
			SchemaVersion: clusterFingerprintSchemaVersion,
			Workers:       []fingerprintWorker{},
		}
		raw, _ := json.Marshal(payload)
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:]), string(raw)
	}

	ids := make([]string, 0, len(workers))
	for id := range workers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	entries := make([]fingerprintWorker, 0, len(ids))
	for _, id := range ids {
		w := workers[id]
		if w == nil {
			continue
		}
		entries = append(entries, fingerprintWorker{
			WorkerID:     id,
			TotalCPU:     w.TotalCPU,
			TotalMemory:  w.TotalMemory,
			TotalStorage: w.TotalStorage,
		})
	}

	payload := fingerprintPayload{
		SchemaVersion: clusterFingerprintSchemaVersion,
		Workers:       entries,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), string(raw)
}
