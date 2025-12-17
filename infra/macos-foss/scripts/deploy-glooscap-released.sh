#!/usr/bin/env bash
# deploy-glooscap-released.sh
# Deploys Glooscap operator and UI to the Kubernetes cluster using released/pre-built images
# Uses make build-installer to generate current CRDs and manifests
# Expects GLOOSCAP_VERSION env var (defaults to 'latest')

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
OPERATOR_DIR="${PROJECT_ROOT}/operator"
UI_DIR="${PROJECT_ROOT}/ui"
MANIFESTS_DIR="$(cd "${SCRIPT_DIR}/../manifests" && pwd)"

NAMESPACE="${NAMESPACE:-glooscap-system}"

# Image version/tag to use (defaults to 'released', can be overridden with GLOOSCAP_VERSION env var)
# The 'released' tag represents the latest release images pushed by release_images.sh
GLOOSCAP_VERSION="${GLOOSCAP_VERSION:-released}"

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

log_info "Deploying Glooscap to Kubernetes cluster using released images..."
log_info "Using image version: ${GLOOSCAP_VERSION}"

# Use released images (no architecture-specific tags for released images)
OPERATOR_IMG="ghcr.io/dasmlab/glooscap-operator:${GLOOSCAP_VERSION}"
UI_IMG="ghcr.io/dasmlab/glooscap-ui:${GLOOSCAP_VERSION}"
RUNNER_IMG="ghcr.io/dasmlab/glooscap-translation-runner:${GLOOSCAP_VERSION}"

log_info "Using operator image: ${OPERATOR_IMG}"
log_info "Using UI image: ${UI_IMG}"
log_info "Using translation-runner image: ${RUNNER_IMG}"

# Generate manifests and build installer (like cycleme.sh)
log_info "Generating manifests and building installer..."
cd "${OPERATOR_DIR}"

log_info "Generating code..."
make generate

log_info "Generating manifests (CRDs, RBAC, etc)..."
make manifests

# Ensure kustomization uses the correct namespace
KUSTOMIZATION_FILE="${OPERATOR_DIR}/config/default/kustomization.yaml"
CURRENT_NAMESPACE=$(grep "^namespace:" "${KUSTOMIZATION_FILE}" | awk '{print $2}' || echo "")
if [ "${CURRENT_NAMESPACE}" != "${NAMESPACE}" ]; then
    log_info "Updating kustomization namespace from '${CURRENT_NAMESPACE}' to '${NAMESPACE}'..."
    sed -i.bak "s/^namespace:.*/namespace: ${NAMESPACE}/" "${KUSTOMIZATION_FILE}"
    RESTORE_KUSTOMIZATION=true
else
    log_info "Kustomization namespace is already '${NAMESPACE}'"
    RESTORE_KUSTOMIZATION=false
fi

log_info "Building installer (generating dist/install.yaml)..."
make build-installer IMG="${OPERATOR_IMG}"

# Restore original namespace if we changed it
if [ "${RESTORE_KUSTOMIZATION}" = "true" ] && [ -f "${KUSTOMIZATION_FILE}.bak" ]; then
    log_info "Restoring original kustomization namespace..."
    mv "${KUSTOMIZATION_FILE}.bak" "${KUSTOMIZATION_FILE}"
fi

if [ ! -f "${OPERATOR_DIR}/dist/install.yaml" ]; then
    log_error "dist/install.yaml was not generated"
    exit 1
fi

log_success "Installer generated: ${OPERATOR_DIR}/dist/install.yaml"

# Patch the generated install.yaml to use released image tags and LoadBalancer
log_info "Patching install.yaml with released image tags and LoadBalancer service..."

# Patch operator image
sed -i.bak "s|image:.*controller.*|image: ${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap:latest|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-operator:.*|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|controller:latest|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"

# Patch operator API service to use LoadBalancer
sed -i.bak '/name:.*glooscap-operator-api/,/^---$/ {
  /^spec:$/,/^---$/ {
    /ports:/,/^---$/ {
      /targetPort: http-api$/a\
  type: LoadBalancer
    }
  }
}' "${OPERATOR_DIR}/dist/install.yaml" || \
sed -i.bak '/name:.*glooscap-operator-api/,/^---$/ {
  /^spec:$/,/^---$/ {
    /^  selector:$/i\
  type: LoadBalancer
  }
}' "${OPERATOR_DIR}/dist/install.yaml"

# Patch translation-runner image
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:.*|${RUNNER_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:.*|glooscap-translation-runner:${GLOOSCAP_VERSION}|g" "${OPERATOR_DIR}/dist/install.yaml"

# Set imagePullPolicy to Always for released images (to ensure latest is pulled)
perl -i.bak -pe 's/imagePullPolicy:.*/imagePullPolicy: Always/g' "${OPERATOR_DIR}/dist/install.yaml" 2>/dev/null || \
sed -i.bak 's|imagePullPolicy:.*|imagePullPolicy: Always|g' "${OPERATOR_DIR}/dist/install.yaml"

# Add imagePullPolicy if it doesn't exist after image line
if ! grep -A 1 "image:.*glooscap-operator" "${OPERATOR_DIR}/dist/install.yaml" | grep -q "imagePullPolicy:"; then
    awk '/image:.*glooscap-operator/ {print; print "        imagePullPolicy: Always"; next}1' "${OPERATOR_DIR}/dist/install.yaml" > "${OPERATOR_DIR}/dist/install.yaml.tmp" && \
    mv "${OPERATOR_DIR}/dist/install.yaml.tmp" "${OPERATOR_DIR}/dist/install.yaml"
fi

rm -f "${OPERATOR_DIR}/dist/install.yaml.bak"
log_success "install.yaml patched with released image tags and LoadBalancer service"

# Create namespace
log_info "Creating namespace..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Apply the generated install.yaml (includes CRDs, RBAC, and operator deployment)
log_info "Applying dist/install.yaml (includes CRDs and operator deployment)..."
kubectl apply -f "${OPERATOR_DIR}/dist/install.yaml"
log_success "Operator deployed from dist/install.yaml"

# Patch the operator API service to ensure it's LoadBalancer (fallback)
log_info "Ensuring operator API service is exposed as LoadBalancer..."
sleep 2
if kubectl get service operator-glooscap-operator-api -n "${NAMESPACE}" &>/dev/null; then
    kubectl patch service operator-glooscap-operator-api -n "${NAMESPACE}" -p '{"spec":{"type":"LoadBalancer"}}' || {
        log_warn "Failed to patch service to LoadBalancer"
    }
    SERVICE_TYPE=$(kubectl get service operator-glooscap-operator-api -n "${NAMESPACE}" -o jsonpath='{.spec.type}' 2>/dev/null || echo "")
    if [ "${SERVICE_TYPE}" != "LoadBalancer" ]; then
        log_warn "Service type is ${SERVICE_TYPE}, expected LoadBalancer. Attempting to fix..."
        kubectl patch service glooscap-operator-api -n "${NAMESPACE}" -p '{"spec":{"type":"LoadBalancer"}}' 2>/dev/null || true
    else
        log_success "Operator API service is LoadBalancer"
    fi
else
    log_warn "Service operator-glooscap-operator-api not found, may need to wait for deployment"
fi

# Deploy UI (using released image)
log_info "Deploying UI with released image..."
if [ -f "${MANIFESTS_DIR}/ui/deployment.yaml" ]; then
    # Patch UI deployment to use released image
    TEMP_UI_DEPLOYMENT=$(mktemp)
    cp "${MANIFESTS_DIR}/ui/deployment.yaml" "${TEMP_UI_DEPLOYMENT}"
    sed -i.bak "s|ghcr.io/dasmlab/glooscap-ui:.*|${UI_IMG}|g" "${TEMP_UI_DEPLOYMENT}"
    sed -i.bak "s|imagePullPolicy:.*|imagePullPolicy: Always|g" "${TEMP_UI_DEPLOYMENT}"
    # Add imagePullPolicy if it doesn't exist
    if ! grep -A 1 "image:.*glooscap-ui" "${TEMP_UI_DEPLOYMENT}" | grep -q "imagePullPolicy:"; then
        awk '/image:.*glooscap-ui/ {print; print "        imagePullPolicy: Always"; next}1' "${TEMP_UI_DEPLOYMENT}" > "${TEMP_UI_DEPLOYMENT}.tmp" && \
        mv "${TEMP_UI_DEPLOYMENT}.tmp" "${TEMP_UI_DEPLOYMENT}"
    fi
    kubectl apply -f "${TEMP_UI_DEPLOYMENT}"
    rm -f "${TEMP_UI_DEPLOYMENT}" "${TEMP_UI_DEPLOYMENT}.bak"
    log_success "UI deployment applied with released image (${GLOOSCAP_VERSION})"
else
    log_warn "UI deployment manifest not found at ${MANIFESTS_DIR}/ui/deployment.yaml"
fi

# Deploy UI service (LoadBalancer for direct host access)
if [ -f "${MANIFESTS_DIR}/ui/service.yaml" ]; then
    log_info "Deploying UI service (LoadBalancer)..."
    kubectl apply -f "${MANIFESTS_DIR}/ui/service.yaml"
    log_success "UI service deployed (LoadBalancer on port 80 -> 8080)"
else
    log_warn "UI service manifest not found at ${MANIFESTS_DIR}/ui/service.yaml"
fi

# Wait for operator to be ready
log_info "Waiting for operator to be ready..."
if kubectl wait --for=condition=available --timeout=10s deployment/operator-controller-manager -n "${NAMESPACE}" 2>/dev/null; then
    log_success "Operator is ready"
else
    log_info "Waiting for operator to become ready (this may take a moment)..."
    kubectl wait --for=condition=available --timeout=300s deployment/operator-controller-manager -n "${NAMESPACE}" || {
        log_warn "Operator deployment may not be ready yet"
        log_info "Check status with: kubectl get pods -n ${NAMESPACE}"
    }
fi

# Wait for UI to be ready
log_info "Waiting for UI to be ready..."
if kubectl wait --for=condition=available --timeout=10s deployment/glooscap-ui -n "${NAMESPACE}" 2>/dev/null; then
    log_success "UI is ready"
else
    log_info "Waiting for UI to become ready (this may take a moment)..."
    kubectl wait --for=condition=available --timeout=300s deployment/glooscap-ui -n "${NAMESPACE}" || {
        log_warn "UI deployment may not be ready yet"
        log_info "Check status with: kubectl get pods -n ${NAMESPACE}"
    }
fi

# Show status
echo ""
log_success "Glooscap deployed successfully with released images!"
echo ""
log_info "Deployment status:"
kubectl get pods -n "${NAMESPACE}"
echo ""
log_info "Services:"
kubectl get svc -n "${NAMESPACE}"
echo ""

# Show access instructions
log_info "Services are accessible directly on host ports (LoadBalancer):"
echo "  UI: http://localhost:8080"
echo "  Operator API: http://localhost:3000"
echo ""
log_info "To view logs:"
echo "  Operator: kubectl logs -f -n ${NAMESPACE} deployment/operator-controller-manager"
echo "  UI: kubectl logs -f -n ${NAMESPACE} deployment/glooscap-ui"
echo ""
log_info "Images used:"
echo "  Operator: ${OPERATOR_IMG}"
echo "  UI: ${UI_IMG}"
echo "  Translation Runner: ${RUNNER_IMG}"
echo ""

