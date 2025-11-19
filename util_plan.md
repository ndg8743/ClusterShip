### Goal
Run the board as the control plane, run each match as a short-lived “game” container that drives turns, and run each ship as its own pod that scales its in‑game stats from the hardware it actually gets in the cluster.

### Architecture
- Board (control plane): a long‑lived service that owns truth for placements, health, turns, and exposes WebSocket for ship heartbeats and an HTTP API for attacks.
- Game container (per match): a short job that alternates turns by calling the board’s attack API until the match ends.
- Ship pods (one per ship): lightweight processes that connect to the board and periodically heartbeat. Each ship measures its own CPU allocation and network RTT to the board and derives its size, health, and latency from that.

### How ships communicate with hardware
- Each ship process reads its effective CPU allocation inside the pod (cpuset) and maps that to ship size and health, so pods with more CPUs get “bigger/healthier” ships.
- Each ship measures RTT to the board’s health endpoint and uses that as its per‑message latency, simulating slower/faster links based on actual cluster/network conditions.
- On its first heartbeat the ship includes its chosen size/health; the board accepts those initial values for that ship, assigns a non‑overlapping placement, and then remains authoritative thereafter.

### Kubernetes laIt
- Board: one Deployment with a ClusterIP Service in a control‑plane namespace.
- Match: one namespace (or a `game_id` routed to the board), with:
  - Two ship Deployments (e.g., red and blue). I set different CPU requests/limits if I want asymmetric ships, which then flow into their size/health.
  - One Job for the game container that drives turns by calling the board’s attack API and exits when a winner is decided.

### Security and multi‑game support
- Run separate namespaces per match or include a `game_id` so one board instance can partition many concurrent `GameBoard`s.
- Add simple auth on the attack API so only the game container can fire shots.
- Add liveness/readiness probes on the board; use resource requests/limits on ships and runner; consider NetworkPolicy to restrict access to the board.
- Cap and validate the first‑sight size/health server‑side to prevent out‑of‑range values.

### Libaries
- Go standard libraries already in the repo plus:
  - `k8s.io/utils` (notably `cpuset`) to read the pod’s cpuset and derive available CPU count.
  - Existing `github.com/gorilla/websocket` for ship heartbeats.
- Kubernetes primitives:
  - Deployment and Service for the board.
  - Deployment per ship; Job for the game container.
  - Namespaces for isolation; resource requests/limits; optionally CPUManager static policy if I want pinned CPUs to influence cpusets.
  - ServiceAccount/RBAC and NetworkPolicy for basic security.
- Ir existing code:
  - `pkg/api/server.go` for WebSocket and attack HTTP endpoints.
  - `pkg/game/gameboard.go` to accept client‑provided size/health on first sighting and to keep authoritative state.
  - A small ship entrypoint binary that reads hardware and config from env and heartbeats to the board.
  - A game‑runner entrypoint that alternates turns via the board’s HTTP API.

### Plan
1) Split the demo: promote the board into its own long‑lived service; add an attack HTTP endpoint; leave WebSocket handler as‑is.  
2) Ship process: add a small binary that reads cpuset CPU count and RTT to set size/health/latency, then connects via WebSocket and heartbeats.  
3) Board behavior: on first heartbeat, accept size/health from the ship (with caps and defaults); keep board authoritative afterwards.  
4) Game container: create a job that alternates red/blue turns by calling the board’s attack API until one ship is destroyed.  
5) Containerize and deploy: board Deployment + Service; per‑match namespace with two ship Deployments and one game Job; tune CPU requests/limits to shape ships.  
6) Hardening and scale: add probes, auth, resource policies, and per‑game isolation; optionally run multiple concurrent games by namespace or by `game_id`.  

- Board runs as control plane with WebSocket and attack API.
- Ships are pods; each ship reads cpusets and RTT to derive size/health/latency.
- A game job drives turns and exits on game over.
- I’ll use `k8s.io/utils` (cpuset), gorilla WebSocket, standard Go net/http, and core Kubernetes objects (Deployment, Service, Job, Namespace, RBAC, NetworkPolicy).