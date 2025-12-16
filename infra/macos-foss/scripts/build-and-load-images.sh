#!/usr/bin/env bash
# build-and-load-images.sh
# Builds Glooscap operator and UI images for the current architecture and pushes to ghcr.io
# Images are tagged with architecture-specific tags (e.g., local-arm64, local-amd64)
# This allows parallel development on different architectures

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
        log_warn "Unknown architecture: ${ARCH}, defaulting to 'unknown'"
        ARCH_TAG="unknown"
        ;;
esac

log_info "Detected architecture: ${ARCH} (tag: ${ARCH_TAG})"

# Registry configuration
REGISTRY="ghcr.io/dasmlab"
OPERATOR_IMG="${REGISTRY}/glooscap-operator:local-${ARCH_TAG}"
UI_IMG="${REGISTRY}/glooscap-ui:local-${ARCH_TAG}"
RUNNER_IMG="${REGISTRY}/glooscap-translation-runner:local-${ARCH_TAG}"

# Check for GitHub token
GHCR_PAT="${DASMLAB_GHCR_PAT:-}"
if [ -z "${GHCR_PAT}" ]; then
    log_error "DASMLAB_GHCR_PAT environment variable is required"
    log_info "Set it with: export DASMLAB_GHCR_PAT=your_token"
    log_info "The token should be a GitHub PAT with 'write:packages' permission"
    exit 1
fi

# Authenticate with GitHub Container Registry
log_info "Authenticating with GitHub Container Registry..."
echo "${GHCR_PAT}" | docker login ghcr.io -u lmcdasm --password-stdin || {
    log_error "Failed to authenticate with ghcr.io"
    exit 1
}
log_success "Authenticated with ghcr.io"

log_info "Building and pushing images for architecture: ${ARCH_TAG}..."

# Build operator image
log_info "Building operator image..."
cd "${PROJECT_ROOT}/operator"
make docker-build IMG="${OPERATOR_IMG}" || {
    log_error "Failed to build operator image"
    exit 1
}
log_success "Operator image built: ${OPERATOR_IMG}"

# Push operator image
log_info "Pushing operator image to registry..."
docker push "${OPERATOR_IMG}" || {
    log_error "Failed to push operator image"
    exit 1
}
log_success "Operator image pushed: ${OPERATOR_IMG}"

# Build UI image
log_info "Building UI image..."
cd "${PROJECT_ROOT}/ui"
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
    # Fallback to docker build (buildx may not be available with Podman)
    log_info "Using docker build (buildx not available or --load not supported)"
    docker build --tag "${UI_IMG}" . || {
        log_error "Failed to build UI image"
        exit 1
    }
fi
log_success "UI image built: ${UI_IMG}"

# Push UI image
log_info "Pushing UI image to registry..."
docker push "${UI_IMG}" || {
    log_error "Failed to push UI image"
    exit 1
}
log_success "UI image pushed: ${UI_IMG}"

# Build translation-runner image
log_info "Building translation-runner image..."
cd "${PROJECT_ROOT}"

# Validate PROJECT_ROOT and translation-runner directory
if [ ! -d "${PROJECT_ROOT}/translation-runner" ]; then
    log_error "translation-runner directory not found at: ${PROJECT_ROOT}/translation-runner"
    log_info "PROJECT_ROOT resolved to: ${PROJECT_ROOT}"
    log_info "Script directory: ${SCRIPT_DIR}"
    log_info "Current working directory: $(pwd)"
    exit 1
fi

BUILD_SCRIPT="${PROJECT_ROOT}/translation-runner/build.sh"
if [ -f "${BUILD_SCRIPT}" ]; then
    log_info "Found build script at: ${BUILD_SCRIPT}"
    # Use the build script with architecture-specific tag
    # The build script will create both the specified tag and "latest"
    bash "${BUILD_SCRIPT}" "local-${ARCH_TAG}" || {
        log_error "Failed to build translation-runner image"
        exit 1
    }
    # Ensure the architecture-specific tag exists (build.sh may have created it)
    if ! docker images | grep -q "glooscap-translation-runner.*local-${ARCH_TAG}"; then
        # Tag from latest if needed
        docker tag "${RUNNER_IMG%:*}:latest" "${RUNNER_IMG}" || {
            log_error "Failed to tag translation-runner image"
            exit 1
        }
    fi
else
    log_error "translation-runner/build.sh not found at: ${BUILD_SCRIPT}"
    log_info "PROJECT_ROOT resolved to: ${PROJECT_ROOT}"
    log_info "Script directory: ${SCRIPT_DIR}"
    log_info "Current working directory: $(pwd)"
    log_info "Contents of translation-runner directory:"
    ls -la "${PROJECT_ROOT}/translation-runner/" 2>/dev/null || log_warn "Cannot list translation-runner directory"
    exit 1
fi
log_success "Translation-runner image built: ${RUNNER_IMG}"

# Push translation-runner image
log_info "Pushing translation-runner image to registry..."
docker push "${RUNNER_IMG}" || {
    log_error "Failed to push translation-runner image"
    exit 1
}
log_success "Translation-runner image pushed: ${RUNNER_IMG}"

log_success "All images built and pushed successfully!"
log_info "Images available in registry:"
echo "  - ${OPERATOR_IMG}"
echo "  - ${UI_IMG}"
echo "  - ${RUNNER_IMG}"
echo ""
log_info "Deployment manifests should use these images:"
echo "  operator: image: ${OPERATOR_IMG}"
echo "  ui: image: ${UI_IMG}"
echo "  translation-runner (VLLM_JOB_IMAGE): ${RUNNER_IMG}"

