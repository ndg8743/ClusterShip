# ClusterShip

Distributed battleship game where ships run as independent nodes that connect via websocket and report their state. Two bots battle each other by taking turns guessing coordinates.

## Current State

- Ships are stationary nodes that connect via websocket
- Two bots (red-bot and blue-bot) take turns guessing random cells
- ASCII display shows both bot's shot boards side by side
- Game ends when one ship is destroyed
- Real-time updates with latency tracking

## Prerequisites
- Go 1.22+ installed

## Run
```powershell
go run ./cmd/clustership-game
```

The game will:
- Start HTTP server on `:8080`
- Show ASCII display updating every second
- Two ships connect and send heartbeats
- Bots take turns attacking until one dies

Press Ctrl+C to stop.

## Build
```powershell
go build -o bin/clustership-game ./cmd/clustership-game
```

## Test
```powershell
go test ./...
```



