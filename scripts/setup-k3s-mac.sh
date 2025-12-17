#!/bin/bash
# ClusterShip k3s Server Setup for Mac
# Run this on your M1 MacBook Pro

set -e

echo "ClusterShip k3s Server Setup"
echo "============================"
echo ""

# Check if running on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    echo "ERROR: This script is for macOS only"
    exit 1
fi

# Get Mac's LAN IP
MAC_IP=$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || echo "unknown")
echo "Your Mac's LAN IP: $MAC_IP"
echo ""

# Check if k3s is already installed
if command -v k3s &> /dev/null; then
    echo "k3s is already installed"
    k3s --version
else
    echo "Installing k3s..."
    curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
fi

echo ""
echo "Waiting for k3s to be ready..."
sleep 5

# Check k3s status
sudo k3s kubectl get nodes

echo ""
echo "=============================="
echo "SETUP COMPLETE!"
echo "=============================="
echo ""
echo "Your Mac IP: $MAC_IP"
echo ""
echo "Join token (copy this for Windows if adding as worker):"
echo "------"
sudo cat /var/lib/rancher/k3s/server/node-token
echo ""
echo "------"
echo ""
echo "Kubeconfig for Windows client (copy this):"
echo "------"
sudo cat /etc/rancher/k3s/k3s.yaml | sed "s/127.0.0.1/$MAC_IP/g"
echo "------"
echo ""
echo "Save the kubeconfig above to Windows at:"
echo "  C:\\Users\\<username>\\.kube\\config-clustership"
echo ""
echo "Then on Windows, test with:"
echo "  kubectl --kubeconfig=\$env:USERPROFILE\\.kube\\config-clustership get nodes"
