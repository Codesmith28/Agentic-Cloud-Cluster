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

package server

import (
	"context"
	"testing"

	pb "master/proto"
)

func TestManualRegisterWorkerUpdatesAddressForExistingWorker(t *testing.T) {
	s := NewMasterServer(nil, nil, nil, nil, nil, nil, nil)
	s.workers["worker-1"] = &WorkerState{
		Info:         &pb.WorkerInfo{WorkerId: "worker-1", WorkerIp: "10.0.0.1:50052"},
		IsActive:     true,
		RunningTasks: make(map[string]bool),
	}

	if err := s.ManualRegisterWorker(context.Background(), "worker-1", "10.0.0.2:50077"); err != nil {
		t.Fatalf("ManualRegisterWorker returned error: %v", err)
	}

	worker := s.workers["worker-1"]
	if worker.Info.WorkerIp != "10.0.0.2:50077" {
		t.Fatalf("expected updated worker address, got %s", worker.Info.WorkerIp)
	}
	if worker.IsActive {
		t.Fatalf("expected worker to be marked inactive until re-registration")
	}
}

func TestRegisterWorkerKeepsConfiguredAddress(t *testing.T) {
	s := NewMasterServer(nil, nil, nil, nil, nil, nil, nil)
	s.workers["worker-1"] = &WorkerState{
		Info:         &pb.WorkerInfo{WorkerId: "worker-1", WorkerIp: "10.0.0.1:50052"},
		IsActive:     false,
		RunningTasks: make(map[string]bool),
	}

	ack, err := s.RegisterWorker(context.Background(), &pb.WorkerInfo{
		WorkerId:     "worker-1",
		WorkerIp:     "172.16.0.9:61234",
		TotalCpu:     4,
		TotalMemory:  16,
		TotalStorage: 100,
	})
	if err != nil {
		t.Fatalf("RegisterWorker returned error: %v", err)
	}
	if ack == nil || !ack.Success {
		t.Fatalf("expected successful registration ack, got %+v", ack)
	}

	worker := s.workers["worker-1"]
	if worker.Info.WorkerIp != "10.0.0.1:50052" {
		t.Fatalf("expected configured address to be preserved, got %s", worker.Info.WorkerIp)
	}
}
