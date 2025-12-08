package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"clustership/pkg/game"
)

var (
	shipID    = flag.String("id", "", "Ship ID (e.g., ship-1)")
	shipSize  = flag.Int("size", 3, "Ship size/health")
	serverURL = flag.String("server", "ws://localhost:8080/ws/battleship", "Board WebSocket URL")
	latencyMs = flag.Int("latency", 50, "Simulated latency in milliseconds")
	gameID    = flag.String("game", "", "Game ID for multi-game mode")
	team      = flag.String("team", "", "Team: 'red' or 'blue'")
)

func main() {
	flag.Parse()

	if *shipID == "" {
		log.Fatal("ship id required: -id=ship-1")
	}

	// Determine team: prefer explicit flag, fall back to deriving from ID
	shipTeam := *team
	if shipTeam == "" {
		idLower := strings.ToLower(*shipID)
		if strings.Contains(idLower, "red") {
			shipTeam = "red"
		} else if strings.Contains(idLower, "blue") {
			shipTeam = "blue"
		}
	}
	if shipTeam != "red" && shipTeam != "blue" {
		log.Fatal("team required: -team=red or -team=blue (or include 'red'/'blue' in ship ID)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	url := *serverURL
	if *gameID != "" {
		url += "?game_id=" + *gameID
	}

	latency := time.Duration(*latencyMs) * time.Millisecond
	ship := game.NewBattleshipNode(*shipID, 0, 0, *shipSize, *shipSize, latency, url)
	ship.Team = shipTeam

	log.Printf("ship %s (team=%s) connecting to %s (size=%d, latency=%v)", *shipID, shipTeam, url, *shipSize, latency)
	ship.Run(ctx)
}
