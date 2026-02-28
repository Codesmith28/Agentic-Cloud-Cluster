//go:build !linux

package system

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
)

// getTotalMemory returns total system memory in GB on non-Linux systems.
func getTotalMemory() (float64, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("virtual memory lookup failed: %w", err)
	}
	return float64(vm.Total) / (1024.0 * 1024.0 * 1024.0), nil
}
