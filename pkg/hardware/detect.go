package hardware

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GPUVendor identifies the GPU manufacturer
type GPUVendor int

const (
	GPUNone GPUVendor = iota
	GPUNvidia
	GPUAMD
	GPUIntel
	GPUApple
)

func (v GPUVendor) String() string {
	switch v {
	case GPUNvidia:
		return "NVIDIA"
	case GPUAMD:
		return "AMD"
	case GPUIntel:
		return "Intel"
	case GPUApple:
		return "Apple"
	default:
		return "None"
	}
}

// GPUInfo contains detected GPU information
type GPUInfo struct {
	Vendor    GPUVendor
	Model     string
	MemoryMB  int64
	Available bool
}

// SystemInfo contains detected system hardware information
type SystemInfo struct {
	TotalRAMMB   int64
	CPUCores     int
	GPU          GPUInfo
	OS           string
	Arch         string
}

// PerformanceTier defines the system capability level
type PerformanceTier int

const (
	TierBasic    PerformanceTier = iota // CPU only, small boards
	TierStandard                        // CPU, medium boards
	TierAdvanced                        // GPU available, large boards
	TierExtreme                         // High-end GPU, massive boards
)

func (t PerformanceTier) String() string {
	switch t {
	case TierBasic:
		return "Basic"
	case TierStandard:
		return "Standard"
	case TierAdvanced:
		return "Advanced"
	case TierExtreme:
		return "Extreme"
	default:
		return "Unknown"
	}
}

// TierLimits defines the maximum values for each performance tier
type TierLimits struct {
	MaxBoardWidth  int64
	MaxBoardHeight int64
	MaxCompanies   int
	MaxShipsTotal  int
	MaxRacksPerShip int
	UseGPU         bool
	UseSparseBoard bool
}

// GetTierLimits returns the limits for a performance tier
func GetTierLimits(tier PerformanceTier) TierLimits {
	switch tier {
	case TierBasic:
		return TierLimits{
			MaxBoardWidth:   100,
			MaxBoardHeight:  100,
			MaxCompanies:    6,
			MaxShipsTotal:   60,
			MaxRacksPerShip: 7,
			UseGPU:          false,
			UseSparseBoard:  false,
		}
	case TierStandard:
		return TierLimits{
			MaxBoardWidth:   1000,
			MaxBoardHeight:  1000,
			MaxCompanies:    20,
			MaxShipsTotal:   200,
			MaxRacksPerShip: 20,
			UseGPU:          false,
			UseSparseBoard:  true,
		}
	case TierAdvanced:
		return TierLimits{
			MaxBoardWidth:   100000,
			MaxBoardHeight:  100000,
			MaxCompanies:    100,
			MaxShipsTotal:   1000,
			MaxRacksPerShip: 100,
			UseGPU:          true,
			UseSparseBoard:  true,
		}
	case TierExtreme:
		return TierLimits{
			MaxBoardWidth:   10000000,
			MaxBoardHeight:  10000000,
			MaxCompanies:    1000,
			MaxShipsTotal:   10000,
			MaxRacksPerShip: 1000,
			UseGPU:          true,
			UseSparseBoard:  true,
		}
	default:
		return GetTierLimits(TierBasic)
	}
}

// Detect performs hardware detection and returns system info
func Detect() *SystemInfo {
	info := &SystemInfo{
		CPUCores: runtime.NumCPU(),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	// Detect RAM
	info.TotalRAMMB = detectRAM()

	// Detect GPU
	info.GPU = detectGPU()

	return info
}

// DetermineTier calculates the performance tier based on hardware
func DetermineTier(sys *SystemInfo) PerformanceTier {
	// Extreme tier: 32GB+ RAM and 8GB+ VRAM
	if sys.TotalRAMMB >= 32*1024 && sys.GPU.Available && sys.GPU.MemoryMB >= 8*1024 {
		return TierExtreme
	}

	// Advanced tier: 16GB+ RAM and 4GB+ VRAM
	if sys.TotalRAMMB >= 16*1024 && sys.GPU.Available && sys.GPU.MemoryMB >= 4*1024 {
		return TierAdvanced
	}

	// Standard tier: 8GB+ RAM
	if sys.TotalRAMMB >= 8*1024 {
		return TierStandard
	}

	return TierBasic
}

// detectGPU detects available GPU
func detectGPU() GPUInfo {
	// Try NVIDIA first
	if gpu := detectNvidia(); gpu.Available {
		return gpu
	}

	// Try AMD
	if gpu := detectAMD(); gpu.Available {
		return gpu
	}

	// Try Apple (macOS only)
	if runtime.GOOS == "darwin" {
		if gpu := detectApple(); gpu.Available {
			return gpu
		}
	}

	// Try Intel iGPU
	if gpu := detectIntel(); gpu.Available {
		return gpu
	}

	return GPUInfo{Vendor: GPUNone, Available: false}
}

// detectNvidia detects NVIDIA GPU using nvidia-smi
func detectNvidia() GPUInfo {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return GPUInfo{Vendor: GPUNvidia, Available: false}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return GPUInfo{Vendor: GPUNvidia, Available: false}
	}

	// Parse first GPU
	parts := strings.Split(lines[0], ", ")
	if len(parts) < 2 {
		return GPUInfo{Vendor: GPUNvidia, Available: false}
	}

	memMB, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)

	return GPUInfo{
		Vendor:    GPUNvidia,
		Model:     strings.TrimSpace(parts[0]),
		MemoryMB:  memMB,
		Available: true,
	}
}

// detectAMD detects AMD GPU using rocm-smi
func detectAMD() GPUInfo {
	cmd := exec.Command("rocm-smi", "--showproductname", "--showmeminfo", "vram")
	output, err := cmd.Output()
	if err != nil {
		return GPUInfo{Vendor: GPUAMD, Available: false}
	}

	// Parse output for model and memory
	lines := strings.Split(string(output), "\n")
	var model string
	var memMB int64

	for _, line := range lines {
		if strings.Contains(line, "Card series:") {
			model = strings.TrimSpace(strings.TrimPrefix(line, "Card series:"))
		}
		if strings.Contains(line, "VRAM Total Memory") {
			// Parse memory value
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "MB" && i > 0 {
					memMB, _ = strconv.ParseInt(parts[i-1], 10, 64)
				}
			}
		}
	}

	if model == "" {
		return GPUInfo{Vendor: GPUAMD, Available: false}
	}

	return GPUInfo{
		Vendor:    GPUAMD,
		Model:     model,
		MemoryMB:  memMB,
		Available: true,
	}
}

// detectApple detects Apple Silicon GPU
func detectApple() GPUInfo {
	// Check for Apple Silicon by looking at CPU info
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	output, err := cmd.Output()
	if err != nil {
		return GPUInfo{Vendor: GPUApple, Available: false}
	}

	cpuBrand := strings.TrimSpace(string(output))
	if !strings.Contains(cpuBrand, "Apple") {
		return GPUInfo{Vendor: GPUApple, Available: false}
	}

	// Get memory (unified on Apple Silicon)
	memCmd := exec.Command("sysctl", "-n", "hw.memsize")
	memOutput, err := memCmd.Output()
	if err != nil {
		return GPUInfo{Vendor: GPUApple, Available: false}
	}

	memBytes, _ := strconv.ParseInt(strings.TrimSpace(string(memOutput)), 10, 64)
	memMB := memBytes / (1024 * 1024)

	// Determine chip model
	var model string
	if strings.Contains(cpuBrand, "M3") {
		model = "Apple M3"
	} else if strings.Contains(cpuBrand, "M2") {
		model = "Apple M2"
	} else if strings.Contains(cpuBrand, "M1") {
		model = "Apple M1"
	} else {
		model = "Apple Silicon"
	}

	return GPUInfo{
		Vendor:    GPUApple,
		Model:     model,
		MemoryMB:  memMB, // Unified memory
		Available: true,
	}
}

// detectIntel detects Intel integrated GPU
func detectIntel() GPUInfo {
	// On Windows, check for Intel GPU in registry or use wmic
	if runtime.GOOS == "windows" {
		cmd := exec.Command("wmic", "path", "win32_videocontroller", "get", "name,adapterram")
		output, err := cmd.Output()
		if err != nil {
			return GPUInfo{Vendor: GPUIntel, Available: false}
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "intel") {
				// Extract model name
				parts := strings.Fields(line)
				if len(parts) > 0 {
					// Last field is adapter RAM in bytes
					memBytes, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
					memMB := memBytes / (1024 * 1024)

					model := strings.Join(parts[:len(parts)-1], " ")
					return GPUInfo{
						Vendor:    GPUIntel,
						Model:     strings.TrimSpace(model),
						MemoryMB:  memMB,
						Available: true,
					}
				}
			}
		}
	}

	// On Linux, check lspci
	if runtime.GOOS == "linux" {
		cmd := exec.Command("lspci", "-v")
		output, err := cmd.Output()
		if err != nil {
			return GPUInfo{Vendor: GPUIntel, Available: false}
		}

		if strings.Contains(strings.ToLower(string(output)), "intel") && strings.Contains(strings.ToLower(string(output)), "vga") {
			return GPUInfo{
				Vendor:    GPUIntel,
				Model:     "Intel Integrated Graphics",
				MemoryMB:  512, // Assume shared memory
				Available: true,
			}
		}
	}

	return GPUInfo{Vendor: GPUIntel, Available: false}
}
