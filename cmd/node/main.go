package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"clustership/pkg/game"
)

// main: run a single battleship node pod
// reads env vars: NODE_ID, X, Y, SIZE, HEALTH, LATENCY_MS, SERVER_URL
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	nodeID := getEnv("NODE_ID", "ship-1")
	x := getEnvInt("X", rand.Intn(10))
	y := getEnvInt("Y", rand.Intn(10))
	size := getEnvInt("SIZE", 3)
	health := getEnvInt("HEALTH", size)
	latencyMs := getEnvInt("LATENCY_MS", 100)
	serverURL := getEnv("SERVER_URL", "ws://clustership-server:8080/ws/battleship")

	log.Printf("starting node %s at (%d,%d) size=%d hp=%d latency=%dms", nodeID, x, y, size, health, latencyMs)

	node := game.NewBattleshipNode(nodeID, x, y, size, health, time.Duration(latencyMs)*time.Millisecond, serverURL)
	node.Run(ctx)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

