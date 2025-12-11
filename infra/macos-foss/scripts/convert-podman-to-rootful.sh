#!/usr/bin/env bash
# convert-podman-to-rootful.sh
# Converts existing rootless Podman machine to rootful mode

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

if ! command -v podman &> /dev/null; then
    log_error "Podman not found"
    exit 1
fi

# Get Podman machine name
PODMAN_MACHINE=$(podman machine list --format json 2>/dev/null | grep -o '"Name":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "podman-machine-default")

log_info "Podman Machine: ${PODMAN_MACHINE}"

# Check if machine is running
if podman machine list 2>/dev/null | grep -q "${PODMAN_MACHINE}.*running"; then
    log_warn "Podman machine is currently running"
    log_info "Stopping machine..."
    podman machine stop "${PODMAN_MACHINE}" || {
        log_error "Failed to stop Podman machine"
        exit 1
    }
    log_success "Machine stopped"
fi

# Check current resources
log_info "Checking current machine resources..."
MACHINE_INFO=$(podman machine inspect "${PODMAN_MACHINE}" 2>/dev/null || echo "")
CPU_COUNT=$(echo "${MACHINE_INFO}" | grep -o '"CPUs":[0-9]*' | cut -d':' -f2 || echo "2")
MEMORY=$(echo "${MACHINE_INFO}" | grep -o '"Memory":[0-9]*' | cut -d':' -f2 || echo "2048")

log_info "Current resources:"
log_info "  CPUs: ${CPU_COUNT}"
log_info "  Memory: ${MEMORY}MB"

# Confirm deletion
log_warn ""
log_warn "WARNING: This will DELETE your existing Podman machine and all containers/images in it!"
log_warn "Make sure you've backed up anything important."
log_warn ""
read -p "Continue? (yes/no): " CONFIRM

if [ "${CONFIRM}" != "yes" ]; then
    log_info "Aborted"
    exit 0
fi

# Remove existing machine
log_info "Removing existing machine..."
podman machine rm "${PODMAN_MACHINE}" || {
    log_error "Failed to remove Podman machine"
    exit 1
}
log_success "Machine removed"

# Create new machine in rootful mode
log_info "Creating new machine in rootful mode..."
log_info "  CPUs: ${CPU_COUNT}"
log_info "  Memory: ${MEMORY}MB"
log_info "  Mode: rootful"

podman machine init --rootful --cpus "${CPU_COUNT}" --memory "${MEMORY}" "${PODMAN_MACHINE}" || {
    log_error "Failed to create Podman machine in rootful mode"
    exit 1
}
log_success "Machine created in rootful mode"

# Start machine
log_info "Starting Podman machine..."
podman machine start "${PODMAN_MACHINE}" || {
    log_error "Failed to start Podman machine"
    exit 1
}
log_success "Podman machine started"

# Verify rootful mode
log_info "Verifying rootful mode..."
MACHINE_CONFIG="${HOME}/.config/containers/podman/machine/applehv/${PODMAN_MACHINE}.json"
if [ -f "${MACHINE_CONFIG}" ]; then
    if grep -q '"Rootful":true' "${MACHINE_CONFIG}" 2>/dev/null; then
        log_success "✓ Podman machine is confirmed to be in rootful mode"
    else
        log_warn "⚠ Could not verify rootful mode in config"
    fi
fi

log_success ""
log_success "Conversion complete!"
log_info "You can now run: ./scripts/start-k3d.sh"

