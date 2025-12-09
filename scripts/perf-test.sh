#!/bin/bash
# quick perf test - spawns ships and bots, prints final stats

SHIPS=${1:-5}
WIDTH=${2:-50}
HEIGHT=${3:-50}

echo "Starting board ${WIDTH}x${HEIGHT}, $SHIPS ships/team"
go run ./cmd/clustership-board -width=$WIDTH -height=$HEIGHT -expected-ships=$SHIPS -display=false &
BOARD_PID=$!
sleep 2

for i in $(seq 1 $SHIPS); do
    go run ./cmd/clustership-ship -id=r$i -team=red -size=3 -latency=10 &
    go run ./cmd/clustership-ship -id=b$i -team=blue -size=3 -latency=10 &
done
sleep 3

go run ./cmd/clustership-bot -id=red-bot -delay=50ms &
go run ./cmd/clustership-bot -id=blue-bot -delay=50ms &

wait
curl -s http://localhost:8080/status | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8080/status
kill $BOARD_PID 2>/dev/null
