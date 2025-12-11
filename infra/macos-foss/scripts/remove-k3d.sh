#!/usr/bin/env bash
# remove-k3d.sh
# Removes/deletes the k3d cluster completely

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

CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

log_info "Removing k3d cluster '${CLUSTER_NAME}'..."

if ! command -v k3d &> /dev/null; then
    log_warn "k3d not found, but checking if cluster is accessible via kubectl..."
    if kubectl cluster-info &> /dev/null 2>&1; then
        log_warn "Cluster is still accessible via kubectl"
        log_info "You may need to manually clean up the cluster"
        log_info "Or the cluster is running in a different Docker context"
    fi
    exit 0
fi

# Try to delete via k3d
if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}"; then
    log_info "Deleting cluster via k3d..."
    k3d cluster delete "${CLUSTER_NAME}" || {
        log_warn "k3d delete failed (cluster may be in different Docker context)"
    }
    log_success "Cluster deletion attempted"
else
    log_warn "Cluster '${CLUSTER_NAME}' not found in k3d list"
    if kubectl cluster-info &> /dev/null 2>&1; then
        log_warn "But cluster is still accessible via kubectl"
        log_info "Cluster may be running in a different Docker context"
        log_info "You may need to manually clean up"
    else
        log_success "Cluster appears to be already removed"
    fi
fi

# Clean up kubeconfig if it points to this cluster
if [ -f "${HOME}/.kube/config" ]; then
    CURRENT_CONTEXT=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ "${CURRENT_CONTEXT}" == "k3d-${CLUSTER_NAME}" ]] || [[ "${CURRENT_CONTEXT}" == *"${CLUSTER_NAME}"* ]]; then
        log_info "Removing cluster context from kubeconfig..."
        kubectl config delete-context "${CURRENT_CONTEXT}" 2>/dev/null || true
        kubectl config delete-cluster "k3d-${CLUSTER_NAME}" 2>/dev/null || true
        log_success "Kubeconfig cleaned"
    fi
fi

log_success "Cluster removal complete"

