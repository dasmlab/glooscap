#!/bin/bash
#
# cycleme.sh - Cycles your Operator installation, container build and pusblish so you are always working on a CI/CD production like way.
#
#  Assumes you have set names and vars appropraitely.

set -euo pipefail

NAMESPACE="${NAMESPACE:-glooscap-system}"
MAX_WAIT="${MAX_WAIT:-120}"  # Maximum seconds to wait for namespace termination

echo "🔄 Cycling operator deployment..."

# REMOVE OPERATOR AND BITS FIRST
echo "📦 Undeploying operator..."
make undeploy uninstall || true

# Wait for namespace to fully terminate
echo "⏳ Waiting for namespace '${NAMESPACE}' to terminate..."
if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "   Namespace exists, waiting for termination..."
  timeout="${MAX_WAIT}"
  while [ "${timeout}" -gt 0 ]; do
    if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
      echo "   ✅ Namespace terminated"
      break
    fi
    phase=$(kubectl get namespace "${NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Terminating")
    if [ "${phase}" != "Terminating" ] && [ "${phase}" != "Active" ]; then
      echo "   ✅ Namespace phase: ${phase}"
      break
    fi
    echo "   ⏳ Still terminating... (${timeout}s remaining)"
    sleep 2
    timeout=$((timeout - 2))
  done
  
  if kubectl get namespace "${NAMESPACE}" &>/dev/null; then
    echo "   ⚠️  Warning: Namespace still exists after ${MAX_WAIT}s, proceeding anyway..."
    echo "   💡 You may need to manually clean up: kubectl delete namespace ${NAMESPACE} --force --grace-period=0"
  fi
else
  echo "   ✅ Namespace does not exist, proceeding..."
fi

# Small additional wait to ensure API server has processed deletions
echo "⏳ Brief pause for API server to catch up..."
sleep 3

echo "🔧 Generating manifests..."
make generate
make manifests

# Build a new version of the operator and publish it, bumping SemVer
# Source gh-pat to get DASMLAB_GHCR_PAT for pushing images
if [ -f "${HOME}/gh-pat" ]; then
  echo "🔑 Sourcing ${HOME}/gh-pat for image push credentials..."
  source "${HOME}/gh-pat"
fi
echo "🏗️  Building operator image..."
./buildme.sh
echo "📤 Pushing operator image..."
./pushme.sh

# Deploy CRDs to the Target Cluster (Assumes Kubeconfig is set properly, perms, etc)
echo "🚀 Deploying to cluster..."
make install deploy

# Wait for CRDs to be fully registered in the API server
echo "⏳ Waiting for CRDs to be registered..."
sleep 5

# Create a Registry secret with your Token (pullSecret)
# Source gh-pat to get DASMLAB_GHCR_PAT
if [ -f "${HOME}/gh-pat" ]; then
  echo "🔑 Sourcing ${HOME}/gh-pat for registry credentials..."
  source "${HOME}/gh-pat"
fi
echo "🔐 Creating registry secret..."
./create-registry-secret.sh || echo "⚠️  Warning: Registry secret creation failed"

# Applying OCP Route
echo "🔐 Aplying OCP Route for API ..."
kubectl apply -f ../infra/openshift/operator-api-route.yaml

# Building Web Ui
echo "🏗️  Building UI image..."
cd ../ui
./buildme.sh
./pushme.sh
cd ../operator

# Deploying Web Ui
echo "🚀 Deploying UI to cluster..."
kubectl apply -f ../infra/openshift/glooscap-ui.yaml

# Deploying Wiki Target 
echo "Sleeping 3 seconds..."
sleep 3
echo "🚀 Deploying Wiki Target (wiki.infra.dasmlab.org) to cluster..."
kubectl apply -f ../infra/openshift/wikitarget-infra-dasmlab-org.yaml

# Deploying Translation Service
echo "Sleeping 2 seconds..."
sleep 2
echo "🚀 Deploying Translation Service (iskoces) to cluster..."
kubectl apply -f ../infra/openshift/translationservice-iskoces.yaml

echo "✅ Cycle complete!"
