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

package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"master/internal/scheduler"
	"master/internal/server"
)

// SchedulerRegistry holds named schedulers that can be switched at runtime.
type SchedulerRegistry struct {
	mu         sync.RWMutex
	schedulers map[string]scheduler.Scheduler // canonical key → scheduler
}

// NewSchedulerRegistry creates a registry from the provided schedulers.
// Only non-nil schedulers are registered. Keys are upper-cased.
func NewSchedulerRegistry(entries map[string]scheduler.Scheduler) *SchedulerRegistry {
	r := &SchedulerRegistry{schedulers: make(map[string]scheduler.Scheduler, len(entries))}
	for k, v := range entries {
		if v == nil {
			continue
		}
		r.schedulers[strings.ToUpper(k)] = v
	}
	return r
}

// Get returns the scheduler for the given algo code (case-insensitive).
func (r *SchedulerRegistry) Get(algo string) (scheduler.Scheduler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.schedulers[strings.ToUpper(algo)]
	return s, ok
}

// Available returns the list of registered algorithm codes.
func (r *SchedulerRegistry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.schedulers))
	for k := range r.schedulers {
		out = append(out, k)
	}
	return out
}

// SchedulerSwitchHandler handles POST/GET /api/config/scheduler.
type SchedulerSwitchHandler struct {
	masterServer *server.MasterServer
	registry     *SchedulerRegistry
}

// NewSchedulerSwitchHandler creates a handler that can swap the active scheduler.
func NewSchedulerSwitchHandler(ms *server.MasterServer, registry *SchedulerRegistry) *SchedulerSwitchHandler {
	return &SchedulerSwitchHandler{
		masterServer: ms,
		registry:     registry,
	}
}

// HandleScheduler dispatches to GET or POST.
func (h *SchedulerSwitchHandler) HandleScheduler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SchedulerSwitchHandler) handleGet(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"algorithm": h.masterServer.GetSchedulerName(),
		"available": h.registry.Available(),
	})
}

func (h *SchedulerSwitchHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Algorithm string `json:"algorithm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	algo := strings.TrimSpace(body.Algorithm)
	if algo == "" {
		writeJSONError(w, http.StatusBadRequest, "algorithm is required")
		return
	}

	sched, ok := h.registry.Get(algo)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":   false,
			"error":     "unknown algorithm: " + algo,
			"available": h.registry.Available(),
		})
		return
	}

	sched.Reset()
	h.masterServer.SetScheduler(sched)
	log.Printf("Scheduler dynamically switched to %s (via API)", sched.GetName())

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":   true,
		"algorithm": sched.GetName(),
	})
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   msg,
	})
}
