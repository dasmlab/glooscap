#!/usr/bin/env bash
# start-minikube.sh
# Starts a minikube cluster using Podman (recommended alternative to k3d on macOS with Podman)

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

# Check if minikube is installed
if ! command -v minikube &> /dev/null; then
    log_error "minikube not found. Installing..."
    brew install minikube
    log_success "minikube installed"
fi

# Check if Podman is available
if ! command -v podman &> /dev/null || ! podman info &> /dev/null; then
    log_error "Podman is not available or not running"
    log_info "Please ensure Podman machine is started: podman machine start"
    exit 1
fi

log_info "Starting minikube cluster with Podman driver..."

# Check if minikube is already running
if minikube status &> /dev/null; then
    log_warn "minikube cluster is already running"
    log_info "To stop it, run: ./scripts/stop-minikube.sh"
    exit 0
fi

# Start minikube with Podman driver
log_info "Starting minikube (this may take a few minutes on first run)..."
minikube start \
    --driver=podman \
    --container-runtime=containerd \
    --memory=4096 \
    --cpus=2 \
    --disk-size=20g \
    --addons=ingress \
    --addons=metrics-server

if [[ $? -eq 0 ]]; then
    log_success "minikube cluster started successfully!"
    
    # Configure kubectl
    log_info "Configuring kubectl..."
    minikube kubectl -- get nodes
    
    # Show cluster info
    echo ""
    log_info "Cluster information:"
    minikube status
    echo ""
    kubectl get nodes
    
    echo ""
    log_info "To stop minikube, run: ./scripts/stop-minikube.sh"
    log_info "To access minikube dashboard: minikube dashboard"
    log_info "To use minikube kubectl: minikube kubectl -- <command>"
else
    log_error "Failed to start minikube cluster"
    exit 1
fi

