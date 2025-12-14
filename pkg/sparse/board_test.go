package sparse

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewSparseBoard(t *testing.T) {
	tests := []struct {
		name   string
		width  int64
		height int64
	}{
		{"small board", 100, 100},
		{"large board", 10000, 10000},
		{"massive board", 10000000, 10000000},
		{"rectangular", 5000, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewSparseBoard(tt.width, tt.height)
			if sb == nil {
				t.Fatal("NewSparseBoard returned nil")
			}
			if sb.Width != tt.width {
				t.Errorf("Width = %d, want %d", sb.Width, tt.width)
			}
			if sb.Height != tt.height {
				t.Errorf("Height = %d, want %d", sb.Height, tt.height)
			}
			if sb.tree == nil {
				t.Error("tree should not be nil")
			}
			if sb.cellOwners == nil {
				t.Error("cellOwners should not be nil")
			}
			if sb.hits == nil {
				t.Error("hits should not be nil")
			}
		})
	}
}

func TestSparseBoardPlaceCell(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	tests := []struct {
		name    string
		x       int64
		y       int64
		ownerID string
		rackID  string
		want    bool
	}{
		{"valid placement", 50, 50, "player1", "rack1", true},
		{"another valid", 25, 75, "player2", "rack2", true},
		{"out of bounds x", 100, 50, "player1", "rack1", false},
		{"out of bounds y", 50, 100, "player1", "rack1", false},
		{"negative x", -1, 50, "player1", "rack1", false},
		{"negative y", 50, -1, "player1", "rack1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sb.PlaceCell(tt.x, tt.y, tt.ownerID, tt.rackID)
			if got != tt.want {
				t.Errorf("PlaceCell() = %v, want %v", got, tt.want)
			}

			if tt.want {
				// Verify cell was placed
				cell := sb.GetCell(tt.x, tt.y)
				if cell == nil {
					t.Error("GetCell returned nil after successful placement")
				} else {
					if cell.OwnerID != tt.ownerID {
						t.Errorf("OwnerID = %s, want %s", cell.OwnerID, tt.ownerID)
					}
					if cell.RackID != tt.rackID {
						t.Errorf("RackID = %s, want %s", cell.RackID, tt.rackID)
					}
				}

				// Verify ownership map
				owner := sb.GetOwner(tt.x, tt.y)
				if owner != tt.ownerID {
					t.Errorf("GetOwner() = %s, want %s", owner, tt.ownerID)
				}

				// Verify IsOccupied
				if !sb.IsOccupied(tt.x, tt.y) {
					t.Error("IsOccupied should return true")
				}
			}
		})
	}
}

func TestSparseBoardGetCell(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	// Place some cells
	sb.PlaceCell(10, 10, "player1", "rack1")
	sb.PlaceCell(50, 50, "player2", "rack2")

	tests := []struct {
		name      string
		x         int64
		y         int64
		wantOwner string
		wantNil   bool
	}{
		{"existing cell 1", 10, 10, "player1", false},
		{"existing cell 2", 50, 50, "player2", false},
		{"empty cell", 20, 20, "", true},
		{"out of bounds", 100, 100, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := sb.GetCell(tt.x, tt.y)
			if tt.wantNil {
				if cell != nil {
					t.Errorf("GetCell() = %v, want nil", cell)
				}
			} else {
				if cell == nil {
					t.Fatal("GetCell() returned nil")
				}
				if cell.OwnerID != tt.wantOwner {
					t.Errorf("OwnerID = %s, want %s", cell.OwnerID, tt.wantOwner)
				}
			}
		})
	}
}

func TestSparseBoardHit(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	// Place a cell
	sb.PlaceCell(50, 50, "player1", "rack1")

	// Hit occupied cell
	if !sb.Hit(50, 50) {
		t.Error("Hit on occupied cell should return true")
	}

	// Verify hit was recorded
	if !sb.WasHit(50, 50) {
		t.Error("WasHit should return true after hit")
	}

	cell := sb.GetCell(50, 50)
	if cell == nil || !cell.WasHit {
		t.Error("Cell should be marked as hit")
	}

	// Hit empty cell
	if sb.Hit(60, 60) {
		t.Error("Hit on empty cell should return false")
	}

	// But it should still be marked as hit in the hits map
	if !sb.WasHit(60, 60) {
		t.Error("Empty cell should still be marked as hit in hits map")
	}
}

func TestSparseBoardGetViewport(t *testing.T) {
	sb := NewSparseBoard(1000, 1000)

	// Place a 10x10 grid of cells
	for x := int64(100); x < 110; x++ {
		for y := int64(100); y < 110; y++ {
			sb.PlaceCell(x, y, "player1", fmt.Sprintf("rack-%d-%d", x, y))
		}
	}

	tests := []struct {
		name      string
		minX      int64
		minY      int64
		maxX      int64
		maxY      int64
		wantCount int
	}{
		{"full viewport", 100, 100, 110, 110, 100},
		{"partial viewport", 100, 100, 105, 105, 25},
		{"single cell", 100, 100, 101, 101, 1},
		{"empty viewport", 200, 200, 210, 210, 0},
		{"overlapping viewport", 105, 105, 115, 115, 25},
		{"edge of board", 0, 0, 10, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := sb.GetViewport(tt.minX, tt.minY, tt.maxX, tt.maxY)
			if len(cells) != tt.wantCount {
				t.Errorf("GetViewport returned %d cells, want %d", len(cells), tt.wantCount)
			}
		})
	}
}

func TestSparseBoardViewportAtEdges(t *testing.T) {
	const boardSize = 10000000
	sb := NewSparseBoard(boardSize, boardSize)

	// Place cells at various edges
	testPositions := []struct {
		name string
		x    int64
		y    int64
	}{
		{"top-left corner", 0, 0},
		{"top-right corner", boardSize - 1, 0},
		{"bottom-left corner", 0, boardSize - 1},
		{"bottom-right corner", boardSize - 1, boardSize - 1},
		{"center", boardSize / 2, boardSize / 2},
	}

	for _, pos := range testPositions {
		sb.PlaceCell(pos.x, pos.y, "player1", pos.name)
	}

	// Test viewports at edges
	tests := []struct {
		name      string
		minX      int64
		minY      int64
		maxX      int64
		maxY      int64
		wantCount int
	}{
		{"top-left viewport", 0, 0, 100, 100, 1},
		{"top-right viewport", boardSize - 100, 0, boardSize, 100, 1},
		{"bottom-left viewport", 0, boardSize - 100, 100, boardSize, 1},
		{"bottom-right viewport", boardSize - 100, boardSize - 100, boardSize, boardSize, 1},
		{"center viewport", boardSize/2 - 50, boardSize/2 - 50, boardSize/2 + 50, boardSize/2 + 50, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := sb.GetViewport(tt.minX, tt.minY, tt.maxX, tt.maxY)
			if len(cells) != tt.wantCount {
				t.Errorf("GetViewport returned %d cells, want %d", len(cells), tt.wantCount)
			}
		})
	}
}

func TestSparseBoardBatchPlaceCells(t *testing.T) {
	sb := NewSparseBoard(1000, 1000)

	cells := []CellPlacement{
		{X: 10, Y: 10, OwnerID: "player1", RackID: "rack1"},
		{X: 20, Y: 20, OwnerID: "player1", RackID: "rack2"},
		{X: 30, Y: 30, OwnerID: "player2", RackID: "rack3"},
		{X: 40, Y: 40, OwnerID: "player2", RackID: "rack4"},
		{X: 1000, Y: 1000, OwnerID: "player3", RackID: "rack5"}, // out of bounds
	}

	count := sb.BatchPlaceCells(cells)
	expectedCount := 4 // 5th is out of bounds
	if count != expectedCount {
		t.Errorf("BatchPlaceCells returned %d, want %d", count, expectedCount)
	}

	if sb.GetCellCount() != int64(expectedCount) {
		t.Errorf("CellCount = %d, want %d", sb.GetCellCount(), expectedCount)
	}

	// Verify cells were placed
	for i, c := range cells[:4] {
		cell := sb.GetCell(c.X, c.Y)
		if cell == nil {
			t.Errorf("Cell %d was not placed", i)
			continue
		}
		if cell.OwnerID != c.OwnerID {
			t.Errorf("Cell %d: OwnerID = %s, want %s", i, cell.OwnerID, c.OwnerID)
		}
	}
}

func TestSparseBoardGetStats(t *testing.T) {
	sb := NewSparseBoard(1000, 1000)

	// Empty board
	stats := sb.GetStats()
	if stats.Width != 1000 {
		t.Errorf("Width = %d, want 1000", stats.Width)
	}
	if stats.Height != 1000 {
		t.Errorf("Height = %d, want 1000", stats.Height)
	}
	if stats.CellCount != 0 {
		t.Errorf("CellCount = %d, want 0", stats.CellCount)
	}
	if stats.HitCount != 0 {
		t.Errorf("HitCount = %d, want 0", stats.HitCount)
	}
	if stats.Density != 0 {
		t.Errorf("Density = %f, want 0", stats.Density)
	}

	// Add some cells
	for i := int64(0); i < 100; i++ {
		sb.PlaceCell(i, i, "player1", "rack1")
	}

	// Hit some cells
	for i := int64(0); i < 50; i++ {
		sb.Hit(i, i)
	}

	stats = sb.GetStats()
	if stats.CellCount != 100 {
		t.Errorf("CellCount = %d, want 100", stats.CellCount)
	}
	if stats.HitCount != 50 {
		t.Errorf("HitCount = %d, want 50", stats.HitCount)
	}

	expectedDensity := 100.0 / (1000.0 * 1000.0)
	if stats.Density != expectedDensity {
		t.Errorf("Density = %f, want %f", stats.Density, expectedDensity)
	}
}

func TestSparseBoardRemoveCell(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	// Place cells
	sb.PlaceCell(10, 10, "player1", "rack1")
	sb.PlaceCell(20, 20, "player2", "rack2")

	// Remove existing cell
	if !sb.RemoveCell(10, 10) {
		t.Error("RemoveCell should return true for existing cell")
	}

	// Verify removal
	if sb.GetCell(10, 10) != nil {
		t.Error("GetCell should return nil after removal")
	}
	if sb.IsOccupied(10, 10) {
		t.Error("IsOccupied should return false after removal")
	}

	// Remove non-existent cell
	if sb.RemoveCell(30, 30) {
		t.Error("RemoveCell should return false for non-existent cell")
	}
}

func TestSparseBoardClear(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	// Place many cells and hit some
	for i := int64(0); i < 50; i++ {
		sb.PlaceCell(i, i, "player1", "rack1")
		if i%2 == 0 {
			sb.Hit(i, i)
		}
	}

	if sb.GetCellCount() == 0 {
		t.Fatal("Should have cells before clear")
	}

	sb.Clear()

	if sb.GetCellCount() != 0 {
		t.Errorf("CellCount after clear = %d, want 0", sb.GetCellCount())
	}

	stats := sb.GetStats()
	if stats.HitCount != 0 {
		t.Errorf("HitCount after clear = %d, want 0", stats.HitCount)
	}

	// Verify maps are empty
	if len(sb.cellOwners) != 0 {
		t.Errorf("cellOwners length = %d, want 0", len(sb.cellOwners))
	}
	if len(sb.hits) != 0 {
		t.Errorf("hits length = %d, want 0", len(sb.hits))
	}
}

func TestSparseBoardEmptyRegions(t *testing.T) {
	const boardSize = 10000000
	sb := NewSparseBoard(boardSize, boardSize)

	// Place cells only in one small region
	for x := int64(1000); x < 1100; x++ {
		for y := int64(1000); y < 1100; y++ {
			sb.PlaceCell(x, y, "player1", "rack1")
		}
	}

	// Query completely empty regions
	emptyRegions := []struct {
		name string
		minX int64
		minY int64
		maxX int64
		maxY int64
	}{
		{"far top-left", 0, 0, 100, 100},
		{"far top-right", boardSize - 100, 0, boardSize, 100},
		{"far bottom-left", 0, boardSize - 100, 100, boardSize},
		{"far bottom-right", boardSize - 100, boardSize - 100, boardSize, boardSize},
		{"far center", boardSize/2 - 50, boardSize/2 - 50, boardSize/2 + 50, boardSize/2 + 50},
	}

	for _, region := range emptyRegions {
		t.Run(region.name, func(t *testing.T) {
			cells := sb.GetViewport(region.minX, region.minY, region.maxX, region.maxY)
			if len(cells) != 0 {
				t.Errorf("GetViewport in empty region returned %d cells, want 0", len(cells))
			}
		})
	}

	// Query the populated region
	cells := sb.GetViewport(1000, 1000, 1100, 1100)
	if len(cells) != 10000 {
		t.Errorf("GetViewport in populated region returned %d cells, want 10000", len(cells))
	}
}

func TestSparseBoardIterateViewport(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	// Place cells
	for x := int64(10); x < 20; x++ {
		for y := int64(10); y < 20; y++ {
			sb.PlaceCell(x, y, "player1", fmt.Sprintf("rack-%d-%d", x, y))
		}
	}

	// Iterate and count
	count := 0
	sb.IterateViewport(10, 10, 20, 20, func(c *Cell) bool {
		count++
		if c.OwnerID != "player1" {
			t.Errorf("Expected OwnerID player1, got %s", c.OwnerID)
		}
		return true // continue iteration
	})

	if count != 100 {
		t.Errorf("Iterated over %d cells, want 100", count)
	}

	// Test early termination
	count = 0
	sb.IterateViewport(10, 10, 20, 20, func(c *Cell) bool {
		count++
		return count < 5 // stop after 5 cells
	})

	if count != 5 {
		t.Errorf("Iterated over %d cells, want 5", count)
	}
}

func TestSparseBoardCountInRegion(t *testing.T) {
	sb := NewSparseBoard(100, 100)

	// Place cells in a 10x10 grid
	for x := int64(10); x < 20; x++ {
		for y := int64(10); y < 20; y++ {
			sb.PlaceCell(x, y, "player1", "rack1")
		}
	}

	tests := []struct {
		name      string
		minX      int64
		minY      int64
		maxX      int64
		maxY      int64
		wantCount int
	}{
		{"full region", 10, 10, 20, 20, 100},
		{"half region", 10, 10, 15, 20, 50},
		{"quarter region", 10, 10, 15, 15, 25},
		{"empty region", 50, 50, 60, 60, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := sb.CountInRegion(tt.minX, tt.minY, tt.maxX, tt.maxY)
			if count != tt.wantCount {
				t.Errorf("CountInRegion = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestSparseBoardConcurrentAccess(t *testing.T) {
	sb := NewSparseBoard(10000, 10000)

	const numGoroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup

	// Concurrent writes
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				x := int64(gid*opsPerGoroutine + i)
				y := int64(gid*opsPerGoroutine + i)
				sb.PlaceCell(x, y, fmt.Sprintf("player%d", gid), "rack1")
			}
		}(g)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				x := int64(gid*opsPerGoroutine + i)
				y := int64(gid*opsPerGoroutine + i)
				cell := sb.GetCell(x, y)
				if cell == nil {
					t.Errorf("GetCell(%d, %d) returned nil", x, y)
				}
			}
		}(g)
	}
	wg.Wait()

	// Concurrent hits
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				x := int64(gid*opsPerGoroutine + i)
				y := int64(gid*opsPerGoroutine + i)
				sb.Hit(x, y)
			}
		}(g)
	}
	wg.Wait()

	// Verify all cells were hit
	stats := sb.GetStats()
	expectedCount := int64(numGoroutines * opsPerGoroutine)
	if stats.HitCount != expectedCount {
		t.Errorf("HitCount = %d, want %d", stats.HitCount, expectedCount)
	}
}

// Benchmarks

func BenchmarkSparseBoardPlaceCell(b *testing.B) {
	sb := NewSparseBoard(10000, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 10000)
		y := int64(i / 10000 % 10000)
		sb.PlaceCell(x, y, "player1", "rack1")
	}
}

func BenchmarkSparseBoardGetCell(b *testing.B) {
	sb := NewSparseBoard(10000, 10000)

	// Pre-populate
	for i := int64(0); i < 1000; i++ {
		for j := int64(0); j < 1000; j++ {
			sb.PlaceCell(i, j, "player1", "rack1")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 1000)
		y := int64(i / 1000 % 1000)
		sb.GetCell(x, y)
	}
}

func BenchmarkSparseBoardHit(b *testing.B) {
	sb := NewSparseBoard(10000, 10000)

	// Pre-populate
	for i := int64(0); i < 1000; i++ {
		for j := int64(0); j < 1000; j++ {
			sb.PlaceCell(i, j, "player1", "rack1")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 1000)
		y := int64(i / 1000 % 1000)
		sb.Hit(x, y)
	}
}

func BenchmarkSparseBoardGetViewport100x100(b *testing.B) {
	sb := NewSparseBoard(10000, 10000)

	// Pre-populate a large region
	for i := int64(0); i < 5000; i++ {
		for j := int64(0); j < 5000; j++ {
			sb.PlaceCell(i, j, "player1", "rack1")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 4900)
		y := int64(i / 4900 % 4900)
		sb.GetViewport(x, y, x+100, y+100)
	}
}

func BenchmarkSparseBoardBatchPlaceCells(b *testing.B) {
	sb := NewSparseBoard(10000, 10000)

	cells := make([]CellPlacement, 100)
	for i := range cells {
		cells[i] = CellPlacement{
			X:       int64(i % 100),
			Y:       int64(i / 100),
			OwnerID: "player1",
			RackID:  "rack1",
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sb.BatchPlaceCells(cells)
	}
}

func BenchmarkSparseBoardGetStats(b *testing.B) {
	sb := NewSparseBoard(10000, 10000)

	// Pre-populate
	for i := int64(0); i < 1000; i++ {
		sb.PlaceCell(i, i, "player1", "rack1")
		sb.Hit(i, i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sb.GetStats()
	}
}
