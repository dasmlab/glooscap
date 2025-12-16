#!/bin/bash
set -e

IMAGE_NAME="ghcr.io/dasmlab/glooscap-translation-runner"
TAG="${1:-latest}"

echo "Building translation-runner image: ${IMAGE_NAME}:${TAG}"

# Build context is parent directory to include operator
cd "$(dirname "$0")/.."

# Check if buildx is available and supports --load
if docker buildx version >/dev/null 2>&1 && docker buildx build --help 2>&1 | grep -q "\--load"; then
    # Use buildx with --load to load image into local Docker daemon
    docker buildx build --load -f translation-runner/Dockerfile -t "${IMAGE_NAME}:${TAG}" . || {
        echo "Warning: buildx build failed, trying regular docker build..."
        docker build -f translation-runner/Dockerfile -t "${IMAGE_NAME}:${TAG}" .
    }
else
    # Use regular docker build
    docker build -f translation-runner/Dockerfile -t "${IMAGE_NAME}:${TAG}" .
fi

# If we have a tag, also tag as latest
if [ "$TAG" != "latest" ]; then
    docker tag "${IMAGE_NAME}:${TAG}" "${IMAGE_NAME}:latest"
fi

echo "Build complete: ${IMAGE_NAME}:${TAG}"

