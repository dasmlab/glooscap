#!/usr/bin/env bash
set -euo pipefail

# Mirror of operator/create-registry-secret.sh for UI deployments.
# Replace DASMLAB_GHCR_PAT with your registry credential.

GHCR_PAT="${DASMLAB_GHCR_PAT:?missing DASMLAB_GHCR_PAT env var}"
NAMESPACE="${1:-glooscap-system}"

echo "Creating image pull secret in namespace ${NAMESPACE}"
kubectl create secret docker-registry dasmlab-ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=lmcdasm \
  --docker-password="${GHCR_PAT}" \
  --docker-email=dasmlab-bot@dasmlab.org \
  --namespace "${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Secret dasmlab-ghcr-pull ensured in namespace ${NAMESPACE}"

