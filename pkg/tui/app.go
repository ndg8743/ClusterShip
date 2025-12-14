package tui

import (
	"clustership/pkg/config"
	"clustership/pkg/game"
	"clustership/pkg/k8s"
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tickMsg is sent to animate turns
type tickMsg time.Time

// doTick returns a command that sends a tick after a delay
func doTick() tea.Cmd {
	return doTickWithDelay(200)
}

// doTickWithDelay returns a tick command with custom delay in milliseconds
func doTickWithDelay(ms int) tea.Cmd {
	return tea.Tick(time.Duration(ms)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// tick returns a tick command using the configured delay
func (m *AppModel) tick() tea.Cmd {
	delay := m.cfg.TurnDelayMs
	if delay <= 0 {
		delay = 200 // default
	}
	return doTickWithDelay(delay)
}

// k8sPollMsg triggers K8s event polling
type k8sPollMsg struct{}

// doK8sPoll returns a command that polls K8s events every second
func doK8sPoll() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return k8sPollMsg{}
	})
}

// GameState represents the current phase of the game
type GameState int

const (
	StateMenu GameState = iota
	StateCompanySelect
	StateEnemyCountSelect // select how many enemies (1-5)
	StateEnemySelect      // select which enemy companies
	StatePlacement
	StateBattle
	StatePodView // view pod details for selected service
	StateGameOver
	// settings states
	StateSettings       // main settings menu
	StateSettingsBoard  // board width/height
	StateSettingsShips  // ships, racks config
	StateSettingsPods   // pods per rack
	StateSettingsBots   // bot count, difficulty
	StateSettingsTiming // turn delay, animation speed
	StateSettingsK8s    // k8s integration options
)

// ViewLevel represents the hierarchical view depth (1-5 keys)
type ViewLevel int

const (
	ViewMap        ViewLevel = 1 // overview map
	ViewShip       ViewLevel = 2 // ship/region detail
	ViewRack       ViewLevel = 3 // rack detail with pods
	ViewYAML       ViewLevel = 4 // yaml manifest
	ViewRackLayout ViewLevel = 5 // visual rack grid with pod distribution
)

// AppModel is the main Bubble Tea model for the game
type AppModel struct {
	state    GameState
	styles   *Styles
	width    int
	height   int

	// config
	cfg *config.GameConfig

	// menu state
	menuCursor int
	menuItems  []string

	// settings state
	settingsCursor int // cursor for settings menu/items
	settingsItems  []string

	// company selection
	companies      []string
	companyCursor  int
	playerCompany  *game.Company
	enemyCompany   *game.Company // legacy single enemy (backward compat)

	// Multi-enemy support
	enemyCompanies    []*game.Company      // slice of enemy companies
	selectedEnemies   []string             // IDs of selected enemies
	enemyCountCursor  int                  // for selecting number of enemies
	enemySelectCursor int                  // cursor for enemy selection
	maxEnemies        int                  // max enemies available

	// game state
	board    *Board
	ai       *AIPlayer            // legacy single enemy AI
	ais      map[string]*AIPlayer // one AI per enemy company
	playerAI *AIPlayer            // player AI for demo mode
	cursor   [2]int               // current cursor position on board
	viewport [2]int               // viewport offset for scrolling large boards
	turn     int

	// Turn queue system
	turnQueue        []string // company IDs in turn order
	currentTurnIndex int      // index into turnQueue
	currentAttacker  string   // company ID currently attacking

	isPlayerTurn bool
	gameOver     bool
	winner       string
	lastMessage  string   // last attack result message
	battleLog    []string // recent battle messages

	// animation state
	animating       bool     // whether turn is animating
	pendingAttacks  [][2]int // attacks to animate
	pendingTargetID string   // target company ID for pending attacks

	// display
	compactMode      bool
	viewW            int // viewport width in cells
	viewH            int // viewport height in cells
	demoMode         bool
	debugMode        bool // show all ships (no fog of war)
	serviceViewIndex int  // which company's services to display (0=player, 1+=enemies)

	// view hierarchy (1-4 keys)
	viewLevel        ViewLevel     // current view depth
	selectedShipID   string        // ship selected for drill-down
	selectedRackID   string        // rack selected for drill-down
	selectedSvcID    string        // service selected for yaml view
	selectedShip     *game.Region  // cached selected ship
	selectedRack     *game.Rack    // cached selected rack
	selectedService  *game.Service // cached selected service

	// pod view state (legacy, kept for backward compat)
	podViewCompany  *game.Company // company whose services are being viewed
	podViewSvcIndex int           // selected service index

	// K8s integration fields
	k8sClient    *k8s.Client                       // K8s cluster client
	k8sWatcher   *k8s.PodWatcher                   // watches for pod events
	k8sManifests map[string][]*k8s.ServiceManifest // companyID -> manifests
	k8sDeployed  bool                              // whether K8s resources are deployed
	k8sError     error                             // any K8s connection error
	k8sPodEvents []k8s.PodEvent                    // recent K8s events for display
}

// NewAppModel creates a fresh game instance
func NewAppModel() AppModel {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Default()
	}
	cfg.Validate()

	return AppModel{
		state:         StateMenu,
		styles:        DefaultStyles(),
		menuItems:     []string{"New Game", "Demo", "Settings", "Quit"},
		settingsItems: []string{"Board", "Ships", "Pods", "Bots", "Timing", "Kubernetes", "Save & Back"},
		companies:     game.ListCompanies(),
		cfg:           cfg,
		viewW:         30, // viewport size
		viewH:         20,
		viewLevel:     ViewMap, // start at map view
		battleLog:     make([]string, 0),
		// Explicitly set game state to prevent carryover
		demoMode:     false,
		debugMode:    false,
		isPlayerTurn: false,
		animating:    false,
		gameOver:     false,
		playerAI:     nil,
		ai:           nil,
		ais:          nil,
		board:        nil,
	}
}

// ============================================================================
// K8s Integration Methods
// ============================================================================

// initK8sClient initializes the K8s client if real K8s mode is enabled
func (m *AppModel) initK8sClient() error {
	if !m.cfg.EnableRealK8s || m.k8sClient != nil {
		return nil
	}

	client, err := k8s.NewClient(m.cfg.Kubeconfig, m.cfg.K8sNamespace)
	if err != nil {
		m.k8sError = fmt.Errorf("failed to create K8s client: %w", err)
		return m.k8sError
	}

	if !client.IsClusterAvailable() {
		m.k8sError = fmt.Errorf("K8s cluster not available")
		return m.k8sError
	}

	m.k8sClient = client
	m.k8sError = nil
	return nil
}

// deployK8sResources deploys K8s manifests for all companies in the game
func (m *AppModel) deployK8sResources() error {
	if !m.cfg.EnableRealK8s || m.k8sClient == nil {
		return nil
	}

	ctx := context.Background()

	// Ensure namespace exists
	if err := m.k8sClient.EnsureNamespace(ctx, m.cfg.K8sNamespace); err != nil {
		m.addBattleLog(fmt.Sprintf("K8s: Failed to create namespace: %v", err))
		return err
	}

	templatesDir := k8s.GetTemplatesDir()
	m.k8sManifests = make(map[string][]*k8s.ServiceManifest)

	// Deploy player company manifests
	if manifests, err := k8s.LoadCompanyManifests(templatesDir, m.playerCompany.ID); err == nil && len(manifests) > 0 {
		m.k8sManifests[m.playerCompany.ID] = manifests
		for _, manifest := range manifests {
			if err := m.k8sClient.DeployManifest(ctx, manifest); err != nil {
				m.addBattleLog(fmt.Sprintf("K8s: Deploy failed: %s - %v", manifest.Name, err))
			} else {
				m.addBattleLog(fmt.Sprintf("K8s: Deployed %s", manifest.Name))
			}
		}
	}

	// Deploy enemy manifests
	for _, enemy := range m.enemyCompanies {
		if manifests, err := k8s.LoadCompanyManifests(templatesDir, enemy.ID); err == nil && len(manifests) > 0 {
			m.k8sManifests[enemy.ID] = manifests
			for _, manifest := range manifests {
				if err := m.k8sClient.DeployManifest(ctx, manifest); err != nil {
					m.addBattleLog(fmt.Sprintf("K8s: Deploy failed: %s - %v", manifest.Name, err))
				} else {
					m.addBattleLog(fmt.Sprintf("K8s: Deployed %s", manifest.Name))
				}
			}
		}
	}

	m.k8sDeployed = true
	m.addBattleLog("K8s: All resources deployed")
	return nil
}

// startK8sWatcher begins watching K8s pod events
func (m *AppModel) startK8sWatcher() error {
	if !m.cfg.EnableRealK8s || m.k8sClient == nil {
		return nil
	}

	m.k8sWatcher = k8s.NewPodWatcher(m.k8sClient)
	m.k8sPodEvents = make([]k8s.PodEvent, 0, 20)

	return m.k8sWatcher.Start()
}

// pollK8sEvents drains events from the watcher channel
func (m *AppModel) pollK8sEvents() {
	if m.k8sWatcher == nil {
		return
	}

	// Drain all available events
	for {
		select {
		case event, ok := <-m.k8sWatcher.Events():
			if !ok {
				return
			}
			m.k8sPodEvents = append(m.k8sPodEvents, event)
			if len(m.k8sPodEvents) > 20 {
				m.k8sPodEvents = m.k8sPodEvents[1:]
			}

			// Sync deleted/terminating pods to game state
			if event.Type == "Deleted" || event.Pod.Status == k8s.PodTerminating {
				m.syncK8sPodDeletion(event.Pod)
			}
		default:
			return
		}
	}
}

// syncK8sPodDeletion updates game state when a real K8s pod is deleted
func (m *AppModel) syncK8sPodDeletion(podInfo k8s.PodInfo) {
	if m.board == nil {
		return
	}

	// Find the fleet for this company
	fleet, ok := m.board.Fleets[podInfo.Company]
	if !ok || fleet == nil {
		return
	}

	// Find and update the matching game pod
	for _, svc := range fleet.Company.Services {
		if svc.ID != podInfo.ServiceID {
			continue
		}
		for _, pod := range svc.Pods {
			if pod.Status == game.PodRunning {
				// Mark first matching running pod as terminated
				pod.Status = game.PodTerminated
				pod.Health = 0
				m.addBattleLog(fmt.Sprintf("K8s: Pod %s terminated externally", podInfo.Name))
				return
			}
		}
	}
}

// performAttackWithK8sSync executes game attack and syncs to K8s
func (m *AppModel) performAttackWithK8sSync(x, y int, attackerID, targetID string) (*ShotResult, []game.GameEvent) {
	result, events := m.board.AttackMulti(x, y, attackerID, targetID)

	// If K8s enabled and we killed a pod, delete the real K8s pod
	if m.cfg.EnableRealK8s && m.k8sClient != nil && result != nil && result.KilledPod && result.HitPod != nil {
		m.deleteK8sPodForGamePod(result.HitPod, targetID)
	}

	return result, events
}

// deleteK8sPodForGamePod deletes the corresponding K8s pod when game pod is killed
func (m *AppModel) deleteK8sPodForGamePod(gamePod *game.Pod, companyID string) {
	if gamePod == nil || m.k8sClient == nil {
		return
	}

	// Find the company's manifests
	manifests := m.k8sManifests[companyID]
	if manifests == nil {
		return
	}

	// Find manifest matching this service and scale down or delete
	ctx := context.Background()
	for _, manifest := range manifests {
		if manifest.ServiceID == gamePod.ServiceID {
			// Use DeleteService to remove pods for this service
			if err := m.k8sClient.DeleteService(ctx, manifest.ServiceID, manifest.Company); err != nil {
				m.addBattleLog(fmt.Sprintf("K8s: Delete failed: %s", manifest.ServiceID))
			} else {
				m.addBattleLog(fmt.Sprintf("K8s: Deleted %s service pods", manifest.ServiceID))
			}
			return
		}
	}
}

// cleanupK8sResources stops watcher and deletes all K8s resources
func (m *AppModel) cleanupK8sResources() {
	if m.k8sWatcher != nil {
		m.k8sWatcher.Stop()
		m.k8sWatcher = nil
	}

	if m.k8sClient != nil && m.k8sDeployed {
		ctx := context.Background()
		if err := m.k8sClient.CleanupNamespace(ctx, m.cfg.K8sNamespace); err != nil {
			// Log error but don't fail
			m.addBattleLog(fmt.Sprintf("K8s: Cleanup error: %v", err))
		} else {
			m.addBattleLog("K8s: Resources cleaned up")
		}
		m.k8sDeployed = false
	}

	m.k8sManifests = nil
	m.k8sPodEvents = nil
}

// renderK8sStatus returns a string showing K8s integration status
func (m *AppModel) renderK8sStatus() string {
	if !m.cfg.EnableRealK8s {
		return m.styles.Muted.Render("K8s: Disabled")
	}

	if m.k8sError != nil {
		return m.styles.Error.Render(fmt.Sprintf("K8s: Error - %v", m.k8sError))
	}

	if !m.k8sDeployed {
		return m.styles.Warning.Render("K8s: Not deployed")
	}

	// Get real pod status
	if m.k8sClient != nil {
		ctx := context.Background()
		pods, err := m.k8sClient.GetPodStatus(ctx, m.cfg.K8sNamespace, "app=clustership")
		if err == nil {
			running, pending := 0, 0
			for _, p := range pods {
				if p.Status == k8s.PodRunning {
					running++
				} else if p.Status == k8s.PodPending {
					pending++
				}
			}
			return m.styles.Success.Render(fmt.Sprintf("K8s: %d running, %d pending", running, pending))
		}
	}

	return m.styles.Success.Render("K8s: Active")
}

// renderK8sEvents renders recent K8s pod events
func (m *AppModel) renderK8sEvents() string {
	if !m.cfg.EnableRealK8s || len(m.k8sPodEvents) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, m.styles.Title.Render("K8s EVENTS"))

	// Show last 5 events
	start := len(m.k8sPodEvents) - 5
	if start < 0 {
		start = 0
	}

	for _, ev := range m.k8sPodEvents[start:] {
		line := fmt.Sprintf("%s: %s", ev.Type, ev.Message)
		if len(line) > 40 {
			line = line[:40] + "..."
		}
		lines = append(lines, m.styles.Muted.Render(line))
	}

	return "\n" + lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// ============================================================================
// End K8s Integration Methods
// ============================================================================

// Init is the Bubble Tea init function
func (m AppModel) Init() tea.Cmd {
	return nil
}

// Update handles all input and state changes
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// adjust viewport to fit terminal
		m.viewW = min(40, msg.Width/3)
		m.viewH = min(25, msg.Height-12)
		return m, nil

	case tickMsg:
		return m.handleTick()

	case k8sPollMsg:
		// Poll K8s events during battle
		m.pollK8sEvents()
		// Continue polling if in battle and K8s is enabled
		if m.state == StateBattle && m.cfg.EnableRealK8s && m.k8sWatcher != nil {
			return m, doK8sPoll()
		}
		return m, nil

	case tea.KeyMsg:
		// global keys first
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == StateMenu {
				return m, tea.Quit
			}
			// Cleanup K8s resources when leaving battle
			if m.state == StateBattle || m.state == StateGameOver {
				m.cleanupK8sResources()
			}
			// q goes back to menu in other states
			m.state = StateMenu
			return m, nil
		}

		// state-specific handling
		switch m.state {
		case StateMenu:
			return m.updateMenu(msg)
		case StateCompanySelect:
			return m.updateCompanySelect(msg)
		case StateEnemyCountSelect:
			return m.updateEnemyCountSelect(msg)
		case StateEnemySelect:
			return m.updateEnemySelect(msg)
		case StatePlacement:
			return m.updatePlacement(msg)
		case StateBattle:
			return m.updateBattle(msg)
		case StatePodView:
			return m.updatePodView(msg)
		case StateGameOver:
			return m.updateGameOver(msg)
		case StateSettings:
			return m.updateSettings(msg)
		case StateSettingsBoard:
			return m.updateSettingsBoard(msg)
		case StateSettingsShips:
			return m.updateSettingsShips(msg)
		case StateSettingsPods:
			return m.updateSettingsPods(msg)
		case StateSettingsBots:
			return m.updateSettingsBots(msg)
		case StateSettingsTiming:
			return m.updateSettingsTiming(msg)
		case StateSettingsK8s:
			return m.updateSettingsK8s(msg)
		}
	}

	return m, nil
}

// updateMenu handles menu navigation
func (m AppModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(m.menuItems)-1 {
			m.menuCursor++
		}
	case "enter", " ":
		switch m.menuCursor {
		case 0: // New Game
			m.demoMode = false
			m.state = StateCompanySelect
			m.companyCursor = 0
		case 1: // Demo - auto plays both sides
			m.demoMode = true
			m.state = StateCompanySelect
			m.companyCursor = 0
		case 2: // Settings
			m.state = StateSettings
			m.settingsCursor = 0
		case 3: // Quit
			return m, tea.Quit
		}
	}
	return m, nil
}

// updateCompanySelect handles company selection
func (m AppModel) updateCompanySelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.companyCursor > 0 {
			m.companyCursor--
		}
	case "down", "j":
		if m.companyCursor < len(m.companies)-1 {
			m.companyCursor++
		}
	case "enter", " ":
		// load selected company as player
		template, err := game.LoadCompanyTemplate(m.companies[m.companyCursor])
		if err != nil {
			return m, nil
		}
		m.playerCompany = game.CompanyFromTemplate(template)
		m.playerCompany.ID = "player"
		// Adjust to match config settings (ships, racks, pods)
		m.playerCompany.AdjustToConfig(m.cfg.ShipsPerPlayer, m.cfg.RacksPerShip, m.cfg.PodsPerRack)

		// Move to enemy count selection
		m.state = StateEnemyCountSelect
		m.enemyCountCursor = 0
		m.maxEnemies = len(m.companies) - 1 // all except player
		if m.maxEnemies > 5 {
			m.maxEnemies = 5
		}
	case "esc":
		m.state = StateMenu
	}
	return m, nil
}

// updateEnemyCountSelect handles selecting number of enemies
func (m AppModel) updateEnemyCountSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.enemyCountCursor > 0 {
			m.enemyCountCursor--
		}
	case "down", "j":
		if m.enemyCountCursor < m.maxEnemies-1 {
			m.enemyCountCursor++
		}
	case "enter", " ":
		enemyCount := m.enemyCountCursor + 1
		m.selectedEnemies = make([]string, 0, enemyCount)
		m.enemyCompanies = make([]*game.Company, 0, enemyCount)

		if enemyCount == 1 {
			// Single enemy - auto-pick random
			enemyID := m.pickRandomEnemy(m.playerCompany.ID)
			m.selectedEnemies = append(m.selectedEnemies, enemyID)
			m.state = StatePlacement
		} else {
			// Multiple enemies - go to enemy selection screen
			m.state = StateEnemySelect
			m.enemySelectCursor = 0
		}
	case "esc":
		m.state = StateCompanySelect
	}
	return m, nil
}

// updateEnemySelect handles selecting which enemy companies to fight
func (m AppModel) updateEnemySelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	targetCount := m.enemyCountCursor + 1

	switch msg.String() {
	case "up", "k":
		if m.enemySelectCursor > 0 {
			m.enemySelectCursor--
		}
	case "down", "j":
		if m.enemySelectCursor < len(m.companies)-1 {
			m.enemySelectCursor++
		}
	case "enter", " ":
		selectedID := m.companies[m.enemySelectCursor]

		// Can't select player's company
		if selectedID == m.playerCompany.ID {
			return m, nil
		}

		// Toggle selection
		idx := -1
		for i, id := range m.selectedEnemies {
			if id == selectedID {
				idx = i
				break
			}
		}

		if idx >= 0 {
			// Deselect
			m.selectedEnemies = append(m.selectedEnemies[:idx], m.selectedEnemies[idx+1:]...)
		} else if len(m.selectedEnemies) < targetCount {
			// Select
			m.selectedEnemies = append(m.selectedEnemies, selectedID)
		}
	case "c": // Confirm selection
		if len(m.selectedEnemies) == targetCount {
			m.state = StatePlacement
		}
	case "esc":
		m.state = StateEnemyCountSelect
		m.selectedEnemies = nil
	}
	return m, nil
}

// updatePlacement handles ship placement phase (auto-place for now)
func (m AppModel) updatePlacement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		// Build list of all companies
		allCompanies := []*game.Company{m.playerCompany}
		m.enemyCompanies = make([]*game.Company, 0, len(m.selectedEnemies))

		for _, enemyID := range m.selectedEnemies {
			template, _ := game.LoadCompanyTemplate(enemyID)
			if template != nil {
				enemy := game.CompanyFromTemplate(template)
				// Adjust enemy to match config settings
				enemy.AdjustToConfig(m.cfg.ShipsPerPlayer, m.cfg.RacksPerShip, m.cfg.PodsPerRack)
				m.enemyCompanies = append(m.enemyCompanies, enemy)
				allCompanies = append(allCompanies, enemy)
			}
		}

		// Legacy single enemy (for backward compat)
		if len(m.enemyCompanies) > 0 {
			m.enemyCompany = m.enemyCompanies[0]
		}

		// Create multi-company board using config dimensions
		m.board = NewMultiBoard(m.cfg.BoardWidth, m.cfg.BoardHeight, allCompanies)

		// Build opponent list for each AI
		allIDs := make([]string, 0, len(allCompanies))
		for _, c := range allCompanies {
			allIDs = append(allIDs, c.ID)
		}

		// Create AI for each enemy using config dimensions
		m.ais = make(map[string]*AIPlayer)
		for _, enemy := range m.enemyCompanies {
			opponents := make([]string, 0)
			for _, id := range allIDs {
				if id != enemy.ID {
					opponents = append(opponents, id)
				}
			}
			m.ais[enemy.ID] = NewMultiAIPlayer(enemy.ID, enemy.AIStrategy, m.cfg.BoardWidth, m.cfg.BoardHeight, opponents)
		}

		// Legacy single AI
		if len(m.enemyCompanies) > 0 {
			m.ai = m.ais[m.enemyCompanies[0].ID]
		}

		// Build turn queue: player first, then enemies in order
		m.turnQueue = []string{m.playerCompany.ID}
		for _, enemy := range m.enemyCompanies {
			m.turnQueue = append(m.turnQueue, enemy.ID)
		}
		m.currentTurnIndex = 0

		// In demo mode, create player AI too
		if m.demoMode {
			opponents := make([]string, 0)
			for _, enemy := range m.enemyCompanies {
				opponents = append(opponents, enemy.ID)
			}
			m.playerAI = NewMultiAIPlayer("player", game.AIHunter, m.cfg.BoardWidth, m.cfg.BoardHeight, opponents)
		}

		// Start cursor in center of board
		m.cursor = [2]int{m.cfg.BoardWidth / 2, m.cfg.BoardHeight / 2}
		// Calculate viewport with bounds checking to prevent negative values
		vpX := m.cfg.BoardWidth/2 - m.viewW/2
		vpY := m.cfg.BoardHeight/2 - m.viewH/2
		if vpX < 0 {
			vpX = 0
		}
		if vpY < 0 {
			vpY = 0
		}
		m.viewport = [2]int{vpX, vpY}
		m.battleLog = make([]string, 0)

		// Initialize K8s if enabled
		if m.cfg.EnableRealK8s {
			if err := m.initK8sClient(); err != nil {
				m.addBattleLog(fmt.Sprintf("K8s: %v (continuing in simulation mode)", err))
			} else {
				// Deploy manifests
				m.deployK8sResources()
				// Start watching pods
				m.startK8sWatcher()
			}
		}

		m.state = StateBattle
		m.isPlayerTurn = true
		m.turn = 1

		// Build initial commands
		var cmds []tea.Cmd

		// Start K8s polling if enabled
		if m.cfg.EnableRealK8s && m.k8sWatcher != nil {
			cmds = append(cmds, doK8sPoll())
		}

		// In demo mode, start auto-play
		if m.demoMode {
			cmds = append(cmds, doTickWithDelay(m.cfg.TurnDelayMs))
		}

		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
	case "esc":
		if len(m.selectedEnemies) > 1 {
			m.state = StateEnemySelect
		} else {
			m.state = StateEnemyCountSelect
		}
	}
	return m, nil
}

// updateBattle handles the main battle phase
func (m AppModel) updateBattle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// these keys work even during animation
	switch msg.String() {
	case "v":
		m.debugMode = !m.debugMode
		return m, nil
	case "c":
		totalCompanies := 1 + len(m.enemyCompanies)
		m.serviceViewIndex = (m.serviceViewIndex + 1) % totalCompanies
		return m, nil
	case "p":
		// open pod detail view for current service view company
		if m.serviceViewIndex == 0 {
			m.podViewCompany = m.playerCompany
		} else if m.serviceViewIndex <= len(m.enemyCompanies) {
			m.podViewCompany = m.enemyCompanies[m.serviceViewIndex-1]
		}
		if m.podViewCompany != nil {
			m.podViewSvcIndex = 0
			m.state = StatePodView
		}
		return m, nil
	// view hierarchy keys 1-4
	case "1":
		m.viewLevel = ViewMap
		return m, nil
	case "2":
		m.viewLevel = ViewShip
		// select first ship if none selected
		if m.selectedShipID == "" && m.playerCompany != nil && len(m.playerCompany.Regions) > 0 {
			m.selectedShipID = m.playerCompany.Regions[0].ID
			m.selectedShip = m.playerCompany.Regions[0]
		}
		return m, nil
	case "3":
		m.viewLevel = ViewRack
		// select first rack if none selected
		if m.selectedRackID == "" && m.selectedShip != nil && len(m.selectedShip.Racks) > 0 {
			m.selectedRackID = m.selectedShip.Racks[0].ID
			m.selectedRack = m.selectedShip.Racks[0]
		}
		return m, nil
	case "4":
		m.viewLevel = ViewYAML
		// select first service if none selected
		if m.selectedSvcID == "" && m.playerCompany != nil && len(m.playerCompany.Services) > 0 {
			m.selectedSvcID = m.playerCompany.Services[0].ID
			m.selectedService = m.playerCompany.Services[0]
		}
		return m, nil
	case "5":
		m.viewLevel = ViewRackLayout
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}

	// ignore other input during animation
	if m.animating {
		return m, nil
	}

	// handle navigation based on view level
	switch msg.String() {
	case "up", "k":
		switch m.viewLevel {
		case ViewShip:
			m.selectPrevShip()
		case ViewRack:
			m.selectPrevRack()
		case ViewYAML:
			m.selectPrevService()
		default:
			if m.cursor[1] > 0 {
				m.cursor[1]--
			}
		}
	case "down", "j":
		switch m.viewLevel {
		case ViewShip:
			m.selectNextShip()
		case ViewRack:
			m.selectNextRack()
		case ViewYAML:
			m.selectNextService()
		default:
			if m.board != nil && m.cursor[1] < m.board.Height-1 {
				m.cursor[1]++
			}
		}
	case "left", "h":
		if m.viewLevel == ViewMap {
			if m.cursor[0] > 0 {
				m.cursor[0]--
			}
		}
	case "right", "l":
		if m.viewLevel == ViewMap {
			if m.board != nil && m.cursor[0] < m.board.Width-1 {
				m.cursor[0]++
			}
		}
	// viewport panning with WASD
	case "w":
		if m.viewport[1] > 0 {
			m.viewport[1] -= 5
			if m.viewport[1] < 0 {
				m.viewport[1] = 0
			}
		}
	case "s":
		if m.board != nil && m.viewport[1] < m.board.Height-m.viewH {
			m.viewport[1] += 5
		}
	case "a":
		if m.viewport[0] > 0 {
			m.viewport[0] -= 5
			if m.viewport[0] < 0 {
				m.viewport[0] = 0
			}
		}
	case "d":
		if m.board != nil && m.viewport[0] < m.board.Width-m.viewW {
			m.viewport[0] += 5
		}
	case "enter", " ":
		if m.isPlayerTurn && m.board != nil && !m.animating && !m.demoMode {
			// Player attacks using multi-company system (with K8s sync)
			result, _ := m.performAttackWithK8sSync(m.cursor[0], m.cursor[1], "player", "")
			if result == nil {
				// Already attacked this cell, don't advance turn
				m.lastMessage = "Already attacked here!"
				return m, nil
			}
			m.lastMessage = result.Message
			m.centerViewportOn(m.cursor[0], m.cursor[1])
			// log the attack
			player := "You"
			if result.Hit {
				m.addBattleLog(fmt.Sprintf("%s hit at (%d,%d)!", player, m.cursor[0], m.cursor[1]))
			} else {
				m.addBattleLog(fmt.Sprintf("%s missed at (%d,%d)", player, m.cursor[0], m.cursor[1]))
			}
			m.checkMultiWinCondition()
			if !m.gameOver {
				// Advance to next turn in queue
				m.advanceTurn()
				return m, doTickWithDelay(m.cfg.TurnDelayMs)
			}
		}
	case "tab":
		m.compactMode = !m.compactMode
	}
	return m, nil
}

// updateGameOver handles game over screen
func (m AppModel) updateGameOver(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		m.state = StateMenu
		m = NewAppModel() // reset game
	}
	return m, nil
}

// updatePodView handles navigation in the pod detail view
func (m AppModel) updatePodView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.podViewCompany == nil {
		m.state = StateBattle
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.podViewSvcIndex > 0 {
			m.podViewSvcIndex--
		}
	case "down", "j":
		if m.podViewSvcIndex < len(m.podViewCompany.Services)-1 {
			m.podViewSvcIndex++
		}
	case "esc", "p", "q":
		m.state = StateBattle
		return m, nil
	}
	return m, nil
}

// updateSettings handles the main settings menu
func (m AppModel) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < len(m.settingsItems)-1 {
			m.settingsCursor++
		}
	case "enter", " ":
		switch m.settingsCursor {
		case 0:
			m.state = StateSettingsBoard
		case 1:
			m.state = StateSettingsShips
		case 2:
			m.state = StateSettingsPods
		case 3:
			m.state = StateSettingsBots
		case 4:
			m.state = StateSettingsTiming
		case 5:
			m.state = StateSettingsK8s
		case 6: // save and back
			m.cfg.Save()
			m.state = StateMenu
		}
	case "esc":
		m.state = StateMenu
	}
	return m, nil
}

// updateSettingsBoard handles board size config
func (m AppModel) updateSettingsBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cfg.BoardHeight += 5
		m.cfg.Validate()
	case "down", "j":
		m.cfg.BoardHeight -= 5
		m.cfg.Validate()
	case "right", "l":
		m.cfg.BoardWidth += 5
		m.cfg.Validate()
	case "left", "h":
		m.cfg.BoardWidth -= 5
		m.cfg.Validate()
	case "esc", "enter":
		m.state = StateSettings
	}
	return m, nil
}

// updateSettingsShips handles ships/racks config
func (m AppModel) updateSettingsShips(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cfg.ShipsPerPlayer++
		m.cfg.Validate()
	case "down", "j":
		m.cfg.ShipsPerPlayer--
		m.cfg.Validate()
	case "right", "l":
		m.cfg.RacksPerShip++
		m.cfg.Validate()
	case "left", "h":
		m.cfg.RacksPerShip--
		m.cfg.Validate()
	case "esc", "enter":
		m.state = StateSettings
	}
	return m, nil
}

// updateSettingsPods handles pods config
func (m AppModel) updateSettingsPods(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "right", "l":
		m.cfg.PodsPerRack++
		m.cfg.Validate()
	case "down", "j", "left", "h":
		m.cfg.PodsPerRack--
		m.cfg.Validate()
	case "esc", "enter":
		m.state = StateSettings
	}
	return m, nil
}

// updateSettingsBots handles bot config
func (m AppModel) updateSettingsBots(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cfg.MaxBots++
		m.cfg.Validate()
	case "down", "j":
		m.cfg.MaxBots--
		m.cfg.Validate()
	case "right", "l":
		// cycle difficulty
		switch m.cfg.BotDifficulty {
		case "easy":
			m.cfg.BotDifficulty = "medium"
		case "medium":
			m.cfg.BotDifficulty = "hard"
		case "hard":
			m.cfg.BotDifficulty = "easy"
		}
	case "left", "h":
		switch m.cfg.BotDifficulty {
		case "easy":
			m.cfg.BotDifficulty = "hard"
		case "medium":
			m.cfg.BotDifficulty = "easy"
		case "hard":
			m.cfg.BotDifficulty = "medium"
		}
	case "esc", "enter":
		m.state = StateSettings
	}
	return m, nil
}

// updateSettingsTiming handles turn delay config
func (m AppModel) updateSettingsTiming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "right", "l":
		m.cfg.TurnDelayMs += 50
		m.cfg.Validate()
	case "down", "j", "left", "h":
		m.cfg.TurnDelayMs -= 50
		m.cfg.Validate()
	case "esc", "enter":
		m.state = StateSettings
	}
	return m, nil
}

// updateSettingsK8s handles kubernetes config
func (m AppModel) updateSettingsK8s(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "down", "j", " ":
		m.cfg.EnableRealK8s = !m.cfg.EnableRealK8s
	case "esc", "enter":
		m.state = StateSettings
	}
	return m, nil
}

// handleTick processes animation ticks
func (m AppModel) handleTick() (tea.Model, tea.Cmd) {
	if m.state != StateBattle || m.gameOver {
		return m, nil
	}

	currentCompanyID := m.getCurrentTurnCompanyID()

	// Process AI turn animation
	if m.animating && len(m.pendingAttacks) > 0 {
		attack := m.pendingAttacks[0]
		m.pendingAttacks = m.pendingAttacks[1:]

		m.centerViewportOn(attack[0], attack[1])

		// Get AI for current company
		ai := m.ais[currentCompanyID]
		var result *ShotResult

		if ai != nil {
			// AI attacks using stored target ID from KNN selection (with K8s sync)
			targetID := m.pendingTargetID
			result, _ = m.performAttackWithK8sSync(attack[0], attack[1], currentCompanyID, targetID)
			ai.RecordResultAgainst(attack[0], attack[1], targetID, result)
		} else {
			result, _ = m.performAttackWithK8sSync(attack[0], attack[1], currentCompanyID, "")
		}

		// Log the attack
		company := m.getCompanyByID(currentCompanyID)
		companyName := "Unknown"
		if company != nil {
			companyName = company.Name
		}
		if result != nil && result.Hit {
			m.addBattleLog(fmt.Sprintf("%s hit at (%d,%d)!", companyName, attack[0], attack[1]))
		} else {
			m.addBattleLog(fmt.Sprintf("%s missed at (%d,%d)", companyName, attack[0], attack[1]))
		}

		if len(m.pendingAttacks) > 0 {
			return m, doTickWithDelay(m.cfg.TurnDelayMs)
		}

		// Done with this company's turn
		m.animating = false
		m.checkMultiWinCondition()

		if !m.gameOver {
			m.advanceTurn()
			// If next turn is AI, continue ticking
			if !m.isPlayerTurn || m.demoMode {
				return m, doTickWithDelay(m.cfg.TurnDelayMs)
			}
		}
		return m, nil
	}

	// Demo mode: auto-play player turn using KNN
	if m.demoMode && m.isPlayerTurn && !m.animating && !m.gameOver && m.playerAI != nil {
		activeOpponents := m.getActiveOpponentsFor("player")
		target, targetID := m.playerAI.PickTargetAgainstKNN(activeOpponents, 3)
		m.cursor = target

		m.centerViewportOn(target[0], target[1])
		result, _ := m.performAttackWithK8sSync(target[0], target[1], "player", targetID)
		m.playerAI.RecordResultAgainst(target[0], target[1], targetID, result)

		player := m.playerCompany.Name
		if result != nil && result.Hit {
			m.addBattleLog(fmt.Sprintf("%s hit at (%d,%d)!", player, target[0], target[1]))
			m.lastMessage = result.Message
		} else {
			m.addBattleLog(fmt.Sprintf("%s missed at (%d,%d)", player, target[0], target[1]))
			m.lastMessage = "Miss"
		}

		m.checkMultiWinCondition()
		if !m.gameOver {
			m.advanceTurn()
			return m, doTickWithDelay(m.cfg.TurnDelayMs)
		}
	}

	// Start next AI turn if it's an AI's turn
	if !m.isPlayerTurn && !m.animating && !m.gameOver {
		if m.startAITurn(currentCompanyID) {
			return m, doTickWithDelay(m.cfg.TurnDelayMs)
		}
		// AI couldn't start turn (no AI for this company), skip to next
		m.advanceTurn()
		if !m.isPlayerTurn {
			return m, doTickWithDelay(m.cfg.TurnDelayMs)
		}
	}

	return m, nil
}

// getCurrentTurnCompanyID returns the company whose turn it is
func (m *AppModel) getCurrentTurnCompanyID() string {
	if len(m.turnQueue) == 0 {
		return "player"
	}
	if m.currentTurnIndex >= len(m.turnQueue) {
		m.currentTurnIndex = 0
	}
	return m.turnQueue[m.currentTurnIndex]
}

// advanceTurn moves to the next company in the turn queue
func (m *AppModel) advanceTurn() {
	// Remove eliminated companies from queue
	newQueue := make([]string, 0)
	for _, id := range m.turnQueue {
		if m.board.FleetHealthyByID(id) {
			newQueue = append(newQueue, id)
		}
	}
	m.turnQueue = newQueue

	if len(m.turnQueue) == 0 {
		return
	}

	// Advance index
	m.currentTurnIndex = (m.currentTurnIndex + 1) % len(m.turnQueue)

	// If wrapped to 0, increment turn counter
	if m.currentTurnIndex == 0 {
		m.turn++
	}

	// Update legacy flag
	m.isPlayerTurn = (m.getCurrentTurnCompanyID() == "player")
}

// startAITurn queues attacks for an AI company. Returns true if turn was started.
func (m *AppModel) startAITurn(companyID string) bool {
	ai := m.ais[companyID]
	if ai == nil {
		return false
	}

	// Get active opponents for this AI
	activeOpponents := m.getActiveOpponentsFor(companyID)
	if len(activeOpponents) == 0 {
		return false
	}

	m.animating = true
	m.currentAttacker = companyID
	m.pendingAttacks = make([][2]int, 0)

	// Pick target using KNN algorithm (K=3 nearest hits)
	target, targetID := ai.PickTargetAgainstKNN(activeOpponents, 3)
	m.pendingAttacks = append(m.pendingAttacks, target)
	m.pendingTargetID = targetID
	return true
}

// getActiveOpponentsFor returns company IDs of active opponents
func (m *AppModel) getActiveOpponentsFor(companyID string) []string {
	opponents := make([]string, 0)

	// Player is an opponent if not the current company and still alive
	if companyID != "player" && m.board.FleetHealthyByID("player") {
		opponents = append(opponents, "player")
	}

	// Other companies are opponents
	for _, enemy := range m.enemyCompanies {
		if enemy.ID != companyID && m.board.FleetHealthyByID(enemy.ID) {
			opponents = append(opponents, enemy.ID)
		}
	}

	return opponents
}

// getCompanyByID returns the company with the given ID
func (m *AppModel) getCompanyByID(id string) *game.Company {
	if id == "player" {
		return m.playerCompany
	}
	for _, enemy := range m.enemyCompanies {
		if enemy.ID == id {
			return enemy
		}
	}
	return nil
}

// Legacy startEnemyTurn for backward compatibility
func (m *AppModel) startEnemyTurn() {
	if len(m.enemyCompanies) > 0 {
		m.startAITurn(m.enemyCompanies[0].ID)
	}
}

// addBattleLog adds a message to the battle log (keeps last 5)
func (m *AppModel) addBattleLog(msg string) {
	m.battleLog = append(m.battleLog, msg)
	if len(m.battleLog) > 5 {
		m.battleLog = m.battleLog[1:]
	}
}

// View renders the current state
func (m AppModel) View() string {
	switch m.state {
	case StateMenu:
		return m.renderMenu()
	case StateSettings:
		return m.renderSettings()
	case StateSettingsBoard:
		return m.renderSettingsBoard()
	case StateSettingsShips:
		return m.renderSettingsShips()
	case StateSettingsPods:
		return m.renderSettingsPods()
	case StateSettingsBots:
		return m.renderSettingsBots()
	case StateSettingsTiming:
		return m.renderSettingsTiming()
	case StateSettingsK8s:
		return m.renderSettingsK8s()
	case StateCompanySelect:
		return m.renderCompanySelect()
	case StateEnemyCountSelect:
		return m.renderEnemyCountSelect()
	case StateEnemySelect:
		return m.renderEnemySelect()
	case StatePlacement:
		return m.renderPlacement()
	case StateBattle:
		return m.renderBattle()
	case StatePodView:
		return m.renderPodView()
	case StateGameOver:
		return m.renderGameOver()
	}
	return ""
}

// renderMenu draws the main menu
func (m AppModel) renderMenu() string {
	title := m.styles.Title.Render("CLUSTERSHIP")
	subtitle := m.styles.Muted.Render("Battleship meets Kubernetes")

	var menu string
	for i, item := range m.menuItems {
		if i == m.menuCursor {
			menu += m.styles.MenuItemSelected.Render("> "+item) + "\n"
		} else {
			menu += m.styles.MenuItem.Render("  "+item) + "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[up/down] navigate  [enter] select  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			subtitle,
			"",
			menu,
			hint,
		),
	)
}

// renderSettings draws the main settings menu
func (m AppModel) renderSettings() string {
	title := m.styles.Title.Render("SETTINGS")
	subtitle := m.styles.Muted.Render("Configure your game")

	var menu string
	for i, item := range m.settingsItems {
		if i == m.settingsCursor {
			menu += m.styles.MenuItemSelected.Render("> "+item) + "\n"
		} else {
			menu += m.styles.MenuItem.Render("  "+item) + "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[up/down] navigate  [enter] select  [esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, subtitle, "", menu, hint),
	)
}

// renderSettingsBoard draws board size config
func (m AppModel) renderSettingsBoard() string {
	title := m.styles.Title.Render("BOARD SETTINGS")

	info := fmt.Sprintf("Width:  %d  [</>]\n", m.cfg.BoardWidth)
	info += fmt.Sprintf("Height: %d  [^/v]\n", m.cfg.BoardHeight)

	visual := fmt.Sprintf("\n%dx%d board = %d cells\n",
		m.cfg.BoardWidth, m.cfg.BoardHeight,
		m.cfg.BoardWidth*m.cfg.BoardHeight)

	hint := m.styles.Muted.Render("\n[arrows] adjust  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hint),
	)
}

// renderSettingsShips draws ships/racks config
func (m AppModel) renderSettingsShips() string {
	title := m.styles.Title.Render("SHIP SETTINGS")

	info := fmt.Sprintf("Ships per player: %d  [^/v]\n", m.cfg.ShipsPerPlayer)
	info += fmt.Sprintf("Racks per ship:   %d  [</>]\n", m.cfg.RacksPerShip)

	visual := fmt.Sprintf("\nTotal racks: %d per player\n",
		m.cfg.ShipsPerPlayer*m.cfg.RacksPerShip)

	hint := m.styles.Muted.Render("\n[arrows] adjust  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hint),
	)
}

// renderSettingsPods draws pods config
func (m AppModel) renderSettingsPods() string {
	title := m.styles.Title.Render("POD SETTINGS")

	info := fmt.Sprintf("Pods per rack: %d\n", m.cfg.PodsPerRack)

	totalRacks := m.cfg.ShipsPerPlayer * m.cfg.RacksPerShip
	visual := fmt.Sprintf("\nTotal pods per player: %d\n",
		totalRacks*m.cfg.PodsPerRack)

	hint := m.styles.Muted.Render("\n[arrows] adjust  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hint),
	)
}

// renderSettingsBots draws bot config
func (m AppModel) renderSettingsBots() string {
	title := m.styles.Title.Render("BOT SETTINGS")

	info := fmt.Sprintf("Max bots:   %d       [^/v]\n", m.cfg.MaxBots)
	info += fmt.Sprintf("Difficulty: %-6s  [</>]\n", m.cfg.BotDifficulty)

	visual := "\nDifficulty affects AI targeting accuracy\n"
	visual += "  easy:   random targeting\n"
	visual += "  medium: some pattern detection\n"
	visual += "  hard:   aggressive hunting\n"

	hint := m.styles.Muted.Render("\n[arrows] adjust  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hint),
	)
}

// renderSettingsTiming draws timing/animation config
func (m AppModel) renderSettingsTiming() string {
	title := m.styles.Title.Render("TIMING SETTINGS")

	info := fmt.Sprintf("Turn Delay: %dms\n", m.cfg.TurnDelayMs)

	// Visual speed indicator
	var speedLabel string
	switch {
	case m.cfg.TurnDelayMs <= 100:
		speedLabel = "LIGHTNING FAST"
	case m.cfg.TurnDelayMs <= 200:
		speedLabel = "Fast"
	case m.cfg.TurnDelayMs <= 500:
		speedLabel = "Normal"
	case m.cfg.TurnDelayMs <= 1000:
		speedLabel = "Slow"
	default:
		speedLabel = "Very Slow"
	}

	visual := fmt.Sprintf("\nSpeed: %s\n", speedLabel)
	visual += "\nThis controls how fast AI turns animate.\n"
	visual += "Lower = faster gameplay, harder to follow.\n"
	visual += "Higher = slower, easier to watch battles.\n"
	visual += fmt.Sprintf("\nRange: 50ms - 2000ms (current: %dms)\n", m.cfg.TurnDelayMs)

	hint := m.styles.Muted.Render("\n[arrows] adjust (+/- 50ms)  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hint),
	)
}

// renderSettingsK8s draws kubernetes config
func (m AppModel) renderSettingsK8s() string {
	title := m.styles.Title.Render("KUBERNETES SETTINGS")

	enabled := "OFF"
	if m.cfg.EnableRealK8s {
		enabled = "ON"
	}

	info := fmt.Sprintf("Real K8s deployment: %s\n", enabled)
	info += fmt.Sprintf("Namespace: %s\n", m.cfg.K8sNamespace)
	info += fmt.Sprintf("Kubeconfig: %s\n", m.cfg.Kubeconfig)

	visual := "\nWhen enabled, game deploys real pods to your cluster.\n"
	visual += "Requires local k8s (minikube/kind).\n"

	hint := m.styles.Muted.Render("\n[space] toggle  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hint),
	)
}

// renderCompanySelect draws the company selection screen
func (m AppModel) renderCompanySelect() string {
	title := m.styles.Title.Render("SELECT YOUR COMPANY")
	subtitle := m.styles.Muted.Render("Choose who you'll defend")

	var list string
	for i, id := range m.companies {
		template, _ := game.LoadCompanyTemplate(id)
		if template == nil {
			continue
		}
		if i == m.companyCursor {
			list += m.styles.MenuItemSelected.Render("> "+template.Name) + "\n"
			list += m.styles.Muted.Render("   "+template.Description) + "\n"
		} else {
			list += m.styles.MenuItem.Render("  "+template.Name) + "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[up/down] navigate  [enter] select  [esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			subtitle,
			"",
			list,
			hint,
		),
	)
}

// renderEnemyCountSelect draws the enemy count selection screen
func (m AppModel) renderEnemyCountSelect() string {
	title := m.styles.Title.Render("SELECT NUMBER OF ENEMIES")
	subtitle := m.styles.Muted.Render("Battle Royale: Everyone attacks everyone!")

	var list string
	for i := 0; i < m.maxEnemies; i++ {
		count := i + 1
		label := fmt.Sprintf("%d enemy", count)
		if count > 1 {
			label = fmt.Sprintf("%d enemies", count)
		}

		if i == m.enemyCountCursor {
			list += m.styles.MenuItemSelected.Render("> "+label) + "\n"
		} else {
			list += m.styles.MenuItem.Render("  "+label) + "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[↑/↓] select  [enter] confirm  [esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, subtitle, "", list, hint),
	)
}

// renderEnemySelect draws the enemy company selection screen
func (m AppModel) renderEnemySelect() string {
	targetCount := m.enemyCountCursor + 1
	title := m.styles.Title.Render(fmt.Sprintf("SELECT %d ENEMIES", targetCount))
	subtitle := m.styles.Muted.Render(fmt.Sprintf("Selected: %d/%d", len(m.selectedEnemies), targetCount))

	var list string
	for i, id := range m.companies {
		template, _ := game.LoadCompanyTemplate(id)
		if template == nil {
			continue
		}

		// Skip player's company
		if m.playerCompany != nil && id == m.playerCompany.ID {
			list += m.styles.Muted.Render(fmt.Sprintf("  %s (you)", template.Name)) + "\n"
			continue
		}

		// Check if selected
		selected := false
		for _, selID := range m.selectedEnemies {
			if selID == id {
				selected = true
				break
			}
		}

		line := template.Name
		if selected {
			line = "[X] " + line
		} else {
			line = "[ ] " + line
		}

		if i == m.enemySelectCursor {
			list += m.styles.MenuItemSelected.Render("> "+line) + "\n"
		} else {
			list += m.styles.MenuItem.Render("  "+line) + "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[↑/↓] navigate  [enter] toggle  [c] confirm  [esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, subtitle, "", list, hint),
	)
}

// renderPlacement draws the placement phase (auto-place for now)
func (m AppModel) renderPlacement() string {
	title := m.styles.Title.Render("DEPLOYMENT PHASE")

	var info string
	if m.playerCompany != nil {
		info = fmt.Sprintf("Your fleet: %s\n", m.playerCompany.Name)
		info += fmt.Sprintf("Regions: %d | Total Racks: %d\n", len(m.playerCompany.Regions), m.playerCompany.TotalRacks())
	}

	// Show all selected enemies
	if len(m.selectedEnemies) > 0 {
		info += fmt.Sprintf("\nEnemy fleets (%d):\n", len(m.selectedEnemies))
		for _, enemyID := range m.selectedEnemies {
			template, _ := game.LoadCompanyTemplate(enemyID)
			if template != nil {
				info += fmt.Sprintf("  %s\n", template.Name)
			}
		}
	} else if m.enemyCompany != nil {
		info += fmt.Sprintf("\nEnemy fleet: %s\n", m.enemyCompany.Name)
	}

	hint := m.styles.Muted.Render("\n[enter] auto-deploy and start battle  [esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			info,
			hint,
		),
	)
}

// renderBattle draws the main battle screen (uses viewLevel)
func (m AppModel) renderBattle() string {
	if m.playerCompany == nil || m.enemyCompany == nil {
		return "Loading..."
	}

	// render based on view level
	switch m.viewLevel {
	case ViewShip:
		return m.renderShipView()
	case ViewRack:
		return m.renderRackView()
	case ViewYAML:
		return m.renderYAMLView()
	case ViewRackLayout:
		return m.renderRackLayoutView()
	default:
		return m.renderMapView()
	}
}

// renderMapView is the default battle view (level 1)
func (m AppModel) renderMapView() string {
	// header with view level indicator
	turnInfo := fmt.Sprintf("Turn: %d", m.turn)
	if m.isPlayerTurn {
		turnInfo += " | YOUR TURN"
	}
	viewIndicator := fmt.Sprintf(" [VIEW 1/5]")
	header := m.styles.Header.Render(
		fmt.Sprintf("CLUSTERSHIP - %s vs %s   %s%s",
			m.playerCompany.Name, m.enemyCompany.Name, turnInfo, viewIndicator),
	)

	// board
	board := m.renderSimpleBoard()

	// sidebar with service status and events
	sidebar := m.renderServiceStatus()

	// combine board and sidebar
	main := lipgloss.JoinHorizontal(lipgloss.Top, board, "  ", sidebar)

	// status line with last message
	var statusLine string
	if m.lastMessage != "" {
		statusLine = m.styles.Warning.Render(m.lastMessage) + "\n"
	}

	// footer with controls
	footer := m.styles.Footer.Render(
		"[1-5] view level  [arrows] move  [enter] fire  [c] cycle  [q] quit",
	)

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			main,
			statusLine,
			footer,
		),
	)
}

// renderShipView shows ship/region detail (level 2)
func (m AppModel) renderShipView() string {
	header := m.styles.Header.Render(
		fmt.Sprintf("SHIP VIEW - %s  [VIEW 2/5]", m.playerCompany.Name),
	)

	var content string
	content += m.styles.Subtitle.Render("YOUR REGIONS (SHIPS):") + "\n\n"

	for i, region := range m.playerCompany.Regions {
		status := "ACTIVE"
		style := m.styles.Success
		if region.IsDestroyed {
			status = "DESTROYED"
			style = m.styles.Error
		}

		// count healthy racks
		healthyRacks := 0
		for _, rack := range region.Racks {
			if !rack.IsDestroyed {
				healthyRacks++
			}
		}

		selected := ""
		if region.ID == m.selectedShipID {
			selected = "> "
		} else {
			selected = "  "
		}

		line := fmt.Sprintf("%s%s (%s) - %d/%d racks - %s",
			selected, region.Name, region.ID,
			healthyRacks, len(region.Racks),
			style.Render(status))
		content += line + "\n"

		// show racks if selected
		if region.ID == m.selectedShipID {
			for _, rack := range region.Racks {
				rackStatus := "OK"
				rackStyle := m.styles.Success
				if rack.IsDestroyed {
					rackStatus = "HIT"
					rackStyle = m.styles.Error
				}
				content += fmt.Sprintf("    [%s] %s at (%d,%d) - %d pods\n",
					rackStyle.Render(rackStatus), rack.ID,
					rack.Position[0], rack.Position[1], len(rack.Pods))
			}
		}

		// navigation hint on first ship
		if i == 0 {
			content += "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[up/down] select ship  [1-5] change view  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content, hint),
	)
}

// renderRackView shows rack detail with pods (level 3)
func (m AppModel) renderRackView() string {
	header := m.styles.Header.Render(
		fmt.Sprintf("RACK VIEW - %s  [VIEW 3/5]", m.playerCompany.Name),
	)

	var content string

	// show selected rack if any
	if m.selectedRack != nil {
		rack := m.selectedRack
		status := "ONLINE"
		statusStyle := m.styles.Success
		if rack.IsDestroyed {
			status = "DESTROYED"
			statusStyle = m.styles.Error
		}

		content += m.styles.Subtitle.Render(fmt.Sprintf("RACK: %s", rack.ID)) + "\n"
		content += fmt.Sprintf("Position: (%d, %d)\n", rack.Position[0], rack.Position[1])
		content += fmt.Sprintf("Status: %s\n", statusStyle.Render(status))
		content += fmt.Sprintf("Capacity: %d pods\n\n", len(rack.Pods))

		content += m.styles.Subtitle.Render("PODS:") + "\n"
		for _, pod := range rack.Pods {
			podStatus := "Running"
			podStyle := m.styles.Success
			switch pod.Status {
			case game.PodPending:
				podStatus = "Pending"
				podStyle = m.styles.Warning
			case game.PodTerminated:
				podStatus = "Terminated"
				podStyle = m.styles.Error
			}
			content += fmt.Sprintf("  [%s] %s (svc: %s)\n",
				podStyle.Render(podStatus[:1]), pod.ID, pod.ServiceID)
		}
	} else {
		content = m.styles.Muted.Render("No rack selected. Press 2 to select a ship first.")
	}

	// show all racks for navigation
	content += "\n" + m.styles.Subtitle.Render("ALL RACKS:") + "\n"
	for _, region := range m.playerCompany.Regions {
		content += fmt.Sprintf("  %s:\n", region.Name)
		for _, rack := range region.Racks {
			selected := "  "
			if rack.ID == m.selectedRackID {
				selected = "> "
			}
			status := "OK"
			if rack.IsDestroyed {
				status = "HIT"
			}
			content += fmt.Sprintf("  %s[%s] %s\n", selected, status, rack.ID)
		}
	}

	hint := m.styles.Muted.Render("\n[up/down] select rack  [1-5] change view  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content, hint),
	)
}

// renderYAMLView shows YAML manifest (level 4, fog of war applies)
func (m AppModel) renderYAMLView() string {
	header := m.styles.Header.Render("YAML VIEW  [VIEW 4/5]")

	var content string

	// YOUR SERVICES - always fully visible
	content += m.styles.Subtitle.Render("YOUR SERVICES (full visibility):") + "\n"
	content += m.renderServiceYAMLList(m.playerCompany, true) + "\n"

	// ENEMY SERVICES - fog of war applies
	for _, enemy := range m.enemyCompanies {
		content += m.styles.Subtitle.Render(fmt.Sprintf("ENEMY: %s (fog of war):", enemy.Name)) + "\n"
		content += m.renderServiceYAMLList(enemy, false) + "\n"
	}

	hint := m.styles.Muted.Render("\n[up/down] select service  [1-5] change view  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content, hint),
	)
}

// renderServiceYAMLList renders services for a company with fog of war
func (m AppModel) renderServiceYAMLList(company *game.Company, isOwn bool) string {
	var content string

	for i, svc := range company.Services {
		selected := "  "
		if svc.ID == m.selectedSvcID {
			selected = "> "
		}

		// determine visibility based on fog of war rules
		canSeeName := isOwn || m.canSeeServiceName(company, svc)
		canSeeYAML := isOwn || m.canSeeServiceYAML(company, svc)

		// count pods by status
		running, pending, terminated := 0, 0, 0
		for _, p := range svc.Pods {
			switch p.Status {
			case game.PodRunning:
				running++
			case game.PodPending:
				pending++
			case game.PodTerminated:
				terminated++
			}
		}

		// service name line
		var line string
		if canSeeName {
			line = fmt.Sprintf("%s%s - R:%d P:%d T:%d", selected, svc.Name, running, pending, terminated)
		} else {
			line = fmt.Sprintf("%s??? - unknown service", selected)
		}
		content += line + "\n"

		// show yaml if selected
		if svc.ID == m.selectedSvcID {
			content += m.styles.Muted.Render("    ---") + "\n"

			if canSeeYAML {
				// full yaml visible
				yaml := fmt.Sprintf(`    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: %s
      labels:
        service: %s
        company: %s
    spec:
      replicas: %d
      selector:
        matchLabels:
          service: %s
      template:
        spec:
          containers:
          - name: %s
            # full yaml with real K8s enabled`,
					svc.ID, svc.ID, company.ID, len(svc.Pods), svc.ID, svc.ID)
				content += m.styles.Muted.Render(yaml) + "\n"
			} else if canSeeName {
				// partial - name visible but yaml hidden
				yaml := fmt.Sprintf(`    apiVersion: ???
    kind: ???
    metadata:
      name: %s
      # YAML REDACTED
      # destroy the ship to reveal full manifest`,
					svc.ID)
				content += m.styles.Warning.Render(yaml) + "\n"
			} else {
				// fully hidden
				yaml := `    # SERVICE NOT YET DISCOVERED
    # land a hit to reveal service name
    # destroy ship to reveal full YAML`
				content += m.styles.Error.Render(yaml) + "\n"
			}
		}

		if i < len(company.Services)-1 {
			content += "\n"
		}
	}

	return content
}

// canSeeServiceName returns true if service name is visible (own or has hit)
func (m AppModel) canSeeServiceName(company *game.Company, svc *game.Service) bool {
	// can always see own services
	if company.ID == m.playerCompany.ID {
		return true
	}

	// can see enemy service if any pod has been hit (terminated)
	for _, p := range svc.Pods {
		if p.Status == game.PodTerminated {
			return true
		}
	}

	// can see if any rack with this service has been hit
	for _, region := range company.Regions {
		for _, rack := range region.Racks {
			if rack.IsDestroyed {
				// check if rack had pods from this service
				for _, p := range rack.Pods {
					if p.ServiceID == svc.ID {
						return true
					}
				}
			}
		}
	}

	return false
}

// canSeeServiceYAML returns true if full YAML is visible (own or ship destroyed)
func (m AppModel) canSeeServiceYAML(company *game.Company, svc *game.Service) bool {
	// can always see own services
	if company.ID == m.playerCompany.ID {
		return true
	}

	// can see enemy YAML only if the ship containing service is destroyed
	for _, region := range company.Regions {
		if region.IsDestroyed {
			// check if region had pods from this service
			for _, rack := range region.Racks {
				for _, p := range rack.Pods {
					if p.ServiceID == svc.ID {
						return true
					}
				}
			}
		}
	}

	return false
}

// renderRackLayoutView shows a visual grid of rack/pod distribution (VIEW 5)
func (m AppModel) renderRackLayoutView() string {
	header := m.styles.Header.Render("RACK LAYOUT  [VIEW 5/5]")

	var content string

	// Show player company first
	content += m.styles.Subtitle.Render(fmt.Sprintf("YOUR FLEET: %s", m.playerCompany.Name)) + "\n\n"
	content += m.renderCompanyRackLayout(m.playerCompany, true) + "\n"

	// Show enemy companies
	for _, enemy := range m.enemyCompanies {
		content += m.styles.Subtitle.Render(fmt.Sprintf("ENEMY: %s", enemy.Name)) + "\n\n"
		content += m.renderCompanyRackLayout(enemy, false) + "\n"
	}

	// Legend
	legend := m.styles.Muted.Render(
		"LEGEND: [!]=Hard Affinity (critical)  [~]=Spread  [o]=Soft  [-]=None  "+
			"X=Destroyed  _=Empty") + "\n"

	hint := m.styles.Muted.Render("[1-5] change view  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content, legend, hint),
	)
}

// renderCompanyRackLayout renders rack layout for a single company
func (m AppModel) renderCompanyRackLayout(company *game.Company, isOwn bool) string {
	var output string

	for _, region := range company.Regions {
		// Ship header with status
		status := "OPERATIONAL"
		statusStyle := m.styles.Success
		if region.IsDestroyed {
			status = "DESTROYED"
			statusStyle = m.styles.Error
		} else {
			// Check how damaged
			destroyedRacks := 0
			for _, rack := range region.Racks {
				if rack.IsDestroyed {
					destroyedRacks++
				}
			}
			if destroyedRacks > 0 {
				status = fmt.Sprintf("DAMAGED (%d/%d)", destroyedRacks, len(region.Racks))
				statusStyle = m.styles.Warning
			}
		}

		shipType := getShipType(len(region.Racks))
		output += fmt.Sprintf("%s %s [%d racks] - %s\n",
			region.Name, shipType, len(region.Racks), statusStyle.Render(status))

		// Build rack grid
		rackWidth := 10 // characters per rack column
		numRacks := len(region.Racks)

		// Top border
		topBorder := "+"
		for i := 0; i < numRacks; i++ {
			topBorder += fmt.Sprintf("%s+", repeatChar('-', rackWidth))
		}
		output += topBorder + "\n"

		// Rack names row
		nameRow := "|"
		for _, rack := range region.Racks {
			rackLabel := rack.ID
			if len(rackLabel) > rackWidth-2 {
				rackLabel = rackLabel[:rackWidth-2]
			}
			if rack.IsDestroyed {
				rackLabel = "DESTROYED"
			}
			nameRow += fmt.Sprintf(" %-*s|", rackWidth-1, rackLabel)
		}
		output += nameRow + "\n"

		// Middle border
		output += topBorder + "\n"

		// Build pod lists per rack (up to 4 pods shown per rack)
		maxPodRows := 4
		podRows := make([][]string, maxPodRows)
		for i := range podRows {
			podRows[i] = make([]string, numRacks)
		}

		for rackIdx, rack := range region.Racks {
			if rack.IsDestroyed {
				// Show X for destroyed rack
				podRows[0][rackIdx] = "    X    "
				for row := 1; row < maxPodRows; row++ {
					podRows[row][rackIdx] = "         "
				}
				continue
			}

			// Collect pods on this rack grouped by service
			svcPods := make(map[string]int)        // serviceID -> count
			svcAffinity := make(map[string]string) // serviceID -> affinity icon
			svcNames := make(map[string]string)    // serviceID -> abbreviated name

			for _, pod := range rack.Pods {
				if pod.Status == game.PodTerminated {
					continue
				}
				svcPods[pod.ServiceID]++

				// Get affinity icon for this service
				for _, svc := range company.Services {
					if svc.ID == pod.ServiceID {
						svcAffinity[pod.ServiceID] = getAffinityIcon(svc.Affinity)
						// Abbreviate service name to 3-4 chars
						svcNames[pod.ServiceID] = abbreviateService(svc.Name)
						break
					}
				}
			}

			// Convert to display rows
			rowIdx := 0
			for svcID, count := range svcPods {
				if rowIdx >= maxPodRows {
					break
				}
				icon := svcAffinity[svcID]
				name := svcNames[svcID]
				if name == "" {
					name = svcID[:min(4, len(svcID))]
				}

				// Format: [!]API(2)
				entry := fmt.Sprintf("%s%s", icon, name)
				if count > 1 {
					entry += fmt.Sprintf("(%d)", count)
				}
				// Pad to rack width
				if len(entry) < rackWidth-1 {
					entry = fmt.Sprintf("%-*s", rackWidth-1, entry)
				}
				podRows[rowIdx][rackIdx] = entry
				rowIdx++
			}

			// Fill remaining rows with empty space
			for ; rowIdx < maxPodRows; rowIdx++ {
				podRows[rowIdx][rackIdx] = fmt.Sprintf("%-*s", rackWidth-1, "")
			}
		}

		// Render pod rows
		for _, row := range podRows {
			rowStr := "|"
			for _, cell := range row {
				rowStr += fmt.Sprintf(" %s|", cell)
			}
			output += rowStr + "\n"
		}

		// Bottom border
		output += topBorder + "\n"

		// Capacity row
		capRow := " "
		for _, rack := range region.Racks {
			if rack.IsDestroyed {
				capRow += fmt.Sprintf(" %-*s ", rackWidth-1, "N/A")
			} else {
				// Count running pods
				running := 0
				for _, p := range rack.Pods {
					if p.Status == game.PodRunning {
						running++
					}
				}
				capStr := fmt.Sprintf("%d/%d pods", running, rack.Capacity)
				capRow += fmt.Sprintf(" %-*s ", rackWidth-1, capStr)
			}
		}
		output += m.styles.Muted.Render(capRow) + "\n\n"
	}

	return output
}

// getShipType returns ship type name based on rack count
func getShipType(racks int) string {
	switch racks {
	case 5:
		return "CARRIER"
	case 4:
		return "CRUISER"
	case 3:
		return "DESTROYER"
	case 2:
		return "PATROL"
	default:
		return "VESSEL"
	}
}

// abbreviateService returns a short abbreviation for a service name
func abbreviateService(name string) string {
	// Common abbreviations
	abbrevs := map[string]string{
		"CDN Edge Cache":     "CDN",
		"Playback Service":   "Play",
		"Origin API":         "API",
		"Primary Database":   "DB",
		"Encoding Workers":   "Enc",
		"EC2 API Gateway":    "API",
		"CloudFront CDN":     "CDN",
		"S3 Storage":         "S3",
		"Lambda Workers":     "Lamb",
		"RDS Database":       "RDS",
	}

	if abbr, ok := abbrevs[name]; ok {
		return abbr
	}

	// Default: first 4 characters
	if len(name) > 4 {
		return name[:4]
	}
	return name
}

// repeatChar returns a string with char repeated n times
func repeatChar(char rune, n int) string {
	result := make([]rune, n)
	for i := range result {
		result[i] = char
	}
	return string(result)
}

// renderSimpleBoard renders a simple view of the enemy board
func (m AppModel) renderSimpleBoard() string {
	if m.board == nil {
		return "No board"
	}

	viewW, viewH := m.viewW, m.viewH
	if viewW <= 0 {
		viewW = 30
	}
	if viewH <= 0 {
		viewH = 20
	}

	// title showing viewport position
	title := fmt.Sprintf("OCEAN MAP [%d-%d, %d-%d] of %dx%d",
		m.viewport[0], m.viewport[0]+viewW,
		m.viewport[1], m.viewport[1]+viewH,
		m.board.Width, m.board.Height)

	// header rows with column numbers (tens row + ones row)
	tensRow := "    "
	onesRow := "    "
	for x := 0; x < viewW; x++ {
		col := m.viewport[0] + x
		tens := (col / 10) % 10
		ones := col % 10
		tensRow += fmt.Sprintf("%d ", tens)
		onesRow += fmt.Sprintf("%d ", ones)
	}
	header := tensRow + "\n" + onesRow + "\n"

	var grid string
	for y := 0; y < viewH; y++ {
		boardY := m.viewport[1] + y
		if boardY >= m.board.Height {
			break
		}
		// row number
		grid += fmt.Sprintf("%3d ", boardY)

		for x := 0; x < viewW; x++ {
			boardX := m.viewport[0] + x
			if boardX >= m.board.Width {
				break
			}

			cell := m.getUnifiedCellDisplay(boardX, boardY)

			// highlight cursor
			if boardX == m.cursor[0] && boardY == m.cursor[1] {
				cell = m.styles.Cursor.Render(cell)
			}

			grid += cell + " "
		}
		grid += "\n"
	}

	// cursor info and turn status
	cursorInfo := fmt.Sprintf("Cursor: (%d, %d)", m.cursor[0], m.cursor[1])
	if m.animating {
		cursorInfo += " | ENEMY ATTACKING..."
	} else if m.isPlayerTurn {
		cursorInfo += " | YOUR TURN"
	}
	if m.demoMode {
		cursorInfo += " [DEMO]"
	}
	if m.debugMode {
		cursorInfo += " [DEBUG: FOG OFF]"
	}

	// Hover info - show details about what's at cursor position
	hoverInfo := m.getHoverInfo()

	// legend
	legend := fmt.Sprintf("%s=Water  %s=Miss  %s=Hit  %s=Ship  %s=Destroyed",
		m.styles.Water.Render(SymWater),
		m.styles.Miss.Render(SymMiss),
		m.styles.Hit.Render(SymHit),
		m.styles.Ship.Render(SymShip),
		m.styles.Destroyed.Render(SymDestroyed))

	// controls
	controls := "[arrows] move  [WASD] pan  [enter] fire  [p] pods  [v] debug"

	// Build output with optional hover info
	elements := []string{
		m.styles.Subtitle.Render(title),
		header + grid,
		m.styles.Muted.Render(cursorInfo),
	}
	if hoverInfo != "" {
		elements = append(elements, hoverInfo)
	}
	elements = append(elements, legend, m.styles.Muted.Render(controls))

	return m.styles.BoardArea.Render(
		lipgloss.JoinVertical(lipgloss.Left, elements...),
	)
}

// getUnifiedCellDisplay shows both player ships and all shots on the shared ocean
func (m AppModel) getUnifiedCellDisplay(x, y int) string {
	if m.board == nil {
		return SymWater
	}

	playerState := m.board.GetEnemyCellState(x, y)
	enemyState := m.board.GetPlayerCellState(x, y)

	// show hits/misses first
	if playerState == CellHit || playerState == CellDestroyed {
		if playerState == CellDestroyed {
			return m.styles.Destroyed.Render(SymDestroyed)
		}
		return m.styles.Hit.Render(SymHit)
	}

	if playerState == CellMiss {
		return m.styles.Miss.Render(SymMiss)
	}

	if enemyState == CellHit || enemyState == CellDestroyed {
		if enemyState == CellDestroyed {
			return m.styles.Destroyed.Render(SymDestroyed)
		}
		return m.styles.Hit.Render(SymHit)
	}

	if enemyState == CellMiss {
		return m.styles.Miss.Render(SymMiss)
	}

	if enemyState == CellShip {
		return m.styles.Ship.Render(SymShip)
	}

	if m.debugMode && m.board.HasEnemyShipAt(x, y) {
		return m.styles.Ship.Render(SymShip)
	}

	return m.styles.Water.Render(SymWater)
}

// renderServiceStatus shows the status of services and K8s events
func (m AppModel) renderServiceStatus() string {
	// Determine which company to display based on serviceViewIndex
	var viewCompany *game.Company
	var titleLabel string
	totalCompanies := 1 + len(m.enemyCompanies)

	if m.serviceViewIndex == 0 {
		viewCompany = m.playerCompany
		titleLabel = "YOUR SERVICES"
	} else if m.serviceViewIndex > 0 && m.serviceViewIndex <= len(m.enemyCompanies) {
		viewCompany = m.enemyCompanies[m.serviceViewIndex-1]
		titleLabel = viewCompany.Name
	}

	if viewCompany == nil {
		return ""
	}

	// service status section with navigation hint
	navHint := fmt.Sprintf(" [C] %d/%d", m.serviceViewIndex+1, totalCompanies)
	serviceTitle := m.styles.Subtitle.Render(titleLabel) + m.styles.Muted.Render(navHint)

	var services string
	for _, svc := range viewCompany.Services {
		healthy := 0
		total := len(svc.Pods)
		for _, p := range svc.Pods {
			if p.Status == game.PodRunning {
				healthy++
			}
		}

		// health bar
		pct := 0
		if total > 0 {
			pct = (healthy * 100) / total
		}
		bar := m.renderHealthBar(pct)

		// Affinity icon
		affinityIcon := getAffinityIcon(svc.Affinity)

		// truncate name to fit (account for icon)
		name := svc.Name
		if len(name) > 10 {
			name = name[:10]
		}
		status := fmt.Sprintf("%s%-10s %s", affinityIcon, name, bar)
		services += status + "\n"
	}

	// events section
	eventTitle := m.styles.Subtitle.Render("K8S EVENTS")
	var events string
	if m.board != nil && len(m.board.Events) > 0 {
		// show last 5 events
		start := len(m.board.Events) - 5
		if start < 0 {
			start = 0
		}
		for _, evt := range m.board.Events[start:] {
			style := m.styles.Normal
			prefix := "[ ]"
			switch evt.Type {
			case "Normal":
				prefix = "[+]"
				style = m.styles.Success
			case "Warning":
				prefix = "[!]"
				style = m.styles.Warning
			case "Error":
				prefix = "[x]"
				style = m.styles.Error
			}
			msg := evt.Message
			if len(msg) > 25 {
				msg = msg[:25] + "..."
			}
			events += fmt.Sprintf("%s %s\n", prefix, style.Render(msg))
		}
	} else {
		events = m.styles.Muted.Render("No events yet...")
	}

	// fleet stats for all companies
	statsTitle := m.styles.Subtitle.Render("FLEET STATUS")
	var stats string
	if m.board != nil {
		// Player stats
		playerFleet := m.board.Fleets["player"]
		if playerFleet != nil {
			playerStats := m.board.GetFleetStats(playerFleet)
			stats = fmt.Sprintf("You:    %d/%d pods | %d/%d racks\n",
				playerStats.RunningPods, playerStats.TotalPods,
				playerStats.AliveRacks, playerStats.TotalRacks)
		}
		// Enemy stats
		for _, enemy := range m.enemyCompanies {
			enemyFleet := m.board.Fleets[enemy.ID]
			if enemyFleet != nil {
				enemyStats := m.board.GetFleetStats(enemyFleet)
				name := enemy.Name
				if len(name) > 7 {
					name = name[:7]
				}
				stats += fmt.Sprintf("%-7s %d/%d pods | %d/%d racks\n",
					name+":", enemyStats.RunningPods, enemyStats.TotalPods,
					enemyStats.AliveRacks, enemyStats.TotalRacks)
			}
		}
	}

	// affinity legend
	legendTitle := m.styles.Subtitle.Render("AFFINITY LEGEND")
	legend := m.styles.Muted.Render("[!]=critical  [~]=spread\n[o]=preferred [-]=flexible")

	// K8s status (only if real K8s is enabled)
	k8sStatus := m.renderK8sStatus()
	k8sEvents := m.renderK8sEvents()

	return m.styles.Sidebar.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			serviceTitle, services,
			"",
			eventTitle, events,
			"",
			statsTitle, m.styles.Muted.Render(stats),
			"",
			legendTitle, legend,
			"",
			k8sStatus, k8sEvents,
		),
	)
}

// renderHealthBar creates a simple health bar
func (m AppModel) renderHealthBar(pct int) string {
	width := 10
	filled := (pct * width) / 100
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}

	style := m.styles.HealthGood
	if pct < 50 {
		style = m.styles.HealthMid
	}
	if pct < 25 {
		style = m.styles.HealthBad
	}

	return style.Render("[" + bar + "]")
}

// renderGameOver shows the game over screen
func (m AppModel) renderGameOver() string {
	title := m.styles.Title.Render("GAME OVER")

	result := m.styles.Success.Render("YOU WIN!")
	if m.winner != "player" {
		result = m.styles.Error.Render("YOU LOSE")
	}

	stats := fmt.Sprintf("Turns: %d", m.turn)

	hint := m.styles.Muted.Render("\n[enter] return to menu")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			title,
			"",
			result,
			stats,
			hint,
		),
	)
}

// helper functions

func (m *AppModel) pickRandomEnemy(exclude string) string {
	for _, id := range m.companies {
		if id != exclude {
			return id
		}
	}
	return m.companies[0] // fallback
}

func (m *AppModel) checkWinCondition() {
	if m.board == nil {
		return
	}

	// check if enemy fleet is destroyed
	if !m.board.FleetHealthy(m.board.EnemyFleet) {
		m.gameOver = true
		m.winner = "player"
		m.state = StateGameOver
		return
	}

	// check if player fleet is destroyed
	if !m.board.FleetHealthy(m.board.PlayerFleet) {
		m.gameOver = true
		m.winner = "enemy"
		m.state = StateGameOver
		return
	}
}

// checkMultiWinCondition checks if game is over in multi-company mode
func (m *AppModel) checkMultiWinCondition() {
	if m.board == nil {
		return
	}

	// Count active companies
	activeCompanies := make([]string, 0)

	if m.board.FleetHealthyByID("player") {
		activeCompanies = append(activeCompanies, "player")
	}

	for _, enemy := range m.enemyCompanies {
		if m.board.FleetHealthyByID(enemy.ID) {
			activeCompanies = append(activeCompanies, enemy.ID)
		}
	}

	// Game over if only one company left
	if len(activeCompanies) <= 1 {
		m.gameOver = true
		m.state = StateGameOver

		if len(activeCompanies) == 1 {
			if activeCompanies[0] == "player" {
				m.winner = "player"
			} else {
				m.winner = activeCompanies[0]
			}
		}
		return
	}

	// Player eliminated = lose (even if enemies remain)
	if !m.board.FleetHealthyByID("player") {
		m.gameOver = true
		m.state = StateGameOver
		m.winner = "enemy"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// centerViewportOn centers the viewport on the given coordinates
func (m *AppModel) centerViewportOn(x, y int) {
	// Calculate target viewport position (centering the coordinate)
	targetX := x - m.viewW/2
	targetY := y - m.viewH/2

	// Clamp to board bounds
	if targetX < 0 {
		targetX = 0
	}
	if targetY < 0 {
		targetY = 0
	}
	if m.board != nil {
		if targetX > m.board.Width-m.viewW {
			targetX = m.board.Width - m.viewW
		}
		if targetY > m.board.Height-m.viewH {
			targetY = m.board.Height - m.viewH
		}
	}

	m.viewport[0] = targetX
	m.viewport[1] = targetY
}

// renderPodView displays detailed pod information for the selected service
func (m AppModel) renderPodView() string {
	if m.podViewCompany == nil || len(m.podViewCompany.Services) == 0 {
		return m.styles.App.Render("No services to display")
	}

	title := m.styles.Title.Render(fmt.Sprintf("POD VIEW - %s", m.podViewCompany.Name))

	// Service list on left
	var serviceList string
	for i, svc := range m.podViewCompany.Services {
		running := 0
		pending := 0
		terminated := 0
		for _, p := range svc.Pods {
			switch p.Status {
			case game.PodRunning:
				running++
			case game.PodPending:
				pending++
			case game.PodTerminated:
				terminated++
			}
		}

		status := fmt.Sprintf("%-14s R:%d P:%d T:%d", svc.Name, running, pending, terminated)
		if i == m.podViewSvcIndex {
			serviceList += m.styles.MenuItemSelected.Render("> "+status) + "\n"
		} else {
			serviceList += m.styles.MenuItem.Render("  "+status) + "\n"
		}
	}

	// Selected service details
	svc := m.podViewCompany.Services[m.podViewSvcIndex]
	detailTitle := m.styles.Subtitle.Render(fmt.Sprintf("SERVICE: %s", svc.Name))

	// Group pods by region/rack
	podsByRegion := make(map[string][]*game.Pod)
	for _, p := range svc.Pods {
		podsByRegion[p.RegionID] = append(podsByRegion[p.RegionID], p)
	}

	var podDetail string
	podDetail += fmt.Sprintf("Affinity: %s | CanFailover: %v\n", svc.Affinity, svc.CanFailover)
	podDetail += fmt.Sprintf("Criticality: %s\n\n", svc.Criticality)

	// Show pods per region
	for regionID, pods := range podsByRegion {
		// Find region info
		var regionName string
		var regionDestroyed bool
		for _, r := range m.podViewCompany.Regions {
			if r.ID == regionID {
				regionName = r.Name
				regionDestroyed = r.IsDestroyed
				break
			}
		}
		if regionName == "" {
			regionName = regionID
		}

		regionStatus := ""
		if regionDestroyed {
			regionStatus = m.styles.Error.Render(" [DESTROYED]")
		}
		podDetail += m.styles.Subtitle.Render(fmt.Sprintf("Region: %s%s\n", regionName, regionStatus))

		// Group pods by rack
		podsByRack := make(map[string][]*game.Pod)
		for _, p := range pods {
			podsByRack[p.RackID] = append(podsByRack[p.RackID], p)
		}

		for rackID, rackPods := range podsByRack {
			rackStatus := ""
			// Check if rack is destroyed (find rack in region)
			for _, r := range m.podViewCompany.Regions {
				if r.ID == regionID {
					for _, rack := range r.Racks {
						if rack.ID == rackID && rack.IsDestroyed {
							rackStatus = m.styles.Error.Render(" [HIT]")
						}
					}
				}
			}

			podDetail += fmt.Sprintf("  Rack %s%s:\n", rackID, rackStatus)
			for _, p := range rackPods {
				statusStyle := m.styles.Success
				statusChar := "+"
				switch p.Status {
				case game.PodPending:
					statusStyle = m.styles.Warning
					statusChar = "!"
				case game.PodTerminated:
					statusStyle = m.styles.Error
					statusChar = "x"
				}
				podDetail += fmt.Sprintf("    [%s] %s\n", statusStyle.Render(statusChar), p.ID)
			}
		}
		podDetail += "\n"
	}

	// Rescaling explanation
	rescaleInfo := m.styles.Subtitle.Render("RESCALING BEHAVIOR:\n")
	switch svc.Affinity {
	case game.AffinityHard:
		rescaleInfo += m.styles.Error.Render("Hard affinity: Pods CANNOT reschedule when rack fails.\n")
		rescaleInfo += "Pods will enter Pending state permanently.\n"
	case game.AffinitySoft:
		rescaleInfo += m.styles.Warning.Render("Soft affinity: Pods PREFER current rack.\n")
		rescaleInfo += "Will reschedule to another rack in same region if available.\n"
	case game.AffinitySpread:
		rescaleInfo += m.styles.Success.Render("Spread affinity: Pods spread across racks.\n")
		rescaleInfo += "Will reschedule to least loaded rack when one fails.\n"
	default:
		rescaleInfo += "No affinity: Pods reschedule freely when rack fails.\n"
	}

	if !svc.CanFailover {
		rescaleInfo += m.styles.Error.Render("\nFailover DISABLED: Service cannot migrate between regions.\n")
	}

	controls := m.styles.Muted.Render("\n[up/down] select service  [esc/p] return to battle")

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.styles.Subtitle.Render("SERVICES"),
		serviceList,
	)

	right := lipgloss.JoinVertical(lipgloss.Left,
		detailTitle,
		podDetail,
		rescaleInfo,
	)

	main := lipgloss.JoinHorizontal(lipgloss.Top,
		m.styles.Sidebar.Render(left),
		"  ",
		m.styles.BoardArea.Render(right),
	)

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			main,
			controls,
		),
	)
}

// view hierarchy selection helpers

func (m *AppModel) selectNextShip() {
	if m.playerCompany == nil || len(m.playerCompany.Regions) == 0 {
		return
	}

	// find current index
	idx := 0
	for i, r := range m.playerCompany.Regions {
		if r.ID == m.selectedShipID {
			idx = i
			break
		}
	}

	// next
	idx = (idx + 1) % len(m.playerCompany.Regions)
	m.selectedShipID = m.playerCompany.Regions[idx].ID
	m.selectedShip = m.playerCompany.Regions[idx]

	// also update selected rack to first in this ship
	if len(m.selectedShip.Racks) > 0 {
		m.selectedRackID = m.selectedShip.Racks[0].ID
		m.selectedRack = m.selectedShip.Racks[0]
	}
}

func (m *AppModel) selectPrevShip() {
	if m.playerCompany == nil || len(m.playerCompany.Regions) == 0 {
		return
	}

	idx := 0
	for i, r := range m.playerCompany.Regions {
		if r.ID == m.selectedShipID {
			idx = i
			break
		}
	}

	idx--
	if idx < 0 {
		idx = len(m.playerCompany.Regions) - 1
	}
	m.selectedShipID = m.playerCompany.Regions[idx].ID
	m.selectedShip = m.playerCompany.Regions[idx]

	if len(m.selectedShip.Racks) > 0 {
		m.selectedRackID = m.selectedShip.Racks[0].ID
		m.selectedRack = m.selectedShip.Racks[0]
	}
}

func (m *AppModel) selectNextRack() {
	if m.playerCompany == nil {
		return
	}

	// build flat list of all racks
	allRacks := make([]*game.Rack, 0)
	for _, r := range m.playerCompany.Regions {
		allRacks = append(allRacks, r.Racks...)
	}

	if len(allRacks) == 0 {
		return
	}

	idx := 0
	for i, rack := range allRacks {
		if rack.ID == m.selectedRackID {
			idx = i
			break
		}
	}

	idx = (idx + 1) % len(allRacks)
	m.selectedRackID = allRacks[idx].ID
	m.selectedRack = allRacks[idx]
}

func (m *AppModel) selectPrevRack() {
	if m.playerCompany == nil {
		return
	}

	allRacks := make([]*game.Rack, 0)
	for _, r := range m.playerCompany.Regions {
		allRacks = append(allRacks, r.Racks...)
	}

	if len(allRacks) == 0 {
		return
	}

	idx := 0
	for i, rack := range allRacks {
		if rack.ID == m.selectedRackID {
			idx = i
			break
		}
	}

	idx--
	if idx < 0 {
		idx = len(allRacks) - 1
	}
	m.selectedRackID = allRacks[idx].ID
	m.selectedRack = allRacks[idx]
}

func (m *AppModel) selectNextService() {
	if m.playerCompany == nil || len(m.playerCompany.Services) == 0 {
		return
	}

	idx := 0
	for i, svc := range m.playerCompany.Services {
		if svc.ID == m.selectedSvcID {
			idx = i
			break
		}
	}

	idx = (idx + 1) % len(m.playerCompany.Services)
	m.selectedSvcID = m.playerCompany.Services[idx].ID
	m.selectedService = m.playerCompany.Services[idx]
}

func (m *AppModel) selectPrevService() {
	if m.playerCompany == nil || len(m.playerCompany.Services) == 0 {
		return
	}

	idx := 0
	for i, svc := range m.playerCompany.Services {
		if svc.ID == m.selectedSvcID {
			idx = i
			break
		}
	}

	idx--
	if idx < 0 {
		idx = len(m.playerCompany.Services) - 1
	}
	m.selectedSvcID = m.playerCompany.Services[idx].ID
	m.selectedService = m.playerCompany.Services[idx]
}

// getAffinityIcon returns an icon representing the affinity type
func getAffinityIcon(affinity game.AffinityType) string {
	switch affinity {
	case game.AffinityHard:
		return "[!]" // critical, can't reschedule
	case game.AffinitySpread:
		return "[~]" // distributed across racks
	case game.AffinitySoft:
		return "[o]" // preferred location
	default:
		return "[-]" // no preference, flexible
	}
}

// getHoverInfo returns detailed info about what's at the cursor position
func (m *AppModel) getHoverInfo() string {
	if m.board == nil {
		return ""
	}

	cellInfo := m.board.GetCellInfo(m.cursor[0], m.cursor[1], "player")

	// Empty cell - no info to show
	if cellInfo.Empty {
		return ""
	}

	// Determine if this is player's own ship or enemy
	isOwn := cellInfo.OwnerID == "player"

	var info string
	if isOwn {
		// Full info for own ships
		status := m.styles.Success.Render("ACTIVE")
		if cellInfo.IsDestroyed {
			status = m.styles.Error.Render("DESTROYED")
		}

		info = fmt.Sprintf("[YOUR SHIP] %s | Region: %s | Rack: %s | %s",
			cellInfo.OwnerName, cellInfo.RegionName, cellInfo.RackID, status)

		// Show services with affinity icons
		if len(cellInfo.ServicesOnRack) > 0 {
			info += "\n  Services: "
			for i, svc := range cellInfo.ServicesOnRack {
				icon := getAffinityIcon(svc.Affinity)
				if i > 0 {
					info += ", "
				}
				info += fmt.Sprintf("%s%s(%d)", icon, svc.ServiceName, svc.PodCount)
			}
		}
	} else {
		// Limited info for enemy ships (fog of war)
		if m.debugMode || cellInfo.WasHit {
			// Show more info if debug mode or we've hit it
			status := m.styles.Success.Render("ACTIVE")
			if cellInfo.IsDestroyed {
				status = m.styles.Error.Render("DESTROYED")
			}

			// Critical warning for hard-affinity services
			if cellInfo.HasCritical && !cellInfo.IsDestroyed {
				status = m.styles.Error.Render("CRITICAL TARGET")
			}

			info = fmt.Sprintf("[ENEMY: %s] Region: %s | Rack: %s | %s",
				cellInfo.OwnerName, cellInfo.RegionName, cellInfo.RackID, status)

			// Show services with affinity if discovered
			if cellInfo.WasHit && len(cellInfo.ServicesOnRack) > 0 {
				info += "\n  Services: "
				for i, svc := range cellInfo.ServicesOnRack {
					icon := getAffinityIcon(svc.Affinity)
					if i > 0 {
						info += ", "
					}
					svcInfo := fmt.Sprintf("%s%s(%d/%d)", icon, svc.ServiceName, svc.RunningPods, svc.PodCount)
					if svc.Affinity == game.AffinityHard {
						svcInfo = m.styles.Error.Render(svcInfo + " CRITICAL!")
					}
					info += svcInfo
				}
			}

			if cellInfo.CanAttack {
				if cellInfo.HasCritical {
					info += m.styles.Error.Render("\n  [FIRE to destroy critical service!]")
				} else {
					info += m.styles.Warning.Render(" [FIRE to attack!]")
				}
			} else if cellInfo.WasHit {
				info += m.styles.Muted.Render(" [Already hit]")
			}
		} else {
			// Unexplored enemy cell - show only that something is there
			info = fmt.Sprintf("[ENEMY: %s] Unknown position - ", cellInfo.OwnerName)
			info += m.styles.Warning.Render("FIRE to reveal!")
		}
	}

	return info
}
