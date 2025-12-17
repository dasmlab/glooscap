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

# Helper function to source GitHub token from standard locations
# Tries: /Users/dasm/gh_token, ~/gh-pat (script), ~/gh-pat/token (file), then checks env var
source_gh_token() {
    # Try /Users/dasm/gh_token (primary location)
    if [ -f "/Users/dasm/gh_token" ]; then
        log_info "🔑 Reading token from /Users/dasm/gh_token..."
        export DASMLAB_GHCR_PAT="$(cat "/Users/dasm/gh_token" | tr -d '\n\r ')"
    # Try ~/gh-pat (bash script that exports DASMLAB_GHCR_PAT)
    elif [ -f "${HOME}/gh-pat" ]; then
        log_info "🔑 Sourcing ${HOME}/gh-pat for credentials..."
        source "${HOME}/gh-pat" || log_warn "Failed to source ${HOME}/gh-pat"
    # Try ~/gh-pat/token (plain token file)
    elif [ -f "${HOME}/gh-pat/token" ]; then
        log_info "🔑 Reading token from ${HOME}/gh-pat/token..."
        export DASMLAB_GHCR_PAT="$(cat "${HOME}/gh-pat/token" | tr -d '\n\r ')"
    fi
    
    # Verify token is set
    if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
        return 1
    fi
    return 0
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
if [ -f "${SCRIPT_DIR}/scripts/undeploy-glooscap.sh" ]; then
    DELETE_NAMESPACE="${DELETE_NAMESPACE}" bash "${SCRIPT_DIR}/scripts/undeploy-glooscap.sh" || {
        log_warn "Undeploy failed (may not be deployed)"
    }
else
    log_warn "undeploy-glooscap.sh not found, skipping undeploy"
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
            echo "   ⚠️  Warning: Namespace still exists after ${MAX_WAIT}s, proceeding anyway..."
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

# Step 2: Set architecture-specific image tags
log_step "Step 2: Setting architecture-specific image tags"

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

# Step 3: Build and push operator and UI images using build-and-load-images.sh
log_step "Step 3: Building and pushing operator and UI images"

# Source GitHub token from standard locations
if ! source_gh_token; then
    log_error "DASMLAB_GHCR_PAT is required to push images to registry"
    log_info "Set it via one of:"
    log_info "  1. export DASMLAB_GHCR_PAT=your_token"
    log_info "  2. Create ~/gh-pat file with: export DASMLAB_GHCR_PAT=your_token"
    log_info "  3. Create ~/gh-pat/token file with just the token"
    exit 1
fi

log_info "🏗️  Building and pushing images using build-and-load-images.sh..."
if [ -f "${SCRIPT_DIR}/scripts/build-and-load-images.sh" ]; then
    bash "${SCRIPT_DIR}/scripts/build-and-load-images.sh" || {
        log_error "Failed to build and push images"
        exit 1
    }
    log_success "Images built and pushed: ${OPERATOR_IMG}, ${UI_IMG}"
else
    log_error "build-and-load-images.sh not found at ${SCRIPT_DIR}/scripts/build-and-load-images.sh"
    exit 1
fi

# Step 4: Generate manifests and build installer (after images are built)
log_step "Step 4: Generating manifests and building installer"

cd "${OPERATOR_DIR}"

echo "🔧 Generating manifests..."
make generate
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

# Patch the generated install.yaml to use architecture-specific tags and LoadBalancer for API service
log_info "Patching install.yaml with architecture-specific image tags, LoadBalancer service, and Always pull policy..."

# Patch operator image (may be controller:latest or ghcr.io/dasmlab/glooscap:latest)
sed -i.bak "s|image:.*controller.*|image: ${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap:latest|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|controller:latest|${OPERATOR_IMG}|g" "${OPERATOR_DIR}/dist/install.yaml"

# Set imagePullPolicy to Always to ensure latest images are pulled
# First, replace any existing imagePullPolicy
perl -i.bak -pe 's/imagePullPolicy:.*/imagePullPolicy: Always/g' "${OPERATOR_DIR}/dist/install.yaml" 2>/dev/null || \
sed -i.bak 's|imagePullPolicy:.*|imagePullPolicy: Always|g' "${OPERATOR_DIR}/dist/install.yaml"

# Fix any cases where imagePullPolicy was added without a newline (from previous sed issues)
perl -i.bak -pe 's/imagePullPolicy: Always\s+livenessProbe:/imagePullPolicy: Always\n        livenessProbe:/g' "${OPERATOR_DIR}/dist/install.yaml" 2>/dev/null || \
sed -i.bak 's|imagePullPolicy: Always[[:space:]]*livenessProbe:|imagePullPolicy: Always\
        livenessProbe:|g' "${OPERATOR_DIR}/dist/install.yaml"

# Add imagePullPolicy if it doesn't exist after image line (use awk for reliable insertion)
if ! grep -A 1 "image:.*glooscap-operator" "${OPERATOR_DIR}/dist/install.yaml" | grep -q "imagePullPolicy:"; then
    awk '/image:.*glooscap-operator/ {print; print "        imagePullPolicy: Always"; next}1' "${OPERATOR_DIR}/dist/install.yaml" > "${OPERATOR_DIR}/dist/install.yaml.tmp" && \
    mv "${OPERATOR_DIR}/dist/install.yaml.tmp" "${OPERATOR_DIR}/dist/install.yaml"
fi

# Patch operator API service to use LoadBalancer (for k3d external access)
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
    /^  selector:$/i\
  type: LoadBalancer
  }
}' "${OPERATOR_DIR}/dist/install.yaml"

# Patch translation-runner image
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:latest|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:local-arm64|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|ghcr.io/dasmlab/glooscap-translation-runner:local-amd64|ghcr.io/dasmlab/glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
# Also replace image names without registry (for backwards compatibility)
sed -i.bak "s|glooscap-translation-runner:latest|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:local-arm64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
sed -i.bak "s|glooscap-translation-runner:local-amd64|glooscap-translation-runner:local-${ARCH_TAG}|g" "${OPERATOR_DIR}/dist/install.yaml"
rm -f "${OPERATOR_DIR}/dist/install.yaml.bak"
log_success "install.yaml patched with architecture-specific tags and LoadBalancer service"

# Step 5: Create namespace and registry secret
log_step "Step 4: Creating namespace and registry secret"

if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
    log_info "Namespace ${NAMESPACE} already exists"
else
    log_info "Creating namespace ${NAMESPACE}..."
    kubectl create namespace "${NAMESPACE}"
    log_success "Namespace created"
fi

log_info "🔐 Creating registry secret..."
# Ensure token is available (should already be set from earlier, but double-check)
if ! source_gh_token; then
    log_warn "DASMLAB_GHCR_PAT not available, skipping registry secret creation"
else
    # Use the script from scripts directory (handles token sourcing itself)
    if [ -f "${SCRIPT_DIR}/scripts/create-registry-secret.sh" ]; then
        bash "${SCRIPT_DIR}/scripts/create-registry-secret.sh" "${NAMESPACE}" || {
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
fi

log_success "Registry secret ensured"

# Step 5: Deploy operator using generated install.yaml
log_step "Step 5: Deploying operator from dist/install.yaml"

log_info "Applying dist/install.yaml (includes CRDs and operator deployment)..."
kubectl apply -f "${OPERATOR_DIR}/dist/install.yaml"

log_info "⏳ Waiting for CRDs to be registered..."
sleep 5

# Force rollout restart to ensure new image is pulled (even with Always policy, restart ensures fresh start)
log_info "🔄 Restarting operator deployment to pull latest image..."
kubectl rollout restart deployment/operator-controller-manager -n "${NAMESPACE}" 2>/dev/null || {
    log_warn "Failed to restart deployment (may not exist yet, will restart when ready)"
}

log_success "Operator deployed from dist/install.yaml"

# Patch the operator API service to use LoadBalancer (if not already patched in install.yaml)
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

# Step 7: Build and push translation-runner image
log_step "Step 7: Building and pushing translation-runner image"
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

# Step 8: Deploy UI
log_step "Step 8: Deploying UI"

log_info "Applying UI manifests..."
if [ -f "${SCRIPT_DIR}/manifests/ui/deployment.yaml" ]; then
    # Patch UI deployment to use Always pull policy
    log_info "Patching UI deployment with Always pull policy..."
    kubectl apply -f "${SCRIPT_DIR}/manifests/ui/"
    kubectl patch deployment glooscap-ui -n "${NAMESPACE}" -p '{"spec":{"template":{"spec":{"containers":[{"name":"glooscap-ui","imagePullPolicy":"Always"}]}}}}' 2>/dev/null || {
        log_warn "Failed to patch UI deployment pull policy (may not exist yet)"
    }
    # Restart UI to pull latest image
    kubectl rollout restart deployment/glooscap-ui -n "${NAMESPACE}" 2>/dev/null || {
        log_warn "Failed to restart UI deployment (may not exist yet)"
    }
    log_success "UI deployed"
else
    log_warn "UI manifests not found at ${SCRIPT_DIR}/manifests/ui/"
fi

# Step 9: Deploy WikiTarget (if exists)
log_step "Step 9: Deploying WikiTarget"

sleep 3  # Brief pause for operator to be ready

if [ -f "${SCRIPT_DIR}/manifests/wikitarget.yaml" ]; then
    log_info "Deploying WikiTarget..."
    kubectl apply -f "${SCRIPT_DIR}/manifests/wikitarget.yaml"
    log_success "WikiTarget deployed"
else
    log_info "WikiTarget manifest not found (skipping)"
fi

# Step 10: Deploy TranslationService (if exists)
log_step "Step 10: Deploying TranslationService"

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

