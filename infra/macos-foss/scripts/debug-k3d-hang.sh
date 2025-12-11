#!/usr/bin/env bash
# debug-k3d-hang.sh
# Debug script to see what's happening when k3d hangs

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

CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

log_info "Debugging k3d hang for cluster: ${CLUSTER_NAME}"
echo ""

# Check if using Podman
USING_PODMAN=false
if command -v podman &> /dev/null && podman machine list 2>/dev/null | grep -q "running"; then
    USING_PODMAN=true
    CONTAINER_CMD="podman"
else
    CONTAINER_CMD="docker"
fi

log_info "Container runtime: ${CONTAINER_CMD}"
echo ""

# Check for k3d containers
log_info "k3d Containers:"
if [ "${USING_PODMAN}" = "true" ]; then
    if podman ps -a 2>/dev/null | grep -q "k3d"; then
        podman ps -a --filter "name=k3d" --format "table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}\t{{.Image}}"
    else
        log_warn "No k3d containers found"
    fi
else
    if docker ps -a 2>/dev/null | grep -q "k3d"; then
        docker ps -a --filter "name=k3d" --format "table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}\t{{.Image}}"
    else
        log_warn "No k3d containers found"
    fi
fi

echo ""

# Check server container specifically
SERVER_CONTAINER="k3d-${CLUSTER_NAME}-server-0"
log_info "Server Container: ${SERVER_CONTAINER}"

if [ "${USING_PODMAN}" = "true" ]; then
    if podman ps -a 2>/dev/null | grep -q "${SERVER_CONTAINER}"; then
        log_info "Container exists, checking status..."
        podman inspect "${SERVER_CONTAINER}" --format "{{.State.Status}}" 2>/dev/null || log_warn "Could not inspect container"
        
        echo ""
        log_info "Container logs (last 30 lines):"
        podman logs --tail 30 "${SERVER_CONTAINER}" 2>/dev/null || log_warn "Could not get logs"
        
        echo ""
        log_info "Container processes:"
        podman top "${SERVER_CONTAINER}" 2>/dev/null || log_warn "Could not get processes"
        
        echo ""
        log_info "Container resource usage:"
        podman stats --no-stream "${SERVER_CONTAINER}" 2>/dev/null || log_warn "Could not get stats"
    else
        log_warn "Server container not found"
    fi
else
    if docker ps -a 2>/dev/null | grep -q "${SERVER_CONTAINER}"; then
        log_info "Container exists, checking status..."
        docker inspect "${SERVER_CONTAINER}" --format "{{.State.Status}}" 2>/dev/null || log_warn "Could not inspect container"
        
        echo ""
        log_info "Container logs (last 30 lines):"
        docker logs --tail 30 "${SERVER_CONTAINER}" 2>/dev/null || log_warn "Could not get logs"
    else
        log_warn "Server container not found"
    fi
fi

echo ""
log_info "k3d process status:"
if pgrep -f "k3d.*cluster.*create" > /dev/null; then
    log_warn "k3d process is still running"
    ps aux | grep "k3d.*cluster.*create" | grep -v grep || true
else
    log_info "No k3d create process found"
fi

echo ""
log_info "k3d log file (last 20 lines):"
if [ -f /tmp/k3d-create.log ]; then
    tail -20 /tmp/k3d-create.log
else
    log_warn "Log file not found: /tmp/k3d-create.log"
fi

echo ""
log_info "Recommendations:"
log_info "  1. If container is 'Created' but not running, check resource limits"
log_info "  2. If k3s logs show errors, check k3s compatibility with Podman"
log_info "  3. Try increasing Podman machine resources: ./scripts/check-podman-resources.sh"
log_info "  4. Check if k3d is waiting for something: tail -f /tmp/k3d-create.log"

