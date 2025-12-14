package sparse

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewQuadTree(t *testing.T) {
	tests := []struct {
		name   string
		width  int64
		height int64
	}{
		{"small board", 100, 100},
		{"rectangular", 200, 100},
		{"large board", 10000, 10000},
		{"massive board", 10000000, 10000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qt := NewQuadTree(tt.width, tt.height)
			if qt == nil {
				t.Fatal("NewQuadTree returned nil")
			}
			if qt.Width != tt.width {
				t.Errorf("Width = %d, want %d", qt.Width, tt.width)
			}
			if qt.Height != tt.height {
				t.Errorf("Height = %d, want %d", qt.Height, tt.height)
			}
			if qt.CellCount != 0 {
				t.Errorf("Initial CellCount = %d, want 0", qt.CellCount)
			}
			if !qt.Root.IsLeaf {
				t.Error("Root should be a leaf node initially")
			}
		})
	}
}

func TestQuadTreeInsert(t *testing.T) {
	tests := []struct {
		name    string
		width   int64
		height  int64
		x       int64
		y       int64
		ownerID string
		rackID  string
		want    bool
	}{
		{"valid insert", 100, 100, 50, 50, "player1", "rack1", true},
		{"boundary min", 100, 100, 0, 0, "player1", "rack1", true},
		{"boundary max", 100, 100, 99, 99, "player1", "rack1", true},
		{"out of bounds x negative", 100, 100, -1, 50, "player1", "rack1", false},
		{"out of bounds y negative", 100, 100, 50, -1, "player1", "rack1", false},
		{"out of bounds x too large", 100, 100, 100, 50, "player1", "rack1", false},
		{"out of bounds y too large", 100, 100, 50, 100, "player1", "rack1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qt := NewQuadTree(tt.width, tt.height)
			got := qt.Insert(tt.x, tt.y, tt.ownerID, tt.rackID)
			if got != tt.want {
				t.Errorf("Insert() = %v, want %v", got, tt.want)
			}
			if tt.want {
				if qt.CellCount != 1 {
					t.Errorf("CellCount = %d, want 1", qt.CellCount)
				}
			}
		})
	}
}

func TestQuadTreeInsertUpdate(t *testing.T) {
	qt := NewQuadTree(100, 100)

	// Insert cell
	qt.Insert(50, 50, "player1", "rack1")
	if qt.CellCount != 1 {
		t.Fatalf("CellCount after insert = %d, want 1", qt.CellCount)
	}

	// Update same cell
	qt.Insert(50, 50, "player2", "rack2")
	if qt.CellCount != 1 {
		t.Errorf("CellCount after update = %d, want 1 (should update, not insert)", qt.CellCount)
	}

	cell := qt.Query(50, 50)
	if cell == nil {
		t.Fatal("Query returned nil")
	}
	if cell.OwnerID != "player2" {
		t.Errorf("OwnerID = %s, want player2", cell.OwnerID)
	}
	if cell.RackID != "rack2" {
		t.Errorf("RackID = %s, want rack2", cell.RackID)
	}
}

func TestQuadTreeSubdivision(t *testing.T) {
	qt := NewQuadTree(100, 100)
	qt.MaxCells = 4 // Set low threshold for testing

	// Insert up to MaxCells
	for i := int64(0); i < 4; i++ {
		qt.Insert(i, i, fmt.Sprintf("player%d", i), fmt.Sprintf("rack%d", i))
	}

	if !qt.Root.IsLeaf {
		t.Error("Root should still be leaf with MaxCells items")
	}

	// Insert one more to trigger subdivision
	qt.Insert(4, 4, "player4", "rack4")

	if qt.Root.IsLeaf {
		t.Error("Root should no longer be leaf after exceeding MaxCells")
	}

	if qt.Root.Children[0] == nil {
		t.Error("Children should be created after subdivision")
	}

	// Verify all cells are still queryable
	for i := int64(0); i <= 4; i++ {
		cell := qt.Query(i, i)
		if cell == nil {
			t.Errorf("Query(%d, %d) returned nil after subdivision", i, i)
		}
	}
}

func TestQuadTreeQuery(t *testing.T) {
	qt := NewQuadTree(100, 100)

	// Insert some cells
	qt.Insert(10, 10, "player1", "rack1")
	qt.Insert(50, 50, "player2", "rack2")
	qt.Insert(90, 90, "player3", "rack3")

	tests := []struct {
		name      string
		x         int64
		y         int64
		wantOwner string
		wantNil   bool
	}{
		{"existing cell 1", 10, 10, "player1", false},
		{"existing cell 2", 50, 50, "player2", false},
		{"existing cell 3", 90, 90, "player3", false},
		{"empty cell", 20, 20, "", true},
		{"out of bounds", 100, 100, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := qt.Query(tt.x, tt.y)
			if tt.wantNil {
				if cell != nil {
					t.Errorf("Query() = %v, want nil", cell)
				}
			} else {
				if cell == nil {
					t.Fatal("Query() returned nil, want non-nil")
				}
				if cell.OwnerID != tt.wantOwner {
					t.Errorf("OwnerID = %s, want %s", cell.OwnerID, tt.wantOwner)
				}
			}
		})
	}
}

func TestQuadTreeRangeQuery(t *testing.T) {
	qt := NewQuadTree(100, 100)

	// Insert a grid of cells
	for x := int64(10); x < 20; x++ {
		for y := int64(10); y < 20; y++ {
			qt.Insert(x, y, "player1", "rack1")
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
		{"full range", 10, 10, 20, 20, 100}, // 10x10 grid
		{"partial range", 10, 10, 15, 15, 25}, // 5x5 grid
		{"single cell", 10, 10, 11, 11, 1},
		{"no cells", 50, 50, 60, 60, 0},
		{"overlapping edge", 15, 15, 25, 25, 25}, // 5x5 grid (15-19 in each dimension)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := qt.RangeQuery(tt.minX, tt.minY, tt.maxX, tt.maxY)
			if len(cells) != tt.wantCount {
				t.Errorf("RangeQuery returned %d cells, want %d", len(cells), tt.wantCount)
			}
		})
	}
}

func TestQuadTreeSetHit(t *testing.T) {
	qt := NewQuadTree(100, 100)

	qt.Insert(50, 50, "player1", "rack1")

	// Hit existing cell
	if !qt.SetHit(50, 50) {
		t.Error("SetHit on existing cell should return true")
	}

	cell := qt.Query(50, 50)
	if cell == nil || !cell.WasHit {
		t.Error("Cell should be marked as hit")
	}

	// Hit non-existent cell
	if qt.SetHit(60, 60) {
		t.Error("SetHit on non-existent cell should return false")
	}
}

func TestQuadTreeRemove(t *testing.T) {
	qt := NewQuadTree(100, 100)

	// Insert cells
	qt.Insert(10, 10, "player1", "rack1")
	qt.Insert(20, 20, "player2", "rack2")
	qt.Insert(30, 30, "player3", "rack3")

	initialCount := qt.GetCellCount()
	if initialCount != 3 {
		t.Fatalf("Initial count = %d, want 3", initialCount)
	}

	// Remove existing cell
	if !qt.Remove(20, 20) {
		t.Error("Remove on existing cell should return true")
	}

	if qt.GetCellCount() != 2 {
		t.Errorf("Count after remove = %d, want 2", qt.GetCellCount())
	}

	if qt.Query(20, 20) != nil {
		t.Error("Removed cell should return nil on query")
	}

	// Remove non-existent cell
	if qt.Remove(40, 40) {
		t.Error("Remove on non-existent cell should return false")
	}

	if qt.GetCellCount() != 2 {
		t.Errorf("Count after failed remove = %d, want 2", qt.GetCellCount())
	}
}

func TestQuadTreeClear(t *testing.T) {
	qt := NewQuadTree(100, 100)

	// Insert many cells
	for i := int64(0); i < 50; i++ {
		qt.Insert(i, i, "player1", "rack1")
	}

	if qt.GetCellCount() == 0 {
		t.Fatal("Should have cells before clear")
	}

	qt.Clear()

	if qt.GetCellCount() != 0 {
		t.Errorf("CellCount after clear = %d, want 0", qt.GetCellCount())
	}

	if !qt.Root.IsLeaf {
		t.Error("Root should be leaf after clear")
	}
}

func TestQuadTreeConcurrentAccess(t *testing.T) {
	qt := NewQuadTree(1000, 1000)

	const numGoroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent inserts
	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				x := int64(gid*opsPerGoroutine + i)
				y := int64(gid*opsPerGoroutine + i)
				qt.Insert(x, y, fmt.Sprintf("player%d", gid), "rack1")
			}
		}(g)
	}

	wg.Wait()

	expectedCount := int64(numGoroutines * opsPerGoroutine)
	actualCount := qt.GetCellCount()
	if actualCount != expectedCount {
		t.Errorf("After concurrent inserts: count = %d, want %d", actualCount, expectedCount)
	}

	// Concurrent reads
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				x := int64(gid*opsPerGoroutine + i)
				y := int64(gid*opsPerGoroutine + i)
				cell := qt.Query(x, y)
				if cell == nil {
					t.Errorf("Query(%d, %d) returned nil during concurrent reads", x, y)
				}
			}
		}(g)
	}

	wg.Wait()

	// Concurrent removes
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				x := int64(gid*opsPerGoroutine + i)
				y := int64(gid*opsPerGoroutine + i)
				qt.Remove(x, y)
			}
		}(g)
	}

	wg.Wait()

	if qt.GetCellCount() != 0 {
		t.Errorf("After concurrent removes: count = %d, want 0", qt.GetCellCount())
	}
}

func TestRectContains(t *testing.T) {
	rect := Rect{X: 10, Y: 10, Width: 20, Height: 20}

	tests := []struct {
		name string
		p    Point
		want bool
	}{
		{"inside", Point{15, 15}, true},
		{"top-left corner", Point{10, 10}, true},
		{"bottom-right edge excluded", Point{30, 30}, false},
		{"outside left", Point{5, 15}, false},
		{"outside right", Point{35, 15}, false},
		{"outside top", Point{15, 5}, false},
		{"outside bottom", Point{15, 35}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rect.Contains(tt.p); got != tt.want {
				t.Errorf("Contains(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestRectIntersects(t *testing.T) {
	rect := Rect{X: 10, Y: 10, Width: 20, Height: 20}

	tests := []struct {
		name  string
		other Rect
		want  bool
	}{
		{"fully inside", Rect{15, 15, 5, 5}, true},
		{"fully outside left", Rect{0, 10, 5, 20}, false},
		{"fully outside right", Rect{35, 10, 5, 20}, false},
		{"fully outside top", Rect{10, 0, 20, 5}, false},
		{"fully outside bottom", Rect{10, 35, 20, 5}, false},
		{"overlapping left", Rect{5, 10, 10, 20}, true},
		{"overlapping right", Rect{25, 10, 10, 20}, true},
		{"overlapping top", Rect{10, 5, 20, 10}, true},
		{"overlapping bottom", Rect{10, 25, 20, 10}, true},
		{"same rect", rect, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rect.Intersects(tt.other); got != tt.want {
				t.Errorf("Intersects(%v) = %v, want %v", tt.other, got, tt.want)
			}
		})
	}
}

// Benchmarks

func BenchmarkQuadTreeInsert(b *testing.B) {
	qt := NewQuadTree(10000, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 10000)
		y := int64(i / 10000)
		qt.Insert(x, y, "player1", "rack1")
	}
}

func BenchmarkQuadTreeInsert1M(b *testing.B) {
	qt := NewQuadTree(10000000, 10000000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Distribute across the large space
		x := int64((i * 997) % 10000000)
		y := int64((i * 991) % 10000000)
		qt.Insert(x, y, "player1", "rack1")
	}
}

func BenchmarkQuadTreeQuery(b *testing.B) {
	qt := NewQuadTree(10000, 10000)

	// Pre-populate with cells
	for i := int64(0); i < 1000; i++ {
		for j := int64(0); j < 1000; j++ {
			qt.Insert(i, j, "player1", "rack1")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 1000)
		y := int64(i / 1000 % 1000)
		qt.Query(x, y)
	}
}

func BenchmarkQuadTreeRangeQuery100x100(b *testing.B) {
	qt := NewQuadTree(10000, 10000)

	// Pre-populate with cells
	for i := int64(0); i < 5000; i++ {
		for j := int64(0); j < 5000; j++ {
			qt.Insert(i, j, "player1", "rack1")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Query 100x100 viewport at different positions
		x := int64(i % 4900)
		y := int64(i / 4900 % 4900)
		qt.RangeQuery(x, y, x+100, y+100)
	}
}

func BenchmarkQuadTreeRemove(b *testing.B) {
	qt := NewQuadTree(10000, 10000)

	// Pre-populate with cells
	for i := int64(0); i < 1000; i++ {
		for j := int64(0); j < 1000; j++ {
			qt.Insert(i, j, "player1", "rack1")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := int64(i % 1000)
		y := int64(i / 1000 % 1000)
		qt.Remove(x, y)
	}
}

func BenchmarkQuadTreeConcurrent(b *testing.B) {
	qt := NewQuadTree(10000, 10000)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			x := int64(i % 10000)
			y := int64(i / 10000)
			qt.Insert(x, y, "player1", "rack1")
			qt.Query(x, y)
			i++
		}
	})
}
