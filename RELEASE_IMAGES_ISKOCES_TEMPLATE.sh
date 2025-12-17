#!/usr/bin/env bash
# release_images.sh (Iskoces version)
# Builds and pushes Iskoces images with the :released tag
# This tag represents the latest release and is used by the user install script
# Run this script manually when creating a new release
#
# NOTE: This is a template for Iskoces. Adjust the image names and build process
#       to match your Iskoces repository structure.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory (project root - adjust for Iskoces repo structure)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}"

# Release tag
RELEASE_TAG="released"

# Registry configuration
REGISTRY="ghcr.io/dasmlab"
# Adjust image name to match Iskoces image name (e.g., iskoces-server, iskoces, etc.)
ISKOCES_IMG="${REGISTRY}/iskoces-server:${RELEASE_TAG}"

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

log_step "Building and pushing Iskoces release images"
log_info "Release tag: ${RELEASE_TAG}"
log_info "This will build and push Iskoces images with the '${RELEASE_TAG}' tag"
log_info "These images will be used by the user install script (install_glooscap.sh --plugins iskoces)"
echo ""

# Build Iskoces image
log_step "Building Iskoces image"
log_info "Building Iskoces image..."

# Adjust the build directory and method based on Iskoces structure
# Example patterns (choose the one that matches your Iskoces repo):
#
# Pattern 1: If Iskoces has a buildme.sh script
if [ -f "${PROJECT_ROOT}/buildme.sh" ]; then
    cd "${PROJECT_ROOT}"
    ./buildme.sh || {
        log_error "Failed to build Iskoces image"
        exit 1
    }
    # Retag from scratch to released (adjust scratch tag name if different)
    docker tag iskoces-server:scratch "${ISKOCES_IMG}" || {
        log_error "Failed to tag Iskoces image"
        exit 1
    }
# Pattern 2: If Iskoces uses Makefile
elif [ -f "${PROJECT_ROOT}/Makefile" ]; then
    cd "${PROJECT_ROOT}"
    make docker-build IMG="${ISKOCES_IMG}" || {
        log_error "Failed to build Iskoces image"
        exit 1
    }
# Pattern 3: If Iskoces has a Dockerfile in root
elif [ -f "${PROJECT_ROOT}/Dockerfile" ]; then
    cd "${PROJECT_ROOT}"
    docker build -t "${ISKOCES_IMG}" . || {
        log_error "Failed to build Iskoces image"
        exit 1
    }
# Pattern 4: If Iskoces has infra/macos-foss/scripts/build-and-load-images.sh
elif [ -f "${PROJECT_ROOT}/infra/macos-foss/scripts/build-and-load-images.sh" ]; then
    cd "${PROJECT_ROOT}/infra/macos-foss"
    # Build with released tag (may need to modify the script or pass env var)
    bash scripts/build-and-load-images.sh || {
        log_error "Failed to build Iskoces image"
        exit 1
    }
    # Retag to released (adjust based on what the script creates)
    # Example: docker tag "${REGISTRY}/iskoces-server:local-arm64" "${ISKOCES_IMG}"
else
    log_error "Could not find Iskoces build script or Dockerfile"
    log_info "Please adjust this script to match your Iskoces build process"
    log_info "Expected locations:"
    log_info "  - ${PROJECT_ROOT}/buildme.sh"
    log_info "  - ${PROJECT_ROOT}/Makefile"
    log_info "  - ${PROJECT_ROOT}/Dockerfile"
    log_info "  - ${PROJECT_ROOT}/infra/macos-foss/scripts/build-and-load-images.sh"
    exit 1
fi

log_success "Iskoces image built: ${ISKOCES_IMG}"

# Push Iskoces image
log_info "Pushing Iskoces image to registry..."
docker push "${ISKOCES_IMG}" || {
    log_error "Failed to push Iskoces image"
    exit 1
}
log_success "Iskoces image pushed: ${ISKOCES_IMG}"

# Success summary
log_step "Release images pushed successfully!"
log_success "Iskoces image has been built and pushed with the '${RELEASE_TAG}' tag"
echo ""
log_info "Released image:"
echo "  - ${ISKOCES_IMG}"
echo ""
log_info "This image is now available for use by:"
echo "  - install_glooscap.sh --plugins iskoces (user installation script)"
echo "  - Any deployment using ISKOCES_VERSION=released"
echo ""
log_info "To verify, you can check the registry:"
echo "  docker pull ${ISKOCES_IMG}"
echo ""

