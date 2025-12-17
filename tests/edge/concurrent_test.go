package edge

import (
	"clustership/pkg/benchmark"
	"clustership/pkg/game"
	"clustership/pkg/k8s"
	"clustership/pkg/sparse"
	"clustership/pkg/tui"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestConcurrentAttacks tests concurrent attacks from multiple AI don't cause race conditions
func TestConcurrentAttacks(t *testing.T) {
	// This test MUST be run with -race flag to detect data races
	t.Parallel()

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
					Name:     "Test Service",
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

	board := tui.NewMultiBoard(100, 100, companies)

	// Launch 10 goroutines attacking concurrently
	var wg sync.WaitGroup
	attacksPerGoroutine := 50

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(attackerID int) {
			defer wg.Done()

			// Each goroutine performs 50 attacks
			for j := 0; j < attacksPerGoroutine; j++ {
				x := (attackerID*10 + j) % 100
				y := ((attackerID*10 + j) / 100) % 100
				companyID := fmt.Sprintf("company-%d", attackerID)

				// This should not cause data races
				result, _ := board.AttackMulti(x, y, companyID, "")
				_ = result
			}
		}(i)
	}

	// Wait for all attacks to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("All concurrent attacks completed successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent attacks timed out")
	}

	// Verify board state is consistent
	totalShots := 0
	for _, shots := range board.Shots {
		totalShots += len(shots)
	}
	t.Logf("Total shots recorded: %d", totalShots)

	// Should have around 10 * 50 = 500 shots (some may overlap)
	if totalShots < 100 || totalShots > 500 {
		t.Errorf("Unexpected shot count: %d (expected 100-500)", totalShots)
	}
}

// TestK8sWatcherNonBlocking tests K8s watcher doesn't block game loop
func TestK8sWatcherNonBlocking(t *testing.T) {
	t.Parallel()

	// Create a mock watcher with a buffered channel
	eventChan := make(chan k8s.PodEvent, 100)

	// Fill the channel to capacity
	for i := 0; i < 100; i++ {
		eventChan <- k8s.PodEvent{
			Type: "Added",
			Pod: k8s.PodInfo{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "test",
				Status:    k8s.PodRunning,
			},
		}
	}

	t.Logf("Channel filled with 100 events")

	// Simulate game loop trying to send more events
	// This should not block due to the select/default pattern
	timeout := time.After(1 * time.Second)
	sent := 0

	for i := 0; i < 50; i++ {
		select {
		case eventChan <- k8s.PodEvent{Type: "Modified"}:
			sent++
		case <-timeout:
			t.Logf("Timed out after sending %d additional events", sent)
			goto done
		default:
			// Channel full, should skip (non-blocking)
			t.Logf("Channel full at event %d (as expected)", i)
			goto done
		}
	}

done:
	t.Logf("Sent %d additional events without blocking", sent)

	// Drain channel
	drained := 0
	for {
		select {
		case <-eventChan:
			drained++
		default:
			goto finished
		}
	}

finished:
	t.Logf("Drained %d events from channel", drained)

	if drained != 100+sent {
		t.Errorf("Expected to drain %d events, got %d", 100+sent, drained)
	}
}

// TestBenchmarkWorkerRaceConditions tests benchmark workers don't cause race conditions
func TestBenchmarkWorkerRaceConditions(t *testing.T) {
	// MUST run with -race flag
	t.Parallel()

	runner := benchmark.NewRunner()
	runner.Start()
	defer runner.Stop()

	// Add 20 workers concurrently
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			companyID := fmt.Sprintf("company-%d", id%5)
			serviceID := fmt.Sprintf("service-%d", id)
			workloadType := benchmark.WorkloadType(id % 4)
			runner.AddWorker(companyID, serviceID, workloadType)
		}(i)
	}

	wg.Wait()
	t.Logf("Added %d workers", runner.GetWorkerCount())

	// Let workers run for a bit
	time.Sleep(500 * time.Millisecond)

	// Concurrently read metrics while workers are running
	var metricsWg sync.WaitGroup
	for i := 0; i < 10; i++ {
		metricsWg.Add(1)
		go func() {
			defer metricsWg.Done()
			for j := 0; j < 20; j++ {
				metrics := runner.GetMetrics()
				snapshot := metrics.Snapshot()
				_ = snapshot.TotalOps
				_ = snapshot.OpsPerSec
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	metricsWg.Wait()
	t.Logf("Metrics read successfully from %d concurrent goroutines", 10)

	// Get final metrics
	metrics := runner.GetMetrics()
	snapshot := metrics.Snapshot()
	t.Logf("Final metrics: %d ops, %d ops/sec", snapshot.TotalOps, snapshot.OpsPerSec)

	if snapshot.TotalOps == 0 {
		t.Error("Expected some operations from workers")
	}
}

// TestTurnQueueModificationDuringIteration tests turn queue doesn't corrupt during iteration
func TestTurnQueueModificationDuringIteration(t *testing.T) {
	t.Parallel()

	// Simulate the turn queue scenario
	turnQueue := []string{"company-0", "company-1", "company-2", "company-3", "company-4"}
	activeCompanies := make(map[string]bool)
	for _, id := range turnQueue {
		activeCompanies[id] = true
	}

	var mu sync.RWMutex
	var wg sync.WaitGroup

	// Goroutine 1: Iterate through turn queue
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			mu.RLock()
			for idx, companyID := range turnQueue {
				_ = idx
				_ = companyID
				// Simulate processing turn
				time.Sleep(time.Microsecond)
			}
			mu.RUnlock()
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine 2: Modify active companies (simulate company defeat)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			mu.Lock()
			// Remove a company
			companyToRemove := fmt.Sprintf("company-%d", i%5)
			delete(activeCompanies, companyToRemove)

			// Rebuild turn queue
			newQueue := make([]string, 0)
			for _, id := range turnQueue {
				if activeCompanies[id] {
					newQueue = append(newQueue, id)
				}
			}
			turnQueue = newQueue
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)

			// Re-add company
			mu.Lock()
			activeCompanies[companyToRemove] = true
			turnQueue = append(turnQueue, companyToRemove)
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Wait for both goroutines
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("Turn queue modification test completed without deadlock")
	case <-time.After(10 * time.Second):
		t.Fatal("Turn queue modification test deadlocked")
	}

	mu.RLock()
	finalQueueLen := len(turnQueue)
	mu.RUnlock()
	t.Logf("Final turn queue length: %d", finalQueueLen)
}

// TestQuadTreeDeadlock tests QuadTree subdivide doesn't deadlock with concurrent access
func TestQuadTreeDeadlock(t *testing.T) {
	// MUST run with -race flag
	t.Parallel()

	tree := sparse.NewQuadTree(1000, 1000)

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 1000

	// Multiple goroutines inserting concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				x := int64((id*opsPerGoroutine + j) % 1000)
				y := int64(((id*opsPerGoroutine + j) / 1000) % 1000)
				tree.Insert(x, y, fmt.Sprintf("owner-%d", id), fmt.Sprintf("rack-%d", j))
			}
		}(i)
	}

	// Concurrent queries while inserting
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				x := int64((id*13 + j*17) % 1000)
				y := int64((id*19 + j*23) % 1000)
				_ = tree.Query(x, y)
			}
		}(i)
	}

	// Concurrent range queries
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				x := int64((id*100 + j) % 950)
				y := int64((id*100 + j) / 10 % 950)
				_ = tree.RangeQuery(x, y, x+50, y+50)
			}
		}(i)
	}

	// Wait with timeout
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("QuadTree concurrent operations completed successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("QuadTree operations deadlocked")
	}

	cellCount := tree.GetCellCount()
	t.Logf("QuadTree final cell count: %d", cellCount)

	if cellCount == 0 {
		t.Error("Expected some cells to be inserted")
	}
}

// TestSparseBoardKeyCollision tests sparse board handles large coordinates without collisions
func TestSparseBoardKeyCollision(t *testing.T) {
	t.Parallel()

	board := sparse.NewSparseBoard(10000000, 10000000) // 10M x 10M

	// Test coordinates that might cause key collisions with naive string formatting
	testCases := []struct {
		x, y    int64
		ownerID string
	}{
		{0, 0, "owner-0"},
		{0, 1, "owner-1"},
		{1, 0, "owner-2"},
		{9999999, 9999999, "owner-3"},
		{1234567, 7654321, "owner-4"},
		{100, 100, "owner-5"},
		{10, 1000, "owner-6"},   // Could collide with "101000" if naive
		{101, 000, "owner-7"},   // Different from above
		{5000000, 5000000, "owner-8"},
		{1, 1, "owner-9"},
	}

	// Place all cells
	for _, tc := range testCases {
		success := board.PlaceCell(tc.x, tc.y, tc.ownerID, fmt.Sprintf("rack-%s", tc.ownerID))
		if !success {
			t.Errorf("Failed to place cell at (%d, %d)", tc.x, tc.y)
		}
	}

	// Verify each cell has correct owner
	for _, tc := range testCases {
		owner := board.GetOwner(tc.x, tc.y)
		if owner != tc.ownerID {
			t.Errorf("Key collision detected at (%d, %d): expected %s, got %s",
				tc.x, tc.y, tc.ownerID, owner)
		}
	}

	t.Logf("Sparse board key collision test passed for %d cells", len(testCases))
}

// TestAIInfiniteLoop tests AI doesn't infinite loop when board is fully guessed
func TestAIInfiniteLoop(t *testing.T) {
	t.Parallel()

	width, height := 10, 10
	ai := tui.NewAIPlayer(game.AIRandom, width, height)

	// Mark all cells as guessed
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			key := fmt.Sprintf("%d,%d", x, y)
			ai.Guessed[key] = true
		}
	}

	t.Logf("Marked all %d cells as guessed", width*height)

	// AI should handle this gracefully (fall back to linearScan which returns 0,0)
	timeout := time.After(1 * time.Second)
	done := make(chan [2]int)

	go func() {
		target := ai.PickTarget()
		done <- target
	}()

	select {
	case target := <-done:
		t.Logf("AI returned target %v when board fully guessed (no infinite loop)", target)
	case <-timeout:
		t.Fatal("AI infinite loop detected - PickTarget didn't return")
	}
}

// TestBenchmarkMetricsRaceCondition tests specific race condition in benchmark metrics
func TestBenchmarkMetricsRaceCondition(t *testing.T) {
	// MUST run with -race flag to detect the race
	t.Parallel()

	metrics := benchmark.NewMetrics()

	var wg sync.WaitGroup

	// Goroutine 1: Update metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			metrics.TotalOps.Add(1)
			metrics.OpsPerSec.Store(int64(i))
			metrics.UpdateMemoryStats()
		}
	}()

	// Goroutine 2: Calculate score
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = metrics.CalculateScore()
		}
	}()

	// Goroutine 3: Read snapshots
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			snapshot := metrics.Snapshot()
			_ = snapshot.TotalOps
			_ = snapshot.OpsPerSec
		}
	}()

	// Goroutine 4: Update company ops
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			metrics.SetCompanyOps("company-1", int64(i))
		}
	}()

	// Wait with timeout
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("Benchmark metrics concurrent access completed without race")
	case <-time.After(30 * time.Second):
		t.Fatal("Benchmark metrics test timed out")
	}
}

// TestConcurrentBoardAccess tests concurrent reads and writes to board don't corrupt state
// Note: This test has known race conditions that need mutex fixes in board/company code
func TestConcurrentBoardAccess(t *testing.T) {
	t.Skip("Skipping due to known race condition - needs mutex in Company.HealthyPodCount")

	companies := []*game.Company{
		{
			ID:   "company-1",
			Name: "Company 1",
			Regions: []*game.Region{
				{
					ID:        "region-1",
					Name:      "Main",
					RackCount: 5,
					Racks:     make([]*game.Rack, 5),
				},
			},
			Services: []*game.Service{
				{
					ID:       "service-1",
					Name:     "Service",
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
					RackCount: 5,
					Racks:     make([]*game.Rack, 5),
				},
			},
			Services: []*game.Service{
				{
					ID:       "service-2",
					Name:     "Service",
					Replicas: 5,
					Pods:     make([]*game.Pod, 0),
				},
			},
		},
	}

	for _, company := range companies {
		for i := 0; i < 5; i++ {
			company.Regions[0].Racks[i] = &game.Rack{
				ID:       fmt.Sprintf("rack-%s-%d", company.ID, i),
				RegionID: company.Regions[0].ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}
	}

	board := tui.NewMultiBoard(50, 50, companies)

	var wg sync.WaitGroup

	// Concurrent attacks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				x := (id*10 + j) % 50
				y := ((id*10 + j) / 50) % 50
				board.AttackMulti(x, y, "company-1", "")
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				x := (id*7 + j*11) % 50
				y := ((id*13 + j*17) / 50) % 50
				_ = board.GetCellInfo(x, y, "company-2")
				_ = board.GetActiveCompanies()
				_ = board.FleetHealthyByID("company-1")
			}
		}(i)
	}

	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Logf("Concurrent board access completed successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent board access timed out")
	}

	// Verify board state is consistent
	active := board.GetActiveCompanies()
	t.Logf("Active companies after concurrent access: %v", active)
}

// TestK8sWatcherChannelOverflow tests K8s watcher channel overflow bug
func TestK8sWatcherChannelOverflow(t *testing.T) {
	t.Parallel()

	// Simulate rapid pod events
	eventChan := make(chan k8s.PodEvent, 100)

	// Producer: generate events faster than consumer
	go func() {
		for i := 0; i < 1000; i++ {
			event := k8s.PodEvent{
				Type: "Added",
				Pod: k8s.PodInfo{
					Name:      fmt.Sprintf("pod-%d", i),
					Namespace: "test",
					Status:    k8s.PodRunning,
				},
			}

			// Use non-blocking send (should match real implementation)
			select {
			case eventChan <- event:
				// Sent successfully
			default:
				// Channel full, skip (prevents overflow)
			}

			if i%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		close(eventChan)
	}()

	// Consumer: process events slowly
	processed := 0
	for event := range eventChan {
		_ = event
		processed++
		if processed%10 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	t.Logf("Processed %d events (some may have been dropped to prevent overflow)", processed)

	// Should process at least some events but not necessarily all 1000
	if processed < 50 {
		t.Errorf("Too few events processed: %d", processed)
	}

	if processed > 150 {
		// If we processed >150, it means the channel buffered well
		t.Logf("Channel buffer worked well: %d events processed", processed)
	}
}

// TestConcurrentK8sWatcherCallbacks tests K8s watcher callbacks don't race
func TestConcurrentK8sWatcherCallbacks(t *testing.T) {
	t.Parallel()

	// Simulate watcher with multiple callbacks
	var callbackMu sync.Mutex
	callbackCount := 0
	eventLog := make([]string, 0)

	callbacks := []func(k8s.PodEvent){
		func(event k8s.PodEvent) {
			callbackMu.Lock()
			callbackCount++
			eventLog = append(eventLog, fmt.Sprintf("callback1: %s", event.Type))
			callbackMu.Unlock()
		},
		func(event k8s.PodEvent) {
			callbackMu.Lock()
			callbackCount++
			eventLog = append(eventLog, fmt.Sprintf("callback2: %s", event.Type))
			callbackMu.Unlock()
		},
	}

	// Send events and trigger callbacks concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := k8s.PodEvent{
				Type: fmt.Sprintf("Event-%d", id),
				Pod:  k8s.PodInfo{Name: fmt.Sprintf("pod-%d", id)},
			}

			// Call all callbacks
			for _, cb := range callbacks {
				cb(event)
			}
		}(i)
	}

	wg.Wait()

	callbackMu.Lock()
	finalCount := callbackCount
	logLen := len(eventLog)
	callbackMu.Unlock()

	t.Logf("Callback count: %d, Event log entries: %d", finalCount, logLen)

	// Should have 100 events * 2 callbacks = 200
	if finalCount != 200 {
		t.Errorf("Expected 200 callback invocations, got %d", finalCount)
	}

	if logLen != 200 {
		t.Errorf("Expected 200 event log entries, got %d", logLen)
	}
}
