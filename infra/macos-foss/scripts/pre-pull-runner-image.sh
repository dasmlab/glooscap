#!/usr/bin/env bash
# pre-pull-runner-image.sh
# Pre-pulls the translation-runner image on all cluster nodes
# This ensures the image is cached before going into isolated operation (e.g., VPN-connected)
# Run this script when VPN is disconnected and GHCR is accessible

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

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

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to Kubernetes cluster"
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case "${ARCH}" in
    arm64|aarch64)
        ARCH_TAG="arm64"
        ;;
    x86_64|amd64)
        ARCH_TAG="amd64"
        ;;
    *)
        log_warn "Unknown architecture: ${ARCH}, defaulting to 'latest'"
        ARCH_TAG="latest"
        ;;
esac

# Get image from environment or use default
RUNNER_IMAGE="${VLLM_JOB_IMAGE:-ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}}"
if [ -z "${VLLM_JOB_IMAGE:-}" ]; then
    # Try to get from operator deployment
    if kubectl get deployment operator-controller-manager -n glooscap-system &>/dev/null; then
        ENV_IMAGE=$(kubectl get deployment operator-controller-manager -n glooscap-system -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="VLLM_JOB_IMAGE")].value}' 2>/dev/null || echo "")
        if [ -n "${ENV_IMAGE}" ]; then
            RUNNER_IMAGE="${ENV_IMAGE}"
            log_info "Detected image from operator deployment: ${RUNNER_IMAGE}"
        fi
    fi
fi

log_info "Pre-pulling translation-runner image: ${RUNNER_IMAGE}"
log_info "This ensures the image is cached on all nodes before isolated operation"

NAMESPACE="${NAMESPACE:-glooscap-system}"

# Create namespace if it doesn't exist
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - &>/dev/null || true

# Create a DaemonSet that will pull the image on all nodes
# The pod will just sleep, but starting it will trigger the image pull
log_info "Creating DaemonSet to pre-pull image on all nodes..."

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: pre-pull-runner-image
  namespace: ${NAMESPACE}
  labels:
    app: pre-pull-runner-image
spec:
  selector:
    matchLabels:
      app: pre-pull-runner-image
  template:
    metadata:
      labels:
        app: pre-pull-runner-image
    spec:
      imagePullSecrets:
        - name: dasmlab-ghcr-pull
      containers:
      - name: pre-pull
        image: ${RUNNER_IMAGE}
        command: ["/bin/sh", "-c", "echo 'Image pulled successfully' && sleep 3600"]
        imagePullPolicy: Always
      restartPolicy: Always
      tolerations:
        - operator: Exists
EOF

log_info "Waiting for DaemonSet pods to start and pull images..."
sleep 5

# Wait for all pods to be ready (or at least started)
MAX_WAIT=120
ELAPSED=0
while [ ${ELAPSED} -lt ${MAX_WAIT} ]; do
    READY=$(kubectl get daemonset pre-pull-runner-image -n "${NAMESPACE}" -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")
    DESIRED=$(kubectl get daemonset pre-pull-runner-image -n "${NAMESPACE}" -o jsonpath='{.status.desiredNumberScheduled}' 2>/dev/null || echo "0")
    
    if [ "${READY}" = "${DESIRED}" ] && [ "${DESIRED}" != "0" ]; then
        log_success "All pods are ready! Image has been pulled on all nodes."
        break
    fi
    
    # Check for ImagePullBackOff errors
    if kubectl get pods -n "${NAMESPACE}" -l app=pre-pull-runner-image -o jsonpath='{.items[*].status.containerStatuses[*].state.waiting.reason}' 2>/dev/null | grep -q "ImagePullBackOff\|ErrImagePull"; then
        log_error "Image pull failed on some nodes. Check pod status:"
        kubectl get pods -n "${NAMESPACE}" -l app=pre-pull-runner-image
        log_error "This may indicate:"
        log_error "  1. VPN is connected and GHCR is unreachable"
        log_error "  2. Image pull secret is missing or incorrect"
        log_error "  3. Image does not exist in registry"
        exit 1
    fi
    
    log_info "Waiting for pods to pull image... (${READY}/${DESIRED} ready, ${ELAPSED}s elapsed)"
    sleep 5
    ELAPSED=$((ELAPSED + 5))
done

if [ ${ELAPSED} -ge ${MAX_WAIT} ]; then
    log_warn "Timeout waiting for all pods to be ready. Check status:"
    kubectl get pods -n "${NAMESPACE}" -l app=pre-pull-runner-image
    log_warn "Some nodes may not have pulled the image yet"
else
    log_success "Image ${RUNNER_IMAGE} has been successfully pre-pulled on all nodes"
    log_info "You can now safely connect to VPN - the image is cached and will be used with PullIfNotPresent policy"
fi

# Clean up the DaemonSet
log_info "Cleaning up pre-pull DaemonSet..."
kubectl delete daemonset pre-pull-runner-image -n "${NAMESPACE}" --ignore-not-found=true

log_success "Pre-pull complete!"

