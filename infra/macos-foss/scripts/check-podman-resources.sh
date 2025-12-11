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

# Get machine info to find config location
log_info "Checking Podman machine configuration..."
MACHINE_INFO=$(podman machine inspect "${PODMAN_MACHINE}" 2>/dev/null || echo "")

# Try to find config location from machine info or common locations
CONFIG_FILE=""
BACKEND="unknown"

# Check for AppleHV backend (macOS native)
APPLEHV_CONFIG="${HOME}/.config/containers/podman/machine/applehv/${PODMAN_MACHINE}.json"
if [ -f "${APPLEHV_CONFIG}" ]; then
    CONFIG_FILE="${APPLEHV_CONFIG}"
    BACKEND="AppleHV"
    log_info "Detected AppleHV backend"
# Check for Lima backend
elif [ -f "${HOME}/.lima/${PODMAN_MACHINE}/lima.yaml" ]; then
    CONFIG_FILE="${HOME}/.lima/${PODMAN_MACHINE}/lima.yaml"
    BACKEND="Lima"
    log_info "Detected Lima backend"
# Try to find from machine info
elif [ -n "${MACHINE_INFO}" ]; then
    # Extract config path from machine info if available
    CONFIG_PATH=$(echo "${MACHINE_INFO}" | grep -i "config\|path" | head -1 || echo "")
    if [ -n "${CONFIG_PATH}" ]; then
        log_info "Found config reference: ${CONFIG_PATH}"
    fi
fi

if [ -z "${CONFIG_FILE}" ] || [ ! -f "${CONFIG_FILE}" ]; then
    log_warn "Could not find Podman machine config file"
    log_info "Tried locations:"
    log_info "  - ${APPLEHV_CONFIG}"
    log_info "  - ${HOME}/.lima/${PODMAN_MACHINE}/lima.yaml"
    log_info ""
    log_info "Checking machine info for config location..."
    podman machine inspect "${PODMAN_MACHINE}" 2>/dev/null | head -20 || true
    log_info ""
    log_info "For AppleHV backend, resources are typically set when creating the machine:"
    log_info "  podman machine stop"
    log_info "  podman machine rm ${PODMAN_MACHINE}"
    log_info "  podman machine init --cpus 4 --memory 4096 ${PODMAN_MACHINE}"
    log_info "  podman machine start ${PODMAN_MACHINE}"
    exit 1
fi

log_info "Config file: ${CONFIG_FILE}"
log_info "Backend: ${BACKEND}"

# Read current resources based on backend
if [ "${BACKEND}" = "Lima" ]; then
    CPU_COUNT=$(grep "^cpus:" "${CONFIG_FILE}" 2>/dev/null | awk '{print $2}' || echo "unknown")
    MEMORY=$(grep "^memory:" "${CONFIG_FILE}" 2>/dev/null | awk '{print $2}' || echo "unknown")
elif [ "${BACKEND}" = "AppleHV" ]; then
    # AppleHV uses JSON format
    CPU_COUNT=$(grep -o '"CPUs":[0-9]*' "${CONFIG_FILE}" 2>/dev/null | cut -d':' -f2 || echo "unknown")
    MEMORY=$(grep -o '"Memory":[0-9]*' "${CONFIG_FILE}" 2>/dev/null | cut -d':' -f2 || echo "unknown")
    if [ "${MEMORY}" != "unknown" ]; then
        # Memory is in MB, convert to GiB for display
        MEMORY_GB=$(( MEMORY / 1024 ))
        MEMORY="${MEMORY}MB (${MEMORY_GB}GiB)"
    fi
else
    # Try to get from machine info
    CPU_COUNT=$(echo "${MACHINE_INFO}" | grep -i "cpu" | head -1 || echo "unknown")
    MEMORY=$(echo "${MACHINE_INFO}" | grep -i "memory" | head -1 || echo "unknown")
fi

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
if [ "${BACKEND}" = "AppleHV" ]; then
    log_info "  AppleHV backend requires recreating the machine:"
    log_info "  1. Stop machine: podman machine stop"
    log_info "  2. Remove machine: podman machine rm ${PODMAN_MACHINE}"
    log_info "  3. Create new with more resources (rootful mode for k3d):"
    log_info "     podman machine init --rootful --cpus 4 --memory 4096 ${PODMAN_MACHINE}"
    log_info "  4. Start machine: podman machine start ${PODMAN_MACHINE}"
    log_info ""
    log_info "  Note: This will delete existing containers/images in the machine"
    log_info "  Note: --rootful is required for k3d to work properly"
elif [ "${BACKEND}" = "Lima" ]; then
    log_info "  1. Stop Podman machine: podman machine stop"
    log_info "  2. Edit config: ${CONFIG_FILE}"
    log_info "  3. Increase 'cpus:' and 'memory:' values (e.g., 'cpus: 4', 'memory: 4GiB')"
    log_info "  4. Start Podman machine: podman machine start"
else
    log_info "  Unknown backend - check Podman documentation for your setup"
fi

# Check if rootful
if [ -f "${CONFIG_FILE}" ]; then
    if grep -q '"Rootful":true' "${CONFIG_FILE}" 2>/dev/null || grep -q "rootful: true" "${CONFIG_FILE}" 2>/dev/null; then
        log_success "Podman machine is in rootful mode (good for k3d)"
    else
        log_warn "Podman machine is in rootless mode"
        log_info "k3d works better with rootful mode - consider recreating machine with --rootful"
    fi
fi
log_info ""
log_info "Recommended minimum for k3d:"
log_info "  CPUs: 2-4"
log_info "  Memory: 4-8 GiB"

