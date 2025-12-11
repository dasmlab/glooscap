#!/usr/bin/env bash
# stop-k3d.sh
# Stops the k3d cluster

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

log_info "Stopping k3d cluster '${CLUSTER_NAME}'..."

# Check if cluster is accessible via kubectl
if kubectl cluster-info &> /dev/null 2>&1; then
    log_warn "Cluster is still accessible via kubectl"
    log_info "Cluster may be running in a different Docker context"
    log_info "k3d may not be able to stop it from here"
fi

if ! command -v k3d &> /dev/null; then
    log_warn "k3d not found"
    log_info "Cannot stop cluster via k3d"
    exit 0
fi

# Try to stop via k3d
if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}.*running"; then
    k3d cluster stop "${CLUSTER_NAME}" || {
        log_warn "k3d stop failed (cluster may be in different Docker context)"
    }
    log_success "k3d cluster stop attempted"
    
    # Optionally delete cluster
    if [[ "${DELETE_CLUSTER:-false}" == "true" ]]; then
        log_warn "Deleting k3d cluster..."
        k3d cluster delete "${CLUSTER_NAME}" || {
            log_warn "k3d delete failed (cluster may be in different Docker context)"
        }
        log_success "Cluster deletion attempted"
    fi
else
    if kubectl cluster-info &> /dev/null 2>&1; then
        log_warn "k3d cannot see cluster, but kubectl can still connect"
        log_info "Cluster is running in a different Docker context"
    else
        log_success "Cluster is not running"
    fi
fi

