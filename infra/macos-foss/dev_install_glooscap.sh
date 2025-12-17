#!/usr/bin/env bash
# dev_install_glooscap.sh
# Developer installation script - builds containers from source
# Sets up everything needed to run Glooscap on macOS: dependencies, cluster, and deployment
# Supports optional plugins: --plugins iskoces,nokomis or --all
# For end users, use install_glooscap.sh instead (uses pre-built released images)

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

log_info "Glooscap Installation for macOS"
log_info "This will set up everything needed to run Glooscap locally"
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
fi

# Step 4: Build and push images
log_step "Step 4: Building and pushing images"
if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
    log_error "DASMLAB_GHCR_PAT is required to push images to registry"
    log_info "Set it with: export DASMLAB_GHCR_PAT=your_token"
    log_info "The token should be a GitHub PAT with 'write:packages' permission"
    exit 1
fi

if ! bash "${SCRIPT_DIR}/scripts/build-and-load-images.sh"; then
    log_error "Failed to build and push images"
    exit 1
fi

# Step 5: Deploy Glooscap
log_step "Step 5: Deploying Glooscap"
if ! bash "${SCRIPT_DIR}/scripts/deploy-glooscap.sh"; then
    log_error "Failed to deploy Glooscap"
    exit 1
fi

# Step 6: Install plugins (if any)
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
    log_step "Step 6: Installing plugins"
    
    # Create temp directory for plugin repos
    mkdir -p "${TEMP_PLUGIN_DIR}"
    
    for plugin in "${SELECTED_PLUGINS[@]}"; do
        log_info "Installing plugin: ${plugin}"
        
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
        
        # Clone or update plugin repo
        if [ -d "${PLUGIN_DIR}" ]; then
            log_info "Updating existing plugin repo: ${plugin}"
            cd "${PLUGIN_DIR}"
            git pull origin main || {
                log_warn "Failed to update ${plugin} repo, trying fresh clone..."
                rm -rf "${PLUGIN_DIR}"
            }
        fi
        
        if [ ! -d "${PLUGIN_DIR}" ]; then
            log_info "Cloning plugin repo: ${plugin}"
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
        
        # Run plugin setup script (if it exists) to install build dependencies
        if [ -f "${PLUGIN_INFRA_DIR}/scripts/setup-macos-env.sh" ]; then
            log_info "Setting up build environment for ${plugin}..."
            cd "${PLUGIN_INFRA_DIR}"
            if ! bash scripts/setup-macos-env.sh; then
                log_warn "Plugin ${plugin} setup failed (continuing anyway)"
            fi
        else
            log_info "No setup script found for ${plugin} (skipping environment setup)"
        fi
        
        # Ensure registry secret exists in plugin namespace (if it has one)
        # Most plugins will use the same secret from glooscap-system
        log_info "Ensuring registry secret exists for ${plugin}..."
        # Try to create secret in plugin namespace if it exists, otherwise use glooscap-system
        PLUGIN_NAMESPACE="${plugin}"
        if kubectl get namespace "${PLUGIN_NAMESPACE}" &>/dev/null; then
            # Copy secret from glooscap-system if it exists
            if kubectl get secret dasmlab-ghcr-pull -n glooscap-system &>/dev/null; then
                kubectl get secret dasmlab-ghcr-pull -n glooscap-system -o yaml | \
                    sed "s/namespace: glooscap-system/namespace: ${PLUGIN_NAMESPACE}/" | \
                    kubectl apply -f - || log_warn "Failed to copy registry secret to ${PLUGIN_NAMESPACE}"
            fi
        fi
        
        # Build and push plugin image
        if [ -f "${PLUGIN_INFRA_DIR}/scripts/build-and-load-images.sh" ]; then
            log_info "Building and pushing ${plugin} image..."
            cd "${PLUGIN_INFRA_DIR}"
            if ! bash scripts/build-and-load-images.sh; then
                log_warn "Failed to build ${plugin} image (continuing)"
                continue
            fi
        else
            log_warn "Plugin ${plugin} does not have build-and-load-images.sh (skipping build)"
        fi
        
        # Deploy plugin (try different script name patterns)
        DEPLOY_SCRIPT=""
        if [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy-${plugin}.sh" ]; then
            DEPLOY_SCRIPT="scripts/deploy-${plugin}.sh"
        elif [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy.sh" ]; then
            DEPLOY_SCRIPT="scripts/deploy.sh"
        fi
        
        if [ -n "${DEPLOY_SCRIPT}" ]; then
            log_info "Deploying ${plugin}..."
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
echo "  Operator: kubectl logs -f -n glooscap-system deployment/glooscap-operator"
echo "  UI: kubectl logs -f -n glooscap-system deployment/glooscap-ui"
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ] && [[ " ${SELECTED_PLUGINS[*]} " =~ " iskoces " ]]; then
    echo "  Iskoces: kubectl logs -f -n iskoces deployment/iskoces-server"
fi
echo ""
log_info "To uninstall, run: ./uninstall_glooscap.sh"
echo ""

