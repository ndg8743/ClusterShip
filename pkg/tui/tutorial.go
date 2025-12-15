package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TutorialStep represents a single step in the tutorial
type TutorialStep struct {
	Title   string
	Content string
	Visual  string
}

// tutorialSteps defines all tutorial steps with ASCII visuals
var tutorialSteps = []TutorialStep{
	{
		Title:   "Welcome to ClusterShip",
		Content: "Learn Kubernetes concepts through naval warfare!\n\nYou command a fleet of ships (nodes) carrying pods (containers).\nBattle AI companies by attacking their infrastructure.",
		Visual: `
    YOUR FLEET              ENEMY FLEET
   +-----------+           +-----------+
   | # # # # # |    VS     | ? ? ? ? ? |
   | # # # # # |           | ? ? ? ? ? |
   | # # # # # |           | ? ? ? ? ? |
   +-----------+           +-----------+
    (visible)               (hidden)
`,
	},
	{
		Title:   "The Ocean Board",
		Content: "The game board is a 100x100 ocean grid.\nEach cell can contain a rack (part of a ship).\n\nSymbols:\n ~ = Water (empty)\n # = Your ship\n X = Hit (damaged)\n o = Miss\n ! = Destroyed (rack)\n ≡ = Sunk (region)",
		Visual: `
   +---+---+---+---+---+---+---+---+
   | ~ | ~ | ~ | # | # | # | ~ | ~ |
   +---+---+---+---+---+---+---+---+
   | ~ | ~ | ~ | ~ | ~ | ~ | ~ | ~ |
   +---+---+---+---+---+---+---+---+
   | ~ | X | ~ | ~ | ~ | ~ | o | ~ |
   +---+---+---+---+---+---+---+---+
   | ~ | ~ | ~ | ~ | # | # | # | # |
   +---+---+---+---+---+---+---+---+
`,
	},
	{
		Title:   "Companies = Namespaces",
		Content: "Each company represents a K8s namespace.\nCompanies have services (deployments) that run pods.\n\nYou pick your company, then face AI opponents.\nEach has different services and AI strategies.",
		Visual: `
   NETFLIX (You)           AWS (Enemy)
  +------------+          +------------+
  | CDN Edge   |          | CloudFront |
  | Playback   |          | Lambda     |
  | Video DB   |          | RDS        |
  | Auth       |          | Cognito    |
  +------------+          +------------+
      |                        |
      v                        v
   Namespace:              Namespace:
   clustership-netflix     clustership-aws
`,
	},
	{
		Title:   "Ships = Nodes (Regions)",
		Content: "Ships are called 'Regions' - they map to K8s Nodes.\nEach region contains multiple racks.\nRacks hold pods (containers).\n\nIf a region is destroyed, pods may be lost!",
		Visual: `
   REGION: us-east-1
  +----------------------------------+
  |  RACK 1    RACK 2    RACK 3      |
  | +------+  +------+  +------+     |
  | |[pod] |  |[pod] |  |[pod] |     |
  | |[pod] |  |[pod] |  |      |     |
  | +------+  +------+  +------+     |
  +----------------------------------+
       ^          ^          ^
       |          |          |
   4 capacity  4 capacity  4 capacity
`,
	},
	{
		Title:   "Racks = Node Capacity",
		Content: "Each rack can hold up to 4 pods.\nThis simulates node resource capacity.\n\nWhen you attack a cell, you damage pods in that rack.\nDamaged pods may reschedule based on affinity.",
		Visual: `
   RACK DETAIL (Cell 45,23)
  +------------------------+
  | Service: playback-api  |
  |                        |
  | [pod-1] OK   [pod-2] OK|
  | [pod-3] HIT  [    ]    |
  |                        |
  | Capacity: 3/4 pods     |
  +------------------------+
`,
	},
	{
		Title:   "Pods & Services",
		Content: "Services spread pods across your fleet.\nEach service has multiple pod replicas.\n\nA service is 'down' when all its pods are destroyed.\nLose all services = Game Over!",
		Visual: `
   SERVICE: playback-api (replicas: 6)

   Region 1       Region 2       Region 3
  +--------+     +--------+     +--------+
  | [pod]  |     | [pod]  |     | [pod]  |
  | [pod]  |     | [pod]  |     | [pod]  |
  +--------+     +--------+     +--------+

   All pods healthy = Service UP
   All pods dead    = Service DOWN
`,
	},
	{
		Title:   "Pod Affinity Types",
		Content: "Affinity controls where pods can reschedule.\n\n[!] HARD  - Pods stuck in region (can't move)\n[o] SOFT  - Prefers same region, will move if needed\n[~] SPREAD - Distributes across all regions\n[-] NONE  - No preference, goes anywhere",
		Visual: `
   AFFINITY BEHAVIOR ON POD DEATH:

   HARD [!]         SOFT [o]        SPREAD [~]
  +--------+       +--------+       +--------+
  |XXXXXXXX| LOST  |  move  |  OK   | move   | OK
  |  pod   |-----> | nearby |-----> | to     |---->
  |  dies  |       | region |       | empty  |
  +--------+       +--------+       +--------+

   Hard affinity = HIGH RISK (database pods)
   Spread = HIGH AVAILABILITY (stateless)
`,
	},
	{
		Title:   "Attacking",
		Content: "Use arrow keys to move cursor on the map.\nPress Enter or Space to fire at that cell.\n\nHits damage pods in the rack.\nMisses mark water as searched.",
		Visual: `
   ATTACK SEQUENCE:

   1. Select target    2. Fire!         3. Result
  +---+---+---+       +---+---+---+    +---+---+---+
  | ~ | ~ | ~ |       | ~ | ~ | ~ |    | ~ | ~ | ~ |
  +---+---+---+       +---+---+---+    +---+---+---+
  | ~ |[#]| ~ |  -->  | ~ |[*]| ~ | -> | ~ | X | ~ |
  +---+---+---+       +---+---+---+    +---+---+---+
  | ~ | ~ | ~ |       | ~ | ~ | ~ |    | ~ | ~ | ~ |
  +---+---+---+       +---+---+---+    +---+---+---+
    cursor              BOOM!            HIT!
`,
	},
	{
		Title:   "Rescheduling on Damage",
		Content: "When pods die, the scheduler tries to move them.\nThis mimics K8s pod rescheduling on node failure.\n\nGo code: board.go:tryReschedulePod()\nChecks affinity, finds available rack, moves pod.",
		Visual: `
   POD RESCHEDULING:

   Before Attack          After Attack
  +---------+---------+  +---------+---------+
  | Region1 | Region2 |  | Region1 | Region2 |
  |  [pod]  |  [pod]  |  |  [XXX]  |  [pod]  |
  |  [pod]  |         |  |         |  [pod]  | <-- moved!
  +---------+---------+  +---------+---------+

   Pod from Region1 rescheduled to Region2
   (only if affinity allows)
`,
	},
	{
		Title:   "Real K8s Mode",
		Content: "Enable in Settings > Kubernetes to deploy real pods!\n\nWhen enabled:\n- Manifests from templates/k8s/ deploy to cluster\n- Attacks trigger 'kubectl delete pod'\n- Pod events sync back to game\n\nRequires: Kubernetes cluster (kind, minikube, etc)",
		Visual: `
   GAME                    REAL CLUSTER
  +--------+              +-------------+
  | Attack | -----------> | kubectl     |
  | cell   |   sync       | delete pod  |
  +--------+              +-------------+
       ^                        |
       |    watch events        v
       +<---------------  [Pod Terminated]

   templates/k8s/netflix/playback-api.yaml
   --> Deployment + Service in cluster
`,
	},
	{
		Title:   "Real Company Services",
		Content: "Each company runs services that emulate real workloads:\n\nNETFLIX: CDN (nginx), API (python), Video encoding\nAWS: EC2 API, S3 storage (minio), Lambda workers\nGOOGLE: Search API with Redis cache, BigQuery\nMETA: Feed API, Messenger, Instagram backend\nSPOTIFY: Audio CDN, Playback API, Recommendations",
		Visual: `
   REAL SERVICES IN YOUR CLUSTER:

   netflix-cdn-edge       aws-ec2-api
  +----------------+     +----------------+
  | nginx:1.25     |     | python:3.11    |
  | stress-ng      |     | EC2 API sim    |
  | curl (traffic) |     | rds-client     |
  +----------------+     +----------------+

   google-search          spotify-playback
  +----------------+     +----------------+
  | python search  |     | python API     |
  | stress-ng hash |     | redis cache    |
  | redis:7 cache  |     | curl sidecars  |
  +----------------+     +----------------+
`,
	},
	{
		Title:   "How YAMLs Emulate Services",
		Content: "Each manifest creates realistic workloads:\n\n- Main container: Runs actual service (nginx, python API)\n- Stress sidecar: CPU/memory load (stress-ng)\n- Traffic sidecar: Network I/O (curl loops)\n- Cache sidecar: Redis/memory pressure\n\nLabels link pods to game: company, service, tier",
		Visual: `
   DEPLOYMENT STRUCTURE (cdn-edge.yaml):

   +------------------------------------------+
   | Deployment: netflix-cdn-edge             |
   | labels: company=netflix, service=cdn     |
   |                                          |
   |  +------+  +--------+  +---------+       |
   |  | nginx|  |stress- |  | traffic |       |
   |  | :80  |  |ng CPU  |  | curl    |       |
   |  +------+  +--------+  +---------+       |
   |      |                      |            |
   |      v                      v            |
   |   Serves    <---------  Generates        |
   |   content               requests         |
   +------------------------------------------+
`,
	},
	{
		Title:   "Resource Benchmarks",
		Content: "Services use real CPU/memory to stress your cluster:\n\nLight: 100m CPU, 64Mi memory (CDN edges)\nMedium: 300m CPU, 256Mi memory (APIs)\nHeavy: 500m+ CPU, 512Mi memory (databases)\n\nEach pod runs stress-ng for realistic load.\nWatch 'kubectl top pods' during battles!",
		Visual: `
   RESOURCE USAGE BY TIER:

   Edge (nginx,cdn)    API (python,node)    DB (postgres,mysql)
   +----------+        +-----------+        +------------+
   |  100m    |        |   300m    |        |   500m+    |
   |  CPU     |        |   CPU     |        |   CPU      |
   |  64Mi    |        |   256Mi   |        |   512Mi    |
   |  Memory  |        |   Memory  |        |   Memory   |
   +----------+        +-----------+        +------------+
       light               medium               heavy

   stress-ng runs: matrixprod, hash, cache methods
`,
	},
}

// TotalTutorialSteps returns the number of tutorial steps
func TotalTutorialSteps() int {
	return len(tutorialSteps)
}

// updateTutorial handles key input during tutorial
func (m AppModel) updateTutorial(msg tea.KeyMsg) (AppModel, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		// Exit tutorial
		m.state = StateMenu
		m.tutorialStep = 0
		return m, nil

	case "enter", "right", " ", "n":
		// Next step
		if m.tutorialStep < len(tutorialSteps)-1 {
			m.tutorialStep++
		} else {
			// Tutorial complete, return to menu
			m.state = StateMenu
			m.tutorialStep = 0
		}
		return m, nil

	case "left", "backspace", "p":
		// Previous step
		if m.tutorialStep > 0 {
			m.tutorialStep--
		}
		return m, nil

	case "home":
		m.tutorialStep = 0
		return m, nil

	case "end":
		m.tutorialStep = len(tutorialSteps) - 1
		return m, nil
	}

	return m, nil
}

// renderTutorial renders the tutorial view
func (m AppModel) renderTutorial() string {
	if m.tutorialStep < 0 || m.tutorialStep >= len(tutorialSteps) {
		m.tutorialStep = 0
	}

	step := tutorialSteps[m.tutorialStep]

	// Title with step indicator
	stepIndicator := fmt.Sprintf("Step %d/%d", m.tutorialStep+1, len(tutorialSteps))
	title := m.styles.Title.Render(step.Title)
	indicator := m.styles.Muted.Render(stepIndicator)
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", indicator)

	// Visual box with monospace styling
	visualStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Foreground(lipgloss.Color("39"))
	visual := visualStyle.Render(step.Visual)

	// Content text
	content := m.styles.Normal.Render(step.Content)

	// Navigation hints
	var navHint string
	if m.tutorialStep == 0 {
		navHint = "[Enter/Right] Next    [Esc] Exit"
	} else if m.tutorialStep == len(tutorialSteps)-1 {
		navHint = "[Left] Back    [Enter] Finish    [Esc] Exit"
	} else {
		navHint = "[Left] Back    [Enter/Right] Next    [Esc] Exit"
	}
	nav := m.styles.Muted.Render(navHint)

	// Progress bar
	progress := m.renderTutorialProgress()

	// Compose the view
	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		visual,
		"",
		content,
		"",
		progress,
		"",
		nav,
	)

	// Box the whole thing
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(1, 2).
		Width(70)

	boxed := boxStyle.Render(body)

	// Center on screen
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		boxed,
	)
}

// renderTutorialProgress renders a visual progress bar
func (m AppModel) renderTutorialProgress() string {
	total := len(tutorialSteps)
	current := m.tutorialStep + 1

	filled := ""
	empty := ""
	for i := 0; i < total; i++ {
		if i < current {
			filled += "="
		} else {
			empty += "-"
		}
	}

	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return "[" + progressStyle.Render(filled) + emptyStyle.Render(empty) + "]"
}
