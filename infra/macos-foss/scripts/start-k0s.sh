#!/usr/bin/env bash
# start-k0s.sh
# Starts a k0s cluster for local Glooscap development (alternative to k3s)

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

# Check if k0s is installed
if ! command -v k0s &> /dev/null; then
    log_error "k0s not found. Please run ./scripts/setup-macos-env.sh first"
    exit 1
fi

# Configuration
K0S_DATA_DIR="${HOME}/.k0s"
K0S_KUBECONFIG="${HOME}/.kube/config-k0s"

# Create data directory
mkdir -p "${K0S_DATA_DIR}"

log_info "Starting k0s cluster..."

# Check if k0s is already running
if pgrep -f "k0s" > /dev/null; then
    log_warn "k0s appears to be already running"
    exit 0
fi

# Start k0s controller
log_info "Starting k0s controller..."
k0s start --data-dir "${K0S_DATA_DIR}" --single > "${K0S_DATA_DIR}/k0s.log" 2>&1 &

K0S_PID=$!

# Wait for k0s to be ready
log_info "Waiting for k0s to be ready..."
MAX_WAIT=60
WAIT_COUNT=0

while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    if k0s kubeconfig admin > "${K0S_KUBECONFIG}" 2>/dev/null; then
        if KUBECONFIG="${K0S_KUBECONFIG}" kubectl cluster-info &> /dev/null; then
            break
        fi
    fi
    sleep 2
    WAIT_COUNT=$((WAIT_COUNT + 2))
    echo -n "."
done
echo ""

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    log_error "k0s failed to start within ${MAX_WAIT} seconds"
    log_info "Check logs: ${K0S_DATA_DIR}/k0s.log"
    kill $K0S_PID 2>/dev/null || true
    exit 1
fi

# Merge kubeconfig
log_info "Merging kubeconfig..."
if [ -f "${HOME}/.kube/config" ]; then
    KUBECONFIG="${HOME}/.kube/config:${K0S_KUBECONFIG}" kubectl config view --flatten > "${HOME}/.kube/config.tmp"
    mv "${HOME}/.kube/config.tmp" "${HOME}/.kube/config"
else
    cp "${K0S_KUBECONFIG}" "${HOME}/.kube/config"
fi

# Set current context
kubectl config use-context default || kubectl config set-context default --cluster=default --user=default

log_success "k0s cluster started successfully!"
log_info "k0s PID: ${K0S_PID}"
log_info "kubeconfig: ${HOME}/.kube/config"
log_info "Data directory: ${K0S_DATA_DIR}"
log_info "Logs: ${K0S_DATA_DIR}/k0s.log"

# Show cluster info
echo ""
log_info "Cluster information:"
kubectl cluster-info
echo ""
kubectl get nodes

echo ""
log_info "To stop k0s, run: k0s stop"
log_info "To view logs: tail -f ${K0S_DATA_DIR}/k0s.log"

