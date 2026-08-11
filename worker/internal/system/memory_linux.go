//go:build linux

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

package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// getTotalMemory returns total system memory in GB on Linux.
func getTotalMemory() (float64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return getMemoryViaSysinfo()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			memKB, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse memory value: %w", err)
			}
			return float64(memKB) / (1024.0 * 1024.0), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return getMemoryViaSysinfo()
	}
	return getMemoryViaSysinfo()
}

func getMemoryViaSysinfo() (float64, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, fmt.Errorf("sysinfo syscall failed: %w", err)
	}

	totalRAM := info.Totalram * uint64(info.Unit)
	return float64(totalRAM) / (1024.0 * 1024.0 * 1024.0), nil
}
