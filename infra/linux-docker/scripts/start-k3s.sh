#!/usr/bin/env bash
# start-k3s.sh
# Starts a k3s cluster for local Glooscap development (Linux version)

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

# Check if k3s is installed
if ! command -v k3s &> /dev/null; then
    log_error "k3s not found. Please run ./scripts/setup-linux-env.sh first"
    exit 1
fi

# Check if k3s is already running (via systemd)
if sudo systemctl is-active --quiet k3s 2>/dev/null; then
    log_warn "k3s appears to be already running (systemd service)"
    log_info "To stop it, run: ./scripts/stop-k3s.sh"
    exit 0
fi

# Check if k3s process is running
if pgrep -f "k3s server" > /dev/null; then
    log_warn "k3s process appears to be running"
    log_info "To stop it, run: ./scripts/stop-k3s.sh"
    exit 0
fi

log_info "Starting k3s cluster..."

# Check if port is available
K3S_PORT="${K3S_PORT:-6443}"
if sudo netstat -tlnp 2>/dev/null | grep -q ":${K3S_PORT} " || \
   ss -tlnp 2>/dev/null | grep -q ":${K3S_PORT} "; then
    log_error "Port ${K3S_PORT} is already in use"
    log_info "Please stop the service using port ${K3S_PORT} or set K3S_PORT to a different value"
    exit 1
fi

# Start k3s service (if installed via systemd)
if sudo systemctl list-unit-files | grep -q k3s.service; then
    log_info "Starting k3s systemd service..."
    sudo systemctl start k3s
    sudo systemctl enable k3s
    
    # Wait for k3s to be ready
    log_info "Waiting for k3s to be ready..."
    MAX_WAIT=60
    WAIT_COUNT=0
    
    while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
        if sudo k3s kubectl cluster-info &> /dev/null; then
            break
        fi
        sleep 2
        WAIT_COUNT=$((WAIT_COUNT + 2))
        echo -n "."
    done
    echo ""
    
    if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
        log_error "k3s failed to start within ${MAX_WAIT} seconds"
        log_info "Check logs: sudo journalctl -u k3s -f"
        exit 1
    fi
    
    # Copy kubeconfig
    log_info "Copying kubeconfig..."
    mkdir -p "${HOME}/.kube"
    sudo cp /etc/rancher/k3s/k3s.yaml "${HOME}/.kube/config"
    sudo chown $USER:$USER "${HOME}/.kube/config"
    sed -i 's/127.0.0.1/localhost/g' "${HOME}/.kube/config"
    
    log_success "k3s cluster started successfully!"
    log_info "k3s is running as a systemd service"
    log_info "kubeconfig: ${HOME}/.kube/config"
    
else
    # Fallback: start k3s manually
    log_warn "k3s systemd service not found, starting manually..."
    K3S_DATA_DIR="${HOME}/.k3s"
    K3S_KUBECONFIG="${HOME}/.kube/config-k3s"
    
    mkdir -p "${K3S_DATA_DIR}"
    
    # Start k3s server in the background
    log_info "Starting k3s server on port ${K3S_PORT}..."
    
    sudo k3s server \
        --data-dir "${K3S_DATA_DIR}" \
        --write-kubeconfig "${K3S_KUBECONFIG}" \
        --bind-address 127.0.0.1 \
        --https-listen-port ${K3S_PORT} \
        --disable traefik \
        --disable servicelb \
        > "${K3S_DATA_DIR}/k3s.log" 2>&1 &
    
    K3S_PID=$!
    
    # Wait for k3s to be ready
    log_info "Waiting for k3s to be ready..."
    MAX_WAIT=60
    WAIT_COUNT=0
    
    while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
        if sudo KUBECONFIG="${K3S_KUBECONFIG}" kubectl cluster-info &> /dev/null; then
            break
        fi
        sleep 2
        WAIT_COUNT=$((WAIT_COUNT + 2))
        echo -n "."
    done
    echo ""
    
    if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
        log_error "k3s failed to start within ${MAX_WAIT} seconds"
        log_info "Check logs: ${K3S_DATA_DIR}/k3s.log"
        sudo kill $K3S_PID 2>/dev/null || true
        exit 1
    fi
    
    # Copy kubeconfig
    sudo cp "${K3S_KUBECONFIG}" "${HOME}/.kube/config"
    sudo chown $USER:$USER "${HOME}/.kube/config"
    
    log_success "k3s cluster started successfully!"
    log_info "k3s PID: ${K3S_PID}"
    log_info "kubeconfig: ${HOME}/.kube/config"
    log_info "Data directory: ${K3S_DATA_DIR}"
    log_info "Logs: ${K3S_DATA_DIR}/k3s.log"
fi

# Show cluster info
echo ""
log_info "Cluster information:"
kubectl cluster-info
echo ""
kubectl get nodes

echo ""
log_info "To stop k3s, run: ./scripts/stop-k3s.sh"
if sudo systemctl list-unit-files | grep -q k3s.service; then
    log_info "To view logs: sudo journalctl -u k3s -f"
else
    log_info "To view logs: tail -f ${K3S_DATA_DIR}/k3s.log"
fi

