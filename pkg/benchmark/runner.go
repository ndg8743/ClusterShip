package benchmark

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// WorkloadType defines the type of benchmark workload
type WorkloadType int

const (
	WorkloadCPU WorkloadType = iota
	WorkloadMemory
	WorkloadNetwork
	WorkloadGPU
)

func (w WorkloadType) String() string {
	switch w {
	case WorkloadCPU:
		return "CPU"
	case WorkloadMemory:
		return "Memory"
	case WorkloadNetwork:
		return "Network"
	case WorkloadGPU:
		return "GPU"
	default:
		return "Unknown"
	}
}

// Worker represents a benchmark worker goroutine
type Worker struct {
	ID           string
	CompanyID    string
	ServiceID    string
	WorkloadType WorkloadType
	OpsCount     atomic.Int64
	BytesProc    atomic.Int64
	Latencies    []time.Duration
	latencyMu    sync.Mutex
	running      atomic.Bool
	cancel       context.CancelFunc
}

// Runner manages multiple benchmark workers
type Runner struct {
	workers    []*Worker
	workersMu  sync.RWMutex
	metrics    *Metrics
	startTime  time.Time
	running    atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewRunner creates a new benchmark runner
func NewRunner() *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		workers:   make([]*Worker, 0),
		metrics:   NewMetrics(),
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}
}

// Start begins the benchmark
func (r *Runner) Start() {
	r.running.Store(true)
	r.startTime = time.Now()

	// Start metrics collection
	go r.collectMetrics()
}

// Stop stops all workers and the benchmark
func (r *Runner) Stop() {
	r.running.Store(false)
	r.cancel()

	r.workersMu.Lock()
	for _, w := range r.workers {
		w.Stop()
	}
	r.workersMu.Unlock()
}

// IsRunning returns true if the benchmark is running
func (r *Runner) IsRunning() bool {
	return r.running.Load()
}

// AddWorker creates and starts a new worker
func (r *Runner) AddWorker(companyID, serviceID string, wtype WorkloadType) *Worker {
	ctx, cancel := context.WithCancel(r.ctx)
	w := &Worker{
		ID:           generateWorkerID(),
		CompanyID:    companyID,
		ServiceID:    serviceID,
		WorkloadType: wtype,
		Latencies:    make([]time.Duration, 0, 1000),
		cancel:       cancel,
	}

	r.workersMu.Lock()
	r.workers = append(r.workers, w)
	r.workersMu.Unlock()

	go w.Run(ctx)
	return w
}

// RemoveWorker stops and removes a worker
func (r *Runner) RemoveWorker(workerID string) {
	r.workersMu.Lock()
	defer r.workersMu.Unlock()

	for i, w := range r.workers {
		if w.ID == workerID {
			w.Stop()
			r.workers = append(r.workers[:i], r.workers[i+1:]...)
			return
		}
	}
}

// GetWorkerCount returns the number of active workers
func (r *Runner) GetWorkerCount() int {
	r.workersMu.RLock()
	defer r.workersMu.RUnlock()
	return len(r.workers)
}

// GetMetrics returns current benchmark metrics
func (r *Runner) GetMetrics() *Metrics {
	return r.metrics
}

// collectMetrics periodically updates metrics from workers
func (r *Runner) collectMetrics() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.updateMetrics()
		}
	}
}

// updateMetrics aggregates metrics from all workers
func (r *Runner) updateMetrics() {
	r.workersMu.RLock()
	defer r.workersMu.RUnlock()

	var totalOps int64
	var totalBytes int64
	companyOps := make(map[string]int64)

	for _, w := range r.workers {
		ops := w.OpsCount.Load()
		bytes := w.BytesProc.Load()
		totalOps += ops
		totalBytes += bytes
		companyOps[w.CompanyID] += ops
	}

	elapsed := time.Since(r.startTime).Seconds()
	if elapsed > 0 {
		r.metrics.OpsPerSec.Store(int64(float64(totalOps) / elapsed))
		r.metrics.BytesPerSec.Store(int64(float64(totalBytes) / elapsed))
	}

	r.metrics.TotalOps.Store(totalOps)
	r.metrics.TotalBytes.Store(totalBytes)
	r.metrics.WorkerCount.Store(int64(len(r.workers)))

	r.metrics.companyOpsMu.Lock()
	r.metrics.CompanyOps = companyOps
	r.metrics.companyOpsMu.Unlock()
}

// Run executes the worker's benchmark loop
func (w *Worker) Run(ctx context.Context) {
	w.running.Store(true)
	defer w.running.Store(false)

	switch w.WorkloadType {
	case WorkloadCPU:
		w.runCPUBench(ctx)
	case WorkloadMemory:
		w.runMemoryBench(ctx)
	case WorkloadNetwork:
		w.runNetworkBench(ctx)
	case WorkloadGPU:
		w.runGPUBench(ctx)
	}
}

// Stop stops the worker
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

// IsRunning returns true if the worker is running
func (w *Worker) IsRunning() bool {
	return w.running.Load()
}

// GetOps returns the operation count
func (w *Worker) GetOps() int64 {
	return w.OpsCount.Load()
}

// AddLatency records a latency measurement
func (w *Worker) AddLatency(d time.Duration) {
	w.latencyMu.Lock()
	defer w.latencyMu.Unlock()

	// Keep last 1000 latencies for percentile calculation
	if len(w.Latencies) >= 1000 {
		w.Latencies = w.Latencies[1:]
	}
	w.Latencies = append(w.Latencies, d)
}

// runCPUBench performs CPU-intensive matrix operations
func (w *Worker) runCPUBench(ctx context.Context) {
	size := 64
	a := make([][]float64, size)
	b := make([][]float64, size)
	c := make([][]float64, size)

	for i := 0; i < size; i++ {
		a[i] = make([]float64, size)
		b[i] = make([]float64, size)
		c[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			a[i][j] = float64(i + j)
			b[i][j] = float64(i * j)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()

			// Matrix multiplication
			for i := 0; i < size; i++ {
				for j := 0; j < size; j++ {
					sum := 0.0
					for k := 0; k < size; k++ {
						sum += a[i][k] * b[k][j]
					}
					c[i][j] = sum
				}
			}

			w.OpsCount.Add(1)
			w.AddLatency(time.Since(start))
		}
	}
}

// runMemoryBench performs memory bandwidth testing
func (w *Worker) runMemoryBench(ctx context.Context) {
	bufSize := 1024 * 1024 // 1MB
	src := make([]byte, bufSize)
	dst := make([]byte, bufSize)

	// Initialize source
	for i := range src {
		src[i] = byte(i % 256)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()

			// Memory copy
			copy(dst, src)

			// Touch all bytes to prevent optimization
			sum := 0
			for i := 0; i < len(dst); i += 64 {
				sum += int(dst[i])
			}
			_ = sum

			w.OpsCount.Add(1)
			w.BytesProc.Add(int64(bufSize))
			w.AddLatency(time.Since(start))
		}
	}
}

// runNetworkBench simulates network I/O (local loopback)
func (w *Worker) runNetworkBench(ctx context.Context) {
	// Simulate network latency with sleep
	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()

			// Simulate request/response
			time.Sleep(time.Microsecond * 100)

			w.OpsCount.Add(1)
			w.AddLatency(time.Since(start))
		}
	}
}

// runGPUBench simulates GPU compute (placeholder - would need CUDA/OpenCL)
func (w *Worker) runGPUBench(ctx context.Context) {
	// GPU benchmark is a placeholder - real implementation would use
	// CUDA, OpenCL, or Metal compute shaders

	// For now, do heavy FP math as a stand-in
	size := 256
	data := make([]float64, size*size)
	for i := range data {
		data[i] = float64(i)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()

			// Simulate GPU workload with heavy FP ops
			for i := 0; i < len(data); i++ {
				data[i] = data[i]*1.0001 + 0.0001
				if data[i] > 1e10 {
					data[i] = float64(i)
				}
			}

			w.OpsCount.Add(1)
			w.AddLatency(time.Since(start))
		}
	}
}

var workerCounter atomic.Int64

func generateWorkerID() string {
	return "worker-" + string(rune('A'+workerCounter.Add(1)%26))
}
