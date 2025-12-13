package game

// Company represents a tech company's infrastructure (the fleet).
type Company struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Emoji       string    `json:"emoji"`
	Description string    `json:"description"`
	Regions     []*Region `json:"regions"`
	Services    []*Service `json:"services"`
	AIStrategy  AIStrategy `json:"ai_strategy"`
	Difficulty  string    `json:"difficulty"`
}

// Region is a data center containing multiple racks.
type Region struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Emoji     string   `json:"emoji"`
	RackCount int      `json:"racks"` // how many server racks in this region
	LatencyMs int      `json:"latency_ms"`
	Racks     []*Rack  `json:"-"` // populated at runtime
	Placement [][2]int `json:"-"` // board coordinates, set during placement
	IsDestroyed bool   `json:"-"`
}

// Rack is a server rack (single board cell).
type Rack struct {
	ID          string
	RegionID    string
	Position    [2]int // x, y on board
	Capacity    int    // max pods this rack can hold
	Pods        []*Pod
	IsDestroyed bool
	HitCount    int
}

// Pod is a workload unit running on a rack.
type Pod struct {
	ID        string
	ServiceID string
	RackID    string
	RegionID  string
	Health    int
	MaxHealth int
	Status    PodStatus
	Position  [2]int // inherited from rack
}

// Service routes traffic to healthy pods.
type Service struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Emoji       string       `json:"emoji"`
	Replicas    int          `json:"replicas"`
	PodsPerReplica int       `json:"pods_per_replica"`
	Affinity    AffinityType `json:"affinity"`
	Criticality string       `json:"criticality"`
	CanFailover bool         `json:"can_failover"`
	Pods        []*Pod       `json:"-"` // populated at runtime
	IsHealthy   bool         `json:"-"`
	Latency     int          `json:"-"` // current effective latency
}

// PodStatus tracks the state of a pod (Running, Pending, Terminated)
type PodStatus string

const (
	PodRunning    PodStatus = "Running"
	PodPending    PodStatus = "Pending"
	PodTerminated PodStatus = "Terminated"
)

// AffinityType determines how pods get scheduled across racks.
// This affects what happens when a rack gets destroyed.
type AffinityType string

const (
	// AffinityHard: pods MUST stay on specific racks, can't reschedule
	AffinityHard AffinityType = "hard"
	// AffinitySoft: prefer specific racks but can move if needed
	AffinitySoft AffinityType = "soft"
	// AffinitySpread: actively spread across racks for redundancy
	AffinitySpread AffinityType = "spread"
	// AffinityNone: no preference, schedule anywhere
	AffinityNone AffinityType = "none"
)

// AIStrategy defines how the AI opponent plays
type AIStrategy string

const (
	AIRandom     AIStrategy = "random"     // pure random targeting
	AIHunter     AIStrategy = "hunter"     // hunt neighbors after hit
	AIDefensive  AIStrategy = "defensive"  // protect critical services
	AIAggressive AIStrategy = "aggressive" // target high-value first
)

// WinCondition describes how the game can end
type WinCondition string

const (
	WinKnockout    WinCondition = "knockout"    // all enemy pods terminated
	WinDegradation WinCondition = "degradation" // >50% pods Pending for N turns
	WinCritical    WinCondition = "critical"    // critical service offline
	WinLatency     WinCondition = "latency"     // global latency too high
)

// GameEvent represents a K8s-style event for the sidebar
type GameEvent struct {
	Timestamp   int64  // turn number or ms
	Type        string // "Normal", "Warning", "Error"
	Reason      string // "PodTerminated", "PodRescheduled", etc
	Message     string
	ServiceID   string
	PodID       string
	RegionID    string
}

// CompanyTemplate is the JSON structure for loading companies
type CompanyTemplate struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Emoji       string           `json:"emoji"`
	Description string           `json:"description"`
	Regions     []RegionTemplate `json:"regions"`
	Services    []ServiceTemplate `json:"services"`
	AIStrategy  AIStrategy       `json:"ai_strategy"`
	Difficulty  string           `json:"difficulty"`
}

// RegionTemplate is the JSON structure for regions
type RegionTemplate struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Emoji     string `json:"emoji"`
	Racks     int    `json:"racks"`
	LatencyMs int    `json:"latency_ms"`
}

// ServiceTemplate is the JSON structure for services
type ServiceTemplate struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Emoji          string       `json:"emoji"`
	Replicas       int          `json:"replicas"`
	PodsPerReplica int          `json:"pods_per_replica"`
	Affinity       AffinityType `json:"affinity"`
	Criticality    string       `json:"criticality"`
	CanFailover    bool         `json:"can_failover"`
}
