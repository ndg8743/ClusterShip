package sparse

import (
	"sync"
)

// Point represents a coordinate in the sparse board
type Point struct {
	X, Y int64
}

// Cell represents a cell in the sparse board
type Cell struct {
	X, Y    int64
	OwnerID string // company ID that owns this cell
	RackID  string // rack ID if this cell has a rack
	WasHit  bool   // whether this cell has been attacked
}

// Rect represents a rectangular boundary
type Rect struct {
	X, Y          int64 // top-left corner
	Width, Height int64
}

// Contains checks if a point is inside the rectangle
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width &&
		p.Y >= r.Y && p.Y < r.Y+r.Height
}

// Intersects checks if two rectangles overlap
func (r Rect) Intersects(other Rect) bool {
	return !(other.X >= r.X+r.Width ||
		other.X+other.Width <= r.X ||
		other.Y >= r.Y+r.Height ||
		other.Y+other.Height <= r.Y)
}

// QuadNode represents a node in the QuadTree
type QuadNode struct {
	Bounds   Rect
	Cells    []*Cell     // cells at this node (only for leaf nodes)
	Children [4]*QuadNode // NW, NE, SW, SE quadrants
	IsLeaf   bool
	mu       sync.RWMutex
}

// QuadTree represents a sparse spatial data structure
type QuadTree struct {
	Root      *QuadNode
	Width     int64
	Height    int64
	MaxDepth  int
	MaxCells  int // max cells per leaf before subdivision
	CellCount int64
	mu        sync.RWMutex
}

// NewQuadTree creates a new QuadTree for the given dimensions
func NewQuadTree(width, height int64) *QuadTree {
	return &QuadTree{
		Root: &QuadNode{
			Bounds: Rect{X: 0, Y: 0, Width: width, Height: height},
			Cells:  make([]*Cell, 0),
			IsLeaf: true,
		},
		Width:    width,
		Height:   height,
		MaxDepth: 20,  // allows for very large boards
		MaxCells: 16,  // cells per leaf before split
	}
}

// Insert adds a cell to the QuadTree
func (qt *QuadTree) Insert(x, y int64, ownerID, rackID string) bool {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if x < 0 || x >= qt.Width || y < 0 || y >= qt.Height {
		return false
	}

	cell := &Cell{
		X:       x,
		Y:       y,
		OwnerID: ownerID,
		RackID:  rackID,
		WasHit:  false,
	}

	if qt.insertIntoNode(qt.Root, cell, 0) {
		qt.CellCount++
		return true
	}
	return false
}

// insertIntoNode recursively inserts a cell into the appropriate node
func (qt *QuadTree) insertIntoNode(node *QuadNode, cell *Cell, depth int) bool {
	node.mu.Lock()
	defer node.mu.Unlock()

	point := Point{X: cell.X, Y: cell.Y}
	if !node.Bounds.Contains(point) {
		return false
	}

	if node.IsLeaf {
		// Check if cell already exists at this position
		for i, existing := range node.Cells {
			if existing.X == cell.X && existing.Y == cell.Y {
				// Update existing cell
				node.Cells[i] = cell
				return true
			}
		}

		// Add cell if under capacity or at max depth
		if len(node.Cells) < qt.MaxCells || depth >= qt.MaxDepth {
			node.Cells = append(node.Cells, cell)
			return true
		}

		// Subdivide and redistribute
		qt.subdivide(node)
	}

	// Insert into appropriate child
	for _, child := range node.Children {
		if child != nil && child.Bounds.Contains(point) {
			node.mu.Unlock()
			result := qt.insertIntoNode(child, cell, depth+1)
			node.mu.Lock()
			return result
		}
	}

	return false
}

// subdivide splits a leaf node into four children
func (qt *QuadTree) subdivide(node *QuadNode) {
	b := node.Bounds
	halfW := b.Width / 2
	halfH := b.Height / 2

	// Create four children: NW, NE, SW, SE
	node.Children[0] = &QuadNode{ // NW
		Bounds: Rect{X: b.X, Y: b.Y, Width: halfW, Height: halfH},
		Cells:  make([]*Cell, 0),
		IsLeaf: true,
	}
	node.Children[1] = &QuadNode{ // NE
		Bounds: Rect{X: b.X + halfW, Y: b.Y, Width: b.Width - halfW, Height: halfH},
		Cells:  make([]*Cell, 0),
		IsLeaf: true,
	}
	node.Children[2] = &QuadNode{ // SW
		Bounds: Rect{X: b.X, Y: b.Y + halfH, Width: halfW, Height: b.Height - halfH},
		Cells:  make([]*Cell, 0),
		IsLeaf: true,
	}
	node.Children[3] = &QuadNode{ // SE
		Bounds: Rect{X: b.X + halfW, Y: b.Y + halfH, Width: b.Width - halfW, Height: b.Height - halfH},
		Cells:  make([]*Cell, 0),
		IsLeaf: true,
	}

	node.IsLeaf = false

	// Redistribute existing cells to children
	for _, cell := range node.Cells {
		point := Point{X: cell.X, Y: cell.Y}
		for _, child := range node.Children {
			if child.Bounds.Contains(point) {
				child.Cells = append(child.Cells, cell)
				break
			}
		}
	}
	node.Cells = nil
}

// Query returns the cell at the given coordinates, or nil if empty
func (qt *QuadTree) Query(x, y int64) *Cell {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	return qt.queryNode(qt.Root, Point{X: x, Y: y})
}

// queryNode recursively searches for a cell at the given point
func (qt *QuadTree) queryNode(node *QuadNode, point Point) *Cell {
	if node == nil || !node.Bounds.Contains(point) {
		return nil
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	if node.IsLeaf {
		for _, cell := range node.Cells {
			if cell.X == point.X && cell.Y == point.Y {
				return cell
			}
		}
		return nil
	}

	for _, child := range node.Children {
		if child != nil && child.Bounds.Contains(point) {
			node.mu.RUnlock()
			result := qt.queryNode(child, point)
			node.mu.RLock()
			return result
		}
	}

	return nil
}

// RangeQuery returns all cells within the given rectangle
func (qt *QuadTree) RangeQuery(minX, minY, maxX, maxY int64) []*Cell {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	rect := Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
	cells := make([]*Cell, 0)
	qt.rangeQueryNode(qt.Root, rect, &cells)
	return cells
}

// rangeQueryNode recursively collects cells within the given rectangle
func (qt *QuadTree) rangeQueryNode(node *QuadNode, rect Rect, cells *[]*Cell) {
	if node == nil || !node.Bounds.Intersects(rect) {
		return
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	if node.IsLeaf {
		for _, cell := range node.Cells {
			point := Point{X: cell.X, Y: cell.Y}
			if rect.Contains(point) {
				*cells = append(*cells, cell)
			}
		}
		return
	}

	for _, child := range node.Children {
		if child != nil {
			node.mu.RUnlock()
			qt.rangeQueryNode(child, rect, cells)
			node.mu.RLock()
		}
	}
}

// SetHit marks a cell as hit
func (qt *QuadTree) SetHit(x, y int64) bool {
	cell := qt.Query(x, y)
	if cell == nil {
		return false
	}
	cell.WasHit = true
	return true
}

// Remove removes a cell from the QuadTree
func (qt *QuadTree) Remove(x, y int64) bool {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if qt.removeFromNode(qt.Root, Point{X: x, Y: y}) {
		qt.CellCount--
		return true
	}
	return false
}

// removeFromNode recursively removes a cell from the tree
func (qt *QuadTree) removeFromNode(node *QuadNode, point Point) bool {
	if node == nil || !node.Bounds.Contains(point) {
		return false
	}

	node.mu.Lock()
	defer node.mu.Unlock()

	if node.IsLeaf {
		for i, cell := range node.Cells {
			if cell.X == point.X && cell.Y == point.Y {
				// Remove cell by swapping with last and truncating
				node.Cells[i] = node.Cells[len(node.Cells)-1]
				node.Cells = node.Cells[:len(node.Cells)-1]
				return true
			}
		}
		return false
	}

	for _, child := range node.Children {
		if child != nil && child.Bounds.Contains(point) {
			node.mu.Unlock()
			result := qt.removeFromNode(child, point)
			node.mu.Lock()
			return result
		}
	}

	return false
}

// GetCellCount returns the total number of cells in the tree
func (qt *QuadTree) GetCellCount() int64 {
	qt.mu.RLock()
	defer qt.mu.RUnlock()
	return qt.CellCount
}

// Clear removes all cells from the tree
func (qt *QuadTree) Clear() {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	qt.Root = &QuadNode{
		Bounds: Rect{X: 0, Y: 0, Width: qt.Width, Height: qt.Height},
		Cells:  make([]*Cell, 0),
		IsLeaf: true,
	}
	qt.CellCount = 0
}
