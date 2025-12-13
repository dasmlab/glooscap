#!/usr/bin/env bash
# uninstall_glooscap.sh
# Uninstallation script for Glooscap on OpenShift
# Removes Glooscap deployment and cleans up resources

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
TEMP_PLUGIN_DIR="${HOME}/.glooscap-plugins"

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

# Check prerequisites
if ! command -v oc &> /dev/null; then
    log_error "oc CLI not found. Please install OpenShift CLI."
    exit 1
fi

if ! oc whoami &> /dev/null; then
    log_error "Not logged in to OpenShift cluster. Please run: oc login"
    exit 1
fi

log_info "Glooscap Uninstallation for OpenShift"
log_info "This will remove Glooscap and clean up resources"
echo ""

# Confirm uninstallation
read -p "Are you sure you want to uninstall Glooscap? (y/N): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Uninstallation cancelled"
    exit 0
fi

NAMESPACE="glooscap-system"

# Step 1: Undeploy plugins (if any were installed)
if [ -d "${TEMP_PLUGIN_DIR}" ]; then
    log_step "Step 1: Removing plugins"
    
    for plugin_dir in "${TEMP_PLUGIN_DIR}"/*; do
        if [ -d "${plugin_dir}" ]; then
            plugin=$(basename "${plugin_dir}")
            log_info "Undeploying plugin: ${plugin}"
            
            PLUGIN_INFRA_DIR="${plugin_dir}/infra/openshift"
            if [ -d "${PLUGIN_INFRA_DIR}" ]; then
                # Try different script name patterns
                UNDEPLOY_SCRIPT=""
                if [ -f "${PLUGIN_INFRA_DIR}/scripts/undeploy-${plugin}.sh" ]; then
                    UNDEPLOY_SCRIPT="scripts/undeploy-${plugin}.sh"
                elif [ -f "${PLUGIN_INFRA_DIR}/scripts/undeploy.sh" ]; then
                    UNDEPLOY_SCRIPT="scripts/undeploy.sh"
                elif [ -f "${PLUGIN_INFRA_DIR}/uninstall_glooscap.sh" ]; then
                    UNDEPLOY_SCRIPT="uninstall_glooscap.sh"
                fi
                
                if [ -n "${UNDEPLOY_SCRIPT}" ]; then
                    cd "${PLUGIN_INFRA_DIR}"
                    bash "${UNDEPLOY_SCRIPT}" || log_warn "Failed to undeploy ${plugin}"
                else
                    # Try to delete namespace if it exists
                    PLUGIN_NAMESPACE="${plugin}"
                    if oc get namespace "${PLUGIN_NAMESPACE}" &>/dev/null; then
                        log_info "Deleting plugin namespace: ${PLUGIN_NAMESPACE}"
                        oc delete namespace "${PLUGIN_NAMESPACE}" --wait=true --timeout=120s || log_warn "Failed to delete namespace ${PLUGIN_NAMESPACE}"
                    fi
                fi
            fi
        fi
    done
    
    # Clean up plugin repos
    log_info "Cleaning up plugin repositories..."
    rm -rf "${TEMP_PLUGIN_DIR}"
    log_success "Plugin repositories cleaned up"
fi

# Step 2: Delete UI
log_step "Step 2: Removing UI deployment"

if [ -f "${SCRIPT_DIR}/glooscap-ui.yaml" ]; then
    log_info "Deleting UI..."
    oc delete -f "${SCRIPT_DIR}/glooscap-ui.yaml" --ignore-not-found=true || log_warn "UI deletion may have failed"
    log_success "UI removed"
else
    log_warn "UI manifest not found (skipping)"
fi

# Step 3: Delete API route
log_step "Step 3: Removing API route"

if [ -f "${SCRIPT_DIR}/operator-api-route.yaml" ]; then
    log_info "Deleting API route..."
    oc delete -f "${SCRIPT_DIR}/operator-api-route.yaml" --ignore-not-found=true || log_warn "Route deletion may have failed"
    log_success "API route removed"
else
    log_warn "API route manifest not found (skipping)"
fi

# Step 4: Delete WikiTarget
log_step "Step 4: Removing WikiTarget"

if [ -f "${SCRIPT_DIR}/wikitarget-infra-dasmlab-org.yaml" ]; then
    log_info "Deleting WikiTarget..."
    oc delete -f "${SCRIPT_DIR}/wikitarget-infra-dasmlab-org.yaml" --ignore-not-found=true || log_warn "WikiTarget deletion may have failed"
    log_success "WikiTarget removed"
else
    log_warn "WikiTarget manifest not found (skipping)"
fi

# Step 5: Undeploy operator
log_step "Step 5: Removing operator deployment"

cd "${OPERATOR_DIR}"

log_info "Undeploying operator..."
make undeploy || log_warn "Operator undeploy may have failed"

log_success "Operator undeployed"

# Step 6: Uninstall CRDs
log_step "Step 6: Removing CRDs"

log_info "Uninstalling CRDs..."
make uninstall || log_warn "CRD uninstall may have failed"

log_success "CRDs removed"

# Step 7: Delete namespace (optional - commented out by default)
# Uncomment if you want to completely remove the namespace
# log_step "Step 7: Removing namespace"
# 
# if oc get namespace "${NAMESPACE}" &>/dev/null; then
#     log_info "Deleting namespace ${NAMESPACE}..."
#     oc delete namespace "${NAMESPACE}" --wait=true --timeout=120s || log_warn "Namespace deletion may have failed"
#     log_success "Namespace removed"
# else
#     log_info "Namespace ${NAMESPACE} does not exist"
# fi

# Success!
echo ""
log_success "Glooscap uninstallation complete!"
echo ""
log_info "The following resources have been removed:"
echo "  - Operator deployment"
echo "  - UI deployment"
echo "  - API route"
echo "  - WikiTarget CR"
echo "  - CRDs"
if [ -d "${TEMP_PLUGIN_DIR}" ]; then
    echo "  - Plugin deployments"
fi
echo ""
log_info "Note: The namespace '${NAMESPACE}' was not deleted."
log_info "To completely remove it, run: oc delete namespace ${NAMESPACE}"
echo ""
log_info "To reinstall, run: ./install_glooscap.sh"
echo ""

