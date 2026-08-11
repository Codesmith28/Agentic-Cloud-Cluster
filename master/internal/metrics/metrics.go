package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Recorder struct {
	queueDepth          prometheus.Gauge
	taskEnqueuedTotal   *prometheus.CounterVec
	taskDequeuedTotal   *prometheus.CounterVec
	schedulingLatency   *prometheus.HistogramVec
	queueWait           *prometheus.HistogramVec
	schedulerSelections *prometheus.CounterVec
	taskTerminalTotal   *prometheus.CounterVec
	workerTimeoutsTotal *prometheus.CounterVec
	taskRequeuesTotal   *prometheus.CounterVec
	staleResultsTotal   *prometheus.CounterVec
	recoveryDuration    *prometheus.HistogramVec
}

var (
	once     sync.Once
	recorder *Recorder
)

func Get() *Recorder {
	once.Do(func() {
		recorder = &Recorder{
			queueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "queue_depth",
				Help:      "Current number of tasks waiting in the scheduling queue.",
			}),
			taskEnqueuedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "tasks_enqueued_total",
				Help:      "Total number of tasks enqueued for scheduling.",
			}, []string{"reason"}),
			taskDequeuedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "tasks_dequeued_total",
				Help:      "Total number of queued tasks that left the scheduling queue.",
			}, []string{"outcome"}),
			schedulingLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "scheduling_latency_seconds",
				Help:      "Wall-clock time spent selecting and assigning a worker.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"scheduler"}),
			queueWait: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "task_queue_wait_seconds",
				Help:      "Time a task spent waiting in the queue before assignment.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"scheduler", "task_type"}),
			schedulerSelections: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "scheduler_selections_total",
				Help:      "Number of successful worker selections.",
			}, []string{"scheduler", "task_type", "worker_id"}),
			taskTerminalTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "task_terminal_total",
				Help:      "Number of logical tasks reaching a terminal state.",
			}, []string{"status", "task_type"}),
			workerTimeoutsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "worker_timeouts_total",
				Help:      "Number of worker heartbeat timeouts detected by the master.",
			}, []string{"worker_id"}),
			taskRequeuesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "task_requeues_total",
				Help:      "Number of logical task requeues triggered by recovery.",
			}, []string{"failure_reason", "task_type"}),
			staleResultsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "stale_results_total",
				Help:      "Number of late or stale attempt results ignored by the master.",
			}, []string{"reason"}),
			recoveryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "cloudai",
				Subsystem: "master",
				Name:      "recovery_duration_seconds",
				Help:      "Time spent recovering and requeuing tasks after a worker failure.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"failure_reason"}),
		}

		prometheus.MustRegister(
			recorder.queueDepth,
			recorder.taskEnqueuedTotal,
			recorder.taskDequeuedTotal,
			recorder.schedulingLatency,
			recorder.queueWait,
			recorder.schedulerSelections,
			recorder.taskTerminalTotal,
			recorder.workerTimeoutsTotal,
			recorder.taskRequeuesTotal,
			recorder.staleResultsTotal,
			recorder.recoveryDuration,
		)
	})
	return recorder
}

func (r *Recorder) SetQueueDepth(depth int) {
	if r != nil {
		r.queueDepth.Set(float64(depth))
	}
}

func (r *Recorder) IncTaskEnqueued(reason string) {
	if r != nil {
		r.taskEnqueuedTotal.WithLabelValues(reason).Inc()
	}
}

func (r *Recorder) IncTaskDequeued(outcome string) {
	if r != nil {
		r.taskDequeuedTotal.WithLabelValues(outcome).Inc()
	}
}

func (r *Recorder) ObserveSchedulingLatency(schedulerName string, started time.Time) {
	if r != nil {
		r.schedulingLatency.WithLabelValues(schedulerName).Observe(time.Since(started).Seconds())
	}
}

func (r *Recorder) ObserveQueueWait(schedulerName, taskType string, queuedAt time.Time) {
	if r != nil {
		r.queueWait.WithLabelValues(schedulerName, normalizeTaskType(taskType)).Observe(time.Since(queuedAt).Seconds())
	}
}

func (r *Recorder) IncSchedulerSelection(schedulerName, taskType, workerID string) {
	if r != nil {
		r.schedulerSelections.WithLabelValues(schedulerName, normalizeTaskType(taskType), workerID).Inc()
	}
}

func (r *Recorder) IncTaskTerminal(status, taskType string) {
	if r != nil {
		r.taskTerminalTotal.WithLabelValues(status, normalizeTaskType(taskType)).Inc()
	}
}

func (r *Recorder) IncWorkerTimeout(workerID string) {
	if r != nil {
		r.workerTimeoutsTotal.WithLabelValues(workerID).Inc()
	}
}

func (r *Recorder) IncTaskRequeue(failureReason, taskType string) {
	if r != nil {
		r.taskRequeuesTotal.WithLabelValues(failureReason, normalizeTaskType(taskType)).Inc()
	}
}

func (r *Recorder) IncStaleResult(reason string) {
	if r != nil {
		r.staleResultsTotal.WithLabelValues(reason).Inc()
	}
}

func (r *Recorder) ObserveRecoveryDuration(failureReason string, started time.Time) {
	if r != nil {
		r.recoveryDuration.WithLabelValues(failureReason).Observe(time.Since(started).Seconds())
	}
}

func normalizeTaskType(taskType string) string {
	if taskType == "" {
		return "unknown"
	}
	return taskType
}
