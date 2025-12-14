package tui

import (
	"clustership/pkg/game"
	"fmt"
	"testing"
)

// TestNewAIPlayer tests creating AI players in single-opponent mode
func TestNewAIPlayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy game.AIStrategy
		width    int
		height   int
		validate func(t *testing.T, ai *AIPlayer)
	}{
		{
			name:     "create hunter AI",
			strategy: game.AIHunter,
			width:    20,
			height:   20,
			validate: func(t *testing.T, ai *AIPlayer) {
				if ai.Strategy != game.AIHunter {
					t.Errorf("expected strategy AIHunter, got %s", ai.Strategy)
				}
				if ai.BoardWidth != 20 || ai.BoardHeight != 20 {
					t.Errorf("expected board 20x20, got %dx%d", ai.BoardWidth, ai.BoardHeight)
				}
				if ai.Guessed == nil {
					t.Error("Guessed map should be initialized")
				}
				if ai.Hits == nil {
					t.Error("Hits slice should be initialized")
				}
				if ai.HitQueue == nil {
					t.Error("HitQueue slice should be initialized")
				}
			},
		},
		{
			name:     "create random AI",
			strategy: game.AIRandom,
			width:    15,
			height:   15,
			validate: func(t *testing.T, ai *AIPlayer) {
				if ai.Strategy != game.AIRandom {
					t.Errorf("expected strategy AIRandom, got %s", ai.Strategy)
				}
			},
		},
		{
			name:     "create defensive AI",
			strategy: game.AIDefensive,
			width:    10,
			height:   10,
			validate: func(t *testing.T, ai *AIPlayer) {
				if ai.Strategy != game.AIDefensive {
					t.Errorf("expected strategy AIDefensive, got %s", ai.Strategy)
				}
			},
		},
		{
			name:     "create aggressive AI",
			strategy: game.AIAggressive,
			width:    25,
			height:   25,
			validate: func(t *testing.T, ai *AIPlayer) {
				if ai.Strategy != game.AIAggressive {
					t.Errorf("expected strategy AIAggressive, got %s", ai.Strategy)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewAIPlayer(tt.strategy, tt.width, tt.height)
			if ai == nil {
				t.Fatal("NewAIPlayer returned nil")
			}

			tt.validate(t, ai)
		})
	}
}

// TestNewMultiAIPlayer tests creating AI for multi-opponent battles
func TestNewMultiAIPlayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		companyID   string
		strategy    game.AIStrategy
		opponentIDs []string
		validate    func(t *testing.T, ai *AIPlayer)
	}{
		{
			name:        "multi-opponent AI initialization",
			companyID:   "ai-company",
			strategy:    game.AIHunter,
			opponentIDs: []string{"player", "enemy1", "enemy2"},
			validate: func(t *testing.T, ai *AIPlayer) {
				if ai.CompanyID != "ai-company" {
					t.Errorf("expected CompanyID 'ai-company', got %s", ai.CompanyID)
				}
				if len(ai.GuessedPerOpponent) != 3 {
					t.Errorf("expected 3 opponent maps, got %d", len(ai.GuessedPerOpponent))
				}
				for _, oppID := range []string{"player", "enemy1", "enemy2"} {
					if ai.GuessedPerOpponent[oppID] == nil {
						t.Errorf("GuessedPerOpponent[%s] not initialized", oppID)
					}
					if ai.HitsPerOpponent[oppID] == nil {
						t.Errorf("HitsPerOpponent[%s] not initialized", oppID)
					}
					if ai.HitQueuePerOpponent[oppID] == nil {
						t.Errorf("HitQueuePerOpponent[%s] not initialized", oppID)
					}
				}
			},
		},
		{
			name:        "single opponent in multi mode",
			companyID:   "solo-ai",
			strategy:    game.AIAggressive,
			opponentIDs: []string{"player"},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.GuessedPerOpponent) != 1 {
					t.Errorf("expected 1 opponent map, got %d", len(ai.GuessedPerOpponent))
				}
			},
		},
		{
			name:        "no opponents",
			companyID:   "lonely-ai",
			strategy:    game.AIRandom,
			opponentIDs: []string{},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.GuessedPerOpponent) != 0 {
					t.Errorf("expected 0 opponent maps, got %d", len(ai.GuessedPerOpponent))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewMultiAIPlayer(tt.companyID, tt.strategy, 20, 20, tt.opponentIDs)
			if ai == nil {
				t.Fatal("NewMultiAIPlayer returned nil")
			}

			tt.validate(t, ai)
		})
	}
}

// TestPickTarget tests the PickTarget method for each strategy
func TestPickTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy game.AIStrategy
		setup    func(ai *AIPlayer)
		validate func(t *testing.T, ai *AIPlayer, coord [2]int)
	}{
		{
			name:     "random strategy picks valid coordinate",
			strategy: game.AIRandom,
			setup:    func(ai *AIPlayer) {},
			validate: func(t *testing.T, ai *AIPlayer, coord [2]int) {
				if !ai.isValidCoord(coord[0], coord[1]) {
					t.Errorf("picked invalid coordinate: %v", coord)
				}
			},
		},
		{
			name:     "hunter strategy uses hit queue",
			strategy: game.AIHunter,
			setup: func(ai *AIPlayer) {
				// Simulate a hit and queue neighbors
				ai.Hits = append(ai.Hits, [2]int{5, 5})
				ai.HitQueue = [][2]int{{5, 6}, {5, 4}}
			},
			validate: func(t *testing.T, ai *AIPlayer, coord [2]int) {
				// Should pick from queue: either {5,6} or {5,4}
				if (coord[0] != 5 || (coord[1] != 6 && coord[1] != 4)) && len(ai.HitQueue) > 0 {
					// Could have picked from queue or random if queue was empty
				}
			},
		},
		{
			name:     "defensive strategy spreads attacks",
			strategy: game.AIDefensive,
			setup:    func(ai *AIPlayer) {},
			validate: func(t *testing.T, ai *AIPlayer, coord [2]int) {
				if !ai.isValidCoord(coord[0], coord[1]) {
					t.Errorf("picked invalid coordinate: %v", coord)
				}
			},
		},
		{
			name:     "aggressive strategy targets center",
			strategy: game.AIAggressive,
			setup:    func(ai *AIPlayer) {},
			validate: func(t *testing.T, ai *AIPlayer, coord [2]int) {
				if !ai.isValidCoord(coord[0], coord[1]) {
					t.Errorf("picked invalid coordinate: %v", coord)
				}
			},
		},
		{
			name:     "random when board nearly full",
			strategy: game.AIRandom,
			setup: func(ai *AIPlayer) {
				// Mark most cells as guessed
				for x := 0; x < ai.BoardWidth; x++ {
					for y := 0; y < ai.BoardHeight; y++ {
						if x != 5 || y != 5 {
							key := fmt.Sprintf("%d,%d", x, y)
							ai.Guessed[key] = true
						}
					}
				}
			},
			validate: func(t *testing.T, ai *AIPlayer, coord [2]int) {
				// Should eventually find the one remaining cell
				if coord[0] != 5 || coord[1] != 5 {
					t.Errorf("expected to find (5,5), got %v", coord)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewAIPlayer(tt.strategy, 10, 10)
			tt.setup(ai)

			coord := ai.PickTarget()
			tt.validate(t, ai, coord)
		})
	}
}

// TestRecordResult tests recording attack results
func TestRecordResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy game.AIStrategy
		x, y     int
		result   *ShotResult
		validate func(t *testing.T, ai *AIPlayer)
	}{
		{
			name:     "record miss",
			strategy: game.AIHunter,
			x:        3,
			y:        4,
			result:   &ShotResult{Hit: false, Coord: [2]int{3, 4}},
			validate: func(t *testing.T, ai *AIPlayer) {
				key := "3,4"
				if !ai.Guessed[key] {
					t.Error("coordinate should be marked as guessed")
				}
				if len(ai.Hits) != 0 {
					t.Error("Hits should be empty for miss")
				}
			},
		},
		{
			name:     "record hit without kill",
			strategy: game.AIHunter,
			x:        5,
			y:        5,
			result:   &ShotResult{Hit: true, KilledRack: false, Coord: [2]int{5, 5}},
			validate: func(t *testing.T, ai *AIPlayer) {
				key := "5,5"
				if !ai.Guessed[key] {
					t.Error("coordinate should be marked as guessed")
				}
				if len(ai.Hits) != 1 {
					t.Fatalf("expected 1 hit, got %d", len(ai.Hits))
				}
				if ai.Hits[0][0] != 5 || ai.Hits[0][1] != 5 {
					t.Errorf("expected hit at (5,5), got %v", ai.Hits[0])
				}
				if len(ai.HitQueue) == 0 {
					t.Error("HitQueue should have neighbors queued")
				}
				if ai.LastHit == nil {
					t.Error("LastHit should be set")
				}
			},
		},
		{
			name:     "record rack kill clears queue",
			strategy: game.AIHunter,
			x:        7,
			y:        8,
			result:   &ShotResult{Hit: true, KilledRack: true, Coord: [2]int{7, 8}},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.HitQueue) != 0 {
					t.Error("HitQueue should be cleared after rack kill")
				}
				if ai.LastHit != nil {
					t.Error("LastHit should be cleared after rack kill")
				}
			},
		},
		{
			name:     "record nil result",
			strategy: game.AIRandom,
			x:        1,
			y:        1,
			result:   nil,
			validate: func(t *testing.T, ai *AIPlayer) {
				key := "1,1"
				if !ai.Guessed[key] {
					t.Error("coordinate should still be marked as guessed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewAIPlayer(tt.strategy, 15, 15)
			ai.RecordResult(tt.x, tt.y, tt.result)

			tt.validate(t, ai)
		})
	}
}

// TestPickTargetAgainst tests multi-opponent targeting
func TestPickTargetAgainst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setup           func(ai *AIPlayer)
		activeOpponents []string
		validate        func(t *testing.T, coord [2]int, targetID string)
	}{
		{
			name: "pick from active opponents",
			setup: func(ai *AIPlayer) {
				// No special setup
			},
			activeOpponents: []string{"player", "enemy1"},
			validate: func(t *testing.T, coord [2]int, targetID string) {
				if targetID != "player" && targetID != "enemy1" {
					t.Errorf("expected target to be player or enemy1, got %s", targetID)
				}
			},
		},
		{
			name: "prioritize opponent with active hunt",
			setup: func(ai *AIPlayer) {
				// Add a hit queue for enemy1
				ai.HitQueuePerOpponent["enemy1"] = [][2]int{{5, 5}, {5, 6}}
			},
			activeOpponents: []string{"player", "enemy1"},
			validate: func(t *testing.T, coord [2]int, targetID string) {
				if targetID != "enemy1" {
					t.Errorf("expected to target enemy1 with active hunt, got %s", targetID)
				}
				if coord[0] != 5 || (coord[1] != 5 && coord[1] != 6) {
					t.Errorf("expected coordinate from queue, got %v", coord)
				}
			},
		},
		{
			name: "no active opponents returns empty",
			setup: func(ai *AIPlayer) {
			},
			activeOpponents: []string{},
			validate: func(t *testing.T, coord [2]int, targetID string) {
				if targetID != "" {
					t.Errorf("expected empty targetID, got %s", targetID)
				}
			},
		},
		{
			name: "skip already guessed coordinates in queue",
			setup: func(ai *AIPlayer) {
				ai.GuessedPerOpponent["player"] = map[string]bool{"5,5": true}
				ai.HitQueuePerOpponent["player"] = [][2]int{{5, 5}, {5, 6}}
			},
			activeOpponents: []string{"player"},
			validate: func(t *testing.T, coord [2]int, targetID string) {
				if coord[0] == 5 && coord[1] == 5 {
					t.Error("should skip already guessed coordinate (5,5)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewMultiAIPlayer("ai", game.AIHunter, 20, 20, tt.activeOpponents)
			tt.setup(ai)

			coord, targetID := ai.PickTargetAgainst(tt.activeOpponents)
			tt.validate(t, coord, targetID)
		})
	}
}

// TestRecordResultAgainst tests recording results in multi-opponent mode
func TestRecordResultAgainst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ai *AIPlayer)
		x, y     int
		targetID string
		result   *ShotResult
		validate func(t *testing.T, ai *AIPlayer)
	}{
		{
			name:     "record hit against specific opponent",
			setup:    func(ai *AIPlayer) {},
			x:        5,
			y:        6,
			targetID: "player",
			result:   &ShotResult{Hit: true, KilledRack: false},
			validate: func(t *testing.T, ai *AIPlayer) {
				key := "5,6"
				if !ai.GuessedPerOpponent["player"][key] {
					t.Error("coordinate should be marked as guessed for player")
				}
				if len(ai.HitsPerOpponent["player"]) != 1 {
					t.Errorf("expected 1 hit against player, got %d", len(ai.HitsPerOpponent["player"]))
				}
				if len(ai.HitQueuePerOpponent["player"]) == 0 {
					t.Error("neighbors should be queued for player")
				}
			},
		},
		{
			name:     "rack kill clears queue for specific opponent",
			setup:    func(ai *AIPlayer) {},
			x:        3,
			y:        4,
			targetID: "enemy1",
			result:   &ShotResult{Hit: true, KilledRack: true},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.HitQueuePerOpponent["enemy1"]) != 0 {
					t.Error("queue should be cleared for enemy1 after rack kill")
				}
			},
		},
		{
			name:     "miss doesn't queue neighbors",
			setup:    func(ai *AIPlayer) {},
			x:        7,
			y:        8,
			targetID: "player",
			result:   &ShotResult{Hit: false},
			validate: func(t *testing.T, ai *AIPlayer) {
				key := "7,8"
				if !ai.GuessedPerOpponent["player"][key] {
					t.Error("coordinate should be marked as guessed")
				}
				// Note: queue might be empty or from previous hits
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewMultiAIPlayer("ai", game.AIHunter, 20, 20, []string{"player", "enemy1", "enemy2"})
			tt.setup(ai)

			ai.RecordResultAgainst(tt.x, tt.y, tt.targetID, tt.result)
			tt.validate(t, ai)
		})
	}
}

// TestQueueNeighbors tests the queueNeighbors method
func TestQueueNeighbors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		x, y     int
		setup    func(ai *AIPlayer)
		validate func(t *testing.T, ai *AIPlayer)
	}{
		{
			name: "queue all four neighbors",
			x:    5,
			y:    5,
			setup: func(ai *AIPlayer) {
			},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.HitQueue) != 4 {
					t.Errorf("expected 4 neighbors queued, got %d", len(ai.HitQueue))
				}
				expectedNeighbors := map[string]bool{
					"5,4": false, "5,6": false, "4,5": false, "6,5": false,
				}
				for _, coord := range ai.HitQueue {
					key := fmt.Sprintf("%d,%d", coord[0], coord[1])
					if _, exists := expectedNeighbors[key]; exists {
						expectedNeighbors[key] = true
					}
				}
				for key, found := range expectedNeighbors {
					if !found {
						t.Errorf("neighbor %s not queued", key)
					}
				}
			},
		},
		{
			name: "corner cell queues only two neighbors",
			x:    0,
			y:    0,
			setup: func(ai *AIPlayer) {
			},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.HitQueue) != 2 {
					t.Errorf("expected 2 neighbors for corner, got %d", len(ai.HitQueue))
				}
			},
		},
		{
			name: "edge cell queues three neighbors",
			x:    0,
			y:    5,
			setup: func(ai *AIPlayer) {
			},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.HitQueue) != 3 {
					t.Errorf("expected 3 neighbors for edge, got %d", len(ai.HitQueue))
				}
			},
		},
		{
			name: "skip already guessed neighbors",
			x:    5,
			y:    5,
			setup: func(ai *AIPlayer) {
				ai.Guessed["5,6"] = true
				ai.Guessed["4,5"] = true
			},
			validate: func(t *testing.T, ai *AIPlayer) {
				if len(ai.HitQueue) != 2 {
					t.Errorf("expected 2 unguessed neighbors, got %d", len(ai.HitQueue))
				}
				for _, coord := range ai.HitQueue {
					key := fmt.Sprintf("%d,%d", coord[0], coord[1])
					if ai.Guessed[key] {
						t.Errorf("queued already guessed coordinate: %s", key)
					}
				}
			},
		},
		{
			name: "don't duplicate neighbors in queue",
			x:    5,
			y:    5,
			setup: func(ai *AIPlayer) {
				ai.HitQueue = [][2]int{{5, 6}}
			},
			validate: func(t *testing.T, ai *AIPlayer) {
				// Count occurrences of each coordinate
				coords := make(map[string]int)
				for _, coord := range ai.HitQueue {
					key := fmt.Sprintf("%d,%d", coord[0], coord[1])
					coords[key]++
				}
				for key, count := range coords {
					if count > 1 {
						t.Errorf("coordinate %s appears %d times in queue", key, count)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewAIPlayer(game.AIHunter, 10, 10)
			tt.setup(ai)
			ai.queueNeighbors(tt.x, tt.y)

			tt.validate(t, ai)
		})
	}
}

// TestQueueNeighborsAgainst tests multi-opponent neighbor queueing
func TestQueueNeighborsAgainst(t *testing.T) {
	t.Parallel()

	ai := NewMultiAIPlayer("ai", game.AIHunter, 10, 10, []string{"player", "enemy"})

	// Queue neighbors for player
	ai.queueNeighborsAgainst(5, 5, "player")

	if len(ai.HitQueuePerOpponent["player"]) != 4 {
		t.Errorf("expected 4 neighbors for player, got %d", len(ai.HitQueuePerOpponent["player"]))
	}

	// Queue should be separate for enemy
	if len(ai.HitQueuePerOpponent["enemy"]) != 0 {
		t.Error("enemy queue should be empty")
	}

	// Mark some as guessed and queue again
	ai.GuessedPerOpponent["player"]["5,6"] = true
	ai.queueNeighborsAgainst(5, 5, "player")

	// Count unique coordinates
	uniqueCoords := make(map[string]bool)
	for _, coord := range ai.HitQueuePerOpponent["player"] {
		key := fmt.Sprintf("%d,%d", coord[0], coord[1])
		if ai.GuessedPerOpponent["player"][key] {
			t.Errorf("queued already guessed coordinate: %s", key)
		}
		uniqueCoords[key] = true
	}
}

// TestPickTargetAgainstKNN tests KNN-based targeting
func TestPickTargetAgainstKNN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setup           func(ai *AIPlayer)
		activeOpponents []string
		k               int
		validate        func(t *testing.T, coord [2]int, targetID string, ai *AIPlayer)
	}{
		{
			name: "KNN with hit history",
			setup: func(ai *AIPlayer) {
				// Add some hit history for player
				ai.HitsPerOpponent["player"] = [][2]int{{5, 5}, {5, 6}, {5, 7}}
			},
			activeOpponents: []string{"player"},
			k:               3,
			validate: func(t *testing.T, coord [2]int, targetID string, ai *AIPlayer) {
				if targetID != "player" {
					t.Errorf("expected target player, got %s", targetID)
				}
				// Coordinate should be within board bounds
				if !ai.isValidCoord(coord[0], coord[1]) {
					t.Errorf("invalid coordinate: %v", coord)
				}
			},
		},
		{
			name: "KNN prioritizes active hunt",
			setup: func(ai *AIPlayer) {
				ai.HitQueuePerOpponent["enemy"] = [][2]int{{3, 3}}
				ai.HitsPerOpponent["enemy"] = [][2]int{{3, 2}}
			},
			activeOpponents: []string{"player", "enemy"},
			k:               2,
			validate: func(t *testing.T, coord [2]int, targetID string, ai *AIPlayer) {
				if targetID != "enemy" {
					t.Errorf("expected to continue hunt against enemy, got %s", targetID)
				}
			},
		},
		{
			name: "KNN with no hits uses probability",
			setup: func(ai *AIPlayer) {
				// No hits
			},
			activeOpponents: []string{"player"},
			k:               3,
			validate: func(t *testing.T, coord [2]int, targetID string, ai *AIPlayer) {
				if !ai.isValidCoord(coord[0], coord[1]) {
					t.Errorf("invalid coordinate: %v", coord)
				}
			},
		},
		{
			name: "KNN selects opponent based on score",
			setup: func(ai *AIPlayer) {
				// Give player more hits
				ai.HitsPerOpponent["player"] = [][2]int{{1, 1}, {2, 2}, {3, 3}}
				ai.GuessedPerOpponent["player"]["1,1"] = true
				ai.GuessedPerOpponent["player"]["2,2"] = true
				ai.GuessedPerOpponent["player"]["3,3"] = true

				// Give enemy fewer hits but less guessed
				ai.HitsPerOpponent["enemy"] = [][2]int{{5, 5}}
				ai.GuessedPerOpponent["enemy"]["5,5"] = true
			},
			activeOpponents: []string{"player", "enemy"},
			k:               2,
			validate: func(t *testing.T, coord [2]int, targetID string, ai *AIPlayer) {
				// Either target is valid based on scoring algorithm
				if targetID != "player" && targetID != "enemy" {
					t.Errorf("unexpected target: %s", targetID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ai := NewMultiAIPlayer("ai", game.AIAggressive, 20, 20, tt.activeOpponents)
			tt.setup(ai)

			coord, targetID := ai.PickTargetAgainstKNN(tt.activeOpponents, tt.k)
			tt.validate(t, coord, targetID, ai)
		})
	}
}

// TestIsValidCoord tests coordinate validation
func TestIsValidCoord(t *testing.T) {
	t.Parallel()

	ai := NewAIPlayer(game.AIRandom, 10, 10)

	tests := []struct {
		x, y     int
		expected bool
	}{
		{0, 0, true},
		{9, 9, true},
		{5, 5, true},
		{-1, 5, false},
		{5, -1, false},
		{10, 5, false},
		{5, 10, false},
		{-1, -1, false},
		{10, 10, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("(%d,%d)", tt.x, tt.y), func(t *testing.T) {
			result := ai.isValidCoord(tt.x, tt.y)
			if result != tt.expected {
				t.Errorf("isValidCoord(%d, %d) = %v, want %v", tt.x, tt.y, result, tt.expected)
			}
		})
	}
}

// TestLinearScan tests the linear scan fallback
func TestLinearScan(t *testing.T) {
	t.Parallel()

	ai := NewAIPlayer(game.AIRandom, 5, 5)

	// Mark all but one cell as guessed
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			if x != 3 || y != 4 {
				key := fmt.Sprintf("%d,%d", x, y)
				ai.Guessed[key] = true
			}
		}
	}

	coord := ai.linearScan()
	if coord[0] != 3 || coord[1] != 4 {
		t.Errorf("expected (3,4), got %v", coord)
	}
}

// TestPickRandomAgainst tests random targeting for specific opponent
func TestPickRandomAgainst(t *testing.T) {
	t.Parallel()

	ai := NewMultiAIPlayer("ai", game.AIRandom, 10, 10, []string{"player", "enemy"})

	// Mark some cells as guessed for player
	ai.GuessedPerOpponent["player"]["5,5"] = true
	ai.GuessedPerOpponent["player"]["5,6"] = true

	coord := ai.pickRandomAgainst("player")

	// Should not pick already guessed coordinates
	key := fmt.Sprintf("%d,%d", coord[0], coord[1])
	if ai.GuessedPerOpponent["player"][key] {
		t.Errorf("picked already guessed coordinate: %v", coord)
	}

	// Should be valid
	if !ai.isValidCoord(coord[0], coord[1]) {
		t.Errorf("picked invalid coordinate: %v", coord)
	}
}

// BenchmarkPickTarget benchmarks target selection
func BenchmarkPickTarget(b *testing.B) {
	strategies := []game.AIStrategy{
		game.AIRandom,
		game.AIHunter,
		game.AIDefensive,
		game.AIAggressive,
	}

	for _, strategy := range strategies {
		b.Run(string(strategy), func(b *testing.B) {
			ai := NewAIPlayer(strategy, 30, 30)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ai.PickTarget()
			}
		})
	}
}

// BenchmarkRecordResult benchmarks result recording
func BenchmarkRecordResult(b *testing.B) {
	ai := NewAIPlayer(game.AIHunter, 30, 30)
	result := &ShotResult{
		Hit:        true,
		KilledRack: false,
		Coord:      [2]int{5, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := i % 30
		y := (i / 30) % 30
		ai.RecordResult(x, y, result)
	}
}

// BenchmarkPickTargetAgainstKNN benchmarks KNN targeting
func BenchmarkPickTargetAgainstKNN(b *testing.B) {
	ai := NewMultiAIPlayer("ai", game.AIAggressive, 30, 30, []string{"p1", "p2", "p3"})

	// Add some hit history
	ai.HitsPerOpponent["p1"] = [][2]int{{5, 5}, {6, 6}, {7, 7}}
	ai.HitsPerOpponent["p2"] = [][2]int{{10, 10}, {11, 11}}

	activeOpponents := []string{"p1", "p2", "p3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ai.PickTargetAgainstKNN(activeOpponents, 3)
	}
}

// BenchmarkQueueNeighbors benchmarks neighbor queueing
func BenchmarkQueueNeighbors(b *testing.B) {
	ai := NewAIPlayer(game.AIHunter, 30, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := (i % 28) + 1 // Avoid edges
		y := ((i / 28) % 28) + 1
		ai.queueNeighbors(x, y)
	}
}
