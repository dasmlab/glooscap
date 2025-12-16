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
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
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

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found. Please install kubectl first"
    log_info "Run: ./scripts/setup-macos-env.sh"
    exit 1
fi

# Check if cluster is accessible
log_info "Checking cluster connectivity..."
if ! kubectl cluster-info &> /dev/null 2>&1; then
    log_error "Cannot connect to Kubernetes cluster"
    log_info "Please ensure the cluster is running:"
    log_info "  Run: ./scripts/start-k3d.sh"
    log_info "  Or check cluster status: kubectl cluster-info"
    exit 1
fi
log_success "Cluster is accessible"

log_info "🔄 Cycling Glooscap deployment for macOS FOSS..."

# Step 1: Undeploy existing installation
log_step "Step 1: Undeploying existing Glooscap"
DELETE_NAMESPACE="${DELETE_NAMESPACE:-true}"  # Default to deleting namespace for cycle

# First, try to delete the namespace directly if DELETE_NAMESPACE=true (this handles stuck namespaces)
if [ "${DELETE_NAMESPACE}" = "true" ]; then
    log_info "Deleting namespace ${NAMESPACE} (if it exists)..."
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true --timeout=10s 2>/dev/null || true
fi

# Then run undeploy to clean up resources (this will fail gracefully if nothing is deployed)
UNDEPLOY_SCRIPT="${SCRIPT_DIR}/scripts/undeploy-glooscap.sh"
if [ -f "${UNDEPLOY_SCRIPT}" ]; then
    DELETE_NAMESPACE="false" bash "${UNDEPLOY_SCRIPT}" || {
        log_warn "Undeploy failed (may not be deployed, continuing...)"
    }
else
    log_warn "undeploy-glooscap.sh not found at ${UNDEPLOY_SCRIPT}, skipping undeploy"
fi

# Wait for namespace to fully terminate (only if we're deleting it)
if [ "${DELETE_NAMESPACE}" = "true" ]; then
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
            echo "   ⚠️  Warning: Namespace still exists after ${MAX_WAIT}s, attempting force delete..."
            kubectl delete namespace "${NAMESPACE}" --force --grace-period=0 --timeout=10s 2>/dev/null || true
            sleep 3
            if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
                echo "   ⚠️  Warning: Namespace still exists after force delete, proceeding anyway..."
                echo "   💡 You may need to manually clean up: kubectl delete namespace ${NAMESPACE} --force --grace-period=0"
            else
                echo "   ✅ Namespace force deleted"
            fi
        fi
    else
        echo "   ✅ Namespace does not exist, proceeding..."
    fi
else
    log_info "Namespace preserved (DELETE_NAMESPACE=false), skipping termination wait"
fi

# Small additional wait
log_info "⏳ Brief pause for API server to catch up..."
sleep 3

# Step 2: Generate manifests and determine architecture
log_step "Step 2: Generating manifests"
cd "${OPERATOR_DIR}"

log_info "Generating code..."
make generate

log_info "Generating manifests (CRDs, RBAC, etc)..."
make manifests

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
UI_IMG="ghcr.io/dasmlab/glooscap-ui:local-${ARCH_TAG}"
log_info "Using operator image: ${OPERATOR_IMG}"
log_info "Using UI image: ${UI_IMG}"

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

# Restore original namespace if we changed it (before deploying)
if [ "${RESTORE_KUSTOMIZATION}" = "true" ] && [ -f "${KUSTOMIZATION_FILE}.bak" ]; then
    log_info "Restoring original kustomization namespace..."
    mv "${KUSTOMIZATION_FILE}.bak" "${KUSTOMIZATION_FILE}"
fi

# Step 3: Build and push operator image
log_step "Step 3: Building and pushing operator image"

# Source gh-pat to get DASMLAB_GHCR_PAT for pushing images
if [ -f "${HOME}/gh-pat" ]; then
    log_info "🔑 Sourcing ${HOME}/gh-pat for image push credentials..."
    source "${HOME}/gh-pat"
fi

if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
    log_error "DASMLAB_GHCR_PAT is required to push images to registry"
    log_info "Set it with: export DASMLAB_GHCR_PAT=your_token"
    exit 1
fi

log_info "🏗️  Building operator image..."
cd "${OPERATOR_DIR}"
make docker-build IMG="${OPERATOR_IMG}" || {
    log_error "Failed to build operator image"
    exit 1
}
log_success "Operator image built: ${OPERATOR_IMG}"

log_info "📤 Pushing operator image..."
docker push "${OPERATOR_IMG}" || {
    log_error "Failed to push operator image"
    exit 1
}
log_success "Operator image pushed: ${OPERATOR_IMG}"

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

# Step 5: Deploy operator using dist/install.yaml (includes CRDs, RBAC, deployment, service account, etc.)
log_step "Step 5: Deploying operator from dist/install.yaml"

log_info "Applying dist/install.yaml (includes CRDs, RBAC, deployment, service account, etc.)..."
kubectl apply -f "${OPERATOR_DIR}/dist/install.yaml"

log_info "⏳ Waiting for CRDs to be registered..."
sleep 5

# Patch VLLM_JOB_IMAGE env var in the operator deployment if needed
RUNNER_IMG_VALUE="ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}"
if kubectl get deployment operator-controller-manager -n "${NAMESPACE}" &>/dev/null; then
    log_info "Patching VLLM_JOB_IMAGE env var in operator deployment..."
    kubectl set env deployment/operator-controller-manager -n "${NAMESPACE}" \
        VLLM_JOB_IMAGE="${RUNNER_IMG_VALUE}" || {
        log_warn "Failed to set VLLM_JOB_IMAGE (may not be needed)"
    }
fi

# Verify the operator deployment was created
log_info "Verifying operator deployment was created..."
if kubectl get deployment operator-controller-manager -n "${NAMESPACE}" &>/dev/null; then
    log_success "Operator deployment found in namespace ${NAMESPACE}"
else
    log_error "Operator deployment 'operator-controller-manager' not found in namespace '${NAMESPACE}'"
    log_info "Checking what was actually deployed:"
    kubectl get all -n "${NAMESPACE}" || true
    log_info "Checking if deployment exists in wrong namespace:"
    kubectl get deployment operator-controller-manager -A || true
    exit 1
fi

# Restore original namespace if we changed it
if [ "${RESTORE_KUSTOMIZATION}" = "true" ] && [ -f "${KUSTOMIZATION_FILE}.bak" ]; then
    log_info "Restoring original kustomization namespace..."
    mv "${KUSTOMIZATION_FILE}.bak" "${KUSTOMIZATION_FILE}"
fi

log_success "Operator deployed"

# Step 6: Build and push UI image
log_step "Step 6: Building and pushing UI image"

cd "${UI_DIR}"

log_info "🏗️  Building UI image..."
# Use buildme.sh if available, otherwise use docker build directly
if [ -f "./buildme.sh" ]; then
    # buildme.sh builds with tag "scratch", we'll retag
    ./buildme.sh || {
        log_error "Failed to build UI image"
        exit 1
    }
    docker tag glooscap-ui:scratch "${UI_IMG}" || {
        log_error "Failed to tag UI image"
        exit 1
    }
else
    # Fallback to docker build
    log_info "Using docker build (buildme.sh not found)"
    docker build --tag "${UI_IMG}" . || {
        log_error "Failed to build UI image"
        exit 1
    }
fi
log_success "UI image built: ${UI_IMG}"

log_info "📤 Pushing UI image..."
docker push "${UI_IMG}" || {
    log_error "Failed to push UI image"
    exit 1
}
log_success "UI image pushed: ${UI_IMG}"

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
    # Verify the architecture-specific tag exists
    log_info "Verifying translation-runner image exists locally..."
    if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${RUNNER_IMG}$"; then
        log_info "Architecture-specific tag not found, checking for latest tag..."
        if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${RUNNER_IMG%:*}:latest$"; then
            log_info "Tagging from latest..."
            docker tag "${RUNNER_IMG%:*}:latest" "${RUNNER_IMG}" || {
                log_error "Failed to tag translation-runner from latest"
                log_info "Available translation-runner images:"
                docker images | grep "glooscap-translation-runner" || log_warn "No translation-runner images found"
                exit 1
            }
        else
            log_error "Translation-runner image not found locally"
            log_info "Available translation-runner images:"
            docker images | grep "glooscap-translation-runner" || log_warn "No translation-runner images found"
            exit 1
        fi
    fi
    log_success "Verified translation-runner image exists: ${RUNNER_IMG}"
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
    # Patch UI deployment to use architecture-specific image tag
    TEMP_UI_DEPLOYMENT=$(mktemp)
    cp "${SCRIPT_DIR}/manifests/ui/deployment.yaml" "${TEMP_UI_DEPLOYMENT}"
    # Update image tags to match detected architecture
    sed -i.bak "s|:local-arm64|:local-${ARCH_TAG}|g" "${TEMP_UI_DEPLOYMENT}"
    sed -i.bak "s|:local-amd64|:local-${ARCH_TAG}|g" "${TEMP_UI_DEPLOYMENT}"
    kubectl apply -f "${TEMP_UI_DEPLOYMENT}"
    rm -f "${TEMP_UI_DEPLOYMENT}" "${TEMP_UI_DEPLOYMENT}.bak"
    log_success "UI deployed with architecture-specific tag (${ARCH_TAG})"
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
# First check if the deployment exists
if ! kubectl get deployment operator-controller-manager -n "${NAMESPACE}" &>/dev/null; then
    log_error "Operator deployment 'operator-controller-manager' not found in namespace '${NAMESPACE}'"
    log_info "Checking what deployments exist in namespace:"
    kubectl get deployments -n "${NAMESPACE}" || true
    log_info "Checking what pods exist in namespace:"
    kubectl get pods -n "${NAMESPACE}" || true
    log_error "Cannot proceed without operator deployment"
    exit 1
fi

log_info "Operator deployment found, waiting for it to become available..."
if kubectl wait --for=condition=available --timeout=10s deployment/operator-controller-manager -n "${NAMESPACE}" 2>/dev/null; then
    log_success "Operator is ready"
else
    log_info "Waiting for operator to become ready (this may take a moment)..."
    kubectl wait --for=condition=available --timeout=300s deployment/operator-controller-manager -n "${NAMESPACE}" || {
        log_warn "Operator deployment may not be ready yet"
        log_info "Check status with: kubectl get pods -n ${NAMESPACE}"
        log_info "Check deployment: kubectl describe deployment operator-controller-manager -n ${NAMESPACE}"
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

