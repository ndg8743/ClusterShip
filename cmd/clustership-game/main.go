package main

import (
	"context"
	"flag"
	"fmt"
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

var (
	boardWidth  = flag.Int("width", 100, "Board width (1-100)")
	boardHeight = flag.Int("height", 100, "Board height (1-100)")
)

// main starts the ClusterShip battle game.
// Architecture:
//   - GameBoard: central authority for ship state and attacks
//   - BattleCoordinator: manages turn-based battle loop with smart targeting
//   - BattleshipNodes: simulated ships that report heartbeats via WebSocket
func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	rand.Seed(time.Now().UnixNano())

	// Configure game with CLI flags and defaults
	config := game.GameConfig{
		BoardWidth:    *boardWidth,
		BoardHeight:   *boardHeight,
		BoatCount:     3,
		ShipSizes:     []int{4, 3, 2}, // 3 ships per team
		TurnDelay:     400 * time.Millisecond,
		TargetingMode: game.TargetingHitNeighbor,
	}

	// Create game board
	board := game.NewGameBoardWithBoats(config.BoardWidth, config.BoardHeight, config.BoatCount)

	// Setup HTTP server
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

	// Spawn ships for each team (positions assigned by board on heartbeat)
	spawnTeamShips(ctx, "red", config.ShipSizes)
	spawnTeamShips(ctx, "blue", config.ShipSizes)

	// Create battle coordinator
	coordinator := game.NewBattleCoordinator(board, config)
	coordinator.RegisterBot("red-bot", "red")
	coordinator.RegisterBot("blue-bot", "blue")

	// Run battle loop
	go coordinator.Run(ctx)

	<-ctx.Done()
	close(stopDisplay)

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)

	printFinalStats(coordinator)
}

// spawnTeamShips creates ships for a team with given sizes
func spawnTeamShips(ctx context.Context, team string, sizes []int) {
	for i, size := range sizes {
		id := fmt.Sprintf("%s-%d", team, i+1)
		latency := time.Duration(50+rand.Intn(50)) * time.Millisecond
		ship := game.NewBattleshipNode(id, 0, 0, size, size, latency,
			"ws://localhost:8080/ws/battleship")
		go ship.Run(ctx)
	}
}

// printFinalStats outputs end-game statistics.
func printFinalStats(coordinator *game.BattleCoordinator) {
	log.Println("=== BATTLE COMPLETE ===")
	for _, stat := range coordinator.GetBotStats() {
		log.Printf("%s (%s): %d shots, %d hits, %d kills",
			stat.ID, stat.Team, stat.ShotsFired, stat.HitsLanded, stat.ShipsSunk)
	}
}
