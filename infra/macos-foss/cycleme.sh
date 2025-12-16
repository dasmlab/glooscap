#!/usr/bin/env bash
#
# cycleme.sh - Cycles Glooscap installation for macOS FOSS setup
# Uses make build-installer to generate dist/install.yaml with all CRDs and manifests
# This ensures CRD updates are captured from kubebuilder during development
#
# Assumes you have set names and vars appropriately.

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

NAMESPACE="${NAMESPACE:-glooscap-system}"
MAX_WAIT="${MAX_WAIT:-120}"  # Maximum seconds to wait for namespace termination

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

log_step() {
    echo ""
    echo "=========================================="
    echo -e "${BLUE}$1${NC}"
    echo "=========================================="
    echo ""
}

# Check if running on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    log_error "This script is designed for macOS only"
    exit 1
fi

log_info "🔄 Cycling Glooscap deployment for macOS FOSS..."

# Step 1: Undeploy existing installation
log_step "Step 1: Undeploying existing Glooscap"
if [ -f "${SCRIPT_DIR}/scripts/undeploy-glooscap.sh" ]; then
    bash "${SCRIPT_DIR}/scripts/undeploy-glooscap.sh" || {
        log_warn "Undeploy failed (may not be deployed)"
    }
else
    log_warn "undeploy-glooscap.sh not found, skipping undeploy"
fi

# Wait for namespace to fully terminate
log_info "⏳ Waiting for namespace '${NAMESPACE}' to terminate..."
if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
    echo "   Namespace exists, waiting for termination..."
    timeout="${MAX_WAIT}"
    while [ "${timeout}" -gt 0 ]; do
        if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
            echo "   ✅ Namespace terminated"
            break
        fi
        phase=$(kubectl get namespace "${NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Terminating")
        if [ "${phase}" != "Terminating" ] && [ "${phase}" != "Active" ]; then
            echo "   ✅ Namespace phase: ${phase}"
            break
        fi
        echo "   ⏳ Still terminating... (${timeout}s remaining)"
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
        echo "   ⚠️  Warning: Namespace still exists after ${MAX_WAIT}s, proceeding anyway..."
    fi
else
    echo "   ✅ Namespace does not exist, proceeding..."
fi

# Small additional wait
log_info "⏳ Brief pause for API server to catch up..."
sleep 3

# Step 2: Generate manifests and build installer
log_step "Step 2: Generating manifests and building installer"
cd "${OPERATOR_DIR}"

log_info "Generating code..."
make generate

log_info "Generating manifests (CRDs, RBAC, etc)..."
make manifests

log_info "Building installer (generating dist/install.yaml)..."
# Set IMG to the architecture-specific image we'll build
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
log_info "Using operator image: ${OPERATOR_IMG}"

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

# Patch the generated install.yaml to use architecture-specific tags for VLLM_JOB_IMAGE
log_info "Patching install.yaml with architecture-specific translation-runner tag..."
# Replace full registry paths
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:latest|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:local-arm64|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:local-amd64|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
# Also replace image names without registry (for backwards compatibility)
sed -i.bak "s|glooscap-translation-runner:latest|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:local-arm64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:local-amd64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
rm -f "${OPERATOR_DIR}/dist/install.yaml.bak"
log_success "install.yaml patched with architecture-specific tags"

# Step 3: Build and push operator image
log_step "Step 3: Building and pushing operator image"

# Source gh-pat to get DASMLAB_GHCR_PAT for pushing images
if [ -f /home/dasm/gh-pat ]; then
    log_info "🔑 Sourcing /home/dasm/gh-pat for image push credentials..."
    source /home/dasm/gh-pat
fi

if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
    log_error "DASMLAB_GHCR_PAT is required to push images to registry"
    log_info "Set it with: export DASMLAB_GHCR_PAT=your_token"
    exit 1
fi

log_info "🏗️  Building operator image..."
if [ -f "${OPERATOR_DIR}/buildme.sh" ]; then
    bash "${OPERATOR_DIR}/buildme.sh"
else
    log_error "buildme.sh not found in operator directory"
    exit 1
fi

log_info "📤 Pushing operator image..."
if [ -f "${OPERATOR_DIR}/pushme.sh" ]; then
    bash "${OPERATOR_DIR}/pushme.sh"
else
    log_error "pushme.sh not found in operator directory"
    exit 1
fi

log_success "Operator image built and pushed: ${OPERATOR_IMG}"

# Step 4: Create namespace and registry secret
log_step "Step 4: Creating namespace and registry secret"

if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
    log_info "Namespace ${NAMESPACE} already exists"
else
    log_info "Creating namespace ${NAMESPACE}..."
    kubectl create namespace "${NAMESPACE}"
    log_success "Namespace created"
fi

log_info "🔐 Creating registry secret..."
if [ -f "${OPERATOR_DIR}/create-registry-secret.sh" ]; then
    bash "${OPERATOR_DIR}/create-registry-secret.sh" || {
        log_warn "Registry secret creation failed (may already exist)"
    }
else
    log_warn "create-registry-secret.sh not found, creating secret manually..."
    if ! kubectl get secret dasmlab-ghcr-pull -n "${NAMESPACE}" &>/dev/null; then
        echo "${DASMLAB_GHCR_PAT}" | kubectl create secret docker-registry dasmlab-ghcr-pull \
            --docker-server=ghcr.io \
            --docker-username=lmcdasm \
            --docker-password-stdin \
            --docker-email=dasmlab-bot@dasmlab.org \
            --namespace="${NAMESPACE}" || {
            log_warn "Failed to create registry secret (may already exist)"
        }
    fi
fi

log_success "Registry secret ensured"

# Step 5: Deploy operator using generated install.yaml
log_step "Step 5: Deploying operator from dist/install.yaml"

log_info "Applying dist/install.yaml (includes CRDs and operator deployment)..."
kubectl apply -f "${OPERATOR_DIR}/dist/install.yaml"

log_info "⏳ Waiting for CRDs to be registered..."
sleep 5

log_success "Operator deployed from dist/install.yaml"

# Step 6: Build and push UI image
log_step "Step 6: Building and pushing UI image"

cd "${UI_DIR}"

log_info "🏗️  Building UI image..."
if [ -f "${UI_DIR}/buildme.sh" ]; then
    bash "${UI_DIR}/buildme.sh"
else
    log_error "buildme.sh not found in UI directory"
    exit 1
fi

log_info "📤 Pushing UI image..."
if [ -f "${UI_DIR}/pushme.sh" ]; then
    bash "${UI_DIR}/pushme.sh"
else
    log_error "pushme.sh not found in UI directory"
    exit 1
fi

log_success "UI image built and pushed"

# Build and push translation-runner image
log_info "🏗️  Building translation-runner image..."
cd "${PROJECT_ROOT}"

# Validate PROJECT_ROOT and translation-runner directory
BUILD_SCRIPT="${PROJECT_ROOT}/translation-runner/build.sh"
if [ -f "${BUILD_SCRIPT}" ]; then
    log_info "Found build script at: ${BUILD_SCRIPT}"
    RUNNER_IMG="ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}"
    bash "${BUILD_SCRIPT}" "local-${ARCH_TAG}" || {
        log_error "Failed to build translation-runner image"
        exit 1
    }
    # The build script creates: ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}
    # and also tags it as: ghcr.io/dasmlab/glooscap-translation-runner:latest
    # Verify the architecture-specific tag exists, if not tag from latest
    if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${RUNNER_IMG}$"; then
        log_info "Architecture-specific tag not found, tagging from latest..."
        docker tag "${RUNNER_IMG%:*}:latest" "${RUNNER_IMG}" || {
            log_warn "Failed to tag translation-runner, trying to push latest"
            log_info "Available translation-runner images:"
            docker images | grep "glooscap-translation-runner" || log_warn "No translation-runner images found"
            RUNNER_IMG="${RUNNER_IMG%:*}:latest"
        }
    fi
    log_info "Verified translation-runner image exists: ${RUNNER_IMG}"
    log_info "📤 Pushing translation-runner image..."
    docker push "${RUNNER_IMG}" || {
        log_error "Failed to push translation-runner image"
        exit 1
    }
    log_success "Translation-runner image built and pushed: ${RUNNER_IMG}"
else
    log_warn "translation-runner/build.sh not found at: ${BUILD_SCRIPT}"
    log_info "PROJECT_ROOT resolved to: ${PROJECT_ROOT}"
    log_info "Skipping translation-runner build"
fi

# Step 7: Deploy UI
log_step "Step 7: Deploying UI"

log_info "Applying UI manifests..."
if [ -f "${SCRIPT_DIR}/manifests/ui/deployment.yaml" ]; then
    kubectl apply -f "${SCRIPT_DIR}/manifests/ui/"
    log_success "UI deployed"
else
    log_warn "UI manifests not found at ${SCRIPT_DIR}/manifests/ui/"
fi

# Step 8: Deploy WikiTarget (if exists)
log_step "Step 8: Deploying WikiTarget"

sleep 3  # Brief pause for operator to be ready

if [ -f "${SCRIPT_DIR}/manifests/wikitarget.yaml" ]; then
    log_info "Deploying WikiTarget..."
    kubectl apply -f "${SCRIPT_DIR}/manifests/wikitarget.yaml"
    log_success "WikiTarget deployed"
else
    log_info "WikiTarget manifest not found (skipping)"
fi

# Step 9: Deploy TranslationService (if exists)
log_step "Step 9: Deploying TranslationService"

sleep 2  # Brief pause

if [ -f "${SCRIPT_DIR}/manifests/translationservice.yaml" ]; then
    log_info "Deploying TranslationService..."
    kubectl apply -f "${SCRIPT_DIR}/manifests/translationservice.yaml"
    log_success "TranslationService deployed"
else
    log_info "TranslationService manifest not found (skipping)"
fi

# Wait for operator to be ready
log_info "⏳ Waiting for operator to be ready..."
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

echo ""
log_success "✅ Cycle complete!"
echo ""
log_info "Services are accessible on host ports (LoadBalancer):"
echo "  UI: http://localhost:8080"
echo "  Operator API: http://localhost:3000"
echo ""
log_info "To view logs:"
echo "  Operator: kubectl logs -f -n ${NAMESPACE} deployment/operator-controller-manager"
echo "  UI: kubectl logs -f -n ${NAMESPACE} deployment/glooscap-ui"
echo ""

