#!/usr/bin/env bash
# copy-crds.sh
# Copies CRDs from operator config to manifests directory

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
CRD_SOURCE="${OPERATOR_DIR}/config/crd/bases"
CRD_DEST="${SCRIPT_DIR}/../manifests/crd"

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

log_info "Copying CRDs from operator config..."

# Check if operator directory exists
if [ ! -d "${OPERATOR_DIR}" ]; then
    log_error "Operator directory not found: ${OPERATOR_DIR}"
    exit 1
fi

# Check if CRD source exists
if [ ! -d "${CRD_SOURCE}" ]; then
    log_warn "CRD source directory not found: ${CRD_SOURCE}"
    log_info "Generating CRDs first..."
    
    # Try to generate CRDs
    if [ -f "${OPERATOR_DIR}/Makefile" ]; then
        cd "${OPERATOR_DIR}"
        if command -v make &> /dev/null; then
            make manifests
        else
            log_error "make not found. Please generate CRDs manually:"
            log_info "  cd ${OPERATOR_DIR} && make manifests"
            exit 1
        fi
    else
        log_error "Cannot generate CRDs. Please run:"
        log_info "  cd ${OPERATOR_DIR} && make manifests"
        exit 1
    fi
fi

# Create destination directory
mkdir -p "${CRD_DEST}"

# Copy CRDs
if [ "$(ls -A ${CRD_SOURCE}/*.yaml 2>/dev/null)" ]; then
    cp "${CRD_SOURCE}"/*.yaml "${CRD_DEST}/"
    log_success "CRDs copied to ${CRD_DEST}"
    
    # List copied files
    echo ""
    log_info "Copied CRDs:"
    ls -1 "${CRD_DEST}"/*.yaml | xargs -n1 basename
else
    log_error "No CRD files found in ${CRD_SOURCE}"
    exit 1
fi

