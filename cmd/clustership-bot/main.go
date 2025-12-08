package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clustership/pkg/game"
)

var (
	botID     = flag.String("id", "", "Bot ID (e.g., red-bot)")
	serverURL = flag.String("server", "http://localhost:8080", "Board HTTP URL")
	gameID    = flag.String("game", "", "Game ID for multi-game mode")
	turnDelay = flag.Duration("delay", 400*time.Millisecond, "Delay between turns")
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	if *botID == "" {
		log.Fatal("bot id required: -id=red-bot")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot := &RemoteBot{
		id:      *botID,
		server:  *serverURL,
		gameID:  *gameID,
		guessed: make(map[string]bool),
		client:  &http.Client{Timeout: 10 * time.Second},
	}

	log.Printf("bot %s starting against %s", *botID, *serverURL)
	bot.Run(ctx, *turnDelay)
}

// RemoteBot attacks the board via HTTP API.
type RemoteBot struct {
	id       string
	server   string
	gameID   string
	guessed  map[string]bool
	hitQueue []game.Coord
	client   *http.Client
}

// Run executes the battle loop until game over.
func (b *RemoteBot) Run(ctx context.Context, delay time.Duration) {
	// Wait for ships to connect
	for {
		view := b.getView()
		if view != nil && view.EnemyAlive > 0 {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		view := b.getView()
		if view == nil {
			time.Sleep(delay)
			continue
		}
		if view.GameOver {
			log.Printf("game over! winner: %s", view.Winner)
			return
		}

		target := b.pickTarget(view)
		if target == nil {
			log.Println("no targets left")
			return
		}

		b.guessed[target.Key()] = true
		hit, killedID := b.attack(target.X, target.Y)

		if hit && killedID == "" {
			b.queueNeighbors(*target, view.Width, view.Height)
		} else if killedID != "" {
			b.hitQueue = nil
		}

		time.Sleep(delay)
	}
}

func (b *RemoteBot) pickTarget(view *game.BotView) *game.Coord {
	// Try queued neighbors first
	for len(b.hitQueue) > 0 {
		next := b.hitQueue[0]
		b.hitQueue = b.hitQueue[1:]
		if !b.guessed[next.Key()] {
			return &next
		}
	}

	// Random search
	for i := 0; i < view.Width*view.Height; i++ {
		x := rand.Intn(view.Width)
		y := rand.Intn(view.Height)
		c := game.Coord{X: x, Y: y}
		if !b.guessed[c.Key()] {
			return &c
		}
	}
	return nil
}

func (b *RemoteBot) queueNeighbors(hit game.Coord, w, h int) {
	neighbors := []game.Coord{
		{X: hit.X, Y: hit.Y - 1},
		{X: hit.X, Y: hit.Y + 1},
		{X: hit.X - 1, Y: hit.Y},
		{X: hit.X + 1, Y: hit.Y},
	}
	for _, n := range neighbors {
		if n.X < 0 || n.X >= w || n.Y < 0 || n.Y >= h {
			continue
		}
		if b.guessed[n.Key()] {
			continue
		}
		b.hitQueue = append(b.hitQueue, n)
	}
}

func (b *RemoteBot) getView() *game.BotView {
	url := fmt.Sprintf("%s/view?bot_id=%s", b.server, b.id)
	if b.gameID != "" {
		url += "&game_id=" + b.gameID
	}

	resp, err := b.client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var view game.BotView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		return nil
	}
	return &view
}

func (b *RemoteBot) attack(x, y int) (bool, string) {
	url := fmt.Sprintf("%s/attack?bot_id=%s", b.server, b.id)
	if b.gameID != "" {
		url += "&game_id=" + b.gameID
	}

	body, _ := json.Marshal(map[string]int{"x": x, "y": y})
	resp, err := b.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result struct {
		Hit      bool   `json:"hit"`
		KilledID string `json:"killed_id"`
	}
	json.Unmarshal(data, &result)
	return result.Hit, result.KilledID
}
