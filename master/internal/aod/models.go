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

package aod

// Metrics represents performance metrics for the scheduling system.
// Kept for reference and potential future monitoring.
type Metrics struct {
	// SLASuccess is the ratio of tasks that met their SLA deadline
	// Range: [0.0, 1.0], higher is better
	SLASuccess float64

	// Utilization is the average resource utilization across all workers
	// Range: [0.0, 1.0], higher is better (but not at cost of SLA)
	Utilization float64

	// EnergyNorm is the normalized energy consumption
	// Range: [0.0, 1.0], lower is better
	EnergyNorm float64

	// OverloadNorm is the normalized time workers spent overloaded
	// Range: [0.0, 1.0], lower is better
	OverloadNorm float64
}

// IsValid checks if metrics are within valid ranges
func (m *Metrics) IsValid() bool {
	// All metrics should be in [0, 1] range
	if m.SLASuccess < 0 || m.SLASuccess > 1.0 {
		return false
	}
	if m.Utilization < 0 || m.Utilization > 1.0 {
		return false
	}
	if m.EnergyNorm < 0 || m.EnergyNorm > 1.0 {
		return false
	}
	if m.OverloadNorm < 0 || m.OverloadNorm > 1.0 {
		return false
	}
	return true
}
