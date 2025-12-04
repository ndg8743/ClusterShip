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
	numGames    = flag.Int("games", 1, "Number of concurrent games to run")
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

	config := game.GameConfig{
		BoardWidth:    *boardWidth,
		BoardHeight:   *boardHeight,
		BoatCount:     3,
		ShipSizes:     []int{4, 3, 2},
		TurnDelay:     400 * time.Millisecond,
		TargetingMode: game.TargetingHitNeighbor,
	}

	var server *api.Server
	var coordinators []*game.BattleCoordinator
	var stopDisplays []chan struct{}

	if *numGames > 1 {
		// Multi-game mode with GameManager
		manager := game.NewGameManager(*numGames)
		server = api.NewServerWithManager(manager)

		for i := 0; i < *numGames; i++ {
			gi, err := manager.CreateGame(ctx, config)
			if err != nil {
				log.Fatalf("failed to create game: %v", err)
			}
			log.Printf("Created %s (%dx%d)", gi.ID, config.BoardWidth, config.BoardHeight)

			// Start display loop for this game
			go gi.Board.DisplayLoop(gi.StopDisplay)

			// Spawn ships with game-specific WebSocket URL
			spawnGameShips(ctx, gi.ID, "red", config.ShipSizes)
			spawnGameShips(ctx, gi.ID, "blue", config.ShipSizes)

			gi.Coordinator.RegisterBot(gi.ID+"-red-bot", "red")
			gi.Coordinator.RegisterBot(gi.ID+"-blue-bot", "blue")
			go gi.Coordinator.Run(ctx)

			coordinators = append(coordinators, gi.Coordinator)
			stopDisplays = append(stopDisplays, gi.StopDisplay)
		}
	} else {
		// Single game mode (backwards compatible)
		board := game.NewGameBoardWithBoats(config.BoardWidth, config.BoardHeight, config.BoatCount)
		server = api.NewServer(board)

		stopDisplay := make(chan struct{})
		go board.DisplayLoop(stopDisplay)
		stopDisplays = append(stopDisplays, stopDisplay)

		spawnTeamShips(ctx, "red", config.ShipSizes)
		spawnTeamShips(ctx, "blue", config.ShipSizes)

		coordinator := game.NewBattleCoordinator(board, config)
		coordinator.RegisterBot("red-bot", "red")
		coordinator.RegisterBot("blue-bot", "blue")
		go coordinator.Run(ctx)

		coordinators = append(coordinators, coordinator)
	}

	mux := http.NewServeMux()
	server.Routes(mux)
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on %s (%d game(s))", srv.Addr, *numGames)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	for _, ch := range stopDisplays {
		close(ch)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)

	for _, c := range coordinators {
		printFinalStats(c)
	}
}

// spawnTeamShips creates ships for a team (single game mode)
func spawnTeamShips(ctx context.Context, team string, sizes []int) {
	for i, size := range sizes {
		id := fmt.Sprintf("%s-%d", team, i+1)
		latency := time.Duration(50+rand.Intn(50)) * time.Millisecond
		ship := game.NewBattleshipNode(id, 0, 0, size, size, latency,
			"ws://localhost:8080/ws/battleship")
		go ship.Run(ctx)
	}
}

// spawnGameShips creates ships for a specific game instance
func spawnGameShips(ctx context.Context, gameID, team string, sizes []int) {
	for i, size := range sizes {
		id := fmt.Sprintf("%s-%s-%d", gameID, team, i+1)
		latency := time.Duration(50+rand.Intn(50)) * time.Millisecond
		url := fmt.Sprintf("ws://localhost:8080/ws/battleship?game_id=%s", gameID)
		ship := game.NewBattleshipNode(id, 0, 0, size, size, latency, url)
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
