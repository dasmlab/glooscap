#!/usr/bin/env bash
# undeploy-glooscap.sh
# Removes Glooscap from the Kubernetes cluster

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS_DIR="$(cd "${SCRIPT_DIR}/../manifests" && pwd)"

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

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found"
    exit 1
fi

log_info "Removing Glooscap from Kubernetes cluster..."

NAMESPACE="${NAMESPACE:-glooscap-system}"

# Delete UI deployment
log_info "Deleting UI..."
kubectl delete deployment glooscap-ui -n "${NAMESPACE}" --ignore-not-found=true || true
kubectl delete service glooscap-ui -n "${NAMESPACE}" --ignore-not-found=true || true
# Also try from manifests if they exist
if [ -d "${MANIFESTS_DIR}/ui/" ]; then
    kubectl delete -f "${MANIFESTS_DIR}/ui/" --ignore-not-found=true || true
fi

# Delete operator deployment (deployed from dist/install.yaml)
log_info "Deleting operator..."
kubectl delete deployment operator-controller-manager -n "${NAMESPACE}" --ignore-not-found=true || true
kubectl delete service operator-controller-manager-metrics-service -n "${NAMESPACE}" --ignore-not-found=true || true
kubectl delete serviceaccount operator-controller-manager -n "${NAMESPACE}" --ignore-not-found=true || true
# Also try from manifests if they exist
if [ -d "${MANIFESTS_DIR}/operator/" ]; then
    kubectl delete -f "${MANIFESTS_DIR}/operator/" --ignore-not-found=true || true
fi

# Delete RBAC resources
log_info "Deleting RBAC resources..."
kubectl delete role operator-controller-manager -n "${NAMESPACE}" --ignore-not-found=true || true
kubectl delete rolebinding operator-controller-manager -n "${NAMESPACE}" --ignore-not-found=true || true
kubectl delete clusterrolebinding operator-controller-manager --ignore-not-found=true || true
kubectl delete clusterrole operator-controller-manager --ignore-not-found=true || true
# Also try from manifests if they exist
if [ -d "${MANIFESTS_DIR}/rbac/" ]; then
    kubectl delete -f "${MANIFESTS_DIR}/rbac/" --ignore-not-found=true || true
fi

# Delete CRDs (optional, may fail if CRs exist)
if [[ "${DELETE_CRDS:-false}" == "true" ]]; then
    log_info "Deleting CRDs..."
    kubectl delete crd translationjobs.wiki.glooscap.dasmlab.org --ignore-not-found=true || true
    kubectl delete crd wikitargets.wiki.glooscap.dasmlab.org --ignore-not-found=true || true
    # Also try from manifests if they exist
    if [ -d "${MANIFESTS_DIR}/crd/" ]; then
        kubectl delete -f "${MANIFESTS_DIR}/crd/" --ignore-not-found=true || true
    fi
fi

# Delete namespace (optional, will delete all resources in namespace)
if [[ "${DELETE_NAMESPACE:-false}" == "true" ]]; then
    log_warn "Deleting namespace (this will delete all resources in ${NAMESPACE})..."
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true || true
    log_success "Namespace deleted"
else
    log_info "Namespace preserved. To delete it, run:"
    log_info "  kubectl delete namespace ${NAMESPACE}"
fi

log_success "Glooscap removed from cluster"

