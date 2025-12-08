package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clustership/pkg/game"
)

var (
	shipID    = flag.String("id", "", "Ship ID (e.g., red-1)")
	shipSize  = flag.Int("size", 3, "Ship size/health")
	serverURL = flag.String("server", "ws://localhost:8080/ws/battleship", "Board WebSocket URL")
	latencyMs = flag.Int("latency", 50, "Simulated latency in milliseconds")
	gameID    = flag.String("game", "", "Game ID for multi-game mode")
)

func main() {
	flag.Parse()

	if *shipID == "" {
		log.Fatal("ship id required: -id=red-1")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	url := *serverURL
	if *gameID != "" {
		url += "?game_id=" + *gameID
	}

	latency := time.Duration(*latencyMs) * time.Millisecond
	ship := game.NewBattleshipNode(*shipID, 0, 0, *shipSize, *shipSize, latency, url)

	log.Printf("ship %s connecting to %s (size=%d, latency=%v)", *shipID, url, *shipSize, latency)
	ship.Run(ctx)
}
