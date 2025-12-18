#!/usr/bin/env bash
# test-public-images.sh
# Tests if Glooscap images can be pulled without authentication (i.e., are public)
# This helps determine if ImagePullSecrets are needed

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

log_info "Testing if Glooscap images are publicly accessible..."
echo ""

# List of images to test
IMAGES=(
    "ghcr.io/dasmlab/glooscap-operator:released"
    "ghcr.io/dasmlab/glooscap-ui:released"
    "ghcr.io/dasmlab/glooscap-translation-runner:released"
    "ghcr.io/dasmlab/iskoces-server:released"
)

# Logout from Docker to ensure we're not using cached credentials
log_info "Logging out from Docker to test public access..."
docker logout ghcr.io 2>/dev/null || true

PUBLIC_COUNT=0
PRIVATE_COUNT=0

for IMAGE in "${IMAGES[@]}"; do
    log_info "Testing: ${IMAGE}"
    
    # Try to pull without authentication
    if docker pull "${IMAGE}" >/dev/null 2>&1; then
        log_success "✓ ${IMAGE} is PUBLIC (can be pulled without authentication)"
        PUBLIC_COUNT=$((PUBLIC_COUNT + 1))
        # Remove the image to clean up
        docker rmi "${IMAGE}" >/dev/null 2>&1 || true
    else
        log_error "✗ ${IMAGE} is PRIVATE (requires authentication)"
        PRIVATE_COUNT=$((PRIVATE_COUNT + 1))
    fi
done

echo ""
log_info "Summary:"
echo "  Public images: ${PUBLIC_COUNT}"
echo "  Private images: ${PRIVATE_COUNT}"
echo ""

if [ ${PRIVATE_COUNT} -eq 0 ]; then
    log_success "All images are public! ImagePullSecrets are NOT required."
    echo ""
    log_info "You can update the install script to make ImagePullSecrets optional."
elif [ ${PUBLIC_COUNT} -eq 0 ]; then
    log_warn "All images are private. ImagePullSecrets ARE required."
    echo ""
    log_info "Users will need to provide GitHub token during installation."
else
    log_warn "Mixed visibility: Some images are public, some are private."
    echo ""
    log_info "ImagePullSecrets may still be needed for private images."
fi

echo ""
log_info "To make images public, run:"
echo "  ./scripts/make-images-public.sh"
echo ""

