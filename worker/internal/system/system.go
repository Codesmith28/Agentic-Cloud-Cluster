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

// FindAvailablePort finds an available port starting from the given port number
func FindAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+100; port++ { // Try up to 100 ports
		address := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", address)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found starting from %d", startPort)
}
