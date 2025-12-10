#!/usr/bin/env bash
# start-k3d.sh
# Starts a k3d cluster (k3s in Docker/Podman containers) for local Glooscap development

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# Check if k3d is installed
if ! command -v k3d &> /dev/null; then
    log_error "k3d not found. Please run ./scripts/setup-macos-env.sh first"
    exit 1
fi

# Check if Podman or Docker is available
CONTAINER_RUNTIME=""
DOCKER_HOST=""

if command -v podman &> /dev/null && podman info &> /dev/null; then
    CONTAINER_RUNTIME="podman"
    log_info "Using Podman as container runtime"
    
    # k3d needs DOCKER_HOST to point to Podman socket
    # On macOS, Podman machine socket is typically at ~/.local/share/containers/podman/machine/podman-machine-default/podman.sock
    # Or we can use podman machine inspect to find it
    PODMAN_SOCKET=""
    
    # Try to find Podman socket
    if [[ -S "${HOME}/.local/share/containers/podman/machine/podman-machine-default/podman.sock" ]]; then
        PODMAN_SOCKET="${HOME}/.local/share/containers/podman/machine/podman-machine-default/podman.sock"
    elif [[ -S "/run/user/$(id -u)/podman/podman.sock" ]]; then
        PODMAN_SOCKET="/run/user/$(id -u)/podman/podman.sock"
    else
        # Try to get socket from podman machine inspect
        PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
    fi
    
    if [[ -n "${PODMAN_SOCKET}" && -S "${PODMAN_SOCKET}" ]]; then
        DOCKER_HOST="unix://${PODMAN_SOCKET}"
        export DOCKER_HOST
        log_info "Configured DOCKER_HOST for Podman: ${DOCKER_HOST}"
    else
        log_warn "Could not find Podman socket automatically"
        log_info "Trying default Podman socket locations..."
        
        # Try common locations for macOS Podman
        PODMAN_SOCKET_FOUND=false
        for sock in \
            "${HOME}/.local/share/containers/podman/machine/podman-machine-default/podman.sock" \
            "${HOME}/.local/share/containers/podman/machine/qemu/podman.sock" \
            "/run/user/$(id -u)/podman/podman.sock" \
            "/var/run/podman/podman.sock"; do
            if [[ -S "${sock}" ]]; then
                DOCKER_HOST="unix://${sock}"
                export DOCKER_HOST
                log_info "Found Podman socket: ${DOCKER_HOST}"
                PODMAN_SOCKET_FOUND=true
                break
            fi
        done
        
        # If still not found, try to get from podman context
        if [[ "${PODMAN_SOCKET_FOUND}" == "false" ]]; then
            # Try podman context
            PODMAN_CONTEXT=$(podman context ls --format json 2>/dev/null | grep -o '"Name":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
            if [[ -n "${PODMAN_CONTEXT}" ]]; then
                PODMAN_SOCKET=$(podman context inspect "${PODMAN_CONTEXT}" --format '{{.Docker.SocketPath}}' 2>/dev/null || echo "")
                if [[ -n "${PODMAN_SOCKET}" && -S "${PODMAN_SOCKET}" ]]; then
                    DOCKER_HOST="unix://${PODMAN_SOCKET}"
                    export DOCKER_HOST
                    log_info "Found Podman socket via context: ${DOCKER_HOST}"
                    PODMAN_SOCKET_FOUND=true
                fi
            fi
        fi
        
        if [[ "${PODMAN_SOCKET_FOUND}" == "false" ]]; then
            log_error "Could not find Podman socket. Please ensure Podman machine is running."
            log_info "Try: podman machine start"
            log_info "Check Podman machine status: podman machine list"
            log_info "Or set DOCKER_HOST manually:"
            log_info "  export DOCKER_HOST=unix://\$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')"
            exit 1
        fi
    fi
    
elif command -v docker &> /dev/null && docker info &> /dev/null; then
    CONTAINER_RUNTIME="docker"
    log_info "Using Docker as container runtime"
    # Docker uses default socket, no need to set DOCKER_HOST
else
    log_error "Neither Podman nor Docker is available or running"
    log_info "Please ensure Podman machine is started: podman machine start"
    exit 1
fi

# Check if cluster already exists
CLUSTER_NAME="${K3D_CLUSTER_NAME:-glooscap}"
if k3d cluster list | grep -q "${CLUSTER_NAME}"; then
    if k3d cluster list | grep -q "${CLUSTER_NAME}.*running"; then
        log_warn "k3d cluster '${CLUSTER_NAME}' is already running"
        log_info "To stop it, run: ./scripts/stop-k3d.sh"
        exit 0
    else
        log_info "Cluster '${CLUSTER_NAME}' exists but is not running. Starting it..."
        k3d cluster start "${CLUSTER_NAME}"
        log_success "Cluster started"
    fi
else
    log_info "Creating k3d cluster '${CLUSTER_NAME}'..."
    
    # Create cluster (DOCKER_HOST is already set for Podman if needed)
    log_info "Creating k3d cluster with ${CONTAINER_RUNTIME}..."
    
    # Build k3d command with appropriate flags
    K3D_CMD="k3d cluster create ${CLUSTER_NAME}"
    K3D_CMD="${K3D_CMD} --api-port 6443"
    K3D_CMD="${K3D_CMD} --port '8080:80@loadbalancer'"
    K3D_CMD="${K3D_CMD} --port '8443:443@loadbalancer'"
    K3D_CMD="${K3D_CMD} --port '3000:3000@loadbalancer'"
    K3D_CMD="${K3D_CMD} --agents 1"
    K3D_CMD="${K3D_CMD} --k3s-arg '--disable=traefik@server:0'"
    K3D_CMD="${K3D_CMD} --k3s-arg '--disable=servicelb@server:0'"
    
    # Add Podman-specific flags if using Podman
    if [[ "${CONTAINER_RUNTIME}" == "podman" ]]; then
        log_info "Adding Podman-specific configuration..."
        # Use network host mode for better compatibility with Podman
        K3D_CMD="${K3D_CMD} --network host"
        # Increase timeout for Podman (can be slower)
        K3D_CMD="${K3D_CMD} --timeout 300s"
        # Use specific image registry that works better with Podman
        K3D_CMD="${K3D_CMD} --image rancher/k3s:latest"
    fi
    
    log_info "Running: ${K3D_CMD}"
    log_info "This may take a few minutes, especially on first run (pulling images)..."
    
    # Execute the command
    eval "${K3D_CMD}" || {
        log_error "Failed to create k3d cluster"
        log_info "If it hung, try:"
        log_info "  1. Check Podman is running: podman machine list"
        log_info "  2. Check DOCKER_HOST: echo \$DOCKER_HOST"
        log_info "  3. Try with verbose output: k3d cluster create ${CLUSTER_NAME} --verbose"
        exit 1
    }
    
    log_success "k3d cluster created"
fi

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
MAX_WAIT=60
WAIT_COUNT=0

while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    if kubectl cluster-info &> /dev/null; then
        break
    fi
    sleep 2
    WAIT_COUNT=$((WAIT_COUNT + 2))
    echo -n "."
done
echo ""

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    log_error "Cluster failed to become ready within ${MAX_WAIT} seconds"
    log_info "Check cluster status: k3d cluster list"
    exit 1
fi

# Get kubeconfig
log_info "Configuring kubeconfig..."
k3d kubeconfig merge "${CLUSTER_NAME}" --kubeconfig-switch-context

log_success "k3d cluster is ready!"
log_info "Cluster name: ${CLUSTER_NAME}"
log_info "Container runtime: ${CONTAINER_RUNTIME}"
log_info "kubeconfig: ${HOME}/.kube/config"

# Show cluster info
echo ""
log_info "Cluster information:"
kubectl cluster-info
echo ""
kubectl get nodes

echo ""
log_info "To stop the cluster, run: ./scripts/stop-k3d.sh"
log_info "To delete the cluster, run: k3d cluster delete ${CLUSTER_NAME}"
log_info "Port mappings:"
log_info "  - 8080:80 (HTTP)"
log_info "  - 8443:443 (HTTPS)"
log_info "  - 3000:3000 (API)"

