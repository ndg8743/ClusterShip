package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"clustership/pkg/game"
)

// Server: http server wrapper with multi-game support
type Server struct {
	Board    *game.GameBoard   // legacy single-game support
	Manager  *game.GameManager // multi-game support
	upgrader websocket.Upgrader
}

// NewServer: make a server for single game (backwards compatible)
func NewServer(board *game.GameBoard) *Server {
	return &Server{
		Board: board,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// NewServerWithManager: make a server with multi-game support
func NewServerWithManager(manager *game.GameManager) *Server {
	return &Server{
		Manager: manager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
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
	mux.HandleFunc("/games", s.GamesHandler)
	mux.HandleFunc("/attack", s.AttackHandler)
	mux.HandleFunc("/view", s.ViewHandler)
	mux.HandleFunc("/stats", s.StatsHandler)
}

// BattleshipNodeHandler: upgrade to ws, read node state messages
// Supports game_id query param for multi-game routing
func (s *Server) BattleshipNodeHandler(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	gameID := r.URL.Query().Get("game_id")

	if nodeID == "" {
		http.Error(w, "missing node_id", http.StatusBadRequest)
		return
	}

	// Get the right board (single game or from manager)
	var board *game.GameBoard
	if s.Manager != nil && gameID != "" {
		var ok bool
		board, ok = s.Manager.GetBoard(gameID)
		if !ok {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
	} else if s.Board != nil {
		board = s.Board
	} else {
		http.Error(w, "no game available", http.StatusNotFound)
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
		board.HandleNodeUpdate(msg)
	}
}

// GamesHandler: list active games (GET)
func (s *Server) GamesHandler(w http.ResponseWriter, r *http.Request) {
	if s.Manager == nil {
		http.Error(w, "multi-game not enabled", http.StatusNotImplemented)
		return
	}

	games := s.Manager.ListGames()
	type gameInfo struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}

	list := make([]gameInfo, len(games))
	for i, g := range games {
		list[i] = gameInfo{
			ID:     g.ID,
			Status: int(g.Status),
			Width:  g.Config.BoardWidth,
			Height: g.Config.BoardHeight,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// AttackHandler: POST /attack?bot_id=X&game_id=Y with JSON body {x, y}
func (s *Server) AttackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	botID := r.URL.Query().Get("bot_id")
	gameID := r.URL.Query().Get("game_id")
	if botID == "" {
		http.Error(w, "missing bot_id", http.StatusBadRequest)
		return
	}

	board := s.getBoard(gameID)
	if board == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	var req struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	hit, killedID := board.Attack(req.X, req.Y, botID)

	resp := struct {
		Hit      bool   `json:"hit"`
		KilledID string `json:"killed_id,omitempty"`
	}{Hit: hit, KilledID: killedID}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ViewHandler: GET /view?bot_id=X&game_id=Y returns bot's view of board
func (s *Server) ViewHandler(w http.ResponseWriter, r *http.Request) {
	botID := r.URL.Query().Get("bot_id")
	gameID := r.URL.Query().Get("game_id")
	if botID == "" {
		http.Error(w, "missing bot_id", http.StatusBadRequest)
		return
	}

	board := s.getBoard(gameID)
	if board == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	view := board.GetBotView(botID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(view)
}

// StatsHandler: GET /stats?game_id=Y returns board statistics
func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("game_id")

	board := s.getBoard(gameID)
	if board == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	stats := board.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// getBoard resolves board from game_id or returns single-game board
func (s *Server) getBoard(gameID string) *game.GameBoard {
	if s.Manager != nil && gameID != "" {
		board, ok := s.Manager.GetBoard(gameID)
		if ok {
			return board
		}
		return nil
	}
	return s.Board
}
