# ClusterShip

Distributed battleship game where ships run as independent nodes that connect via websocket and report their state. Two bots battle each other using smart targeting.

## Features

- **Configurable board size**: Up to 100x100 (default)
- **Multiple ships per team**: 3 ships with sizes [4, 3, 2]
- **Smart targeting**: Hit-nearest-neighbor algorithm
- **Multiple concurrent games**: Run isolated game instances
- **Compact display**: Stats-only view for large boards

## Prerequisites
- Go 1.22+ installed

## Run
```powershell
# Small board (visual display with ASCII grid)
go run ./cmd/clustership-game --width=10 --height=10

# Default: 100x100 board (compact stats display)
go run ./cmd/clustership-game

# Multiple concurrent games
go run ./cmd/clustership-game --games=3 --width=10 --height=10
```

The game displays an ASCII grid showing both teams' boards side-by-side (for boards ≤20x20) or compact stats (for larger boards). Press `Ctrl+C` to stop.

### CLI Flags
- `--width=100`: Board width (1-100)
- `--height=100`: Board height (1-100)
- `--games=1`: Number of concurrent games

## Build
```powershell
go build -o bin/clustership-game ./cmd/clustership-game
```

## Test
```powershell
go test ./...
```

## Architecture

- **GameBoard**: Central authority for ship state and attacks
- **BattleCoordinator**: Turn-based battle loop with targeting
- **GameManager**: Manages multiple concurrent game instances
- **BattleshipNode**: Ships that send heartbeats via WebSocket



