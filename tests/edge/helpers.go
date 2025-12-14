package edge

import (
	"clustership/pkg/game"
	"clustership/pkg/tui"
	"fmt"
	"testing"
)

// CreateTestCompany creates a test company with configurable parameters
func CreateTestCompany(id string, regionCount, racksPerRegion, serviceCount, replicasPerService int) *game.Company {
	company := &game.Company{
		ID:      id,
		Name:    fmt.Sprintf("Company %s", id),
		Regions: make([]*game.Region, regionCount),
		Services: make([]*game.Service, serviceCount),
	}

	// Create regions
	for i := 0; i < regionCount; i++ {
		region := &game.Region{
			ID:        fmt.Sprintf("%s-region-%d", id, i),
			Name:      fmt.Sprintf("Region %d", i),
			RackCount: racksPerRegion,
			Racks:     make([]*game.Rack, racksPerRegion),
		}

		// Create racks
		for j := 0; j < racksPerRegion; j++ {
			region.Racks[j] = &game.Rack{
				ID:       fmt.Sprintf("%s-rack-%d-%d", id, i, j),
				RegionID: region.ID,
				Capacity: 10,
				Pods:     make([]*game.Pod, 0),
			}
		}

		company.Regions[i] = region
	}

	// Create services
	for i := 0; i < serviceCount; i++ {
		company.Services[i] = &game.Service{
			ID:          fmt.Sprintf("%s-service-%d", id, i),
			Name:        fmt.Sprintf("Service %d", i),
			Replicas:    replicasPerService,
			Affinity:    game.AffinityNone,
			CanFailover: true,
			Pods:        make([]*game.Pod, 0),
		}
	}

	return company
}

// CreateTestCompanyWithAffinity creates a company with specific affinity settings
func CreateTestCompanyWithAffinity(id string, affinity game.AffinityType, canFailover bool, racks, replicas int) *game.Company {
	company := &game.Company{
		ID:   id,
		Name: fmt.Sprintf("Company %s", id),
		Regions: []*game.Region{
			{
				ID:        fmt.Sprintf("%s-region", id),
				Name:      "Main Region",
				RackCount: racks,
				Racks:     make([]*game.Rack, racks),
			},
		},
		Services: []*game.Service{
			{
				ID:          fmt.Sprintf("%s-service", id),
				Name:        "Test Service",
				Replicas:    replicas,
				Affinity:    affinity,
				CanFailover: canFailover,
				Pods:        make([]*game.Pod, 0),
			},
		},
	}

	// Create racks
	for i := 0; i < racks; i++ {
		company.Regions[0].Racks[i] = &game.Rack{
			ID:       fmt.Sprintf("%s-rack-%d", id, i),
			RegionID: company.Regions[0].ID,
			Capacity: 10,
			Pods:     make([]*game.Pod, 0),
		}
	}

	return company
}

// DestroyRack completely destroys a rack by attacking until all pods are dead
func DestroyRack(t *testing.T, board *tui.Board, rack *game.Rack, attackerID, targetID string) {
	t.Helper()

	pos := rack.Position
	maxAttempts := 100
	attempts := 0

	for !rack.IsDestroyed && attempts < maxAttempts {
		result, _ := board.AttackMulti(pos[0], pos[1], attackerID, targetID)
		if result == nil {
			// Already attacked this position
			break
		}
		attempts++
	}

	if !rack.IsDestroyed {
		t.Errorf("Failed to destroy rack %s after %d attempts", rack.ID, attempts)
	}
}

// CountRunningPods counts running pods for a company
func CountRunningPods(company *game.Company) int {
	count := 0
	for _, svc := range company.Services {
		for _, pod := range svc.Pods {
			if pod.Status == game.PodRunning {
				count++
			}
		}
	}
	return count
}

// CountRunningPodsForService counts running pods for a specific service
func CountRunningPodsForService(service *game.Service) int {
	count := 0
	for _, pod := range service.Pods {
		if pod.Status == game.PodRunning {
			count++
		}
	}
	return count
}

// GetPodDistribution returns a map of rackID -> pod count for a service
func GetPodDistribution(service *game.Service) map[string]int {
	dist := make(map[string]int)
	for _, pod := range service.Pods {
		if pod.Status == game.PodRunning {
			dist[pod.RackID]++
		}
	}
	return dist
}

// GetRegionDistribution returns a map of regionID -> pod count for a service
func GetRegionDistribution(service *game.Service) map[string]int {
	dist := make(map[string]int)
	for _, pod := range service.Pods {
		if pod.Status == game.PodRunning {
			dist[pod.RegionID]++
		}
	}
	return dist
}

// VerifyNoPodsOnDestroyedRacks verifies no running pods exist on destroyed racks
func VerifyNoPodsOnDestroyedRacks(t *testing.T, company *game.Company) {
	t.Helper()

	for _, region := range company.Regions {
		for _, rack := range region.Racks {
			if rack.IsDestroyed {
				runningCount := 0
				for _, pod := range rack.Pods {
					if pod.Status == game.PodRunning {
						runningCount++
					}
				}
				if runningCount > 0 {
					t.Errorf("Destroyed rack %s has %d running pods", rack.ID, runningCount)
				}
			}
		}
	}
}

// AssertDistributionIsEven verifies pod distribution is relatively even (for spread affinity)
func AssertDistributionIsEven(t *testing.T, distribution map[string]int, maxDeviation int) {
	t.Helper()

	if len(distribution) == 0 {
		return
	}

	total := 0
	for _, count := range distribution {
		total += count
	}

	expectedPerRack := total / len(distribution)

	for rackID, count := range distribution {
		deviation := count - expectedPerRack
		if deviation < 0 {
			deviation = -deviation
		}

		if deviation > maxDeviation {
			t.Errorf("Rack %s has uneven distribution: %d pods (expected ~%d, deviation %d)",
				rackID, count, expectedPerRack, deviation)
		}
	}
}

// CreateMinimalBoard creates a minimal test board for simple tests
func CreateMinimalBoard(t *testing.T) *tui.Board {
	t.Helper()

	player := CreateTestCompany("player", 1, 3, 1, 3)
	enemy := CreateTestCompany("enemy", 1, 3, 1, 3)

	return tui.NewBoard(50, 50, player, enemy)
}

// CreateMultiCompanyBoard creates a board with multiple companies
func CreateMultiCompanyBoard(t *testing.T, companyCount int) *tui.Board {
	t.Helper()

	companies := make([]*game.Company, companyCount)
	for i := 0; i < companyCount; i++ {
		companies[i] = CreateTestCompany(fmt.Sprintf("company-%d", i), 1, 5, 1, 5)
	}

	return tui.NewMultiBoard(100, 100, companies)
}

// LogPodStatus logs detailed pod status for debugging
func LogPodStatus(t *testing.T, company *game.Company) {
	t.Helper()

	for _, svc := range company.Services {
		running := 0
		terminated := 0
		pending := 0

		for _, pod := range svc.Pods {
			switch pod.Status {
			case game.PodRunning:
				running++
			case game.PodTerminated:
				terminated++
			case game.PodPending:
				pending++
			}
		}

		t.Logf("Service %s: Running=%d, Terminated=%d, Pending=%d, Total=%d",
			svc.ID, running, terminated, pending, len(svc.Pods))
	}
}

// LogRackStatus logs detailed rack status for debugging
func LogRackStatus(t *testing.T, company *game.Company) {
	t.Helper()

	for _, region := range company.Regions {
		t.Logf("Region %s:", region.Name)
		for _, rack := range region.Racks {
			status := "ACTIVE"
			if rack.IsDestroyed {
				status = "DESTROYED"
			}
			t.Logf("  Rack %s: %s, Pods=%d, HitCount=%d",
				rack.ID, status, len(rack.Pods), rack.HitCount)
		}
	}
}
