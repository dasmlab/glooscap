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

GHCR_PAT="${DASMLAB_GHCR_PAT:-}"
NAMESPACE="${1:-glooscap-system}"

if [ -z "${GHCR_PAT}" ]; then
    echo "ERROR: DASMLAB_GHCR_PAT environment variable is required"
    echo ""
    echo "Usage:"
    echo "  DASMLAB_GHCR_PAT=your_token ./create-registry-secret.sh [namespace]"
    echo ""
    echo "The token should be a GitHub Personal Access Token (PAT) with 'read:packages' permission"
    exit 1
fi

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

