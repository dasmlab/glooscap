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

# Try buildx with --load first, fall back to regular build if not supported
if docker buildx version >/dev/null 2>&1 && docker buildx build --help 2>&1 | grep -q "\--load"; then
    echo "[buildme] Using docker buildx build --load"
    docker buildx build --load \
      --build-arg BUILD_VERSION="${tag}" \
      --build-arg BUILD_NUMBER="${next}" \
      --build-arg BUILD_SHA="${git_sha}" \
      --tag "${app}:${version}" .
else
    echo "[buildme] Using docker build (buildx --load not available)"
    docker build \
      --build-arg BUILD_VERSION="${tag}" \
      --build-arg BUILD_NUMBER="${next}" \
      --build-arg BUILD_SHA="${git_sha}" \
      --tag "${app}:${version}" .
fi

