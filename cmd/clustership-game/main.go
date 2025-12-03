package main

import (
	"context"
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

// main starts the HTTP server, ASCII display loop, and one demo node.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	rand.Seed(time.Now().UnixNano())

	board := game.NewGameBoard(10, 10)
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

	// Start display loop
	stopDisplay := make(chan struct{})
	go board.DisplayLoop(stopDisplay)

	// place 2 ships at diff coords (we keep simple: one cell per ship)
	x1, y1 := rand.Intn(10), rand.Intn(10)
	x2, y2 := rand.Intn(10), rand.Intn(10)
	for x2 == x1 && y2 == y1 {
		x2, y2 = rand.Intn(10), rand.Intn(10)
	}

	// Start two nodes (stationary)
	red := game.NewBattleshipNode("red-1", x1, y1, 5, 5, 60*time.Millisecond, "ws://localhost:8080/ws/battleship")
	blue := game.NewBattleshipNode("blue-1", x2, y2, 4, 4, 80*time.Millisecond, "ws://localhost:8080/ws/battleship")
	go red.Run(ctx)
	go blue.Run(ctx)

	// Battle Loop Implementation Plan:
	// 1. Battle loop: bots take turns guessing cells until one dies
	// 2. Node architecture:
	//    - Each node should maintain its own board state
	//    - Master utility function that iterates through nodes to coordinate shooting
	//    - Trigger communication to global node on each turn
	// 3. Communication protocol:
	//    - Communicate state when a hit occurs
	//    - Update other nodes with hit/miss information
	// 4. Hit strategy:
	//    - If hit, implement "hit nearest neighbor" algorithm for follow-up shots
	// 5. Board configuration:
	//    - Scale to 100x100 board
	//    - Add parameter for configurable number of boats
	// 6. Scaling:
	//    - Support game and nodes scaling to handle larger deployments
	go func() {
		turn := 0 // 0 = red, 1 = blue
		seenRed := map[string]struct{}{}
		seenBlue := map[string]struct{}{}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if board.AliveCount() <= 1 {
				return
			}

			x, y := rand.Intn(10), rand.Intn(10)
			key := fmt.Sprintf("%d,%d", x, y)
			if turn == 0 {
				if _, ok := seenRed[key]; ok {
					time.Sleep(50 * time.Millisecond)
					continue
				}
				seenRed[key] = struct{}{}
				board.Attack(x, y, "red-bot")
				turn = 1
			} else {
				if _, ok := seenBlue[key]; ok {
					time.Sleep(50 * time.Millisecond)
					continue
				}
				seenBlue[key] = struct{}{}
				board.Attack(x, y, "blue-bot")
				turn = 0
			}
			time.Sleep(400 * time.Millisecond)
		}
	}()

	<-ctx.Done()
	close(stopDisplay)
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
}
