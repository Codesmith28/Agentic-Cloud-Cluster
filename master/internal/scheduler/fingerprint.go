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
	TotalGPU     float64 `json:"total_gpu"`
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
			TotalGPU:     w.TotalGPU,
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
