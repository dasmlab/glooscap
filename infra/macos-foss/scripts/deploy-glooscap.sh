#!/usr/bin/env bash
# deploy-glooscap.sh
# Deploys Glooscap operator and UI to the Kubernetes cluster
# Uses make build-installer to generate current CRDs and manifests (like cycleme.sh)
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
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
OPERATOR_DIR="${PROJECT_ROOT}/operator"
UI_DIR="${PROJECT_ROOT}/ui"
MANIFESTS_DIR="$(cd "${SCRIPT_DIR}/../manifests" && pwd)"

NAMESPACE="${NAMESPACE:-glooscap-system}"

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

# Detect architecture for image tags
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

OPERATOR_IMG="ghcr.io/dasmlab/glooscap-operator:local-${ARCH_TAG}"
UI_IMG="ghcr.io/dasmlab/glooscap-ui:local-${ARCH_TAG}"
log_info "Using operator image: ${OPERATOR_IMG}"
log_info "Using UI image: ${UI_IMG}"

# Check if images exist
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

# Patch the generated install.yaml to use architecture-specific tags and LoadBalancer
log_info "Patching install.yaml with architecture-specific image tags and LoadBalancer service..."

# Patch operator image
sed -i.bak "s|image:.*controller.*|image: ${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap:latest|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|controller:latest|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"

# Patch operator API service to use LoadBalancer
# Add type: LoadBalancer at the spec level (after ports section)
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
    /^  ports:$/,/^---$/ {
      /^    - name: http-api$/,/^---$/ {
        /^      targetPort: http-api$/a\
  type: LoadBalancer
      }
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
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:latest|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:local-arm64|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:local-amd64|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:latest|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:local-arm64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:local-amd64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
rm -f "${OPERATOR_DIR}/dist/install.yaml.bak"
log_success "install.yaml patched with architecture-specific tags and LoadBalancer service"

# Create namespace
log_info "Creating namespace..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Apply the generated install.yaml (includes CRDs, RBAC, and operator deployment)
log_info "Applying dist/install.yaml (includes CRDs and operator deployment)..."
kubectl apply -f "${OPERATOR_DIR}/dist/install.yaml"
log_success "Operator deployed from dist/install.yaml"

# Patch the operator API service to ensure it's LoadBalancer (fallback)
log_info "Ensuring operator API service is exposed as LoadBalancer..."
# Wait a moment for the service to be created
sleep 2
# Try to patch the service - this is the most reliable method
if kubectl get service operator-glooscap-operator-api -n "${NAMESPACE}" &>/dev/null; then
    kubectl patch service operator-glooscap-operator-api -n "${NAMESPACE}" -p '{"spec":{"type":"LoadBalancer"}}' || {
        log_warn "Failed to patch service to LoadBalancer"
    }
    # Verify the service type
    SERVICE_TYPE=$(kubectl get service operator-glooscap-operator-api -n "${NAMESPACE}" -o jsonpath='{.spec.type}' 2>/dev/null || echo "")
    if [ "${SERVICE_TYPE}" != "LoadBalancer" ]; then
        log_warn "Service type is ${SERVICE_TYPE}, expected LoadBalancer. Attempting to fix..."
        # Try alternative service name (without prefix)
        kubectl patch service glooscap-operator-api -n "${NAMESPACE}" -p '{"spec":{"type":"LoadBalancer"}}' 2>/dev/null || true
    else
        log_success "Operator API service is LoadBalancer"
    fi
else
    log_warn "Service operator-glooscap-operator-api not found, may need to wait for deployment"
fi

# Deploy UI (using current architecture-specific image)
log_info "Deploying UI..."
if [ -f "${MANIFESTS_DIR}/ui/deployment.yaml" ]; then
    # Patch UI deployment to use architecture-specific image
    TEMP_UI_DEPLOYMENT=$(mktemp)
    cp "${MANIFESTS_DIR}/ui/deployment.yaml" "${TEMP_UI_DEPLOYMENT}"
    sed -i.bak "s|:local-arm64|:local-${ARCH_TAG}|g" "${TEMP_UI_DEPLOYMENT}"
    sed -i.bak "s|:local-amd64|:local-${ARCH_TAG}|g" "${TEMP_UI_DEPLOYMENT}"
    sed -i.bak "s|ghcr.io/dasmlab/glooscap-ui:latest|${UI_IMG}|g" "${TEMP_UI_DEPLOYMENT}"
    kubectl apply -f "${TEMP_UI_DEPLOYMENT}"
    rm -f "${TEMP_UI_DEPLOYMENT}" "${TEMP_UI_DEPLOYMENT}.bak"
    log_success "UI deployed with architecture-specific tags (${ARCH_TAG})"
else
    log_warn "UI deployment manifest not found at ${MANIFESTS_DIR}/ui/deployment.yaml"
fi

# Wait for operator to be ready (idempotent - won't fail if already ready)
log_info "Waiting for operator to be ready..."
# The generated install.yaml uses "operator-controller-manager" as the deployment name
if kubectl wait --for=condition=available --timeout=10s deployment/operator-controller-manager -n "${NAMESPACE}" 2>/dev/null; then
    log_success "Operator is ready"
else
    log_info "Waiting for operator to become ready (this may take a moment)..."
    kubectl wait --for=condition=available --timeout=300s deployment/operator-controller-manager -n "${NAMESPACE}" || {
        log_warn "Operator deployment may not be ready yet"
        log_info "Check status with: kubectl get pods -n ${NAMESPACE}"
    }
fi

# Wait for UI to be ready (idempotent - won't fail if already ready)
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
log_success "Glooscap deployed successfully!"
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

