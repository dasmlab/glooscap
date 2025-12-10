#!/usr/bin/env bash
# start-k3d.sh
# Starts a k3d cluster (k3s in Docker/Podman containers) for local Glooscap development

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# Check if k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found. Please run ./scripts/setup-macos-env.sh first"
    exit 1
fi

# Check if Podman or Docker is available
CONTAINER_RUNTIME=""
if command -v podman &> /dev/null && podman info &> /dev/null; then
    CONTAINER_RUNTIME="podman"
    log_info "Using Podman as container runtime"
elif command -v docker &> /dev/null && docker info &> /dev/null; then
    CONTAINER_RUNTIME="docker"
    log_info "Using Docker as container runtime"
else
    log_error "Neither Podman nor Docker is available or running"
    log_info "Please ensure Podman machine is started: podman machine start"
    exit 1
fi

# Check if cluster already exists
CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"
if k3d cluster list | grep -q "${CLUSTER_NAME}"; then
    if k3d cluster list | grep -q "${CLUSTER_NAME}.*running"; then
        log_warn "k3d cluster '${CLUSTER_NAME}' is already running"
        log_info "To stop it, run: ./scripts/stop-k3d.sh"
        exit 0
    else
        log_info "Cluster '${CLUSTER_NAME}' exists but is not running. Starting it..."
        k3d cluster start "${CLUSTER_NAME}"
        log_success "Cluster started"
    fi
else
    log_info "Creating k3d cluster '${CLUSTER_NAME}'..."
    
    # Create cluster with appropriate runtime
    if [[ "${CONTAINER_RUNTIME}" == "podman" ]]; then
        # k3d with Podman
        k3d cluster create "${CLUSTER_NAME}" \
            --api-port 6443 \
            --port "8080:80@loadbalancer" \
            --port "8443:443@loadbalancer" \
            --port "3000:3000@loadbalancer" \
            --agents 1 \
            --k3s-arg "--disable=traefik@server:0" \
            --k3s-arg "--disable=servicelb@server:0"
    else
        # k3d with Docker
        k3d cluster create "${CLUSTER_NAME}" \
            --api-port 6443 \
            --port "8080:80@loadbalancer" \
            --port "8443:443@loadbalancer" \
            --port "3000:3000@loadbalancer" \
            --agents 1 \
            --k3s-arg "--disable=traefik@server:0" \
            --k3s-arg "--disable=servicelb@server:0"
    fi
    
    log_success "k3d cluster created"
fi

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
MAX_WAIT=60
WAIT_COUNT=0

while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    if kubectl cluster-info &> /dev/null; then
        break
    fi
    sleep 2
    WAIT_COUNT=$((WAIT_COUNT + 2))
    echo -n "."
done
echo ""

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    log_error "Cluster failed to become ready within ${MAX_WAIT} seconds"
    log_info "Check cluster status: k3d cluster list"
    exit 1
fi

# Get kubeconfig
log_info "Configuring kubeconfig..."
k3d kubeconfig merge "${CLUSTER_NAME}" --kubeconfig-switch-context

log_success "k3d cluster is ready!"
log_info "Cluster name: ${CLUSTER_NAME}"
log_info "Container runtime: ${CONTAINER_RUNTIME}"
log_info "kubeconfig: ${HOME}/.kube/config"

# Show cluster info
echo ""
log_info "Cluster information:"
kubectl cluster-info
echo ""
kubectl get nodes

echo ""
log_info "To stop the cluster, run: ./scripts/stop-k3d.sh"
log_info "To delete the cluster, run: k3d cluster delete ${CLUSTER_NAME}"
log_info "Port mappings:"
log_info "  - 8080:80 (HTTP)"
log_info "  - 8443:443 (HTTPS)"
log_info "  - 3000:3000 (API)"

