#!/usr/bin/env bash
# stop-colima.sh
# Stops Colima

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

log_info "Stopping Colima..."

if ! command -v colima &> /dev/null; then
    log_error "Colima not found"
    exit 1
fi

if colima status &> /dev/null; then
    colima stop
    log_success "Colima stopped"
    
    # Optionally delete Colima VM
    if [[ "${DELETE_VM:-false}" == "true" ]]; then
        log_warn "Deleting Colima VM..."
        colima delete -f
        log_success "Colima VM deleted"
    fi
else
    log_warn "Colima is not running"
fi

