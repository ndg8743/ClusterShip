package game

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// NodeView: what the board thinks bout one ship right now
type NodeView struct {
	ID         string
	X          int // anchor; first cell position (for legacy/UI)
	Y          int
	Health     int
	Size       int
	IsDead     bool
	LastUpdate time.Time
	Latency    time.Duration
	Team       string            // "red" or "blue"
	Cells      [][2]int          // occupied cells for this ship
	CellHit    map[string]bool   // "x,y" -> hit
}

// BoardStats tracks game and communication statistics.
type BoardStats struct {
	TotalAttacks      int            `json:"total_attacks"`
	TotalHits         int            `json:"total_hits"`
	TotalMisses       int            `json:"total_misses"`
	AttacksByTeam     map[string]int `json:"attacks_by_team"`
	HitsByTeam        map[string]int `json:"hits_by_team"`
	Heartbeats        int            `json:"heartbeats"`
	Connections       int            `json:"connections"`
	ActiveConnections int            `json:"active_connections"`
	Disconnections    int            `json:"disconnections"`
	AvgLatencyMs      float64        `json:"avg_latency_ms"`
	latencySum        time.Duration
	latencyCount      int
}

// GameBoard: keeps grid size + all ships. reads/writes use a lock so no races
type GameBoard struct {
	Width                int
	Height               int
	BoatCount            int // configurable boats per team
	ExpectedShipsPerTeam int // ships needed per team before game starts (0 = use BoatCount)
	Battleships          map[string]*NodeView

	recentUpdates []string
	shots         map[string]map[string]bool // bot -> coord -> hit
	lastTurn      string
	stats         BoardStats
	mu            sync.RWMutex
}

// NewGameBoard creates a board with width/height (supports up to 100x100).
func NewGameBoard(width, height int) *GameBoard {
	return NewGameBoardWithBoats(width, height, 1)
}

// NewGameBoardWithBoats creates a board with configurable dimensions and boat count.
func NewGameBoardWithBoats(width, height, boatCount int) *GameBoard {
	// Clamp dimensions to valid range (1-100)
	width = clampInt(width, 1, 100)
	height = clampInt(height, 1, 100)
	boatCount = clampInt(boatCount, 1, 10)

	return &GameBoard{
		Width:         width,
		Height:        height,
		BoatCount:     boatCount,
		Battleships:   make(map[string]*NodeView),
		recentUpdates: make([]string, 0, 32),
		shots:         make(map[string]map[string]bool),
		stats: BoardStats{
			AttacksByTeam: make(map[string]int),
			HitsByTeam:    make(map[string]int),
		},
	}
}

// clampInt limits v to the inclusive range [min, max].
func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// HandleNodeUpdate: merge a node heartbeat, record latency
// we only set hp/size/dead on first time seeing it. after that, board owns hp
func (b *GameBoard) HandleNodeUpdate(update NodeStateMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lat := time.Since(update.Timestamp)
	v, ok := b.Battleships[update.NodeID]
	if !ok {
		v = &NodeView{ID: update.NodeID}
		// Determine team: prefer explicit from message, fall back to derived
		team := update.Team
		if team == "" {
			team = deriveTeam(update.NodeID)
		}
		v.Team = team
		v.Size = update.Size
		v.Health = update.Size // health equals size
		v.IsDead = false
		v.Cells = b.generateBoatPlacementLocked(v.Size)
		v.CellHit = make(map[string]bool, v.Size)
		if len(v.Cells) > 0 {
			v.X, v.Y = v.Cells[0][0], v.Cells[0][1]
		}
		b.Battleships[update.NodeID] = v
		b.stats.Connections++
		b.stats.ActiveConnections++
	}
	// The board owns position/health after first sighting; only latency gets refreshed here.
	v.LastUpdate = time.Now()
	v.Latency = lat

	// Track heartbeat stats
	b.stats.Heartbeats++
	b.stats.latencySum += lat
	b.stats.latencyCount++
	if b.stats.latencyCount > 0 {
		b.stats.AvgLatencyMs = float64(b.stats.latencySum.Milliseconds()) / float64(b.stats.latencyCount)
	}

	b.pushRecentUpdate(update, lat)
}

// pushRecentUpdate: keep small list of last few things for ui
func (b *GameBoard) pushRecentUpdate(update NodeStateMessage, latency time.Duration) {
	entry := ""
	if update.IsDead {
		entry = formatUpdate(latency, update.NodeID, "DESTROYED")
	} else {
		entry = formatUpdate(latency, update.NodeID, "heartbeat OK")
	}
	if len(b.recentUpdates) >= 10 {
		b.recentUpdates = b.recentUpdates[1:]
	}
	b.recentUpdates = append(b.recentUpdates, entry)
}

// pushRecentText: add arbitrary line (like hits/miss)
func (b *GameBoard) pushRecentText(s string) {
	if len(b.recentUpdates) >= 10 {
		b.recentUpdates = b.recentUpdates[1:]
	}
	b.recentUpdates = append(b.recentUpdates, s)
}

// Snapshot: copy state for renderer so we don't hold lock long
func (b *GameBoard) Snapshot() (int, int, []*NodeView, []string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	nodes := make([]*NodeView, 0, len(b.Battleships))
	for _, v := range b.Battleships {
		c := *v
		// Deep copy variable-size fields to avoid races
		if v.Cells != nil {
			c.Cells = make([][2]int, len(v.Cells))
			copy(c.Cells, v.Cells)
		}
		if v.CellHit != nil {
			c.CellHit = make(map[string]bool, len(v.CellHit))
			for k, val := range v.CellHit {
				c.CellHit[k] = val
			}
		}
		nodes = append(nodes, &c)
	}
	updates := make([]string, len(b.recentUpdates))
	copy(updates, b.recentUpdates)
	return b.Width, b.Height, nodes, updates
}

// ShotsSnapshot: copy shots map for renderer
func (b *GameBoard) ShotsSnapshot() map[string]map[string]bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make(map[string]map[string]bool)
	for bot, m := range b.shots {
		result[bot] = make(map[string]bool)
		for k, v := range m {
			result[bot][k] = v
		}
	}
	return result
}

// LastTurn: get last turn bot name
func (b *GameBoard) LastTurn() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastTurn
}

// AliveCount: how many ships still not been destroyed
func (b *GameBoard) AliveCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, v := range b.Battleships {
		if !v.IsDead {
			n++
		}
	}
	return n
}

// TeamAliveCount returns how many ships are alive for a specific team
func (b *GameBoard) TeamAliveCount(team string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, v := range b.Battleships {
		if !v.IsDead && v.Team == team {
			n++
		}
	}
	return n
}

// TeamShipCount returns total ships registered for a team (alive or dead)
func (b *GameBoard) TeamShipCount(team string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, v := range b.Battleships {
		if v.Team == team {
			n++
		}
	}
	return n
}

// GameReady returns true if both teams have expected ships registered
func (b *GameBoard) GameReady() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.gameReadyLocked()
}

// gameReadyLocked checks if game is ready (caller must hold lock)
func (b *GameBoard) gameReadyLocked() bool {
	expected := b.ExpectedShipsPerTeam
	if expected <= 0 {
		expected = b.BoatCount
	}
	redCount := 0
	blueCount := 0
	for _, ship := range b.Battleships {
		if ship.Team == "red" {
			redCount++
		} else if ship.Team == "blue" {
			blueCount++
		}
	}
	return redCount >= expected && blueCount >= expected
}

// HandleDisconnect marks a ship as disconnected and updates stats
func (b *GameBoard) HandleDisconnect(nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.Battleships[nodeID]; ok {
		if b.stats.ActiveConnections > 0 {
			b.stats.ActiveConnections--
		}
		b.stats.Disconnections++
	}
}

// GameReport represents complete game state for reporting
type GameReport struct {
	Ready        bool             `json:"ready"`
	GameOver     bool             `json:"game_over"`
	Winner       string           `json:"winner"`
	RedAlive     int              `json:"red_alive"`
	BlueAlive    int              `json:"blue_alive"`
	RedTotal     int              `json:"red_total"`
	BlueTotal    int              `json:"blue_total"`
	Stats        BoardStats       `json:"stats"`
	ShipStatuses []ShipStatusView `json:"ships"`
}

// ShipStatusView is a public view of ship status
type ShipStatusView struct {
	ID     string `json:"id"`
	Team   string `json:"team"`
	Health int    `json:"health"`
	Size   int    `json:"size"`
	IsDead bool   `json:"is_dead"`
}

// GetGameReport returns comprehensive game status for reporting
func (b *GameBoard) GetGameReport() GameReport {
	b.mu.RLock()
	defer b.mu.RUnlock()

	status := GameReport{
		Ready: b.gameReadyLocked(),
		Stats: b.stats,
	}
	// Deep copy maps for stats
	status.Stats.AttacksByTeam = make(map[string]int)
	status.Stats.HitsByTeam = make(map[string]int)
	for k, v := range b.stats.AttacksByTeam {
		status.Stats.AttacksByTeam[k] = v
	}
	for k, v := range b.stats.HitsByTeam {
		status.Stats.HitsByTeam[k] = v
	}

	for _, ship := range b.Battleships {
		if ship.Team == "red" {
			status.RedTotal++
			if !ship.IsDead {
				status.RedAlive++
			}
		} else if ship.Team == "blue" {
			status.BlueTotal++
			if !ship.IsDead {
				status.BlueAlive++
			}
		}
		status.ShipStatuses = append(status.ShipStatuses, ShipStatusView{
			ID:     ship.ID,
			Team:   ship.Team,
			Health: ship.Health,
			Size:   ship.Size,
			IsDead: ship.IsDead,
		})
	}

	// Determine winner only if game started
	if status.Ready {
		if status.RedAlive == 0 && status.BlueAlive > 0 {
			status.GameOver = true
			status.Winner = "blue"
		} else if status.BlueAlive == 0 && status.RedAlive > 0 {
			status.GameOver = true
			status.Winner = "red"
		}
	}

	return status
}

// Attack: do a shot at x,y. returns hit + killed id if any
func (b *GameBoard) Attack(x, y int, by string) (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastTurn = by
	key := fmt.Sprintf("%d,%d", x, y)

	if b.shots[by] == nil {
		b.shots[by] = make(map[string]bool)
	}

	// Track attack stats
	shooterTeam := deriveTeam(by)
	b.stats.TotalAttacks++
	b.stats.AttacksByTeam[shooterTeam]++

	var hit *NodeView
	for _, v := range b.Battleships {
		if v.IsDead {
			continue
		}
		// only target opponent boats
		if v.Team == "" || v.Team == shooterTeam {
			continue
		}
		for _, cell := range v.Cells {
			if cell[0] == x && cell[1] == y {
				hit = v
				goto foundHit
			}
		}
	}
foundHit:

	if hit == nil {
		b.shots[by][key] = false
		b.stats.TotalMisses++
		b.pushRecentText(fmt.Sprintf("%s guess (%d,%d) -> miss", by, x, y))
		return false, ""
	}

	b.shots[by][key] = true
	b.stats.TotalHits++
	b.stats.HitsByTeam[shooterTeam]++

	cellKey := key
	if !hit.CellHit[cellKey] {
		hit.CellHit[cellKey] = true
		if hit.Health > 0 {
			hit.Health--
		}
	}
	if hit.Health <= 0 {
		hit.IsDead = true
		b.pushRecentText(fmt.Sprintf("%s hit %s at (%d,%d) -> DESTROYED", by, hit.ID, x, y))
		return true, hit.ID
	}
	b.pushRecentText(fmt.Sprintf("%s hit %s at (%d,%d) -> hp:%d", by, hit.ID, x, y, hit.Health))
	return true, ""
}

// formatUpdate: build a recent line
func formatUpdate(latency time.Duration, nodeID, msg string) string {
	return "[" + latency.String() + "] " + nodeID + " → " + msg
}

// deriveTeam inspects an identifier to determine team ("red" / "blue").
func deriveTeam(id string) string {
	if containsFold(id, "red") {
		return "red"
	}
	if containsFold(id, "blue") {
		return "blue"
	}
	return ""
}

// containsFold is a lightweight case-insensitive contains for ASCII
func containsFold(s, sub string) bool {
	// Simple lowercasing by bytes; avoid adding strings.ToLower to imports
	sl := []rune(s)
	subl := []rune(sub)
	// build lower copies
	for i := range sl {
		if 'A' <= sl[i] && sl[i] <= 'Z' {
			sl[i] = sl[i] + ('a' - 'A')
		}
	}
	for i := range subl {
		if 'A' <= subl[i] && subl[i] <= 'Z' {
			subl[i] = subl[i] + ('a' - 'A')
		}
	}
	S := string(sl)
	Sub := string(subl)
	return len(Sub) == 0 || (len(S) >= len(Sub) && indexOf(S, Sub) >= 0)
}

// indexOf returns index of sub in s, or -1
func indexOf(s, sub string) int {
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// generateBoatPlacementLocked picks a random placement of given length without overlapping existing ships.
// b.mu MUST be held by caller.
func (b *GameBoard) generateBoatPlacementLocked(length int) [][2]int {
	if length <= 0 {
		return nil
	}
	try := 0
	for {
		try++
		vertical := rand.Intn(2) == 0
		var x0, y0 int
		if vertical {
			x0 = rand.Intn(b.Width)
			y0 = rand.Intn(b.Height - (length - 1))
		} else {
			x0 = rand.Intn(b.Width - (length - 1))
			y0 = rand.Intn(b.Height)
		}
		cells := make([][2]int, length)
		ok := true
		for i := 0; i < length; i++ {
			x := x0
			y := y0
			if vertical {
				y += i
			} else {
				x += i
			}
			// ensure not overlapping existing boats
			for _, other := range b.Battleships {
				for _, oc := range other.Cells {
					if oc[0] == x && oc[1] == y {
						ok = false
						break
					}
				}
				if !ok {
					break
				}
			}
			if !ok {
				break
			}
			cells[i] = [2]int{x, y}
		}
		if ok {
			return cells
		}
		// extremely unlikely to loop long on 10x10; keep trying
		if try > 10000 {
			// fallback: place partially
			return cells
		}
	}
}

// Stats returns a copy of the current board statistics.
func (b *GameBoard) Stats() BoardStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s := b.stats
	s.AttacksByTeam = make(map[string]int)
	s.HitsByTeam = make(map[string]int)
	for k, v := range b.stats.AttacksByTeam {
		s.AttacksByTeam[k] = v
	}
	for k, v := range b.stats.HitsByTeam {
		s.HitsByTeam[k] = v
	}
	return s
}

// BotView represents what a bot can see: own ships and attack results.
type BotView struct {
	Team       string              `json:"team"`
	Width      int                 `json:"width"`
	Height     int                 `json:"height"`
	OwnShips   []ShipView          `json:"own_ships"`
	EnemyAlive int                 `json:"enemy_alive"`
	Attacks    map[string]bool     `json:"attacks"` // "x,y" -> hit
	GameOver   bool                `json:"game_over"`
	Winner     string              `json:"winner"`
}

// ShipView is a bot's view of one of its own ships.
type ShipView struct {
	ID     string   `json:"id"`
	Health int      `json:"health"`
	Size   int      `json:"size"`
	IsDead bool     `json:"is_dead"`
	Cells  [][2]int `json:"cells"`
}

// GetBotView returns what a specific bot/team can see.
func (b *GameBoard) GetBotView(botID string) BotView {
	b.mu.RLock()
	defer b.mu.RUnlock()

	team := deriveTeam(botID)
	view := BotView{
		Team:    team,
		Width:   b.Width,
		Height:  b.Height,
		Attacks: make(map[string]bool),
	}

	// Copy own ships
	for _, ship := range b.Battleships {
		if ship.Team == team {
			sv := ShipView{
				ID:     ship.ID,
				Health: ship.Health,
				Size:   ship.Size,
				IsDead: ship.IsDead,
				Cells:  make([][2]int, len(ship.Cells)),
			}
			copy(sv.Cells, ship.Cells)
			view.OwnShips = append(view.OwnShips, sv)
		}
	}

	// Count enemy ships alive
	enemyTeam := "blue"
	if team == "blue" {
		enemyTeam = "red"
	}
	for _, ship := range b.Battleships {
		if ship.Team == enemyTeam && !ship.IsDead {
			view.EnemyAlive++
		}
	}

	// Copy attack results for this bot
	if attacks, ok := b.shots[botID]; ok {
		for k, v := range attacks {
			view.Attacks[k] = v
		}
	}

	// Check game over only if game has started (both teams have ships)
	if b.gameReadyLocked() {
		redAlive := 0
		blueAlive := 0
		for _, ship := range b.Battleships {
			if !ship.IsDead {
				if ship.Team == "red" {
					redAlive++
				} else if ship.Team == "blue" {
					blueAlive++
				}
			}
		}
		if redAlive == 0 && blueAlive > 0 {
			view.GameOver = true
			view.Winner = "blue"
		} else if blueAlive == 0 && redAlive > 0 {
			view.GameOver = true
			view.Winner = "red"
		}
	}

	return view
}

