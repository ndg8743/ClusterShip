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

// ListCompanies returns IDs of all available company templates
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

// CompanyFromTemplate creates a Company from a template.
// This initializes the runtime structures (Racks, Pods) but doesn't place them on the board yet.
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

	// create regions with their racks
	for i, rt := range t.Regions {
		region := &Region{
			ID:        rt.ID,
			Name:      rt.Name,
			Emoji:     rt.Emoji,
			RackCount: rt.Racks,
			LatencyMs: rt.LatencyMs,
			Racks:     make([]*Rack, rt.Racks),
		}

		// create empty racks for this region
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

	// create services (pods get created during placement)
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

// TotalPods returns the total number of pods across all services.
func (c *Company) TotalPods() int {
	total := 0
	for _, s := range c.Services {
		total += s.Replicas * s.PodsPerReplica
	}
	return total
}

// HealthyPodCount returns how many pods are still running
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

// PendingPodCount returns how many pods are pending (can't be scheduled)
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
