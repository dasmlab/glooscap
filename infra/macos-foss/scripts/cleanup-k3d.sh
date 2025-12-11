#!/usr/bin/env bash
# cleanup-k3d.sh
# Aggressively cleans up k3d cluster and containers

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

log_info "Cleaning up k3d cluster '${CLUSTER_NAME}'..."

# Detect Podman
USING_PODMAN=false
if command -v podman &> /dev/null && podman machine list 2>/dev/null | grep -q "running"; then
    USING_PODMAN=true
    CONTAINER_CMD="podman"
else
    CONTAINER_CMD="docker"
fi

# Try to delete cluster via k3d first
if command -v k3d &> /dev/null; then
    log_info "Attempting to delete cluster via k3d..."
    k3d cluster delete "${CLUSTER_NAME}" 2>/dev/null || log_warn "k3d cluster delete failed (may not exist)"
fi

# Force remove any remaining k3d containers
log_info "Removing k3d containers..."
if [ "${USING_PODMAN}" = "true" ]; then
    # Podman
    for container in $(${CONTAINER_CMD} ps -a --filter "name=k3d" --format "{{.Names}}" 2>/dev/null); do
        log_info "  Removing container: ${container}"
        ${CONTAINER_CMD} rm -f "${container}" 2>/dev/null || true
    done
else
    # Docker
    for container in $(${CONTAINER_CMD} ps -a --filter "name=k3d" --format "{{.Names}}" 2>/dev/null); do
        log_info "  Removing container: ${container}"
        ${CONTAINER_CMD} rm -f "${container}" 2>/dev/null || true
    done
fi

# Remove kubeconfig entry if it exists
if [ -f "${HOME}/.kube/config" ]; then
    log_info "Removing k3d context from kubeconfig..."
    kubectl config delete-context "k3d-${CLUSTER_NAME}" 2>/dev/null || true
    kubectl config delete-cluster "k3d-${CLUSTER_NAME}" 2>/dev/null || true
    kubectl config unset "users.k3d-${CLUSTER_NAME}" 2>/dev/null || true
fi

log_success "Cleanup complete!"
log_info "You can now try creating the cluster again: ./scripts/start-k3d.sh"

