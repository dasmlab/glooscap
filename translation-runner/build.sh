#!/bin/bash
set -e

IMAGE_NAME="ghcr.io/dasmlab/glooscap-translation-runner"
TAG="${1:-latest}"

echo "Building translation-runner image: ${IMAGE_NAME}:${TAG}"

# Build context is parent directory to include operator
cd "$(dirname "$0")/.."
docker build -f translation-runner/Dockerfile -t "${IMAGE_NAME}:${TAG}" .

# If we have a tag, also tag as latest
if [ "$TAG" != "latest" ]; then
    docker tag "${IMAGE_NAME}:${TAG}" "${IMAGE_NAME}:latest"
fi

echo "Build complete: ${IMAGE_NAME}:${TAG}"

