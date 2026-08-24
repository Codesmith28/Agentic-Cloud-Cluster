package ports

import "context"

// TelemetrySource provides cluster telemetry snapshots for risk computation and monitoring.
type TelemetrySource interface {
	GetWorkerViews(ctx context.Context) ([]WorkerView, error)
	GetWorkerLoad(workerID string) float64
}
