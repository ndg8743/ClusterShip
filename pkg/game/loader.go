package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TemplatesDir is the path to company templates. Can be overridden.
var TemplatesDir = "templates"

// LoadCompanyTemplate loads a company template from the templates directory
func LoadCompanyTemplate(id string) (*CompanyTemplate, error) {
	path := filepath.Join(TemplatesDir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("company not found: %s", id)
	}

	var template CompanyTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", id, err)
	}

	return &template, nil
}

// ListCompanies returns available company template IDs
func ListCompanies() []string {
	entries, err := os.ReadDir(TemplatesDir)
	if err != nil {
		return nil
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			ids = append(ids, id)
		}
	}
	return ids
}

// CompanyFromTemplate creates a Company from a template
func CompanyFromTemplate(t *CompanyTemplate) *Company {
	c := &Company{
		ID:          t.ID,
		Name:        t.Name,
		Emoji:       t.Emoji,
		Description: t.Description,
		AIStrategy:  t.AIStrategy,
		Difficulty:  t.Difficulty,
		Regions:     make([]*Region, len(t.Regions)),
		Services:    make([]*Service, len(t.Services)),
	}

	for i, rt := range t.Regions {
		region := &Region{
			ID:        rt.ID,
			Name:      rt.Name,
			Emoji:     rt.Emoji,
			RackCount: rt.Racks,
			LatencyMs: rt.LatencyMs,
			Racks:     make([]*Rack, rt.Racks),
		}

		for j := 0; j < rt.Racks; j++ {
			region.Racks[j] = &Rack{
				ID:       fmt.Sprintf("%s-rack-%d", region.ID, j),
				RegionID: region.ID,
				Capacity: 4, // default capacity per rack
				Pods:     make([]*Pod, 0),
			}
		}

		c.Regions[i] = region
	}

	for i, st := range t.Services {
		svc := &Service{
			ID:             st.ID,
			Name:           st.Name,
			Emoji:          st.Emoji,
			Replicas:       st.Replicas,
			PodsPerReplica: st.PodsPerReplica,
			Affinity:       st.Affinity,
			Criticality:    st.Criticality,
			CanFailover:    st.CanFailover,
			IsHealthy:      true,
			Pods:           make([]*Pod, 0),
		}
		c.Services[i] = svc
	}

	return c
}

// TotalRacks returns the total number of racks across all regions.
func (c *Company) TotalRacks() int {
	total := 0
	for _, r := range c.Regions {
		total += r.RackCount
	}
	return total
}

// AdjustToConfig modifies company to match game config settings
func (c *Company) AdjustToConfig(shipsPerPlayer, racksPerShip, podsPerRack int) {
	// adjust number of regions (ships)
	if len(c.Regions) > shipsPerPlayer {
		c.Regions = c.Regions[:shipsPerPlayer]
	} else {
		// add more regions if needed
		for len(c.Regions) < shipsPerPlayer {
			idx := len(c.Regions)
			region := &Region{
				ID:        fmt.Sprintf("region-%d", idx),
				Name:      fmt.Sprintf("Region %d", idx+1),
				RackCount: racksPerShip,
				Racks:     make([]*Rack, racksPerShip),
			}
			for j := 0; j < racksPerShip; j++ {
				region.Racks[j] = &Rack{
					ID:       fmt.Sprintf("%s-rack-%d", region.ID, j),
					RegionID: region.ID,
					Capacity: podsPerRack,
					Pods:     make([]*Pod, 0),
				}
			}
			c.Regions = append(c.Regions, region)
		}
	}

	// adjust racks per region
	for _, region := range c.Regions {
		if len(region.Racks) > racksPerShip {
			region.Racks = region.Racks[:racksPerShip]
			region.RackCount = racksPerShip
		} else {
			for len(region.Racks) < racksPerShip {
				j := len(region.Racks)
				region.Racks = append(region.Racks, &Rack{
					ID:       fmt.Sprintf("%s-rack-%d", region.ID, j),
					RegionID: region.ID,
					Capacity: podsPerRack,
					Pods:     make([]*Pod, 0),
				})
			}
			region.RackCount = racksPerShip
		}

		// update rack capacity
		for _, rack := range region.Racks {
			rack.Capacity = podsPerRack
		}
	}
}

// TotalPods returns the total number of pods across all services.
func (c *Company) TotalPods() int {
	total := 0
	for _, s := range c.Services {
		total += s.Replicas * s.PodsPerReplica
	}
	return total
}

// HealthyPodCount returns running pod count
func (c *Company) HealthyPodCount() int {
	count := 0
	for _, s := range c.Services {
		for _, p := range s.Pods {
			if p.Status == PodRunning {
				count++
			}
		}
	}
	return count
}

// PendingPodCount returns pending pod count
func (c *Company) PendingPodCount() int {
	count := 0
	for _, s := range c.Services {
		for _, p := range s.Pods {
			if p.Status == PodPending {
				count++
			}
		}
	}
	return count
}
