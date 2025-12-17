#!/usr/bin/env bash
# install_glooscap.sh
# User installation script - uses released/pre-built container images
# Sets up everything needed to run Glooscap on macOS: dependencies, cluster, and deployment
# Supports optional plugins: --plugins iskoces,nokomis or --all
# For developers building from source, use dev_install_glooscap.sh instead

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMP_PLUGIN_DIR="${HOME}/.glooscap-plugins"

# Image version/tag to use (defaults to 'released', can be overridden with GLOOSCAP_VERSION env var)
# The 'released' tag represents the latest release images pushed by release_images.sh
GLOOSCAP_VERSION="${GLOOSCAP_VERSION:-released}"

# Plugin repository URLs (adjust as needed)
PLUGIN_REPOS=(
    "iskoces:https://github.com/dasmlab/iskoces.git"
    "nokomis:https://github.com/dasmlab/nokomis.git"
    # nanabush is excluded - not suitable for macOS dev
)

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

# Parse arguments
PLUGINS=""
ALL_PLUGINS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --plugins)
            PLUGINS="$2"
            shift 2
            ;;
        --all)
            ALL_PLUGINS=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            log_info "Usage: $0 [--plugins iskoces,nokomis] [--all]"
            exit 1
            ;;
    esac
done

# Determine which plugins to install
SELECTED_PLUGINS=()  # Initialize as empty array
if [ "${ALL_PLUGINS}" = "true" ]; then
    # --all includes all plugins except nanabush
    for plugin_repo in "${PLUGIN_REPOS[@]}"; do
        plugin_name="${plugin_repo%%:*}"
        SELECTED_PLUGINS+=("${plugin_name}")
    done
    log_info "Installing all plugins: ${SELECTED_PLUGINS[*]}"
elif [ -n "${PLUGINS}" ]; then
    # Parse comma-separated list
    IFS=',' read -ra PLUGIN_LIST <<< "${PLUGINS}"
    for plugin in "${PLUGIN_LIST[@]}"; do
        plugin=$(echo "${plugin}" | xargs) # trim whitespace
        # Validate plugin name
        valid=false
        for plugin_repo in "${PLUGIN_REPOS[@]}"; do
            if [ "${plugin_repo%%:*}" = "${plugin}" ]; then
                valid=true
                break
            fi
        done
        if [ "${valid}" = "true" ]; then
            SELECTED_PLUGINS+=("${plugin}")
        else
            log_warn "Unknown plugin: ${plugin} (skipping)"
        fi
    done
    if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
        log_info "Installing plugins: ${SELECTED_PLUGINS[*]}"
    fi
fi

# Check if running on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    log_error "This script is designed for macOS only"
    log_info "For Linux, see: infra/linux-docker/"
    exit 1
fi

log_info "Glooscap Installation for macOS (User Mode)"
log_info "This will set up everything needed to run Glooscap locally using released images"
log_info "Using image version: ${GLOOSCAP_VERSION}"
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
    log_info "Plugins to install: ${SELECTED_PLUGINS[*]}"
else
    log_info "No plugins specified - installing Glooscap only"
fi
echo ""

# Step 1: Setup environment
log_step "Step 1: Setting up macOS environment"
if ! bash "${SCRIPT_DIR}/scripts/setup-macos-env.sh"; then
    log_error "Environment setup failed"
    exit 1
fi

# Step 2: Start Kubernetes cluster
log_step "Step 2: Starting Kubernetes cluster (k3d)"
if ! bash "${SCRIPT_DIR}/scripts/start-k3d.sh"; then
    log_error "Failed to start Kubernetes cluster"
    exit 1
fi

# Step 3: Create registry credentials (if token provided)
if [ -n "${DASMLAB_GHCR_PAT:-}" ]; then
    log_step "Step 3: Creating registry credentials"
    if ! bash "${SCRIPT_DIR}/scripts/create-registry-secret.sh"; then
        log_warn "Registry secret creation failed (images may not pull from registry)"
        log_info "Continuing anyway..."
    fi
else
    log_warn "DASMLAB_GHCR_PAT not set - skipping registry secret creation"
    log_info "If you need to pull images from ghcr.io, set: export DASMLAB_GHCR_PAT=your_token"
    log_info "Note: Public images may still be pullable without authentication"
fi

# Step 4: Deploy Glooscap (using released images)
log_step "Step 4: Deploying Glooscap (using released images)"
log_info "Deploying with released images (version: ${GLOOSCAP_VERSION})"
log_info "Skipping build step - using pre-built images from registry"

# Export version for deploy script
export GLOOSCAP_VERSION="${GLOOSCAP_VERSION}"
export USE_RELEASED_IMAGES=true

if ! bash "${SCRIPT_DIR}/scripts/deploy-glooscap-released.sh"; then
    log_error "Failed to deploy Glooscap"
    exit 1
fi

# Step 5: Install plugins (if any) - using released images
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
    log_step "Step 5: Installing plugins (using released images)"
    
    # Create temp directory for plugin repos (for manifests only, not building)
    mkdir -p "${TEMP_PLUGIN_DIR}"
    
    for plugin in "${SELECTED_PLUGINS[@]}"; do
        log_info "Installing plugin: ${plugin} (using released images)"
        
        # Find plugin repo URL
        PLUGIN_REPO=""
        for plugin_repo in "${PLUGIN_REPOS[@]}"; do
            if [ "${plugin_repo%%:*}" = "${plugin}" ]; then
                PLUGIN_REPO="${plugin_repo#*:}"
                break
            fi
        done
        
        if [ -z "${PLUGIN_REPO}" ]; then
            log_warn "Plugin ${plugin} not found in repository list (skipping)"
            continue
        fi
        
        PLUGIN_DIR="${TEMP_PLUGIN_DIR}/${plugin}"
        
        # Clone or update plugin repo (for manifests only)
        if [ -d "${PLUGIN_DIR}" ]; then
            log_info "Updating existing plugin repo: ${plugin}"
            cd "${PLUGIN_DIR}"
            git pull origin main || {
                log_warn "Failed to update ${plugin} repo, trying fresh clone..."
                rm -rf "${PLUGIN_DIR}"
            }
        fi
        
        if [ ! -d "${PLUGIN_DIR}" ]; then
            log_info "Cloning plugin repo: ${plugin} (for manifests only)"
            git clone "${PLUGIN_REPO}" "${PLUGIN_DIR}" || {
                log_error "Failed to clone ${plugin} repository"
                continue
            }
        fi
        
        # Check if plugin has infra/macos-foss directory
        PLUGIN_INFRA_DIR="${PLUGIN_DIR}/infra/macos-foss"
        if [ ! -d "${PLUGIN_INFRA_DIR}" ]; then
            log_warn "Plugin ${plugin} does not have infra/macos-foss directory (skipping)"
            continue
        fi
        
        # Ensure registry secret exists in plugin namespace (if it has one)
        log_info "Ensuring registry secret exists for ${plugin}..."
        PLUGIN_NAMESPACE="${plugin}"
        if kubectl get namespace "${PLUGIN_NAMESPACE}" &>/dev/null; then
            # Copy secret from glooscap-system if it exists
            if kubectl get secret dasmlab-ghcr-pull -n glooscap-system &>/dev/null; then
                kubectl get secret dasmlab-ghcr-pull -n glooscap-system -o yaml | \
                    sed "s/namespace: glooscap-system/namespace: ${PLUGIN_NAMESPACE}/" | \
                    kubectl apply -f - || log_warn "Failed to copy registry secret to ${PLUGIN_NAMESPACE}"
            fi
        fi
        
        # Deploy plugin using released images (skip build step)
        DEPLOY_SCRIPT=""
        if [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy-${plugin}-released.sh" ]; then
            DEPLOY_SCRIPT="scripts/deploy-${plugin}-released.sh"
        elif [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy-${plugin}.sh" ]; then
            # Check if deploy script supports USE_RELEASED_IMAGES
            if grep -q "USE_RELEASED_IMAGES\|GLOOSCAP_VERSION" "${PLUGIN_INFRA_DIR}/scripts/deploy-${plugin}.sh"; then
                DEPLOY_SCRIPT="scripts/deploy-${plugin}.sh"
                export USE_RELEASED_IMAGES=true
                export GLOOSCAP_VERSION="${GLOOSCAP_VERSION}"
            else
                log_warn "Plugin ${plugin} deploy script does not support released images"
                log_info "Trying to deploy anyway (may fail if images not available)..."
                DEPLOY_SCRIPT="scripts/deploy-${plugin}.sh"
            fi
        elif [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy.sh" ]; then
            # Check if deploy script supports USE_RELEASED_IMAGES
            if grep -q "USE_RELEASED_IMAGES\|GLOOSCAP_VERSION" "${PLUGIN_INFRA_DIR}/scripts/deploy.sh"; then
                DEPLOY_SCRIPT="scripts/deploy.sh"
                export USE_RELEASED_IMAGES=true
                export GLOOSCAP_VERSION="${GLOOSCAP_VERSION}"
            else
                log_warn "Plugin ${plugin} deploy script does not support released images"
                log_info "Trying to deploy anyway (may fail if images not available)..."
                DEPLOY_SCRIPT="scripts/deploy.sh"
            fi
        fi
        
        if [ -n "${DEPLOY_SCRIPT}" ]; then
            log_info "Deploying ${plugin} with released images (version: ${GLOOSCAP_VERSION})..."
            cd "${PLUGIN_INFRA_DIR}"
            if ! bash "${DEPLOY_SCRIPT}"; then
                log_warn "Failed to deploy ${plugin} (continuing)"
                continue
            fi
            log_success "Plugin ${plugin} deployed"
        else
            log_warn "Plugin ${plugin} does not have deploy script (skipping deployment)"
        fi
    done
fi

# Success!
echo ""
log_success "Glooscap installation complete!"
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
    log_success "Plugins installed: ${SELECTED_PLUGINS[*]}"
fi
echo ""
log_info "Access the services directly on host ports:"
echo "  UI: http://localhost:8080"
echo "  Operator API: http://localhost:3000"
echo ""
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ] && [[ " ${SELECTED_PLUGINS[*]} " =~ " iskoces " ]]; then
    log_info "Iskoces service: iskoces-service.iskoces.svc:50051"
    log_info "Configure in Glooscap UI: Settings → Translation Service"
fi
echo ""
log_info "View logs:"
echo "  Operator: kubectl logs -f -n glooscap-system deployment/operator-controller-manager"
echo "  UI: kubectl logs -f -n glooscap-system deployment/glooscap-ui"
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ] && [[ " ${SELECTED_PLUGINS[*]} " =~ " iskoces " ]]; then
    echo "  Iskoces: kubectl logs -f -n iskoces deployment/iskoces-server"
fi
echo ""
log_info "To uninstall, run: ./uninstall_glooscap.sh"
echo ""
log_info "Note: This installation used released images (version: ${GLOOSCAP_VERSION})"
log_info "For development builds, use: ./dev_install_glooscap.sh"
echo ""

