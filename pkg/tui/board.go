package tui

import (
	"clustership/pkg/game"
	"fmt"
	"math/rand"
	"sync"
)

// ShotResult tracks what happened when a cell was attacked
type ShotResult struct {
	Hit          bool
	Coord        [2]int
	HitRack      *game.Rack
	HitPod       *game.Pod
	KilledPod    bool
	KilledRack   bool
	KilledRegion bool   // true if this attack destroyed the entire region
	RegionName   string // name of the destroyed region (for "sunk battleship" message)
	Message      string
}

// Board represents the game board with all fleets placed
type Board struct {
	Width  int
	Height int

	// Multi-fleet support
	Fleets    map[string]*Fleet                  // companyID -> Fleet
	Shots     map[string]map[string]*ShotResult  // attackerID -> coordKey -> result
	CellOwner map[string]string                  // coordKey -> companyID (who owns this cell)

	// Legacy single-enemy fields (for backward compatibility)
	PlayerFleet *Fleet
	EnemyFleet  *Fleet
	PlayerShots map[string]*ShotResult
	EnemyShots  map[string]*ShotResult

	// events log
	Events []game.GameEvent

	// Mutex to protect concurrent map access
	mu sync.RWMutex
}

// Fleet represents a placed company on the board
type Fleet struct {
	Company *game.Company
	Regions []*PlacedRegion
}

// PlacedRegion is a region with its racks placed on the board
type PlacedRegion struct {
	Region *game.Region
	Racks  []*PlacedRack
}

// PlacedRack is a rack at a specific board position
type PlacedRack struct {
	Rack     *game.Rack
	Position [2]int
}

// NewBoard creates a game board and places both fleets on the shared ocean (legacy 1v1 mode)
func NewBoard(width, height int, player, enemy *game.Company) *Board {
	if player.ID == "" {
		player.ID = "player"
	}
	if enemy.ID == "" {
		enemy.ID = "enemy"
	}
	return NewMultiBoard(width, height, []*game.Company{player, enemy})
}

// NewMultiBoardWithManualPlacement creates a board where the first company (player) has already been placed
func NewMultiBoardWithManualPlacement(width, height int, companies []*game.Company, playerOccupied map[string]bool) *Board {
	b := &Board{
		Width:       width,
		Height:      height,
		Fleets:      make(map[string]*Fleet),
		Shots:       make(map[string]map[string]*ShotResult),
		CellOwner:   make(map[string]string),
		PlayerShots: make(map[string]*ShotResult),
		EnemyShots:  make(map[string]*ShotResult),
		Events:      make([]game.GameEvent, 0),
	}

	// Initialize shots map for each company
	for _, company := range companies {
		b.Shots[company.ID] = make(map[string]*ShotResult)
	}

	// Start with player's occupied cells
	occupied := make(map[string]bool)
	for k, v := range playerOccupied {
		occupied[k] = v
	}

	// Place all fleets
	for i, company := range companies {
		var fleet *Fleet
		if i == 0 {
			// Player fleet - already placed manually
			fleet = b.buildFleetFromManualPlacement(company)
		} else {
			// Enemy fleets - auto-place
			fleet = b.placeFleet(company, occupied)
		}
		b.Fleets[company.ID] = fleet

		// Track cell ownership for rendering
		for _, region := range fleet.Regions {
			for _, rack := range region.Racks {
				key := fmt.Sprintf("%d,%d", rack.Position[0], rack.Position[1])
				b.CellOwner[key] = company.ID
			}
		}

		b.initPods(fleet)

		if i == 0 {
			b.PlayerFleet = fleet
		} else if i == 1 {
			b.EnemyFleet = fleet
		}
	}

	return b
}

// buildFleetFromManualPlacement creates a fleet from manually placed regions
func (b *Board) buildFleetFromManualPlacement(company *game.Company) *Fleet {
	fleet := &Fleet{
		Company: company,
		Regions: make([]*PlacedRegion, len(company.Regions)),
	}

	for i, region := range company.Regions {
		placed := &PlacedRegion{
			Region: region,
			Racks:  make([]*PlacedRack, len(region.Racks)),
		}

		for j, rack := range region.Racks {
			placed.Racks[j] = &PlacedRack{
				Rack:     rack,
				Position: rack.Position,
			}
		}

		fleet.Regions[i] = placed
	}

	return fleet
}

// NewMultiBoard creates a game board with multiple companies
func NewMultiBoard(width, height int, companies []*game.Company) *Board {
	b := &Board{
		Width:       width,
		Height:      height,
		Fleets:      make(map[string]*Fleet),
		Shots:       make(map[string]map[string]*ShotResult),
		CellOwner:   make(map[string]string),
		PlayerShots: make(map[string]*ShotResult),
		EnemyShots:  make(map[string]*ShotResult),
		Events:      make([]game.GameEvent, 0),
	}

	// Initialize shots map for each company
	for _, company := range companies {
		b.Shots[company.ID] = make(map[string]*ShotResult)
	}

	// shared occupied map - all ships on same ocean, can't overlap
	occupied := make(map[string]bool)

	// place all fleets
	for i, company := range companies {
		fleet := b.placeFleet(company, occupied)
		b.Fleets[company.ID] = fleet

		// Track cell ownership for rendering
		for _, region := range fleet.Regions {
			for _, rack := range region.Racks {
				key := fmt.Sprintf("%d,%d", rack.Position[0], rack.Position[1])
				b.CellOwner[key] = company.ID
			}
		}

		b.initPods(fleet)

		if i == 0 {
			b.PlayerFleet = fleet
		} else if i == 1 {
			b.EnemyFleet = fleet
		}
	}

	return b
}

// placeFleet places a company's regions on the shared ocean
func (b *Board) placeFleet(company *game.Company, occupied map[string]bool) *Fleet {
	fleet := &Fleet{
		Company: company,
		Regions: make([]*PlacedRegion, len(company.Regions)),
	}

	for i, region := range company.Regions {
		placed := &PlacedRegion{
			Region: region,
			Racks:  make([]*PlacedRack, len(region.Racks)),
		}

		cells := b.findPlacement(region.RackCount, 0, b.Width, 0, b.Height, occupied)

		for j, cell := range cells {
			key := fmt.Sprintf("%d,%d", cell[0], cell[1])
			occupied[key] = true

			region.Racks[j].Position = cell
			placed.Racks[j] = &PlacedRack{
				Rack:     region.Racks[j],
				Position: cell,
			}
		}

		region.Placement = cells
		fleet.Regions[i] = placed
	}

	return fleet
}

// findPlacement finds a valid placement for a ship of given length
func (b *Board) findPlacement(length, minX, maxX, minY, maxY int, occupied map[string]bool) [][2]int {
	for attempts := 0; attempts < 1000; attempts++ {
		// random orientation
		vertical := rand.Intn(2) == 0

		var x0, y0 int
		if vertical {
			x0 = minX + rand.Intn(maxX-minX)
			y0 = minY + rand.Intn(maxY-minY-(length-1))
		} else {
			x0 = minX + rand.Intn(maxX-minX-(length-1))
			y0 = minY + rand.Intn(maxY-minY)
		}

		cells := make([][2]int, length)
		valid := true

		for i := 0; i < length; i++ {
			x, y := x0, y0
			if vertical {
				y += i
			} else {
				x += i
			}

			if x < minX || x >= maxX || y < minY || y >= maxY {
				valid = false
				break
			}

			key := fmt.Sprintf("%d,%d", x, y)
			if occupied[key] {
				valid = false
				break
			}

			cells[i] = [2]int{x, y}
		}

		if valid {
			return cells
		}
	}

	return make([][2]int, length)
}

// initPods creates pods for all services and assigns them to racks
func (b *Board) initPods(fleet *Fleet) {
	if fleet == nil || fleet.Company == nil {
		return
	}

	podID := 0
	for _, svc := range fleet.Company.Services {
		// If pods already exist (e.g., from tests), skip initialization for this service
		if len(svc.Pods) > 0 {
			continue
		}

		svc.Pods = make([]*game.Pod, 0)

		for i := 0; i < svc.Replicas; i++ {
			rack := b.findRackForPod(fleet, svc)

			pod := &game.Pod{
				ID:        fmt.Sprintf("%s-%d", svc.ID, podID),
				ServiceID: svc.ID,
				Health:    1,
				MaxHealth: 1,
				Status:    game.PodRunning,
			}

			if rack != nil {
				pod.RackID = rack.ID
				pod.RegionID = rack.RegionID
				pod.Position = rack.Position
				rack.Pods = append(rack.Pods, pod)
			}

			svc.Pods = append(svc.Pods, pod)
			podID++
		}
	}
}

// findRackForPod finds a suitable rack based on affinity
// excludeRackID optionally excludes a rack (for rescheduling scenarios)
func (b *Board) findRackForPod(fleet *Fleet, svc *game.Service) *game.Rack {
	return b.findRackForPodExcluding(fleet, svc, "")
}

// findRackForPodExcluding finds a suitable rack, optionally excluding a specific rack
func (b *Board) findRackForPodExcluding(fleet *Fleet, svc *game.Service, excludeRackID string) *game.Rack {
	var racks []*game.Rack
	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if !rack.IsDestroyed && len(rack.Pods) < rack.Capacity && rack.ID != excludeRackID {
				racks = append(racks, rack)
			}
		}
	}

	if len(racks) == 0 {
		return nil
	}

	switch svc.Affinity {
	case game.AffinitySpread:
		minPods := len(racks[0].Pods)
		best := racks[0]
		for _, r := range racks {
			if len(r.Pods) < minPods {
				minPods = len(r.Pods)
				best = r
			}
		}
		return best

	case game.AffinityHard:
		if len(svc.Pods) > 0 {
			for _, r := range racks {
				if r.RegionID == svc.Pods[0].RegionID {
					return r
				}
			}
		}
		return racks[0]

	default:
		return racks[rand.Intn(len(racks))]
	}
}

// Attack executes an attack at the given coordinates (legacy 1v1 mode)
// Returns hit result and any events generated
func (b *Board) Attack(x, y int, byPlayer bool) (*ShotResult, []game.GameEvent) {
	if byPlayer {
		return b.AttackMulti(x, y, "player", "")
	}
	for id := range b.Fleets {
		if id != "player" {
			return b.AttackMulti(x, y, id, "player")
		}
	}
	return b.AttackMulti(x, y, "enemy", "player")
}

// AttackMulti executes an attack from one company
func (b *Board) AttackMulti(x, y int, attackerID, targetID string) (*ShotResult, []game.GameEvent) {
	key := fmt.Sprintf("%d,%d", x, y)

	// Check if shot already exists (read lock)
	b.mu.RLock()
	if b.Shots[attackerID] == nil {
		b.mu.RUnlock()
		b.mu.Lock()
		b.Shots[attackerID] = make(map[string]*ShotResult)
		b.mu.Unlock()
		b.mu.RLock()
	}

	if _, exists := b.Shots[attackerID][key]; exists {
		b.mu.RUnlock()
		return nil, nil
	}
	b.mu.RUnlock()

	result := &ShotResult{
		Coord: [2]int{x, y},
	}
	events := make([]game.GameEvent, 0)

	ownerID := b.CellOwner[key]

	if ownerID == "" || ownerID == attackerID {
		result.Hit = false
		result.Message = "Miss"
		b.mu.Lock()
		b.Shots[attackerID][key] = result
		if attackerID == "player" {
			b.PlayerShots[key] = result
		} else {
			b.EnemyShots[key] = result
		}
		b.mu.Unlock()
		return result, events
	}

	if targetID != "" && ownerID != targetID {
		result.Hit = false
		result.Message = "Miss"
		b.mu.Lock()
		b.Shots[attackerID][key] = result
		if attackerID == "player" {
			b.PlayerShots[key] = result
		} else {
			b.EnemyShots[key] = result
		}
		b.mu.Unlock()
		return result, events
	}

	targetFleet := b.Fleets[ownerID]
	if targetFleet == nil {
		result.Hit = false
		result.Message = "Miss"
		b.mu.Lock()
		b.Shots[attackerID][key] = result
		b.mu.Unlock()
		return result, events
	}

	rack := b.findRackAt(targetFleet, x, y)
	if rack != nil && !rack.IsDestroyed {
		result.Hit = true
		result.HitRack = rack
		rack.HitCount++

		for _, pod := range rack.Pods {
			if pod.Status == game.PodRunning {
				pod.Health--
				if pod.Health <= 0 {
					result.KilledPod = true
					result.HitPod = pod

					rescheduled := b.tryReschedulePod(targetFleet, pod)

					if rescheduled {
						pod.Status = game.PodRunning
						events = append(events, game.GameEvent{
							Type:      "Normal",
							Reason:    "PodRescheduled",
							Message:   fmt.Sprintf("Pod %s rescheduled", pod.ID),
							ServiceID: pod.ServiceID,
							PodID:     pod.ID,
						})
					} else {
						pod.Status = game.PodTerminated
						events = append(events, game.GameEvent{
							Type:      "Warning",
							Reason:    "PodTerminated",
							Message:   fmt.Sprintf("Pod %s terminated - no reschedule available", pod.ID),
							ServiceID: pod.ServiceID,
							PodID:     pod.ID,
						})
					}
				}
			}
		}

		allDead := true
		for _, pod := range rack.Pods {
			if pod.Status == game.PodRunning {
				allDead = false
				break
			}
		}
		// Rack is destroyed if: all pods are dead OR rack has no pods (destroyed on first hit)
		shouldDestroy := (allDead && len(rack.Pods) > 0) || len(rack.Pods) == 0
		if shouldDestroy {
			rack.IsDestroyed = true
			result.KilledRack = true
			events = append(events, game.GameEvent{
				Type:     "Warning",
				Reason:   "RackDestroyed",
				Message:  fmt.Sprintf("Rack %s destroyed", rack.ID),
				RegionID: rack.RegionID,
			})

			// Check if entire region is destroyed (all racks in region destroyed)
			for _, region := range targetFleet.Company.Regions {
				if region.ID == rack.RegionID {
					allRacksDestroyed := true
					for _, r := range region.Racks {
						if !r.IsDestroyed {
							allRacksDestroyed = false
							break
						}
					}
					if allRacksDestroyed && len(region.Racks) > 0 {
						region.IsDestroyed = true
						result.KilledRegion = true
						result.RegionName = region.Name
						events = append(events, game.GameEvent{
							Type:     "Warning",
							Reason:   "RegionDestroyed",
							Message:  fmt.Sprintf("You've sunk my battleship! %s's %s region destroyed!", targetFleet.Company.Name, region.Name),
							RegionID: region.ID,
						})
					}
					break
				}
			}
		}

		result.Message = fmt.Sprintf("Hit! %s's Rack %s", targetFleet.Company.Name, rack.ID)
	} else {
		result.Hit = false
		result.Message = "Miss"
	}

	b.mu.Lock()
	b.Shots[attackerID][key] = result
	if attackerID == "player" {
		b.PlayerShots[key] = result
	} else {
		b.EnemyShots[key] = result
	}
	b.Events = append(b.Events, events...)
	b.mu.Unlock()

	return result, events
}

// findRackAt returns the rack at the given position, if any
func (b *Board) findRackAt(fleet *Fleet, x, y int) *game.Rack {
	if fleet == nil {
		return nil
	}

	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if rack.Position[0] == x && rack.Position[1] == y {
				return rack
			}
		}
	}
	return nil
}

// tryReschedulePod attempts to move a pod to another rack based on affinity
func (b *Board) tryReschedulePod(fleet *Fleet, pod *game.Pod) bool {
	var svc *game.Service
	for _, s := range fleet.Company.Services {
		if s.ID == pod.ServiceID {
			svc = s
			break
		}
	}

	if svc == nil || !svc.CanFailover {
		return false
	}

	if svc.Affinity == game.AffinityHard {
		return false
	}

	oldRackID := pod.RackID

	// Find a new rack, excluding the current rack
	newRack := b.findRackForPodExcluding(fleet, svc, oldRackID)
	if newRack == nil {
		return false
	}

	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if rack.ID == oldRackID {
				newPods := make([]*game.Pod, 0, len(rack.Pods))
				for _, p := range rack.Pods {
					if p.ID != pod.ID {
						newPods = append(newPods, p)
					}
				}
				rack.Pods = newPods
				break
			}
		}
	}

	pod.RackID = newRack.ID
	pod.RegionID = newRack.RegionID
	pod.Position = newRack.Position
	pod.Health = pod.MaxHealth
	newRack.Pods = append(newRack.Pods, pod)

	return true
}

// GetCellState returns what's visible at a cell for rendering
type CellState int

const (
	CellWater CellState = iota
	CellShip
	CellHit
	CellMiss
	CellDestroyed
)

// GetPlayerCellState returns the state of a cell from the player's perspective
// (seeing their own fleet and where enemy has shot)
func (b *Board) GetPlayerCellState(x, y int) CellState {
	key := fmt.Sprintf("%d,%d", x, y)

	b.mu.RLock()
	result, ok := b.EnemyShots[key]
	b.mu.RUnlock()

	if ok {
		if result.Hit {
			if result.KilledRack {
				return CellDestroyed
			}
			return CellHit
		}
		return CellMiss
	}

	if b.findRackAt(b.PlayerFleet, x, y) != nil {
		return CellShip
	}

	return CellWater
}

// GetEnemyCellState returns the state of a cell from attacking enemy
// (only shows what player has discovered)
func (b *Board) GetEnemyCellState(x, y int) CellState {
	key := fmt.Sprintf("%d,%d", x, y)

	b.mu.RLock()
	result, ok := b.PlayerShots[key]
	b.mu.RUnlock()

	if ok {
		if result.Hit {
			if result.KilledRack {
				return CellDestroyed
			}
			return CellHit
		}
		return CellMiss
	}

	return CellWater
}

// FleetHealthy returns true if the fleet has any running pods
func (b *Board) FleetHealthy(fleet *Fleet) bool {
	if fleet == nil || fleet.Company == nil {
		return false
	}
	return fleet.Company.HealthyPodCount() > 0
}

// HasEnemyShipAt returns true if there's an enemy ship at the given position
func (b *Board) HasEnemyShipAt(x, y int) bool {
	return b.findRackAt(b.EnemyFleet, x, y) != nil
}

// FleetHealthyByID returns if a specific fleet has running pods
func (b *Board) FleetHealthyByID(companyID string) bool {
	fleet, ok := b.Fleets[companyID]
	if !ok || fleet == nil || fleet.Company == nil {
		return false
	}
	return fleet.Company.HealthyPodCount() > 0
}

// GetActiveCompanies returns IDs of companies with healthy fleets
func (b *Board) GetActiveCompanies() []string {
	active := make([]string, 0)
	for id := range b.Fleets {
		if b.FleetHealthyByID(id) {
			active = append(active, id)
		}
	}
	return active
}

// GetCellOwner returns the company ID that owns the cell at (x, y)
func (b *Board) GetCellOwner(x, y int) string {
	key := fmt.Sprintf("%d,%d", x, y)
	return b.CellOwner[key]
}

// HasShipAt returns true if any company has a ship at (x, y)
func (b *Board) HasShipAt(x, y int) bool {
	key := fmt.Sprintf("%d,%d", x, y)
	return b.CellOwner[key] != ""
}

// IsRegionDestroyedAt returns true if the cell at (x, y) is in a completely destroyed region
func (b *Board) IsRegionDestroyedAt(x, y int) bool {
	key := fmt.Sprintf("%d,%d", x, y)
	ownerID := b.CellOwner[key]
	if ownerID == "" {
		return false
	}

	fleet := b.Fleets[ownerID]
	if fleet == nil || fleet.Company == nil {
		return false
	}

	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if rack.Position[0] == x && rack.Position[1] == y {
				return region.IsDestroyed
			}
		}
	}
	return false
}

// ServiceOnRack contains info about a service's pods on a specific rack
type ServiceOnRack struct {
	ServiceID   string
	ServiceName string
	Affinity    game.AffinityType
	PodCount    int
	RunningPods int
	CanFailover bool
}

// CellInfo contains details about what's at a cell (for hover display)
type CellInfo struct {
	Empty            bool
	OwnerID          string
	OwnerName        string
	RegionID         string
	RegionName       string
	RackID           string
	IsDestroyed      bool
	IsRegionDestroyed bool // true if the entire region is destroyed
	PodCount         int
	RunningPods      int
	ServiceIDs       []string
	ServicesOnRack   []ServiceOnRack // detailed service info
	WasHit           bool
	CanAttack        bool // true if not your own and not already hit by you
	HasCritical      bool // true if rack has hard-affinity (critical) services
}

// GetCellInfo returns detailed info about what's at (x, y) from the attacker's perspective
func (b *Board) GetCellInfo(x, y int, attackerID string) CellInfo {
	key := fmt.Sprintf("%d,%d", x, y)
	info := CellInfo{Empty: true}

	ownerID := b.CellOwner[key]
	if ownerID == "" {
		return info
	}

	info.Empty = false
	info.OwnerID = ownerID

	fleet := b.Fleets[ownerID]
	if fleet == nil || fleet.Company == nil {
		return info
	}

	info.OwnerName = fleet.Company.Name

	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if rack.Position[0] == x && rack.Position[1] == y {
				info.RegionID = region.ID
				info.RegionName = region.Name
				info.RackID = rack.ID
				info.IsDestroyed = rack.IsDestroyed
				info.IsRegionDestroyed = region.IsDestroyed
				info.PodCount = len(rack.Pods)

				svcPods := make(map[string]*ServiceOnRack)
				for _, pod := range rack.Pods {
					if pod.Status == game.PodRunning {
						info.RunningPods++
					}

					if _, exists := svcPods[pod.ServiceID]; !exists {
						var svc *game.Service
						for _, s := range fleet.Company.Services {
							if s.ID == pod.ServiceID {
								svc = s
								break
							}
						}
						if svc != nil {
							svcPods[pod.ServiceID] = &ServiceOnRack{
								ServiceID:   svc.ID,
								ServiceName: svc.Name,
								Affinity:    svc.Affinity,
								CanFailover: svc.CanFailover,
							}
							if svc.Affinity == game.AffinityHard {
								info.HasCritical = true
							}
						}
					}
					if svcPods[pod.ServiceID] != nil {
						svcPods[pod.ServiceID].PodCount++
						if pod.Status == game.PodRunning {
							svcPods[pod.ServiceID].RunningPods++
						}
					}
				}

				for svcID, svcInfo := range svcPods {
					info.ServiceIDs = append(info.ServiceIDs, svcID)
					info.ServicesOnRack = append(info.ServicesOnRack, *svcInfo)
				}

				break
			}
		}
	}

	b.mu.RLock()
	if b.Shots[attackerID] != nil {
		if _, hit := b.Shots[attackerID][key]; hit {
			info.WasHit = true
		}
	}
	b.mu.RUnlock()

	info.CanAttack = ownerID != attackerID && !info.WasHit

	return info
}

// GetFleetStats returns stats about a fleet
type FleetStats struct {
	TotalPods   int
	RunningPods int
	PendingPods int
	DeadPods    int
	TotalRacks  int
	AliveRacks  int
}

func (b *Board) GetFleetStats(fleet *Fleet) FleetStats {
	stats := FleetStats{}
	if fleet == nil || fleet.Company == nil {
		return stats
	}

	for _, svc := range fleet.Company.Services {
		for _, pod := range svc.Pods {
			stats.TotalPods++
			switch pod.Status {
			case game.PodRunning:
				stats.RunningPods++
			case game.PodPending:
				stats.PendingPods++
			case game.PodTerminated:
				stats.DeadPods++
			}
		}
	}

	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			stats.TotalRacks++
			if !rack.IsDestroyed {
				stats.AliveRacks++
			}
		}
	}

	return stats
}
