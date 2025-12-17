#!/usr/bin/env bash
set -euo pipefail

# create-registry-secret.sh
# Creates a Kubernetes secret for pulling images from GitHub Container Registry (ghcr.io)
# This is required for pulling private images from ghcr.io/dasmlab/*
#
# Usage:
#   DASMLAB_GHCR_PAT=your_token ./create-registry-secret.sh [namespace]
#
# The secret will be created in the specified namespace (default: glooscap-system)

NAMESPACE="${1:-glooscap-system}"

# Try to source GitHub token from standard locations if not set
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
    echo "ERROR: DASMLAB_GHCR_PAT environment variable is required"
    echo ""
    echo "Usage:"
    echo "  1. export DASMLAB_GHCR_PAT=your_token"
    echo "  2. Create ~/gh-pat file with: export DASMLAB_GHCR_PAT=your_token"
    echo "  3. Create ~/gh-pat/token file with just the token"
    echo ""
    echo "The token should be a GitHub Personal Access Token (PAT) with 'read:packages' permission"
    exit 1
fi

# Ensure namespace exists
echo "Ensuring namespace '${NAMESPACE}' exists..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - || {
    echo "ERROR: Failed to create namespace ${NAMESPACE}"
    exit 1
}
echo "Namespace '${NAMESPACE}' ensured"

echo "Creating image pull secret 'dasmlab-ghcr-pull' in namespace ${NAMESPACE}..."

# Use --dry-run=client -o yaml | kubectl apply -f - to make it idempotent
kubectl create secret docker-registry dasmlab-ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=lmcdasm \
  --docker-password="${GHCR_PAT}" \
  --docker-email=dasmlab-bot@dasmlab.org \
  --namespace "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Secret 'dasmlab-ghcr-pull' ensured in namespace ${NAMESPACE}"

