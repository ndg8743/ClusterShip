package tui

import (
	"clustership/pkg/game"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
	cursor       [2]int // current cursor position on board
	viewport     [2]int // viewport offset for scrolling large boards
	turn         int
	isPlayerTurn bool
	gameOver     bool
	winner       string
	lastMessage  string // last attack result message

	// display
	showEmoji   bool
	compactMode bool
}

// NewAppModel creates a fresh game instance
func NewAppModel() AppModel {
	return AppModel{
		state:     StateMenu,
		styles:    DefaultStyles(),
		menuItems: []string{"New Game", "Settings", "Quit"},
		companies: game.ListCompanies(),
		showEmoji: false, // ascii mode by default, easier on terminals
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
			m.state = StateCompanySelect
			m.companyCursor = 0
		case 1: // Settings
			m.showEmoji = !m.showEmoji // toggle emoji mode for now
		case 2: // Quit
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
		// create board with auto-placed fleets
		m.board = NewBoard(20, 20, m.playerCompany, m.enemyCompany)
		m.state = StateBattle
		m.isPlayerTurn = true
		m.turn = 1
	case "esc":
		m.state = StateCompanySelect
	}
	return m, nil
}

// updateBattle handles the main battle phase
func (m AppModel) updateBattle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "enter", " ":
		if m.isPlayerTurn && m.board != nil {
			result, _ := m.board.Attack(m.cursor[0], m.cursor[1], true)
			if result != nil {
				m.lastMessage = result.Message
			}
			m.checkWinCondition()
			if !m.gameOver {
				m.executeAITurn()
				m.checkWinCondition()
				m.turn++
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

	// show a 10x10 viewport for now
	viewW, viewH := 10, 10
	if m.compactMode {
		viewW, viewH = 20, 20
	}

	// header row with column numbers
	header := "   "
	for x := 0; x < viewW; x++ {
		header += fmt.Sprintf("%d ", (m.viewport[0]+x)%10)
	}
	header += "\n"

	var grid string
	for y := 0; y < viewH; y++ {
		boardY := m.viewport[1] + y
		if boardY >= m.board.Height {
			break
		}
		// row number
		grid += fmt.Sprintf("%2d ", boardY%100)

		for x := 0; x < viewW; x++ {
			boardX := m.viewport[0] + x
			if boardX >= m.board.Width {
				break
			}

			cell := m.getCellDisplay(boardX, boardY)

			// highlight cursor
			if boardX == m.cursor[0] && boardY == m.cursor[1] {
				cell = m.styles.Cursor.Render(cell)
			}

			grid += cell + " "
		}
		grid += "\n"
	}

	cursorInfo := fmt.Sprintf("Target: (%d, %d)", m.cursor[0], m.cursor[1])

	return m.styles.BoardArea.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.styles.Subtitle.Render("ENEMY TERRITORY"),
			header+grid,
			m.styles.Muted.Render(cursorInfo),
		),
	)
}

// getCellDisplay returns the display character for a board cell
func (m AppModel) getCellDisplay(x, y int) string {
	if m.board == nil {
		return SymWater
	}

	state := m.board.GetEnemyCellState(x, y)

	switch state {
	case CellHit:
		if m.showEmoji {
			return EmojiHit
		}
		return m.styles.Hit.Render(SymHit)
	case CellMiss:
		if m.showEmoji {
			return EmojiMiss
		}
		return m.styles.Miss.Render(SymMiss)
	case CellDestroyed:
		if m.showEmoji {
			return EmojiDestroyed
		}
		return m.styles.Destroyed.Render(SymDestroyed)
	default:
		if m.showEmoji {
			return EmojiWater
		}
		return m.styles.Water.Render(SymWater)
	}
}

// renderServiceStatus shows the status of enemy services
func (m AppModel) renderServiceStatus() string {
	if m.enemyCompany == nil {
		return ""
	}

	title := m.styles.Subtitle.Render("ENEMY SERVICES")

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

		status := fmt.Sprintf("%s %s %s", svc.Emoji, svc.Name[:min(12, len(svc.Name))], bar)
		services += status + "\n"
	}

	return m.styles.Sidebar.Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", services),
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

func (m *AppModel) executeAITurn() {
	if m.board == nil {
		return
	}

	// simple random targeting - find an unshot cell
	for attempts := 0; attempts < 1000; attempts++ {
		x := randInt(m.board.Width)
		y := randInt(m.board.Height)
		key := fmt.Sprintf("%d,%d", x, y)

		if _, exists := m.board.EnemyShots[key]; !exists {
			m.board.Attack(x, y, false)
			return
		}
	}
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

// simple random - using math/rand seeded by default in go 1.20+
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return int(uint32(n) * uint32(fastRand()) >> 32)
}

var fastRandState uint32 = 1

func fastRand() uint32 {
	// simple xorshift
	fastRandState ^= fastRandState << 13
	fastRandState ^= fastRandState >> 17
	fastRandState ^= fastRandState << 5
	return fastRandState
}
