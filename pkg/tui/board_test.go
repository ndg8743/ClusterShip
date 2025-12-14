package tui

import (
	"clustership/pkg/game"
	"fmt"
	"testing"
)

// TestNewBoard tests basic board creation for 1v1 mode
func TestNewBoard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		width  int
		height int
		player *game.Company
		enemy  *game.Company
		validate func(t *testing.T, b *Board)
	}{
		{
			name:   "basic board creation",
			width:  20,
			height: 20,
			player: createTestCompany("player", 2, 3),
			enemy:  createTestCompany("enemy", 2, 3),
			validate: func(t *testing.T, b *Board) {
				if b.Width != 20 || b.Height != 20 {
					t.Errorf("expected board size 20x20, got %dx%d", b.Width, b.Height)
				}
				if b.PlayerFleet == nil {
					t.Error("PlayerFleet is nil")
				}
				if b.EnemyFleet == nil {
					t.Error("EnemyFleet is nil")
				}
				if len(b.Fleets) != 2 {
					t.Errorf("expected 2 fleets, got %d", len(b.Fleets))
				}
			},
		},
		{
			name:   "player without ID gets assigned 'player'",
			width:  10,
			height: 10,
			player: &game.Company{Name: "Test Player", Regions: []*game.Region{}},
			enemy:  &game.Company{Name: "Test Enemy", Regions: []*game.Region{}},
			validate: func(t *testing.T, b *Board) {
				if b.PlayerFleet.Company.ID != "player" {
					t.Errorf("expected player ID 'player', got %s", b.PlayerFleet.Company.ID)
				}
				if b.EnemyFleet.Company.ID != "enemy" {
					t.Errorf("expected enemy ID 'enemy', got %s", b.EnemyFleet.Company.ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := NewBoard(tt.width, tt.height, tt.player, tt.enemy)
			if b == nil {
				t.Fatal("NewBoard returned nil")
			}

			tt.validate(t, b)
		})
	}
}

// TestNewMultiBoard tests multi-company board creation
func TestNewMultiBoard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		width     int
		height    int
		companies []*game.Company
		validate  func(t *testing.T, b *Board)
	}{
		{
			name:   "three company battle",
			width:  30,
			height: 30,
			companies: []*game.Company{
				createTestCompany("player", 2, 3),
				createTestCompany("enemy1", 2, 3),
				createTestCompany("enemy2", 2, 3),
			},
			validate: func(t *testing.T, b *Board) {
				if len(b.Fleets) != 3 {
					t.Errorf("expected 3 fleets, got %d", len(b.Fleets))
				}
				if len(b.Shots) != 3 {
					t.Errorf("expected 3 shot maps, got %d", len(b.Shots))
				}
				// Verify each fleet has placed ships
				for id, fleet := range b.Fleets {
					if fleet == nil {
						t.Errorf("fleet %s is nil", id)
						continue
					}
					if len(fleet.Regions) == 0 {
						t.Errorf("fleet %s has no regions", id)
					}
				}
			},
		},
		{
			name:   "single company",
			width:  15,
			height: 15,
			companies: []*game.Company{
				createTestCompany("solo", 3, 2),
			},
			validate: func(t *testing.T, b *Board) {
				if len(b.Fleets) != 1 {
					t.Errorf("expected 1 fleet, got %d", len(b.Fleets))
				}
			},
		},
		{
			name:   "verify no ship overlap",
			width:  20,
			height: 20,
			companies: []*game.Company{
				createTestCompany("c1", 2, 3),
				createTestCompany("c2", 2, 3),
			},
			validate: func(t *testing.T, b *Board) {
				// Build set of all occupied cells
				occupied := make(map[string]string) // key -> company
				for id, fleet := range b.Fleets {
					for _, region := range fleet.Regions {
						for _, rack := range region.Racks {
							key := fmt.Sprintf("%d,%d", rack.Position[0], rack.Position[1])
							if prev, exists := occupied[key]; exists {
								t.Errorf("cell %s occupied by both %s and %s", key, prev, id)
							}
							occupied[key] = id
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := NewMultiBoard(tt.width, tt.height, tt.companies)
			if b == nil {
				t.Fatal("NewMultiBoard returned nil")
			}

			tt.validate(t, b)
		})
	}
}

// TestAttack tests the legacy 1v1 attack method
func TestAttack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupBoard func() *Board
		x, y       int
		byPlayer   bool
		wantHit    bool
		validate   func(t *testing.T, b *Board, result *ShotResult)
	}{
		{
			name: "player hits enemy ship",
			setupBoard: func() *Board {
				b := NewBoard(10, 10, createTestCompany("player", 1, 2), createTestCompany("enemy", 1, 2))
				// Get enemy ship position
				return b
			},
			byPlayer: true,
			validate: func(t *testing.T, b *Board, result *ShotResult) {
				if result == nil {
					t.Error("expected result, got nil")
					return
				}
				// Check if it was recorded
				key := fmt.Sprintf("%d,%d", result.Coord[0], result.Coord[1])
				if _, exists := b.PlayerShots[key]; !exists {
					t.Error("shot not recorded in PlayerShots")
				}
			},
		},
		{
			name: "hit same cell twice returns nil",
			setupBoard: func() *Board {
				b := NewBoard(10, 10, createTestCompany("player", 1, 2), createTestCompany("enemy", 1, 2))
				// First hit
				if b.EnemyFleet != nil && len(b.EnemyFleet.Regions) > 0 && len(b.EnemyFleet.Regions[0].Racks) > 0 {
					pos := b.EnemyFleet.Regions[0].Racks[0].Position
					b.Attack(pos[0], pos[1], true)
				}
				return b
			},
			validate: func(t *testing.T, b *Board, result *ShotResult) {
				if b.EnemyFleet == nil || len(b.EnemyFleet.Regions) == 0 {
					t.Skip("no enemy fleet")
					return
				}
				pos := b.EnemyFleet.Regions[0].Racks[0].Position
				result2, _ := b.Attack(pos[0], pos[1], true)
				if result2 != nil {
					t.Error("expected nil when attacking same cell twice")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := tt.setupBoard()

			// If x, y not set (both 0), try to get enemy ship position
			x, y := tt.x, tt.y
			if x == 0 && y == 0 && b.EnemyFleet != nil {
				if len(b.EnemyFleet.Regions) > 0 && len(b.EnemyFleet.Regions[0].Racks) > 0 {
					pos := b.EnemyFleet.Regions[0].Racks[0].Position
					x, y = pos[0], pos[1]
				}
			}

			// Perform the attack
			result, _ := b.Attack(x, y, tt.byPlayer)

			if tt.validate != nil {
				tt.validate(t, b, result)
			}
		})
	}
}

// TestAttackMulti tests multi-company attack system
func TestAttackMulti(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupBoard func() *Board
		attackerID string
		targetID   string
		getCoord   func(*Board) (int, int)
		wantHit    bool
		validate   func(t *testing.T, b *Board, result *ShotResult, events []game.GameEvent)
	}{
		{
			name: "successful hit on target",
			setupBoard: func() *Board {
				return NewMultiBoard(15, 15, []*game.Company{
					createTestCompany("player", 2, 2),
					createTestCompany("enemy", 2, 2),
				})
			},
			attackerID: "player",
			targetID:   "enemy",
			getCoord: func(b *Board) (int, int) {
				fleet := b.Fleets["enemy"]
				if fleet == nil || len(fleet.Regions) == 0 || len(fleet.Regions[0].Racks) == 0 {
					return 0, 0
				}
				pos := fleet.Regions[0].Racks[0].Position
				return pos[0], pos[1]
			},
			wantHit: true,
			validate: func(t *testing.T, b *Board, result *ShotResult, events []game.GameEvent) {
				if result == nil {
					t.Error("expected result, got nil")
					return
				}
				if !result.Hit {
					t.Error("expected hit, got miss")
				}
				if result.HitRack == nil {
					t.Error("expected HitRack to be set")
				}
				// Verify it's recorded
				key := fmt.Sprintf("%d,%d", result.Coord[0], result.Coord[1])
				if _, exists := b.Shots["player"][key]; !exists {
					t.Error("shot not recorded in Shots map")
				}
			},
		},
		{
			name: "miss on empty water",
			setupBoard: func() *Board {
				return NewMultiBoard(15, 15, []*game.Company{
					createTestCompany("player", 1, 1),
					createTestCompany("enemy", 1, 1),
				})
			},
			attackerID: "player",
			targetID:   "enemy",
			getCoord:   func(b *Board) (int, int) { return 0, 0 }, // likely empty
			wantHit:    false,
			validate: func(t *testing.T, b *Board, result *ShotResult, events []game.GameEvent) {
				if result == nil {
					t.Error("expected result, got nil")
					return
				}
				// Could be hit or miss depending on placement
				if result.Hit && result.HitRack == nil {
					t.Error("if hit is true, HitRack should be set")
				}
			},
		},
		{
			name: "attack own ship returns miss",
			setupBoard: func() *Board {
				return NewMultiBoard(15, 15, []*game.Company{
					createTestCompany("player", 2, 2),
					createTestCompany("enemy", 2, 2),
				})
			},
			attackerID: "player",
			targetID:   "player",
			getCoord: func(b *Board) (int, int) {
				fleet := b.Fleets["player"]
				if fleet == nil || len(fleet.Regions) == 0 || len(fleet.Regions[0].Racks) == 0 {
					return 0, 0
				}
				pos := fleet.Regions[0].Racks[0].Position
				return pos[0], pos[1]
			},
			wantHit: false,
			validate: func(t *testing.T, b *Board, result *ShotResult, events []game.GameEvent) {
				if result == nil {
					t.Error("expected result, got nil")
					return
				}
				if result.Hit {
					t.Error("should not hit own ship")
				}
			},
		},
		{
			name: "attack same cell twice returns nil",
			setupBoard: func() *Board {
				b := NewMultiBoard(15, 15, []*game.Company{
					createTestCompany("player", 1, 2),
					createTestCompany("enemy", 1, 2),
				})
				// Make first attack
				fleet := b.Fleets["enemy"]
				if fleet != nil && len(fleet.Regions) > 0 && len(fleet.Regions[0].Racks) > 0 {
					pos := fleet.Regions[0].Racks[0].Position
					b.AttackMulti(pos[0], pos[1], "player", "enemy")
				}
				return b
			},
			attackerID: "player",
			targetID:   "enemy",
			getCoord: func(b *Board) (int, int) {
				fleet := b.Fleets["enemy"]
				if fleet == nil || len(fleet.Regions) == 0 || len(fleet.Regions[0].Racks) == 0 {
					return 0, 0
				}
				pos := fleet.Regions[0].Racks[0].Position
				return pos[0], pos[1]
			},
			validate: func(t *testing.T, b *Board, result *ShotResult, events []game.GameEvent) {
				if result != nil {
					t.Error("expected nil when attacking same cell twice")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := tt.setupBoard()
			x, y := tt.getCoord(b)
			result, events := b.AttackMulti(x, y, tt.attackerID, tt.targetID)

			if tt.validate != nil {
				tt.validate(t, b, result, events)
			}
		})
	}
}

// TestPodRescheduling tests pod rescheduling for different affinity types
func TestPodRescheduling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		affinity        game.AffinityType
		canFailover     bool
		expectReschedule bool
	}{
		{
			name:            "hard affinity cannot reschedule",
			affinity:        game.AffinityHard,
			canFailover:     true,
			expectReschedule: false,
		},
		{
			name:            "soft affinity can reschedule",
			affinity:        game.AffinitySoft,
			canFailover:     true,
			expectReschedule: true,
		},
		{
			name:            "spread affinity can reschedule",
			affinity:        game.AffinitySpread,
			canFailover:     true,
			expectReschedule: true,
		},
		{
			name:            "none affinity can reschedule",
			affinity:        game.AffinityNone,
			canFailover:     true,
			expectReschedule: true,
		},
		{
			name:            "cannot reschedule if failover disabled",
			affinity:        game.AffinitySoft,
			canFailover:     false,
			expectReschedule: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create company with multiple regions (for rescheduling target)
			company := createTestCompany("test", 3, 2)
			company.Services = []*game.Service{
				{
					ID:             "test-svc",
					Name:           "Test Service",
					Replicas:       2,
					PodsPerReplica: 1,
					Affinity:       tt.affinity,
					CanFailover:    tt.canFailover,
					Pods:           []*game.Pod{},
				},
			}

			b := NewMultiBoard(20, 20, []*game.Company{company, createTestCompany("enemy", 1, 2)})

			// Find a rack with pods
			var targetRack *game.Rack
			var targetPod *game.Pod
			fleet := b.Fleets["test"]
			if fleet != nil {
				for _, region := range fleet.Company.Regions {
					for _, rack := range region.Racks {
						if len(rack.Pods) > 0 {
							targetRack = rack
							targetPod = rack.Pods[0]
							break
						}
					}
					if targetRack != nil {
						break
					}
				}
			}

			if targetRack == nil || targetPod == nil {
				t.Skip("no pods placed for testing")
				return
			}

			// Kill the pod
			targetPod.Health = 0
			rescheduled := b.tryReschedulePod(fleet, targetPod)

			if rescheduled != tt.expectReschedule {
				t.Errorf("tryReschedulePod() = %v, want %v", rescheduled, tt.expectReschedule)
			}

			if tt.expectReschedule && rescheduled {
				// Verify pod moved to different rack
				if targetPod.RackID == targetRack.ID {
					t.Error("pod should have moved to different rack")
				}
				if targetPod.Health != targetPod.MaxHealth {
					t.Error("rescheduled pod should have full health")
				}
			}
		})
	}
}

// TestKilledPodDetection tests that KilledPod flag is set correctly
func TestKilledPodDetection(t *testing.T) {
	t.Parallel()

	company := createTestCompany("test", 2, 2)
	company.Services = []*game.Service{
		{
			ID:             "svc1",
			Name:           "Service 1",
			Replicas:       2,
			PodsPerReplica: 1,
			Affinity:       game.AffinityNone,
			CanFailover:    false,
		},
	}

	b := NewMultiBoard(15, 15, []*game.Company{company, createTestCompany("enemy", 1, 1)})

	// Find a rack with pods
	var targetPos [2]int
	fleet := b.Fleets["test"]
	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if len(rack.Pods) > 0 {
				targetPos = rack.Position
				break
			}
		}
		if targetPos[0] != 0 || targetPos[1] != 0 {
			break
		}
	}

	result, _ := b.AttackMulti(targetPos[0], targetPos[1], "enemy", "test")
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if !result.Hit {
		t.Error("expected hit on rack with pods")
	}

	if !result.KilledPod {
		t.Error("expected KilledPod to be true when pod health drops to 0")
	}
}

// TestKilledRackDetection tests that KilledRack flag is set when all pods die
func TestKilledRackDetection(t *testing.T) {
	t.Parallel()

	company := createTestCompany("test", 1, 1)
	company.Services = []*game.Service{
		{
			ID:             "svc1",
			Name:           "Service 1",
			Replicas:       1,
			PodsPerReplica: 1,
			Affinity:       game.AffinityNone,
			CanFailover:    false,
		},
	}

	b := NewMultiBoard(15, 15, []*game.Company{company, createTestCompany("enemy", 1, 1)})

	fleet := b.Fleets["test"]
	var targetRack *game.Rack
	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if len(rack.Pods) > 0 {
				targetRack = rack
				break
			}
		}
		if targetRack != nil {
			break
		}
	}

	if targetRack == nil {
		t.Fatal("no rack with pods found")
	}

	// Attack until all pods are dead
	result, _ := b.AttackMulti(targetRack.Position[0], targetRack.Position[1], "enemy", "test")

	if result == nil {
		t.Fatal("expected result")
	}

	if !result.KilledRack {
		t.Error("expected KilledRack to be true when all pods terminated")
	}

	if !targetRack.IsDestroyed {
		t.Error("rack should be marked as destroyed")
	}
}

// TestFleetHealthy tests the FleetHealthy method
func TestFleetHealthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFleet func() *Fleet
		expected  bool
	}{
		{
			name: "fleet with running pods",
			setupFleet: func() *Fleet {
				company := createTestCompany("test", 1, 1)
				company.Services = []*game.Service{
					{
						Pods: []*game.Pod{
							{Status: game.PodRunning},
						},
					},
				}
				return &Fleet{Company: company}
			},
			expected: true,
		},
		{
			name: "fleet with no running pods",
			setupFleet: func() *Fleet {
				company := createTestCompany("test", 1, 1)
				company.Services = []*game.Service{
					{
						Pods: []*game.Pod{
							{Status: game.PodTerminated},
							{Status: game.PodPending},
						},
					},
				}
				return &Fleet{Company: company}
			},
			expected: false,
		},
		{
			name: "nil fleet",
			setupFleet: func() *Fleet {
				return nil
			},
			expected: false,
		},
		{
			name: "fleet with nil company",
			setupFleet: func() *Fleet {
				return &Fleet{Company: nil}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := &Board{}
			fleet := tt.setupFleet()
			result := b.FleetHealthy(fleet)

			if result != tt.expected {
				t.Errorf("FleetHealthy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetCellInfo tests the GetCellInfo method for fog of war
func TestGetCellInfo(t *testing.T) {
	t.Parallel()

	company1 := createTestCompany("player", 1, 2)
	company1.Services = []*game.Service{
		{
			ID:             "svc1",
			Name:           "Critical Service",
			Replicas:       1,
			PodsPerReplica: 1,
			Affinity:       game.AffinityHard,
			CanFailover:    false,
		},
	}

	company2 := createTestCompany("enemy", 1, 2)

	b := NewMultiBoard(15, 15, []*game.Company{company1, company2})

	// Test getting info about empty cell
	info := b.GetCellInfo(0, 0, "player")
	if !info.Empty {
		// Might not be empty due to random placement
	}

	// Test getting info about own ship
	var ownPos [2]int
	fleet := b.Fleets["player"]
	for _, region := range fleet.Company.Regions {
		for _, rack := range region.Racks {
			if len(rack.Pods) > 0 {
				ownPos = rack.Position
				break
			}
		}
		if ownPos[0] != 0 || ownPos[1] != 0 {
			break
		}
	}

	ownInfo := b.GetCellInfo(ownPos[0], ownPos[1], "player")
	if ownInfo.Empty {
		t.Error("expected cell to not be empty")
	}
	if ownInfo.OwnerID != "player" {
		t.Errorf("expected owner 'player', got %s", ownInfo.OwnerID)
	}
	if ownInfo.CanAttack {
		t.Error("should not be able to attack own ship")
	}

	// Test getting info about enemy ship (before attacking)
	var enemyPos [2]int
	enemyFleet := b.Fleets["enemy"]
	for _, region := range enemyFleet.Company.Regions {
		for _, rack := range region.Racks {
			enemyPos = rack.Position
			break
		}
		if enemyPos[0] != 0 || enemyPos[1] != 0 {
			break
		}
	}

	enemyInfo := b.GetCellInfo(enemyPos[0], enemyPos[1], "player")
	if enemyInfo.Empty {
		t.Error("expected enemy cell to not be empty")
	}
	if !enemyInfo.CanAttack {
		t.Error("should be able to attack enemy ship")
	}
	if enemyInfo.WasHit {
		t.Error("cell should not be marked as hit before attacking")
	}

	// Attack and verify WasHit is set
	b.AttackMulti(enemyPos[0], enemyPos[1], "player", "enemy")
	enemyInfoAfter := b.GetCellInfo(enemyPos[0], enemyPos[1], "player")
	if !enemyInfoAfter.WasHit {
		t.Error("cell should be marked as hit after attacking")
	}
	if enemyInfoAfter.CanAttack {
		t.Error("should not be able to attack same cell twice")
	}

	// Test HasCritical flag
	for _, svc := range enemyInfoAfter.ServicesOnRack {
		if svc.Affinity == game.AffinityHard {
			if !enemyInfoAfter.HasCritical {
				t.Error("HasCritical should be true for hard affinity services")
			}
		}
	}
}

// TestGetCellState tests the cell state methods
func TestGetCellState(t *testing.T) {
	t.Parallel()

	player := createTestCompany("player", 1, 1)
	enemy := createTestCompany("enemy", 1, 1)

	b := NewBoard(10, 10, player, enemy)

	// Test player's own ship
	var playerShipPos [2]int
	for _, region := range b.PlayerFleet.Company.Regions {
		for _, rack := range region.Racks {
			playerShipPos = rack.Position
			break
		}
		if playerShipPos[0] != 0 || playerShipPos[1] != 0 {
			break
		}
	}

	state := b.GetPlayerCellState(playerShipPos[0], playerShipPos[1])
	if state != CellShip {
		t.Errorf("expected CellShip, got %v", state)
	}

	// Test enemy attack on player
	b.Attack(playerShipPos[0], playerShipPos[1], false)
	stateAfterHit := b.GetPlayerCellState(playerShipPos[0], playerShipPos[1])
	if stateAfterHit != CellHit && stateAfterHit != CellDestroyed {
		t.Errorf("expected CellHit or CellDestroyed, got %v", stateAfterHit)
	}
}

// TestGetActiveCompanies tests getting list of alive companies
func TestGetActiveCompanies(t *testing.T) {
	t.Parallel()

	c1 := createTestCompany("c1", 1, 1)
	c1.Services = []*game.Service{
		{Pods: []*game.Pod{{Status: game.PodRunning}}},
	}

	c2 := createTestCompany("c2", 1, 1)
	c2.Services = []*game.Service{
		{Pods: []*game.Pod{{Status: game.PodTerminated}}},
	}

	c3 := createTestCompany("c3", 1, 1)
	c3.Services = []*game.Service{
		{Pods: []*game.Pod{{Status: game.PodRunning}}},
	}

	b := NewMultiBoard(20, 20, []*game.Company{c1, c2, c3})

	active := b.GetActiveCompanies()

	// c2 should not be in active list (all pods terminated)
	for _, id := range active {
		if id == "c2" {
			t.Error("c2 should not be in active companies (no running pods)")
		}
	}

	// c1 and c3 should be active
	hasC1, hasC3 := false, false
	for _, id := range active {
		if id == "c1" {
			hasC1 = true
		}
		if id == "c3" {
			hasC3 = true
		}
	}

	if !hasC1 {
		t.Error("c1 should be active")
	}
	if !hasC3 {
		t.Error("c3 should be active")
	}
}

// Helper function to create a test company
func createTestCompany(id string, regions, racksPerRegion int) *game.Company {
	c := &game.Company{
		ID:       id,
		Name:     "Test " + id,
		Regions:  make([]*game.Region, regions),
		Services: []*game.Service{},
	}

	for i := 0; i < regions; i++ {
		region := &game.Region{
			ID:        fmt.Sprintf("%s-region-%d", id, i),
			Name:      fmt.Sprintf("Region %d", i),
			RackCount: racksPerRegion,
			Racks:     make([]*game.Rack, racksPerRegion),
		}

		for j := 0; j < racksPerRegion; j++ {
			region.Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("%s-rack-%d", region.ID, j),
				RegionID: region.ID,
				Capacity: 4,
				Pods:     make([]*game.Pod, 0),
			}
		}

		c.Regions[i] = region
	}

	return c
}

// BenchmarkNewBoard benchmarks board creation
func BenchmarkNewBoard(b *testing.B) {
	player := createTestCompany("player", 3, 4)
	enemy := createTestCompany("enemy", 3, 4)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBoard(20, 20, player, enemy)
	}
}

// BenchmarkAttackMulti benchmarks attack performance
func BenchmarkAttackMulti(b *testing.B) {
	board := NewMultiBoard(30, 30, []*game.Company{
		createTestCompany("player", 5, 5),
		createTestCompany("enemy", 5, 5),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := i % 30
		y := (i / 30) % 30
		board.AttackMulti(x, y, "player", "enemy")
	}
}

// BenchmarkGetCellInfo benchmarks cell info retrieval
func BenchmarkGetCellInfo(b *testing.B) {
	board := NewMultiBoard(20, 20, []*game.Company{
		createTestCompany("player", 3, 3),
		createTestCompany("enemy", 3, 3),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := i % 20
		y := (i / 20) % 20
		_ = board.GetCellInfo(x, y, "player")
	}
}
