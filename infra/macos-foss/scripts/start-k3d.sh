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

# First, check if kubectl can already connect - if so, we're done!
if kubectl cluster-info &> /dev/null 2>&1; then
    log_success "Cluster is already accessible via kubectl"
    # Fall through to show status
else
    # Cluster not accessible, try to manage via k3d
    # Check if k3d is installed
    if ! command -v k3d &> /dev/null; then
        log_error "k3d not found. Installing..."
        brew install k3d
        log_success "k3d installed"
    fi

    CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

    # Try to list clusters - if this fails, we can't manage via k3d
    if k3d cluster list &> /dev/null 2>&1; then
        # k3d can see Docker, try to manage cluster
        if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}"; then
            if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}.*running"; then
                log_success "k3d cluster '${CLUSTER_NAME}' is running"
                # Fall through to wait/status section
            else
                log_info "Cluster '${CLUSTER_NAME}' exists but is not running. Starting it..."
                k3d cluster start "${CLUSTER_NAME}" || {
                    log_error "Failed to start cluster"
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
                --k3s-arg "--disable=servicelb@server:0" || {
                log_error "Failed to create k3d cluster"
                exit 1
            }
            log_success "k3d cluster created successfully!"
        fi
    else
        log_warn "k3d cannot access Docker daemon, but checking if cluster is accessible anyway..."
        log_info "If kubectl works, the cluster is running in a different Docker context"
        # Fall through to check kubectl again
    fi
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

# Show cluster info
echo ""
log_info "Cluster information:"
k3d cluster list
echo ""
kubectl cluster-info 2>/dev/null || log_warn "kubectl not yet configured"
kubectl get nodes 2>/dev/null || log_warn "Nodes not yet available"

echo ""
log_success "k3d cluster is ready!"
log_info "Cluster name: ${CLUSTER_NAME}"
log_info "kubeconfig: ${HOME}/.kube/config"
log_info ""
log_info "To stop the cluster, run: ./scripts/stop-k3d.sh"
log_info "To delete the cluster, run: DELETE_CLUSTER=true ./scripts/stop-k3d.sh"

