package game

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// DisplayLoop redraws the board once per second until the stop channel is closed.
func (b *GameBoard) DisplayLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if b.Width > 20 || b.Height > 20 {
				b.RenderCompact()
			} else {
				b.RenderBoardsDual()
			}
		case <-stop:
			return
		}
	}
}

// RenderCompact shows stats-only view for large boards (>20x20)
func (b *GameBoard) RenderCompact() {
	width, height, nodes, recent := b.Snapshot()
	shots := b.ShotsSnapshot()
	turn := b.LastTurn()

	// Count stats per team
	redAlive, blueAlive := 0, 0
	redHits, blueHits := 0, 0
	redMiss, blueMiss := 0, 0

	for _, n := range nodes {
		if !n.IsDead {
			if n.Team == "red" {
				redAlive++
			} else if n.Team == "blue" {
				blueAlive++
			}
		}
	}

	for k, hit := range shots["red-bot"] {
		_ = k
		if hit {
			redHits++
		} else {
			redMiss++
		}
	}
	for k, hit := range shots["blue-bot"] {
		_ = k
		if hit {
			blueHits++
		} else {
			blueMiss++
		}
	}

	fmt.Fprint(os.Stdout, "\033[H\033[2J")
	fmt.Println("=== Distributed Battleship Cluster (Compact Mode) ===")
	fmt.Printf("Board: %dx%d | Turn: %s\n\n", width, height, turn)

	fmt.Println("RED TEAM                    BLUE TEAM")
	fmt.Printf("Ships alive: %d              Ships alive: %d\n", redAlive, blueAlive)
	fmt.Printf("Hits: %d                     Hits: %d\n", redHits, blueHits)
	fmt.Printf("Misses: %d                   Misses: %d\n", redMiss, blueMiss)

	fmt.Println("\nShip Status:")
	for _, n := range nodes {
		status := "ALIVE"
		if n.IsDead {
			status = "DEAD"
		}
		fmt.Printf("  %s: HP %d/%d [%s] lat=%s\n", n.ID, n.Health, n.Size, status, n.Latency.Truncate(time.Millisecond))
	}

	fmt.Println("\nRecent Updates:")
	for i := len(recent) - 1; i >= 0; i-- {
		fmt.Println(recent[i])
	}
}

// RenderBoardsDual: show two boards side by side for red/blue bot guesses
func (b *GameBoard) RenderBoardsDual() {
	width, height, nodes, recent := b.Snapshot()
	shots := b.ShotsSnapshot()
	turn := b.LastTurn()

	left := make([][]rune, height)  // red-bot
	right := make([][]rune, height) // blue-bot
	for y := 0; y < height; y++ {
		left[y] = make([]rune, width)
		right[y] = make([]rune, width)
		for x := 0; x < width; x++ {
			left[y][x] = '~'
			right[y][x] = '~'
		}
	}

	// paint shots
	if m := shots["red-bot"]; m != nil {
		for k, hit := range m {
			var x, y int
			_, _ = fmt.Sscanf(k, "%d,%d", &x, &y)
			if y >= 0 && y < height && x >= 0 && x < width {
				if hit {
					left[y][x] = 'X'
				} else {
					left[y][x] = 'o'
				}
			}
		}
	}
	if m := shots["blue-bot"]; m != nil {
		for k, hit := range m {
			var x, y int
			_, _ = fmt.Sscanf(k, "%d,%d", &x, &y)
			if y >= 0 && y < height && x >= 0 && x < width {
				if hit {
					right[y][x] = 'X'
				} else {
					right[y][x] = 'o'
				}
			}
		}
	}

	// now place each team's ships on their own board, after shots
	// keep hits (X) visible; overwrite misses/water
	for _, n := range nodes {
		idLower := strings.ToLower(n.ID)
		isRed := strings.Contains(idLower, "red")
		isBlue := strings.Contains(idLower, "blue")
		var target [][]rune
		if isRed {
			target = left
		} else if isBlue {
			target = right
		} else {
			continue
		}
		vertical := false
		if len(n.Cells) >= 2 {
			vertical = n.Cells[0][0] == n.Cells[1][0]
		}
		ch := '|'
		if !vertical {
			ch = '-'
		}
		for _, cell := range n.Cells {
			x, y := cell[0], cell[1]
			if y >= 0 && y < height && x >= 0 && x < width {
				if target[y][x] != 'X' { // don't hide hits
					target[y][x] = ch
				}
			}
		}
	}

	alive := 0
	total := len(nodes)
	var latencies []time.Duration
	for _, n := range nodes {
		if !n.IsDead {
			alive++
		}
		latencies = append(latencies, n.Latency)
	}
	var avg time.Duration
	if len(latencies) > 0 {
		var sum time.Duration
		for _, l := range latencies {
			sum += l
		}
		avg = sum / time.Duration(len(latencies))
	}

	// Clear screen
	fmt.Fprint(os.Stdout, "\033[H\033[2J")
	fmt.Println("=== Distributed Battleship Cluster ===")
	fmt.Printf("turn: %s | Nodes: %d | Alive: %d | Dead: %d | Avg Latency: %s\n\n", turn, total, alive, total-alive, avg.Truncate(time.Millisecond))

	// headings
	fmt.Println("red-bot shots                         blue-bot shots")
	fmt.Println("Legend: -=horizontal ship, |=vertical ship, X=hit, o=miss, ~=water")

	// header rows (coords)
	gap := "    "
	// build header strings
	leftHeader := "  "
	for x := 0; x < width; x++ {
		leftHeader += fmt.Sprintf(" %d", x)
	}
	rightHeader := leftHeader
	fmt.Println(leftHeader + gap + rightHeader)

	for y := 0; y < height; y++ {
		ls := fmt.Sprintf("%d ", y)
		for x := 0; x < width; x++ {
			ls += fmt.Sprintf(" %c", left[y][x])
		}
		rs := fmt.Sprintf("%d ", y)
		for x := 0; x < width; x++ {
			rs += fmt.Sprintf(" %c", right[y][x])
		}
		fmt.Println(ls + gap + rs)
	}

	fmt.Println("\nRecent Updates:")
	for i := len(recent) - 1; i >= 0; i-- {
		fmt.Println(recent[i])
	}
}

// RenderBoard renders the ASCII grid, per-row summaries, and recent update list.
func (b *GameBoard) RenderBoard() {
	width, height, nodes, recent := b.Snapshot()

	grid := make([][]rune, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, width)
		for x := 0; x < width; x++ {
			grid[y][x] = '~'
		}
	}

	alive := 0
	total := len(nodes)
	var latencies []time.Duration
	for _, n := range nodes {
		ch := 'S'
		if n.IsDead {
			ch = 'X'
		} else {
			alive++
		}
		if n.Y >= 0 && n.Y < height && n.X >= 0 && n.X < width {
			grid[n.Y][n.X] = ch
		}
		latencies = append(latencies, n.Latency)
	}

	var avg time.Duration
	if len(latencies) > 0 {
		var sum time.Duration
		for _, l := range latencies {
			sum += l
		}
		avg = sum / time.Duration(len(latencies))
	}

	// Clear screen (best-effort, works in many terminals)
	fmt.Fprint(os.Stdout, "\033[H\033[2J")
	fmt.Println("=== Distributed Battleship Cluster ===")
	fmt.Printf("Nodes: %d | Alive: %d | Dead: %d | Avg Latency: %s\n\n", total, alive, total-alive, avg.Truncate(time.Millisecond))

	// Header row
	fmt.Print("  ")
	for x := 0; x < width; x++ {
		fmt.Printf(" %d", x)
	}
	fmt.Println()
	for y := 0; y < height; y++ {
		fmt.Printf("%d ", y)
		for x := 0; x < width; x++ {
			fmt.Printf(" %c", grid[y][x])
		}
		// Append line summary if any node on this row
		var onRow []*NodeView
		for _, n := range nodes {
			if n.Y == y {
				onRow = append(onRow, n)
			}
		}
		if len(onRow) > 0 {
			sort.Slice(onRow, func(i, j int) bool { return onRow[i].X < onRow[j].X })
			fmt.Print("  ")
			for idx, n := range onRow {
				if idx > 0 {
					fmt.Print("  ")
				}
				status := fmt.Sprintf("%s (HP: %d/%d) %s %s", n.ID, n.Health, n.Size, latencyMarker(n.Latency), n.Latency.Truncate(time.Millisecond))
				if n.IsDead {
					status = fmt.Sprintf("%s (DEAD) %s %s", n.ID, latencyMarker(n.Latency), n.Latency.Truncate(time.Millisecond))
				}
				fmt.Print(status)
			}
		}
		fmt.Println()
	}

	fmt.Println("\nRecent Updates:")
	for i := len(recent) - 1; i >= 0; i-- {
		fmt.Println(recent[i])
	}
}

// latencyMarker returns an emoji indicating latency severity.
func latencyMarker(lat time.Duration) string {
	ms := lat.Milliseconds()
	if ms < 50 {
		return "🟢"
	}
	if ms <= 200 {
		return "🟡"
	}
	return "🔴"
}
