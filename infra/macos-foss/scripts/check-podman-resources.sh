#!/usr/bin/env bash
# check-podman-resources.sh
# Check and optionally increase Podman machine resources

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
if ! podman machine list 2>/dev/null | grep -q "${PODMAN_MACHINE}.*running"; then
    log_warn "Podman machine is not running"
    log_info "Start it with: podman machine start"
    exit 1
fi

# Find Lima config
LIMA_CONFIG="${HOME}/.lima/${PODMAN_MACHINE}/lima.yaml"
if [ ! -f "${LIMA_CONFIG}" ]; then
    log_error "Lima config not found at: ${LIMA_CONFIG}"
    log_info "Podman machine may use a different backend"
    exit 1
fi

log_info "Lima config: ${LIMA_CONFIG}"

# Read current resources
CPU_COUNT=$(grep "^cpus:" "${LIMA_CONFIG}" 2>/dev/null | awk '{print $2}' || echo "unknown")
MEMORY=$(grep "^memory:" "${LIMA_CONFIG}" 2>/dev/null | awk '{print $2}' || echo "unknown")

log_info ""
log_info "Current Resources:"
log_info "  CPUs: ${CPU_COUNT}"
log_info "  Memory: ${MEMORY}"

# Get system resources for comparison
if [[ "$(uname)" == "Darwin" ]]; then
    SYS_CPUS=$(sysctl -n hw.ncpu)
    SYS_MEMORY_GB=$(( $(sysctl -n hw.memsize) / 1024 / 1024 / 1024 ))
    log_info ""
    log_info "System Resources:"
    log_info "  CPUs: ${SYS_CPUS}"
    log_info "  Memory: ${SYS_MEMORY_GB} GB"
fi

log_info ""
log_info "To increase resources:"
log_info "  1. Stop Podman machine: podman machine stop"
log_info "  2. Edit config: ${LIMA_CONFIG}"
log_info "  3. Increase 'cpus:' and 'memory:' values (e.g., 'cpus: 4', 'memory: 4GiB')"
log_info "  4. Start Podman machine: podman machine start"
log_info ""
log_info "Recommended minimum for k3d:"
log_info "  CPUs: 2-4"
log_info "  Memory: 4-8 GiB"

