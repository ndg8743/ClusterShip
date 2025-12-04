package game

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// TargetingMode defines how bots select attack coordinates.
type TargetingMode int

const (
	// TargetingRandom uses pure random shooting
	TargetingRandom TargetingMode = iota
	// TargetingHitNeighbor targets adjacent cells after a hit (smart targeting)
	TargetingHitNeighbor
)

// GameConfig holds configurable game parameters.
type GameConfig struct {
	BoardWidth    int           // Board width (default 100, max 100)
	BoardHeight   int           // Board height (default 100, max 100)
	BoatCount     int           // Number of boats per team (legacy, use ShipSizes)
	ShipSizes     []int         // Sizes for each ship (e.g., [4, 3, 2])
	TurnDelay     time.Duration // Delay between turns
	TargetingMode TargetingMode // Targeting strategy (default: TargetingHitNeighbor)
}

// DefaultConfig returns sensible defaults for a standard game.
func DefaultConfig() GameConfig {
	return GameConfig{
		BoardWidth:    100,
		BoardHeight:   100,
		BoatCount:     3,
		ShipSizes:     []int{4, 3, 2},
		TurnDelay:     400 * time.Millisecond,
		TargetingMode: TargetingHitNeighbor,
	}
}

// BotState tracks what a single bot knows about the game.
// Each bot maintains its own view of the enemy board.
type BotState struct {
	ID        string
	Team      string
	Guessed   map[string]bool // "x,y" -> true if already guessed
	Hits      []Coord         // Coordinates where we hit something
	LastHit   *Coord          // Most recent hit (for neighbor targeting)
	HitQueue  []Coord         // Queue of neighbors to try after a hit
	KillCount int             // Ships destroyed by this bot
}

// Coord represents a board coordinate.
type Coord struct {
	X int
	Y int
}

// Key returns string key for map lookups.
func (c Coord) Key() string {
	return fmt.Sprintf("%d,%d", c.X, c.Y)
}

// NewBotState creates a fresh bot with empty knowledge.
func NewBotState(id, team string) *BotState {
	return &BotState{
		ID:       id,
		Team:     team,
		Guessed:  make(map[string]bool),
		Hits:     make([]Coord, 0),
		HitQueue: make([]Coord, 0),
	}
}

// BattleCoordinator manages the turn-based battle loop.
// It iterates through bots and coordinates attacks against the global board.
type BattleCoordinator struct {
	board   *GameBoard
	bots    []*BotState
	config  GameConfig
	current int // Index of current bot's turn
	mu      sync.Mutex
}

// NewBattleCoordinator creates a coordinator for the given board and config.
func NewBattleCoordinator(board *GameBoard, config GameConfig) *BattleCoordinator {
	return &BattleCoordinator{
		board:  board,
		bots:   make([]*BotState, 0),
		config: config,
	}
}

// RegisterBot adds a bot to participate in the battle.
func (bc *BattleCoordinator) RegisterBot(id, team string) *BotState {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bot := NewBotState(id, team)
	bc.bots = append(bc.bots, bot)
	return bot
}

// Run executes the battle loop until one team has no ships left.
func (bc *BattleCoordinator) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check win condition: battle ends when either team has no ships left
		if bc.board.TeamAliveCount("red") == 0 || bc.board.TeamAliveCount("blue") == 0 {
			return
		}

		// Get current bot
		bc.mu.Lock()
		if len(bc.bots) == 0 {
			bc.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		bot := bc.bots[bc.current]
		bc.current = (bc.current + 1) % len(bc.bots)
		bc.mu.Unlock()

		// Execute turn for this bot
		bc.executeTurn(bot)

		time.Sleep(bc.config.TurnDelay)
	}
}

// executeTurn performs one attack for a bot using smart targeting.
func (bc *BattleCoordinator) executeTurn(bot *BotState) {
	// Pick target coordinate
	target := bc.pickTarget(bot)
	if target == nil {
		return // No valid targets left
	}

	// Mark as guessed
	bot.Guessed[target.Key()] = true

	// Execute attack on global board
	hit, killedID := bc.board.Attack(target.X, target.Y, bot.ID)

	// Update bot's local state based on result
	bc.updateBotState(bot, *target, hit, killedID)

	// Notify board of state change (for communication protocol)
	bc.broadcastTurnResult(bot, *target, hit, killedID)
}

// pickTarget chooses the next coordinate to attack based on targeting mode.
func (bc *BattleCoordinator) pickTarget(bot *BotState) *Coord {
	switch bc.config.TargetingMode {
	case TargetingRandom:
		return bc.pickRandomTarget(bot)
	case TargetingHitNeighbor:
		return bc.pickSmartTarget(bot)
	default:
		return bc.pickSmartTarget(bot)
	}
}

// pickSmartTarget uses hit-nearest-neighbor strategy.
// When there's an active hit, it targets adjacent cells first.
func (bc *BattleCoordinator) pickSmartTarget(bot *BotState) *Coord {
	// Strategy 1: If we have queued neighbors from a hit, try those first
	for len(bot.HitQueue) > 0 {
		// Pop from queue
		next := bot.HitQueue[0]
		bot.HitQueue = bot.HitQueue[1:]

		// Skip if already guessed
		if bot.Guessed[next.Key()] {
			continue
		}

		// Validate bounds
		if bc.isValidCoord(next) {
			return &next
		}
	}

	// Strategy 2: Random search for unexplored cells
	return bc.pickRandomTarget(bot)
}

// pickRandomTarget finds a random unguessed coordinate.
func (bc *BattleCoordinator) pickRandomTarget(bot *BotState) *Coord {
	maxAttempts := bc.config.BoardWidth * bc.config.BoardHeight

	for i := 0; i < maxAttempts; i++ {
		x := rand.Intn(bc.config.BoardWidth)
		y := rand.Intn(bc.config.BoardHeight)
		coord := Coord{X: x, Y: y}

		if !bot.Guessed[coord.Key()] {
			return &coord
		}
	}

	// Fallback: linear scan for any unguessed cell
	for x := 0; x < bc.config.BoardWidth; x++ {
		for y := 0; y < bc.config.BoardHeight; y++ {
			coord := Coord{X: x, Y: y}
			if !bot.Guessed[coord.Key()] {
				return &coord
			}
		}
	}

	return nil // Board fully explored
}

// updateBotState updates the bot's local knowledge after an attack.
func (bc *BattleCoordinator) updateBotState(bot *BotState, target Coord, hit bool, killedID string) {
	if !hit {
		return // Nothing to update on miss
	}

	// Record the hit
	bot.Hits = append(bot.Hits, target)
	bot.LastHit = &target

	if killedID != "" {
		// Ship destroyed - clear the hit queue, target is dead
		bot.KillCount++
		bot.HitQueue = nil
		bot.LastHit = nil
	} else {
		// Hit but not sunk - queue neighbors for follow-up
		bc.queueNeighbors(bot, target)
	}
}

// queueNeighbors adds adjacent cells to the hit queue.
// This implements the "hit nearest neighbor" algorithm.
func (bc *BattleCoordinator) queueNeighbors(bot *BotState, hit Coord) {
	// Four cardinal directions (up, down, left, right)
	neighbors := []Coord{
		{X: hit.X, Y: hit.Y - 1}, // up
		{X: hit.X, Y: hit.Y + 1}, // down
		{X: hit.X - 1, Y: hit.Y}, // left
		{X: hit.X + 1, Y: hit.Y}, // right
	}

	for _, n := range neighbors {
		// Skip invalid or already guessed
		if !bc.isValidCoord(n) || bot.Guessed[n.Key()] {
			continue
		}

		// Skip if already in queue
		inQueue := false
		for _, q := range bot.HitQueue {
			if q.X == n.X && q.Y == n.Y {
				inQueue = true
				break
			}
		}
		if !inQueue {
			bot.HitQueue = append(bot.HitQueue, n)
		}
	}
}

// isValidCoord checks if a coordinate is within board bounds.
func (bc *BattleCoordinator) isValidCoord(c Coord) bool {
	return c.X >= 0 && c.X < bc.config.BoardWidth &&
		c.Y >= 0 && c.Y < bc.config.BoardHeight
}

// broadcastTurnResult notifies the system of the turn outcome.
// This triggers communication to nodes about hit/miss state.
func (bc *BattleCoordinator) broadcastTurnResult(bot *BotState, target Coord, hit bool, killedID string) {
	// The board already updates its internal state in Attack()
	// This method exists for future expansion:
	// - WebSocket push to connected nodes
	// - Event logging
	// - Metrics collection
}

// GetBotStats returns current statistics for all bots.
func (bc *BattleCoordinator) GetBotStats() []BotStats {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	stats := make([]BotStats, len(bc.bots))
	for i, bot := range bc.bots {
		stats[i] = BotStats{
			ID:         bot.ID,
			Team:       bot.Team,
			ShotsFired: len(bot.Guessed),
			HitsLanded: len(bot.Hits),
			ShipsSunk:  bot.KillCount,
		}
	}
	return stats
}

// BotStats holds summary statistics for a bot.
type BotStats struct {
	ID         string
	Team       string
	ShotsFired int
	HitsLanded int
	ShipsSunk  int
}
