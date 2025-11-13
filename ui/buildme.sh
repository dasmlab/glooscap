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
tag="0.1.${next}-alpha"

echo "[buildme] docker build -t ${app}:${version} ."
echo "  Build number: ${next}"
echo "  Version tag: ${tag}"

docker build \
  --build-arg BUILD_VERSION="${tag}" \
  --build-arg BUILD_NUMBER="${next}" \
  -t "${app}:${version}" .

