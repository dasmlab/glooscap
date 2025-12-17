#!/usr/bin/env bash
set -euo pipefail

app=glooscap-ui
version=scratch
buildfile=".lastbuild"

# Read or initialize build number
if [[ ! -f "$buildfile" ]]; then
  echo "0" > "$buildfile"
fi

build=$(cat "$buildfile")
# Use next build number (what pushme.sh will tag as)
next=$((build + 1))

# Create version tag that will match pushme.sh
# Version bumped to 0.3.x-alpha to match current release
tag="0.3.${next}-alpha"

# Get git SHA (first 6 characters) for build identification
# Use HEAD commit if in git repo, otherwise use "unknown"
if git rev-parse --git-dir > /dev/null 2>&1; then
  git_sha=$(git rev-parse --short=6 HEAD)
else
  git_sha="unknown"
fi

echo "[buildme] Building ${app}:${version}..."
echo "  Build number: ${next}"
echo "  Version tag: ${tag}"
echo "  Git SHA: ${git_sha}"
echo "  Working directory: $(pwd)"

# Verify required files exist before building
if [ ! -f "package.json" ]; then
    echo "ERROR: package.json not found in current directory ($(pwd))"
    echo "Please ensure you are running buildme.sh from the ui directory"
    exit 1
fi

if [ ! -f "Dockerfile" ]; then
    echo "ERROR: Dockerfile not found in current directory ($(pwd))"
    echo "Please ensure you are running buildme.sh from the ui directory"
    exit 1
fi

echo "  Verifying build context: package.json=$(test -f package.json && echo 'OK' || echo 'MISSING'), Dockerfile=$(test -f Dockerfile && echo 'OK' || echo 'MISSING')"

# Try buildx with --load first, fall back to regular build if not supported
# Enable BuildKit for cache mounts (faster npm installs)
export DOCKER_BUILDKIT=1
if docker buildx version >/dev/null 2>&1 && docker buildx build --help 2>&1 | grep -q "\--load"; then
    echo "[buildme] Using docker buildx build --load (with BuildKit cache mounts)"
    docker buildx build --load \
      --build-arg BUILD_VERSION="${tag}" \
      --build-arg BUILD_NUMBER="${next}" \
      --build-arg BUILD_SHA="${git_sha}" \
      --tag "${app}:${version}" .
else
    echo "[buildme] Using docker build (buildx --load not available, BuildKit cache mounts may not work)"
    docker build \
      --build-arg BUILD_VERSION="${tag}" \
      --build-arg BUILD_NUMBER="${next}" \
      --build-arg BUILD_SHA="${git_sha}" \
      --tag "${app}:${version}" .
fi

