# Kubernetes Deployment

Run each battleship ship as its own pod, with a central server pod.

## Build Images

```bash
# Build server image
docker build -f Dockerfile.server -t clustership-server:latest .

# Build node image  
docker build -f Dockerfile.node -t clustership-node:latest .
```

## Deploy

```bash
# Deploy server first
kubectl apply -f k8s/server-deployment.yaml

# Wait for server to be ready
kubectl wait --for=condition=available --timeout=60s deployment/clustership-server

# Deploy ship nodes
kubectl apply -f k8s/node-deployment.yaml

# Watch pods
kubectl get pods -w

# View server logs (has the display)
kubectl logs -f deployment/clustership-server

# View a ship node logs
kubectl logs -f deployment/red-ship
kubectl logs -f deployment/blue-ship
```

## Port Forward to View Display

```bash
# Forward server port to view the ASCII display
kubectl port-forward deployment/clustership-server 8080:8080

# Then in another terminal, you can curl the health endpoint
curl http://localhost:8080/healthz
```

## Scale Ships

```bash
# Add more ships by scaling deployments
kubectl scale deployment red-ship --replicas=2
kubectl scale deployment blue-ship --replicas=2
```

## Cleanup

```bash
kubectl delete -f k8s/node-deployment.yaml
kubectl delete -f k8s/server-deployment.yaml
```

