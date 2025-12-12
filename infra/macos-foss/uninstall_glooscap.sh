#!/usr/bin/env bash
# uninstall_glooscap.sh
# Simple uninstallation script for end users
# Removes Glooscap deployment and cleans up the cluster

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
    exit 1
fi

log_info "Glooscap Uninstallation for macOS"
log_info "This will remove Glooscap and clean up the cluster"
echo ""

# Confirm uninstallation
read -p "Are you sure you want to uninstall Glooscap? (y/N): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Uninstallation cancelled"
    exit 0
fi

# Step 1: Undeploy Glooscap
log_step "Step 1: Removing Glooscap deployment"
if bash "${SCRIPT_DIR}/scripts/undeploy-glooscap.sh"; then
    log_success "Glooscap undeployed"
else
    log_warn "Undeploy may have failed (continuing cleanup)"
fi

# Step 2: Stop cluster
log_step "Step 2: Stopping Kubernetes cluster"
if bash "${SCRIPT_DIR}/scripts/stop-k3d.sh"; then
    log_success "Cluster stopped"
else
    log_warn "Cluster stop may have failed (continuing cleanup)"
fi

# Step 3: Remove cluster
log_step "Step 3: Removing Kubernetes cluster"
if bash "${SCRIPT_DIR}/scripts/remove-k3d.sh"; then
    log_success "Cluster removed"
else
    log_warn "Cluster removal may have failed"
fi

# Success!
echo ""
log_success "Glooscap uninstallation complete!"
echo ""
log_info "The cluster and all Glooscap resources have been removed"
log_info "To reinstall, run: ./install_glooscap.sh"
echo ""

