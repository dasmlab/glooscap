#!/usr/bin/env bash
# build-and-load-images.sh
# Builds Glooscap operator and UI images locally and loads them into k3d cluster
# This ensures images are built for the correct architecture (ARM64 on macOS)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

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

# Check if k3d cluster exists
K3D_CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"
if ! k3d cluster list | grep -q "${K3D_CLUSTER_NAME}"; then
    log_error "k3d cluster '${K3D_CLUSTER_NAME}' not found"
    log_info "Please start the cluster first: ./scripts/start-k3d.sh"
    exit 1
fi

log_info "Building and loading images for k3d cluster '${K3D_CLUSTER_NAME}'..."

# Build operator image
log_info "Building operator image..."
cd "${PROJECT_ROOT}/operator"
OPERATOR_IMG="glooscap-operator:local"
make docker-build IMG="${OPERATOR_IMG}" || {
    log_error "Failed to build operator image"
    exit 1
}
log_success "Operator image built: ${OPERATOR_IMG}"

# Load operator image into k3d
log_info "Loading operator image into k3d cluster..."
k3d image import "${OPERATOR_IMG}" -c "${K3D_CLUSTER_NAME}" || {
    log_error "Failed to load operator image into k3d"
    exit 1
}
log_success "Operator image loaded into k3d"

# Build UI image
log_info "Building UI image..."
cd "${PROJECT_ROOT}/ui"
UI_IMG="glooscap-ui:local"
# Use buildme.sh if available, otherwise use docker build directly
if [ -f "./buildme.sh" ]; then
    # buildme.sh builds with tag "scratch", we'll retag
    ./buildme.sh || {
        log_error "Failed to build UI image"
        exit 1
    }
    docker tag glooscap-ui:scratch "${UI_IMG}" || {
        log_error "Failed to tag UI image"
        exit 1
    }
else
    # Fallback to docker build (buildx may not be available with Podman)
    log_info "Using docker build (buildx not available or --load not supported)"
    docker build --tag "${UI_IMG}" . || {
        log_error "Failed to build UI image"
        exit 1
    }
fi
log_success "UI image built: ${UI_IMG}"

# Load UI image into k3d
log_info "Loading UI image into k3d cluster..."
k3d image import "${UI_IMG}" -c "${K3D_CLUSTER_NAME}" || {
    log_error "Failed to load UI image into k3d"
    exit 1
}
log_success "UI image loaded into k3d"

log_success "All images built and loaded successfully!"
log_info "Images available in cluster:"
echo "  - ${OPERATOR_IMG}"
echo "  - ${UI_IMG}"
echo ""
log_info "Update deployment manifests to use these local images:"
echo "  operator: image: ${OPERATOR_IMG}"
echo "  ui: image: ${UI_IMG}"

