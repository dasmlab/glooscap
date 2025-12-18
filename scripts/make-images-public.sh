#!/usr/bin/env bash
# make-images-public.sh
# Makes all Glooscap container images public on GitHub Container Registry
# Requires GitHub CLI (gh) to be installed and authenticated

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

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    log_error "GitHub CLI (gh) is not installed"
    log_info "Install it with: brew install gh"
    log_info "Then authenticate with: gh auth login"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    log_error "Not authenticated with GitHub CLI"
    log_info "Run: gh auth login"
    exit 1
fi

log_info "Making Glooscap container images public on GitHub Container Registry..."
echo ""

# List of images to make public
IMAGES=(
    "glooscap-operator"
    "glooscap-ui"
    "glooscap-translation-runner"
    "iskoces-server"
)

OWNER="dasmlab"

for IMAGE in "${IMAGES[@]}"; do
    PACKAGE="${OWNER}/${IMAGE}"
    log_info "Making ${PACKAGE} public..."
    
    # Try organization-level API first
    log_info "Attempting to change visibility via API..."
    API_RESPONSE=$(gh api \
        -X PATCH \
        "orgs/${OWNER}/packages/container/${IMAGE}" \
        -f visibility=public \
        2>&1) || {
        API_ERROR=$?
        log_warn "API call failed (exit code: ${API_ERROR})"
        log_info "Response: ${API_RESPONSE}"
        log_info ""
        log_info "Trying alternative: user-level package..."
        # Try user-level (if package was pushed by user, not org)
        USER=$(gh api user --jq .login 2>/dev/null || echo "")
        if [ -n "${USER}" ]; then
            if gh api \
                -X PATCH \
                "users/${USER}/packages/container/${IMAGE}" \
                -f visibility=public \
                &>/dev/null 2>&1; then
                log_success "✓ ${PACKAGE} is now public (user-level)"
                continue
            fi
        fi
        log_error "✗ Failed to make ${PACKAGE} public via API"
        log_info ""
        log_info "Manual steps required:"
        log_info "  1. Go to: https://github.com/orgs/${OWNER}/packages/container/${IMAGE}"
        log_info "  2. Click 'Package settings' (if visible)"
        log_info "  3. Look for 'Change visibility' or 'Danger Zone'"
        log_info "  4. Or check organization settings: https://github.com/orgs/${OWNER}/settings/packages"
        log_info ""
        continue
    }
    
    log_success "✓ ${PACKAGE} is now public"
done

echo ""
log_success "Done! All images should now be public."
log_info "Verify at: https://github.com/orgs/${OWNER}/packages"
echo ""
log_info "Note: If the API call fails, you can make images public manually:"
log_info "  1. Go to https://github.com/orgs/${OWNER}/packages"
log_info "  2. Click on each package"
log_info "  3. Go to Package settings → Change visibility → Public"
echo ""

