package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// InfoContent holds the title and body for info overlays
type InfoContent struct {
	Title string
	Body  string
}

// getInfoForCurrentState returns contextual info based on current game state and view
func (m AppModel) getInfoForCurrentState() InfoContent {
	switch m.state {
	case StateMenu:
		return InfoContent{
			Title: "ClusterShip",
			Body: `Battleship meets Kubernetes!

GAME CONCEPT
Battle AI companies on a shared ocean while learning
Kubernetes concepts through gameplay.

GO CODE vs KUBERNETES
- Game board: Pure Go simulation (pkg/tui/board.go)
- Companies: Go structs from JSON templates
- AI strategies: Go code (pkg/tui/ai.go)

When K8s Mode is enabled (Settings):
- Real pods deploy to your cluster
- Attacks trigger kubectl delete pod
- Pod events sync back to game state

FILES
- pkg/tui/app.go    Main game loop
- pkg/game/         Data structures
- pkg/k8s/          Real K8s integration
- templates/        Company & manifest files`,
		}

	case StateCompanySelect:
		return InfoContent{
			Title: "Company Selection",
			Body: `Each company = K8s namespace worth of services.

GO CODE
- templates/companies/{id}.json defines services
- pkg/game/company.go loads and validates
- Each service has pods, affinity, regions

KUBERNETES MAPPING
- Company -> Namespace
- Service -> Deployment + Service
- Region -> Node concept
- Rack -> Node capacity slot

REAL K8s (when enabled)
- templates/k8s/{company}/*.yaml deployed
- Each YAML creates Deployment + Service
- Labels: app=clustership, company={id}`,
		}

	case StateEnemyCountSelect, StateEnemySelect:
		return InfoContent{
			Title: "Enemy Selection",
			Body: `Choose opponents for multi-company battle.

GAME MECHANICS
- Up to 5 AI enemies supported
- Each AI uses different targeting strategy
- Turn order rotates through all companies

AI STRATEGIES (pkg/tui/ai.go)
- Random: Pure random targeting
- Hunter: Follows up on hits
- Aggressive: Prioritizes vulnerable targets

KUBERNETES PARALLEL
- Multiple namespaces competing
- Like chaos engineering scenarios
- Tests pod failover resilience`,
		}

	case StatePlacement:
		return InfoContent{
			Title: "Fleet Placement",
			Body: `Position your ships (regions) on the ocean.

GO SIMULATION
- board.go:placeFleet() handles placement
- Uses collision detection for valid spots
- Random placement for AI companies

KUBERNETES MAPPING
- Ships = Nodes in your cluster
- Placement = Node provisioning
- Each ship has racks (capacity)

AFFINITY PREVIEW
- Hard [!] pods cannot move if ship hit
- Soft [o] pods prefer same region
- Spread [~] pods distribute across racks
- None [-] pods go anywhere`,
		}

	case StateBattle:
		return m.getBattleViewInfo()

	case StatePodView:
		return InfoContent{
			Title: "Pod View",
			Body: `Deep dive into service health and status.

GO CODE
- game.Service struct holds pods
- board.go tracks pod status per rack
- Damage applies to specific pods

AFFINITY & RESCHEDULING
When pods die:
- Hard: NO rescheduling (critical)
- Soft: Tries same region first
- Spread: Moves to emptiest rack
- None: Goes anywhere available

REAL K8s (when enabled)
- Shows actual pod status from cluster
- pkg/k8s/watcher.go monitors events
- Sync between game state and reality`,
		}

	case StateSettings:
		return InfoContent{
			Title: "Settings",
			Body: `Configure game parameters and K8s integration.

STORED IN
~/.clustership/config.json (pkg/config/)

CATEGORIES
- Board: Ocean dimensions (affects scale)
- Ships: Regions per player
- Pods: Capacity per rack
- Bots: AI difficulty settings
- Timing: Animation speed
- K8s: Real cluster integration

LARGE BOARDS
Boards over 1000x1000 use sparse QuadTree
data structure (pkg/sparse/) for performance.`,
		}

	case StateSettingsBoard:
		return InfoContent{
			Title: "Board Settings",
			Body: `Ocean dimensions affect game scale.

GO CODE
- Board is 2D slice in board.go
- Large boards use pkg/sparse/quadtree.go
- Viewport scrolls over large boards

SIZING
- Small: 100x100 (default)
- Medium: 500x500
- Large: 1000x1000
- Epic: 10000x10000 (sparse mode)

HARDWARE TIERS
Detected hardware limits max board size.
pkg/benchmark/ measures system capability.`,
		}

	case StateSettingsShips:
		return InfoContent{
			Title: "Ship Settings",
			Body: `Ships = Regions in Kubernetes terms.

GO STRUCTS
type Region struct {
    ID    string
    Name  string
    Racks []*Rack
}

KUBERNETES MAPPING
- Region -> Node
- More ships = more nodes
- Each ship has rack capacity

GAME IMPACT
More ships = more targets
Fewer ships = concentrated pods`,
		}

	case StateSettingsPods:
		return InfoContent{
			Title: "Pod Settings",
			Body: `Pods per rack = node capacity simulation.

GO CODE
- game.Rack has Capacity field
- board.go enforces pod limits
- Overfull racks = scheduling failure

KUBERNETES PARALLEL
- Resource limits in YAML
- requests.cpu, limits.memory
- Pod eviction when exceeded

DEFAULTS
- 4 pods per rack typical
- Matches small node capacity`,
		}

	case StateSettingsBots:
		return InfoContent{
			Title: "Bot Settings",
			Body: `AI enemy configuration.

GO CODE
- pkg/tui/ai.go implements strategies
- AIPlayer struct holds state
- pickTarget() selects attack coords

STRATEGIES
- Random: Uniform distribution
- Hunter: Memory-based followup
- Aggressive: Risk assessment

DIFFICULTY
Higher = smarter target selection
More enemies = faster game pace`,
		}

	case StateSettingsTiming:
		return InfoContent{
			Title: "Timing Settings",
			Body: `Animation and turn speed control.

GO CODE
- tickMsg drives game loop
- TurnDelayMs in config
- tea.Tick() for async timing

SETTINGS
- Turn delay: ms between actions
- Animation speed: visual updates

DEMO MODE
Uses timing for auto-play display.
Lower delay = faster gameplay.`,
		}

	case StateSettingsK8s:
		return InfoContent{
			Title: "Kubernetes Settings",
			Body: `Toggle real K8s cluster integration.

WHEN DISABLED (default)
- Pure Go simulation
- No external dependencies
- All affinity simulated in code

WHEN ENABLED
- Connects via kubeconfig
- Creates 'clustership' namespace
- Deploys real workloads from templates/k8s/
- Attacks = kubectl delete pod
- Watches pod events via informer

FILES
- pkg/k8s/client.go    Cluster connection
- pkg/k8s/deployer.go  Manifest deployment
- pkg/k8s/watcher.go   Event monitoring

REQUIREMENTS
- Kubernetes cluster (kind, minikube, etc)
- Valid kubeconfig path`,
		}

	case StateTutorial:
		return InfoContent{
			Title: "Tutorial",
			Body: `Step-by-step gameplay guide.

Use arrow keys or Enter to navigate.
Press Esc to exit tutorial.

This tutorial explains:
- Board and ocean concepts
- Company and service structure
- Pod affinity and rescheduling
- Attack mechanics
- K8s integration mode`,
		}

	case StateGameOver:
		return InfoContent{
			Title: "Game Over",
			Body: `Battle complete!

VICTORY CONDITIONS
All enemy pods destroyed = Win
All your pods destroyed = Lose

KUBERNETES LESSON
- Redundancy matters (spread pods)
- Hard affinity is risky
- Node failure = data loss

NEXT STEPS
- Try different companies
- Enable K8s mode for real pods
- Experiment with affinity types`,
		}

	default:
		return InfoContent{
			Title: "Info",
			Body:  "Press [i] to toggle this overlay.\nPress [Esc] to close.",
		}
	}
}

// getBattleViewInfo returns info specific to the current battle view level
func (m AppModel) getBattleViewInfo() InfoContent {
	switch m.viewLevel {
	case ViewMap:
		return InfoContent{
			Title: "Ocean Map (View 1)",
			Body: `Board = API Server simulation.

GO CODE
- board.go manages 2D grid state
- Each cell = potential rack location
- Viewport scrolls over large boards

SYMBOLS
~ Water (empty cell)
# Ship (your region)
X Hit (damaged pod)
o Miss (wasted attack)
! Destroyed (region lost)

ATTACKS
Click or Enter to fire at cursor.
Game calls board.AttackMulti() which:
1. Checks cell contents
2. Applies damage to pods
3. Triggers rescheduling if needed

REAL K8s (when enabled)
Attacks also call kubectl delete pod
via pkg/k8s/client.go DeletePod()`,
		}

	case ViewShip:
		return InfoContent{
			Title: "Regions (View 2)",
			Body: `Regions = Kubernetes Nodes.

GO STRUCT (pkg/game/company.go)
type Region struct {
    ID    string
    Name  string
    Racks []*Rack
}

KUBERNETES MAPPING
- Region -> Node
- Racks -> Node capacity slots
- Pods scheduled to racks

DISPLAY
Shows all regions for selected company.
Health bar = remaining pod capacity.

NAVIGATION
Arrow keys to select region.
Enter or [3] to drill into rack view.`,
		}

	case ViewRack:
		return InfoContent{
			Title: "Rack Detail (View 3)",
			Body: `Rack = board cell with up to 4 pods.

GO CODE
- game.Rack struct holds pods
- board.go tracks rack at each cell
- Capacity enforced during scheduling

POD PLACEMENT (board.go:findRackForPod)
1. Checks affinity requirements
2. Finds rack with capacity
3. Places pod or fails

AFFINITY TYPES
[!] Hard - requiredDuringScheduling
[o] Soft - preferredDuringScheduling
[~] Spread - podAntiAffinity
[-] None - no constraints

REAL K8s (when enabled)
pkg/k8s/deployer.go creates Deployments
with resource limits from YAML templates.`,
		}

	case ViewYAML:
		return InfoContent{
			Title: "K8s Manifests (View 4)",
			Body: `Shows actual YAML from templates.

FILES
templates/k8s/{company}/*.yaml

STRUCTURE
- Deployment: pod spec, replicas
- Service: networking, ports
- Labels: app, company, service

LABELS (required)
app: clustership
company: {company-id}
service: {service-id}

WHEN K8s ENABLED
These exact manifests deploy to cluster.
Changes here = changes in cluster.

WORKLOAD TYPES
- stress-ng for CPU/memory load
- Redis for caching simulation
- Postgres for database workload
- Python http.server for API mock`,
		}

	case ViewRackLayout:
		return InfoContent{
			Title: "Pod Distribution (View 5)",
			Body: `Visual of affinity placement results.

GO CODE (board.go)
- findRackForPod() places pods
- tryReschedulePod() moves on death
- Affinity rules checked each time

VISUAL LEGEND
[!] Hard affinity - stuck in region
[o] Soft affinity - prefers region
[~] Spread - distributed across racks
[-] None - random placement

RESCHEDULING BEHAVIOR
When pod dies:
- Hard: Pod LOST (can't move)
- Soft: Try same region, else anywhere
- Spread: Pick emptiest rack
- None: Any rack with capacity

K8s PARALLEL
Real K8s affinity in pod spec:
- nodeAffinity
- podAffinity
- podAntiAffinity
- topologySpreadConstraints`,
		}

	default:
		return InfoContent{
			Title: "Battle View",
			Body: `Use keys 1-5 to switch views:
[1] Ocean Map - overview
[2] Regions - ship list
[3] Rack - pod details
[4] YAML - manifest view
[5] Layout - pod distribution

Press [i] to see view-specific info.`,
		}
	}
}

// renderInfoOverlay renders the info overlay on top of the game view
func (m AppModel) renderInfoOverlay(underneath string) string {
	info := m.getInfoForCurrentState()

	// Build info box content
	title := m.styles.Title.Render("[i] " + info.Title)
	body := m.styles.Normal.Render(info.Body)
	hint := m.styles.Muted.Render("\n[i] or [Esc] to close")

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", body, hint)
	infoBox := m.styles.InfoBox.Render(content)

	// Center overlay on screen
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		infoBox,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("238")),
	)
}
