package tui

import (
	"clustership/pkg/benchmark"
	"clustership/pkg/config"
	"clustership/pkg/game"
	"clustership/pkg/hardware"
	"clustership/pkg/k8s"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// benchmarkPollMsg triggers benchmark metrics update
type benchmarkPollMsg struct{}

// doBenchmarkPoll returns a command that polls benchmark metrics
func doBenchmarkPoll() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return benchmarkPollMsg{}
	})
}

// GameState represents the current phase of the game
type GameState int

const (
	StateMenu GameState = iota
	StateCompanySelect
	StateEnemyCountSelect // select how many enemies (1-5)
	StateEnemySelect      // select which enemy companies
	StatePlacementPrompt  // ask if user wants manual placement
	StatePlacement        // auto-placement or manual placement in progress
	StateBattle
	StatePodView  // view pod details for selected service
	StateTutorial // tutorial walkthrough
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

	// hardware detection
	systemInfo *hardware.SystemInfo
	tier       hardware.PerformanceTier
	tierLimits hardware.TierLimits

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

	// Panel navigation/scrolling state
	serviceScrollOffset int // scroll offset for services list
	eventsScrollOffset  int // scroll offset for K8s events
	battleLogOffset     int // scroll offset for battle log
	fleetStatsOffset    int // scroll offset for fleet stats (when many companies)
	shipViewOffset      int // scroll offset for ship view
	rackViewOffset      int // scroll offset for rack view
	podViewPodOffset    int // scroll offset for pods within a service
	activePanel         int // which panel is active for navigation (0=board, 1=services, 2=events, 3=log)

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

	// Benchmark fields
	benchmarkRunner  *benchmark.Runner          // manages benchmark workers
	benchmarkMetrics *benchmark.MetricsSnapshot // cached metrics for display
	benchmarkMode    bool                       // true when running as benchmark

	// Info overlay and tutorial
	showInfoOverlay bool // toggle info overlay with "i" key
	tutorialStep    int  // current tutorial step (0 = not in tutorial)

	// Manual ship placement
	manualPlacement       bool     // whether manual placement is enabled
	placementRegionIndex  int      // which region is being placed
	placementVertical     bool     // true for vertical orientation
	placementCursor       [2]int   // cursor position for placement preview
	placementOccupied     map[string]bool // cells already occupied during placement

	// K8s settings and health
	k8sSettingsCursor   int  // cursor for K8s settings menu (0=toggle, 1=namespace, 2=kubeconfig)
	k8sClusterConnected bool // cached cluster connection status
	k8sPodCount         int  // total pods in namespace
	k8sHealthyPods      int  // healthy (running+ready) pods
}

// NewAppModel creates a fresh game instance
func NewAppModel() AppModel {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Default()
	}

	// Detect hardware and validate config against tier limits
	// Config.Validate() already applies tier limits internally
	cfg.Validate()

	// Cache the detected tier info for UI display
	tier := cfg.GetTier()
	limits := cfg.GetLimits()
	sysInfo := cfg.GetSystemInfo()

	return AppModel{
		state:         StateMenu,
		styles:        DefaultStyles(),
		menuItems:     []string{"New Game", "Demo", "Tutorial", "Settings", "Quit"},
		settingsItems: []string{"Board", "Ships", "Pods", "Bots", "Timing", "Kubernetes", "Save & Back"},
		companies:     game.ListCompanies(),
		cfg:           cfg,
		systemInfo:    sysInfo,
		tier:          tier,
		tierLimits:    *limits,
		viewW:         30,
		viewH:         20,
		viewLevel:     ViewMap,
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

// checkK8sHealth updates cluster health metrics
func (m *AppModel) checkK8sHealth() {
	if m.k8sClient == nil {
		m.k8sClusterConnected = false
		m.k8sPodCount = 0
		m.k8sHealthyPods = 0
		return
	}

	// Check cluster connectivity
	m.k8sClusterConnected = m.k8sClient.IsClusterAvailable()

	if m.k8sClusterConnected && m.cfg.K8sNamespace != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		pods, err := m.k8sClient.GetPodStatus(ctx, m.cfg.K8sNamespace, "app=clustership")
		if err == nil {
			m.k8sPodCount = len(pods)
			m.k8sHealthyPods = 0
			for _, p := range pods {
				if p.Status == k8s.PodRunning && p.Ready {
					m.k8sHealthyPods++
				}
			}
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
		// Determine target company from board's cell owner map
		key := fmt.Sprintf("%d,%d", x, y)
		companyID := m.board.CellOwner[key]
		if companyID != "" {
			m.deleteK8sPodForGamePod(result.HitPod, companyID)
		}
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

		// Calculate aspect ratio for adaptive layout
		aspectRatio := float64(msg.Width) / float64(msg.Height)
		isLandscape := aspectRatio > 1.5  // Wide terminal
		isPortrait := aspectRatio < 0.8   // Tall terminal

		// Reserve space for UI based on layout
		var sidebarW, headerH int
		if isLandscape {
			// Landscape: smaller sidebar, more board space
			sidebarW = 45
			headerH = 10
		} else if isPortrait {
			// Portrait: minimal sidebar, stack vertically
			sidebarW = 30
			headerH = 8
		} else {
			// Normal: balanced layout
			sidebarW = 50
			headerH = 12
		}

		availableW := msg.Width - sidebarW
		availableH := msg.Height - headerH

		if availableW < 20 {
			availableW = 20
		}
		if availableH < 10 {
			availableH = 10
		}

		// Each cell takes 2 chars (symbol + space)
		// Adjust max limits based on aspect ratio
		var maxViewW, maxViewH int
		if isLandscape {
			// Landscape: prioritize width, allow larger horizontal viewport
			maxViewW = 80
			maxViewH = 25
		} else if isPortrait {
			// Portrait: prioritize height, allow larger vertical viewport
			maxViewW = 35
			maxViewH = 50
		} else {
			// Normal: balanced
			maxViewW = 50
			maxViewH = 35
		}

		m.viewW = min(availableW/2, maxViewW)
		m.viewH = min(availableH, maxViewH)

		// If board exists, cap to board size
		if m.board != nil {
			m.viewW = min(m.viewW, m.board.Width)
			m.viewH = min(m.viewH, m.board.Height)
		}

		// Ensure cursor stays in view after resize
		m.ensureCursorInView()
		return m, nil

	case tickMsg:
		return m.handleTick()

	case k8sPollMsg:
		// Poll K8s events during battle
		m.pollK8sEvents()
		// Update health metrics
		m.checkK8sHealth()
		// Continue polling if in battle and K8s is enabled
		if m.state == StateBattle && m.cfg.EnableRealK8s && m.k8sWatcher != nil {
			return m, doK8sPoll()
		}
		return m, nil

	case benchmarkPollMsg:
		// Update benchmark metrics during battle
		if m.benchmarkRunner != nil && m.benchmarkRunner.IsRunning() {
			snapshot := m.benchmarkRunner.GetMetrics().Snapshot()
			m.benchmarkMetrics = &snapshot
			// Update game metrics
			m.benchmarkRunner.GetMetrics().GameFPS.Store(int64(1000 / max(1, m.cfg.TurnDelayMs)))
			m.benchmarkRunner.GetMetrics().BoardUpdates.Add(1)
			m.benchmarkRunner.GetMetrics().CalculateScore()
		}
		// Continue polling if benchmark is running
		if m.state == StateBattle && m.benchmarkMode && m.benchmarkRunner != nil {
			return m, doBenchmarkPoll()
		}
		return m, nil

	case tea.KeyMsg:
		// global keys first
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "i", "?":
			// Toggle info overlay (not during tutorial)
			if m.state != StateTutorial {
				m.showInfoOverlay = !m.showInfoOverlay
				return m, nil
			}
		case "esc":
			// Close info overlay first, then handle state
			if m.showInfoOverlay {
				m.showInfoOverlay = false
				return m, nil
			}
		case "q":
			// Close info overlay first
			if m.showInfoOverlay {
				m.showInfoOverlay = false
				return m, nil
			}
			if m.state == StateMenu {
				return m, tea.Quit
			}
			// Cleanup resources when leaving battle
			if m.state == StateBattle || m.state == StateGameOver {
				m.cleanupK8sResources()
				m.stopBenchmarkWorkers()
			}
			// q goes back to menu in other states
			m.state = StateMenu
			return m, nil
		}

		// If info overlay is showing, don't process other keys
		if m.showInfoOverlay {
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
		case StatePlacementPrompt:
			return m.updatePlacementPrompt(msg)
		case StatePlacement:
			return m.updatePlacement(msg)
		case StateBattle:
			return m.updateBattle(msg)
		case StatePodView:
			return m.updatePodView(msg)
		case StateTutorial:
			return m.updateTutorial(msg)
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
		case 2: // Tutorial
			m.state = StateTutorial
			m.tutorialStep = 0
		case 3: // Settings
			m.state = StateSettings
			m.settingsCursor = 0
		case 4: // Quit
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
		// Use tier-based limit instead of hardcoded cap
		limits := m.cfg.GetLimits()
		if m.maxEnemies > limits.MaxCompanies-1 {
			m.maxEnemies = limits.MaxCompanies - 1
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
			m.state = StatePlacementPrompt
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
			m.state = StatePlacementPrompt
		}
	case "esc":
		m.state = StateEnemyCountSelect
		m.selectedEnemies = nil
	}
	return m, nil
}

// updatePlacementPrompt handles the y/n prompt for manual ship placement
func (m AppModel) updatePlacementPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Enable manual placement mode
		m.manualPlacement = true
		m.placementRegionIndex = 0
		m.placementVertical = false
		m.placementOccupied = make(map[string]bool)
		boardW, boardH := m.cfg.BoardWidthInt(), m.cfg.BoardHeightInt()
		m.placementCursor = [2]int{boardW / 4, boardH / 4}
		m.state = StatePlacement
	case "n", "N", "enter", " ":
		// Auto-placement (default)
		m.manualPlacement = false
		m.state = StatePlacement
	case "esc":
		m.state = StateEnemySelect
	}
	return m, nil
}

// updatePlacement handles ship placement phase
func (m AppModel) updatePlacement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Manual placement mode
	if m.manualPlacement {
		return m.updateManualPlacement(msg)
	}

	// Auto placement mode
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
		boardW, boardH := m.cfg.BoardWidthInt(), m.cfg.BoardHeightInt()
		m.board = NewMultiBoard(boardW, boardH, allCompanies)

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
			m.ais[enemy.ID] = NewMultiAIPlayer(enemy.ID, enemy.AIStrategy, boardW, boardH, opponents)
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
			m.playerAI = NewMultiAIPlayer("player", game.AIHunter, boardW, boardH, opponents)
		}

		// Start cursor in center of board
		m.cursor = [2]int{boardW / 2, boardH / 2}
		// Calculate viewport with bounds checking to prevent negative values
		vpX := boardW/2 - m.viewW/2
		vpY := boardH/2 - m.viewH/2
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

		// Initialize benchmark mode in demo
		if m.demoMode && m.cfg.BenchmarkMode {
			m.benchmarkMode = true
			m.startBenchmarkWorkers()
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

		// Start benchmark polling if benchmark mode
		if m.benchmarkMode && m.benchmarkRunner != nil {
			cmds = append(cmds, doBenchmarkPoll())
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

// updateManualPlacement handles manual ship placement with arrow keys and rotation
func (m AppModel) updateManualPlacement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.playerCompany == nil || len(m.playerCompany.Regions) == 0 {
		return m, nil
	}

	boardW, boardH := m.cfg.BoardWidthInt(), m.cfg.BoardHeightInt()
	currentRegion := m.playerCompany.Regions[m.placementRegionIndex]

	switch msg.String() {
	case "up", "k":
		if m.placementCursor[1] > 0 {
			m.placementCursor[1]--
		}
	case "down", "j":
		if m.placementCursor[1] < boardH-1 {
			m.placementCursor[1]++
		}
	case "left", "h":
		if m.placementCursor[0] > 0 {
			m.placementCursor[0]--
		}
	case "right", "l":
		if m.placementCursor[0] < boardW-1 {
			m.placementCursor[0]++
		}
	case "r", "R":
		// Rotate orientation
		m.placementVertical = !m.placementVertical
	case "enter", " ":
		// Try to place the current region
		cells := m.getPlacementCells(currentRegion.RackCount)
		if m.canPlaceAt(cells, boardW, boardH) {
			// Place the region
			m.placePlayerRegion(currentRegion, cells)

			// Move to next region
			m.placementRegionIndex++

			// If all regions placed, start the game
			if m.placementRegionIndex >= len(m.playerCompany.Regions) {
				return m.startGameAfterManualPlacement()
			}
		}
	case "esc":
		m.state = StatePlacementPrompt
		m.manualPlacement = false
	}
	return m, nil
}

// getPlacementCells returns the cells that would be occupied by the current placement
func (m AppModel) getPlacementCells(length int) [][2]int {
	cells := make([][2]int, length)
	for i := 0; i < length; i++ {
		if m.placementVertical {
			cells[i] = [2]int{m.placementCursor[0], m.placementCursor[1] + i}
		} else {
			cells[i] = [2]int{m.placementCursor[0] + i, m.placementCursor[1]}
		}
	}
	return cells
}

// canPlaceAt checks if the cells are valid for placement
func (m AppModel) canPlaceAt(cells [][2]int, boardW, boardH int) bool {
	for _, cell := range cells {
		// Check bounds
		if cell[0] < 0 || cell[0] >= boardW || cell[1] < 0 || cell[1] >= boardH {
			return false
		}
		// Check if already occupied
		key := fmt.Sprintf("%d,%d", cell[0], cell[1])
		if m.placementOccupied[key] {
			return false
		}
	}
	return true
}

// placePlayerRegion places a region at the specified cells
func (m *AppModel) placePlayerRegion(region *game.Region, cells [][2]int) {
	// Initialize racks for this region
	region.Racks = make([]*game.Rack, len(cells))
	region.Placement = cells

	for i, cell := range cells {
		key := fmt.Sprintf("%d,%d", cell[0], cell[1])
		m.placementOccupied[key] = true

		region.Racks[i] = &game.Rack{
			ID:       fmt.Sprintf("%s-rack-%d", region.ID, i),
			RegionID: region.ID,
			Position: cell,
			Capacity: m.cfg.PodsPerRack,
			Pods:     make([]*game.Pod, 0),
		}
	}
}

// startGameAfterManualPlacement initializes the game after manual placement is complete
func (m AppModel) startGameAfterManualPlacement() (tea.Model, tea.Cmd) {
	// Build list of all companies (player already has regions placed)
	allCompanies := []*game.Company{m.playerCompany}
	m.enemyCompanies = make([]*game.Company, 0, len(m.selectedEnemies))

	for _, enemyID := range m.selectedEnemies {
		template, _ := game.LoadCompanyTemplate(enemyID)
		if template != nil {
			enemy := game.CompanyFromTemplate(template)
			enemy.AdjustToConfig(m.cfg.ShipsPerPlayer, m.cfg.RacksPerShip, m.cfg.PodsPerRack)
			m.enemyCompanies = append(m.enemyCompanies, enemy)
			allCompanies = append(allCompanies, enemy)
		}
	}

	// Legacy single enemy
	if len(m.enemyCompanies) > 0 {
		m.enemyCompany = m.enemyCompanies[0]
	}

	// Create board with manual player placement
	boardW, boardH := m.cfg.BoardWidthInt(), m.cfg.BoardHeightInt()
	m.board = NewMultiBoardWithManualPlacement(boardW, boardH, allCompanies, m.placementOccupied)

	// Build opponent list for each AI
	allIDs := make([]string, 0, len(allCompanies))
	for _, c := range allCompanies {
		allIDs = append(allIDs, c.ID)
	}

	// Create AI for each enemy
	m.ais = make(map[string]*AIPlayer)
	for _, enemy := range m.enemyCompanies {
		opponents := make([]string, 0)
		for _, id := range allIDs {
			if id != enemy.ID {
				opponents = append(opponents, id)
			}
		}
		m.ais[enemy.ID] = NewMultiAIPlayer(enemy.ID, enemy.AIStrategy, boardW, boardH, opponents)
	}

	// Legacy single AI
	if len(m.enemyCompanies) > 0 {
		m.ai = m.ais[m.enemyCompanies[0].ID]
	}

	// Build turn queue
	m.turnQueue = []string{m.playerCompany.ID}
	for _, enemy := range m.enemyCompanies {
		m.turnQueue = append(m.turnQueue, enemy.ID)
	}
	m.currentTurnIndex = 0

	// Start cursor in center of board
	m.cursor = [2]int{boardW / 2, boardH / 2}
	vpX := boardW/2 - m.viewW/2
	vpY := boardH/2 - m.viewH/2
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
			m.deployK8sResources()
			m.startK8sWatcher()
		}
	}

	m.state = StateBattle
	m.isPlayerTurn = true
	m.turn = 1

	var cmds []tea.Cmd
	if m.cfg.EnableRealK8s && m.k8sWatcher != nil {
		cmds = append(cmds, doK8sPoll())
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
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
		m.serviceScrollOffset = 0 // reset scroll when changing company
		return m, nil
	case "tab":
		// Cycle active panel: 0=board, 1=services, 2=events, 3=fleet stats
		m.activePanel = (m.activePanel + 1) % 4
		return m, nil
	case "shift+tab":
		// Cycle active panel backwards
		m.activePanel = (m.activePanel + 3) % 4
		return m, nil
	case "[":
		// Scroll active panel up
		m.scrollActivePanelUp()
		return m, nil
	case "]":
		// Scroll active panel down
		m.scrollActivePanelDown()
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
				m.ensureCursorInView()
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
				m.ensureCursorInView()
			}
		}
	case "left", "h":
		if m.viewLevel == ViewMap {
			if m.cursor[0] > 0 {
				m.cursor[0]--
				m.ensureCursorInView()
			}
		}
	case "right", "l":
		if m.viewLevel == ViewMap {
			if m.board != nil && m.cursor[0] < m.board.Width-1 {
				m.cursor[0]++
				m.ensureCursorInView()
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
			m.podViewPodOffset = 0 // reset pod scroll when changing service
		}
	case "down", "j":
		if m.podViewSvcIndex < len(m.podViewCompany.Services)-1 {
			m.podViewSvcIndex++
			m.podViewPodOffset = 0 // reset pod scroll when changing service
		}
	case "[":
		// Scroll pods up
		if m.podViewPodOffset > 0 {
			m.podViewPodOffset--
		}
	case "]":
		// Scroll pods down
		if m.podViewSvcIndex < len(m.podViewCompany.Services) {
			svc := m.podViewCompany.Services[m.podViewSvcIndex]
			maxOffset := len(svc.Pods) - 10 // 10 visible at a time
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.podViewPodOffset < maxOffset {
				m.podViewPodOffset++
			}
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
	case "up", "k":
		if m.k8sSettingsCursor > 0 {
			m.k8sSettingsCursor--
		}
	case "down", "j":
		if m.k8sSettingsCursor < 2 {
			m.k8sSettingsCursor++
		}
	case " ", "enter":
		switch m.k8sSettingsCursor {
		case 0: // Toggle EnableRealK8s
			m.cfg.EnableRealK8s = !m.cfg.EnableRealK8s
		case 1: // Cycle namespace
			namespaces := []string{"clustership", "default", "clustership-dev", "clustership-test"}
			found := false
			for i, ns := range namespaces {
				if m.cfg.K8sNamespace == ns {
					m.cfg.K8sNamespace = namespaces[(i+1)%len(namespaces)]
					found = true
					break
				}
			}
			if !found {
				m.cfg.K8sNamespace = namespaces[0]
			}
		case 2: // Cycle kubeconfig
			home, _ := os.UserHomeDir()
			paths := []string{
				filepath.Join(home, ".kube", "config"),
				filepath.Join(home, ".kube", "kind-config"),
			}
			found := false
			for i, p := range paths {
				if m.cfg.Kubeconfig == p {
					m.cfg.Kubeconfig = paths[(i+1)%len(paths)]
					found = true
					break
				}
			}
			if !found {
				m.cfg.Kubeconfig = paths[0]
			}
		}
	case "esc":
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
	var view string

	switch m.state {
	case StateMenu:
		view = m.renderMenu()
	case StateSettings:
		view = m.renderSettings()
	case StateSettingsBoard:
		view = m.renderSettingsBoard()
	case StateSettingsShips:
		view = m.renderSettingsShips()
	case StateSettingsPods:
		view = m.renderSettingsPods()
	case StateSettingsBots:
		view = m.renderSettingsBots()
	case StateSettingsTiming:
		view = m.renderSettingsTiming()
	case StateSettingsK8s:
		view = m.renderSettingsK8s()
	case StateCompanySelect:
		view = m.renderCompanySelect()
	case StateEnemyCountSelect:
		view = m.renderEnemyCountSelect()
	case StateEnemySelect:
		view = m.renderEnemySelect()
	case StatePlacementPrompt:
		view = m.renderPlacementPrompt()
	case StatePlacement:
		view = m.renderPlacement()
	case StateBattle:
		view = m.renderBattle()
	case StatePodView:
		view = m.renderPodView()
	case StateTutorial:
		view = m.renderTutorial()
	case StateGameOver:
		view = m.renderGameOver()
	default:
		view = ""
	}

	// Render info overlay on top if enabled
	if m.showInfoOverlay {
		return m.renderInfoOverlay(view)
	}

	return view
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

	hint := m.styles.Muted.Render("\n[up/down] navigate  [enter] select  [i] info  [q] quit")

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

	// Show detected hardware tier
	tierInfo := fmt.Sprintf("Hardware Tier: %s", m.tier.String())
	if m.systemInfo != nil {
		tierInfo += fmt.Sprintf(" | RAM: %dGB", m.systemInfo.TotalRAMMB/1024)
		if m.systemInfo.GPU.Available {
			tierInfo += fmt.Sprintf(" | GPU: %s", m.systemInfo.GPU.Model)
		}
	}
	tierDisplay := m.styles.Muted.Render(tierInfo)

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
		lipgloss.JoinVertical(lipgloss.Left, title, subtitle, tierDisplay, "", menu, hint),
	)
}

// renderSettingsBoard draws board size config with hardware info
func (m AppModel) renderSettingsBoard() string {
	title := m.styles.Title.Render("BOARD SETTINGS")

	// Get hardware limits
	limits := m.cfg.GetLimits()
	sys := m.cfg.GetSystemInfo()
	tier := m.cfg.GetTier()

	info := fmt.Sprintf("Width:  %d  [</>]  (max: %d)\n", m.cfg.BoardWidth, limits.MaxBoardWidth)
	info += fmt.Sprintf("Height: %d  [^/v]  (max: %d)\n", m.cfg.BoardHeight, limits.MaxBoardHeight)

	visual := fmt.Sprintf("\n%dx%d board = %d cells\n",
		m.cfg.BoardWidth, m.cfg.BoardHeight,
		m.cfg.BoardWidth*m.cfg.BoardHeight)

	// Hardware info
	hwInfo := fmt.Sprintf("\n--- HARDWARE ---\n")
	hwInfo += fmt.Sprintf("Tier: %s\n", tier.String())
	hwInfo += fmt.Sprintf("CPU: %d cores\n", sys.CPUCores)
	hwInfo += fmt.Sprintf("RAM: %d MB\n", sys.TotalRAMMB)
	if sys.GPU.Available {
		hwInfo += fmt.Sprintf("GPU: %s (%d MB)\n", sys.GPU.Model, sys.GPU.MemoryMB)
	} else {
		hwInfo += "GPU: None detected\n"
	}

	hint := m.styles.Muted.Render("\n[arrows] adjust  [enter/esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", info, visual, hwInfo, hint),
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

	// Build menu items
	enabled := "OFF"
	if m.cfg.EnableRealK8s {
		enabled = "ON"
	}

	items := []string{
		fmt.Sprintf("Real K8s:   %s", enabled),
		fmt.Sprintf("Namespace:  %s", m.cfg.K8sNamespace),
		fmt.Sprintf("Kubeconfig: %s", m.cfg.Kubeconfig),
	}

	var content string
	for i, item := range items {
		if i == m.k8sSettingsCursor {
			content += m.styles.MenuItemSelected.Render("> " + item) + "\n"
		} else {
			content += m.styles.MenuItem.Render("  " + item) + "\n"
		}
	}

	// Cluster status indicator
	status := "\nCluster Status: "
	if m.k8sClient != nil && m.k8sClusterConnected {
		status += m.styles.Success.Render("Connected")
		if m.k8sPodCount > 0 {
			status += fmt.Sprintf(" (%d/%d pods healthy)", m.k8sHealthyPods, m.k8sPodCount)
		}
	} else if m.cfg.EnableRealK8s {
		status += m.styles.Warning.Render("Not Connected (will connect on game start)")
	} else {
		status += m.styles.Muted.Render("Disabled")
	}

	visual := "\n\nWhen enabled, game deploys real pods to your cluster.\n"
	visual += "Attacks delete pods. Requires local k8s (kind/minikube).\n"

	hint := m.styles.Muted.Render("\n[up/down] select  [space/enter] change  [esc] back  [i] info")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", content, status, visual, hint),
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

// renderPlacementPrompt draws the manual placement prompt
func (m AppModel) renderPlacementPrompt() string {
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
	}

	prompt := m.styles.Subtitle.Render("\nWould you like to manually place your ships?")
	options := "\n  [Y] Yes - Place ships manually\n  [N] No - Auto-deploy (default)"

	hint := m.styles.Muted.Render("\n[y/n] choose  [esc] back")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			info,
			prompt,
			options,
			hint,
		),
	)
}

// renderPlacement draws the placement phase
func (m AppModel) renderPlacement() string {
	// Manual placement mode
	if m.manualPlacement {
		return m.renderManualPlacement()
	}

	// Auto placement prompt
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

// renderManualPlacement draws the manual ship placement screen
func (m AppModel) renderManualPlacement() string {
	if m.playerCompany == nil || len(m.playerCompany.Regions) == 0 {
		return "No regions to place"
	}

	boardW, boardH := m.cfg.BoardWidthInt(), m.cfg.BoardHeightInt()
	currentRegion := m.playerCompany.Regions[m.placementRegionIndex]

	title := m.styles.Title.Render("MANUAL SHIP PLACEMENT")
	progress := m.styles.Subtitle.Render(fmt.Sprintf("Placing region %d/%d: %s (%d racks)",
		m.placementRegionIndex+1, len(m.playerCompany.Regions), currentRegion.Name, currentRegion.RackCount))

	orientation := "Horizontal"
	if m.placementVertical {
		orientation = "Vertical"
	}
	orientInfo := fmt.Sprintf("Orientation: %s  |  Position: (%d, %d)",
		orientation, m.placementCursor[0], m.placementCursor[1])

	// Draw a mini board preview
	previewSize := 20
	startX := m.placementCursor[0] - previewSize/2
	startY := m.placementCursor[1] - previewSize/2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}
	if startX+previewSize > boardW {
		startX = boardW - previewSize
	}
	if startY+previewSize > boardH {
		startY = boardH - previewSize
	}

	// Get cells for current placement preview
	previewCells := m.getPlacementCells(currentRegion.RackCount)
	canPlace := m.canPlaceAt(previewCells, boardW, boardH)

	// Build preview grid
	var grid string
	for y := startY; y < startY+previewSize && y < boardH; y++ {
		var row string
		for x := startX; x < startX+previewSize && x < boardW; x++ {
			key := fmt.Sprintf("%d,%d", x, y)
			cell := SymWater

			// Check if this cell is occupied by a placed region
			if m.placementOccupied[key] {
				cell = m.styles.Ship.Render(SymShip)
			} else {
				// Check if this cell is part of the preview
				isPreview := false
				for _, pc := range previewCells {
					if pc[0] == x && pc[1] == y {
						isPreview = true
						break
					}
				}
				if isPreview {
					if canPlace {
						cell = m.styles.Success.Render("#")
					} else {
						cell = m.styles.Error.Render("X")
					}
				} else {
					cell = m.styles.Water.Render(SymWater)
				}
			}

			// Highlight cursor position
			if x == m.placementCursor[0] && y == m.placementCursor[1] {
				cell = m.styles.Cursor.Render(cell)
			}

			row += cell + " "
		}
		grid += row + "\n"
	}

	// Placement status
	status := ""
	if canPlace {
		status = m.styles.Success.Render("Valid placement - press [enter] to place")
	} else {
		status = m.styles.Error.Render("Invalid placement - out of bounds or overlapping")
	}

	hint := m.styles.Muted.Render("\n[arrows] move  [r] rotate  [enter] place  [esc] cancel")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			progress,
			orientInfo,
			"",
			grid,
			status,
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
	modeIndicator := ""
	if m.benchmarkMode {
		modeIndicator = " [BENCHMARK]"
	}
	header := m.styles.Header.Render(
		fmt.Sprintf("CLUSTERSHIP - %s vs %s   %s%s%s",
			m.playerCompany.Name, m.enemyCompany.Name, turnInfo, viewIndicator, modeIndicator),
	)

	// board
	board := m.renderSimpleBoard()

	// sidebar with service status and events
	sidebar := m.renderServiceStatus()

	// Add benchmark metrics panel if in benchmark mode
	if m.benchmarkMode && m.benchmarkMetrics != nil {
		benchPanel := m.renderBenchmarkMetrics()
		sidebar = lipgloss.JoinVertical(lipgloss.Left, sidebar, "\n", benchPanel)
	}

	// Add K8s health panel if K8s is enabled
	if m.cfg.EnableRealK8s {
		k8sPanel := m.renderK8sHealthPanel()
		sidebar = lipgloss.JoinVertical(lipgloss.Left, sidebar, "\n", k8sPanel)
	}

	// combine board and sidebar
	main := lipgloss.JoinHorizontal(lipgloss.Top, board, "  ", sidebar)

	// status line with last message
	var statusLine string
	if m.lastMessage != "" {
		statusLine = m.styles.Warning.Render(m.lastMessage) + "\n"
	}

	// footer with controls
	footer := m.styles.Footer.Render(
		"[1-5] view level  [arrows] move  [enter] fire  [c] cycle  [i] info  [q] quit",
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

	regionCount := len(m.playerCompany.Regions)
	scrollInfo := fmt.Sprintf(" (%d regions)", regionCount)
	content := m.styles.Subtitle.Render("YOUR REGIONS (SHIPS):") + m.styles.Muted.Render(scrollInfo) + "\n\n"

	// Calculate visible regions with scrolling
	maxVisible := 6
	startIdx := m.shipViewOffset
	if startIdx > regionCount-maxVisible {
		startIdx = regionCount - maxVisible
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + maxVisible
	if endIdx > regionCount {
		endIdx = regionCount
	}

	// Scroll indicator (up)
	if startIdx > 0 {
		content += m.styles.Muted.Render("  ▲ more regions above\n")
	}

	for i := startIdx; i < endIdx; i++ {
		region := m.playerCompany.Regions[i]
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

		// show racks if selected (with scrolling for many racks)
		if region.ID == m.selectedShipID {
			rackCount := len(region.Racks)
			maxRacksVisible := 4
			rackStart := m.rackViewOffset
			if rackStart > rackCount-maxRacksVisible {
				rackStart = rackCount - maxRacksVisible
			}
			if rackStart < 0 {
				rackStart = 0
			}
			rackEnd := rackStart + maxRacksVisible
			if rackEnd > rackCount {
				rackEnd = rackCount
			}

			if rackStart > 0 {
				content += m.styles.Muted.Render("    ▲ more racks\n")
			}
			for ri := rackStart; ri < rackEnd; ri++ {
				rack := region.Racks[ri]
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
			if rackEnd < rackCount {
				content += m.styles.Muted.Render("    ▼ more racks\n")
			}
		}
	}

	// Scroll indicator (down)
	if endIdx < regionCount {
		content += m.styles.Muted.Render("  ▼ more regions below\n")
	}

	hint := m.styles.Muted.Render("\n[up/down] select ship  [[/]] scroll  [1-5] change view  [i] info  [q] quit")

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

		podCount := len(rack.Pods)
		content += m.styles.Subtitle.Render(fmt.Sprintf("PODS (%d):", podCount)) + "\n"

		// Scrolling for pods within rack
		maxPodsVisible := 5
		podStart := m.podViewPodOffset
		if podStart > podCount-maxPodsVisible {
			podStart = podCount - maxPodsVisible
		}
		if podStart < 0 {
			podStart = 0
		}
		podEnd := podStart + maxPodsVisible
		if podEnd > podCount {
			podEnd = podCount
		}

		if podStart > 0 {
			content += m.styles.Muted.Render("  ▲ more pods\n")
		}
		for pi := podStart; pi < podEnd; pi++ {
			pod := rack.Pods[pi]
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
		if podEnd < podCount {
			content += m.styles.Muted.Render("  ▼ more pods\n")
		}
	} else {
		content = m.styles.Muted.Render("No rack selected. Press 2 to select a ship first.")
	}

	// Count total racks for scrolling
	var allRacks []*game.Rack
	for _, region := range m.playerCompany.Regions {
		allRacks = append(allRacks, region.Racks...)
	}
	rackCount := len(allRacks)

	content += "\n" + m.styles.Subtitle.Render(fmt.Sprintf("ALL RACKS (%d):", rackCount)) + "\n"

	// Scrolling for all racks list
	maxRacksVisible := 8
	rackStart := m.rackViewOffset
	if rackStart > rackCount-maxRacksVisible {
		rackStart = rackCount - maxRacksVisible
	}
	if rackStart < 0 {
		rackStart = 0
	}
	rackEnd := rackStart + maxRacksVisible
	if rackEnd > rackCount {
		rackEnd = rackCount
	}

	if rackStart > 0 {
		content += m.styles.Muted.Render("  ▲ more racks above\n")
	}
	for ri := rackStart; ri < rackEnd; ri++ {
		rack := allRacks[ri]
		selected := "  "
		if rack.ID == m.selectedRackID {
			selected = "> "
		}
		status := "OK"
		statusStyle := m.styles.Success
		if rack.IsDestroyed {
			status = "HIT"
			statusStyle = m.styles.Error
		}
		content += fmt.Sprintf("%s[%s] %s\n", selected, statusStyle.Render(status), rack.ID)
	}
	if rackEnd < rackCount {
		content += m.styles.Muted.Render("  ▼ more racks below\n")
	}

	hint := m.styles.Muted.Render("\n[up/down] select rack  [[/]] scroll  [1-5] change view  [i] info  [q] quit")

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

	hint := m.styles.Muted.Render("\n[up/down] select service  [1-5] change view  [i] info  [q] quit")

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

	hint := m.styles.Muted.Render("[1-5] change view  [i] info  [q] quit")

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

	// legend with company colors
	legend := fmt.Sprintf("%s=Water  %s=Miss  %s=Hit  %s=Yours  %s=Destroyed  %s=Sunk",
		m.styles.Water.Render(SymWater),
		m.styles.Miss.Render(SymMiss),
		m.styles.Hit.Render(SymHit),
		CompanyStyle("player").Render(SymShip),
		m.styles.Destroyed.Render(SymDestroyed),
		m.styles.Sunk.Render(SymSunk))

	// add enemy company colors to legend
	for _, enemy := range m.enemyCompanies {
		legend += fmt.Sprintf("  %s=%s", CompanyStyle(enemy.ID).Render(SymShip), enemy.Name)
	}

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

	// Check if this cell is in a completely destroyed region (sunk battleship)
	isRegionDestroyed := m.board.IsRegionDestroyedAt(x, y)

	// show hits/misses first
	if playerState == CellHit || playerState == CellDestroyed {
		if isRegionDestroyed {
			return m.styles.Sunk.Render(SymSunk)
		}
		if playerState == CellDestroyed {
			return m.styles.Destroyed.Render(SymDestroyed)
		}
		return m.styles.Hit.Render(SymHit)
	}

	if playerState == CellMiss {
		return m.styles.Miss.Render(SymMiss)
	}

	if enemyState == CellHit || enemyState == CellDestroyed {
		if isRegionDestroyed {
			return m.styles.Sunk.Render(SymSunk)
		}
		if enemyState == CellDestroyed {
			return m.styles.Destroyed.Render(SymDestroyed)
		}
		return m.styles.Hit.Render(SymHit)
	}

	if enemyState == CellMiss {
		return m.styles.Miss.Render(SymMiss)
	}

	// show player ships with player color
	if enemyState == CellShip {
		// If region is destroyed, show sunk symbol
		if isRegionDestroyed {
			return m.styles.Sunk.Render(SymSunk)
		}
		return CompanyStyle("player").Render(SymShip)
	}

	// show enemy ships in debug mode with their company color
	if m.debugMode {
		ownerID := m.board.GetCellOwner(x, y)
		if ownerID != "" && ownerID != "player" {
			// If region is destroyed, show sunk symbol
			if isRegionDestroyed {
				return m.styles.Sunk.Render(SymSunk)
			}
			return CompanyStyle(ownerID).Render(SymShip)
		}
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

	// service status section with navigation hint and active indicator
	navHint := fmt.Sprintf(" [C] %d/%d", m.serviceViewIndex+1, totalCompanies)
	activeIndicator := ""
	if m.activePanel == 1 {
		activeIndicator = " [*]"
	}
	serviceTitle := m.styles.Subtitle.Render(titleLabel) + m.styles.Muted.Render(navHint) + m.styles.Success.Render(activeIndicator)

	// Calculate visible services with scrolling
	maxVisible := 5
	serviceCount := len(viewCompany.Services)
	startIdx := m.serviceScrollOffset
	if startIdx > serviceCount-maxVisible {
		startIdx = serviceCount - maxVisible
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + maxVisible
	if endIdx > serviceCount {
		endIdx = serviceCount
	}

	var services string
	// Scroll indicator (up)
	if startIdx > 0 {
		services += m.styles.Muted.Render("  ▲ more above\n")
	}

	for i := startIdx; i < endIdx; i++ {
		svc := viewCompany.Services[i]
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

	// Scroll indicator (down)
	if endIdx < serviceCount {
		services += m.styles.Muted.Render("  ▼ more below\n")
	}

	// events section with active indicator
	eventsActiveIndicator := ""
	if m.activePanel == 2 {
		eventsActiveIndicator = " [*]"
	}
	eventTitle := m.styles.Subtitle.Render("K8S EVENTS") + m.styles.Success.Render(eventsActiveIndicator)

	var events string
	if m.board != nil && len(m.board.Events) > 0 {
		// Calculate visible events with scrolling
		eventCount := len(m.board.Events)
		evStartIdx := m.eventsScrollOffset
		if evStartIdx > eventCount-5 {
			evStartIdx = eventCount - 5
		}
		if evStartIdx < 0 {
			evStartIdx = 0
		}
		evEndIdx := evStartIdx + 5
		if evEndIdx > eventCount {
			evEndIdx = eventCount
		}

		// Scroll indicator (up)
		if evStartIdx > 0 {
			events += m.styles.Muted.Render("▲ more\n")
		}

		for i := evStartIdx; i < evEndIdx; i++ {
			evt := m.board.Events[i]
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

		// Scroll indicator (down)
		if evEndIdx < eventCount {
			events += m.styles.Muted.Render("▼ more\n")
		}
	} else {
		events = m.styles.Muted.Render("No events yet...")
	}

	// fleet stats / battle log with active indicator
	logActiveIndicator := ""
	if m.activePanel == 3 {
		logActiveIndicator = " [*]"
	}
	statsTitle := m.styles.Subtitle.Render("FLEET STATUS") + m.styles.Success.Render(logActiveIndicator)

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

	// Battle log with scrolling
	var battleLogStr string
	if len(m.battleLog) > 0 {
		battleLogStr = "\n" + m.styles.Muted.Render("BATTLE LOG") + "\n"
		logCount := len(m.battleLog)
		logStartIdx := m.battleLogOffset
		if logStartIdx > logCount-3 {
			logStartIdx = logCount - 3
		}
		if logStartIdx < 0 {
			logStartIdx = 0
		}
		logEndIdx := logStartIdx + 3
		if logEndIdx > logCount {
			logEndIdx = logCount
		}

		if logStartIdx > 0 {
			battleLogStr += m.styles.Muted.Render("▲\n")
		}
		for i := logStartIdx; i < logEndIdx; i++ {
			msg := m.battleLog[i]
			if len(msg) > 28 {
				msg = msg[:28] + ".."
			}
			battleLogStr += m.styles.Muted.Render(msg) + "\n"
		}
		if logEndIdx < logCount {
			battleLogStr += m.styles.Muted.Render("▼\n")
		}
	}

	// affinity legend
	legendTitle := m.styles.Subtitle.Render("AFFINITY LEGEND")
	legend := m.styles.Muted.Render("[!]=critical  [~]=spread\n[o]=preferred [-]=flexible")

	// K8s status (only if real K8s is enabled)
	k8sStatus := m.renderK8sStatus()
	k8sEvents := m.renderK8sEvents()

	// Panel navigation hint
	panelHint := m.styles.Muted.Render(fmt.Sprintf("\n[tab] panel:%s [/] scroll", m.getPanelName()))

	return m.styles.Sidebar.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			serviceTitle, services,
			"",
			eventTitle, events,
			"",
			statsTitle, m.styles.Muted.Render(stats),
			battleLogStr,
			"",
			legendTitle, legend,
			"",
			k8sStatus, k8sEvents,
			panelHint,
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

// scrollActivePanelUp scrolls the active panel up
func (m *AppModel) scrollActivePanelUp() {
	// Handle view-level scrolling for ship/rack/pod views
	switch m.viewLevel {
	case ViewShip:
		if m.shipViewOffset > 0 {
			m.shipViewOffset--
		}
		return
	case ViewRack:
		if m.rackViewOffset > 0 {
			m.rackViewOffset--
		}
		return
	case ViewRackLayout:
		if m.podViewPodOffset > 0 {
			m.podViewPodOffset--
		}
		return
	}

	// Handle panel scrolling for battle view
	switch m.activePanel {
	case 1: // services panel
		if m.serviceScrollOffset > 0 {
			m.serviceScrollOffset--
		}
	case 2: // events panel
		if m.eventsScrollOffset > 0 {
			m.eventsScrollOffset--
		}
	case 3: // fleet stats / battle log
		if m.battleLogOffset > 0 {
			m.battleLogOffset--
		}
	}
}

// scrollActivePanelDown scrolls the active panel down
func (m *AppModel) scrollActivePanelDown() {
	// Handle view-level scrolling for ship/rack/pod views
	switch m.viewLevel {
	case ViewShip:
		if m.playerCompany != nil {
			maxOffset := len(m.playerCompany.Regions) - 6 // 6 visible at a time
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.shipViewOffset < maxOffset {
				m.shipViewOffset++
			}
		}
		return
	case ViewRack:
		// Count total racks
		var rackCount int
		if m.playerCompany != nil {
			for _, region := range m.playerCompany.Regions {
				rackCount += len(region.Racks)
			}
		}
		maxOffset := rackCount - 8 // 8 visible at a time
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.rackViewOffset < maxOffset {
			m.rackViewOffset++
		}
		return
	case ViewRackLayout:
		// Handle pod view scrolling
		if m.podViewCompany != nil && m.podViewSvcIndex < len(m.podViewCompany.Services) {
			svc := m.podViewCompany.Services[m.podViewSvcIndex]
			maxOffset := len(svc.Pods) - 10 // 10 visible at a time
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.podViewPodOffset < maxOffset {
				m.podViewPodOffset++
			}
		}
		return
	}

	// Handle panel scrolling for battle view
	switch m.activePanel {
	case 1: // services panel
		// Get current company's service count
		var serviceCount int
		if m.serviceViewIndex == 0 && m.playerCompany != nil {
			serviceCount = len(m.playerCompany.Services)
		} else if m.serviceViewIndex > 0 && m.serviceViewIndex <= len(m.enemyCompanies) {
			serviceCount = len(m.enemyCompanies[m.serviceViewIndex-1].Services)
		}
		maxOffset := serviceCount - 5 // show 5 at a time
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.serviceScrollOffset < maxOffset {
			m.serviceScrollOffset++
		}
	case 2: // events panel
		if m.board != nil {
			maxOffset := len(m.board.Events) - 5
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.eventsScrollOffset < maxOffset {
				m.eventsScrollOffset++
			}
		}
	case 3: // battle log
		maxOffset := len(m.battleLog) - 5
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.battleLogOffset < maxOffset {
			m.battleLogOffset++
		}
	}
}

// getPanelName returns the name of the active panel for display
func (m AppModel) getPanelName() string {
	switch m.activePanel {
	case 0:
		return "BOARD"
	case 1:
		return "SERVICES"
	case 2:
		return "EVENTS"
	case 3:
		return "LOG"
	default:
		return "BOARD"
	}
}

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
		maxX := m.board.Width - m.viewW
		maxY := m.board.Height - m.viewH
		if maxX < 0 {
			maxX = 0
		}
		if maxY < 0 {
			maxY = 0
		}
		if targetX > maxX {
			targetX = maxX
		}
		if targetY > maxY {
			targetY = maxY
		}
	}

	m.viewport[0] = targetX
	m.viewport[1] = targetY
}

// ensureCursorInView adjusts viewport to keep cursor visible with margin
func (m *AppModel) ensureCursorInView() {
	if m.board == nil {
		return
	}

	margin := 2 // cells of margin before scrolling

	// Check if cursor is outside viewport bounds
	cursorX, cursorY := m.cursor[0], m.cursor[1]

	// Scroll left if cursor is near left edge
	if cursorX < m.viewport[0]+margin {
		m.viewport[0] = cursorX - margin
		if m.viewport[0] < 0 {
			m.viewport[0] = 0
		}
	}

	// Scroll right if cursor is near right edge
	if cursorX >= m.viewport[0]+m.viewW-margin {
		m.viewport[0] = cursorX - m.viewW + margin + 1
		maxX := m.board.Width - m.viewW
		if maxX < 0 {
			maxX = 0
		}
		if m.viewport[0] > maxX {
			m.viewport[0] = maxX
		}
	}

	// Scroll up if cursor is near top edge
	if cursorY < m.viewport[1]+margin {
		m.viewport[1] = cursorY - margin
		if m.viewport[1] < 0 {
			m.viewport[1] = 0
		}
	}

	// Scroll down if cursor is near bottom edge
	if cursorY >= m.viewport[1]+m.viewH-margin {
		m.viewport[1] = cursorY - m.viewH + margin + 1
		maxY := m.board.Height - m.viewH
		if maxY < 0 {
			maxY = 0
		}
		if m.viewport[1] > maxY {
			m.viewport[1] = maxY
		}
	}
}

// renderPodView displays detailed pod information for the selected service
func (m AppModel) renderPodView() string {
	if m.podViewCompany == nil || len(m.podViewCompany.Services) == 0 {
		return m.styles.App.Render("No services to display")
	}

	title := m.styles.Title.Render(fmt.Sprintf("POD VIEW - %s", m.podViewCompany.Name))

	// Service list on left with scrolling
	serviceCount := len(m.podViewCompany.Services)
	maxServicesVisible := 8
	svcStart := m.serviceScrollOffset
	if svcStart > serviceCount-maxServicesVisible {
		svcStart = serviceCount - maxServicesVisible
	}
	if svcStart < 0 {
		svcStart = 0
	}
	svcEnd := svcStart + maxServicesVisible
	if svcEnd > serviceCount {
		svcEnd = serviceCount
	}

	var serviceList string
	if svcStart > 0 {
		serviceList += m.styles.Muted.Render("  ▲ more services\n")
	}
	for i := svcStart; i < svcEnd; i++ {
		svc := m.podViewCompany.Services[i]
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

		name := svc.Name
		if len(name) > 12 {
			name = name[:12]
		}
		status := fmt.Sprintf("%-12s R:%d P:%d T:%d", name, running, pending, terminated)
		if i == m.podViewSvcIndex {
			serviceList += m.styles.MenuItemSelected.Render("> "+status) + "\n"
		} else {
			serviceList += m.styles.MenuItem.Render("  "+status) + "\n"
		}
	}
	if svcEnd < serviceCount {
		serviceList += m.styles.Muted.Render("  ▼ more services\n")
	}

	// Selected service details
	svc := m.podViewCompany.Services[m.podViewSvcIndex]
	detailTitle := m.styles.Subtitle.Render(fmt.Sprintf("SERVICE: %s (%d pods)", svc.Name, len(svc.Pods)))

	// Group pods by region/rack for organized display
	podsByRegion := make(map[string][]*game.Pod)
	var regionOrder []string // maintain order
	for _, p := range svc.Pods {
		if _, exists := podsByRegion[p.RegionID]; !exists {
			regionOrder = append(regionOrder, p.RegionID)
		}
		podsByRegion[p.RegionID] = append(podsByRegion[p.RegionID], p)
	}

	var podDetail string
	podDetail += fmt.Sprintf("Affinity: %s | CanFailover: %v\n", svc.Affinity, svc.CanFailover)
	podDetail += fmt.Sprintf("Criticality: %s\n\n", svc.Criticality)

	// Show pods per region with scrolling
	totalPods := len(svc.Pods)
	podDetail += m.styles.Subtitle.Render(fmt.Sprintf("PODS (%d total):\n", totalPods))

	// Flatten pods for scrolling
	var allPods []*game.Pod
	for _, regionID := range regionOrder {
		allPods = append(allPods, podsByRegion[regionID]...)
	}

	maxPodsVisible := 10
	podStart := m.podViewPodOffset
	if podStart > len(allPods)-maxPodsVisible {
		podStart = len(allPods) - maxPodsVisible
	}
	if podStart < 0 {
		podStart = 0
	}
	podEnd := podStart + maxPodsVisible
	if podEnd > len(allPods) {
		podEnd = len(allPods)
	}

	if podStart > 0 {
		podDetail += m.styles.Muted.Render("  ▲ more pods above\n")
	}

	currentRegion := ""
	for pi := podStart; pi < podEnd; pi++ {
		p := allPods[pi]

		// Show region header when region changes
		if p.RegionID != currentRegion {
			currentRegion = p.RegionID
			var regionName string
			var regionDestroyed bool
			for _, r := range m.podViewCompany.Regions {
				if r.ID == p.RegionID {
					regionName = r.Name
					regionDestroyed = r.IsDestroyed
					break
				}
			}
			if regionName == "" {
				regionName = p.RegionID
			}
			regionStatus := ""
			if regionDestroyed {
				regionStatus = m.styles.Error.Render(" [DESTROYED]")
			}
			podDetail += m.styles.Muted.Render(fmt.Sprintf("  -- %s%s --\n", regionName, regionStatus))
		}

		// Check rack status
		rackHit := ""
		for _, r := range m.podViewCompany.Regions {
			if r.ID == p.RegionID {
				for _, rack := range r.Racks {
					if rack.ID == p.RackID && rack.IsDestroyed {
						rackHit = "[HIT]"
					}
				}
			}
		}

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

		podID := p.ID
		if len(podID) > 20 {
			podID = podID[:20] + "..."
		}
		podDetail += fmt.Sprintf("  [%s] %s %s\n", statusStyle.Render(statusChar), podID, m.styles.Error.Render(rackHit))
	}

	if podEnd < len(allPods) {
		podDetail += m.styles.Muted.Render("  ▼ more pods below\n")
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

	controls := m.styles.Muted.Render("\n[up/down] select service  [[/]] scroll pods  [esc/p] return to battle")

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

// startBenchmarkWorkers initializes and starts benchmark workers for all companies
func (m *AppModel) startBenchmarkWorkers() {
	m.benchmarkRunner = benchmark.NewRunner()
	m.benchmarkRunner.Start()

	// Create workers for player company services
	if m.playerCompany != nil {
		for _, svc := range m.playerCompany.Services {
			// Map service type to workload type
			wtype := benchmark.WorkloadCPU
			if svc.Criticality == "compute" {
				wtype = benchmark.WorkloadMemory
			} else if svc.Criticality == "network" || svc.Criticality == "latency-critical" {
				wtype = benchmark.WorkloadNetwork
			}
			// Add workers based on pod count
			for i := 0; i < svc.PodsPerReplica; i++ {
				m.benchmarkRunner.AddWorker(m.playerCompany.ID, svc.ID, wtype)
			}
		}
	}

	// Create workers for each enemy company
	for _, enemy := range m.enemyCompanies {
		for _, svc := range enemy.Services {
			wtype := benchmark.WorkloadCPU
			if svc.Criticality == "compute" {
				wtype = benchmark.WorkloadMemory
			} else if svc.Criticality == "network" || svc.Criticality == "latency-critical" {
				wtype = benchmark.WorkloadNetwork
			}
			for i := 0; i < svc.PodsPerReplica; i++ {
				m.benchmarkRunner.AddWorker(enemy.ID, svc.ID, wtype)
			}
		}
	}

	m.addBattleLog(fmt.Sprintf("Benchmark started with %d workers", m.benchmarkRunner.GetWorkerCount()))
}

// stopBenchmarkWorkers stops all benchmark workers
func (m *AppModel) stopBenchmarkWorkers() {
	if m.benchmarkRunner != nil {
		m.benchmarkRunner.Stop()
		m.benchmarkRunner = nil
	}
	m.benchmarkMode = false
}

// renderBenchmarkMetrics renders the benchmark metrics panel
func (m AppModel) renderBenchmarkMetrics() string {
	if !m.benchmarkMode || m.benchmarkMetrics == nil {
		return ""
	}

	metrics := m.benchmarkMetrics

	// Build metrics display
	var s string
	s += m.styles.Subtitle.Render("BENCHMARK METRICS") + "\n\n"

	// Performance section
	s += m.styles.Title.Render("Performance") + "\n"
	s += fmt.Sprintf("  Ops/sec:     %s\n", benchmark.FormatOps(metrics.OpsPerSec))
	s += fmt.Sprintf("  Throughput:  %s\n", benchmark.FormatBytes(metrics.BytesPerSec))
	s += fmt.Sprintf("  Workers:     %d\n", metrics.WorkerCount)
	s += fmt.Sprintf("  Duration:    %s\n", metrics.Duration.Round(time.Second))
	s += "\n"

	// Latency section
	s += m.styles.Title.Render("Latency") + "\n"
	s += fmt.Sprintf("  P50:  %s\n", benchmark.FormatLatency(metrics.LatencyP50))
	s += fmt.Sprintf("  P95:  %s\n", benchmark.FormatLatency(metrics.LatencyP95))
	s += fmt.Sprintf("  P99:  %s\n", benchmark.FormatLatency(metrics.LatencyP99))
	s += fmt.Sprintf("  Max:  %s\n", benchmark.FormatLatency(metrics.LatencyMax))
	s += "\n"

	// Hardware section
	s += m.styles.Title.Render("Hardware") + "\n"
	cpuBar := renderBar(int(metrics.CPUPercent), 100, 10)
	memBar := renderBar(int(metrics.MemUsedMB), 1024, 10)
	s += fmt.Sprintf("  CPU:  %s %d%%\n", cpuBar, metrics.CPUPercent)
	s += fmt.Sprintf("  RAM:  %s %dMB\n", memBar, metrics.MemUsedMB)
	if metrics.GPUPercent > 0 {
		gpuBar := renderBar(int(metrics.GPUPercent), 100, 10)
		s += fmt.Sprintf("  GPU:  %s %d%%\n", gpuBar, metrics.GPUPercent)
		s += fmt.Sprintf("  VRAM: %dMB\n", metrics.VRAMUsedMB)
	}
	s += "\n"

	// Game metrics section
	s += m.styles.Title.Render("Game") + "\n"
	s += fmt.Sprintf("  FPS:          %d\n", metrics.GameFPS)
	s += fmt.Sprintf("  Board Upd:    %d\n", metrics.BoardUpdates)
	s += fmt.Sprintf("  AI Decisions: %d\n", metrics.AIDecisions)
	s += "\n"

	// Score
	scoreStyle := m.styles.Success
	if metrics.Score < 10000 {
		scoreStyle = m.styles.Warning
	}
	s += fmt.Sprintf("SCORE: %s\n", scoreStyle.Render(fmt.Sprintf("%d", metrics.Score)))

	return m.styles.Box.Render(s)
}

// renderK8sHealthPanel renders the K8s cluster health panel
func (m AppModel) renderK8sHealthPanel() string {
	var s string
	s += m.styles.Subtitle.Render("K8S CLUSTER") + "\n\n"

	// Connection status
	if m.k8sClusterConnected {
		s += m.styles.Success.Render("Status: Connected") + "\n"
		s += fmt.Sprintf("Namespace: %s\n", m.cfg.K8sNamespace)

		// Pod health
		if m.k8sPodCount > 0 {
			healthPct := (m.k8sHealthyPods * 100) / m.k8sPodCount
			healthBar := renderBar(m.k8sHealthyPods, m.k8sPodCount, 10)
			healthStyle := m.styles.Success
			if healthPct < 80 {
				healthStyle = m.styles.Warning
			}
			if healthPct < 50 {
				healthStyle = m.styles.Error
			}
			s += fmt.Sprintf("Pods: %s %s\n", healthBar, healthStyle.Render(fmt.Sprintf("%d/%d", m.k8sHealthyPods, m.k8sPodCount)))
		} else {
			s += m.styles.Muted.Render("Pods: 0 (deploying...)") + "\n"
		}

		// Show recent K8s events
		if len(m.k8sPodEvents) > 0 {
			s += "\n" + m.styles.Muted.Render("Recent Events:") + "\n"
			start := len(m.k8sPodEvents) - 3
			if start < 0 {
				start = 0
			}
			for _, ev := range m.k8sPodEvents[start:] {
				evStyle := m.styles.Normal
				if ev.Type == "Deleted" {
					evStyle = m.styles.Error
				} else if ev.Type == "Added" {
					evStyle = m.styles.Success
				}
				s += evStyle.Render(fmt.Sprintf("  %s %s", ev.Type, ev.Pod.Name)) + "\n"
			}
		}
	} else {
		s += m.styles.Error.Render("Status: Disconnected") + "\n"
		if m.k8sError != nil {
			s += m.styles.Muted.Render(fmt.Sprintf("Error: %v", m.k8sError)) + "\n"
		}
	}

	return m.styles.Box.Render(s)
}

// renderBar renders a simple progress bar
func renderBar(value, max, width int) string {
	if max <= 0 {
		max = 1
	}
	filled := (value * width) / max
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "#"
		} else {
			bar += "-"
		}
	}
	return "[" + bar + "]"
}
