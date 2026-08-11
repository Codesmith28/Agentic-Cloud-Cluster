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
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	pb "master/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	PPOModeShadow   = "shadow"
	PPOModeActive   = "active"
	PPOModeFallback = "fallback"
)

// PPOScheduler forwards scheduling decisions to a Python PPO service via gRPC.
// On any communication/validation failure, it falls back to the configured
// scheduler (typically RTS -> RR).
type PPOScheduler struct {
	client pb.PPOSchedulerClient
	conn   *grpc.ClientConn

	fallback       Scheduler
	requestTimeout time.Duration
	modelPath      string
	deploymentMode string
	onlineUpdates  bool

	mu                  sync.RWMutex
	lastFingerprintHash string
	lastModelVersion    string
}

func NewPPOScheduler(
	grpcAddr string,
	requestTimeout time.Duration,
	fallback Scheduler,
	modelPath string,
	deploymentMode string,
	onlineUpdates bool,
) (*PPOScheduler, error) {
	if fallback == nil {
		return nil, fmt.Errorf("fallback scheduler is required")
	}
	if requestTimeout <= 0 {
		requestTimeout = 1500 * time.Millisecond
	}
	mode := NormalizePPODeploymentMode(deploymentMode)

	scheduler := &PPOScheduler{
		fallback:       fallback,
		requestTimeout: requestTimeout,
		modelPath:      modelPath,
		deploymentMode: mode,
		onlineUpdates:  onlineUpdates,
	}

	// Fallback mode intentionally does not require a live PPO gRPC client.
	if mode == PPOModeFallback {
		return scheduler, nil
	}
	if grpcAddr == "" {
		return nil, fmt.Errorf("empty PPO gRPC address")
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect PPO service at %s: %w", grpcAddr, err)
	}
	scheduler.conn = conn
	scheduler.client = pb.NewPPOSchedulerClient(conn)
	return scheduler, nil
}

func NormalizePPODeploymentMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PPOModeShadow:
		return PPOModeShadow
	case PPOModeFallback:
		return PPOModeFallback
	case "", PPOModeActive:
		return PPOModeActive
	default:
		return PPOModeActive
	}
}

func (s *PPOScheduler) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *PPOScheduler) GetName() string {
	return "PPO"
}

func (s *PPOScheduler) DeploymentMode() string {
	return s.deploymentMode
}

func (s *PPOScheduler) Reset() {
	s.fallback.Reset()
	s.mu.Lock()
	s.lastFingerprintHash = ""
	s.lastModelVersion = ""
	s.mu.Unlock()
}

func (s *PPOScheduler) Ping(ctx context.Context) (*pb.PingResponse, error) {
	if s.client == nil {
		return nil, fmt.Errorf("PPO client unavailable in %s mode", s.deploymentMode)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()
	return s.client.Ping(reqCtx, &pb.PingRequest{})
}

func (s *PPOScheduler) SelectWorker(task *pb.Task, workers map[string]*WorkerInfo) string {
	if s.deploymentMode == PPOModeFallback {
		return s.fallback.SelectWorker(task, workers)
	}
	if task == nil || len(workers) == 0 {
		return s.fallback.SelectWorker(task, workers)
	}
	if s.client == nil {
		log.Printf("⚠️ PPO: client unavailable, fallback -> %s", s.fallback.GetName())
		return s.fallback.SelectWorker(task, workers)
	}

	fingerprintHash, fingerprintPayload := BuildClusterFingerprint(workers)
	if err := s.ensureModelLoaded(fingerprintHash, fingerprintPayload); err != nil {
		log.Printf("⚠️ PPO: model load failed (%v), fallback -> %s", err, s.fallback.GetName())
		return s.fallback.SelectWorker(task, workers)
	}

	candidates := make([]*pb.CandidateWorker, 0, len(workers))
	for _, workerID := range sortedWorkerIDs(workers) {
		w := workers[workerID]
		if w == nil {
			continue
		}
		candidates = append(candidates, &pb.CandidateWorker{
			WorkerId:           w.WorkerID,
			WorkerIp:           w.WorkerIP,
			IsActive:           w.IsActive,
			AvailableCpu:       w.AvailableCPU,
			AvailableMemory:    w.AvailableMemory,
			AvailableStorage:   w.AvailableStorage,
			TotalCpu:           w.TotalCPU,
			TotalMemory:        w.TotalMemory,
			TotalStorage:       w.TotalStorage,
			CurrentCpuUsage:    w.CurrentCPUUsage,
			CurrentMemoryUsage: w.CurrentMemUsage,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.requestTimeout)
	defer cancel()

	resp, err := s.client.SelectWorker(ctx, &pb.SelectWorkerRequest{
		Task:                      task,
		Workers:                   candidates,
		ClusterFingerprintHash:    fingerprintHash,
		ClusterFingerprintPayload: fingerprintPayload,
		FallbackScheduler:         s.fallback.GetName(),
	})
	if err != nil {
		log.Printf("⚠️ PPO: SelectWorker RPC failed (%v), fallback -> %s", err, s.fallback.GetName())
		return s.fallback.SelectWorker(task, workers)
	}
	if resp == nil || resp.WorkerId == "" {
		return s.fallback.SelectWorker(task, workers)
	}

	worker, exists := workers[resp.WorkerId]
	if !exists || !isWorkerSuitableForTask(worker, task) {
		log.Printf("⚠️ PPO: returned invalid/unsuitable worker %q, fallback -> %s", resp.WorkerId, s.fallback.GetName())
		return s.fallback.SelectWorker(task, workers)
	}

	s.mu.Lock()
	s.lastFingerprintHash = fingerprintHash
	if resp.ModelVersion != "" {
		s.lastModelVersion = resp.ModelVersion
	}
	s.mu.Unlock()

	if s.deploymentMode == PPOModeShadow {
		fallbackWorker := s.fallback.SelectWorker(task, workers)
		if fallbackWorker != resp.WorkerId {
			log.Printf(
				"ℹ️ PPO shadow divergence task=%s ppo=%s fallback=%s",
				task.TaskId,
				resp.WorkerId,
				fallbackWorker,
			)
		}
		return fallbackWorker
	}

	return resp.WorkerId
}

func (s *PPOScheduler) ReportOutcome(ctx context.Context, outcome TaskOutcome) error {
	if s.deploymentMode != PPOModeActive || !s.onlineUpdates {
		return nil
	}
	if s.client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.RLock()
	fingerprintHash := s.lastFingerprintHash
	modelVersion := s.lastModelVersion
	s.mu.RUnlock()

	if outcome.ClusterHash != "" {
		fingerprintHash = outcome.ClusterHash
	}
	if outcome.ModelVersionHint != "" {
		modelVersion = outcome.ModelVersionHint
	}

	reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	resp, err := s.client.ReportOutcome(reqCtx, &pb.ReportOutcomeRequest{
		TaskId:          outcome.TaskID,
		WorkerId:        outcome.WorkerID,
		Status:          outcome.Status,
		Reward:          outcome.Reward,
		RuntimeSeconds:  outcome.RuntimeSeconds,
		SlaSuccess:      outcome.SLASuccess,
		FingerprintHash: fingerprintHash,
		ModelVersion:    modelVersion,
		Task:            outcome.Task,
	})
	if err != nil {
		return err
	}
	if resp == nil || !resp.Accepted {
		if resp == nil {
			return fmt.Errorf("empty outcome response")
		}
		return fmt.Errorf("outcome rejected: %s", resp.Message)
	}
	return nil
}

func (s *PPOScheduler) ensureModelLoaded(fingerprintHash, fingerprintPayload string) error {
	if fingerprintHash == "" {
		return nil
	}
	if s.client == nil {
		return fmt.Errorf("PPO client unavailable")
	}

	s.mu.RLock()
	if fingerprintHash == s.lastFingerprintHash {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), s.requestTimeout)
	defer cancel()

	resp, err := s.client.LoadModelForFingerprint(ctx, &pb.LoadModelForFingerprintRequest{
		SchedulerType:      "PPO",
		FingerprintHash:    fingerprintHash,
		FingerprintPayload: fingerprintPayload,
		ModelPath:          s.modelPath,
		CreateIfMissing:    true,
	})
	if err != nil {
		return err
	}
	if resp == nil || !resp.Loaded {
		if resp == nil {
			return fmt.Errorf("empty load-model response")
		}
		return fmt.Errorf("load-model rejected: %s", resp.Message)
	}

	s.mu.Lock()
	s.lastFingerprintHash = fingerprintHash
	if resp.ModelVersion != "" {
		s.lastModelVersion = resp.ModelVersion
	}
	s.mu.Unlock()
	return nil
}

func sortedWorkerIDs(workers map[string]*WorkerInfo) []string {
	ids := make([]string, 0, len(workers))
	for id := range workers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func isWorkerSuitableForTask(worker *WorkerInfo, task *pb.Task) bool {
	if worker == nil || task == nil {
		return false
	}
	if !worker.IsActive || worker.WorkerIP == "" {
		return false
	}
	if worker.AvailableCPU < task.ReqCpu {
		return false
	}
	if worker.AvailableMemory < task.ReqMemory {
		return false
	}
	if worker.AvailableStorage < task.ReqStorage {
		return false
	}
	return true
}
