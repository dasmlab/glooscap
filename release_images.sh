#!/usr/bin/env bash
# release_images.sh
# Builds and pushes Glooscap images with the :released tag
# This tag represents the latest release and is used by the user install script
# Run this script manually when creating a new release

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory (project root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}"

# Release tag
RELEASE_TAG="released"

# Registry configuration
REGISTRY="ghcr.io/dasmlab"
OPERATOR_IMG="${REGISTRY}/glooscap-operator:${RELEASE_TAG}"
UI_IMG="${REGISTRY}/glooscap-ui:${RELEASE_TAG}"
RUNNER_IMG="${REGISTRY}/glooscap-translation-runner:${RELEASE_TAG}"

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

# Check for GitHub token (try to source from standard locations if not set)
if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
    # Try /Users/dasm/gh_token (primary location for macOS)
    if [ -f "/Users/dasm/gh_token" ]; then
        export DASMLAB_GHCR_PAT="$(cat "/Users/dasm/gh_token" | tr -d '\n\r ')"
    # Try ~/gh-pat (bash script)
    elif [ -f "${HOME}/gh-pat" ]; then
        source "${HOME}/gh-pat" 2>/dev/null || true
    # Try ~/gh-pat/token (plain token file)
    elif [ -f "${HOME}/gh-pat/token" ]; then
        export DASMLAB_GHCR_PAT="$(cat "${HOME}/gh-pat/token" | tr -d '\n\r ')"
    fi
fi

GHCR_PAT="${DASMLAB_GHCR_PAT:-}"
if [ -z "${GHCR_PAT}" ]; then
    log_error "DASMLAB_GHCR_PAT environment variable is required"
    log_info "Set it via one of:"
    log_info "  1. export DASMLAB_GHCR_PAT=your_token"
    log_info "  2. Create ~/gh-pat file with: export DASMLAB_GHCR_PAT=your_token"
    log_info "  3. Create ~/gh-pat/token file with just the token"
    log_info "  4. Create /Users/dasm/gh_token file with just the token"
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

log_step "Building and pushing Glooscap release images"
log_info "Release tag: ${RELEASE_TAG}"
log_info "This will build and push all Glooscap images with the '${RELEASE_TAG}' tag"
log_info "These images will be used by the user install script (install_glooscap.sh)"
echo ""

# Build operator image
log_step "Building operator image"
log_info "Building operator image..."
cd "${PROJECT_ROOT}/operator"

# Build with Makefile (uses docker-build target)
log_info "Using make docker-build..."
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
log_step "Building UI image"
log_info "Building UI image..."
cd "${PROJECT_ROOT}/ui"

# Use buildme.sh to build, then retag
if [ -f "./buildme.sh" ]; then
    log_info "Using buildme.sh to build UI..."
    ./buildme.sh || {
        log_error "Failed to build UI image"
        exit 1
    }
    # Retag from scratch to released
    docker tag glooscap-ui:scratch "${UI_IMG}" || {
        log_error "Failed to tag UI image"
        exit 1
    }
else
    log_error "buildme.sh not found in ui directory"
    exit 1
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
log_step "Building translation-runner image"
log_info "Building translation-runner image..."
cd "${PROJECT_ROOT}"

# Validate translation-runner directory
if [ ! -d "${PROJECT_ROOT}/translation-runner" ]; then
    log_error "translation-runner directory not found at: ${PROJECT_ROOT}/translation-runner"
    exit 1
fi

BUILD_SCRIPT="${PROJECT_ROOT}/translation-runner/build.sh"
if [ -f "${BUILD_SCRIPT}" ]; then
    log_info "Using translation-runner build script..."
    # Build with released tag
    bash "${BUILD_SCRIPT}" "${RELEASE_TAG}" || {
        log_error "Failed to build translation-runner image"
        exit 1
    }
    # Verify the image exists
    if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${RUNNER_IMG}$"; then
        log_error "Translation-runner image not found after build: ${RUNNER_IMG}"
        log_info "Available translation-runner images:"
        docker images | grep "glooscap-translation-runner" || log_warn "No translation-runner images found"
        exit 1
    fi
    log_success "Translation-runner image built: ${RUNNER_IMG}"
else
    log_error "translation-runner/build.sh not found at: ${BUILD_SCRIPT}"
    exit 1
fi

# Push translation-runner image
log_info "Pushing translation-runner image to registry..."
docker push "${RUNNER_IMG}" || {
    log_error "Failed to push translation-runner image"
    exit 1
}
log_success "Translation-runner image pushed: ${RUNNER_IMG}"

# Success summary
log_step "Release images pushed successfully!"
log_success "All Glooscap images have been built and pushed with the '${RELEASE_TAG}' tag"
echo ""
log_info "Released images:"
echo "  - ${OPERATOR_IMG}"
echo "  - ${UI_IMG}"
echo "  - ${RUNNER_IMG}"
echo ""
log_info "These images are now available for use by:"
echo "  - install_glooscap.sh (user installation script)"
echo "  - Any deployment using GLOOSCAP_VERSION=released"
echo ""
log_info "To verify, you can check the registry:"
echo "  docker pull ${OPERATOR_IMG}"
echo "  docker pull ${UI_IMG}"
echo "  docker pull ${RUNNER_IMG}"
echo ""

