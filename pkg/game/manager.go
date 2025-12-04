package game

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// GameStatus represents the state of a game instance
type GameStatus int

const (
	GameStatusPending GameStatus = iota
	GameStatusRunning
	GameStatusCompleted
)

// GameInstance represents a single isolated game session
type GameInstance struct {
	ID          string
	Board       *GameBoard
	Coordinator *BattleCoordinator
	Config      GameConfig
	Status      GameStatus
	CreatedAt   time.Time
	ctx         context.Context
	cancel      context.CancelFunc
	stopDisplay chan struct{}
}

// GameManager manages multiple concurrent game instances
type GameManager struct {
	games    map[string]*GameInstance
	mu       sync.RWMutex
	maxGames int
	counter  int
}

// NewGameManager creates a manager with configurable limits
func NewGameManager(maxGames int) *GameManager {
	return &GameManager{
		games:    make(map[string]*GameInstance),
		maxGames: maxGames,
	}
}

// CreateGame creates a new isolated game instance
func (gm *GameManager) CreateGame(parentCtx context.Context, config GameConfig) (*GameInstance, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if len(gm.games) >= gm.maxGames {
		return nil, fmt.Errorf("max games (%d) reached", gm.maxGames)
	}

	gm.counter++
	id := fmt.Sprintf("game-%d", gm.counter)

	ctx, cancel := context.WithCancel(parentCtx)
	board := NewGameBoardWithBoats(config.BoardWidth, config.BoardHeight, config.BoatCount)
	coordinator := NewBattleCoordinator(board, config)

	instance := &GameInstance{
		ID:          id,
		Board:       board,
		Coordinator: coordinator,
		Config:      config,
		Status:      GameStatusPending,
		CreatedAt:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
		stopDisplay: make(chan struct{}),
	}

	gm.games[id] = instance
	return instance, nil
}

// GetGame retrieves a game by ID
func (gm *GameManager) GetGame(id string) (*GameInstance, bool) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	g, ok := gm.games[id]
	return g, ok
}

// DestroyGame cleans up and removes a game
func (gm *GameManager) DestroyGame(id string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	g, ok := gm.games[id]
	if !ok {
		return fmt.Errorf("game %s not found", id)
	}

	g.cancel()
	close(g.stopDisplay)
	delete(gm.games, id)
	return nil
}

// ListGames returns all active games
func (gm *GameManager) ListGames() []*GameInstance {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	list := make([]*GameInstance, 0, len(gm.games))
	for _, g := range gm.games {
		list = append(list, g)
	}
	return list
}

// StartGame begins a game instance (spawns ships and starts battle)
func (gi *GameInstance) Start(spawnShips func(ctx context.Context, gameID, team string, sizes []int)) {
	gi.Status = GameStatusRunning

	// Register bots
	gi.Coordinator.RegisterBot(gi.ID+"-red-bot", "red")
	gi.Coordinator.RegisterBot(gi.ID+"-blue-bot", "blue")

	// Spawn ships for each team
	spawnShips(gi.ctx, gi.ID, "red", gi.Config.ShipSizes)
	spawnShips(gi.ctx, gi.ID, "blue", gi.Config.ShipSizes)

	// Start battle loop
	go func() {
		gi.Coordinator.Run(gi.ctx)
		gi.Status = GameStatusCompleted
	}()
}

// GetBoard returns the game's board (for WebSocket routing)
func (gm *GameManager) GetBoard(gameID string) (*GameBoard, bool) {
	g, ok := gm.GetGame(gameID)
	if !ok {
		return nil, false
	}
	return g.Board, true
}
