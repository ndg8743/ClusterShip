package config

import (
	"clustership/pkg/hardware"
	"encoding/json"
	"os"
	"path/filepath"
)

// GameConfig holds all configurable game settings
type GameConfig struct {
	// Board dimensions (int64 for massive scale support)
	BoardWidth  int64 `json:"board_width"`
	BoardHeight int64 `json:"board_height"`

	// Ships/Regions per player
	ShipsPerPlayer int `json:"ships_per_player"`
	RacksPerShip   int `json:"racks_per_ship"`

	// Pod configuration
	PodsPerRack int `json:"pods_per_rack"`

	// Bot/Enemy configuration
	MaxBots       int    `json:"max_bots"`
	BotDifficulty string `json:"bot_difficulty"`

	// Timing
	TurnDelayMs int `json:"turn_delay_ms"`

	// Kubernetes integration
	EnableRealK8s bool   `json:"enable_real_k8s"`
	K8sNamespace  string `json:"k8s_namespace"`
	Kubeconfig    string `json:"kubeconfig"`

	// GPU/Benchmark settings
	EnableGPU      bool `json:"enable_gpu"`
	BenchmarkMode  bool `json:"benchmark_mode"`
	UseSparseBoard bool `json:"use_sparse_board"`

	// Detected hardware info (not persisted)
	detectedTier   hardware.PerformanceTier `json:"-"`
	detectedLimits *hardware.TierLimits     `json:"-"`
	systemInfo     *hardware.SystemInfo     `json:"-"`
}

// configDir returns config directory path
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".clustership"
	}
	return filepath.Join(home, ".clustership")
}

// configPath returns config file path
func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// Load reads config from disk, returns defaults if not found
func Load() (*GameConfig, error) {
	cfg := Default()

	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return Default(), err
	}

	return cfg, nil
}

// Save writes config to disk
func (c *GameConfig) Save() error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath(), data, 0644)
}

// DetectHardware performs hardware detection and sets tier limits
func (c *GameConfig) DetectHardware() {
	c.systemInfo = hardware.Detect()
	c.detectedTier = hardware.DetermineTier(c.systemInfo)
	limits := hardware.GetTierLimits(c.detectedTier)
	c.detectedLimits = &limits
}

// GetTier returns the detected performance tier
func (c *GameConfig) GetTier() hardware.PerformanceTier {
	if c.detectedLimits == nil {
		c.DetectHardware()
	}
	return c.detectedTier
}

// GetLimits returns the tier-based limits
func (c *GameConfig) GetLimits() *hardware.TierLimits {
	if c.detectedLimits == nil {
		c.DetectHardware()
	}
	return c.detectedLimits
}

// GetSystemInfo returns detected system hardware info
func (c *GameConfig) GetSystemInfo() *hardware.SystemInfo {
	if c.systemInfo == nil {
		c.DetectHardware()
	}
	return c.systemInfo
}

// Validate ensures config values are within tier-based acceptable ranges
func (c *GameConfig) Validate() {
	// Ensure hardware detection is done
	if c.detectedLimits == nil {
		c.DetectHardware()
	}
	limits := c.detectedLimits

	// Board dimensions
	if c.BoardWidth < 20 {
		c.BoardWidth = 20
	}
	if c.BoardWidth > limits.MaxBoardWidth {
		c.BoardWidth = limits.MaxBoardWidth
	}
	if c.BoardHeight < 20 {
		c.BoardHeight = 20
	}
	if c.BoardHeight > limits.MaxBoardHeight {
		c.BoardHeight = limits.MaxBoardHeight
	}

	// Ships per player
	if c.ShipsPerPlayer < 1 {
		c.ShipsPerPlayer = 1
	}
	if c.ShipsPerPlayer > limits.MaxShipsTotal {
		c.ShipsPerPlayer = limits.MaxShipsTotal
	}

	// Racks per ship
	if c.RacksPerShip < 2 {
		c.RacksPerShip = 2
	}
	if c.RacksPerShip > limits.MaxRacksPerShip {
		c.RacksPerShip = limits.MaxRacksPerShip
	}

	// Pods per rack
	if c.PodsPerRack < 1 {
		c.PodsPerRack = 1
	}
	if c.PodsPerRack > 100 {
		c.PodsPerRack = 100
	}

	// Max bots/companies
	if c.MaxBots < 1 {
		c.MaxBots = 1
	}
	if c.MaxBots > limits.MaxCompanies {
		c.MaxBots = limits.MaxCompanies
	}

	// Timing
	if c.TurnDelayMs < 10 {
		c.TurnDelayMs = 10
	}
	if c.TurnDelayMs > 5000 {
		c.TurnDelayMs = 5000
	}

	// K8s namespace
	if c.K8sNamespace == "" {
		c.K8sNamespace = "clustership"
	}

	// Auto-enable sparse board for large scales
	if c.BoardWidth > 1000 || c.BoardHeight > 1000 {
		c.UseSparseBoard = true
	}

	// Auto-disable GPU if not available
	if !limits.UseGPU {
		c.EnableGPU = false
	}
}

// BoardWidthInt returns board width as int (for backward compatibility)
func (c *GameConfig) BoardWidthInt() int {
	if c.BoardWidth > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1) // max int
	}
	return int(c.BoardWidth)
}

// BoardHeightInt returns board height as int (for backward compatibility)
func (c *GameConfig) BoardHeightInt() int {
	if c.BoardHeight > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1) // max int
	}
	return int(c.BoardHeight)
}
