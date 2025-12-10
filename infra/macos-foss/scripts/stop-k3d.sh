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

# Check if k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found"
    exit 1
fi

CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

log_info "Stopping k3d cluster '${CLUSTER_NAME}'..."

# Check if cluster exists
if ! k3d cluster list | grep -q "${CLUSTER_NAME}"; then
    log_warn "Cluster '${CLUSTER_NAME}' does not exist"
    exit 0
fi

# Stop cluster
if k3d cluster list | grep -q "${CLUSTER_NAME}.*running"; then
    k3d cluster stop "${CLUSTER_NAME}"
    log_success "Cluster stopped"
else
    log_warn "Cluster is not running"
fi

# Optionally delete cluster
if [[ "${DELETE_CLUSTER:-false}" == "true" ]]; then
    log_warn "Deleting cluster '${CLUSTER_NAME}'..."
    k3d cluster delete "${CLUSTER_NAME}"
    log_success "Cluster deleted"
else
    log_info "Cluster preserved. To delete it, run:"
    log_info "  DELETE_CLUSTER=true ./scripts/stop-k3d.sh"
    log_info "  or: k3d cluster delete ${CLUSTER_NAME}"
fi

