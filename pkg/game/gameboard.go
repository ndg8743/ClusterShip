package game

import (
	"fmt"
	"sync"
	"time"
)

// NodeView: what the board thinks bout one ship right now
type NodeView struct {
	ID         string
	X          int
	Y          int
	Health     int
	Size       int
	IsDead     bool
	LastUpdate time.Time
	Latency    time.Duration
}

// GameBoard: keeps grid size + all ships. reads/writes use a lock so no races
type GameBoard struct {
	Width       int
	Height      int
	Battleships map[string]*NodeView

	recentUpdates []string
	shots         map[string]map[string]bool // bot -> coord -> hit
	lastTurn      string
	mu            sync.RWMutex
}

// NewGameBoard: make a board with width/height
func NewGameBoard(width, height int) *GameBoard {
	return &GameBoard{
		Width:         width,
		Height:        height,
		Battleships:   make(map[string]*NodeView),
		recentUpdates: make([]string, 0, 32),
		shots:         make(map[string]map[string]bool),
	}
}

// HandleNodeUpdate: merge a node heartbeat, record latency
// note: we only set hp/size/dead on first time seeing it. after that, board owns hp
func (b *GameBoard) HandleNodeUpdate(update NodeStateMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lat := time.Since(update.Timestamp)
	v, ok := b.Battleships[update.NodeID]
	if !ok {
		v = &NodeView{ID: update.NodeID}
		b.Battleships[update.NodeID] = v
		// first time we know about this node, so copy stats
		v.Health = update.Health
		v.Size = update.Size
		v.IsDead = update.IsDead
	}
	v.X = clamp(update.X, 0, b.Width-1)
	v.Y = clamp(update.Y, 0, b.Height-1)
	v.LastUpdate = time.Now()
	v.Latency = lat

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

// AliveCount: how many ships still not dead
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

// Attack: do a shot at x,y. returns hit + killed id if any
func (b *GameBoard) Attack(x, y int, by string) (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastTurn = by
	key := fmt.Sprintf("%d,%d", x, y)

	if b.shots[by] == nil {
		b.shots[by] = make(map[string]bool)
	}

	var hit *NodeView
	for _, v := range b.Battleships {
		if v.X == x && v.Y == y && !v.IsDead {
			hit = v
			break
		}
	}

	if hit == nil {
		b.shots[by][key] = false
		b.pushRecentText(fmt.Sprintf("%s guess (%d,%d) -> miss", by, x, y))
		return false, ""
	}

	b.shots[by][key] = true
	if hit.Health > 0 {
		hit.Health--
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

