package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clustership/pkg/api"
	"clustership/pkg/game"
)

var (
	boardWidth    = flag.Int("width", 100, "Board width (1-100)")
	boardHeight   = flag.Int("height", 100, "Board height (1-100)")
	boatCount     = flag.Int("boats", 3, "Boats per team")
	expectedShips = flag.Int("expected-ships", 0, "Expected ships per team (0 = use boats)")
	listenAddr    = flag.String("addr", ":8080", "HTTP listen address")
	showDisplay   = flag.Bool("display", true, "Show board display")
)

// main runs the board as a standalone control plane service.
// Ships connect via WebSocket, bots attack via HTTP.
func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	board := game.NewGameBoardWithBoats(*boardWidth, *boardHeight, *boatCount)
	if *expectedShips > 0 {
		board.ExpectedShipsPerTeam = *expectedShips
	}
	server := api.NewServer(board)

	var stopDisplay chan struct{}
	if *showDisplay {
		stopDisplay = make(chan struct{})
		go board.DisplayLoop(stopDisplay)
	}

	mux := http.NewServeMux()
	server.Routes(mux)
	srv := &http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("board listening on %s (%dx%d, %d boats/team)", *listenAddr, *boardWidth, *boardHeight, *boatCount)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()

	if stopDisplay != nil {
		close(stopDisplay)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)

	stats := board.Stats()
	log.Printf("=== FINAL STATS ===")
	log.Printf("attacks: %d, hits: %d, misses: %d", stats.TotalAttacks, stats.TotalHits, stats.TotalMisses)
	log.Printf("heartbeats: %d, connections: %d (active: %d), avg latency: %.2fms",
		stats.Heartbeats, stats.Connections, stats.ActiveConnections, stats.AvgLatencyMs)
}
