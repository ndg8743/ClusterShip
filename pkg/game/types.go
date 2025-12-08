package game

import (
	"time"
)

// ShipType describes a kind of ship by its logical size and starting health.
type ShipType struct {
	ID     string
	Size   int
	Health int
}

// GetStandardShips returns a small set of common Battleship pieces.
func GetStandardShips() []ShipType {
	return []ShipType{
		{ID: "carrier-1", Size: 5, Health: 5},
		{ID: "battleship-1", Size: 4, Health: 4},
		{ID: "cruiser-1", Size: 3, Health: 3},
		{ID: "submarine-1", Size: 3, Health: 3},
		{ID: "destroyer-1", Size: 2, Health: 2},
	}
}

// NodeStateMessage is the payload ships send to the board over WebSocket.
type NodeStateMessage struct {
	NodeID    string    `json:"node_id"`
	X         int       `json:"x"`
	Y         int       `json:"y"`
	Health    int       `json:"health"`
	Size      int       `json:"size"`
	IsDead    bool      `json:"isDead"`
	Timestamp time.Time `json:"timestamp"`
	Team      string    `json:"team,omitempty"` // explicit team assignment
}
