package tui

import (
	"clustership/pkg/game"
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// AIPlayer handles AI decision making for the enemy
type AIPlayer struct {
	Strategy  game.AIStrategy
	CompanyID string // which company this AI controls

	// Per-opponent tracking for multi-company battles
	GuessedPerOpponent  map[string]map[string]bool // opponentID -> coordKey -> guessed
	HitsPerOpponent     map[string][][2]int        // opponentID -> hit coords
	HitQueuePerOpponent map[string][][2]int        // opponentID -> neighbors to try

	// Legacy single-target mode (backward compat)
	Guessed  map[string]bool // coordinates already shot at
	Hits     [][2]int        // successful hits
	HitQueue [][2]int        // neighbors to try after a hit
	LastHit  *[2]int

	BoardWidth  int
	BoardHeight int
}

// NewAIPlayer creates an AI with the given strategy (legacy single-opponent mode)
func NewAIPlayer(strategy game.AIStrategy, width, height int) *AIPlayer {
	return &AIPlayer{
		Strategy:            strategy,
		Guessed:             make(map[string]bool),
		Hits:                make([][2]int, 0),
		HitQueue:            make([][2]int, 0),
		GuessedPerOpponent:  make(map[string]map[string]bool),
		HitsPerOpponent:     make(map[string][][2]int),
		HitQueuePerOpponent: make(map[string][][2]int),
		BoardWidth:          width,
		BoardHeight:         height,
	}
}

// NewMultiAIPlayer creates an AI for multi-opponent battles
func NewMultiAIPlayer(companyID string, strategy game.AIStrategy, width, height int, opponentIDs []string) *AIPlayer {
	ai := &AIPlayer{
		Strategy:            strategy,
		CompanyID:           companyID,
		Guessed:             make(map[string]bool),
		Hits:                make([][2]int, 0),
		HitQueue:            make([][2]int, 0),
		GuessedPerOpponent:  make(map[string]map[string]bool),
		HitsPerOpponent:     make(map[string][][2]int),
		HitQueuePerOpponent: make(map[string][][2]int),
		BoardWidth:          width,
		BoardHeight:         height,
	}

	// Initialize per-opponent tracking
	for _, oppID := range opponentIDs {
		ai.GuessedPerOpponent[oppID] = make(map[string]bool)
		ai.HitsPerOpponent[oppID] = make([][2]int, 0)
		ai.HitQueuePerOpponent[oppID] = make([][2]int, 0)
	}

	return ai
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

// PickTargetAgainst chooses a target coordinate and opponent to attack
// Returns (coord, targetCompanyID)
func (ai *AIPlayer) PickTargetAgainst(activeOpponents []string) ([2]int, string) {
	if len(activeOpponents) == 0 {
		return [2]int{0, 0}, ""
	}

	// Strategy: prioritize opponents with active hunt queues
	for _, oppID := range activeOpponents {
		queue := ai.HitQueuePerOpponent[oppID]
		if len(queue) > 0 {
			// Pop from queue
			next := queue[0]
			ai.HitQueuePerOpponent[oppID] = queue[1:]

			key := fmt.Sprintf("%d,%d", next[0], next[1])
			guessed := ai.GuessedPerOpponent[oppID]
			if guessed == nil || !guessed[key] {
				return next, oppID
			}
		}
	}

	// No active hunts - pick random opponent and random coord
	oppID := activeOpponents[rand.Intn(len(activeOpponents))]
	coord := ai.pickRandomAgainst(oppID)
	return coord, oppID
}

// pickRandomAgainst finds an unguessed cell for a specific opponent
func (ai *AIPlayer) pickRandomAgainst(oppID string) [2]int {
	guessed := ai.GuessedPerOpponent[oppID]
	if guessed == nil {
		guessed = make(map[string]bool)
		ai.GuessedPerOpponent[oppID] = guessed
	}

	for attempts := 0; attempts < 10000; attempts++ {
		x := rand.Intn(ai.BoardWidth)
		y := rand.Intn(ai.BoardHeight)
		key := fmt.Sprintf("%d,%d", x, y)

		if !guessed[key] {
			return [2]int{x, y}
		}
	}

	return ai.linearScan()
}

// RecordResultAgainst updates AI state for multi-opponent mode
func (ai *AIPlayer) RecordResultAgainst(x, y int, targetID string, result *ShotResult) {
	key := fmt.Sprintf("%d,%d", x, y)

	// Initialize maps if needed
	if ai.GuessedPerOpponent[targetID] == nil {
		ai.GuessedPerOpponent[targetID] = make(map[string]bool)
	}
	ai.GuessedPerOpponent[targetID][key] = true

	// Also track globally for strategies that use it
	ai.Guessed[key] = true

	if result == nil || !result.Hit {
		return
	}

	// Hit! Record and queue neighbors
	if ai.HitsPerOpponent[targetID] == nil {
		ai.HitsPerOpponent[targetID] = make([][2]int, 0)
	}
	ai.HitsPerOpponent[targetID] = append(ai.HitsPerOpponent[targetID], [2]int{x, y})

	if !result.KilledRack {
		ai.queueNeighborsAgainst(x, y, targetID)
	} else {
		// Clear queue for this opponent if rack destroyed
		ai.HitQueuePerOpponent[targetID] = nil
	}
}

// queueNeighborsAgainst adds neighbors for a specific opponent
func (ai *AIPlayer) queueNeighborsAgainst(x, y int, oppID string) {
	neighbors := [][2]int{
		{x, y - 1}, {x, y + 1}, {x - 1, y}, {x + 1, y},
	}

	if ai.HitQueuePerOpponent[oppID] == nil {
		ai.HitQueuePerOpponent[oppID] = make([][2]int, 0)
	}

	guessed := ai.GuessedPerOpponent[oppID]
	for _, n := range neighbors {
		if !ai.isValidCoord(n[0], n[1]) {
			continue
		}
		key := fmt.Sprintf("%d,%d", n[0], n[1])
		if guessed != nil && guessed[key] {
			continue
		}
		// Add if not already in queue
		inQueue := false
		for _, q := range ai.HitQueuePerOpponent[oppID] {
			if q[0] == n[0] && q[1] == n[1] {
				inQueue = true
				break
			}
		}
		if !inQueue {
			ai.HitQueuePerOpponent[oppID] = append(ai.HitQueuePerOpponent[oppID], n)
		}
	}
}

// =============================================================================
// K-Nearest Neighbor (KNN) Targeting Algorithm
// =============================================================================
// Uses hit history to predict likely ship locations. Cells closer to clusters
// of previous hits have higher probability scores.

// cellScore represents a cell with its KNN-based probability score
type cellScore struct {
	x, y  int
	score float64
}

// euclideanDistance calculates distance between two points
func euclideanDistance(x1, y1, x2, y2 int) float64 {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	return math.Sqrt(dx*dx + dy*dy)
}

// pickKNNTarget uses K-Nearest Neighbor algorithm to find the best target
// K = number of nearest hits to consider for scoring each cell
func (ai *AIPlayer) pickKNNTarget(oppID string, k int) [2]int {
	hits := ai.HitsPerOpponent[oppID]
	guessed := ai.GuessedPerOpponent[oppID]

	if guessed == nil {
		guessed = make(map[string]bool)
	}

	// If no hits yet, fall back to probability density (center-biased)
	if len(hits) == 0 {
		return ai.pickProbabilityDensity(oppID)
	}

	// Adjust K to available hits
	if k > len(hits) {
		k = len(hits)
	}

	// Score all unguessed cells based on KNN
	candidates := make([]cellScore, 0)

	for x := 0; x < ai.BoardWidth; x++ {
		for y := 0; y < ai.BoardHeight; y++ {
			key := fmt.Sprintf("%d,%d", x, y)
			if guessed[key] {
				continue
			}

			// Calculate distances to all hits
			distances := make([]float64, len(hits))
			for i, hit := range hits {
				distances[i] = euclideanDistance(x, y, hit[0], hit[1])
			}

			// Sort distances and take K nearest
			sort.Float64s(distances)
			kNearest := distances[:k]

			// Score = inverse of average distance to K nearest hits
			// Lower average distance = higher score (closer to hit clusters)
			avgDist := 0.0
			for _, d := range kNearest {
				avgDist += d
			}
			avgDist /= float64(k)

			// Avoid division by zero, add small epsilon
			score := 1.0 / (avgDist + 0.1)

			// Bonus for cells adjacent to hits (hunt priority)
			for _, hit := range hits {
				if (abs(x-hit[0]) == 1 && y == hit[1]) || (abs(y-hit[1]) == 1 && x == hit[0]) {
					score *= 2.0 // Double score for adjacent cells
					break
				}
			}

			candidates = append(candidates, cellScore{x, y, score})
		}
	}

	if len(candidates) == 0 {
		return ai.linearScan()
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Pick from top candidates with some randomness to avoid predictability
	// Take top 10% or at least 3 candidates
	topN := len(candidates) / 10
	if topN < 3 {
		topN = 3
	}
	if topN > len(candidates) {
		topN = len(candidates)
	}

	// Weighted random selection from top candidates
	selected := candidates[rand.Intn(topN)]
	return [2]int{selected.x, selected.y}
}

// pickProbabilityDensity targets cells with higher ship probability
// Uses parity pattern (checkerboard) combined with center bias
func (ai *AIPlayer) pickProbabilityDensity(oppID string) [2]int {
	guessed := ai.GuessedPerOpponent[oppID]
	if guessed == nil {
		guessed = make(map[string]bool)
	}

	candidates := make([]cellScore, 0)
	centerX := float64(ai.BoardWidth) / 2
	centerY := float64(ai.BoardHeight) / 2

	for x := 0; x < ai.BoardWidth; x++ {
		for y := 0; y < ai.BoardHeight; y++ {
			key := fmt.Sprintf("%d,%d", x, y)
			if guessed[key] {
				continue
			}

			// Base score from parity (checkerboard pattern finds ships faster)
			score := 1.0
			if (x+y)%2 == 0 {
				score = 1.5 // Prefer checkerboard cells
			}

			// Center bias - ships often placed away from edges
			distFromCenter := euclideanDistance(x, y, int(centerX), int(centerY))
			maxDist := euclideanDistance(0, 0, int(centerX), int(centerY))
			centerScore := 1.0 - (distFromCenter / maxDist) // 0-1, higher near center
			score *= (1.0 + centerScore)

			candidates = append(candidates, cellScore{x, y, score})
		}
	}

	if len(candidates) == 0 {
		return ai.linearScan()
	}

	// Sort and pick from top candidates
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	topN := len(candidates) / 5
	if topN < 5 {
		topN = 5
	}
	if topN > len(candidates) {
		topN = len(candidates)
	}

	selected := candidates[rand.Intn(topN)]
	return [2]int{selected.x, selected.y}
}

// PickTargetAgainstKNN uses KNN algorithm for smarter targeting
// Returns (coord, targetCompanyID)
func (ai *AIPlayer) PickTargetAgainstKNN(activeOpponents []string, k int) ([2]int, string) {
	if len(activeOpponents) == 0 {
		return [2]int{0, 0}, ""
	}

	// First priority: check hunt queues (adjacent to recent hits)
	for _, oppID := range activeOpponents {
		queue := ai.HitQueuePerOpponent[oppID]
		if len(queue) > 0 {
			next := queue[0]
			ai.HitQueuePerOpponent[oppID] = queue[1:]

			key := fmt.Sprintf("%d,%d", next[0], next[1])
			guessed := ai.GuessedPerOpponent[oppID]
			if guessed == nil || !guessed[key] {
				return next, oppID
			}
		}
	}

	// Second priority: use KNN to find best target across all opponents
	// Score each opponent by their hit density (more hits = more likely to have remaining ships nearby)
	type oppScore struct {
		id    string
		score float64
	}

	oppScores := make([]oppScore, 0, len(activeOpponents))
	for _, oppID := range activeOpponents {
		hits := ai.HitsPerOpponent[oppID]
		guessed := ai.GuessedPerOpponent[oppID]
		guessedCount := 0
		if guessed != nil {
			guessedCount = len(guessed)
		}

		// Score based on hit ratio and remaining cells
		totalCells := ai.BoardWidth * ai.BoardHeight
		remainingCells := totalCells - guessedCount
		hitRatio := float64(len(hits)+1) / float64(guessedCount+1)
		score := hitRatio * float64(remainingCells)

		oppScores = append(oppScores, oppScore{oppID, score})
	}

	// Sort by score and pick weighted random from top opponents
	sort.Slice(oppScores, func(i, j int) bool {
		return oppScores[i].score > oppScores[j].score
	})

	// Weighted selection favoring higher-scored opponents
	totalScore := 0.0
	for _, os := range oppScores {
		totalScore += os.score
	}

	r := rand.Float64() * totalScore
	cumulative := 0.0
	selectedOpp := oppScores[0].id
	for _, os := range oppScores {
		cumulative += os.score
		if r <= cumulative {
			selectedOpp = os.id
			break
		}
	}

	// Use KNN to pick target for selected opponent
	coord := ai.pickKNNTarget(selectedOpp, k)
	return coord, selectedOpp
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
