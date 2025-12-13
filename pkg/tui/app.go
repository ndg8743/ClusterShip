package tui

import (
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
	return tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// GameState represents the current phase of the game
type GameState int

const (
	StateMenu GameState = iota
	StateCompanySelect
	StatePlacement
	StateBattle
	StateGameOver
)

// AppModel is the main Bubble Tea model for the game
type AppModel struct {
	state    GameState
	styles   *Styles
	width    int
	height   int

	// menu state
	menuCursor int
	menuItems  []string

	// company selection
	companies      []string
	companyCursor  int
	playerCompany  *game.Company
	enemyCompany   *game.Company

	// game state
	board        *Board
	ai           *AIPlayer // enemy AI
	playerAI     *AIPlayer // player AI for demo mode
	cursor       [2]int    // current cursor position on board
	viewport     [2]int    // viewport offset for scrolling large boards
	turn         int
	isPlayerTurn bool
	gameOver     bool
	winner       string
	lastMessage  string   // last attack result message
	battleLog    []string // recent battle messages

	// animation state
	animating      bool      // whether turn is animating
	pendingAttacks [][2]int  // attacks to animate

	// display
	showEmoji   bool
	compactMode bool
	viewW       int // viewport width in cells
	viewH       int // viewport height in cells
	demoMode    bool
	debugMode   bool // show all ships (no fog of war)
}

// NewAppModel creates a fresh game instance
func NewAppModel() AppModel {
	return AppModel{
		state:     StateMenu,
		styles:    DefaultStyles(),
		menuItems: []string{"New Game", "Demo", "Settings", "Quit"},
		companies: game.ListCompanies(),
		showEmoji: false, // ascii mode by default, easier on terminals
		viewW:     30,    // viewport size
		viewH:     20,
		battleLog: make([]string, 0),
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
		case StatePlacement:
			return m.updatePlacement(msg)
		case StateBattle:
			return m.updateBattle(msg)
		case StateGameOver:
			return m.updateGameOver(msg)
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
			m.showEmoji = !m.showEmoji // toggle emoji mode for now
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
			// just skip on error for now
			return m, nil
		}
		m.playerCompany = game.CompanyFromTemplate(template)

		// pick a random enemy (different from player)
		enemyID := m.pickRandomEnemy(m.companies[m.companyCursor])
		enemyTemplate, _ := game.LoadCompanyTemplate(enemyID)
		m.enemyCompany = game.CompanyFromTemplate(enemyTemplate)

		m.state = StatePlacement
	case "esc":
		m.state = StateMenu
	}
	return m, nil
}

// updatePlacement handles ship placement phase (auto-place for now)
func (m AppModel) updatePlacement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		// create 100x100 board with auto-placed fleets on shared ocean
		m.board = NewBoard(100, 100, m.playerCompany, m.enemyCompany)

		// create enemy AI
		strategy := game.AIHunter // default
		if m.enemyCompany != nil {
			strategy = m.enemyCompany.AIStrategy
		}
		m.ai = NewAIPlayer(strategy, 100, 100)

		// in demo mode, create player AI too
		if m.demoMode {
			m.playerAI = NewAIPlayer(game.AIHunter, 100, 100)
		}

		// start cursor in center of board
		m.cursor = [2]int{50, 50}
		m.viewport = [2]int{35, 40}
		m.battleLog = make([]string, 0)

		m.state = StateBattle
		m.isPlayerTurn = true
		m.turn = 1

		// in demo mode, start auto-play
		if m.demoMode {
			return m, doTick()
		}
	case "esc":
		m.state = StateCompanySelect
	}
	return m, nil
}

// updateBattle handles the main battle phase
func (m AppModel) updateBattle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// these keys work even during animation
	switch msg.String() {
	case "v":
		// toggle debug mode (show all ships, no fog of war)
		m.debugMode = !m.debugMode
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}

	// ignore other input during animation
	if m.animating {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor[1] > 0 {
			m.cursor[1]--
		}
	case "down", "j":
		if m.board != nil && m.cursor[1] < m.board.Height-1 {
			m.cursor[1]++
		}
	case "left", "h":
		if m.cursor[0] > 0 {
			m.cursor[0]--
		}
	case "right", "l":
		if m.board != nil && m.cursor[0] < m.board.Width-1 {
			m.cursor[0]++
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
		if m.isPlayerTurn && m.board != nil {
			result, _ := m.board.Attack(m.cursor[0], m.cursor[1], true)
			if result != nil {
				m.lastMessage = result.Message
				// log the attack
				player := m.playerCompany.Emoji + " You"
				if result.Hit {
					m.addBattleLog(fmt.Sprintf("%s hit at (%d,%d)!", player, m.cursor[0], m.cursor[1]))
				} else {
					m.addBattleLog(fmt.Sprintf("%s missed at (%d,%d)", player, m.cursor[0], m.cursor[1]))
				}
			}
			m.checkWinCondition()
			if !m.gameOver {
				// start enemy turn with animation
				m.isPlayerTurn = false
				m.startEnemyTurn()
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

// handleTick processes animation ticks
func (m AppModel) handleTick() (tea.Model, tea.Cmd) {
	if m.state != StateBattle || m.gameOver {
		return m, nil
	}

	// process enemy turn animation
	if m.animating && len(m.pendingAttacks) > 0 {
		attack := m.pendingAttacks[0]
		m.pendingAttacks = m.pendingAttacks[1:]

		result, _ := m.board.Attack(attack[0], attack[1], false)
		m.ai.RecordResult(attack[0], attack[1], result)

		// log the attack
		enemy := m.enemyCompany.Emoji + " " + m.enemyCompany.Name
		if result != nil && result.Hit {
			m.addBattleLog(fmt.Sprintf("%s hit at (%d,%d)!", enemy, attack[0], attack[1]))
		} else {
			m.addBattleLog(fmt.Sprintf("%s missed at (%d,%d)", enemy, attack[0], attack[1]))
		}

		if len(m.pendingAttacks) > 0 {
			return m, doTick()
		}

		// done with enemy turn
		m.animating = false
		m.isPlayerTurn = true
		m.turn++
		m.checkWinCondition()

		// in demo mode, auto-play player turn
		if m.demoMode && !m.gameOver {
			return m, doTick()
		}
		return m, nil
	}

	// demo mode: auto-play player turn
	if m.demoMode && m.isPlayerTurn && !m.animating && !m.gameOver && m.playerAI != nil {
		target := m.playerAI.PickTarget()
		m.cursor = target

		result, _ := m.board.Attack(target[0], target[1], true)
		m.playerAI.RecordResult(target[0], target[1], result)

		player := m.playerCompany.Emoji + " " + m.playerCompany.Name
		if result != nil && result.Hit {
			m.addBattleLog(fmt.Sprintf("%s hit at (%d,%d)!", player, target[0], target[1]))
			m.lastMessage = result.Message
		} else {
			m.addBattleLog(fmt.Sprintf("%s missed at (%d,%d)", player, target[0], target[1]))
			m.lastMessage = "Miss"
		}

		m.checkWinCondition()
		if !m.gameOver {
			m.isPlayerTurn = false
			m.startEnemyTurn()
			return m, doTick()
		}
	}

	return m, nil
}

// startEnemyTurn queues enemy attacks for animation
func (m *AppModel) startEnemyTurn() {
	if m.ai == nil {
		return
	}
	m.animating = true
	m.pendingAttacks = make([][2]int, 0)

	// queue one attack
	target := m.ai.PickTarget()
	m.pendingAttacks = append(m.pendingAttacks, target)
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
	case StateCompanySelect:
		return m.renderCompanySelect()
	case StatePlacement:
		return m.renderPlacement()
	case StateBattle:
		return m.renderBattle()
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

	hint := m.styles.Muted.Render("\n[↑/↓] navigate  [enter] select  [q] quit")
	if m.showEmoji {
		hint += m.styles.Success.Render("  (emoji mode ON)")
	}

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
		line := fmt.Sprintf("%s %s", template.Emoji, template.Name)
		if i == m.companyCursor {
			list += m.styles.MenuItemSelected.Render("> "+line) + "\n"
			// show description for selected
			list += m.styles.Muted.Render("   "+template.Description) + "\n"
		} else {
			list += m.styles.MenuItem.Render("  "+line) + "\n"
		}
	}

	hint := m.styles.Muted.Render("\n[↑/↓] navigate  [enter] select  [esc] back")

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

// renderPlacement draws the placement phase (auto-place for now)
func (m AppModel) renderPlacement() string {
	title := m.styles.Title.Render("DEPLOYMENT PHASE")

	var info string
	if m.playerCompany != nil {
		info = fmt.Sprintf("Your fleet: %s %s\n", m.playerCompany.Emoji, m.playerCompany.Name)
		info += fmt.Sprintf("Regions: %d | Total Racks: %d\n", len(m.playerCompany.Regions), m.playerCompany.TotalRacks())
	}
	if m.enemyCompany != nil {
		info += fmt.Sprintf("\nEnemy fleet: %s %s\n", m.enemyCompany.Emoji, m.enemyCompany.Name)
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

// renderBattle draws the main battle screen
func (m AppModel) renderBattle() string {
	if m.playerCompany == nil || m.enemyCompany == nil {
		return "Loading..."
	}

	// header
	turnInfo := fmt.Sprintf("Turn: %d", m.turn)
	turnInfo += " | YOUR TURN"
	header := m.styles.Header.Render(
		fmt.Sprintf("CLUSTERSHIP - %s vs %s   %s",
			m.playerCompany.Emoji, m.enemyCompany.Emoji, turnInfo),
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
		"[↑↓←→/hjkl] move  [enter] fire  [tab] toggle view  [q] quit",
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
	var legend string
	if m.showEmoji {
		legend = fmt.Sprintf("%s=Water  %s=Miss  %s=Hit  %s=Ship  %s=Destroyed",
			EmojiWater, EmojiMiss, EmojiHit, EmojiShip, EmojiDestroyed)
	} else {
		legend = fmt.Sprintf("%s=Water  %s=Miss  %s=Hit  %s=Ship  %s=Destroyed",
			m.styles.Water.Render(SymWater),
			m.styles.Miss.Render(SymMiss),
			m.styles.Hit.Render(SymHit),
			m.styles.Ship.Render(SymShip),
			m.styles.Destroyed.Render(SymDestroyed))
	}

	// controls
	controls := "[arrows] move  [WASD] pan  [enter] fire  [v] debug  [tab] zoom"

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

	// check for player shots at enemy fleet
	playerState := m.board.GetEnemyCellState(x, y)
	// check for enemy shots at player fleet
	enemyState := m.board.GetPlayerCellState(x, y)

	// priority: show hits/misses first (both sides)
	if playerState == CellHit || playerState == CellDestroyed {
		if playerState == CellDestroyed {
			if m.showEmoji {
				return EmojiDestroyed
			}
			return m.styles.Destroyed.Render(SymDestroyed)
		}
		if m.showEmoji {
			return EmojiHit
		}
		return m.styles.Hit.Render(SymHit)
	}

	if playerState == CellMiss {
		if m.showEmoji {
			return EmojiMiss
		}
		return m.styles.Miss.Render(SymMiss)
	}

	// show enemy shots at player ships
	if enemyState == CellHit || enemyState == CellDestroyed {
		if enemyState == CellDestroyed {
			if m.showEmoji {
				return EmojiDestroyed
			}
			return m.styles.Destroyed.Render(SymDestroyed)
		}
		if m.showEmoji {
			return EmojiHit
		}
		return m.styles.Hit.Render(SymHit)
	}

	if enemyState == CellMiss {
		if m.showEmoji {
			return EmojiMiss
		}
		return m.styles.Miss.Render(SymMiss)
	}

	// show player's own ships (visible to player)
	if enemyState == CellShip {
		if m.showEmoji {
			return EmojiShip
		}
		return m.styles.Ship.Render(SymShip)
	}

	// debug mode: show enemy ships (fog of war disabled)
	if m.debugMode && m.board.HasEnemyShipAt(x, y) {
		if m.showEmoji {
			return EmojiShip
		}
		return m.styles.Ship.Render(SymShip)
	}

	// water/unexplored
	if m.showEmoji {
		return EmojiWater
	}
	return m.styles.Water.Render(SymWater)
}

// renderServiceStatus shows the status of enemy services and K8s events
func (m AppModel) renderServiceStatus() string {
	if m.enemyCompany == nil {
		return ""
	}

	// service status section
	serviceTitle := m.styles.Subtitle.Render("ENEMY SERVICES")

	var services string
	for _, svc := range m.enemyCompany.Services {
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
		status := fmt.Sprintf("%s %-12s %s", svc.Emoji, name, bar)
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
			icon := "  "
			style := m.styles.Normal
			switch evt.Type {
			case "Normal":
				icon = EmojiPodOK
				style = m.styles.Success
			case "Warning":
				icon = EmojiPodWarn
				style = m.styles.Warning
			case "Error":
				icon = EmojiPodDead
				style = m.styles.Error
			}
			// truncate message
			msg := evt.Message
			if len(msg) > 25 {
				msg = msg[:25] + "..."
			}
			if m.showEmoji {
				events += fmt.Sprintf("%s %s\n", icon, style.Render(msg))
			} else {
				events += fmt.Sprintf("[%s] %s\n", evt.Type[:1], style.Render(msg))
			}
		}
	} else {
		events = m.styles.Muted.Render("No events yet...")
	}

	// fleet stats
	statsTitle := m.styles.Subtitle.Render("FLEET STATUS")
	var stats string
	if m.board != nil {
		enemyStats := m.board.GetFleetStats(m.board.EnemyFleet)
		playerStats := m.board.GetFleetStats(m.board.PlayerFleet)

		stats = fmt.Sprintf("Enemy:  %d/%d pods | %d/%d racks\n",
			enemyStats.RunningPods, enemyStats.TotalPods,
			enemyStats.AliveRacks, enemyStats.TotalRacks)
		stats += fmt.Sprintf("Player: %d/%d pods | %d/%d racks",
			playerStats.RunningPods, playerStats.TotalPods,
			playerStats.AliveRacks, playerStats.TotalRacks)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
