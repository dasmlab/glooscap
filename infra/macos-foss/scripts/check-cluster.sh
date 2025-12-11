#!/usr/bin/env bash
# check-cluster.sh
# Shows where the cluster is running and its status

set -euo pipefail

echo "=== Cluster Status ==="
echo ""

# Check kubectl
if command -v kubectl &> /dev/null; then
    echo "kubectl is installed"
    if kubectl cluster-info &> /dev/null 2>&1; then
        echo "✓ kubectl can connect to cluster"
        echo ""
        echo "Cluster info:"
        kubectl cluster-info
        echo ""
        echo "Nodes:"
        kubectl get nodes
        echo ""
        echo "Current context:"
        kubectl config current-context
    else
        echo "✗ kubectl cannot connect to cluster"
    fi
else
    echo "✗ kubectl not found"
fi

echo ""
echo "=== k3d Status ==="
echo ""

# Check k3d
if command -v k3d &> /dev/null; then
    echo "k3d is installed"
    echo ""
    echo "k3d clusters:"
    k3d cluster list 2>&1 || echo "k3d cluster list failed"
else
    echo "✗ k3d not found"
fi

echo ""
echo "=== Docker Status ==="
echo ""

# Check Docker
if command -v docker &> /dev/null; then
    echo "docker CLI is installed"
    if docker ps &> /dev/null 2>&1; then
        echo "✓ docker ps works"
        echo ""
        echo "k3d containers:"
        docker ps --filter "name=k3d" 2>&1 || echo "No k3d containers found"
    else
        echo "✗ docker ps failed (Docker may not be running or accessible)"
    fi
else
    echo "✗ docker CLI not found"
fi

echo ""
echo "=== Where is the cluster? ==="
echo ""
echo "k3d runs k3s inside Docker containers."
echo "If you see k3d containers above, the cluster is in those Docker containers."
echo "On macOS, Docker runs in a VM (Docker Desktop) or via Docker daemon."
echo ""
echo "To see all k3d containers:"
echo "  docker ps --filter 'name=k3d'"
echo ""
echo "To see k3d cluster details:"
echo "  k3d cluster list"
echo "  kubectl cluster-info"

