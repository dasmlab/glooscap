#!/usr/bin/env bash
# start-colima.sh
# Starts Colima with Kubernetes (RECOMMENDED for macOS with Podman)

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

# Check if Colima is installed
if ! command -v colima &> /dev/null; then
    log_error "Colima not found. Installing..."
    brew install colima
    log_success "Colima installed"
fi

# Check Colima status
if colima status &> /dev/null; then
    log_warn "Colima is already running"
    log_info "To stop it, run: ./scripts/stop-colima.sh"
    
    # Check if Kubernetes is enabled
    if colima status kubernetes &> /dev/null; then
        log_info "Kubernetes is already enabled"
        # Get kubeconfig
        colima kubectl config view --minify --flatten > "${HOME}/.kube/config" 2>/dev/null || true
        log_success "kubeconfig configured"
    else
        log_warn "Kubernetes is not enabled. Restarting Colima with Kubernetes..."
        colima stop
        colima start --kubernetes
    fi
    exit 0
fi

log_info "Starting Colima with Kubernetes support..."
log_info "This will create a lightweight VM and start Kubernetes inside it"
log_info "This may take a few minutes on first run..."

# Start Colima with Kubernetes
colima start --kubernetes

if [[ $? -eq 0 ]]; then
    log_success "Colima started successfully!"
    
    # Configure kubectl
    log_info "Configuring kubectl..."
    colima kubectl config view --minify --flatten > "${HOME}/.kube/config" 2>/dev/null || \
    colima kubectl config view --flatten > "${HOME}/.kube/config" 2>/dev/null || {
        log_warn "Could not automatically configure kubeconfig"
        log_info "Try manually: colima kubectl config view --flatten > ~/.kube/config"
    }
    
    # Wait for Kubernetes to be ready
    log_info "Waiting for Kubernetes to be ready..."
    MAX_WAIT=120
    WAIT_COUNT=0
    
    while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
        if kubectl cluster-info &> /dev/null; then
            break
        fi
        sleep 3
        WAIT_COUNT=$((WAIT_COUNT + 3))
        echo -n "."
    done
    echo ""
    
    if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
        log_warn "Kubernetes may not be fully ready yet"
        log_info "Check status: kubectl cluster-info"
    fi
    
    # Show cluster info
    echo ""
    log_info "Cluster information:"
    colima status
    echo ""
    kubectl cluster-info 2>/dev/null || log_warn "kubectl not yet configured"
    kubectl get nodes 2>/dev/null || log_warn "Nodes not yet available"
    
    echo ""
    log_success "Colima with Kubernetes is ready!"
    log_info "To stop Colima, run: ./scripts/stop-colima.sh"
    log_info "To access Colima shell: colima ssh"
    log_info "Colima provides Docker-compatible API, so you can use 'docker' commands"
else
    log_error "Failed to start Colima"
    exit 1
fi

