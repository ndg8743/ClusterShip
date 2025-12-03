package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clustership/pkg/api"
	"clustership/pkg/game"
)

// main starts the ClusterShip battle game.
// Architecture:
//   - GameBoard: central authority for ship state and attacks
//   - BattleCoordinator: manages turn-based battle loop with smart targeting
//   - BattleshipNodes: simulated ships that report heartbeats via WebSocket
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	rand.Seed(time.Now().UnixNano())

	// Configure game parameters (supports up to 100x100 boards)
	config := game.GameConfig{
		BoardWidth:  10,  // Change to 100 for large-scale battles
		BoardHeight: 10,  // Change to 100 for large-scale battles
		BoatCount:   1,   // Boats per team
		TurnDelay:   400 * time.Millisecond,
	}

	// Create game board with configurable size
	board := game.NewGameBoardWithBoats(config.BoardWidth, config.BoardHeight, config.BoatCount)

	// Setup HTTP server for WebSocket connections
	server := api.NewServer(board)
	mux := http.NewServeMux()
	server.Routes(mux)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Start ASCII display loop
	stopDisplay := make(chan struct{})
	go board.DisplayLoop(stopDisplay)

	// Place ships at random non-overlapping coordinates
	positions := generateShipPositions(config.BoardWidth, config.BoardHeight, 2)

	// Start ship nodes (they send heartbeats to the board)
	red := game.NewBattleshipNode("red-1", positions[0].X, positions[0].Y, 5, 5,
		60*time.Millisecond, "ws://localhost:8080/ws/battleship")
	blue := game.NewBattleshipNode("blue-1", positions[1].X, positions[1].Y, 4, 4,
		80*time.Millisecond, "ws://localhost:8080/ws/battleship")
	go red.Run(ctx)
	go blue.Run(ctx)

	// Create battle coordinator with smart targeting
	coordinator := game.NewBattleCoordinator(board, config)

	// Register bots - each maintains its own board state for targeting
	coordinator.RegisterBot("red-bot", "red")
	coordinator.RegisterBot("blue-bot", "blue")

	// Run battle loop in background
	// Bots take turns, using hit-nearest-neighbor algorithm for follow-up shots
	go coordinator.Run(ctx)

	// Wait for shutdown signal
	<-ctx.Done()
	close(stopDisplay)

	// Graceful shutdown
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)

	// Print final stats
	printFinalStats(coordinator)
}

// shipPosition holds coordinates for ship placement.
type shipPosition struct {
	X, Y int
}

// generateShipPositions creates n non-overlapping positions on the board.
func generateShipPositions(width, height, count int) []shipPosition {
	positions := make([]shipPosition, 0, count)
	used := make(map[string]bool)

	for len(positions) < count {
		x := rand.Intn(width)
		y := rand.Intn(height)
		key := keyFor(x, y)

		if !used[key] {
			used[key] = true
			positions = append(positions, shipPosition{X: x, Y: y})
		}
	}
	return positions
}

// keyFor creates a string key from coordinates.
func keyFor(x, y int) string {
	// Simple concatenation for map keys
	buf := make([]byte, 0, 8)
	buf = appendInt(buf, x)
	buf = append(buf, ',')
	buf = appendInt(buf, y)
	return string(buf)
}

// appendInt appends integer to byte slice without fmt import.
func appendInt(b []byte, n int) []byte {
	if n == 0 {
		return append(b, '0')
	}
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	var digits [10]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return append(b, digits[i:]...)
}

// printFinalStats outputs end-game statistics.
func printFinalStats(coordinator *game.BattleCoordinator) {
	log.Println("=== BATTLE COMPLETE ===")
	for _, stat := range coordinator.GetBotStats() {
		log.Printf("%s (%s): %d shots, %d hits, %d kills",
			stat.ID, stat.Team, stat.ShotsFired, stat.HitsLanded, stat.ShipsSunk)
	}
}
