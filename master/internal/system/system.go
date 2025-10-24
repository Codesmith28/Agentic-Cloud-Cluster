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
	MasterPort  string
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

// GetMasterAddress returns the best IP address for the master to use
func (s *SystemInfo) GetMasterAddress() string {
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
	log.Printf("=== System Information ===")
	log.Printf("Hostname: %s", s.Hostname)
	log.Printf("IP Addresses: %v", s.IPAddresses)
	log.Printf("OS: %s", s.OS)
	log.Printf("Architecture: %s", s.Arch)
	log.Printf("CPU Cores: %d", s.NumCPU)
	log.Printf("Process ID: %d", s.PID)
	log.Printf("User ID: %d", s.UID)
	log.Printf("Group ID: %d", s.GID)
	log.Printf("Master Address: %s", s.GetMasterAddress())
	log.Printf("Master Port: %s", s.GetMasterPort())
	log.Printf("==========================")
}

// SetMasterPort sets the master's communication port
func (s *SystemInfo) SetMasterPort(port string) {
	s.MasterPort = port
}

// GetMasterPort returns the master's communication port
func (s *SystemInfo) GetMasterPort() string {
	return s.MasterPort
}
