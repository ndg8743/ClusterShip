package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GameConfig holds all configurable game settings
type GameConfig struct {
	// Board dimensions
	BoardWidth  int `json:"board_width"`
	BoardHeight int `json:"board_height"`

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

// Validate ensures config values are within acceptable ranges
func (c *GameConfig) Validate() {
	if c.BoardWidth < 20 {
		c.BoardWidth = 20
	}
	if c.BoardWidth > 100 {
		c.BoardWidth = 100
	}
	if c.BoardHeight < 20 {
		c.BoardHeight = 20
	}
	if c.BoardHeight > 100 {
		c.BoardHeight = 100
	}
	if c.ShipsPerPlayer < 1 {
		c.ShipsPerPlayer = 1
	}
	if c.ShipsPerPlayer > 10 {
		c.ShipsPerPlayer = 10
	}
	if c.RacksPerShip < 2 {
		c.RacksPerShip = 2
	}
	if c.RacksPerShip > 7 {
		c.RacksPerShip = 7
	}
	if c.PodsPerRack < 1 {
		c.PodsPerRack = 1
	}
	if c.PodsPerRack > 10 {
		c.PodsPerRack = 10
	}
	if c.MaxBots < 1 {
		c.MaxBots = 1
	}
	if c.MaxBots > 6 {
		c.MaxBots = 6
	}
	if c.TurnDelayMs < 50 {
		c.TurnDelayMs = 50
	}
	if c.TurnDelayMs > 2000 {
		c.TurnDelayMs = 2000
	}
	if c.K8sNamespace == "" {
		c.K8sNamespace = "clustership"
	}
}
