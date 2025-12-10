#!/usr/bin/env bash
# stop-minikube.sh
# Stops the minikube cluster

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

log_info "Stopping minikube cluster..."

if ! command -v minikube &> /dev/null; then
    log_error "minikube not found"
    exit 1
fi

if minikube status &> /dev/null; then
    minikube stop
    log_success "minikube cluster stopped"
    
    # Optionally delete cluster
    if [[ "${DELETE_CLUSTER:-false}" == "true" ]]; then
        log_warn "Deleting minikube cluster..."
        minikube delete
        log_success "Cluster deleted"
    fi
else
    log_warn "minikube cluster is not running"
fi

