package system

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
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
	TotalGPU     float64 // Number of GPU cores (0 if not available)
}

// GPUInfo holds detailed information about a single GPU
type GPUInfo struct {
	Index             int     // GPU index (0, 1, 2, ...)
	Name              string  // GPU model name
	MemoryTotalGB     float64 // Total GPU memory in GB
	ComputeCapability string  // CUDA compute capability (e.g., "8.9")
	DriverVersion     string  // NVIDIA driver version
	CUDAVersion       string  // CUDA version
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

// GetSystemResources retrieves actual system resources (CPU, Memory, Storage, GPU)
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

	// Detect GPU count
	gpuCount, err := detectGPUCount()
	if err != nil {
		log.Printf("Info: No GPU detected or nvidia-smi not available: %v", err)
		resources.TotalGPU = 0.0 // No GPU available
	} else {
		resources.TotalGPU = gpuCount
		log.Printf("Detected %d GPU(s)", int(gpuCount))
	}

	return resources, nil
}

// getTotalMemory returns the total system memory in GB
func getTotalMemory() (float64, error) {
	// Try reading from /proc/meminfo first (Linux)
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		// Fallback to sysinfo syscall
		return getMemoryViaSysinfo()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memKB, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse memory value: %w", err)
				}
				// Convert KB to GB
				memGB := float64(memKB) / (1024.0 * 1024.0)
				return memGB, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return getMemoryViaSysinfo()
	}

	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// getMemoryViaSysinfo gets memory using syscall.Sysinfo (fallback method)
func getMemoryViaSysinfo() (float64, error) {
	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	if err != nil {
		return 0, fmt.Errorf("sysinfo syscall failed: %w", err)
	}

	// Total RAM in bytes = Totalram * Unit
	totalRAM := info.Totalram * uint64(info.Unit)
	// Convert bytes to GB
	memGB := float64(totalRAM) / (1024.0 * 1024.0 * 1024.0)
	return memGB, nil
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

// detectGPUCount detects the number of NVIDIA GPUs using NVML (NVIDIA Management Library)
// This provides more accurate and detailed GPU information than nvidia-smi
func detectGPUCount() (float64, error) {
	// Initialize NVML
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return 0.0, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

	// Get device count
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return 0.0, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	// Log detailed GPU information
	if count > 0 {
		log.Printf("Detected %d NVIDIA GPU(s):", count)
		for i := 0; i < count; i++ {
			device, ret := nvml.DeviceGetHandleByIndex(i)
			if ret != nvml.SUCCESS {
				log.Printf("  [%d] Failed to get device handle: %v", i, nvml.ErrorString(ret))
				continue
			}

			// Get GPU name
			name, ret := device.GetName()
			if ret == nvml.SUCCESS {
				log.Printf("  [%d] %s", i, name)
			}

			// Get memory info (optional, for informational purposes)
			memory, ret := device.GetMemoryInfo()
			if ret == nvml.SUCCESS {
				totalGB := float64(memory.Total) / (1024 * 1024 * 1024)
				log.Printf("      Memory: %.2f GB", totalGB)
			}

			// Get compute capability (optional)
			major, minor, ret := device.GetCudaComputeCapability()
			if ret == nvml.SUCCESS {
				log.Printf("      Compute Capability: %d.%d", major, minor)
			}
		}
	}

	return float64(count), nil
}

// GetDetailedGPUInfo returns detailed information about all GPUs
// This can be used for logging or advanced GPU management
func GetDetailedGPUInfo() ([]GPUInfo, error) {
	// Initialize NVML
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

	// Get device count
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	gpus := make([]GPUInfo, 0, count)

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			log.Printf("Warning: Failed to get device handle for GPU %d: %v", i, nvml.ErrorString(ret))
			continue
		}

		gpuInfo := GPUInfo{Index: i}

		// Get GPU name
		if name, ret := device.GetName(); ret == nvml.SUCCESS {
			gpuInfo.Name = name
		}

		// Get memory info
		if memory, ret := device.GetMemoryInfo(); ret == nvml.SUCCESS {
			gpuInfo.MemoryTotalGB = float64(memory.Total) / (1024 * 1024 * 1024)
		}

		// Get compute capability
		if major, minor, ret := device.GetCudaComputeCapability(); ret == nvml.SUCCESS {
			gpuInfo.ComputeCapability = fmt.Sprintf("%d.%d", major, minor)
		}

		gpus = append(gpus, gpuInfo)
	}

	// Get driver and CUDA versions (global info)
	if len(gpus) > 0 {
		if driverVersion, ret := nvml.SystemGetDriverVersion(); ret == nvml.SUCCESS {
			for i := range gpus {
				gpus[i].DriverVersion = driverVersion
			}
		}

		if cudaVersion, ret := nvml.SystemGetCudaDriverVersion(); ret == nvml.SUCCESS {
			cudaVersionStr := fmt.Sprintf("%d.%d", cudaVersion/1000, (cudaVersion%1000)/10)
			for i := range gpus {
				gpus[i].CUDAVersion = cudaVersionStr
			}
		}
	}

	return gpus, nil
}
