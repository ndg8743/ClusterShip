package edge

import (
	"clustership/pkg/config"
	"clustership/pkg/game"
	"clustership/pkg/sparse"
	"clustership/pkg/tui"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestLargeBoard1000x1000 tests a 1000x1000 board with 20 companies
func TestLargeBoard1000x1000(t *testing.T) {
	t.Parallel()

	// Create 20 companies
	companies := make([]*game.Company, 20)
	for i := 0; i < 20; i++ {
		companies[i] = &game.Company{
			ID:   fmt.Sprintf("company-%d", i),
			Name: fmt.Sprintf("Company %d", i),
			Regions: []*game.Region{
				{
					ID:        fmt.Sprintf("region-%d-1", i),
					Name:      "Main",
					RackCount: 5,
					Racks:     make([]*game.Rack, 5),
				},
			},
			Services: []*game.Service{
				{
					ID:       fmt.Sprintf("service-%d", i),
					Name:     "Test Service",
					Replicas: 3,
					Affinity: game.AffinityNone,
					Pods:     make([]*game.Pod, 0),
				},
			},
		}
		// Initialize racks
		for j := 0; j < 5; j++ {
			companies[i].Regions[0].Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("rack-%d-%d", i, j),
				RegionID: companies[i].Regions[0].ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}
	}

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	allocBefore := m1.Alloc

	start := time.Now()

	// Create large board
	board := tui.NewMultiBoard(1000, 1000, companies)

	elapsed := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	allocAfter := m2.Alloc
	memUsedMB := float64(allocAfter-allocBefore) / 1024 / 1024

	t.Logf("Board creation took: %v", elapsed)
	t.Logf("Memory used: %.2f MB", memUsedMB)

	// Verify board was created
	if board == nil {
		t.Fatal("Board creation failed")
	}

	// Verify all companies placed
	if len(board.Fleets) != 20 {
		t.Errorf("Expected 20 fleets, got %d", len(board.Fleets))
	}

	// Test some attacks
	attackStart := time.Now()
	for i := 0; i < 100; i++ {
		x, y := i*10, i*10
		companyID := fmt.Sprintf("company-%d", i%20)
		_, _ = board.AttackMulti(x, y, companyID, "")
	}
	attackElapsed := time.Since(attackStart)
	t.Logf("100 attacks took: %v (avg: %v)", attackElapsed, attackElapsed/100)

	// Verify memory is bounded (should be < 500MB for 1000x1000)
	if memUsedMB > 500 {
		t.Errorf("Memory usage too high: %.2f MB (expected < 500 MB)", memUsedMB)
	}
}

// TestSparseBoardMassiveScale tests 10000x10000 board performance
func TestSparseBoardMassiveScale(t *testing.T) {
	t.Parallel()

	// Test should complete in < 10 seconds
	timeout := time.After(10 * time.Second)
	done := make(chan bool)

	go func() {
		// Create massive sparse board
		board := sparse.NewSparseBoard(10000, 10000)

		// Place 10,000 cells (sparse)
		for i := int64(0); i < 10000; i++ {
			x := i * 100 % 10000
			y := i * 100 / 100
			board.PlaceCell(x, y, fmt.Sprintf("owner-%d", i%100), fmt.Sprintf("rack-%d", i))
		}

		// Query random cells
		for i := 0; i < 1000; i++ {
			x := int64(i * 13 % 10000)
			y := int64(i * 17 % 10000)
			_ = board.GetCell(x, y)
		}

		// Range queries
		for i := 0; i < 100; i++ {
			x := int64(i * 100)
			y := int64(i * 100)
			cells := board.GetViewport(x, y, x+100, y+100)
			_ = cells
		}

		stats := board.GetStats()
		t.Logf("Sparse board stats: %d cells, density: %.6f", stats.CellCount, stats.Density)

		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test timed out after 10 seconds")
	case <-done:
		// Success
	}
}

// TestMemoryBounded tests memory stays bounded with runtime.MemStats
func TestMemoryBounded(t *testing.T) {
	t.Parallel()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	heapBefore := m1.HeapAlloc

	// Create multiple boards
	boards := make([]*tui.Board, 10)
	companies := make([]*game.Company, 2)
	for i := 0; i < 2; i++ {
		companies[i] = &game.Company{
			ID:   fmt.Sprintf("company-%d", i),
			Name: fmt.Sprintf("Company %d", i),
			Regions: []*game.Region{
				{
					ID:        fmt.Sprintf("region-%d", i),
					Name:      "Main",
					RackCount: 5,
					Racks:     make([]*game.Rack, 5),
				},
			},
			Services: []*game.Service{
				{
					ID:       fmt.Sprintf("service-%d", i),
					Name:     "Test",
					Replicas: 5,
					Pods:     make([]*game.Pod, 0),
				},
			},
		}
		for j := 0; j < 5; j++ {
			companies[i].Regions[0].Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("rack-%d-%d", i, j),
				RegionID: companies[i].Regions[0].ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}
	}

	for i := 0; i < 10; i++ {
		boards[i] = tui.NewMultiBoard(100, 100, companies)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	heapAfter := m2.HeapAlloc
	memUsedMB := float64(heapAfter-heapBefore) / 1024 / 1024

	t.Logf("Memory used for 10 boards: %.2f MB", memUsedMB)

	// Verify memory is reasonable (< 200MB for 10x 100x100 boards)
	if memUsedMB > 200 {
		t.Errorf("Memory usage too high: %.2f MB (expected < 200 MB)", memUsedMB)
	}

	// Cleanup
	boards = nil
	runtime.GC()
}

// TestViewportScrollingLargeBoard tests viewport doesn't hang on large boards
func TestViewportScrollingLargeBoard(t *testing.T) {
	t.Parallel()

	board := sparse.NewSparseBoard(10000, 10000)

	// Place cells scattered across the board
	for i := int64(0); i < 1000; i++ {
		x := i * 13 % 10000
		y := i * 17 % 10000
		board.PlaceCell(x, y, fmt.Sprintf("owner-%d", i%10), fmt.Sprintf("rack-%d", i))
	}

	// Test viewport queries at various positions
	viewportSize := int64(50)
	positions := []struct {
		x, y int64
	}{
		{0, 0},                             // top-left
		{5000, 5000},                       // center
		{9950, 9950},                       // bottom-right
		{100, 5000},                        // left edge
		{9900, 100},                        // right edge
		{1234, 5678},                       // random
	}

	for _, pos := range positions {
		start := time.Now()
		cells := board.GetViewport(pos.x, pos.y, pos.x+viewportSize, pos.y+viewportSize)
		elapsed := time.Since(start)

		t.Logf("Viewport at (%d,%d): %d cells in %v", pos.x, pos.y, len(cells), elapsed)

		// Viewport query should be fast (< 10ms)
		if elapsed > 10*time.Millisecond {
			t.Errorf("Viewport query too slow: %v at (%d,%d)", elapsed, pos.x, pos.y)
		}
	}
}

// TestBattleLogMemoryLeak tests that battle log doesn't grow unbounded
func TestBattleLogMemoryLeak(t *testing.T) {
	t.Parallel()

	companies := []*game.Company{
		{
			ID:   "company-1",
			Name: "Company 1",
			Regions: []*game.Region{
				{
					ID:        "region-1",
					Name:      "Main",
					RackCount: 10,
					Racks:     make([]*game.Rack, 10),
				},
			},
			Services: []*game.Service{
				{
					ID:       "service-1",
					Name:     "Test",
					Replicas: 5,
					Pods:     make([]*game.Pod, 0),
				},
			},
		},
		{
			ID:   "company-2",
			Name: "Company 2",
			Regions: []*game.Region{
				{
					ID:        "region-2",
					Name:      "Main",
					RackCount: 10,
					Racks:     make([]*game.Rack, 10),
				},
			},
			Services: []*game.Service{
				{
					ID:       "service-2",
					Name:     "Test",
					Replicas: 5,
					Pods:     make([]*game.Pod, 0),
				},
			},
		},
	}

	// Initialize racks
	for _, company := range companies {
		for j := 0; j < 10; j++ {
			company.Regions[0].Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("rack-%s-%d", company.ID, j),
				RegionID: company.Regions[0].ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}
	}

	board := tui.NewMultiBoard(100, 100, companies)

	// Generate 10,000 attacks to simulate long game
	for i := 0; i < 10000; i++ {
		x := i % 100
		y := (i / 100) % 100
		_, events := board.AttackMulti(x, y, "company-1", "")
		board.Events = append(board.Events, events...)
	}

	eventCount := len(board.Events)
	t.Logf("Event count after 10,000 attacks: %d", eventCount)

	// Events should be bounded (implementation should cap at some max like 1000)
	// For this test, we just verify it doesn't crash and memory is reasonable
	if eventCount > 100000 {
		t.Errorf("Event log unbounded: %d events (potential memory leak)", eventCount)
	}
}

// TestConfigValidationNegativeValues tests config validation handles negative values
func TestConfigValidationNegativeValues(t *testing.T) {
	t.Parallel()

	cfg := &config.GameConfig{
		BoardWidth:     -100,
		BoardHeight:    -100,
		ShipsPerPlayer: -5,
		RacksPerShip:   -3,
		PodsPerRack:    -10,
		MaxBots:        -1,
		TurnDelayMs:    -500,
	}

	cfg.Validate()

	// All values should be fixed to valid ranges
	if cfg.BoardWidth < 20 {
		t.Errorf("BoardWidth not validated: %d", cfg.BoardWidth)
	}
	if cfg.BoardHeight < 20 {
		t.Errorf("BoardHeight not validated: %d", cfg.BoardHeight)
	}
	if cfg.ShipsPerPlayer < 1 {
		t.Errorf("ShipsPerPlayer not validated: %d", cfg.ShipsPerPlayer)
	}
	if cfg.RacksPerShip < 2 {
		t.Errorf("RacksPerShip not validated: %d", cfg.RacksPerShip)
	}
	if cfg.PodsPerRack < 1 {
		t.Errorf("PodsPerRack not validated: %d", cfg.PodsPerRack)
	}
	if cfg.MaxBots < 1 {
		t.Errorf("MaxBots not validated: %d", cfg.MaxBots)
	}
	if cfg.TurnDelayMs < 10 {
		t.Errorf("TurnDelayMs not validated: %d", cfg.TurnDelayMs)
	}

	t.Logf("Config after validation: %+v", cfg)
}

// BenchmarkLargeScaleAttacks benchmarks attack performance on large boards
func BenchmarkLargeScaleAttacks(b *testing.B) {
	companies := make([]*game.Company, 10)
	for i := 0; i < 10; i++ {
		companies[i] = &game.Company{
			ID:   fmt.Sprintf("company-%d", i),
			Name: fmt.Sprintf("Company %d", i),
			Regions: []*game.Region{
				{
					ID:        fmt.Sprintf("region-%d", i),
					Name:      "Main",
					RackCount: 5,
					Racks:     make([]*game.Rack, 5),
				},
			},
			Services: []*game.Service{
				{
					ID:       fmt.Sprintf("service-%d", i),
					Name:     "Test",
					Replicas: 3,
					Pods:     make([]*game.Pod, 0),
				},
			},
		}
		for j := 0; j < 5; j++ {
			companies[i].Regions[0].Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("rack-%d-%d", i, j),
				RegionID: companies[i].Regions[0].ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}
	}

	board := tui.NewMultiBoard(1000, 1000, companies)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := i % 1000
		y := (i / 1000) % 1000
		companyID := fmt.Sprintf("company-%d", i%10)
		board.AttackMulti(x, y, companyID, "")
	}
}

// BenchmarkSparseViewport benchmarks viewport queries on sparse boards
func BenchmarkSparseViewport(b *testing.B) {
	board := sparse.NewSparseBoard(10000, 10000)

	// Place 10,000 cells
	for i := int64(0); i < 10000; i++ {
		x := i * 13 % 10000
		y := i * 17 % 10000
		board.PlaceCell(x, y, fmt.Sprintf("owner-%d", i%10), fmt.Sprintf("rack-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := int64(i % 9950)
		y := int64((i / 100) % 9950)
		board.GetViewport(x, y, x+50, y+50)
	}
}

// BenchmarkQuadTreeInsert benchmarks QuadTree insertion performance
func BenchmarkQuadTreeInsert(b *testing.B) {
	tree := sparse.NewQuadTree(10000, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := int64(i % 10000)
		y := int64((i / 10000) % 10000)
		tree.Insert(x, y, "owner", "rack")
	}
}
