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
	"context"
	"errors"
	"testing"
	"time"

	pb "master/proto"

	"google.golang.org/grpc"
)

type staticFallbackScheduler struct {
	name   string
	worker string
	calls  int
}

func (s *staticFallbackScheduler) SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string {
	s.calls++
	return s.worker
}

func (s *staticFallbackScheduler) GetName() string {
	if s.name != "" {
		return s.name
	}
	return "fallback"
}

func (s *staticFallbackScheduler) Reset() {}

type fakePPOClient struct {
	pingResp  *pb.PingResponse
	pingErr   error
	pingCalls int

	loadResp  *pb.LoadModelForFingerprintResponse
	loadErr   error
	loadCalls int

	selectResp  *pb.SelectWorkerResponse
	selectErr   error
	selectCalls int

	reportResp  *pb.ReportOutcomeResponse
	reportErr   error
	reportCalls int
}

func (f *fakePPOClient) Ping(ctx context.Context, in *pb.PingRequest, opts ...grpc.CallOption) (*pb.PingResponse, error) {
	f.pingCalls++
	return f.pingResp, f.pingErr
}

func (f *fakePPOClient) LoadModelForFingerprint(ctx context.Context, in *pb.LoadModelForFingerprintRequest, opts ...grpc.CallOption) (*pb.LoadModelForFingerprintResponse, error) {
	f.loadCalls++
	if f.loadResp != nil || f.loadErr != nil {
		return f.loadResp, f.loadErr
	}
	return &pb.LoadModelForFingerprintResponse{Loaded: true, ModelVersion: "v1"}, nil
}

func (f *fakePPOClient) SelectWorker(ctx context.Context, in *pb.SelectWorkerRequest, opts ...grpc.CallOption) (*pb.SelectWorkerResponse, error) {
	f.selectCalls++
	if f.selectResp != nil || f.selectErr != nil {
		return f.selectResp, f.selectErr
	}
	return &pb.SelectWorkerResponse{WorkerId: "w-ppo", ModelVersion: "v1"}, nil
}

func (f *fakePPOClient) ReportOutcome(ctx context.Context, in *pb.ReportOutcomeRequest, opts ...grpc.CallOption) (*pb.ReportOutcomeResponse, error) {
	f.reportCalls++
	if f.reportResp != nil || f.reportErr != nil {
		return f.reportResp, f.reportErr
	}
	return &pb.ReportOutcomeResponse{Accepted: true}, nil
}

func TestNewPPOSchedulerFallbackModeDoesNotRequireGRPC(t *testing.T) {
	fallback := &staticFallbackScheduler{name: "RTS", worker: "w-rts"}
	scheduler, err := NewPPOScheduler("", time.Second, fallback, "", PPOModeFallback, false)
	if err != nil {
		t.Fatalf("expected fallback mode constructor to succeed without gRPC, got error: %v", err)
	}
	if scheduler.client != nil {
		t.Fatalf("expected no PPO client in fallback mode")
	}
	if scheduler.DeploymentMode() != PPOModeFallback {
		t.Fatalf("expected fallback deployment mode, got %s", scheduler.DeploymentMode())
	}
}

func TestPPOSchedulerSelectWorkerActiveUsesPPODecision(t *testing.T) {
	fallback := &staticFallbackScheduler{name: "RTS", worker: "w-fallback"}
	client := &fakePPOClient{
		loadResp:   &pb.LoadModelForFingerprintResponse{Loaded: true, ModelVersion: "v7"},
		selectResp: &pb.SelectWorkerResponse{WorkerId: "w-ppo", ModelVersion: "v7"},
	}
	s := &PPOScheduler{
		client:         client,
		fallback:       fallback,
		requestTimeout: time.Second,
		deploymentMode: PPOModeActive,
		onlineUpdates:  true,
	}

	workers := makeWorkerMap(
		activeWorker("w-fallback", 8, 16, 100, 8, 16, 100),
		activeWorker("w-ppo", 8, 16, 100, 8, 16, 100),
	)
	selected := s.SelectWorker(simpleTask("t-1", 1, 1, 1), workers)

	if selected != "w-ppo" {
		t.Fatalf("expected PPO worker w-ppo, got %s", selected)
	}
	if fallback.calls != 0 {
		t.Fatalf("expected no fallback calls in active PPO success path, got %d", fallback.calls)
	}
	if client.loadCalls != 1 || client.selectCalls != 1 {
		t.Fatalf("expected one load/select call, got load=%d select=%d", client.loadCalls, client.selectCalls)
	}
}

func TestPPOSchedulerSelectWorkerShadowUsesFallbackDecision(t *testing.T) {
	fallback := &staticFallbackScheduler{name: "RTS", worker: "w-fallback"}
	client := &fakePPOClient{
		loadResp:   &pb.LoadModelForFingerprintResponse{Loaded: true, ModelVersion: "v3"},
		selectResp: &pb.SelectWorkerResponse{WorkerId: "w-ppo", ModelVersion: "v3"},
	}
	s := &PPOScheduler{
		client:         client,
		fallback:       fallback,
		requestTimeout: time.Second,
		deploymentMode: PPOModeShadow,
		onlineUpdates:  true,
	}

	workers := makeWorkerMap(
		activeWorker("w-fallback", 8, 16, 100, 8, 16, 100),
		activeWorker("w-ppo", 8, 16, 100, 8, 16, 100),
	)
	selected := s.SelectWorker(simpleTask("t-1", 1, 1, 1), workers)

	if selected != "w-fallback" {
		t.Fatalf("expected shadow mode to return fallback worker, got %s", selected)
	}
	if fallback.calls != 1 {
		t.Fatalf("expected one fallback call in shadow mode, got %d", fallback.calls)
	}
	if client.selectCalls != 1 {
		t.Fatalf("expected PPO select call in shadow mode, got %d", client.selectCalls)
	}
}

func TestPPOSchedulerSelectWorkerFallbackModeBypassesPPOClient(t *testing.T) {
	fallback := &staticFallbackScheduler{name: "RR", worker: "w-rr"}
	client := &fakePPOClient{
		selectErr: errors.New("should not be called"),
	}
	s := &PPOScheduler{
		client:         client,
		fallback:       fallback,
		requestTimeout: time.Second,
		deploymentMode: PPOModeFallback,
	}

	workers := makeWorkerMap(activeWorker("w-rr", 8, 16, 100, 8, 16, 100))
	selected := s.SelectWorker(simpleTask("t-1", 1, 1, 1), workers)
	if selected != "w-rr" {
		t.Fatalf("expected fallback worker, got %s", selected)
	}
	if fallback.calls != 1 {
		t.Fatalf("expected one fallback call, got %d", fallback.calls)
	}
	if client.selectCalls != 0 || client.loadCalls != 0 {
		t.Fatalf("expected no PPO calls in fallback mode, got load=%d select=%d", client.loadCalls, client.selectCalls)
	}
}

func TestPPOSchedulerActiveFallsBackOnRPCFailure(t *testing.T) {
	fallback := &staticFallbackScheduler{name: "RTS", worker: "w-rts"}
	client := &fakePPOClient{
		loadResp:  &pb.LoadModelForFingerprintResponse{Loaded: true, ModelVersion: "v2"},
		selectErr: errors.New("rpc failed"),
	}
	s := &PPOScheduler{
		client:         client,
		fallback:       fallback,
		requestTimeout: time.Second,
		deploymentMode: PPOModeActive,
		onlineUpdates:  true,
	}

	workers := makeWorkerMap(activeWorker("w-rts", 8, 16, 100, 8, 16, 100))
	selected := s.SelectWorker(simpleTask("t-2", 1, 1, 1), workers)
	if selected != "w-rts" {
		t.Fatalf("expected fallback worker after PPO RPC failure, got %s", selected)
	}
	if fallback.calls != 1 {
		t.Fatalf("expected fallback path to run exactly once, got %d", fallback.calls)
	}
}

func TestPPOSchedulerReportOutcomeGating(t *testing.T) {
	outcome := TaskOutcome{TaskID: "t-1", WorkerID: "w-1", Status: "success", RuntimeSeconds: 1.0, SLASuccess: true}

	t.Run("active with online updates reports", func(t *testing.T) {
		client := &fakePPOClient{reportResp: &pb.ReportOutcomeResponse{Accepted: true}}
		s := &PPOScheduler{
			client:         client,
			fallback:       &staticFallbackScheduler{name: "RTS", worker: "w-1"},
			requestTimeout: time.Second,
			deploymentMode: PPOModeActive,
			onlineUpdates:  true,
		}
		if err := s.ReportOutcome(context.Background(), outcome); err != nil {
			t.Fatalf("expected report outcome success, got error: %v", err)
		}
		if client.reportCalls != 1 {
			t.Fatalf("expected one report call, got %d", client.reportCalls)
		}
	})

	t.Run("active with online updates disabled skips reporting", func(t *testing.T) {
		client := &fakePPOClient{reportResp: &pb.ReportOutcomeResponse{Accepted: true}}
		s := &PPOScheduler{
			client:         client,
			fallback:       &staticFallbackScheduler{name: "RTS", worker: "w-1"},
			requestTimeout: time.Second,
			deploymentMode: PPOModeActive,
			onlineUpdates:  false,
		}
		if err := s.ReportOutcome(context.Background(), outcome); err != nil {
			t.Fatalf("expected nil error when updates disabled, got: %v", err)
		}
		if client.reportCalls != 0 {
			t.Fatalf("expected no report calls when updates disabled, got %d", client.reportCalls)
		}
	})

	t.Run("shadow mode skips reporting", func(t *testing.T) {
		client := &fakePPOClient{reportResp: &pb.ReportOutcomeResponse{Accepted: true}}
		s := &PPOScheduler{
			client:         client,
			fallback:       &staticFallbackScheduler{name: "RTS", worker: "w-1"},
			requestTimeout: time.Second,
			deploymentMode: PPOModeShadow,
			onlineUpdates:  true,
		}
		if err := s.ReportOutcome(context.Background(), outcome); err != nil {
			t.Fatalf("expected nil error in shadow mode, got: %v", err)
		}
		if client.reportCalls != 0 {
			t.Fatalf("expected no report calls in shadow mode, got %d", client.reportCalls)
		}
	})
}
