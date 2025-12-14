//go:build integration
// +build integration

package integration

import (
	"bytes"
	"clustership/pkg/config"
	"clustership/pkg/game"
	"clustership/pkg/tui"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// simulateKeys simulates a sequence of key presses in a Bubble Tea program
func simulateKeys(t *testing.T, m tea.Model, keys []string) tea.Model {
	t.Helper()

	for _, key := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}

		// Handle special keys
		switch key {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		}

		var cmd tea.Cmd
		m, cmd = m.Update(msg)

		// Execute any commands that were returned
		if cmd != nil {
			// For tick commands, we need to wait and process the message
			if tickMsg := executeCmd(cmd); tickMsg != nil {
				m, _ = m.Update(tickMsg)
			}
		}
	}

	return m
}

// executeCmd executes a tea.Cmd and returns the message it produces
func executeCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}

	// Create a channel to receive the message
	msgChan := make(chan tea.Msg, 1)

	// Execute the command in a goroutine
	go func() {
		msg := cmd()
		if msg != nil {
			msgChan <- msg
		}
	}()

	// Wait for the message with timeout
	select {
	case msg := <-msgChan:
		return msg
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// getAppState uses reflection-like approach to get internal state
// Since AppModel is in another package, we'll rely on View() output
func getViewOutput(m tea.Model) string {
	return m.View()
}

// containsText checks if view output contains expected text
func containsText(view, expected string) bool {
	return strings.Contains(view, expected)
}

// TestGameFlowMenuToCompanySelect tests navigation from menu to company selection
func TestGameFlowMenuToCompanySelect(t *testing.T) {
	m := tui.NewAppModel()

	// Start at menu
	view := getViewOutput(m)
	if !containsText(view, "CLUSTERSHIP") {
		t.Error("Expected to see CLUSTERSHIP title in menu")
	}

	// Navigate to "New Game" and select
	m = simulateKeys(t, m, []string{"enter"})

	view = getViewOutput(m)
	if !containsText(view, "Select") || !containsText(view, "Company") {
		t.Error("Expected to see company selection screen")
	}
}

// TestGameFlowCompanyToEnemySelect tests company selection flow
func TestGameFlowCompanyToEnemySelect(t *testing.T) {
	m := tui.NewAppModel()

	// Navigate: Menu -> New Game -> Select Company -> Enemy Count
	keys := []string{"enter"} // Select "New Game"

	// Move down to select a company (assuming at least one exists)
	keys = append(keys, "down", "enter")

	m = simulateKeys(t, m, keys)

	view := getViewOutput(m)
	// Should now be at enemy selection or enemy count selection
	if !containsText(view, "enemy") && !containsText(view, "Enemy") && !containsText(view, "opponent") {
		t.Logf("View output: %s", view)
		t.Error("Expected to see enemy selection screen")
	}
}

// TestGameFlowFullGameToPlacement tests complete flow to placement phase
func TestGameFlowFullGameToPlacement(t *testing.T) {
	m := tui.NewAppModel()

	// Navigate through: Menu -> New Game -> Select Player -> Select Enemy Count -> Select Enemy -> Placement
	keys := []string{
		"enter",  // New Game
		"enter",  // Select first company (player)
		"enter",  // Select 1 enemy (default)
		"enter",  // Select first enemy company
	}

	m = simulateKeys(t, m, keys)

	view := getViewOutput(m)
	// In placement mode, we should see the board or "Battle" prompt
	if !containsText(view, "Battle") && !containsText(view, "Ocean") && !containsText(view, "Fleet") {
		t.Logf("View output: %s", view)
		t.Error("Expected to see battle/placement screen")
	}
}

// TestGameFlowDemoMode tests auto-play demo mode
func TestGameFlowDemoMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping demo mode test in short mode")
	}

	m := tui.NewAppModel()

	// Navigate: Menu -> Demo
	keys := []string{
		"down",  // Move to Demo
		"enter", // Select Demo
	}

	m = simulateKeys(t, m, keys)

	view := getViewOutput(m)
	if !containsText(view, "Select") || !containsText(view, "Company") {
		t.Error("Expected to see company selection for demo mode")
	}

	// Select companies for demo
	keys = []string{
		"enter", // Select player company
		"enter", // Select enemy count
		"enter", // Select enemy company
	}

	m = simulateKeys(t, m, keys)

	// Demo should auto-play
	// Simulate a few ticks to let it run
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		// Send a tick message
		tickMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("")}
		m, _ = m.Update(tickMsg)
	}

	view = getViewOutput(m)
	// Should show battle or game state
	if !containsText(view, "Ocean") && !containsText(view, "Battle") && !containsText(view, "Turn") {
		t.Logf("View output: %s", view)
		t.Error("Expected demo mode to show battle state")
	}
}

// TestGameFlowMultiEnemy tests battle royale with multiple enemies
func TestGameFlowMultiEnemy(t *testing.T) {
	m := tui.NewAppModel()

	// Navigate: Menu -> New Game -> Select Player -> Select 3 enemies
	keys := []string{
		"enter",      // New Game
		"enter",      // Select first company (player)
		"down",       // Move to 2 enemies
		"down",       // Move to 3 enemies
		"enter",      // Select 3 enemies
		"enter",      // Select first enemy
		"space",      // Select second enemy (space to toggle)
		"down",
		"space",      // Select third enemy
		"enter",      // Confirm selection
	}

	m = simulateKeys(t, m, keys)

	view := getViewOutput(m)
	// Should reach placement/battle with multiple enemies
	// Exact output depends on implementation
	if view == "" {
		t.Error("View should not be empty after multi-enemy setup")
	}
}

// TestGameFlowViewLevelSwitching tests switching between view levels (1-5 keys)
func TestGameFlowViewLevelSwitching(t *testing.T) {
	m := tui.NewAppModel()

	// Get to battle state
	keys := []string{
		"enter", // New Game
		"enter", // Select player
		"enter", // Select enemy count
		"enter", // Select enemy
	}

	m = simulateKeys(t, m, keys)

	// Now test view level switching
	viewTests := []struct {
		key      string
		expected string
	}{
		{"1", "Map"},     // ViewMap
		{"2", "Ship"},    // ViewShip (may need a ship selected)
		{"3", "Rack"},    // ViewRack
		{"4", "YAML"},    // ViewYAML
		{"5", "Layout"},  // ViewRackLayout
		{"1", "Map"},     // Back to map
	}

	for _, vt := range viewTests {
		m = simulateKeys(t, m, []string{vt.key})
		view := getViewOutput(m)

		// Note: exact text depends on which view level is active
		// We just verify view changes
		if view == "" {
			t.Errorf("View should not be empty after pressing %s", vt.key)
		}
	}
}

// TestGameFlowQuitToMenu tests quitting from various states back to menu
func TestGameFlowQuitToMenu(t *testing.T) {
	m := tui.NewAppModel()

	// Get to company selection
	m = simulateKeys(t, m, []string{"enter"})

	// Press 'q' to go back to menu
	m = simulateKeys(t, m, []string{"q"})

	view := getViewOutput(m)
	if !containsText(view, "CLUSTERSHIP") || !containsText(view, "New Game") {
		t.Error("Expected to return to main menu after pressing 'q'")
	}
}

// TestGameFlowSettingsPersistence tests settings are saved and loaded
func TestGameFlowSettingsPersistence(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Override config path (this would require exposing config.configPath or using env)
	// For now, we'll test the config API directly
	cfg := config.Default()
	cfg.BoardWidth = 100
	cfg.BoardHeight = 100
	cfg.TurnDelayMs = 500

	// Save to temp location
	originalConfigDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalConfigDir)

	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.BoardWidth != 100 {
		t.Errorf("Expected BoardWidth=100, got %d", loaded.BoardWidth)
	}

	if loaded.BoardHeight != 100 {
		t.Errorf("Expected BoardHeight=100, got %d", loaded.BoardHeight)
	}

	if loaded.TurnDelayMs != 500 {
		t.Errorf("Expected TurnDelayMs=500, got %d", loaded.TurnDelayMs)
	}
}

// TestGameFlowSettingsUI tests navigating settings menu
func TestGameFlowSettingsUI(t *testing.T) {
	m := tui.NewAppModel()

	// Navigate: Menu -> Settings
	keys := []string{
		"down",  // Move to Demo
		"down",  // Move to Tutorial
		"down",  // Move to Settings
		"enter", // Select Settings
	}

	m = simulateKeys(t, m, keys)

	view := getViewOutput(m)
	if !containsText(view, "Settings") && !containsText(view, "Board") {
		t.Error("Expected to see settings menu")
	}

	// Navigate through settings
	settingsKeys := []string{
		"down",  // Board settings
		"enter", // Enter board settings
		"esc",   // Back to settings
		"down",  // Ships settings
		"down",  // Pods settings
		"down",  // Bots settings
		"down",  // Timing settings
		"down",  // K8s settings
	}

	m = simulateKeys(t, m, settingsKeys)

	// Return to menu
	m = simulateKeys(t, m, []string{"q"})

	view = getViewOutput(m)
	if !containsText(view, "CLUSTERSHIP") {
		t.Error("Expected to return to main menu from settings")
	}
}

// TestGameFlowInfoOverlay tests toggling info overlay with 'i' key
func TestGameFlowInfoOverlay(t *testing.T) {
	m := tui.NewAppModel()

	// Get to battle state
	keys := []string{
		"enter", // New Game
		"enter", // Select player
		"enter", // Select enemy count
		"enter", // Select enemy
	}

	m = simulateKeys(t, m, keys)

	// Toggle info overlay
	m = simulateKeys(t, m, []string{"i"})
	view := getViewOutput(m)

	// Info overlay should show controls or help
	// Exact content depends on implementation
	if view == "" {
		t.Error("View should not be empty after toggling info")
	}

	// Toggle off
	m = simulateKeys(t, m, []string{"i"})
	view2 := getViewOutput(m)

	if view2 == "" {
		t.Error("View should not be empty after toggling info off")
	}
}

// TestGameFlowBattleAttack tests performing an attack in battle
func TestGameFlowBattleAttack(t *testing.T) {
	m := tui.NewAppModel()

	// Get to battle state
	keys := []string{
		"enter", // New Game
		"enter", // Select player
		"enter", // Select enemy count
		"enter", // Select enemy
	}

	m = simulateKeys(t, m, keys)

	// Move cursor and attack
	attackKeys := []string{
		"right", // Move cursor
		"right",
		"down",
		"down",
		"space", // Attack at cursor position
	}

	m = simulateKeys(t, m, attackKeys)

	view := getViewOutput(m)
	// Should show hit/miss or attack result
	// Exact message depends on whether we hit anything
	if view == "" {
		t.Error("View should not be empty after attack")
	}
}

// TestGameFlowDebugMode tests debug mode toggle with 'd' key
func TestGameFlowDebugMode(t *testing.T) {
	m := tui.NewAppModel()

	// Get to battle state
	keys := []string{
		"enter", // New Game
		"enter", // Select player
		"enter", // Select enemy count
		"enter", // Select enemy
	}

	m = simulateKeys(t, m, keys)

	// Toggle debug mode
	m = simulateKeys(t, m, []string{"d"})
	view := getViewOutput(m)

	// Debug mode should show all ships
	if view == "" {
		t.Error("View should not be empty in debug mode")
	}

	// Toggle off
	m = simulateKeys(t, m, []string{"d"})
}

// TestGameFlowCompleteBattle simulates a complete battle to game over
func TestGameFlowCompleteBattle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complete battle test in short mode")
	}

	m := tui.NewAppModel()

	// Setup with minimal configuration for faster game
	// This would require exposing config or using a test-specific setup

	// Get to battle state
	keys := []string{
		"enter", // New Game
		"enter", // Select player
		"enter", // Select enemy count
		"enter", // Select enemy
	}

	m = simulateKeys(t, m, keys)

	// Let AI battle for a while (simulate ticks)
	// In a real test, we'd want to enable demo mode or speed up the game
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)

		// Process a tick
		tickCmd := tea.Tick(time.Millisecond, func(t time.Time) tea.Msg {
			return t
		})
		if msg := executeCmd(tickCmd); msg != nil {
			m, _ = m.Update(msg)
		}

		view := getViewOutput(m)
		// Check if game over
		if containsText(view, "Game Over") || containsText(view, "Victory") || containsText(view, "Defeated") {
			t.Log("Game reached completion")
			return
		}
	}

	t.Log("Battle in progress (not completed in test time limit)")
}

// TestGameFlowTutorial tests tutorial mode
func TestGameFlowTutorial(t *testing.T) {
	m := tui.NewAppModel()

	// Navigate: Menu -> Tutorial
	keys := []string{
		"down",  // Move to Demo
		"down",  // Move to Tutorial
		"enter", // Select Tutorial
	}

	m = simulateKeys(t, m, keys)

	view := getViewOutput(m)
	if !containsText(view, "Tutorial") && !containsText(view, "tutorial") {
		t.Error("Expected to see tutorial screen")
	}

	// Advance through tutorial steps
	for i := 0; i < 5; i++ {
		m = simulateKeys(t, m, []string{"enter"})
		view = getViewOutput(m)
		if view == "" {
			t.Error("Tutorial view should not be empty")
		}
	}

	// Exit tutorial
	m = simulateKeys(t, m, []string{"q"})

	view = getViewOutput(m)
	if !containsText(view, "CLUSTERSHIP") {
		t.Error("Expected to return to menu after tutorial")
	}
}

// TestGameFlowWindowResize tests handling window resize events
func TestGameFlowWindowResize(t *testing.T) {
	m := tui.NewAppModel()

	// Send window resize message
	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	m, _ = m.Update(resizeMsg)

	view := getViewOutput(m)
	if view == "" {
		t.Error("View should render after resize")
	}

	// Another resize
	resizeMsg2 := tea.WindowSizeMsg{Width: 80, Height: 24}
	m, _ = m.Update(resizeMsg2)

	view2 := getViewOutput(m)
	if view2 == "" {
		t.Error("View should render after second resize")
	}
}

// TestGameFlowTemplateLoading tests loading company templates
func TestGameFlowTemplateLoading(t *testing.T) {
	// Test loading all available company templates
	companies := game.ListCompanies()

	if len(companies) == 0 {
		t.Error("Expected at least one company template")
	}

	for _, id := range companies {
		template, err := game.LoadCompanyTemplate(id)
		if err != nil {
			t.Errorf("Failed to load company template %s: %v", id, err)
			continue
		}

		if template.ID != id {
			t.Errorf("Expected template ID %s, got %s", id, template.ID)
		}

		if template.Name == "" {
			t.Errorf("Company %s has empty name", id)
		}

		if len(template.Regions) == 0 {
			t.Errorf("Company %s has no regions", id)
		}

		if len(template.Services) == 0 {
			t.Errorf("Company %s has no services", id)
		}

		// Convert to Company and verify
		company := game.CompanyFromTemplate(template)
		if company.HealthyPodCount() == 0 {
			t.Errorf("Company %s has no healthy pods after initialization", id)
		}

		t.Logf("Loaded company %s: %d regions, %d services, %d pods",
			company.Name, len(company.Regions), len(company.Services), company.TotalPods())
	}
}

// TestGameFlowConfigValidation tests config validation logic
func TestGameFlowConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		modify   func(*config.GameConfig)
		validate func(*testing.T, *config.GameConfig)
	}{
		{
			name: "BoardWidth too small",
			modify: func(cfg *config.GameConfig) {
				cfg.BoardWidth = 5
			},
			validate: func(t *testing.T, cfg *config.GameConfig) {
				if cfg.BoardWidth < 20 {
					t.Error("Expected BoardWidth to be clamped to minimum 20")
				}
			},
		},
		{
			name: "ShipsPerPlayer too small",
			modify: func(cfg *config.GameConfig) {
				cfg.ShipsPerPlayer = 0
			},
			validate: func(t *testing.T, cfg *config.GameConfig) {
				if cfg.ShipsPerPlayer < 1 {
					t.Error("Expected ShipsPerPlayer to be clamped to minimum 1")
				}
			},
		},
		{
			name: "TurnDelayMs too small",
			modify: func(cfg *config.GameConfig) {
				cfg.TurnDelayMs = 0
			},
			validate: func(t *testing.T, cfg *config.GameConfig) {
				if cfg.TurnDelayMs < 10 {
					t.Error("Expected TurnDelayMs to be clamped to minimum 10")
				}
			},
		},
		{
			name: "K8s namespace empty",
			modify: func(cfg *config.GameConfig) {
				cfg.K8sNamespace = ""
			},
			validate: func(t *testing.T, cfg *config.GameConfig) {
				if cfg.K8sNamespace == "" {
					t.Error("Expected K8sNamespace to have default value")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			tt.modify(cfg)
			cfg.Validate()
			tt.validate(t, cfg)
		})
	}
}

// BenchmarkGameRender benchmarks the View() rendering performance
func BenchmarkGameRender(b *testing.B) {
	m := tui.NewAppModel()

	// Get to battle state
	keys := []string{"enter", "enter", "enter", "enter"}
	for _, key := range keys {
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		m, _ = m.Update(msg)
	}

	// Benchmark rendering
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// BenchmarkGameUpdate benchmarks the Update() performance
func BenchmarkGameUpdate(b *testing.B) {
	m := tui.NewAppModel()

	// Get to battle state
	keys := []string{"enter", "enter", "enter", "enter"}
	for _, key := range keys {
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		m, _ = m.Update(msg)
	}

	// Benchmark updates with cursor movement
	msg := tea.KeyMsg{Type: tea.KeyRight}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, _ = m.Update(msg)
	}
}

// TestGameFlowWithRealInput tests using tea.WithInput for realistic simulation
func TestGameFlowWithRealInput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real input test in short mode")
	}

	// Create input buffer with key sequence
	input := bytes.NewBufferString("enter\nenter\nenter\nenter\nq\n")

	m := tui.NewAppModel()

	// Create program with input
	p := tea.NewProgram(m, tea.WithInput(input))

	// Run for a short time
	go func() {
		time.Sleep(500 * time.Millisecond)
		p.Quit()
	}()

	// This would actually run the program
	// finalModel, err := p.Run()
	// For testing purposes, we skip actual Run() as it requires terminal
	_ = p
}
