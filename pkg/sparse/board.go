package sparse

import (
	"fmt"
	"sync"
)

// SparseBoard provides a sparse representation of very large game boards
// It uses a QuadTree for efficient spatial queries on boards up to 10M x 10M
type SparseBoard struct {
	tree       *QuadTree
	cellOwners map[string]string // "x,y" -> ownerID for quick lookup
	hits       map[string]bool   // "x,y" -> true if hit
	mu         sync.RWMutex
	Width      int64
	Height     int64
}

// NewSparseBoard creates a new sparse board
func NewSparseBoard(width, height int64) *SparseBoard {
	return &SparseBoard{
		tree:       NewQuadTree(width, height),
		cellOwners: make(map[string]string),
		hits:       make(map[string]bool),
		Width:      width,
		Height:     height,
	}
}

// key generates a string key for coordinates
func key(x, y int64) string {
	// Use fmt.Sprintf with separator to prevent collisions
	// e.g., (10, 100) -> "10,100" vs (101, 00) -> "101,0"
	return fmt.Sprintf("%d,%d", x, y)
}

// PlaceCell places a cell owned by the given company at the coordinates
func (sb *SparseBoard) PlaceCell(x, y int64, ownerID, rackID string) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.tree.Insert(x, y, ownerID, rackID) {
		sb.cellOwners[key(x, y)] = ownerID
		return true
	}
	return false
}

// GetCell returns the cell at the given coordinates
func (sb *SparseBoard) GetCell(x, y int64) *Cell {
	return sb.tree.Query(x, y)
}

// GetOwner returns the owner ID of the cell at the given coordinates
func (sb *SparseBoard) GetOwner(x, y int64) string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.cellOwners[key(x, y)]
}

// IsOccupied checks if a cell has an owner
func (sb *SparseBoard) IsOccupied(x, y int64) bool {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	_, exists := sb.cellOwners[key(x, y)]
	return exists
}

// Hit marks a cell as hit and returns true if it was occupied
func (sb *SparseBoard) Hit(x, y int64) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	k := key(x, y)
	sb.hits[k] = true

	cell := sb.tree.Query(x, y)
	if cell != nil {
		cell.WasHit = true
		return true
	}
	return false
}

// WasHit checks if a cell was already hit
func (sb *SparseBoard) WasHit(x, y int64) bool {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.hits[key(x, y)]
}

// GetViewport returns all cells visible in the given viewport
func (sb *SparseBoard) GetViewport(minX, minY, maxX, maxY int64) []*Cell {
	return sb.tree.RangeQuery(minX, minY, maxX, maxY)
}

// RemoveCell removes a cell from the board
func (sb *SparseBoard) RemoveCell(x, y int64) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	k := key(x, y)
	delete(sb.cellOwners, k)
	return sb.tree.Remove(x, y)
}

// GetCellCount returns the total number of occupied cells
func (sb *SparseBoard) GetCellCount() int64 {
	return sb.tree.GetCellCount()
}

// Clear removes all cells from the board
func (sb *SparseBoard) Clear() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.tree.Clear()
	sb.cellOwners = make(map[string]string)
	sb.hits = make(map[string]bool)
}

// GetStats returns statistics about the sparse board
func (sb *SparseBoard) GetStats() BoardStats {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	return BoardStats{
		Width:     sb.Width,
		Height:    sb.Height,
		CellCount: sb.tree.GetCellCount(),
		HitCount:  int64(len(sb.hits)),
		Density:   float64(sb.tree.GetCellCount()) / float64(sb.Width*sb.Height),
	}
}

// BoardStats contains statistics about the sparse board
type BoardStats struct {
	Width     int64
	Height    int64
	CellCount int64
	HitCount  int64
	Density   float64 // ratio of occupied cells to total cells
}

// BatchPlaceCells efficiently places multiple cells at once
func (sb *SparseBoard) BatchPlaceCells(cells []CellPlacement) int {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	count := 0
	for _, c := range cells {
		if sb.tree.Insert(c.X, c.Y, c.OwnerID, c.RackID) {
			sb.cellOwners[key(c.X, c.Y)] = c.OwnerID
			count++
		}
	}
	return count
}

// CellPlacement represents a cell to be placed
type CellPlacement struct {
	X, Y    int64
	OwnerID string
	RackID  string
}

// IterateViewport calls the callback for each cell in the viewport
// More memory-efficient than GetViewport for large viewports
func (sb *SparseBoard) IterateViewport(minX, minY, maxX, maxY int64, callback func(*Cell) bool) {
	cells := sb.tree.RangeQuery(minX, minY, maxX, maxY)
	for _, cell := range cells {
		if !callback(cell) {
			break
		}
	}
}

// CountInRegion counts cells in a rectangular region
func (sb *SparseBoard) CountInRegion(minX, minY, maxX, maxY int64) int {
	cells := sb.tree.RangeQuery(minX, minY, maxX, maxY)
	return len(cells)
}
