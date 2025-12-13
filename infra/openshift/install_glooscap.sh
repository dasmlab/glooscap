#!/usr/bin/env bash
# install_glooscap.sh
# Installation script for Glooscap on OpenShift
# Assumes OpenShift cluster is already available and logged in
# Supports optional plugins: --plugins iskoces,nokomis or --all

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
TEMP_PLUGIN_DIR="${HOME}/.glooscap-plugins"

# Plugin repository URLs (adjust as needed)
PLUGIN_REPOS=(
    "iskoces:https://github.com/dasmlab/iskoces.git"
    "nokomis:https://github.com/dasmlab/nokomis.git"
    # nanabush is excluded - not suitable for OpenShift deployment via this script
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
SELECTED_PLUGINS=()
if [ "${ALL_PLUGINS}" = "true" ]; then
    for plugin_repo in "${PLUGIN_REPOS[@]}"; do
        plugin_name="${plugin_repo%%:*}"
        SELECTED_PLUGINS+=("${plugin_name}")
    done
    log_info "Installing all plugins: ${SELECTED_PLUGINS[*]}"
elif [ -n "${PLUGINS}" ]; then
    IFS=',' read -ra PLUGIN_LIST <<< "${PLUGINS}"
    for plugin in "${PLUGIN_LIST[@]}"; do
        plugin=$(echo "${plugin}" | xargs)
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

log_info "Glooscap Installation for OpenShift"
log_info "This will build, push, and deploy Glooscap to your OpenShift cluster"
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
    log_info "Plugins to install: ${SELECTED_PLUGINS[*]}"
else
    log_info "No plugins specified - installing Glooscap only"
fi
echo ""

# Check prerequisites
log_step "Checking prerequisites"

if ! command -v oc &> /dev/null; then
    log_error "oc CLI not found. Please install OpenShift CLI."
    exit 1
fi

if ! oc whoami &> /dev/null; then
    log_error "Not logged in to OpenShift cluster. Please run: oc login"
    exit 1
fi

log_success "OpenShift CLI found and logged in: $(oc whoami)"

if ! command -v docker &> /dev/null && ! command -v podman &> /dev/null; then
    log_error "Neither docker nor podman found. Please install one."
    exit 1
fi

CONTAINER_CMD=""
if command -v podman &> /dev/null; then
    CONTAINER_CMD="podman"
    log_info "Using podman as container tool"
else
    CONTAINER_CMD="docker"
    log_info "Using docker as container tool"
fi

if ! command -v go &> /dev/null; then
    log_error "Go not found. Please install Go (required for building operator)."
    exit 1
fi

log_success "Go found: $(go version | awk '{print $3}')"

if ! command -v make &> /dev/null; then
    log_error "make not found. Please install make."
    exit 1
fi

log_success "All prerequisites met"

# Step 1: Generate manifests and CRDs
log_step "Step 1: Generating manifests and CRDs"
cd "${OPERATOR_DIR}"

log_info "Generating code..."
make generate

log_info "Generating manifests..."
make manifests

log_success "Manifests generated"

# Step 2: Build and push operator image
log_step "Step 2: Building and pushing operator image"

if [ -z "${DASMLAB_GHCR_PAT:-}" ]; then
    log_error "DASMLAB_GHCR_PAT is required to push images to registry"
    log_info "Set it with: export DASMLAB_GHCR_PAT=your_token"
    log_info "The token should be a GitHub PAT with 'write:packages' permission"
    exit 1
fi

log_info "Building operator image..."
if [ -f "${OPERATOR_DIR}/buildme.sh" ]; then
    bash "${OPERATOR_DIR}/buildme.sh"
else
    log_error "buildme.sh not found in operator directory"
    exit 1
fi

log_info "Pushing operator image..."
if [ -f "${OPERATOR_DIR}/pushme.sh" ]; then
    bash "${OPERATOR_DIR}/pushme.sh"
else
    log_error "pushme.sh not found in operator directory"
    exit 1
fi

log_success "Operator image built and pushed"

# Step 3: Create namespace and registry secret
log_step "Step 3: Creating namespace and registry secret"

NAMESPACE="glooscap-system"

if oc get namespace "${NAMESPACE}" &>/dev/null; then
    log_info "Namespace ${NAMESPACE} already exists"
else
    log_info "Creating namespace ${NAMESPACE}..."
    oc create namespace "${NAMESPACE}"
    log_success "Namespace created"
fi

log_info "Creating registry secret..."
if [ -f "${OPERATOR_DIR}/create-registry-secret.sh" ]; then
    bash "${OPERATOR_DIR}/create-registry-secret.sh" || {
        log_warn "Registry secret creation failed (may already exist)"
    }
else
    log_warn "create-registry-secret.sh not found, creating secret manually..."
    if ! oc get secret dasmlab-ghcr-pull -n "${NAMESPACE}" &>/dev/null; then
        echo "${DASMLAB_GHCR_PAT}" | oc create secret docker-registry dasmlab-ghcr-pull \
            --docker-server=ghcr.io \
            --docker-username=lmcdasm \
            --docker-password-stdin \
            --docker-email=dasmlab-bot@dasmlab.org \
            --namespace="${NAMESPACE}" || {
            log_warn "Failed to create registry secret (may already exist)"
        }
    fi
fi

log_success "Registry secret ensured"

# Step 4: Install CRDs and deploy operator
log_step "Step 4: Installing CRDs and deploying operator"

log_info "Installing CRDs..."
make install

log_info "Deploying operator..."
make deploy

log_success "Operator deployed"

# Step 5: Build and push UI image
log_step "Step 5: Building and pushing UI image"

cd "${UI_DIR}"

log_info "Building UI image..."
if [ -f "${UI_DIR}/buildme.sh" ]; then
    bash "${UI_DIR}/buildme.sh"
else
    log_error "buildme.sh not found in UI directory"
    exit 1
fi

log_info "Pushing UI image..."
if [ -f "${UI_DIR}/pushme.sh" ]; then
    bash "${UI_DIR}/pushme.sh"
else
    log_error "pushme.sh not found in UI directory"
    exit 1
fi

log_success "UI image built and pushed"

# Step 6: Deploy UI
log_step "Step 6: Deploying UI"

log_info "Applying UI manifests..."
oc apply -f "${SCRIPT_DIR}/glooscap-ui.yaml"

log_success "UI deployed"

# Step 7: Create API route
log_step "Step 7: Creating API route"

log_info "Applying API route..."
oc apply -f "${SCRIPT_DIR}/operator-api-route.yaml"

log_success "API route created"

# Step 8: Deploy WikiTarget (if exists)
log_step "Step 8: Deploying WikiTarget"

sleep 3  # Brief pause for operator to be ready

if [ -f "${SCRIPT_DIR}/wikitarget-infra-dasmlab-org.yaml" ]; then
    log_info "Deploying WikiTarget..."
    oc apply -f "${SCRIPT_DIR}/wikitarget-infra-dasmlab-org.yaml"
    log_success "WikiTarget deployed"
else
    log_warn "WikiTarget manifest not found (skipping)"
fi

# Step 9: Install plugins (if any)
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ]; then
    log_step "Step 9: Installing plugins"
    
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
        
        # Check if plugin has infra/openshift directory
        PLUGIN_INFRA_DIR="${PLUGIN_DIR}/infra/openshift"
        if [ ! -d "${PLUGIN_INFRA_DIR}" ]; then
            log_warn "Plugin ${plugin} does not have infra/openshift directory (skipping)"
            continue
        fi
        
        # Ensure registry secret exists in plugin namespace
        PLUGIN_NAMESPACE="${plugin}"
        if oc get namespace "${PLUGIN_NAMESPACE}" &>/dev/null || oc create namespace "${PLUGIN_NAMESPACE}" &>/dev/null; then
            if oc get secret dasmlab-ghcr-pull -n "${NAMESPACE}" &>/dev/null; then
                oc get secret dasmlab-ghcr-pull -n "${NAMESPACE}" -o yaml | \
                    sed "s/namespace: ${NAMESPACE}/namespace: ${PLUGIN_NAMESPACE}/" | \
                    sed "/resourceVersion:/d; /uid:/d; /creationTimestamp:/d" | \
                    oc apply -f - || log_warn "Failed to copy registry secret to ${PLUGIN_NAMESPACE}"
            fi
        fi
        
        # Build and push plugin image
        if [ -f "${PLUGIN_INFRA_DIR}/scripts/build-and-push-images.sh" ] || [ -f "${PLUGIN_DIR}/buildme.sh" ]; then
            log_info "Building and pushing ${plugin} image..."
            if [ -f "${PLUGIN_INFRA_DIR}/scripts/build-and-push-images.sh" ]; then
                cd "${PLUGIN_INFRA_DIR}"
                bash scripts/build-and-push-images.sh || {
                    log_warn "Failed to build ${plugin} image (continuing)"
                    continue
                }
            elif [ -f "${PLUGIN_DIR}/buildme.sh" ]; then
                cd "${PLUGIN_DIR}"
                bash buildme.sh
                if [ -f "${PLUGIN_DIR}/pushme.sh" ]; then
                    bash pushme.sh
                fi
            fi
        else
            log_warn "Plugin ${plugin} does not have build script (skipping build)"
        fi
        
        # Deploy plugin
        DEPLOY_SCRIPT=""
        if [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy-${plugin}.sh" ]; then
            DEPLOY_SCRIPT="scripts/deploy-${plugin}.sh"
        elif [ -f "${PLUGIN_INFRA_DIR}/scripts/deploy.sh" ]; then
            DEPLOY_SCRIPT="scripts/deploy.sh"
        elif [ -f "${PLUGIN_INFRA_DIR}/install_glooscap.sh" ]; then
            DEPLOY_SCRIPT="install_glooscap.sh"
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
log_info "Deployment status:"
echo "  Namespace: ${NAMESPACE}"
echo ""
log_info "View resources:"
echo "  Operator pods: oc get pods -n ${NAMESPACE}"
echo "  UI pods: oc get pods -n ${NAMESPACE} -l app=glooscap-ui"
echo "  Routes: oc get routes -n ${NAMESPACE}"
echo "  Services: oc get svc -n ${NAMESPACE}"
echo ""
log_info "View logs:"
echo "  Operator: oc logs -f -n ${NAMESPACE} deployment/glooscap-controller-manager"
echo "  UI: oc logs -f -n ${NAMESPACE} deployment/glooscap-ui"
echo ""
if [ ${#SELECTED_PLUGINS[@]} -gt 0 ] && [[ " ${SELECTED_PLUGINS[*]} " =~ " iskoces " ]]; then
    log_info "Iskoces service: iskoces-service.iskoces.svc:50051"
    log_info "Configure in Glooscap UI: Settings → Translation Service"
fi
echo ""
log_info "To uninstall, run: ./uninstall_glooscap.sh"
echo ""

