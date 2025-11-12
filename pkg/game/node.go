package game

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// BattleshipNode is a simulated ship that sends periodic state updates to the board.
type BattleshipNode struct {
	ID      string
	X       int
	Y       int
	Health  int
	Size    int
	IsDead  bool
	Latency time.Duration

	serverURL string
	conn      *websocket.Conn
}

// NewBattleshipNode constructs a node with the given id, position, stats, latency, and server URL.
func NewBattleshipNode(id string, x, y, size, health int, latency time.Duration, serverURL string) *BattleshipNode {
	return &BattleshipNode{
		ID:        id,
		X:         x,
		Y:         y,
		Health:    health,
		Size:      size,
		IsDead:    false,
		Latency:   latency,
		serverURL: serverURL,
	}
}

// connect opens the WebSocket to the board and includes node_id in the query string.
func (n *BattleshipNode) connect(ctx context.Context) error {
	u, err := url.Parse(n.serverURL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("node_id", n.ID)
	u.RawQuery = q.Encode()

	c, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return err
	}
	n.conn = c
	return nil
}

// sendState waits for the configured latency and sends a JSON state message.
func (n *BattleshipNode) sendState(ctx context.Context) error {
	if n.conn == nil {
		return fmt.Errorf("connection not established")
	}

	// Simulate latency per send
	if n.Latency > 0 {
		select {
		case <-time.After(n.Latency):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	msg := NodeStateMessage{
		NodeID:    n.ID,
		X:         n.X,
		Y:         n.Y,
		Health:    n.Health,
		Size:      n.Size,
		IsDead:    n.IsDead,
		Timestamp: time.Now().UTC(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return n.conn.WriteMessage(websocket.TextMessage, data)
}

// Run connects to the board and sends a heartbeat every 1–2 seconds.
// node stays put now (no moving), cuz battleship ships don't zip around
func (n *BattleshipNode) Run(ctx context.Context) {
	defer func() {
		if n.conn != nil {
			n.conn.Close()
		}
	}()

	// Attempt initial connect with simple retry
	backoff := time.Millisecond * 200
	for {
		if err := n.connect(ctx); err != nil {
			select {
			case <-time.After(backoff):
				if backoff < time.Second*3 {
					backoff *= 2
				}
				continue
			case <-ctx.Done():
				return
			}
		}
		break
	}

	ticker := time.NewTicker(time.Duration(1000+rand.Intn(1000)) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// just send heartbeat, no move
			_ = n.sendState(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// clamp limits v to the inclusive range [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
