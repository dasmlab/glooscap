#!/usr/bin/env bash
# stop-k0s.sh
# Stops the k0s cluster

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

log_info "Stopping k0s cluster..."

# Try to stop k0s gracefully
if command -v k0s &> /dev/null; then
    if k0s stop &> /dev/null; then
        log_success "k0s stopped gracefully"
    else
        log_warn "k0s stop command failed or k0s was not running"
    fi
fi

# Find and stop any remaining k0s processes
K0S_PIDS=$(pgrep -f "k0s" || true)

if [ -n "${K0S_PIDS}" ]; then
    for PID in ${K0S_PIDS}; do
        log_info "Stopping k0s process (PID: ${PID})..."
        kill "${PID}" 2>/dev/null || true
    done
    
    # Wait for processes to stop
    sleep 2
    
    # Force kill if still running
    REMAINING=$(pgrep -f "k0s" || true)
    if [ -n "${REMAINING}" ]; then
        log_warn "Force killing remaining k0s processes..."
        for PID in ${REMAINING}; do
            kill -9 "${PID}" 2>/dev/null || true
        done
    fi
else
    log_warn "k0s does not appear to be running"
fi

log_success "k0s cluster stopped"

# Optionally clean up data directory
if [[ "${CLEAN_DATA:-false}" == "true" ]]; then
    K0S_DATA_DIR="${HOME}/.k0s"
    if [ -d "${K0S_DATA_DIR}" ]; then
        log_info "Cleaning up k0s data directory: ${K0S_DATA_DIR}"
        rm -rf "${K0S_DATA_DIR}"
        log_success "Data directory cleaned"
    fi
fi

