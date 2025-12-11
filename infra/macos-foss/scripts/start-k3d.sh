#!/usr/bin/env bash
# start-k3d.sh
# Starts a k3d cluster (k3s in Docker containers)
# This script relies entirely on k3d - no Docker checks needed

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

# Check if kubectl can already connect - if so, we're done!
if kubectl cluster-info &> /dev/null 2>&1; then
    log_success "Cluster is already accessible via kubectl"
    log_info "Skipping k3d management (cluster running in different Docker context)"
    
    # Show cluster status via kubectl only
    echo ""
    log_info "Cluster information:"
    kubectl cluster-info
    echo ""
    kubectl get nodes 2>/dev/null || log_warn "Nodes not yet available"
    echo ""
    log_success "k3d cluster is ready!"
    exit 0
fi

# Cluster not accessible, need to create/start it
# Check if k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found. Installing..."
    brew install k3d
    log_success "k3d installed"
fi

CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"

# Configure DOCKER_HOST for Podman if needed
if command -v podman &> /dev/null; then
    PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
    if [ -n "${PODMAN_SOCKET}" ] && [ -z "${DOCKER_HOST:-}" ]; then
        export DOCKER_HOST="unix://${PODMAN_SOCKET}"
        log_info "Configured DOCKER_HOST to use Podman: ${DOCKER_HOST}"
    fi
fi

# Try to create/start cluster - k3d will handle Docker/Podman errors
log_info "Cluster not accessible via kubectl, attempting to create/start with k3d..."

# Verify container runtime is accessible before trying k3d
RUNTIME_ACCESSIBLE=false
if [ "${USING_PODMAN:-false}" = "true" ] || ([ -n "${DOCKER_HOST:-}" ] && echo "${DOCKER_HOST}" | grep -q "podman\|unix://"); then
    # Using Podman - check with podman command
    if command -v podman &> /dev/null && podman ps &> /dev/null 2>&1; then
        RUNTIME_ACCESSIBLE=true
        log_info "Podman runtime is accessible"
    fi
else
    # Using Docker - check with docker command
    if command -v docker &> /dev/null && docker ps &> /dev/null 2>&1; then
        RUNTIME_ACCESSIBLE=true
        log_info "Docker runtime is accessible"
    fi
fi

if [ "${RUNTIME_ACCESSIBLE}" = "false" ]; then
    log_error "Container runtime is not accessible"
    if command -v podman &> /dev/null && podman machine list 2>/dev/null | grep -q "running"; then
        log_info "Podman machine is running, but podman ps failed"
        log_info "Try: podman machine restart"
    elif [ -n "${DOCKER_HOST:-}" ] && echo "${DOCKER_HOST}" | grep -q "podman"; then
        log_info "DOCKER_HOST is set to Podman, but runtime not accessible"
        log_info "Try: podman machine start"
    else
        log_info "Docker/Podman runtime not accessible"
        log_info "Check: docker ps or podman ps"
    fi
    exit 1
fi

# Try to list clusters first (to see if we can access Docker/Podman)
if k3d cluster list &> /dev/null 2>&1; then
    # k3d can see Docker, try to manage cluster
    if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}"; then
        if k3d cluster list 2>/dev/null | grep -q "${CLUSTER_NAME}.*running"; then
            log_success "k3d cluster '${CLUSTER_NAME}' is running"
        else
            log_info "Cluster '${CLUSTER_NAME}' exists but is not running. Starting it..."
            k3d cluster start "${CLUSTER_NAME}" || {
                log_error "Failed to start cluster"
                log_info "Check if Docker is running: docker ps"
                exit 1
            }
            log_success "Cluster started"
        fi
    else
        log_info "Creating k3d cluster '${CLUSTER_NAME}'..."
        
        # Check if we're using Podman (for compatibility flags)
        USING_PODMAN=false
        if [ -n "${DOCKER_HOST:-}" ] && echo "${DOCKER_HOST}" | grep -q "podman\|unix://"; then
            USING_PODMAN=true
            log_info "Detected Podman backend - using Podman-compatible flags"
        fi
        
        # Build k3d command arguments
        K3D_ARGS=(
            "cluster" "create" "${CLUSTER_NAME}"
            "--api-port" "6443"
            "--port" "8080:80@loadbalancer"
            "--port" "8443:443@loadbalancer"
            "--port" "3000:3000@loadbalancer"
            "--agents" "1"
            "--k3s-arg" "--disable=traefik@server:0"
            "--k3s-arg" "--disable=servicelb@server:0"
        )
        
        # Podman-specific: Add compatibility flags
        if [ "${USING_PODMAN}" = "true" ]; then
            log_info "Applying Podman compatibility settings..."
            # Note: --network host doesn't work well with k3d, so we skip it
            # Instead, we rely on k3d's default networking which should work with Podman
            log_info "Using default k3d networking (should work with Podman)"
        fi
        
        # Execute k3d command with timeout and logging
        log_info "Creating cluster (this may take a few minutes)..."
        log_info "Command: k3d ${K3D_ARGS[*]}"
        
        # Use timeout if available (macOS timeout or gtimeout from coreutils)
        if command -v timeout &> /dev/null || command -v gtimeout &> /dev/null; then
            TIMEOUT_CMD="timeout"
            if ! command -v timeout &> /dev/null; then
                TIMEOUT_CMD="gtimeout"
            fi
            ${TIMEOUT_CMD} 300 k3d "${K3D_ARGS[@]}" 2>&1 | tee /tmp/k3d-create.log || {
                log_error "Failed to create k3d cluster"
                if grep -q "Cannot connect to the Docker daemon\|Cannot connect to the Podman" /tmp/k3d-create.log 2>/dev/null; then
                    log_error "Container runtime is not accessible"
                    log_info ""
                    log_info "To fix this:"
                    if [ "${USING_PODMAN}" = "true" ]; then
                        log_info "  1. Ensure Podman machine is running: podman machine start"
                        log_info "  2. Check DOCKER_HOST: echo \$DOCKER_HOST"
                        log_info "  3. Verify Podman: podman ps"
                    else
                        log_info "  1. Start Docker Desktop (or Docker daemon)"
                        log_info "  2. Wait for Docker to be ready"
                    fi
                    log_info "  3. Run this script again"
                    log_info ""
                    log_info "Check container runtime status: docker ps"
                elif grep -q "starting node\|hanging\|timeout" /tmp/k3d-create.log 2>/dev/null; then
                    log_error "Cluster creation hung or timed out"
                    log_info ""
                    log_info "This is a known issue with k3d + rootless Podman"
                    log_info ""
                    log_info "Troubleshooting steps:"
                    log_info "  1. Check Podman containers: podman ps -a"
                    log_info "  2. Check Podman logs: podman logs k3d-glooscap-server-0"
                    log_info "  3. Check Lima VM: limactl list"
                    log_info "  4. Try cleaning up: k3d cluster delete ${CLUSTER_NAME} || true"
                    log_info "  5. Check system resources (CPU/memory)"
                    log_info ""
                    log_info "If the issue persists, you may need to:"
                    log_info "  - Increase Lima VM resources (CPU/memory)"
                    log_info "  - Use Docker Desktop instead of Podman"
                    log_info "  - Run Podman in rootful mode (not recommended)"
                fi
                
                # Show last 20 lines of log for debugging
                log_info ""
                log_info "Last 20 lines of k3d log:"
                tail -20 /tmp/k3d-create.log || true
                
                exit 1
            }
            log_success "k3d cluster created successfully!"
        else
            # No timeout command available, run k3d directly
            log_warn "timeout command not available, running k3d without timeout"
            k3d "${K3D_ARGS[@]}" 2>&1 | tee /tmp/k3d-create.log || {
                log_error "Failed to create k3d cluster"
                log_info "Check /tmp/k3d-create.log for details"
                exit 1
            }
            log_success "k3d cluster created successfully!"
        fi
    fi
else
    # k3d cluster list failed - Docker not accessible
    log_error "k3d cannot access Docker daemon"
    log_info ""
    log_info "k3d needs Docker to be running to create clusters."
    log_info ""
    log_info "To fix this:"
    log_info "  1. Start Docker Desktop (or Docker daemon)"
    log_info "  2. Wait for Docker to be ready"
    log_info "  3. Run this script again: ./scripts/start-k3d.sh"
    log_info ""
    log_info "Check Docker status: docker ps"
    exit 1
fi

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
MAX_WAIT=120
WAIT_COUNT=0

while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    if kubectl cluster-info &> /dev/null 2>&1; then
        break
    fi
    sleep 2
    WAIT_COUNT=$((WAIT_COUNT + 2))
    echo -n "."
done
echo ""

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    log_warn "Cluster may not be fully ready yet"
    log_info "Check status: kubectl cluster-info"
else
    log_success "Cluster is ready!"
fi

# Show cluster info via kubectl (don't use k3d cluster list)
echo ""
log_info "Cluster information:"
kubectl cluster-info 2>/dev/null || log_warn "kubectl not yet configured"
kubectl get nodes 2>/dev/null || log_warn "Nodes not yet available"

echo ""
log_success "k3d cluster is ready!"
log_info "Cluster name: ${CLUSTER_NAME}"
log_info "kubeconfig: ${HOME}/.kube/config"
log_info ""
log_info "To stop the cluster, run: ./scripts/stop-k3d.sh"
log_info "To delete the cluster, run: DELETE_CLUSTER=true ./scripts/stop-k3d.sh"

