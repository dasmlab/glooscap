#!/usr/bin/env bash
# stop-k3s.sh
# Stops the k3s cluster (Linux version)

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

log_info "Stopping k3s cluster..."

# Try systemd service first
if sudo systemctl list-unit-files | grep -q k3s.service; then
    if sudo systemctl is-active --quiet k3s 2>/dev/null; then
        log_info "Stopping k3s systemd service..."
        sudo systemctl stop k3s
        sudo systemctl disable k3s
        log_success "k3s systemd service stopped"
    else
        log_warn "k3s systemd service is not running"
    fi
fi

# Find and stop any remaining k3s processes
K3S_PIDS=$(pgrep -f "k3s server" || true)

if [ -n "${K3S_PIDS}" ]; then
    for PID in ${K3S_PIDS}; do
        log_info "Stopping k3s process (PID: ${PID})..."
        sudo kill "${PID}" 2>/dev/null || true
    done
    
    # Wait for processes to stop
    sleep 2
    
    # Force kill if still running
    REMAINING=$(pgrep -f "k3s server" || true)
    if [ -n "${REMAINING}" ]; then
        log_warn "Force killing remaining k3s processes..."
        for PID in ${REMAINING}; do
            sudo kill -9 "${PID}" 2>/dev/null || true
        done
    fi
fi

log_success "k3s cluster stopped"

# Optionally clean up data directory
if [[ "${CLEAN_DATA:-false}" == "true" ]]; then
    K3S_DATA_DIR="/var/lib/rancher/k3s"
    if [ -d "${K3S_DATA_DIR}" ]; then
        log_info "Cleaning up k3s data directory: ${K3S_DATA_DIR}"
        sudo rm -rf "${K3S_DATA_DIR}"
        log_success "Data directory cleaned"
    fi
    
    # Also clean user data directory if it exists
    USER_K3S_DATA_DIR="${HOME}/.k3s"
    if [ -d "${USER_K3S_DATA_DIR}" ]; then
        log_info "Cleaning up user k3s data directory: ${USER_K3S_DATA_DIR}"
        rm -rf "${USER_K3S_DATA_DIR}"
        log_success "User data directory cleaned"
    fi
fi

