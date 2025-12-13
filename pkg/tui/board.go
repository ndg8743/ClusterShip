package tui

import (
	"clustership/pkg/game"
	"fmt"
	"math/rand"
)

// ShotResult tracks what happened when a cell was attacked
type ShotResult struct {
	Hit        bool
	Coord      [2]int
	HitRack    *game.Rack
	HitPod     *game.Pod
	KilledPod  bool
	KilledRack bool
	Message    string
}

// Board represents the game board with both fleets placed
type Board struct {
	Width  int
	Height int

	// fleets placed on the board
	PlayerFleet *Fleet
	EnemyFleet  *Fleet

	// shot tracking
	PlayerShots map[string]*ShotResult // player's shots at enemy
	EnemyShots  map[string]*ShotResult // enemy's shots at player

	// events log
	Events []game.GameEvent
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

// NewBoard creates a game board and places both fleets
func NewBoard(width, height int, player, enemy *game.Company) *Board {
	b := &Board{
		Width:       width,
		Height:      height,
		PlayerShots: make(map[string]*ShotResult),
		EnemyShots:  make(map[string]*ShotResult),
		Events:      make([]game.GameEvent, 0),
	}

	// place fleets
	b.PlayerFleet = b.placeFleet(player, "left")
	b.EnemyFleet = b.placeFleet(enemy, "right")

	// create pods for services
	b.initPods(b.PlayerFleet)
	b.initPods(b.EnemyFleet)

	return b
}

// placeFleet places a company's regions on the board
// side: "left" or "right" - which half of the board to use
func (b *Board) placeFleet(company *game.Company, side string) *Fleet {
	fleet := &Fleet{
		Company: company,
		Regions: make([]*PlacedRegion, len(company.Regions)),
	}

	// determine placement area
	startX := 0
	endX := b.Width / 2
	if side == "right" {
		startX = b.Width / 2
		endX = b.Width
	}

	occupied := make(map[string]bool)

	for i, region := range company.Regions {
		placed := &PlacedRegion{
			Region: region,
			Racks:  make([]*PlacedRack, len(region.Racks)),
		}

		// try to place this region (ship)
		cells := b.findPlacement(region.RackCount, startX, endX, 0, b.Height, occupied)

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

			// check bounds
			if x < minX || x >= maxX || y < minY || y >= maxY {
				valid = false
				break
			}

			// check overlap
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

	// fallback: return partial placement
	return make([][2]int, length)
}

// initPods creates pods for all services and assigns them to racks
func (b *Board) initPods(fleet *Fleet) {
	if fleet == nil || fleet.Company == nil {
		return
	}

	podID := 0
	for _, svc := range fleet.Company.Services {
		svc.Pods = make([]*game.Pod, 0)

		for i := 0; i < svc.Replicas; i++ {
			// find a rack to place this pod on based on affinity
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

// findRackForPod finds a suitable rack based on service affinity
func (b *Board) findRackForPod(fleet *Fleet, svc *game.Service) *game.Rack {
	// collect all available racks
	var racks []*game.Rack
	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if !rack.IsDestroyed && len(rack.Pods) < rack.Capacity {
				racks = append(racks, rack)
			}
		}
	}

	if len(racks) == 0 {
		return nil
	}

	switch svc.Affinity {
	case game.AffinitySpread:
		// prefer racks with fewer pods
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
		// must be on specific rack (first available in same region as other pods)
		if len(svc.Pods) > 0 {
			for _, r := range racks {
				if r.RegionID == svc.Pods[0].RegionID {
					return r
				}
			}
		}
		// fallback to first available
		return racks[0]

	default:
		// random placement
		return racks[rand.Intn(len(racks))]
	}
}

// Attack executes an attack at the given coordinates
// Returns hit result and any events generated
func (b *Board) Attack(x, y int, byPlayer bool) (*ShotResult, []game.GameEvent) {
	key := fmt.Sprintf("%d,%d", x, y)
	var shots map[string]*ShotResult
	var targetFleet *Fleet

	if byPlayer {
		shots = b.PlayerShots
		targetFleet = b.EnemyFleet
	} else {
		shots = b.EnemyShots
		targetFleet = b.PlayerFleet
	}

	// already shot here?
	if _, exists := shots[key]; exists {
		return nil, nil
	}

	result := &ShotResult{
		Coord: [2]int{x, y},
	}

	events := make([]game.GameEvent, 0)

	// check if we hit a rack
	rack := b.findRackAt(targetFleet, x, y)
	if rack != nil {
		result.Hit = true
		result.HitRack = rack
		rack.HitCount++

		// damage pods on this rack
		for _, pod := range rack.Pods {
			if pod.Status == game.PodRunning {
				pod.Health--
				if pod.Health <= 0 {
					result.KilledPod = true
					result.HitPod = pod

					// try to reschedule based on affinity
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

		// check if rack is destroyed (all pods dead)
		allDead := true
		for _, pod := range rack.Pods {
			if pod.Status == game.PodRunning {
				allDead = false
				break
			}
		}
		if allDead && len(rack.Pods) > 0 {
			rack.IsDestroyed = true
			result.KilledRack = true
			events = append(events, game.GameEvent{
				Type:     "Warning",
				Reason:   "RackDestroyed",
				Message:  fmt.Sprintf("Rack %s destroyed", rack.ID),
				RegionID: rack.RegionID,
			})
		}

		result.Message = fmt.Sprintf("Hit! Rack %s", rack.ID)
	} else {
		result.Hit = false
		result.Message = "Miss"
	}

	shots[key] = result
	b.Events = append(b.Events, events...)

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
	// find the service
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
		// hard affinity can't reschedule to different rack
		return false
	}

	// find a new rack
	newRack := b.findRackForPod(fleet, svc)
	if newRack == nil || newRack.ID == pod.RackID {
		return false
	}

	// move the pod
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

	// check if enemy shot here
	if result, ok := b.EnemyShots[key]; ok {
		if result.Hit {
			if result.KilledRack {
				return CellDestroyed
			}
			return CellHit
		}
		return CellMiss
	}

	// check if we have a ship here
	if b.findRackAt(b.PlayerFleet, x, y) != nil {
		return CellShip
	}

	return CellWater
}

// GetEnemyCellState returns the state of a cell from attacking enemy
// (only shows what player has discovered)
func (b *Board) GetEnemyCellState(x, y int) CellState {
	key := fmt.Sprintf("%d,%d", x, y)

	// check if player shot here
	if result, ok := b.PlayerShots[key]; ok {
		if result.Hit {
			if result.KilledRack {
				return CellDestroyed
			}
			return CellHit
		}
		return CellMiss
	}

	// unexplored
	return CellWater
}

// FleetHealthy returns true if the fleet has any running pods
func (b *Board) FleetHealthy(fleet *Fleet) bool {
	if fleet == nil || fleet.Company == nil {
		return false
	}
	return fleet.Company.HealthyPodCount() > 0
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
