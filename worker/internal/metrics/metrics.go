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

package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Recorder struct {
	lastHeartbeatUnix prometheus.Gauge
	runningTasks      prometheus.Gauge
	resourceUsage     *prometheus.GaugeVec
	imagePullDuration *prometheus.HistogramVec
	containerCreate   *prometheus.HistogramVec
	taskRuntime       *prometheus.HistogramVec
	dockerErrors      *prometheus.CounterVec
	taskStarts        *prometheus.CounterVec
}

var (
	once     sync.Once
	recorder *Recorder
)

func Get() *Recorder {
	once.Do(func() {
		recorder = &Recorder{
			lastHeartbeatUnix: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "last_heartbeat_unix",
				Help:      "Unix timestamp of the last successful heartbeat sent by this worker.",
			}),
			runningTasks: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "running_tasks",
				Help:      "Number of task attempts currently tracked by the worker.",
			}),
			resourceUsage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "resource_usage_ratio",
				Help:      "Current normalized resource usage ratio for the worker.",
			}, []string{"resource"}),
			imagePullDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "task_image_pull_seconds",
				Help:      "Time spent pulling task images.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"task_type"}),
			containerCreate: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "task_container_create_seconds",
				Help:      "Time spent creating task containers.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"task_type"}),
			taskRuntime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "task_runtime_seconds",
				Help:      "Task runtime observed by the worker.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"task_type", "status"}),
			dockerErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "docker_errors_total",
				Help:      "Docker-related task execution errors.",
			}, []string{"stage", "task_type"}),
			taskStarts: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "worker",
				Name:      "task_starts_total",
				Help:      "Number of task attempts started by the worker.",
			}, []string{"task_type"}),
		}

		prometheus.MustRegister(
			recorder.lastHeartbeatUnix,
			recorder.runningTasks,
			recorder.resourceUsage,
			recorder.imagePullDuration,
			recorder.containerCreate,
			recorder.taskRuntime,
			recorder.dockerErrors,
			recorder.taskStarts,
		)
	})
	return recorder
}

func (r *Recorder) SetRunningTasks(count int) {
	if r != nil {
		r.runningTasks.Set(float64(count))
	}
}

func (r *Recorder) RecordHeartbeat(cpuUsage, memoryUsage, storageUsage float64) {
	if r != nil {
		r.lastHeartbeatUnix.Set(float64(time.Now().Unix()))
		r.resourceUsage.WithLabelValues("cpu").Set(cpuUsage)
		r.resourceUsage.WithLabelValues("memory").Set(memoryUsage)
		r.resourceUsage.WithLabelValues("storage").Set(storageUsage)
	}
}

func (r *Recorder) IncTaskStart(taskType string) {
	if r != nil {
		r.taskStarts.WithLabelValues(normalizeTaskType(taskType)).Inc()
	}
}

func (r *Recorder) ObserveImagePull(taskType string, started time.Time) {
	if r != nil {
		r.imagePullDuration.WithLabelValues(normalizeTaskType(taskType)).Observe(time.Since(started).Seconds())
	}
}

func (r *Recorder) ObserveContainerCreate(taskType string, started time.Time) {
	if r != nil {
		r.containerCreate.WithLabelValues(normalizeTaskType(taskType)).Observe(time.Since(started).Seconds())
	}
}

func (r *Recorder) ObserveTaskRuntime(taskType, status string, started time.Time) {
	if r != nil {
		r.taskRuntime.WithLabelValues(normalizeTaskType(taskType), status).Observe(time.Since(started).Seconds())
	}
}

func (r *Recorder) IncDockerError(stage, taskType string) {
	if r != nil {
		r.dockerErrors.WithLabelValues(stage, normalizeTaskType(taskType)).Inc()
	}
}

func normalizeTaskType(taskType string) string {
	if taskType == "" {
		return "unknown"
	}
	return taskType
}
