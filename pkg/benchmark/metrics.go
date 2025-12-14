package benchmark

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds real-time benchmark statistics
type Metrics struct {
	// Performance counters
	TotalOps    atomic.Int64
	TotalBytes  atomic.Int64
	OpsPerSec   atomic.Int64
	BytesPerSec atomic.Int64
	WorkerCount atomic.Int64

	// Hardware utilization (updated externally)
	CPUPercent  atomic.Int64 // 0-100
	MemUsedMB   atomic.Int64
	GPUPercent  atomic.Int64 // 0-100
	VRAMUsedMB  atomic.Int64

	// Latency percentiles
	LatencyP50 atomic.Int64 // microseconds
	LatencyP95 atomic.Int64
	LatencyP99 atomic.Int64
	LatencyMax atomic.Int64

	// Game metrics
	GameFPS    atomic.Int64
	BoardUpdates atomic.Int64
	AIDecisions  atomic.Int64

	// Per-company ops
	CompanyOps   map[string]int64
	companyOpsMu sync.RWMutex

	// Benchmark score
	Score atomic.Int64

	// Start time for duration calculation
	startTime time.Time
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		CompanyOps: make(map[string]int64),
		startTime:  time.Now(),
	}
}

// Reset clears all metrics
func (m *Metrics) Reset() {
	m.TotalOps.Store(0)
	m.TotalBytes.Store(0)
	m.OpsPerSec.Store(0)
	m.BytesPerSec.Store(0)
	m.WorkerCount.Store(0)
	m.CPUPercent.Store(0)
	m.MemUsedMB.Store(0)
	m.GPUPercent.Store(0)
	m.VRAMUsedMB.Store(0)
	m.LatencyP50.Store(0)
	m.LatencyP95.Store(0)
	m.LatencyP99.Store(0)
	m.LatencyMax.Store(0)
	m.GameFPS.Store(0)
	m.BoardUpdates.Store(0)
	m.AIDecisions.Store(0)
	m.Score.Store(0)

	m.companyOpsMu.Lock()
	m.CompanyOps = make(map[string]int64)
	m.companyOpsMu.Unlock()

	m.startTime = time.Now()
}

// Duration returns how long the benchmark has been running
func (m *Metrics) Duration() time.Duration {
	return time.Since(m.startTime)
}

// CalculateScore computes a weighted benchmark score
func (m *Metrics) CalculateScore() int64 {
	// Score formula:
	// - Ops/sec contributes most (raw throughput)
	// - Lower latency is better (inverse contribution)
	// - GPU usage adds bonus points

	opsScore := m.OpsPerSec.Load() * 10

	// Latency penalty (lower is better)
	p50 := m.LatencyP50.Load()
	latencyScore := int64(0)
	if p50 > 0 && p50 < 100000 { // < 100ms
		latencyScore = (100000 - p50) / 100
	}

	// GPU bonus
	gpuBonus := m.GPUPercent.Load() * 100

	score := opsScore + latencyScore + gpuBonus
	m.Score.Store(score)
	return score
}

// UpdateLatencyPercentiles calculates percentiles from raw latencies
func (m *Metrics) UpdateLatencyPercentiles(latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate percentiles
	p50idx := len(sorted) * 50 / 100
	p95idx := len(sorted) * 95 / 100
	p99idx := len(sorted) * 99 / 100

	m.LatencyP50.Store(int64(sorted[p50idx].Microseconds()))
	m.LatencyP95.Store(int64(sorted[p95idx].Microseconds()))
	m.LatencyP99.Store(int64(sorted[p99idx].Microseconds()))
	m.LatencyMax.Store(int64(sorted[len(sorted)-1].Microseconds()))
}

// GetCompanyOps returns ops for a specific company
func (m *Metrics) GetCompanyOps(companyID string) int64 {
	m.companyOpsMu.RLock()
	defer m.companyOpsMu.RUnlock()
	return m.CompanyOps[companyID]
}

// UpdateMemoryStats updates memory usage from runtime
func (m *Metrics) UpdateMemoryStats() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.MemUsedMB.Store(int64(memStats.Alloc / 1024 / 1024))
}

// Snapshot returns a copy of current metrics for display
type MetricsSnapshot struct {
	TotalOps     int64
	OpsPerSec    int64
	BytesPerSec  int64
	WorkerCount  int64
	CPUPercent   int64
	MemUsedMB    int64
	GPUPercent   int64
	VRAMUsedMB   int64
	LatencyP50   int64
	LatencyP95   int64
	LatencyP99   int64
	LatencyMax   int64
	GameFPS      int64
	BoardUpdates int64
	AIDecisions  int64
	Score        int64
	Duration     time.Duration
	CompanyOps   map[string]int64
}

// Snapshot creates a point-in-time copy of metrics
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.companyOpsMu.RLock()
	companyOps := make(map[string]int64)
	for k, v := range m.CompanyOps {
		companyOps[k] = v
	}
	m.companyOpsMu.RUnlock()

	return MetricsSnapshot{
		TotalOps:     m.TotalOps.Load(),
		OpsPerSec:    m.OpsPerSec.Load(),
		BytesPerSec:  m.BytesPerSec.Load(),
		WorkerCount:  m.WorkerCount.Load(),
		CPUPercent:   m.CPUPercent.Load(),
		MemUsedMB:    m.MemUsedMB.Load(),
		GPUPercent:   m.GPUPercent.Load(),
		VRAMUsedMB:   m.VRAMUsedMB.Load(),
		LatencyP50:   m.LatencyP50.Load(),
		LatencyP95:   m.LatencyP95.Load(),
		LatencyP99:   m.LatencyP99.Load(),
		LatencyMax:   m.LatencyMax.Load(),
		GameFPS:      m.GameFPS.Load(),
		BoardUpdates: m.BoardUpdates.Load(),
		AIDecisions:  m.AIDecisions.Load(),
		Score:        m.Score.Load(),
		Duration:     m.Duration(),
		CompanyOps:   companyOps,
	}
}

// FormatOps formats ops/sec with appropriate suffix
func FormatOps(ops int64) string {
	switch {
	case ops >= 1000000000:
		return formatFloat(float64(ops)/1000000000) + "B"
	case ops >= 1000000:
		return formatFloat(float64(ops)/1000000) + "M"
	case ops >= 1000:
		return formatFloat(float64(ops)/1000) + "K"
	default:
		return formatInt(ops)
	}
}

// FormatBytes formats bytes/sec with appropriate suffix
func FormatBytes(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return formatFloat(float64(bytes)/(1024*1024*1024)) + " GB/s"
	case bytes >= 1024*1024:
		return formatFloat(float64(bytes)/(1024*1024)) + " MB/s"
	case bytes >= 1024:
		return formatFloat(float64(bytes)/1024) + " KB/s"
	default:
		return formatInt(bytes) + " B/s"
	}
}

// FormatLatency formats latency in microseconds to human readable
func FormatLatency(us int64) string {
	switch {
	case us >= 1000000:
		return formatFloat(float64(us)/1000000) + "s"
	case us >= 1000:
		return formatFloat(float64(us)/1000) + "ms"
	default:
		return formatInt(us) + "us"
	}
}

func formatFloat(f float64) string {
	if f >= 100 {
		return formatInt(int64(f))
	}
	return string(rune(int(f)))[:1] + "." + string(rune(int(f*10)%10+'0'))
}

func formatInt(i int64) string {
	s := ""
	for i > 0 {
		s = string(rune(i%10+'0')) + s
		i /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}
