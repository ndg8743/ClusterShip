package tui

import (
	"clustership/pkg/config"
	"clustership/pkg/game"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tickMsg is sent to animate turns
type tickMsg time.Time

// doTick returns a command that sends a tick after a delay
func doTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
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
	StateSettings      // main settings menu
	StateSettingsBoard // board width/height
	StateSettingsShips // ships, racks config
	StateSettingsPods  // pods per rack
	StateSettingsBots  // bot count, difficulty
	StateSettingsK8s   // k8s integration options
)

// ViewLevel represents the hierarchical view depth (1-4 keys)
type ViewLevel int

const (
	ViewMap  ViewLevel = 1 // overview map
	ViewShip ViewLevel = 2 // ship/region detail
	ViewRack ViewLevel = 3 // rack detail with pods
	ViewYAML ViewLevel = 4 // yaml manifest
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
		settingsItems: []string{"Board", "Ships", "Pods", "Bots", "Kubernetes", "Save & Back"},
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

	case tea.KeyMsg:
		// global keys first
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == StateMenu {
				return m, tea.Quit
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
				m.enemyCompanies = append(m.enemyCompanies, enemy)
				allCompanies = append(allCompanies, enemy)
			}
		}

		// Legacy single enemy (for backward compat)
		if len(m.enemyCompanies) > 0 {
			m.enemyCompany = m.enemyCompanies[0]
		}

		// Create multi-company board (50x50)
		m.board = NewMultiBoard(50, 50, allCompanies)

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
			m.ais[enemy.ID] = NewMultiAIPlayer(enemy.ID, enemy.AIStrategy, 50, 50, opponents)
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
			m.playerAI = NewMultiAIPlayer("player", game.AIHunter, 50, 50, opponents)
		}

		// Start cursor in center of board
		m.cursor = [2]int{25, 25}
		m.viewport = [2]int{10, 10}
		m.battleLog = make([]string, 0)

		m.state = StateBattle
		m.isPlayerTurn = true
		m.turn = 1

		// In demo mode, start auto-play
		if m.demoMode {
			return m, doTick()
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
			// Player attacks using multi-company system
			result, _ := m.board.AttackMulti(m.cursor[0], m.cursor[1], "player", "")
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
				return m, doTick()
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
			m.state = StateSettingsK8s
		case 5: // save and back
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
			// AI attacks using stored target ID from KNN selection
			targetID := m.pendingTargetID
			result, _ = m.board.AttackMulti(attack[0], attack[1], currentCompanyID, targetID)
			ai.RecordResultAgainst(attack[0], attack[1], targetID, result)
		} else {
			result, _ = m.board.AttackMulti(attack[0], attack[1], currentCompanyID, "")
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
			return m, doTick()
		}

		// Done with this company's turn
		m.animating = false
		m.checkMultiWinCondition()

		if !m.gameOver {
			m.advanceTurn()
			// If next turn is AI, continue ticking
			if !m.isPlayerTurn || m.demoMode {
				return m, doTick()
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
		result, _ := m.board.AttackMulti(target[0], target[1], "player", targetID)
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
			return m, doTick()
		}
	}

	// Start next AI turn if it's an AI's turn
	if !m.isPlayerTurn && !m.animating && !m.gameOver {
		if m.startAITurn(currentCompanyID) {
			return m, doTick()
		}
		// AI couldn't start turn (no AI for this company), skip to next
		m.advanceTurn()
		if !m.isPlayerTurn {
			return m, doTick()
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
	viewIndicator := fmt.Sprintf(" [VIEW 1/4]")
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
		"[1-4] view level  [arrows] move  [enter] fire  [c] cycle  [q] quit",
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
		fmt.Sprintf("SHIP VIEW - %s  [VIEW 2/4]", m.playerCompany.Name),
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

	hint := m.styles.Muted.Render("\n[up/down] select ship  [1-4] change view  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content, hint),
	)
}

// renderRackView shows rack detail with pods (level 3)
func (m AppModel) renderRackView() string {
	header := m.styles.Header.Render(
		fmt.Sprintf("RACK VIEW - %s  [VIEW 3/4]", m.playerCompany.Name),
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

	hint := m.styles.Muted.Render("\n[up/down] select rack  [1-4] change view  [q] quit")

	return m.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content, hint),
	)
}

// renderYAMLView shows YAML manifest (level 4, fog of war applies)
func (m AppModel) renderYAMLView() string {
	header := m.styles.Header.Render("YAML VIEW  [VIEW 4/4]")

	var content string

	// YOUR SERVICES - always fully visible
	content += m.styles.Subtitle.Render("YOUR SERVICES (full visibility):") + "\n"
	content += m.renderServiceYAMLList(m.playerCompany, true) + "\n"

	// ENEMY SERVICES - fog of war applies
	for _, enemy := range m.enemyCompanies {
		content += m.styles.Subtitle.Render(fmt.Sprintf("ENEMY: %s (fog of war):", enemy.Name)) + "\n"
		content += m.renderServiceYAMLList(enemy, false) + "\n"
	}

	hint := m.styles.Muted.Render("\n[up/down] select service  [1-4] change view  [q] quit")

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

	// legend
	legend := fmt.Sprintf("%s=Water  %s=Miss  %s=Hit  %s=Ship  %s=Destroyed",
		m.styles.Water.Render(SymWater),
		m.styles.Miss.Render(SymMiss),
		m.styles.Hit.Render(SymHit),
		m.styles.Ship.Render(SymShip),
		m.styles.Destroyed.Render(SymDestroyed))

	// controls
	controls := "[arrows] move  [WASD] pan  [enter] fire  [p] pods  [v] debug"

	return m.styles.BoardArea.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.styles.Subtitle.Render(title),
			header+grid,
			m.styles.Muted.Render(cursorInfo),
			legend,
			m.styles.Muted.Render(controls),
		),
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

		// truncate name to fit
		name := svc.Name
		if len(name) > 12 {
			name = name[:12]
		}
		status := fmt.Sprintf("%-12s %s", name, bar)
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

	return m.styles.Sidebar.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			serviceTitle, services,
			"",
			eventTitle, events,
			"",
			statsTitle, m.styles.Muted.Render(stats),
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
