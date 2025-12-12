#!/usr/bin/env bash
# install_glooscap.sh
# Simple installation script for end users
# Sets up everything needed to run Glooscap on macOS: dependencies, cluster, and deployment

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

log_step() {
    echo ""
    echo "=========================================="
    echo -e "${BLUE}$1${NC}"
    echo "=========================================="
    echo ""
}

# Check if running on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    log_error "This script is designed for macOS only"
    log_info "For Linux, see: infra/linux-docker/"
    exit 1
fi

log_info "Glooscap Installation for macOS"
log_info "This will set up everything needed to run Glooscap locally"
echo ""

# Step 1: Setup environment
log_step "Step 1: Setting up macOS environment"
if ! bash "${SCRIPT_DIR}/scripts/setup-macos-env.sh"; then
    log_error "Environment setup failed"
    exit 1
fi

# Step 2: Start Kubernetes cluster
log_step "Step 2: Starting Kubernetes cluster (k3d)"
if ! bash "${SCRIPT_DIR}/scripts/start-k3d.sh"; then
    log_error "Failed to start Kubernetes cluster"
    exit 1
fi

# Step 3: Create registry credentials (if token provided)
if [ -n "${DASMLAB_GHCR_PAT:-}" ]; then
    log_step "Step 3: Creating registry credentials"
    if ! bash "${SCRIPT_DIR}/scripts/create-registry-secret.sh"; then
        log_warn "Registry secret creation failed (images may not pull from registry)"
        log_info "Continuing anyway..."
    fi
else
    log_warn "DASMLAB_GHCR_PAT not set - skipping registry secret creation"
    log_info "If you need to pull images from ghcr.io, set: export DASMLAB_GHCR_PAT=your_token"
fi

# Step 4: Build and push images
log_step "Step 4: Building and pushing images"
if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
    log_error "DASMLAB_GHCR_PAT is required to push images to registry"
    log_info "Set it with: export DASMLAB_GHCR_PAT=your_token"
    log_info "The token should be a GitHub PAT with 'write:packages' permission"
    exit 1
fi

if ! bash "${SCRIPT_DIR}/scripts/build-and-load-images.sh"; then
    log_error "Failed to build and push images"
    exit 1
fi

# Step 5: Deploy Glooscap
log_step "Step 5: Deploying Glooscap"
if ! bash "${SCRIPT_DIR}/scripts/deploy-glooscap.sh"; then
    log_error "Failed to deploy Glooscap"
    exit 1
fi

# Success!
echo ""
log_success "Glooscap installation complete!"
echo ""
log_info "Access the services directly on host ports:"
echo "  UI: http://localhost:30080"
echo "  Operator API: http://localhost:30000"
echo "  Operator Health: http://localhost:30081/healthz"
echo ""
log_info "View logs:"
echo "  Operator: kubectl logs -f -n glooscap-system deployment/glooscap-operator"
echo "  UI: kubectl logs -f -n glooscap-system deployment/glooscap-ui"
echo ""
log_info "To uninstall, run: ./uninstall_glooscap.sh"
echo ""

