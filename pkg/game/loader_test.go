package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadCompanyTemplate tests loading company templates from JSON files
func TestLoadCompanyTemplate(t *testing.T) {
	t.Parallel()

	// Save original TemplatesDir and restore after test
	originalDir := TemplatesDir
	t.Cleanup(func() {
		TemplatesDir = originalDir
	})

	tests := []struct {
		name      string
		companyID string
		setupFunc func(t *testing.T) string // returns temp dir path
		wantErr   bool
		validate  func(t *testing.T, tmpl *CompanyTemplate)
	}{
		{
			name:      "load valid template",
			companyID: "testco",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				tmpl := CompanyTemplate{
					ID:          "testco",
					Name:        "Test Company",
					Emoji:       "🧪",
					Description: "Test description",
					Regions: []RegionTemplate{
						{
							ID:        "region-1",
							Name:      "Region 1",
							Emoji:     "🌎",
							Racks:     3,
							LatencyMs: 50,
						},
					},
					Services: []ServiceTemplate{
						{
							ID:             "svc-1",
							Name:           "Test Service",
							Emoji:          "⚙️",
							Replicas:       2,
							PodsPerReplica: 1,
							Affinity:       AffinitySpread,
							Criticality:    "high",
							CanFailover:    true,
						},
					},
					AIStrategy: AIHunter,
					Difficulty: "medium",
				}
				data, _ := json.Marshal(tmpl)
				os.WriteFile(filepath.Join(dir, "testco.json"), data, 0644)
				return dir
			},
			wantErr: false,
			validate: func(t *testing.T, tmpl *CompanyTemplate) {
				if tmpl.ID != "testco" {
					t.Errorf("expected ID 'testco', got %s", tmpl.ID)
				}
				if tmpl.Name != "Test Company" {
					t.Errorf("expected Name 'Test Company', got %s", tmpl.Name)
				}
				if len(tmpl.Regions) != 1 {
					t.Errorf("expected 1 region, got %d", len(tmpl.Regions))
				}
				if len(tmpl.Services) != 1 {
					t.Errorf("expected 1 service, got %d", len(tmpl.Services))
				}
			},
		},
		{
			name:      "missing template file",
			companyID: "nonexistent",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: true,
		},
		{
			name:      "malformed JSON",
			companyID: "malformed",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "malformed.json"), []byte("{invalid json"), 0644)
				return dir
			},
			wantErr: true,
		},
		{
			name:      "empty JSON object",
			companyID: "empty",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "empty.json"), []byte("{}"), 0644)
				return dir
			},
			wantErr: false,
			validate: func(t *testing.T, tmpl *CompanyTemplate) {
				if tmpl.ID != "" {
					t.Errorf("expected empty ID, got %s", tmpl.ID)
				}
				if len(tmpl.Regions) != 0 {
					t.Errorf("expected 0 regions, got %d", len(tmpl.Regions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel - tests write to global TemplatesDir
			TemplatesDir = tt.setupFunc(t)

			tmpl, err := LoadCompanyTemplate(tt.companyID)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCompanyTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, tmpl)
			}
		})
	}
}

// TestCompanyFromTemplate tests converting templates to Company instances
func TestCompanyFromTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template *CompanyTemplate
		validate func(t *testing.T, c *Company)
	}{
		{
			name: "basic template conversion",
			template: &CompanyTemplate{
				ID:          "test",
				Name:        "Test Co",
				Emoji:       "🧪",
				Description: "Test description",
				Regions: []RegionTemplate{
					{
						ID:        "region-1",
						Name:      "Region 1",
						Emoji:     "🌎",
						Racks:     3,
						LatencyMs: 50,
					},
				},
				Services: []ServiceTemplate{
					{
						ID:             "svc-1",
						Name:           "Service 1",
						Emoji:          "⚙️",
						Replicas:       2,
						PodsPerReplica: 1,
						Affinity:       AffinityHard,
						Criticality:    "high",
						CanFailover:    false,
					},
				},
				AIStrategy: AIHunter,
				Difficulty: "hard",
			},
			validate: func(t *testing.T, c *Company) {
				if c.ID != "test" {
					t.Errorf("expected ID 'test', got %s", c.ID)
				}
				if len(c.Regions) != 1 {
					t.Fatalf("expected 1 region, got %d", len(c.Regions))
				}
				region := c.Regions[0]
				if region.RackCount != 3 {
					t.Errorf("expected 3 racks, got %d", region.RackCount)
				}
				if len(region.Racks) != 3 {
					t.Errorf("expected 3 rack instances, got %d", len(region.Racks))
				}
				// Check rack initialization
				for i, rack := range region.Racks {
					if rack == nil {
						t.Errorf("rack %d is nil", i)
						continue
					}
					if rack.RegionID != region.ID {
						t.Errorf("rack %d has wrong region ID: %s", i, rack.RegionID)
					}
					if rack.Capacity != 4 {
						t.Errorf("rack %d has capacity %d, expected 4", i, rack.Capacity)
					}
					if rack.Pods == nil {
						t.Errorf("rack %d has nil Pods slice", i)
					}
				}
				if len(c.Services) != 1 {
					t.Fatalf("expected 1 service, got %d", len(c.Services))
				}
				svc := c.Services[0]
				if svc.Affinity != AffinityHard {
					t.Errorf("expected hard affinity, got %s", svc.Affinity)
				}
				if !svc.IsHealthy {
					t.Error("service should start healthy")
				}
			},
		},
		{
			name: "multiple regions and services",
			template: &CompanyTemplate{
				ID:   "multi",
				Name: "Multi Co",
				Regions: []RegionTemplate{
					{ID: "r1", Name: "Region 1", Racks: 2, LatencyMs: 10},
					{ID: "r2", Name: "Region 2", Racks: 3, LatencyMs: 20},
					{ID: "r3", Name: "Region 3", Racks: 1, LatencyMs: 30},
				},
				Services: []ServiceTemplate{
					{ID: "s1", Name: "Svc 1", Replicas: 1, PodsPerReplica: 1, Affinity: AffinitySpread, CanFailover: true},
					{ID: "s2", Name: "Svc 2", Replicas: 2, PodsPerReplica: 2, Affinity: AffinitySoft, CanFailover: true},
					{ID: "s3", Name: "Svc 3", Replicas: 3, PodsPerReplica: 1, Affinity: AffinityNone, CanFailover: false},
				},
				AIStrategy: AIAggressive,
			},
			validate: func(t *testing.T, c *Company) {
				if len(c.Regions) != 3 {
					t.Errorf("expected 3 regions, got %d", len(c.Regions))
				}
				if len(c.Services) != 3 {
					t.Errorf("expected 3 services, got %d", len(c.Services))
				}
				totalRacks := 0
				for _, r := range c.Regions {
					totalRacks += len(r.Racks)
				}
				if totalRacks != 6 {
					t.Errorf("expected 6 total racks (2+3+1), got %d", totalRacks)
				}
			},
		},
		{
			name: "empty template",
			template: &CompanyTemplate{
				ID:   "empty",
				Name: "Empty Co",
			},
			validate: func(t *testing.T, c *Company) {
				if len(c.Regions) != 0 {
					t.Errorf("expected 0 regions, got %d", len(c.Regions))
				}
				if len(c.Services) != 0 {
					t.Errorf("expected 0 services, got %d", len(c.Services))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := CompanyFromTemplate(tt.template)
			if c == nil {
				t.Fatal("CompanyFromTemplate returned nil")
			}

			tt.validate(t, c)
		})
	}
}

// TestAdjustToConfig tests adjusting company to match game configuration
func TestAdjustToConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		initialRegions int
		initialRacks   int
		shipsPerPlayer int
		racksPerShip   int
		podsPerRack    int
		validate       func(t *testing.T, c *Company)
	}{
		{
			name:           "expand ships",
			initialRegions: 2,
			initialRacks:   3,
			shipsPerPlayer: 5,
			racksPerShip:   3,
			podsPerRack:    4,
			validate: func(t *testing.T, c *Company) {
				if len(c.Regions) != 5 {
					t.Errorf("expected 5 regions, got %d", len(c.Regions))
				}
				for i, r := range c.Regions {
					if r.RackCount != 3 {
						t.Errorf("region %d: expected 3 racks, got %d", i, r.RackCount)
					}
					if len(r.Racks) != 3 {
						t.Errorf("region %d: expected 3 rack instances, got %d", i, len(r.Racks))
					}
					for j, rack := range r.Racks {
						if rack.Capacity != 4 {
							t.Errorf("region %d rack %d: expected capacity 4, got %d", i, j, rack.Capacity)
						}
					}
				}
			},
		},
		{
			name:           "shrink ships",
			initialRegions: 5,
			initialRacks:   4,
			shipsPerPlayer: 3,
			racksPerShip:   4,
			podsPerRack:    2,
			validate: func(t *testing.T, c *Company) {
				if len(c.Regions) != 3 {
					t.Errorf("expected 3 regions, got %d", len(c.Regions))
				}
				for i, r := range c.Regions {
					if r.RackCount != 4 {
						t.Errorf("region %d: expected 4 racks, got %d", i, r.RackCount)
					}
					for j, rack := range r.Racks {
						if rack.Capacity != 2 {
							t.Errorf("region %d rack %d: expected capacity 2, got %d", i, j, rack.Capacity)
						}
					}
				}
			},
		},
		{
			name:           "expand racks per ship",
			initialRegions: 3,
			initialRacks:   2,
			shipsPerPlayer: 3,
			racksPerShip:   5,
			podsPerRack:    3,
			validate: func(t *testing.T, c *Company) {
				if len(c.Regions) != 3 {
					t.Errorf("expected 3 regions, got %d", len(c.Regions))
				}
				for i, r := range c.Regions {
					if len(r.Racks) != 5 {
						t.Errorf("region %d: expected 5 racks, got %d", i, len(r.Racks))
					}
				}
			},
		},
		{
			name:           "shrink racks per ship",
			initialRegions: 2,
			initialRacks:   6,
			shipsPerPlayer: 2,
			racksPerShip:   3,
			podsPerRack:    4,
			validate: func(t *testing.T, c *Company) {
				for i, r := range c.Regions {
					if len(r.Racks) != 3 {
						t.Errorf("region %d: expected 3 racks, got %d", i, len(r.Racks))
					}
				}
			},
		},
		{
			name:           "update pod capacity",
			initialRegions: 2,
			initialRacks:   3,
			shipsPerPlayer: 2,
			racksPerShip:   3,
			podsPerRack:    8,
			validate: func(t *testing.T, c *Company) {
				for i, r := range c.Regions {
					for j, rack := range r.Racks {
						if rack.Capacity != 8 {
							t.Errorf("region %d rack %d: expected capacity 8, got %d", i, j, rack.Capacity)
						}
					}
				}
			},
		},
		{
			name:           "zero regions to some regions",
			initialRegions: 0,
			initialRacks:   0,
			shipsPerPlayer: 3,
			racksPerShip:   4,
			podsPerRack:    2,
			validate: func(t *testing.T, c *Company) {
				if len(c.Regions) != 3 {
					t.Errorf("expected 3 regions, got %d", len(c.Regions))
				}
				for i, r := range c.Regions {
					if len(r.Racks) != 4 {
						t.Errorf("region %d: expected 4 racks, got %d", i, len(r.Racks))
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Company{
				ID:      "test",
				Name:    "Test",
				Regions: make([]*Region, tt.initialRegions),
			}

			// Initialize regions
			for i := 0; i < tt.initialRegions; i++ {
				c.Regions[i] = &Region{
					ID:        "region-" + string(rune('A'+i)),
					Name:      "Region " + string(rune('A'+i)),
					RackCount: tt.initialRacks,
					Racks:     make([]*Rack, tt.initialRacks),
				}
				for j := 0; j < tt.initialRacks; j++ {
					c.Regions[i].Racks[j] = &Rack{
						ID:       "rack-" + string(rune('0'+j)),
						RegionID: c.Regions[i].ID,
						Capacity: 4,
						Pods:     make([]*Pod, 0),
					}
				}
			}

			c.AdjustToConfig(tt.shipsPerPlayer, tt.racksPerShip, tt.podsPerRack)
			tt.validate(t, c)
		})
	}
}

// TestTotalRacks tests the TotalRacks method
func TestTotalRacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		company  *Company
		expected int
	}{
		{
			name: "single region with racks",
			company: &Company{
				Regions: []*Region{
					{RackCount: 5},
				},
			},
			expected: 5,
		},
		{
			name: "multiple regions",
			company: &Company{
				Regions: []*Region{
					{RackCount: 3},
					{RackCount: 4},
					{RackCount: 2},
				},
			},
			expected: 9,
		},
		{
			name: "no regions",
			company: &Company{
				Regions: []*Region{},
			},
			expected: 0,
		},
		{
			name: "regions with zero racks",
			company: &Company{
				Regions: []*Region{
					{RackCount: 0},
					{RackCount: 5},
					{RackCount: 0},
				},
			},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.company.TotalRacks()
			if result != tt.expected {
				t.Errorf("TotalRacks() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestTotalPods tests the TotalPods method
func TestTotalPods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		company  *Company
		expected int
	}{
		{
			name: "single service",
			company: &Company{
				Services: []*Service{
					{Replicas: 3, PodsPerReplica: 2},
				},
			},
			expected: 6,
		},
		{
			name: "multiple services",
			company: &Company{
				Services: []*Service{
					{Replicas: 2, PodsPerReplica: 1},
					{Replicas: 3, PodsPerReplica: 2},
					{Replicas: 1, PodsPerReplica: 4},
				},
			},
			expected: 12, // 2 + 6 + 4
		},
		{
			name: "no services",
			company: &Company{
				Services: []*Service{},
			},
			expected: 0,
		},
		{
			name: "services with zero replicas",
			company: &Company{
				Services: []*Service{
					{Replicas: 0, PodsPerReplica: 5},
					{Replicas: 3, PodsPerReplica: 1},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.company.TotalPods()
			if result != tt.expected {
				t.Errorf("TotalPods() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestHealthyPodCount tests the HealthyPodCount method
func TestHealthyPodCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		company  *Company
		expected int
	}{
		{
			name: "all pods running",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodRunning},
							{Status: PodRunning},
							{Status: PodRunning},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "mixed pod statuses",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodRunning},
							{Status: PodPending},
							{Status: PodRunning},
							{Status: PodTerminated},
							{Status: PodRunning},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "no running pods",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodPending},
							{Status: PodTerminated},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "multiple services",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodRunning},
							{Status: PodRunning},
						},
					},
					{
						Pods: []*Pod{
							{Status: PodPending},
							{Status: PodRunning},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name:     "no services",
			company:  &Company{Services: []*Service{}},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.company.HealthyPodCount()
			if result != tt.expected {
				t.Errorf("HealthyPodCount() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestPendingPodCount tests the PendingPodCount method
func TestPendingPodCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		company  *Company
		expected int
	}{
		{
			name: "all pods pending",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodPending},
							{Status: PodPending},
						},
					},
				},
			},
			expected: 2,
		},
		{
			name: "mixed statuses",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodRunning},
							{Status: PodPending},
							{Status: PodTerminated},
							{Status: PodPending},
						},
					},
				},
			},
			expected: 2,
		},
		{
			name: "no pending pods",
			company: &Company{
				Services: []*Service{
					{
						Pods: []*Pod{
							{Status: PodRunning},
							{Status: PodTerminated},
						},
					},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.company.PendingPodCount()
			if result != tt.expected {
				t.Errorf("PendingPodCount() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// BenchmarkCompanyFromTemplate benchmarks template conversion
func BenchmarkCompanyFromTemplate(b *testing.B) {
	template := &CompanyTemplate{
		ID:   "bench",
		Name: "Benchmark Co",
		Regions: []RegionTemplate{
			{ID: "r1", Racks: 5},
			{ID: "r2", Racks: 4},
			{ID: "r3", Racks: 3},
		},
		Services: []ServiceTemplate{
			{ID: "s1", Replicas: 10, PodsPerReplica: 1},
			{ID: "s2", Replicas: 5, PodsPerReplica: 2},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CompanyFromTemplate(template)
	}
}

// BenchmarkAdjustToConfig benchmarks config adjustment
func BenchmarkAdjustToConfig(b *testing.B) {
	c := &Company{
		ID:   "bench",
		Name: "Benchmark",
		Regions: []*Region{
			{ID: "r1", RackCount: 3, Racks: make([]*Rack, 3)},
			{ID: "r2", RackCount: 3, Racks: make([]*Rack, 3)},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.AdjustToConfig(5, 4, 4)
	}
}

// BenchmarkTotalPods benchmarks pod counting
func BenchmarkTotalPods(b *testing.B) {
	c := &Company{
		Services: []*Service{
			{Replicas: 10, PodsPerReplica: 2},
			{Replicas: 5, PodsPerReplica: 3},
			{Replicas: 8, PodsPerReplica: 1},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.TotalPods()
	}
}
