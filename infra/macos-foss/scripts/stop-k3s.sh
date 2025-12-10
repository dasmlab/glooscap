#!/usr/bin/env bash
# stop-k3s.sh
# NOTE: This script is kept for reference but k3s doesn't work natively on macOS
# Use stop-k3d.sh instead for macOS

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

# Find k3s processes
K3S_PIDS=$(pgrep -f "k3s server" || true)

if [ -z "${K3S_PIDS}" ]; then
    log_warn "k3s does not appear to be running"
    exit 0
fi

# Stop k3s processes
for PID in ${K3S_PIDS}; do
    log_info "Stopping k3s process (PID: ${PID})..."
    kill "${PID}" 2>/dev/null || true
done

# Wait for processes to stop
sleep 2

# Force kill if still running
REMAINING=$(pgrep -f "k3s server" || true)
if [ -n "${REMAINING}" ]; then
    log_warn "Force killing remaining k3s processes..."
    for PID in ${REMAINING}; do
        kill -9 "${PID}" 2>/dev/null || true
    done
fi

log_success "k3s cluster stopped"

# Optionally clean up data directory
if [[ "${CLEAN_DATA:-false}" == "true" ]]; then
    K3S_DATA_DIR="${HOME}/.k3s"
    if [ -d "${K3S_DATA_DIR}" ]; then
        log_info "Cleaning up k3s data directory: ${K3S_DATA_DIR}"
        rm -rf "${K3S_DATA_DIR}"
        log_success "Data directory cleaned"
    fi
fi

