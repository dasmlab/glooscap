#!/bin/bash
#
#  This is a script that creates a Secret on the cluster to match the ImagePullSecret for our operator (so we can fetch our container)
#
#  NOTE: THis wont work for you, you should replace DASMLAB_GHCR_PAT with your own creds for your registry
#
#  Observer that the default namespace is xxxxx-system (e.g my-operator-system), so that is where we put the secret

GHCR_PAT=${DASMLAB_GHCR_PAT}
NAMESPACE="glooscap-system"

echo "DASMLAB_GHCR_PATH (first 8):  REDACTED "
kubectl create secret docker-registry dasmlab-ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=lmcdasm \
  --docker-password=${GHCR_PAT} \
  --docker-email=dasmlab-bot@dasmlab.org \
  -n glooscap-system
