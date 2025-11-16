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
// we only set hp/size/dead on first time seeing it. after that, board owns hp
func (b *GameBoard) HandleNodeUpdate(update NodeStateMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lat := time.Since(update.Timestamp)
	v, ok := b.Battleships[update.NodeID]
	if !ok {
		v = &NodeView{ID: update.NodeID}
		b.Battleships[update.NodeID] = v
		// first time we know about this node, board assigns canonical stats and placement
		v.Team = deriveTeam(update.NodeID)
		v.Size = 4
		v.Health = 4
		v.IsDead = false
		v.Cells = b.generateBoatPlacementLocked(v.Size)
		v.CellHit = make(map[string]bool, v.Size)
		if len(v.Cells) > 0 {
			v.X, v.Y = v.Cells[0][0], v.Cells[0][1]
		}
	}
	// The board owns position/health after first sighting; only latency gets refreshed here.
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
	shooterTeam := deriveTeam(by)
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
		b.pushRecentText(fmt.Sprintf("%s guess (%d,%d) -> miss", by, x, y))
		return false, ""
	}

	b.shots[by][key] = true
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

