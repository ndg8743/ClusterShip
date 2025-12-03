# ClusterShip

## Goal
Run battleship game as distributed Kubernetes workload:
- **Board**: Long-lived control plane service (Deployment)
- **Game containers**: Short-lived Jobs that drive turn-based battles
- **Ship pods**: One Deployment per ship, scaling stats from actual hardware allocation

## Architecture

### Components
- **Board (control plane)**
  - Long-lived service owning game state (placements, health, turns)
  - WebSocket endpoint for ship heartbeats
  - HTTP API for attack commands
  - Source: [`pkg/api/server.go`](pkg/api/server.go), [`pkg/game/gameboard.go`](pkg/game/gameboard.go)

- **Game container (per match)**
  - Short-lived Job that alternates turns via board's attack API
  - Exits when match ends (winner determined)
  - Source: [`pkg/game/battle.go`](pkg/game/battle.go) - `BattleCoordinator`

- **Ship pods (one per ship)**
  - Lightweight processes connecting to board via WebSocket
  - Measure CPU allocation (cpuset) → maps to ship size/health
  - Measure RTT to board → maps to per-message latency
  - Source: [`pkg/game/node.go`](pkg/game/node.go) - `BattleshipNode`

## Hardware Integration

### CPU Allocation → Ship Stats
- Read pod's effective CPU allocation via [`k8s.io/utils/cpuset`](https://pkg.go.dev/k8s.io/utils/cpuset)
- Map CPU count to ship size/health (more CPUs = bigger/healthier ships)
- First heartbeat includes chosen size/health; board accepts and assigns placement
- Board remains authoritative for health after initial registration

### Network RTT → Latency
- Measure RTT to board's `/healthz` endpoint
- Use measured latency as per-message delay
- Simulates slower/faster links based on actual cluster network conditions

## Kubernetes Layout

### Board Deployment
- **Type**: Deployment with ClusterIP Service
- **Namespace**: `clustership-control` (control-plane namespace)
- **Endpoints**:
  - `:8080/ws/battleship` - WebSocket for ship heartbeats
  - `:8080/healthz` - Health check endpoint
  - `:8080/attack` - HTTP API for game container attacks (to be added)

### Per-Match Namespace
- **Isolation**: One namespace per match (or `game_id` routing)
- **Components**:
  - Two ship Deployments (red/blue teams)
    - Different CPU requests/limits for asymmetric ships
    - CPU allocation flows into size/health stats
  - One Job for game container
    - Drives turns by calling board's attack API
    - Exits when winner determined

## Security & Multi-Game Support

### Isolation
- Separate namespaces per match OR `game_id` parameter for board partitioning
- NetworkPolicy to restrict ship → board access
- ServiceAccount/RBAC for pod permissions

### API Security
- Add authentication on attack API (only game container can fire shots)
- Cap and validate initial size/health server-side (prevent out-of-range values)

### Reliability
- Liveness/readiness probes on board Deployment
- Resource requests/limits on ships and game runner
- Optional: CPUManager static policy for pinned CPUs → cpuset influence

## Libraries & Dependencies

### Go Packages
- **WebSocket**: [`github.com/gorilla/websocket`](https://pkg.go.dev/github.com/gorilla/websocket) - Already in use
- **Kubernetes utils**: [`k8s.io/utils/cpuset`](https://pkg.go.dev/k8s.io/utils/cpuset) - For CPU allocation reading
- **Standard library**: `net/http`, `context`, `sync` - Already in use

### Kubernetes Primitives
- **Deployment**: Board service, ship pods
- **Service**: ClusterIP for board
- **Job**: Game container runner
- **Namespace**: Per-match isolation
- **Resource requests/limits**: CPU allocation control
- **NetworkPolicy**: Network isolation
- **ServiceAccount/RBAC**: Security

## Implementation Plan

1. **Split board into service**
   - Promote board to long-lived Deployment
   - Add HTTP attack endpoint (`POST /attack`)
   - Keep WebSocket handler as-is
   - Files: [`pkg/api/server.go`](pkg/api/server.go)

2. **Ship process binary**
   - Create ship entrypoint that:
     - Reads cpuset CPU count via `k8s.io/utils/cpuset`
     - Measures RTT to board `/healthz`
     - Sets size/health/latency from hardware
     - Connects via WebSocket and heartbeats
   - Files: [`pkg/game/node.go`](pkg/game/node.go) - extend `BattleshipNode`

3. **Board first-sight behavior**
   - Accept size/health from ship on first heartbeat
   - Apply caps and defaults server-side
   - Board owns health/placement after registration
   - Files: [`pkg/game/gameboard.go`](pkg/game/gameboard.go) - `HandleNodeUpdate`

4. **Game container Job**
   - Create Job that:
     - Alternates red/blue turns via board's attack API
     - Uses existing `BattleCoordinator` logic
     - Exits when `AliveCount() <= 1`
   - Files: [`pkg/game/battle.go`](pkg/game/battle.go)

5. **Containerize & deploy**
   - Build Docker images for board, ship, game-runner
   - Create Kubernetes manifests:
     - Board Deployment + Service
     - Per-match namespace with 2 ship Deployments + 1 game Job
   - Tune CPU requests/limits to shape ship characteristics

6. **Hardening & scale**
   - Add probes (liveness/readiness)
   - Add auth to attack API
   - Add resource policies
   - Support multiple concurrent games (namespace or `game_id`)

## References

- **Kubernetes Deployments**: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
- **Kubernetes Jobs**: https://kubernetes.io/docs/concepts/workloads/controllers/job/
- **Kubernetes Services**: https://kubernetes.io/docs/concepts/services-networking/service/
- **NetworkPolicy**: https://kubernetes.io/docs/concepts/services-networking/network-policies/
- **CPU Manager**: https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/
- **Go WebSocket**: https://pkg.go.dev/github.com/gorilla/websocket
- **K8s Utils**: https://pkg.go.dev/k8s.io/utils
