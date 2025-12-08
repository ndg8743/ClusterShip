# ClusterShip

Distributed battleship game designed for Kubernetes. Ships run as independent containers connecting via WebSocket, bots attack via HTTP API, and the board acts as the control plane.

## Architecture

```
┌─────────────────────────────────────────┐
│      Board (Control Plane)              │
│  - Owns game state                      │
│  - WebSocket: /ws/battleship            │
│  - HTTP: /attack, /view, /stats         │
└─────────────────────────────────────────┘
         ↑ WebSocket              ↑ HTTP
┌────────────────────┐    ┌────────────────────┐
│   Ship Containers  │    │   Bot Containers   │
│   (send heartbeats)│    │   (call /attack)   │
└────────────────────┘    └────────────────────┘
```

## Quick Start

### All-in-one mode
```powershell
go run ./cmd/clustership-game --width=10 --height=10
```

### Distributed mode
```powershell
# Terminal 1: Start board (control plane)
go run ./cmd/clustership-board --width=10 --height=10

# Terminal 2-4: Start ships
go run ./cmd/clustership-ship -id=red-1 -size=4
go run ./cmd/clustership-ship -id=red-2 -size=3
go run ./cmd/clustership-ship -id=blue-1 -size=4

# Terminal 5-6: Start bots
go run ./cmd/clustership-bot -id=red-bot
go run ./cmd/clustership-bot -id=blue-bot
```

## Binaries

| Binary | Purpose |
|--------|---------|
| `clustership-game` | All-in-one game (board + ships + bots) |
| `clustership-board` | Standalone control plane |
| `clustership-ship` | Standalone ship node |
| `clustership-bot` | Standalone bot attacker |

## HTTP API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws/battleship?node_id=X` | WebSocket | Ship heartbeat connection |
| `/attack?bot_id=X` | POST | Attack at `{x, y}` |
| `/view?bot_id=X` | GET | Bot's view of board |
| `/stats` | GET | Board statistics |
| `/healthz` | GET | Health check |

## Build
```powershell
go build -o bin/ ./cmd/...
```

## Test
```powershell
go test ./...
```



