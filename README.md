# ClusterShip

A terminal-based battleship game that teaches Kubernetes concepts through gameplay. Battle AI companies (Netflix, AWS, Google) on a shared ocean while learning about pods, services, affinity, and rescheduling. Optionally deploy real K8s workloads to your cluster.

```mermaid
flowchart LR
    subgraph Game["ClusterShip Game"]
        Player["You"]
        AI1["Netflix AI"]
        AI2["AWS AI"]
    end

    subgraph Ocean["100x100 Shared Ocean"]
        Board["Board State"]
    end

    Player -->|attack| Board
    AI1 -->|attack| Board
    AI2 -->|attack| Board
    Board -->|damage| Player
    Board -->|damage| AI1
    Board -->|damage| AI2
```

## Quick Start

```bash
# Run the game
go run ./cmd/clustership

# Or build and run
go build -o clustership ./cmd/clustership
./clustership
```

## Game Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  CLUSTERSHIP - Player vs Netflix vs AWS           Turn: 42  [VIEW 1/5]     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  100x100 SHARED OCEAN                    SERVICE STATUS                     │
│  ┌─────────────────────────────┐         [!]Origin API  [████░] 80%        │
│  │ . . . . X . . . . .        │         [~]CDN Edge    [█████] 100%        │
│  │ . . # # # # # . . .        │         [o]Playback    [███░░] 60%         │
│  │ . . . . . . . . . .        │         [-]Encoding    [██░░░] 40%         │
│  │ . . . . . O . . . .  <cursor                                            │
│  │ . . . . . . . . . .        │         K8S EVENTS                         │
│  └─────────────────────────────┘         [+] Pod cdn-edge rescheduled      │
│                                          [!] Pod database pending          │
│  [1-5] view  [arrows] move  [enter] fire  [q] quit                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Features

- **Multi-company battles**: Player vs 1-5 AI opponents (Netflix, AWS, Google, etc.)
- **Shared ocean**: All fleets on one 100x100 board, no overlap
- **5 view levels**: Map, Ship, Rack, YAML, Rack Layout
- **Pod affinity simulation**: Hard, Soft, Spread, None - affects rescheduling
- **Real K8s integration**: Deploy actual pods to your cluster
- **Demo mode**: Watch AI vs AI battles
- **Configurable**: Board size, ships, racks, pods, turn delay

## Controls

| Key | Action |
|-----|--------|
| Arrow keys / WASD | Move cursor |
| Enter / Space | Fire at cursor position |
| 1-5 | Change view level |
| C | Cycle service display (your services vs enemy) |
| D | Toggle debug mode (see all ships) |
| Q | Quit / Back to menu |

## View Levels

| Key | View | Description |
|-----|------|-------------|
| 1 | Map | Overview of the ocean with ships and attacks |
| 2 | Ship | List of your regions (ships) with health status |
| 3 | Rack | Drill into racks within a ship |
| 4 | YAML | K8s manifests (fog of war for enemies) |
| 5 | Rack Layout | Visual grid showing pod distribution per rack |

---

## Game Flow

```mermaid
stateDiagram-v2
    [*] --> Menu

    Menu --> CompanySelect: New Game
    Menu --> Demo: Demo Mode
    Menu --> Settings: Configure
    Menu --> [*]: Quit

    CompanySelect --> EnemySelect: Pick your company
    EnemySelect --> Placement: Pick 1-5 enemies
    Placement --> Battle: Ships placed

    Demo --> Battle: Auto-play

    Battle --> Battle: Take turns
    Battle --> GameOver: All enemies defeated
    Battle --> GameOver: You're defeated

    GameOver --> Menu: Play again

    Settings --> Menu: Save & back
```

---

## Kubernetes Architecture Mapping

ClusterShip mirrors real Kubernetes architecture. Every game component has a direct K8s equivalent.

### System Architecture

```mermaid
flowchart TB
    subgraph ControlPlane["Control Plane"]
        Board["Board<br/><i>API Server + etcd</i>"]
        Scheduler["placeFleet()<br/><i>kube-scheduler</i>"]
    end

    subgraph DataPlane["Data Plane"]
        subgraph Company1["Netflix Fleet"]
            Region1["Region: us-east-1<br/><i>Node</i>"]
            Region2["Region: eu-west-1<br/><i>Node</i>"]
        end

        subgraph Company2["AWS Fleet"]
            Region3["Region: virginia<br/><i>Node</i>"]
            Region4["Region: oregon<br/><i>Node</i>"]
        end
    end

    subgraph Controllers["Controllers"]
        AI1["Netflix AI<br/><i>Operator</i>"]
        AI2["AWS AI<br/><i>Operator</i>"]
        Player["Player<br/><i>kubectl</i>"]
    end

    Board <--> Region1
    Board <--> Region2
    Board <--> Region3
    Board <--> Region4

    Scheduler --> Region1
    Scheduler --> Region2
    Scheduler --> Region3
    Scheduler --> Region4

    AI1 -->|attack| Board
    AI2 -->|attack| Board
    Player -->|attack| Board
```

### Component Mapping

| ClusterShip | Kubernetes | Role |
|-------------|------------|------|
| **Board** | API Server + etcd | Single source of truth, owns all game state |
| **Company** | Namespace | Isolated group of resources |
| **Region (Ship)** | Node | Physical machine / data center |
| **Rack (Cell)** | Node capacity | Individual server slot |
| **Pod** | Pod | Workload unit that can be killed/rescheduled |
| **Service** | Service + Deployment | Logical grouping with desired replica count |
| **AI Player** | Controller/Operator | Watches state, makes decisions, takes actions |
| **Attack** | kubectl delete pod | Destroys workloads |
| **Rescheduling** | Pod eviction/recreation | Moves pods based on affinity rules |

### Hierarchy Diagram

```mermaid
flowchart TB
    subgraph Company["Company (Namespace)"]
        subgraph Service1["Service: cdn-edge"]
            Pod1["Pod 1"]
            Pod2["Pod 2"]
            Pod3["Pod 3"]
        end

        subgraph Service2["Service: database"]
            Pod4["Pod 4"]
        end

        subgraph Region1["Region: us-east-1 (Node)"]
            subgraph Rack1["Rack 0"]
                R1P1["Pod"]
                R1P2["Pod"]
            end
            subgraph Rack2["Rack 1"]
                R2P1["Pod"]
            end
            subgraph Rack3["Rack 2"]
                R3P1["Pod"]
            end
        end
    end

    Pod1 -.-> R1P1
    Pod2 -.-> R2P1
    Pod3 -.-> R3P1
    Pod4 -.-> R1P2
```

### Affinity Types

```mermaid
flowchart LR
    subgraph Hard["[!] Hard Affinity"]
        H1["Pod killed"]
        H2["Cannot reschedule"]
        H3["Status: Pending"]
        H1 --> H2 --> H3
    end

    subgraph Spread["[~] Spread Affinity"]
        S1["Pod killed"]
        S2["Find least-loaded rack"]
        S3["Reschedule there"]
        S1 --> S2 --> S3
    end

    subgraph None["[-] No Affinity"]
        N1["Pod killed"]
        N2["Find any rack"]
        N3["Reschedule randomly"]
        N1 --> N2 --> N3
    end
```

| Affinity | Icon | Rescheduling | K8s Equivalent | Example |
|----------|------|--------------|----------------|---------|
| **Hard** | [!] | NO | requiredDuringScheduling | Primary database - can't move |
| **Soft** | [o] | Yes, prefers same region | preferredDuringScheduling | API servers |
| **Spread** | [~] | Yes, spreads across racks | podAntiAffinity | CDN edge nodes |
| **None** | [-] | Yes, anywhere | No constraints | Encoding workers |

---

## Attack Flow

When you fire at a cell, here's what happens:

```mermaid
sequenceDiagram
    participant P as Player
    participant B as Board
    participant R as Rack
    participant Pod as Pod
    participant Svc as Service

    P->>B: Attack (x, y)
    B->>B: Find cell owner

    alt Cell is empty
        B-->>P: Miss!
    else Cell has enemy rack
        B->>R: Apply damage
        R->>Pod: Reduce health

        alt Pod health > 0
            Pod-->>B: Pod damaged
            B-->>P: Hit!
        else Pod health <= 0
            Pod->>Svc: Check affinity

            alt Hard affinity
                Svc-->>Pod: Cannot reschedule
                Pod-->>B: Pod Pending
            else Soft/Spread/None
                Svc->>R: Find new rack
                R-->>Pod: Reschedule
                Pod-->>B: Pod moved
            end

            B-->>P: Hit! Pod killed
        end
    end
```

### Pod Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Scheduled: Game starts

    Scheduled --> Running: Pod placed on rack

    Running --> Running: Taking damage
    Running --> Terminated: Health reaches 0

    Terminated --> Rescheduling: Can failover?
    Terminated --> Pending: Hard affinity

    Rescheduling --> Running: New rack found
    Rescheduling --> Pending: No capacity

    Pending --> [*]: Game over for this pod
```

---

## AI Controller Pattern

AI players implement the Kubernetes controller reconciliation loop:

```mermaid
flowchart LR
    subgraph Loop["Reconciliation Loop"]
        Watch["Watch<br/>Get board state"]
        Decide["Decide<br/>Pick target"]
        Act["Act<br/>Fire attack"]
        Check["Check<br/>Enemy alive?"]
    end

    Watch --> Decide
    Decide --> Act
    Act --> Check
    Check -->|Yes| Watch
    Check -->|No| Done["Victory!"]
```

### AI Strategies

```mermaid
flowchart TB
    subgraph Random["AIRandom"]
        R1["Pick random cell"]
        R2["Fire"]
        R1 --> R2
    end

    subgraph Hunter["AIHunter"]
        H1["Got a hit?"]
        H2["Hunt neighbors"]
        H3["Random cell"]
        H1 -->|Yes| H2
        H1 -->|No| H3
    end

    subgraph Aggressive["AIAggressive"]
        A1["KNN analysis"]
        A2["Find high-probability area"]
        A3["Target cluster"]
        A1 --> A2 --> A3
    end
```

---

## Real Kubernetes Integration

When `EnableRealK8s=true`, the game deploys actual K8s workloads:

```mermaid
sequenceDiagram
    participant Game as ClusterShip
    participant K8s as Kubernetes API
    participant Pods as Real Pods

    Note over Game,Pods: Game Start
    Game->>K8s: Create namespace
    Game->>K8s: Apply manifests
    K8s->>Pods: Schedule pods
    Pods-->>K8s: Running
    K8s-->>Game: Deployed

    Note over Game,Pods: During Battle
    Game->>Game: Player attacks
    Game->>K8s: Delete deployment
    K8s->>Pods: Terminate
    Pods-->>K8s: Deleted
    K8s-->>Game: Pod event

    Note over Game,Pods: Game End
    Game->>K8s: Delete namespace
    K8s->>Pods: Cleanup all
```

### What Happens

| Game Event | K8s Action |
|------------|------------|
| Start game | `kubectl apply` all company manifests to `clustership` namespace |
| Attack kills pod | `kubectl delete` matching deployment/pods |
| Poll events | Watch API streams pod Added/Modified/Deleted events |
| End game | `kubectl delete namespace clustership` |

### Real Workloads

The templates in `templates/k8s/` are **real K8s manifests** that run actual containers:

```mermaid
flowchart TB
    subgraph Netflix["Netflix Services"]
        CDN["cdn-edge<br/>nginx + curl"]
        API["origin-api<br/>python HTTP"]
        DB["database<br/>postgres"]
        Play["playback<br/>node.js"]
        Enc["encoding<br/>python workers"]

        CDN --> API
        API --> DB
        Play --> CDN
        Enc --> API
    end

    subgraph AWS["AWS Services"]
        CF["cloudfront-cdn<br/>nginx"]
        EC2["ec2-api<br/>python"]
        S3["s3-storage<br/>minio"]
        RDS["rds-database<br/>mysql"]
        Lambda["lambda-worker<br/>python"]

        CF --> EC2
        CF --> S3
        EC2 --> RDS
        Lambda --> EC2
    end
```

Each service uses **250-500m CPU** and **256-512Mi memory** with inter-service traffic via curl sidecars.

### Setup

```bash
# Create a local cluster
kind create cluster

# Or enable Docker Desktop Kubernetes

# Enable in game settings
# Settings -> Kubernetes -> Toggle Real K8s: ON

# Start game - watch pods deploy
kubectl get pods -n clustership -w
```

### Example Session

```bash
# Start game with Netflix as enemy
# Game deploys:
kubectl get deployments -n clustership
# NAME                    READY   UP-TO-DATE   AVAILABLE
# netflix-cdn-edge        3/3     3            3
# netflix-playback        2/2     2            2
# netflix-origin-api      2/2     2            2
# netflix-database        1/1     1            1
# netflix-encoding        3/3     3            3

# Attack kills a pod
# Game runs: kubectl delete deployment netflix-cdn-edge

# End game cleans up
kubectl get ns clustership
# namespace "clustership" deleted
```

---

## Configuration

Settings are stored in `~/.clustership/config.json`:

```json
{
  "boardWidth": 100,
  "boardHeight": 100,
  "shipsPerPlayer": 5,
  "racksPerShip": 3,
  "podsPerRack": 3,
  "turnDelayMs": 200,
  "enableRealK8s": false,
  "k8sNamespace": "clustership",
  "kubeconfig": "~/.kube/config"
}
```

Access via in-game Settings menu.

---

## Project Structure

```mermaid
flowchart TB
    subgraph CMD["cmd/"]
        Main["clustership/<br/>main.go"]
    end

    subgraph PKG["pkg/"]
        subgraph TUI["tui/"]
            App["app.go<br/>Game loop"]
            Board["board.go<br/>State & attacks"]
            AI["ai.go<br/>AI strategies"]
            Styles["styles.go<br/>Lipgloss"]
        end

        subgraph Game["game/"]
            Company["company.go<br/>Data structures"]
            Loader["loader.go<br/>Template loading"]
        end

        subgraph K8S["k8s/"]
            Client["client.go<br/>K8s client"]
            Deployer["deployer.go<br/>Apply/delete"]
            Watcher["watcher.go<br/>Pod events"]
        end

        Config["config/<br/>Settings"]
    end

    subgraph Templates["templates/"]
        Companies["companies/<br/>JSON definitions"]
        K8sManifests["k8s/<br/>YAML manifests"]
    end

    Main --> App
    App --> Board
    App --> AI
    App --> Game
    App --> K8S
    App --> Config
    Loader --> Companies
    Deployer --> K8sManifests
```

```
clustership/
├── cmd/clustership/          # Main entry point
├── pkg/
│   ├── tui/                  # Terminal UI (Bubble Tea)
│   │   ├── app.go            # Main game loop, state machine
│   │   ├── board.go          # Board state, attacks, scheduling
│   │   ├── ai.go             # AI targeting strategies
│   │   └── styles.go         # Lipgloss styling
│   ├── game/                 # Game data structures
│   │   ├── company.go        # Company, Region, Rack, Pod, Service
│   │   ├── loader.go         # Load company templates
│   │   └── events.go         # K8s-style events
│   ├── k8s/                  # Real K8s integration
│   │   ├── client.go         # K8s client wrapper
│   │   ├── deployer.go       # Deploy/delete manifests
│   │   ├── loader.go         # Parse YAML manifests
│   │   ├── watcher.go        # Watch pod events
│   │   └── types.go          # K8s types
│   └── config/               # Configuration
│       └── config.go         # Load/save settings
├── templates/
│   ├── companies/            # Company definitions (JSON)
│   │   ├── netflix.json
│   │   ├── aws.json
│   │   └── ...
│   └── k8s/                  # Real K8s manifests (YAML)
│       ├── netflix/
│       │   ├── cdn-edge.yaml
│       │   ├── database.yaml
│       │   └── ...
│       └── aws/
│           ├── ec2-api.yaml
│           └── ...
└── README.md
```

---

## Build & Run

```bash
# Build
go build -o clustership ./cmd/clustership

# Run
./clustership

# Run with Go directly
go run ./cmd/clustership

# Build all
go build ./...

# Test
go test ./...
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling
- [client-go](https://github.com/kubernetes/client-go) - K8s client (optional)

---

## Learning Kubernetes

ClusterShip teaches these K8s concepts through gameplay:

```mermaid
mindmap
  root((K8s Concepts))
    Workloads
      Pods
      Deployments
      Replicas
    Scheduling
      Node selection
      Affinity rules
      Resource contention
    Resilience
      Pod rescheduling
      Node failure
      Health checks
    Operations
      kubectl commands
      Watch events
      Namespace isolation
```

| Concept | How You Learn It |
|---------|------------------|
| **Pods** | Each unit on a rack is a pod that can be killed |
| **Deployments** | Services maintain desired replica count |
| **Scheduling** | Ships are placed avoiding overlap (resource contention) |
| **Affinity** | Hard affinity pods can't reschedule when hit |
| **Anti-affinity** | Spread affinity distributes pods across racks |
| **Node failure** | Destroying a rack triggers pod rescheduling |
| **Service health** | Health bars show running vs total pods |
| **Events** | K8s-style event log shows pod lifecycle |
| **Namespaces** | Each company is isolated like a namespace |
| **kubectl** | Real K8s mode uses actual kubectl operations |

---

## License

MIT
