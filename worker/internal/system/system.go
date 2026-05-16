package system

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"
)

// SystemInfo holds system information collected at runtime
type SystemInfo struct {
	Hostname    string
	IPAddresses []string
	OS          string
	Arch        string
	NumCPU      int
	PID         int
	UID         int
	GID         int
	WorkerPort  int
}

// ResourceInfo holds system resource information
type ResourceInfo struct {
	TotalCPU     float64 // Number of CPU cores
	TotalMemory  float64 // Total memory in GB
	TotalStorage float64 // Total storage in GB
}

// CollectSystemInfo collects system information using syscalls and Go runtime
func CollectSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
		PID:    os.Getpid(),
		UID:    syscall.Getuid(),
		GID:    syscall.Getgid(),
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}
	info.Hostname = hostname

	// Get IP addresses
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					info.IPAddresses = append(info.IPAddresses, ipnet.IP.String())
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to get IP addresses: %v", err)
	}

	return info, nil
}

// GetWorkerAddress returns the best IP address for the worker to use
func (s *SystemInfo) GetWorkerAddress() string {
	if len(s.IPAddresses) > 0 {
		// Prefer non-localhost addresses
		for _, ip := range s.IPAddresses {
			if ip != "127.0.0.1" && ip != "localhost" {
				return ip
			}
		}
		// Fallback to first available
		return s.IPAddresses[0]
	}
	return "localhost"
}

// LogSystemInfo logs the collected system information
func (s *SystemInfo) LogSystemInfo() {
	log.Printf("=== Worker System Information ===")
	log.Printf("Hostname: %s", s.Hostname)
	log.Printf("IP Addresses: %v", s.IPAddresses)
	log.Printf("OS: %s", s.OS)
	log.Printf("Architecture: %s", s.Arch)
	log.Printf("CPU Cores: %d", s.NumCPU)
	log.Printf("Process ID: %d", s.PID)
	log.Printf("User ID: %d", s.UID)
	log.Printf("Group ID: %d", s.GID)
	log.Printf("Worker Address: %s", s.GetWorkerAddress())
	log.Printf("Worker Port: %s", s.GetWorkerPort())
	log.Printf("===============================")
}

// SetWorkerPort sets the worker's communication port
func (s *SystemInfo) SetWorkerPort(port int) {
	s.WorkerPort = port
}

// GetWorkerPort returns the worker's communication port as a string
func (s *SystemInfo) GetWorkerPort() string {
	return fmt.Sprintf(":%d", s.WorkerPort)
}

// PortScanRange is the number of consecutive ports to probe when
// searching for a free port. 500 allows up to ~500 co-located workers
// on a single host before exhaustion (bare-metal without containers).
const PortScanRange = 500

// FindAvailablePort finds an available port starting from the given port number.
// It probes up to PortScanRange consecutive ports and returns the first one
// that can be bound. For truly unlimited scaling, deploy workers in
// containers (each gets its own network namespace) or on separate hosts.
func FindAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+PortScanRange; port++ {
		if port > 65535 {
			break
		}
		address := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", address)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d–%d", startPort, min(startPort+PortScanRange-1, 65535))
}

// GetSystemResources retrieves actual system resources (CPU, Memory, Storage).
func GetSystemResources() (*ResourceInfo, error) {
	resources := &ResourceInfo{
		TotalCPU: float64(runtime.NumCPU()),
	}

	// Get total memory
	memory, err := getTotalMemory()
	if err != nil {
		log.Printf("Warning: Failed to get total memory, using default: %v", err)
		resources.TotalMemory = 8.0 // Default fallback
	} else {
		resources.TotalMemory = memory
	}

	// Get total storage (disk space)
	storage, err := getTotalStorage()
	if err != nil {
		log.Printf("Warning: Failed to get total storage, using default: %v", err)
		resources.TotalStorage = 100.0 // Default fallback
	} else {
		resources.TotalStorage = storage
	}

	applyResourceOverrides(resources)

	return resources, nil
}

// getTotalStorage returns the total available storage in GB for the root filesystem
func getTotalStorage() (float64, error) {
	var stat syscall.Statfs_t

	// Get filesystem statistics for root directory
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, fmt.Errorf("statfs syscall failed: %w", err)
	}

	// Total size = Blocks * Block size
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	// Convert bytes to GB
	totalGB := float64(totalBytes) / (1024.0 * 1024.0 * 1024.0)

	return totalGB, nil
}

// GetAvailableStorage returns the available storage in GB for the root filesystem
func GetAvailableStorage() (float64, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, fmt.Errorf("statfs syscall failed: %w", err)
	}

	// Available size = Available blocks * Block size
	availBytes := stat.Bavail * uint64(stat.Bsize)
	// Convert bytes to GB
	availGB := float64(availBytes) / (1024.0 * 1024.0 * 1024.0)

	return availGB, nil
}
