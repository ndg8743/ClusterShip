package hardware

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// detectRAM returns total system RAM in MB
func detectRAM() int64 {
	switch runtime.GOOS {
	case "darwin":
		return detectRAMMac()
	case "linux":
		return detectRAMLinux()
	case "windows":
		return detectRAMWindows()
	default:
		return 4096 // Assume 4GB default
	}
}

func detectRAMMac() int64 {
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		return 4096
	}

	memBytes, _ := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	return memBytes / (1024 * 1024)
}

func detectRAMLinux() int64 {
	cmd := exec.Command("grep", "MemTotal", "/proc/meminfo")
	output, err := cmd.Output()
	if err != nil {
		return 4096
	}

	// Parse "MemTotal:    16384000 kB"
	line := strings.TrimSpace(string(output))
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		memKB, _ := strconv.ParseInt(parts[1], 10, 64)
		return memKB / 1024
	}

	return 4096
}

func detectRAMWindows() int64 {
	cmd := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory")
	output, err := cmd.Output()
	if err != nil {
		return 4096
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "TotalPhysicalMemory" {
			memBytes, _ := strconv.ParseInt(line, 10, 64)
			return memBytes / (1024 * 1024)
		}
	}

	return 4096
}
