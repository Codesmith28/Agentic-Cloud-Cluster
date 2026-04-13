package system

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// HostResources holds a point-in-time sample of master host resource usage.
type HostResources struct {
	CPUPercent  float64
	MemUsedGB   float64
	MemTotalGB  float64
	MemPercent  float64
	StorUsedGB  float64
	StorTotalGB float64
	StorPercent float64
	NumCPU      int
	Hostname    string
	SampledAt   time.Time
}

// HostResourceSampler periodically samples host resources.
type HostResourceSampler struct {
	mu       sync.RWMutex
	latest   HostResources
	ticker   *time.Ticker
	stopCh   chan struct{}
	hostname string
}

// NewHostResourceSampler creates a sampler that updates every interval.
func NewHostResourceSampler(interval time.Duration) *HostResourceSampler {
	hostname, _ := os.Hostname()
	s := &HostResourceSampler{
		stopCh:   make(chan struct{}),
		hostname: hostname,
	}
	// Take initial sample
	s.sample()
	// Start periodic sampling
	s.ticker = time.NewTicker(interval)
	go s.run()
	return s
}

func (s *HostResourceSampler) run() {
	for {
		select {
		case <-s.ticker.C:
			s.sample()
		case <-s.stopCh:
			s.ticker.Stop()
			return
		}
	}
}

func (s *HostResourceSampler) sample() {
	hr := HostResources{
		NumCPU:    runtime.NumCPU(),
		Hostname:  s.hostname,
		SampledAt: time.Now(),
	}

	// Memory from runtime (Go-process-level approximation)
	hr.MemTotalGB, hr.MemUsedGB, hr.MemPercent = getSystemMemory()

	// Disk usage for root filesystem
	hr.StorTotalGB, hr.StorUsedGB, hr.StorPercent = getDiskUsage("/")

	// CPU: goroutine-based heuristic (accurate CPU requires cgo or /proc)
	hr.CPUPercent = estimateCPULoad()

	s.mu.Lock()
	s.latest = hr
	s.mu.Unlock()
}

// GetLatest returns the most recent resource sample.
func (s *HostResourceSampler) GetLatest() HostResources {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

// Stop stops the sampler.
func (s *HostResourceSampler) Stop() {
	close(s.stopCh)
}

// getSystemMemory returns total, used, and percent for system memory.
func getSystemMemory() (totalGB, usedGB, percent float64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	totalGB = float64(m.Sys) / (1024 * 1024 * 1024)
	usedGB = float64(m.Alloc) / (1024 * 1024 * 1024)
	if totalGB > 0 {
		percent = (usedGB / totalGB) * 100
	}
	return
}

// getDiskUsage returns total, used, and percent for the given path.
func getDiskUsage(path string) (totalGB, usedGB, percent float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	totalGB = float64(total) / (1024 * 1024 * 1024)
	usedGB = float64(used) / (1024 * 1024 * 1024)
	if total > 0 {
		percent = (float64(used) / float64(total)) * 100
	}
	return
}

// estimateCPULoad returns an approximate CPU load percentage.
// Uses goroutine count as a rough heuristic since accurate CPU sampling
// requires either cgo or /proc parsing.
func estimateCPULoad() float64 {
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()
	load := float64(numGoroutines) / float64(numCPU) * 10.0
	if load > 100 {
		load = 100
	}
	return load
}
