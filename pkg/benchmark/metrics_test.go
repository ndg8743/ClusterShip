package benchmark

import (
	"sync"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	if m.CompanyOps == nil {
		t.Error("CompanyOps should not be nil")
	}

	if m.TotalOps.Load() != 0 {
		t.Error("TotalOps should be 0 initially")
	}

	if m.startTime.IsZero() {
		t.Error("startTime should be set")
	}
}

func TestMetricsReset(t *testing.T) {
	m := NewMetrics()

	// Set some values
	m.TotalOps.Store(1000)
	m.TotalBytes.Store(2000)
	m.OpsPerSec.Store(100)
	m.BytesPerSec.Store(200)
	m.WorkerCount.Store(5)
	m.CPUPercent.Store(75)
	m.MemUsedMB.Store(512)
	m.GPUPercent.Store(50)
	m.VRAMUsedMB.Store(1024)
	m.LatencyP50.Store(100)
	m.LatencyP95.Store(500)
	m.LatencyP99.Store(1000)
	m.LatencyMax.Store(2000)
	m.GameFPS.Store(60)
	m.BoardUpdates.Store(100)
	m.AIDecisions.Store(50)
	m.Score.Store(9999)

	m.companyOpsMu.Lock()
	m.CompanyOps["company1"] = 500
	m.CompanyOps["company2"] = 300
	m.companyOpsMu.Unlock()

	originalStartTime := m.startTime

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	m.Reset()

	// Verify all values are reset
	if m.TotalOps.Load() != 0 {
		t.Errorf("TotalOps = %d, want 0", m.TotalOps.Load())
	}
	if m.TotalBytes.Load() != 0 {
		t.Errorf("TotalBytes = %d, want 0", m.TotalBytes.Load())
	}
	if m.OpsPerSec.Load() != 0 {
		t.Errorf("OpsPerSec = %d, want 0", m.OpsPerSec.Load())
	}
	if m.BytesPerSec.Load() != 0 {
		t.Errorf("BytesPerSec = %d, want 0", m.BytesPerSec.Load())
	}
	if m.WorkerCount.Load() != 0 {
		t.Errorf("WorkerCount = %d, want 0", m.WorkerCount.Load())
	}
	if m.CPUPercent.Load() != 0 {
		t.Errorf("CPUPercent = %d, want 0", m.CPUPercent.Load())
	}
	if m.MemUsedMB.Load() != 0 {
		t.Errorf("MemUsedMB = %d, want 0", m.MemUsedMB.Load())
	}
	if m.GPUPercent.Load() != 0 {
		t.Errorf("GPUPercent = %d, want 0", m.GPUPercent.Load())
	}
	if m.VRAMUsedMB.Load() != 0 {
		t.Errorf("VRAMUsedMB = %d, want 0", m.VRAMUsedMB.Load())
	}
	if m.LatencyP50.Load() != 0 {
		t.Errorf("LatencyP50 = %d, want 0", m.LatencyP50.Load())
	}
	if m.LatencyP95.Load() != 0 {
		t.Errorf("LatencyP95 = %d, want 0", m.LatencyP95.Load())
	}
	if m.LatencyP99.Load() != 0 {
		t.Errorf("LatencyP99 = %d, want 0", m.LatencyP99.Load())
	}
	if m.LatencyMax.Load() != 0 {
		t.Errorf("LatencyMax = %d, want 0", m.LatencyMax.Load())
	}
	if m.GameFPS.Load() != 0 {
		t.Errorf("GameFPS = %d, want 0", m.GameFPS.Load())
	}
	if m.BoardUpdates.Load() != 0 {
		t.Errorf("BoardUpdates = %d, want 0", m.BoardUpdates.Load())
	}
	if m.AIDecisions.Load() != 0 {
		t.Errorf("AIDecisions = %d, want 0", m.AIDecisions.Load())
	}
	if m.Score.Load() != 0 {
		t.Errorf("Score = %d, want 0", m.Score.Load())
	}

	m.companyOpsMu.RLock()
	companyOpsCount := len(m.CompanyOps)
	m.companyOpsMu.RUnlock()

	if companyOpsCount != 0 {
		t.Errorf("CompanyOps length = %d, want 0", companyOpsCount)
	}

	// startTime should be updated
	if !m.startTime.After(originalStartTime) {
		t.Error("startTime should be updated after reset")
	}
}

func TestMetricsDuration(t *testing.T) {
	m := NewMetrics()

	// Duration should be very small initially
	duration := m.Duration()
	if duration < 0 {
		t.Errorf("Duration = %v, want >= 0", duration)
	}

	// Wait and check duration increases
	time.Sleep(50 * time.Millisecond)
	duration2 := m.Duration()

	if duration2 < 50*time.Millisecond {
		t.Errorf("Duration = %v, want >= 50ms", duration2)
	}

	if duration2 <= duration {
		t.Error("Duration should increase over time")
	}
}

func TestMetricsCalculateScore(t *testing.T) {
	tests := []struct {
		name       string
		opsPerSec  int64
		latencyP50 int64
		gpuPercent int64
		wantMin    int64
	}{
		{
			name:       "high ops no latency",
			opsPerSec:  1000,
			latencyP50: 0,
			gpuPercent: 0,
			wantMin:    10000, // 1000 * 10
		},
		{
			name:       "high ops low latency",
			opsPerSec:  1000,
			latencyP50: 1000, // 1ms
			gpuPercent: 0,
			wantMin:    10000, // ops score dominates
		},
		{
			name:       "with gpu bonus",
			opsPerSec:  1000,
			latencyP50: 0,
			gpuPercent: 50,
			wantMin:    15000, // 10000 + 5000
		},
		{
			name:       "low ops",
			opsPerSec:  10,
			latencyP50: 0,
			gpuPercent: 0,
			wantMin:    100, // 10 * 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMetrics()
			m.OpsPerSec.Store(tt.opsPerSec)
			m.LatencyP50.Store(tt.latencyP50)
			m.GPUPercent.Store(tt.gpuPercent)

			score := m.CalculateScore()

			if score < tt.wantMin {
				t.Errorf("CalculateScore() = %d, want >= %d", score, tt.wantMin)
			}

			// Verify score is stored
			if m.Score.Load() != score {
				t.Errorf("Score not stored: got %d, want %d", m.Score.Load(), score)
			}
		})
	}
}

func TestMetricsUpdateLatencyPercentiles(t *testing.T) {
	m := NewMetrics()

	// Empty latencies should not panic
	m.UpdateLatencyPercentiles([]time.Duration{})

	// Single latency
	m.UpdateLatencyPercentiles([]time.Duration{time.Millisecond})
	if m.LatencyP50.Load() != 1000 { // 1ms = 1000us
		t.Errorf("LatencyP50 = %d, want 1000", m.LatencyP50.Load())
	}

	// Multiple latencies
	latencies := make([]time.Duration, 100)
	for i := range latencies {
		latencies[i] = time.Duration(i) * time.Microsecond
	}

	m.UpdateLatencyPercentiles(latencies)

	p50 := m.LatencyP50.Load()
	p95 := m.LatencyP95.Load()
	p99 := m.LatencyP99.Load()
	pMax := m.LatencyMax.Load()

	// p50 should be around 50us
	if p50 < 40 || p50 > 60 {
		t.Errorf("LatencyP50 = %d, want ~50", p50)
	}

	// p95 should be around 95us
	if p95 < 85 || p95 > 100 {
		t.Errorf("LatencyP95 = %d, want ~95", p95)
	}

	// p99 should be around 99us
	if p99 < 90 || p99 > 100 {
		t.Errorf("LatencyP99 = %d, want ~99", p99)
	}

	// max should be 99us
	if pMax != 99 {
		t.Errorf("LatencyMax = %d, want 99", pMax)
	}

	// Verify percentiles are in order
	if !(p50 <= p95 && p95 <= p99 && p99 <= pMax) {
		t.Errorf("Percentiles not in order: p50=%d, p95=%d, p99=%d, max=%d", p50, p95, p99, pMax)
	}
}

func TestMetricsGetCompanyOps(t *testing.T) {
	m := NewMetrics()

	// Non-existent company should return 0
	if ops := m.GetCompanyOps("nonexistent"); ops != 0 {
		t.Errorf("GetCompanyOps for non-existent company = %d, want 0", ops)
	}

	// Set company ops
	m.companyOpsMu.Lock()
	m.CompanyOps["company1"] = 1000
	m.CompanyOps["company2"] = 2000
	m.companyOpsMu.Unlock()

	// Verify retrieval
	if ops := m.GetCompanyOps("company1"); ops != 1000 {
		t.Errorf("GetCompanyOps(company1) = %d, want 1000", ops)
	}

	if ops := m.GetCompanyOps("company2"); ops != 2000 {
		t.Errorf("GetCompanyOps(company2) = %d, want 2000", ops)
	}
}

func TestMetricsUpdateMemoryStats(t *testing.T) {
	m := NewMetrics()

	m.UpdateMemoryStats()

	memUsed := m.MemUsedMB.Load()

	// Should have some memory usage (at least a few MB)
	if memUsed <= 0 {
		t.Errorf("MemUsedMB = %d, want > 0", memUsed)
	}

	// Should be reasonable (less than 10GB for this test)
	if memUsed > 10*1024 {
		t.Errorf("MemUsedMB = %d, seems unreasonably high", memUsed)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics()

	// Set various values
	m.TotalOps.Store(1000)
	m.OpsPerSec.Store(100)
	m.BytesPerSec.Store(200)
	m.WorkerCount.Store(5)
	m.CPUPercent.Store(75)
	m.MemUsedMB.Store(512)
	m.GPUPercent.Store(50)
	m.VRAMUsedMB.Store(1024)
	m.LatencyP50.Store(100)
	m.LatencyP95.Store(500)
	m.LatencyP99.Store(1000)
	m.LatencyMax.Store(2000)
	m.GameFPS.Store(60)
	m.BoardUpdates.Store(100)
	m.AIDecisions.Store(50)
	m.Score.Store(9999)

	m.companyOpsMu.Lock()
	m.CompanyOps["company1"] = 500
	m.CompanyOps["company2"] = 300
	m.companyOpsMu.Unlock()

	snapshot := m.Snapshot()

	// Verify all values are captured
	if snapshot.TotalOps != 1000 {
		t.Errorf("Snapshot.TotalOps = %d, want 1000", snapshot.TotalOps)
	}
	if snapshot.OpsPerSec != 100 {
		t.Errorf("Snapshot.OpsPerSec = %d, want 100", snapshot.OpsPerSec)
	}
	if snapshot.BytesPerSec != 200 {
		t.Errorf("Snapshot.BytesPerSec = %d, want 200", snapshot.BytesPerSec)
	}
	if snapshot.WorkerCount != 5 {
		t.Errorf("Snapshot.WorkerCount = %d, want 5", snapshot.WorkerCount)
	}
	if snapshot.CPUPercent != 75 {
		t.Errorf("Snapshot.CPUPercent = %d, want 75", snapshot.CPUPercent)
	}
	if snapshot.MemUsedMB != 512 {
		t.Errorf("Snapshot.MemUsedMB = %d, want 512", snapshot.MemUsedMB)
	}
	if snapshot.GPUPercent != 50 {
		t.Errorf("Snapshot.GPUPercent = %d, want 50", snapshot.GPUPercent)
	}
	if snapshot.VRAMUsedMB != 1024 {
		t.Errorf("Snapshot.VRAMUsedMB = %d, want 1024", snapshot.VRAMUsedMB)
	}
	if snapshot.LatencyP50 != 100 {
		t.Errorf("Snapshot.LatencyP50 = %d, want 100", snapshot.LatencyP50)
	}
	if snapshot.LatencyP95 != 500 {
		t.Errorf("Snapshot.LatencyP95 = %d, want 500", snapshot.LatencyP95)
	}
	if snapshot.LatencyP99 != 1000 {
		t.Errorf("Snapshot.LatencyP99 = %d, want 1000", snapshot.LatencyP99)
	}
	if snapshot.LatencyMax != 2000 {
		t.Errorf("Snapshot.LatencyMax = %d, want 2000", snapshot.LatencyMax)
	}
	if snapshot.GameFPS != 60 {
		t.Errorf("Snapshot.GameFPS = %d, want 60", snapshot.GameFPS)
	}
	if snapshot.BoardUpdates != 100 {
		t.Errorf("Snapshot.BoardUpdates = %d, want 100", snapshot.BoardUpdates)
	}
	if snapshot.AIDecisions != 50 {
		t.Errorf("Snapshot.AIDecisions = %d, want 50", snapshot.AIDecisions)
	}
	if snapshot.Score != 9999 {
		t.Errorf("Snapshot.Score = %d, want 9999", snapshot.Score)
	}

	if snapshot.CompanyOps["company1"] != 500 {
		t.Errorf("Snapshot.CompanyOps[company1] = %d, want 500", snapshot.CompanyOps["company1"])
	}
	if snapshot.CompanyOps["company2"] != 300 {
		t.Errorf("Snapshot.CompanyOps[company2] = %d, want 300", snapshot.CompanyOps["company2"])
	}

	// Verify it's a copy (modifying snapshot shouldn't affect original)
	snapshot.CompanyOps["company1"] = 999
	if m.GetCompanyOps("company1") != 500 {
		t.Error("Snapshot should be independent of original metrics")
	}
}

func TestMetricsConcurrentAccess(t *testing.T) {
	m := NewMetrics()

	const numGoroutines = 10
	const opsPerGoroutine = 1000

	var wg sync.WaitGroup

	// Concurrent atomic operations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.TotalOps.Add(1)
				m.TotalBytes.Add(100)
				m.OpsPerSec.Store(int64(j))
				_ = m.TotalOps.Load()
			}
		}()
	}
	wg.Wait()

	expectedOps := int64(numGoroutines * opsPerGoroutine)
	if m.TotalOps.Load() != expectedOps {
		t.Errorf("TotalOps = %d, want %d", m.TotalOps.Load(), expectedOps)
	}

	// Concurrent company ops access
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(gid int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				// Mix reads and writes
				_ = m.GetCompanyOps("company1")
				m.companyOpsMu.Lock()
				m.CompanyOps["company1"] = int64(j)
				m.companyOpsMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	// Concurrent snapshots
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.Snapshot()
			}
		}()
	}
	wg.Wait()
}

func TestFormatOps(t *testing.T) {
	tests := []struct {
		name string
		ops  int64
		want string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"hundreds", 500, "500"},
		{"thousands", 5000, "5K"},
		{"millions", 5000000, "5M"},
		{"billions", 5000000000, "5B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatOps(tt.ops)
			if got != tt.want {
				t.Errorf("FormatOps(%d) = %s, want %s", tt.ops, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B/s"},
		{"bytes", 500, "500 B/s"},
		{"kilobytes", 5 * 1024, "5 KB/s"},
		{"megabytes", 5 * 1024 * 1024, "5 MB/s"},
		{"gigabytes", 5 * 1024 * 1024 * 1024, "5 GB/s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		name string
		us   int64
		want string
	}{
		{"microseconds", 500, "500us"},
		{"milliseconds", 5000, "5ms"},
		{"seconds", 5000000, "5s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLatency(tt.us)
			if got != tt.want {
				t.Errorf("FormatLatency(%d) = %s, want %s", tt.us, got, tt.want)
			}
		})
	}
}

// Benchmarks

func BenchmarkMetricsAtomicLoad(b *testing.B) {
	m := NewMetrics()
	m.TotalOps.Store(1000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.TotalOps.Load()
	}
}

func BenchmarkMetricsAtomicStore(b *testing.B) {
	m := NewMetrics()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.TotalOps.Store(int64(i))
	}
}

func BenchmarkMetricsAtomicAdd(b *testing.B) {
	m := NewMetrics()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.TotalOps.Add(1)
	}
}

func BenchmarkMetricsConcurrentLoad(b *testing.B) {
	m := NewMetrics()
	m.TotalOps.Store(1000)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.TotalOps.Load()
		}
	})
}

func BenchmarkMetricsConcurrentAdd(b *testing.B) {
	m := NewMetrics()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.TotalOps.Add(1)
		}
	})
}

func BenchmarkMetricsUpdateLatencyPercentiles(b *testing.B) {
	m := NewMetrics()

	latencies := make([]time.Duration, 1000)
	for i := range latencies {
		latencies[i] = time.Duration(i) * time.Microsecond
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.UpdateLatencyPercentiles(latencies)
	}
}

func BenchmarkMetricsSnapshot(b *testing.B) {
	m := NewMetrics()

	// Set up metrics
	m.TotalOps.Store(1000)
	m.companyOpsMu.Lock()
	m.CompanyOps["company1"] = 500
	m.CompanyOps["company2"] = 300
	m.companyOpsMu.Unlock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Snapshot()
	}
}

func BenchmarkMetricsCalculateScore(b *testing.B) {
	m := NewMetrics()
	m.OpsPerSec.Store(10000)
	m.LatencyP50.Store(1000)
	m.GPUPercent.Store(50)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.CalculateScore()
	}
}

func BenchmarkMetricsGetCompanyOps(b *testing.B) {
	m := NewMetrics()

	m.companyOpsMu.Lock()
	m.CompanyOps["company1"] = 1000
	m.CompanyOps["company2"] = 2000
	m.companyOpsMu.Unlock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.GetCompanyOps("company1")
	}
}

func BenchmarkFormatOps(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatOps(5000000)
	}
}

func BenchmarkFormatBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatBytes(5 * 1024 * 1024)
	}
}

func BenchmarkFormatLatency(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatLatency(5000)
	}
}
