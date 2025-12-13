package config

import (
	"os"
	"path/filepath"
)

// Default returns the default game configuration
func Default() *GameConfig {
	return &GameConfig{
		// Board
		BoardWidth:  50,
		BoardHeight: 50,

		// Ships
		ShipsPerPlayer: 5,
		RacksPerShip:   4,

		// Pods
		PodsPerRack: 4,

		// Bots
		MaxBots:       5,
		BotDifficulty: "medium",

		// Timing
		TurnDelayMs: 200,

		// K8s (disabled by default)
		EnableRealK8s: false,
		K8sNamespace:  "clustership",
		Kubeconfig:    defaultKubeconfig(),
	}
}

// defaultKubeconfig returns the default kubeconfig path
func defaultKubeconfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// DifficultyMultiplier returns AI strength based on difficulty
func DifficultyMultiplier(difficulty string) float64 {
	switch difficulty {
	case "easy":
		return 0.5
	case "hard":
		return 1.5
	default:
		return 1.0
	}
}
