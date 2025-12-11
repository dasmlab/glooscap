#!/usr/bin/env bash
# diagnose-k3d.sh
# Diagnoses k3d cluster issues, especially with Podman

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

echo "=== k3d Cluster Diagnostics ==="
echo ""

# Check container runtime
log_info "Container Runtime Status:"

# Determine if we're using Podman or Docker
USING_PODMAN=false
if command -v podman &> /dev/null && podman machine list 2>/dev/null | grep -q "running"; then
    USING_PODMAN=true
    log_info "  Detected Podman (machine is running)"
elif [ -n "${DOCKER_HOST:-}" ] && echo "${DOCKER_HOST}" | grep -q "podman\|unix://"; then
    USING_PODMAN=true
    log_info "  DOCKER_HOST points to Podman: ${DOCKER_HOST}"
else
    log_info "  Using Docker (or DOCKER_HOST not set for Podman)"
    if [ -n "${DOCKER_HOST:-}" ]; then
        echo "  DOCKER_HOST: ${DOCKER_HOST}"
    fi
fi

# Use the appropriate command based on what's available
if [ "${USING_PODMAN}" = "true" ]; then
    # Use Podman commands directly
    if command -v podman &> /dev/null; then
        if podman ps &> /dev/null; then
            log_success "  Podman can access container runtime"
            echo ""
            log_info "  Running containers (via Podman):"
            podman ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | head -10
        else
            log_error "  Podman cannot access container runtime"
            podman ps 2>&1 | head -5
        fi
    else
        log_error "  Podman not found"
    fi
else
    # Use Docker commands
    if command -v docker &> /dev/null; then
        if docker ps &> /dev/null; then
            log_success "  Docker CLI can access container runtime"
            echo ""
            log_info "  Running containers (via Docker):"
            docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | head -10
        else
            log_error "  Docker CLI cannot access container runtime"
            docker ps 2>&1 | head -5
        fi
    else
        log_error "  Docker CLI not found"
    fi
fi

echo ""

# Check Podman specifically
if command -v podman &> /dev/null; then
    log_info "Podman Status:"
    if podman machine list 2>/dev/null | grep -q "running"; then
        log_success "  Podman machine is running"
        PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
        if [ -n "${PODMAN_SOCKET}" ]; then
            echo "  Socket: ${PODMAN_SOCKET}"
        fi
    else
        log_warn "  Podman machine is not running"
    fi
    
    echo ""
    log_info "  Podman containers:"
    if podman ps -a &> /dev/null; then
        podman ps -a --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | head -10
    else
        log_error "  Cannot list Podman containers"
    fi
fi

echo ""

# Check k3d cluster
log_info "k3d Cluster Status:"
if command -v k3d &> /dev/null; then
    if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}"; then
        log_info "  Cluster '${CLUSTER_NAME}' exists:"
        k3d cluster list | grep "${CLUSTER_NAME}" || true
        
        # Check cluster containers
        echo ""
        log_info "  Cluster containers:"
        if [ "${USING_PODMAN}" = "true" ]; then
            if podman ps 2>/dev/null | grep -q "k3d"; then
                podman ps --filter "name=k3d" --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
            else
                log_warn "  No running k3d containers found"
            fi
            
            # Check for hanging containers
            echo ""
            log_info "  Checking for hanging/stopped containers:"
            if podman ps -a 2>/dev/null | grep -q "k3d"; then
                log_warn "  Found k3d containers (including stopped):"
                podman ps -a --filter "name=k3d" --format "table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}"
            fi
        else
            if docker ps 2>/dev/null | grep -q "k3d"; then
                docker ps --filter "name=k3d" --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
            else
                log_warn "  No running k3d containers found"
            fi
            
            # Check for hanging containers
            echo ""
            log_info "  Checking for hanging/stopped containers:"
            if docker ps -a 2>/dev/null | grep -q "k3d"; then
                log_warn "  Found stopped k3d containers:"
                docker ps -a --filter "name=k3d" --filter "status=exited" --format "table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}"
            fi
        fi
    else
        log_warn "  Cluster '${CLUSTER_NAME}' not found"
    fi
else
    log_error "  k3d not installed"
fi

echo ""

# Check Lima (Podman's VM)
if command -v limactl &> /dev/null; then
    log_info "Lima VM Status (Podman backend):"
    limactl list 2>/dev/null || log_warn "  Cannot list Lima VMs"
    
    # Check Podman machine resources
    if command -v podman &> /dev/null; then
        PODMAN_MACHINE=$(podman machine list --format json 2>/dev/null | grep -o '"Name":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
        if [ -n "${PODMAN_MACHINE}" ]; then
            log_info ""
            log_info "Podman Machine Resources:"
            log_info "  Machine name: ${PODMAN_MACHINE}"
            
            # Check Lima config for this machine
            LIMA_CONFIG="${HOME}/.lima/${PODMAN_MACHINE}/lima.yaml"
            if [ -f "${LIMA_CONFIG}" ]; then
                log_info "  Config file: ${LIMA_CONFIG}"
                CPU_COUNT=$(grep "^cpus:" "${LIMA_CONFIG}" 2>/dev/null | awk '{print $2}' || echo "unknown")
                MEMORY=$(grep "^memory:" "${LIMA_CONFIG}" 2>/dev/null | awk '{print $2}' || echo "unknown")
                log_info "  CPUs: ${CPU_COUNT}"
                log_info "  Memory: ${MEMORY}"
                
                if [ "${CPU_COUNT}" = "unknown" ] || [ "${MEMORY}" = "unknown" ]; then
                    log_warn "  Could not read resource limits from config"
                    log_info "  To increase resources, edit: ${LIMA_CONFIG}"
                    log_info "  Then restart: podman machine stop && podman machine start"
                fi
            else
                log_warn "  Lima config not found at ${LIMA_CONFIG}"
            fi
        fi
    fi
fi

echo ""

# Check kubectl
log_info "kubectl Status:"
if command -v kubectl &> /dev/null; then
    if kubectl cluster-info &> /dev/null 2>&1; then
        log_success "  kubectl can connect to cluster"
        echo ""
        kubectl get nodes 2>/dev/null || log_warn "  Nodes not available"
    else
        log_warn "  kubectl cannot connect to cluster"
        kubectl cluster-info 2>&1 | head -3 || true
    fi
else
    log_error "  kubectl not installed"
fi

echo ""

# Check system resources
log_info "System Resources:"
if [[ "$(uname)" == "Darwin" ]]; then
    echo "  CPU cores: $(sysctl -n hw.ncpu)"
    echo "  Memory: $(sysctl -n hw.memsize | awk '{print $1/1024/1024/1024 " GB"}')"
fi

echo ""
log_info "=== Recommendations ==="
echo ""
if [ -n "${DOCKER_HOST:-}" ] && echo "${DOCKER_HOST}" | grep -q "podman"; then
    log_info "You're using Podman. If k3d is hanging:"
    echo "  1. Check Podman machine: podman machine start"
    echo "  2. Check Lima VM resources: limactl list"
    echo "  3. Check k3d container logs: docker logs k3d-glooscap-server-0"
    echo "  4. Try deleting and recreating: k3d cluster delete ${CLUSTER_NAME}"
    echo "  5. Consider increasing Lima VM resources (CPU/memory)"
fi

