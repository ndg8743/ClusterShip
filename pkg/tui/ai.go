package tui

import (
	"clustership/pkg/game"
	"fmt"
	"math/rand"
)

// AIPlayer handles AI decision making for the enemy
type AIPlayer struct {
	Strategy   game.AIStrategy
	Guessed    map[string]bool // coordinates already shot at
	Hits       [][2]int        // successful hits
	HitQueue   [][2]int        // neighbors to try after a hit
	LastHit    *[2]int
	BoardWidth int
	BoardHeight int
}

// NewAIPlayer creates an AI with the given strategy
func NewAIPlayer(strategy game.AIStrategy, width, height int) *AIPlayer {
	return &AIPlayer{
		Strategy:    strategy,
		Guessed:     make(map[string]bool),
		Hits:        make([][2]int, 0),
		HitQueue:    make([][2]int, 0),
		BoardWidth:  width,
		BoardHeight: height,
	}
}

// PickTarget chooses the next coordinate to attack based on strategy
func (ai *AIPlayer) PickTarget() [2]int {
	switch ai.Strategy {
	case game.AIRandom:
		return ai.pickRandom()
	case game.AIHunter:
		return ai.pickHunter()
	case game.AIDefensive:
		return ai.pickDefensive()
	case game.AIAggressive:
		return ai.pickAggressive()
	default:
		return ai.pickHunter()
	}
}

// RecordResult updates AI state based on attack result
func (ai *AIPlayer) RecordResult(x, y int, result *ShotResult) {
	key := fmt.Sprintf("%d,%d", x, y)
	ai.Guessed[key] = true

	if result == nil || !result.Hit {
		return
	}

	// hit! record it and queue neighbors
	ai.Hits = append(ai.Hits, [2]int{x, y})
	ai.LastHit = &[2]int{x, y}

	if result.KilledRack {
		// destroyed something - clear the hunt queue, target is dead
		ai.HitQueue = nil
		ai.LastHit = nil
	} else {
		// hit but not destroyed - queue neighbors for hunting
		ai.queueNeighbors(x, y)
	}
}

// pickRandom finds any random unguessed cell
func (ai *AIPlayer) pickRandom() [2]int {
	for attempts := 0; attempts < 10000; attempts++ {
		x := rand.Intn(ai.BoardWidth)
		y := rand.Intn(ai.BoardHeight)
		key := fmt.Sprintf("%d,%d", x, y)

		if !ai.Guessed[key] {
			return [2]int{x, y}
		}
	}

	// fallback: linear scan
	return ai.linearScan()
}

// pickHunter uses the classic "hunt nearest neighbor" strategy
// when there's an active hit, target adjacent cells first
func (ai *AIPlayer) pickHunter() [2]int {
	// if we have queued neighbors from a hit, try those first
	for len(ai.HitQueue) > 0 {
		// pop from front
		next := ai.HitQueue[0]
		ai.HitQueue = ai.HitQueue[1:]

		key := fmt.Sprintf("%d,%d", next[0], next[1])
		if ai.Guessed[key] {
			continue
		}

		if ai.isValidCoord(next[0], next[1]) {
			return next
		}
	}

	// no active hunt - random search
	return ai.pickRandom()
}

// pickDefensive focuses on edges and spread-out targeting
// (tries to find ships without revealing too much)
func (ai *AIPlayer) pickDefensive() [2]int {
	// check pattern: every 3rd cell in checkerboard
	for y := 0; y < ai.BoardHeight; y += 2 {
		for x := (y/2)%3; x < ai.BoardWidth; x += 3 {
			key := fmt.Sprintf("%d,%d", x, y)
			if !ai.Guessed[key] {
				return [2]int{x, y}
			}
		}
	}
	// fallback to random
	return ai.pickRandom()
}

// pickAggressive targets center of board first (ships often placed there)
func (ai *AIPlayer) pickAggressive() [2]int {
	// if we're hunting, stay aggressive
	if len(ai.HitQueue) > 0 {
		return ai.pickHunter()
	}

	// target center-ish areas first with some randomness
	centerX := ai.BoardWidth / 2
	centerY := ai.BoardHeight / 2
	radius := 1

	for radius < max(ai.BoardWidth, ai.BoardHeight)/2 {
		for attempts := 0; attempts < 20; attempts++ {
			x := centerX + rand.Intn(radius*2+1) - radius
			y := centerY + rand.Intn(radius*2+1) - radius

			if ai.isValidCoord(x, y) {
				key := fmt.Sprintf("%d,%d", x, y)
				if !ai.Guessed[key] {
					return [2]int{x, y}
				}
			}
		}
		radius++
	}

	return ai.pickRandom()
}

// queueNeighbors adds the four cardinal neighbors to the hunt queue
func (ai *AIPlayer) queueNeighbors(x, y int) {
	neighbors := [][2]int{
		{x, y - 1}, // up
		{x, y + 1}, // down
		{x - 1, y}, // left
		{x + 1, y}, // right
	}

	for _, n := range neighbors {
		if !ai.isValidCoord(n[0], n[1]) {
			continue
		}

		key := fmt.Sprintf("%d,%d", n[0], n[1])
		if ai.Guessed[key] {
			continue
		}

		// check if already in queue
		inQueue := false
		for _, q := range ai.HitQueue {
			if q[0] == n[0] && q[1] == n[1] {
				inQueue = true
				break
			}
		}

		if !inQueue {
			ai.HitQueue = append(ai.HitQueue, n)
		}
	}
}

// isValidCoord checks if coordinates are within board bounds
func (ai *AIPlayer) isValidCoord(x, y int) bool {
	return x >= 0 && x < ai.BoardWidth && y >= 0 && y < ai.BoardHeight
}

// linearScan finds the first unguessed cell by scanning the board
func (ai *AIPlayer) linearScan() [2]int {
	for x := 0; x < ai.BoardWidth; x++ {
		for y := 0; y < ai.BoardHeight; y++ {
			key := fmt.Sprintf("%d,%d", x, y)
			if !ai.Guessed[key] {
				return [2]int{x, y}
			}
		}
	}
	return [2]int{0, 0} // board is full, shouldn't happen
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
