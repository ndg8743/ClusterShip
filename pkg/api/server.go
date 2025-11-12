package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"clustership/pkg/game"
)

// Server: http server wrapper
type Server struct {
	Board    *game.GameBoard
	upgrader websocket.Upgrader
}

// NewServer: make a server and allow any origin
func NewServer(board *game.GameBoard) *Server {
	return &Server{
		Board: board,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Routes: register http handlers
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/ws/battleship", s.BattleshipNodeHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// BattleshipNodeHandler: upgrade to ws, read node state messages and feed to board
func (s *Server) BattleshipNodeHandler(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "missing node_id", http.StatusBadRequest)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("node %s disconnected: %v", nodeID, err)
			return
		}
		var msg game.NodeStateMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("bad message from %s: %v", nodeID, err)
			continue
		}
		s.Board.HandleNodeUpdate(msg)
	}
}
