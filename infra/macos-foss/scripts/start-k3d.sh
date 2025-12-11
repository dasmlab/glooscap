#!/usr/bin/env bash
# start-k3d.sh
# Starts a k3d cluster (k3s in Docker containers)
# This script relies entirely on k3d - no Docker checks needed

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kubectl can already connect - if so, we're done!
if kubectl cluster-info &> /dev/null 2>&1; then
    log_success "Cluster is already accessible via kubectl"
    log_info "Skipping k3d management (cluster running in different Docker context)"
    
    # Show cluster status via kubectl only
    echo ""
    log_info "Cluster information:"
    kubectl cluster-info
    echo ""
    kubectl get nodes 2>/dev/null || log_warn "Nodes not yet available"
    echo ""
    log_success "k3d cluster is ready!"
    exit 0
fi

# Cluster not accessible, need to create/start it
# Check if k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found. Installing..."
    brew install k3d
    log_success "k3d installed"
fi

CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

# Configure DOCKER_HOST for Podman if needed
if command -v podman &> /dev/null; then
    PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
    if [ -n "${PODMAN_SOCKET}" ] && [ -z "${DOCKER_HOST:-}" ]; then
        export DOCKER_HOST="unix://${PODMAN_SOCKET}"
        log_info "Configured DOCKER_HOST to use Podman: ${DOCKER_HOST}"
    fi
fi

# Try to create/start cluster - k3d will handle Docker/Podman errors
log_info "Cluster not accessible via kubectl, attempting to create/start with k3d..."

# Try to list clusters first (to see if we can access Docker/Podman)
if k3d cluster list &> /dev/null 2>&1; then
    # k3d can see Docker, try to manage cluster
    if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}"; then
        if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}.*running"; then
            log_success "k3d cluster '${CLUSTER_NAME}' is running"
        else
            log_info "Cluster '${CLUSTER_NAME}' exists but is not running. Starting it..."
            k3d cluster start "${CLUSTER_NAME}" || {
                log_error "Failed to start cluster"
                log_info "Check if Docker is running: docker ps"
                exit 1
            }
            log_success "Cluster started"
        fi
    else
        log_info "Creating k3d cluster '${CLUSTER_NAME}'..."
        k3d cluster create "${CLUSTER_NAME}" \
            --api-port 6443 \
            --port "8080:80@loadbalancer" \
            --port "8443:443@loadbalancer" \
            --port "3000:3000@loadbalancer" \
            --agents 1 \
            --k3s-arg "--disable=traefik@server:0" \
            --k3s-arg "--disable=servicelb@server:0" 2>&1 | tee /tmp/k3d-create.log || {
            log_error "Failed to create k3d cluster"
            if grep -q "Cannot connect to the Docker daemon" /tmp/k3d-create.log 2>/dev/null; then
                log_error "Docker daemon is not accessible"
                log_info ""
                log_info "To fix this:"
                log_info "  1. Start Docker Desktop (or Docker daemon)"
                log_info "  2. Wait for Docker to be ready"
                log_info "  3. Run this script again"
                log_info ""
                log_info "Or check Docker status: docker ps"
            fi
            exit 1
        }
        log_success "k3d cluster created successfully!"
    fi
else
    # k3d cluster list failed - Docker not accessible
    log_error "k3d cannot access Docker daemon"
    log_info ""
    log_info "k3d needs Docker to be running to create clusters."
    log_info ""
    log_info "To fix this:"
    log_info "  1. Start Docker Desktop (or Docker daemon)"
    log_info "  2. Wait for Docker to be ready"
    log_info "  3. Run this script again: ./scripts/start-k3d.sh"
    log_info ""
    log_info "Check Docker status: docker ps"
    exit 1
fi

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
MAX_WAIT=120
WAIT_COUNT=0

while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    if kubectl cluster-info &> /dev/null 2>&1; then
        break
    fi
    sleep 2
    WAIT_COUNT=$((WAIT_COUNT + 2))
    echo -n "."
done
echo ""

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    log_warn "Cluster may not be fully ready yet"
    log_info "Check status: kubectl cluster-info"
else
    log_success "Cluster is ready!"
fi

# Show cluster info via kubectl (don't use k3d cluster list)
echo ""
log_info "Cluster information:"
kubectl cluster-info 2>/dev/null || log_warn "kubectl not yet configured"
kubectl get nodes 2>/dev/null || log_warn "Nodes not yet available"

echo ""
log_success "k3d cluster is ready!"
log_info "Cluster name: ${CLUSTER_NAME}"
log_info "kubeconfig: ${HOME}/.kube/config"
log_info ""
log_info "To stop the cluster, run: ./scripts/stop-k3d.sh"
log_info "To delete the cluster, run: DELETE_CLUSTER=true ./scripts/stop-k3d.sh"

