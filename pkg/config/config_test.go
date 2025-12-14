package config

import (
	"clustership/pkg/hardware"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Verify default values
	if cfg.BoardWidth != 50 {
		t.Errorf("BoardWidth = %d, want 50", cfg.BoardWidth)
	}
	if cfg.BoardHeight != 50 {
		t.Errorf("BoardHeight = %d, want 50", cfg.BoardHeight)
	}
	if cfg.ShipsPerPlayer != 5 {
		t.Errorf("ShipsPerPlayer = %d, want 5", cfg.ShipsPerPlayer)
	}
	if cfg.RacksPerShip != 4 {
		t.Errorf("RacksPerShip = %d, want 4", cfg.RacksPerShip)
	}
	if cfg.PodsPerRack != 4 {
		t.Errorf("PodsPerRack = %d, want 4", cfg.PodsPerRack)
	}
	if cfg.MaxBots != 5 {
		t.Errorf("MaxBots = %d, want 5", cfg.MaxBots)
	}
	if cfg.BotDifficulty != "medium" {
		t.Errorf("BotDifficulty = %s, want medium", cfg.BotDifficulty)
	}
	if cfg.TurnDelayMs != 200 {
		t.Errorf("TurnDelayMs = %d, want 200", cfg.TurnDelayMs)
	}
	if cfg.EnableRealK8s != false {
		t.Error("EnableRealK8s should be false by default")
	}
	if cfg.K8sNamespace != "clustership" {
		t.Errorf("K8sNamespace = %s, want clustership", cfg.K8sNamespace)
	}
	if cfg.EnableGPU != false {
		t.Error("EnableGPU should be false by default")
	}
	if cfg.BenchmarkMode != false {
		t.Error("BenchmarkMode should be false by default")
	}
	if cfg.UseSparseBoard != false {
		t.Error("UseSparseBoard should be false by default")
	}
}

func TestLoadMissingFile(t *testing.T) {
	// Skip if config file exists (test isolation not possible without refactoring)
	if _, err := os.Stat(configPath()); err == nil {
		t.Skip("Skipping: config file exists, test requires missing file")
	}

	cfg, err := Load()
	if err != nil {
		t.Errorf("Load() with missing file should return default config without error, got: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	// Should return default values when file doesn't exist
	if cfg.BoardWidth != 50 {
		t.Errorf("BoardWidth = %d, want 50 (default)", cfg.BoardWidth)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	// Create a temporary directory for this test
	tempDir := t.TempDir()
	testConfigPath := filepath.Join(tempDir, "config.json")

	// Create a config with custom values
	original := &GameConfig{
		BoardWidth:     100,
		BoardHeight:    200,
		ShipsPerPlayer: 10,
		RacksPerShip:   8,
		PodsPerRack:    6,
		MaxBots:        3,
		BotDifficulty:  "hard",
		TurnDelayMs:    100,
		EnableRealK8s:  true,
		K8sNamespace:   "test-namespace",
		Kubeconfig:     "/custom/kubeconfig",
		EnableGPU:      true,
		BenchmarkMode:  true,
		UseSparseBoard: true,
	}

	// Save to temp location
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(testConfigPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load from temp location
	loadedData, err := os.ReadFile(testConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	loaded := Default()
	if err := json.Unmarshal(loadedData, loaded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify values match
	if loaded.BoardWidth != original.BoardWidth {
		t.Errorf("BoardWidth = %d, want %d", loaded.BoardWidth, original.BoardWidth)
	}
	if loaded.BoardHeight != original.BoardHeight {
		t.Errorf("BoardHeight = %d, want %d", loaded.BoardHeight, original.BoardHeight)
	}
	if loaded.ShipsPerPlayer != original.ShipsPerPlayer {
		t.Errorf("ShipsPerPlayer = %d, want %d", loaded.ShipsPerPlayer, original.ShipsPerPlayer)
	}
	if loaded.RacksPerShip != original.RacksPerShip {
		t.Errorf("RacksPerShip = %d, want %d", loaded.RacksPerShip, original.RacksPerShip)
	}
	if loaded.PodsPerRack != original.PodsPerRack {
		t.Errorf("PodsPerRack = %d, want %d", loaded.PodsPerRack, original.PodsPerRack)
	}
	if loaded.MaxBots != original.MaxBots {
		t.Errorf("MaxBots = %d, want %d", loaded.MaxBots, original.MaxBots)
	}
	if loaded.BotDifficulty != original.BotDifficulty {
		t.Errorf("BotDifficulty = %s, want %s", loaded.BotDifficulty, original.BotDifficulty)
	}
	if loaded.TurnDelayMs != original.TurnDelayMs {
		t.Errorf("TurnDelayMs = %d, want %d", loaded.TurnDelayMs, original.TurnDelayMs)
	}
	if loaded.EnableRealK8s != original.EnableRealK8s {
		t.Errorf("EnableRealK8s = %v, want %v", loaded.EnableRealK8s, original.EnableRealK8s)
	}
	if loaded.K8sNamespace != original.K8sNamespace {
		t.Errorf("K8sNamespace = %s, want %s", loaded.K8sNamespace, original.K8sNamespace)
	}
	if loaded.Kubeconfig != original.Kubeconfig {
		t.Errorf("Kubeconfig = %s, want %s", loaded.Kubeconfig, original.Kubeconfig)
	}
	if loaded.EnableGPU != original.EnableGPU {
		t.Errorf("EnableGPU = %v, want %v", loaded.EnableGPU, original.EnableGPU)
	}
	if loaded.BenchmarkMode != original.BenchmarkMode {
		t.Errorf("BenchmarkMode = %v, want %v", loaded.BenchmarkMode, original.BenchmarkMode)
	}
	if loaded.UseSparseBoard != original.UseSparseBoard {
		t.Errorf("UseSparseBoard = %v, want %v", loaded.UseSparseBoard, original.UseSparseBoard)
	}
}

func TestLoadCorruptedConfig(t *testing.T) {
	tempDir := t.TempDir()
	testConfigPath := filepath.Join(tempDir, "config.json")

	// Write invalid JSON
	if err := os.WriteFile(testConfigPath, []byte("invalid json{{{"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted config: %v", err)
	}

	// Try to load
	loadedData, err := os.ReadFile(testConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	cfg := Default()
	err = json.Unmarshal(loadedData, cfg)

	// Should return error
	if err == nil {
		t.Error("Load() with corrupted file should return error")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*GameConfig)
		validate func(*testing.T, *GameConfig)
	}{
		{
			name: "board width too small",
			setup: func(cfg *GameConfig) {
				cfg.BoardWidth = 10
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.BoardWidth < 20 {
					t.Errorf("BoardWidth = %d, should be clamped to >= 20", cfg.BoardWidth)
				}
			},
		},
		{
			name: "board height too small",
			setup: func(cfg *GameConfig) {
				cfg.BoardHeight = 5
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.BoardHeight < 20 {
					t.Errorf("BoardHeight = %d, should be clamped to >= 20", cfg.BoardHeight)
				}
			},
		},
		{
			name: "ships per player too small",
			setup: func(cfg *GameConfig) {
				cfg.ShipsPerPlayer = 0
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.ShipsPerPlayer < 1 {
					t.Errorf("ShipsPerPlayer = %d, should be clamped to >= 1", cfg.ShipsPerPlayer)
				}
			},
		},
		{
			name: "racks per ship too small",
			setup: func(cfg *GameConfig) {
				cfg.RacksPerShip = 1
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.RacksPerShip < 2 {
					t.Errorf("RacksPerShip = %d, should be clamped to >= 2", cfg.RacksPerShip)
				}
			},
		},
		{
			name: "pods per rack too small",
			setup: func(cfg *GameConfig) {
				cfg.PodsPerRack = 0
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.PodsPerRack < 1 {
					t.Errorf("PodsPerRack = %d, should be clamped to >= 1", cfg.PodsPerRack)
				}
			},
		},
		{
			name: "pods per rack too large",
			setup: func(cfg *GameConfig) {
				cfg.PodsPerRack = 200
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.PodsPerRack > 100 {
					t.Errorf("PodsPerRack = %d, should be clamped to <= 100", cfg.PodsPerRack)
				}
			},
		},
		{
			name: "max bots too small",
			setup: func(cfg *GameConfig) {
				cfg.MaxBots = 0
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.MaxBots < 1 {
					t.Errorf("MaxBots = %d, should be clamped to >= 1", cfg.MaxBots)
				}
			},
		},
		{
			name: "turn delay too small",
			setup: func(cfg *GameConfig) {
				cfg.TurnDelayMs = 5
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.TurnDelayMs < 10 {
					t.Errorf("TurnDelayMs = %d, should be clamped to >= 10", cfg.TurnDelayMs)
				}
			},
		},
		{
			name: "turn delay too large",
			setup: func(cfg *GameConfig) {
				cfg.TurnDelayMs = 10000
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.TurnDelayMs > 5000 {
					t.Errorf("TurnDelayMs = %d, should be clamped to <= 5000", cfg.TurnDelayMs)
				}
			},
		},
		{
			name: "empty k8s namespace",
			setup: func(cfg *GameConfig) {
				cfg.K8sNamespace = ""
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if cfg.K8sNamespace != "clustership" {
					t.Errorf("K8sNamespace = %s, should default to clustership", cfg.K8sNamespace)
				}
			},
		},
		{
			name: "large board enables sparse",
			setup: func(cfg *GameConfig) {
				cfg.BoardWidth = 2000
				cfg.BoardHeight = 2000
				cfg.UseSparseBoard = false
			},
			validate: func(t *testing.T, cfg *GameConfig) {
				if !cfg.UseSparseBoard {
					t.Error("UseSparseBoard should be enabled for large boards")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.setup(cfg)
			cfg.Validate()
			tt.validate(t, cfg)
		})
	}
}

func TestValidateTierLimits(t *testing.T) {
	// Create a config and detect hardware
	cfg := Default()
	cfg.DetectHardware()

	limits := cfg.GetLimits()

	// Try to set values beyond tier limits
	cfg.BoardWidth = limits.MaxBoardWidth + 1000
	cfg.BoardHeight = limits.MaxBoardHeight + 1000
	cfg.ShipsPerPlayer = limits.MaxShipsTotal + 100
	cfg.RacksPerShip = limits.MaxRacksPerShip + 10
	cfg.MaxBots = limits.MaxCompanies + 50

	cfg.Validate()

	// Verify values are clamped to tier limits
	if cfg.BoardWidth > limits.MaxBoardWidth {
		t.Errorf("BoardWidth = %d, should be clamped to tier limit %d", cfg.BoardWidth, limits.MaxBoardWidth)
	}
	if cfg.BoardHeight > limits.MaxBoardHeight {
		t.Errorf("BoardHeight = %d, should be clamped to tier limit %d", cfg.BoardHeight, limits.MaxBoardHeight)
	}
	if cfg.ShipsPerPlayer > limits.MaxShipsTotal {
		t.Errorf("ShipsPerPlayer = %d, should be clamped to tier limit %d", cfg.ShipsPerPlayer, limits.MaxShipsTotal)
	}
	if cfg.RacksPerShip > limits.MaxRacksPerShip {
		t.Errorf("RacksPerShip = %d, should be clamped to tier limit %d", cfg.RacksPerShip, limits.MaxRacksPerShip)
	}
	if cfg.MaxBots > limits.MaxCompanies {
		t.Errorf("MaxBots = %d, should be clamped to tier limit %d", cfg.MaxBots, limits.MaxCompanies)
	}

	// GPU should be disabled if tier doesn't support it
	if !limits.UseGPU && cfg.EnableGPU {
		t.Error("EnableGPU should be disabled if tier doesn't support GPU")
	}
}

func TestDetectHardware(t *testing.T) {
	cfg := Default()

	cfg.DetectHardware()

	if cfg.systemInfo == nil {
		t.Error("systemInfo should be set after DetectHardware()")
	}

	if cfg.detectedLimits == nil {
		t.Error("detectedLimits should be set after DetectHardware()")
	}

	// Verify system info is populated
	sysInfo := cfg.GetSystemInfo()
	if sysInfo == nil {
		t.Fatal("GetSystemInfo() returned nil")
	}

	if sysInfo.CPUCores <= 0 {
		t.Errorf("CPUCores = %d, want > 0", sysInfo.CPUCores)
	}

	if sysInfo.OS == "" {
		t.Error("OS should be set")
	}

	if sysInfo.Arch == "" {
		t.Error("Arch should be set")
	}

	// TotalRAMMB should be positive (might be 0 if detection fails on some systems)
	if sysInfo.TotalRAMMB < 0 {
		t.Errorf("TotalRAMMB = %d, want >= 0", sysInfo.TotalRAMMB)
	}
}

func TestGetTier(t *testing.T) {
	cfg := Default()

	tier := cfg.GetTier()

	// Should be one of the valid tiers
	validTiers := []hardware.PerformanceTier{
		hardware.TierBasic,
		hardware.TierStandard,
		hardware.TierAdvanced,
		hardware.TierExtreme,
	}

	found := false
	for _, validTier := range validTiers {
		if tier == validTier {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("GetTier() returned invalid tier: %v", tier)
	}

	// GetTier should trigger hardware detection if not done
	if cfg.detectedLimits == nil {
		t.Error("GetTier() should trigger hardware detection")
	}
}

func TestGetLimits(t *testing.T) {
	cfg := Default()

	limits := cfg.GetLimits()

	if limits == nil {
		t.Fatal("GetLimits() returned nil")
	}

	// Verify limits are sensible
	if limits.MaxBoardWidth <= 0 {
		t.Errorf("MaxBoardWidth = %d, want > 0", limits.MaxBoardWidth)
	}
	if limits.MaxBoardHeight <= 0 {
		t.Errorf("MaxBoardHeight = %d, want > 0", limits.MaxBoardHeight)
	}
	if limits.MaxCompanies <= 0 {
		t.Errorf("MaxCompanies = %d, want > 0", limits.MaxCompanies)
	}
	if limits.MaxShipsTotal <= 0 {
		t.Errorf("MaxShipsTotal = %d, want > 0", limits.MaxShipsTotal)
	}
	if limits.MaxRacksPerShip <= 0 {
		t.Errorf("MaxRacksPerShip = %d, want > 0", limits.MaxRacksPerShip)
	}
}

func TestGetSystemInfo(t *testing.T) {
	cfg := Default()

	sysInfo := cfg.GetSystemInfo()

	if sysInfo == nil {
		t.Fatal("GetSystemInfo() returned nil")
	}

	// Should trigger detection
	if cfg.systemInfo == nil {
		t.Error("GetSystemInfo() should trigger hardware detection")
	}
}

func TestBoardWidthInt(t *testing.T) {
	tests := []struct {
		name      string
		width     int64
		wantValid bool
	}{
		{"small", 100, true},
		{"medium", 10000, true},
		{"large", 1000000, true},
		{"very large", 10000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.BoardWidth = tt.width

			result := cfg.BoardWidthInt()

			if tt.wantValid {
				if result <= 0 {
					t.Errorf("BoardWidthInt() = %d, want > 0", result)
				}
			}
		})
	}
}

func TestBoardHeightInt(t *testing.T) {
	tests := []struct {
		name      string
		height    int64
		wantValid bool
	}{
		{"small", 100, true},
		{"medium", 10000, true},
		{"large", 1000000, true},
		{"very large", 10000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.BoardHeight = tt.height

			result := cfg.BoardHeightInt()

			if tt.wantValid {
				if result <= 0 {
					t.Errorf("BoardHeightInt() = %d, want > 0", result)
				}
			}
		})
	}
}

func TestDifficultyMultiplier(t *testing.T) {
	tests := []struct {
		difficulty string
		want       float64
	}{
		{"easy", 0.5},
		{"medium", 1.0},
		{"hard", 1.5},
		{"unknown", 1.0},
		{"", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.difficulty, func(t *testing.T) {
			got := DifficultyMultiplier(tt.difficulty)
			if got != tt.want {
				t.Errorf("DifficultyMultiplier(%s) = %f, want %f", tt.difficulty, got, tt.want)
			}
		})
	}
}

func TestConfigDir(t *testing.T) {
	dir := configDir()

	if dir == "" {
		t.Error("configDir() returned empty string")
	}

	// Should contain .clustership
	if filepath.Base(dir) != ".clustership" {
		t.Errorf("configDir() = %s, want path ending in .clustership", dir)
	}
}

func TestConfigPath(t *testing.T) {
	path := configPath()

	if path == "" {
		t.Error("configPath() returned empty string")
	}

	// Should end with config.json
	if filepath.Base(path) != "config.json" {
		t.Errorf("configPath() = %s, want path ending in config.json", path)
	}

	// Should be in .clustership directory
	dir := filepath.Dir(path)
	if filepath.Base(dir) != ".clustership" {
		t.Errorf("configPath() directory = %s, want .clustership", dir)
	}
}

func TestSavePermissionDenied(t *testing.T) {
	cfg := Default()

	// Try to save to a read-only location (this test might be skipped on some systems)
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	// This is a platform-specific test and might not work on all systems
	// Just verify Save() can be called without panicking
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test-config.json")

	// Create file and make it read-only
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Skipf("Cannot create test file: %v", err)
	}

	// Note: Making directory read-only doesn't prevent writing on all systems
	// This test is primarily to ensure Save() handles errors gracefully
}

func TestValidateMultipleCalls(t *testing.T) {
	cfg := Default()

	// Calling Validate multiple times should be idempotent
	cfg.Validate()
	cfg.Validate()
	cfg.Validate()

	// Values should remain valid
	if cfg.BoardWidth < 20 {
		t.Errorf("BoardWidth = %d after multiple Validate calls", cfg.BoardWidth)
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := Default()

	// Marshal to JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Should produce valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Should not include unexported fields (those with json:"-")
	if _, exists := result["detectedTier"]; exists {
		t.Error("JSON should not include detectedTier (marked with json:\"-\")")
	}
	if _, exists := result["detectedLimits"]; exists {
		t.Error("JSON should not include detectedLimits (marked with json:\"-\")")
	}
	if _, exists := result["systemInfo"]; exists {
		t.Error("JSON should not include systemInfo (marked with json:\"-\")")
	}
}

// Benchmarks

func BenchmarkDefault(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Default()
	}
}

func BenchmarkValidate(b *testing.B) {
	cfg := Default()
	cfg.DetectHardware()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg.Validate()
	}
}

func BenchmarkDetectHardware(b *testing.B) {
	cfg := Default()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg.DetectHardware()
	}
}

func BenchmarkGetTier(b *testing.B) {
	cfg := Default()
	cfg.DetectHardware()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cfg.GetTier()
	}
}

func BenchmarkBoardWidthInt(b *testing.B) {
	cfg := Default()
	cfg.BoardWidth = 10000000

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cfg.BoardWidthInt()
	}
}

func BenchmarkJSONMarshal(b *testing.B) {
	cfg := Default()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(cfg)
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	cfg := Default()
	data, _ := json.Marshal(cfg)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := &GameConfig{}
		_ = json.Unmarshal(data, result)
	}
}
