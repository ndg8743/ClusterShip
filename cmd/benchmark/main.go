package main

import (
	"clustership/pkg/game"
	"clustership/pkg/hardware"
	"clustership/pkg/sparse"
	"clustership/pkg/tui"
	"flag"
	"fmt"
	"runtime"
	"strings"
	"time"
)

type TierBenchmark struct {
	Name         string
	BoardWidth   int64
	BoardHeight  int64
	Companies    int
	ShipsPerCo   int
	RacksPerShip int
	UseSparse    bool
}

type BenchmarkResult struct {
	Tier           string
	BoardSize      string
	CreationTimeMs float64
	MemoryMB       float64
	AttacksPerSec  float64
	ViewportMs     float64
	CellCount      int64
	Success        bool
	Error          string
}

var tiers = []TierBenchmark{
	{
		Name:         "Basic",
		BoardWidth:   100,
		BoardHeight:  100,
		Companies:    6,
		ShipsPerCo:   10,
		RacksPerShip: 5,
		UseSparse:    false,
	},
	{
		Name:         "Standard",
		BoardWidth:   1000,
		BoardHeight:  1000,
		Companies:    20,
		ShipsPerCo:   10,
		RacksPerShip: 10,
		UseSparse:    true,
	},
	{
		Name:         "Advanced",
		BoardWidth:   10000,
		BoardHeight:  10000,
		Companies:    50,
		ShipsPerCo:   20,
		RacksPerShip: 20,
		UseSparse:    true,
	},
	{
		Name:         "Extreme",
		BoardWidth:   100000,
		BoardHeight:  100000,
		Companies:    100,
		ShipsPerCo:   50,
		RacksPerShip: 20,
		UseSparse:    true,
	},
}

func main() {
	allTiers := flag.Bool("all-tiers", false, "Run benchmarks for all tiers")
	tierName := flag.String("tier", "", "Run benchmark for specific tier (Basic, Standard, Advanced, Extreme)")
	jsonOutput := flag.Bool("json", false, "Output results as JSON")
	flag.Parse()

	fmt.Println("ClusterShip Scaling Benchmark")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("CPUs: %d\n", runtime.NumCPU())
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))

	sys := hardware.Detect()
	fmt.Printf("RAM: %d MB\n", sys.TotalRAMMB)
	if sys.GPU.Available {
		fmt.Printf("GPU: %s (%s, %d MB)\n", sys.GPU.Model, sys.GPU.Vendor, sys.GPU.MemoryMB)
	} else {
		fmt.Println("GPU: None detected")
	}
	detectedTier := hardware.DetermineTier(sys)
	fmt.Printf("Detected Tier: %s\n", detectedTier)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	var results []BenchmarkResult

	if *allTiers {
		for _, tier := range tiers {
			result := runTierBenchmark(tier)
			results = append(results, result)
			printResult(result)
		}
	} else if *tierName != "" {
		for _, tier := range tiers {
			if strings.EqualFold(tier.Name, *tierName) {
				result := runTierBenchmark(tier)
				results = append(results, result)
				printResult(result)
				break
			}
		}
	} else {
		tier := tiers[0]
		for _, t := range tiers {
			if strings.EqualFold(t.Name, detectedTier.String()) {
				tier = t
				break
			}
		}
		result := runTierBenchmark(tier)
		results = append(results, result)
		printResult(result)
	}

	if *jsonOutput {
		printJSON(results)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Benchmark Summary")
	fmt.Println(strings.Repeat("=", 60))
	printSummaryTable(results)
}

func runTierBenchmark(tier TierBenchmark) BenchmarkResult {
	result := BenchmarkResult{
		Tier:      tier.Name,
		BoardSize: fmt.Sprintf("%dx%d", tier.BoardWidth, tier.BoardHeight),
	}

	fmt.Printf("\n[%s Tier] Running benchmark...\n", tier.Name)
	fmt.Printf("Board: %dx%d, Companies: %d, Ships/Co: %d, Racks/Ship: %d\n",
		tier.BoardWidth, tier.BoardHeight, tier.Companies, tier.ShipsPerCo, tier.RacksPerShip)

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	memBefore := m1.Alloc

	if tier.UseSparse {
		result = runSparseBenchmark(tier, result, memBefore)
	} else {
		result = runDenseBenchmark(tier, result, memBefore)
	}

	return result
}

func runSparseBenchmark(tier TierBenchmark, result BenchmarkResult, memBefore uint64) BenchmarkResult {
	defer func() {
		if r := recover(); r != nil {
			result.Success = false
			result.Error = fmt.Sprintf("panic: %v", r)
		}
	}()

	start := time.Now()
	board := sparse.NewSparseBoard(tier.BoardWidth, tier.BoardHeight)

	cellsPlaced := int64(0)
	for c := 0; c < tier.Companies; c++ {
		companyID := fmt.Sprintf("company-%d", c)
		for s := 0; s < tier.ShipsPerCo; s++ {
			for r := 0; r < tier.RacksPerShip; r++ {
				x := int64((c*tier.ShipsPerCo*tier.RacksPerShip + s*tier.RacksPerShip + r) * 3 % int(tier.BoardWidth))
				y := int64((c*tier.ShipsPerCo*tier.RacksPerShip + s*tier.RacksPerShip + r) * 7 % int(tier.BoardHeight))
				rackID := fmt.Sprintf("rack-%d-%d-%d", c, s, r)
				board.PlaceCell(x, y, companyID, rackID)
				cellsPlaced++
			}
		}
	}

	creationTime := time.Since(start)
	result.CreationTimeMs = float64(creationTime.Microseconds()) / 1000.0

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	if m2.Alloc > memBefore {
		result.MemoryMB = float64(m2.Alloc-memBefore) / 1024 / 1024
	}

	attackStart := time.Now()
	attackCount := 10000
	for i := 0; i < attackCount; i++ {
		x := int64(i * 17 % int(tier.BoardWidth))
		y := int64(i * 31 % int(tier.BoardHeight))
		_ = board.GetCell(x, y)
	}
	attackDuration := time.Since(attackStart)
	if attackDuration.Nanoseconds() > 0 {
		result.AttacksPerSec = float64(attackCount) / attackDuration.Seconds()
	} else {
		result.AttacksPerSec = float64(attackCount) * 1e9
	}

	viewportStart := time.Now()
	viewportCount := 100
	for i := 0; i < viewportCount; i++ {
		x := int64(i * 13 % int(tier.BoardWidth-50))
		y := int64(i * 23 % int(tier.BoardHeight-50))
		_ = board.GetViewport(x, y, x+50, y+50)
	}
	viewportDuration := time.Since(viewportStart)
	result.ViewportMs = float64(viewportDuration.Microseconds()) / float64(viewportCount) / 1000.0

	stats := board.GetStats()
	result.CellCount = stats.CellCount
	result.Success = true

	return result
}

func runDenseBenchmark(tier TierBenchmark, result BenchmarkResult, memBefore uint64) BenchmarkResult {
	defer func() {
		if r := recover(); r != nil {
			result.Success = false
			result.Error = fmt.Sprintf("panic: %v", r)
		}
	}()

	companies := make([]*game.Company, tier.Companies)
	for i := 0; i < tier.Companies; i++ {
		companies[i] = &game.Company{
			ID:   fmt.Sprintf("company-%d", i),
			Name: fmt.Sprintf("Company %d", i),
			Regions: []*game.Region{
				{
					ID:        fmt.Sprintf("region-%d", i),
					Name:      "Main",
					RackCount: tier.RacksPerShip,
					Racks:     make([]*game.Rack, tier.RacksPerShip),
				},
			},
			Services: []*game.Service{
				{
					ID:       fmt.Sprintf("service-%d", i),
					Name:     "TestService",
					Replicas: 3,
					Affinity: game.AffinityNone,
					Pods:     make([]*game.Pod, 0),
				},
			},
		}
		for j := 0; j < tier.RacksPerShip; j++ {
			companies[i].Regions[0].Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("rack-%d-%d", i, j),
				RegionID: companies[i].Regions[0].ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}
	}

	start := time.Now()
	board := tui.NewMultiBoard(int(tier.BoardWidth), int(tier.BoardHeight), companies)
	creationTime := time.Since(start)
	result.CreationTimeMs = float64(creationTime.Microseconds()) / 1000.0

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	if m2.Alloc > memBefore {
		result.MemoryMB = float64(m2.Alloc-memBefore) / 1024 / 1024
	}

	attackStart := time.Now()
	attackCount := 10000
	for i := 0; i < attackCount; i++ {
		x := i % int(tier.BoardWidth)
		y := (i / int(tier.BoardWidth)) % int(tier.BoardHeight)
		companyID := fmt.Sprintf("company-%d", i%tier.Companies)
		_, _ = board.AttackMulti(x, y, companyID, "")
	}
	attackDuration := time.Since(attackStart)
	if attackDuration.Nanoseconds() > 0 {
		result.AttacksPerSec = float64(attackCount) / attackDuration.Seconds()
	} else {
		result.AttacksPerSec = float64(attackCount) * 1e9
	}

	result.ViewportMs = 0
	result.CellCount = tier.BoardWidth * tier.BoardHeight
	result.Success = true

	return result
}

func printResult(result BenchmarkResult) {
	status := "PASS"
	if !result.Success {
		status = "FAIL"
	}

	fmt.Printf("\n--- %s Tier Results [%s] ---\n", result.Tier, status)
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
		return
	}

	fmt.Printf("Board Creation: %.2f ms\n", result.CreationTimeMs)
	fmt.Printf("Memory Used: %.2f MB\n", result.MemoryMB)
	fmt.Printf("Attack Lookups: %.0f ops/sec\n", result.AttacksPerSec)
	if result.ViewportMs > 0 {
		fmt.Printf("Viewport Query: %.3f ms (avg)\n", result.ViewportMs)
	}
	fmt.Printf("Total Cells: %d\n", result.CellCount)
}

func printSummaryTable(results []BenchmarkResult) {
	fmt.Println()
	fmt.Printf("%-10s | %-12s | %-12s | %-10s | %-14s | %-8s\n",
		"Tier", "Board Size", "Creation", "Memory", "Attacks/sec", "Status")
	fmt.Println(strings.Repeat("-", 76))

	for _, r := range results {
		status := "PASS"
		if !r.Success {
			status = "FAIL"
		}
		fmt.Printf("%-10s | %-12s | %10.2f ms | %8.2f MB | %12.0f | %-8s\n",
			r.Tier, r.BoardSize, r.CreationTimeMs, r.MemoryMB, r.AttacksPerSec, status)
	}
}

func printJSON(results []BenchmarkResult) {
	fmt.Println("\n--- JSON Output ---")
	fmt.Println("[")
	for i, r := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		fmt.Printf(`  {"tier": "%s", "board_size": "%s", "creation_ms": %.2f, "memory_mb": %.2f, "attacks_per_sec": %.0f, "viewport_ms": %.3f, "cells": %d, "success": %v}%s`+"\n",
			r.Tier, r.BoardSize, r.CreationTimeMs, r.MemoryMB, r.AttacksPerSec, r.ViewportMs, r.CellCount, r.Success, comma)
	}
	fmt.Println("]")
}
