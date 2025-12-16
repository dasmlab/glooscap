#!/usr/bin/env bash
# deploy-glooscap.sh
# Deploys Glooscap operator and UI to the Kubernetes cluster
# Optionally deploys Iskoces translation service if ISKOCES_DIR is set

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS_DIR="$(cd "${SCRIPT_DIR}/../manifests" && pwd)"

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
    log_error "kubectl not found. Please run ./scripts/setup-macos-env.sh first"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to Kubernetes cluster"
    log_info "Please ensure Kubernetes cluster is running:"
    log_info "  ./scripts/start-k3d.sh"
    exit 1
fi

log_info "Deploying Glooscap to Kubernetes cluster..."

# Detect architecture for image tag checking
ARCH=$(uname -m)
case "${ARCH}" in
    arm64|aarch64)
        ARCH_TAG="arm64"
        ;;
    x86_64|amd64)
        ARCH_TAG="amd64"
        ;;
    *)
        ARCH_TAG="unknown"
        ;;
esac

# Check if local images exist, offer to build if not
if ! docker images | grep -q "glooscap-operator.*local-${ARCH_TAG}"; then
    log_warn "Local operator image not found (glooscap-operator:local-${ARCH_TAG})"
    log_info "To build and load images, run: ./scripts/build-and-load-images.sh"
    log_info "Continuing with deployment (will fail if images not available)..."
else
    log_success "Local operator image found (local-${ARCH_TAG})"
fi

if ! docker images | grep -q "glooscap-ui.*local-${ARCH_TAG}"; then
    log_warn "Local UI image not found (glooscap-ui:local-${ARCH_TAG})"
    log_info "To build and load images, run: ./scripts/build-and-load-images.sh"
    log_info "Continuing with deployment (will fail if images not available)..."
else
    log_success "Local UI image found (local-${ARCH_TAG})"
fi

# Create namespace
log_info "Creating namespace..."
kubectl apply -f "${MANIFESTS_DIR}/namespace.yaml"

# Apply CRDs
log_info "Applying Custom Resource Definitions..."
if [ -d "${MANIFESTS_DIR}/crd" ] && [ "$(ls -A ${MANIFESTS_DIR}/crd/*.yaml 2>/dev/null)" ]; then
    kubectl apply -f "${MANIFESTS_DIR}/crd/"
    log_success "CRDs applied"
else
    log_warn "No CRD files found in ${MANIFESTS_DIR}/crd/"
    log_info "CRDs should be generated from the operator config. You may need to:"
    log_info "  cd operator && make manifests"
    log_info "  cp config/crd/bases/*.yaml ${MANIFESTS_DIR}/crd/"
fi

# Apply RBAC
log_info "Applying RBAC resources..."
kubectl apply -f "${MANIFESTS_DIR}/rbac/"
log_success "RBAC resources applied"

# Apply operator (with architecture-specific image tags)
log_info "Deploying operator..."
# Patch the deployment to use architecture-specific image tags
TEMP_DEPLOYMENT=$(mktemp)
cp "${MANIFESTS_DIR}/operator/deployment.yaml" "${TEMP_DEPLOYMENT}"
# Update image tags to match detected architecture (both operator and runner)
sed -i.bak "s|:local-arm64|:local-${ARCH_TAG}|g" "${TEMP_DEPLOYMENT}"
sed -i.bak "s|:local-amd64|:local-${ARCH_TAG}|g" "${TEMP_DEPLOYMENT}"
# Also update VLLM_JOB_IMAGE env var if it exists
sed -i.bak "s|glooscap-translation-runner:local-arm64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${TEMP_DEPLOYMENT}"
sed -i.bak "s|glooscap-translation-runner:local-amd64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${TEMP_DEPLOYMENT}"
kubectl apply -f "${TEMP_DEPLOYMENT}"
rm -f "${TEMP_DEPLOYMENT}" "${TEMP_DEPLOYMENT}.bak"
log_success "Operator deployed with architecture-specific tags (${ARCH_TAG})"

# Apply UI (with architecture-specific image tags)
log_info "Deploying UI..."
# Patch the deployment to use architecture-specific image tags
TEMP_UI_DEPLOYMENT=$(mktemp)
cp "${MANIFESTS_DIR}/ui/deployment.yaml" "${TEMP_UI_DEPLOYMENT}"
# Update image tags to match detected architecture
sed -i.bak "s|:local-arm64|:local-${ARCH_TAG}|g" "${TEMP_UI_DEPLOYMENT}"
sed -i.bak "s|:local-amd64|:local-${ARCH_TAG}|g" "${TEMP_UI_DEPLOYMENT}"
kubectl apply -f "${TEMP_UI_DEPLOYMENT}"
rm -f "${TEMP_UI_DEPLOYMENT}" "${TEMP_UI_DEPLOYMENT}.bak"
log_success "UI deployed with architecture-specific tags (${ARCH_TAG})"

# Wait for operator to be ready (idempotent - won't fail if already ready)
log_info "Waiting for operator to be ready..."
if kubectl wait --for=condition=available --timeout=10s deployment/glooscap-operator -n glooscap-system 2>/dev/null; then
    log_success "Operator is ready"
else
    log_info "Waiting for operator to become ready (this may take a moment)..."
    kubectl wait --for=condition=available --timeout=300s deployment/glooscap-operator -n glooscap-system || {
        log_warn "Operator deployment may not be ready yet"
        log_info "Check status with: kubectl get pods -n glooscap-system"
    }
fi

# Wait for UI to be ready (idempotent - won't fail if already ready)
log_info "Waiting for UI to be ready..."
if kubectl wait --for=condition=available --timeout=10s deployment/glooscap-ui -n glooscap-system 2>/dev/null; then
    log_success "UI is ready"
else
    log_info "Waiting for UI to become ready (this may take a moment)..."
    kubectl wait --for=condition=available --timeout=300s deployment/glooscap-ui -n glooscap-system || {
        log_warn "UI deployment may not be ready yet"
        log_info "Check status with: kubectl get pods -n glooscap-system"
    }
fi

# Show status
echo ""
log_success "Glooscap deployed successfully!"
echo ""
log_info "Deployment status:"
kubectl get pods -n glooscap-system
echo ""
log_info "Services:"
kubectl get svc -n glooscap-system
echo ""

# Show access instructions
log_info "Services are accessible directly on host ports (LoadBalancer):"
echo "  UI: http://localhost:8080"
echo "  Operator API: http://localhost:3000"
echo ""
log_info "To view logs:"
echo "  Operator: kubectl logs -f -n glooscap-system deployment/glooscap-operator"
echo "  UI: kubectl logs -f -n glooscap-system deployment/glooscap-ui"
echo ""

# Optionally deploy Iskoces if ISKOCES_DIR is set
if [[ -n "${ISKOCES_DIR:-}" ]] && [[ -d "${ISKOCES_DIR}/manifests" ]]; then
    log_info "ISKOCES_DIR is set, deploying Iskoces..."
    if [ -f "${ISKOCES_DIR}/manifests/deploy.sh" ]; then
        "${ISKOCES_DIR}/manifests/deploy.sh"
        echo ""
        log_info "Iskoces deployed! To configure Glooscap to use Iskoces:"
        echo "  1. Go to Glooscap UI Settings → Translation Service"
        echo "  2. Set Address: iskoces-service.iskoces.svc:50051"
        echo "  3. Set Type: iskoces"
        echo "  4. Set Secure: false"
        echo "  5. Click 'Set Configuration'"
    else
        log_warn "Iskoces deploy script not found at ${ISKOCES_DIR}/manifests/deploy.sh"
    fi
else
    log_info "To deploy Iskoces alongside Glooscap, set ISKOCES_DIR:"
    echo "  ISKOCES_DIR=/path/to/iskoces ./scripts/deploy-glooscap.sh"
fi

