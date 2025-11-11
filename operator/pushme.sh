#!/usr/bin/env bash
set -euo pipefail

# --- config ----------------------------------------------------
app=glooscap
scratch="scratch"          # your local image tag, e.g., myapp:scratch
repo="ghcr.io/dasmlab"     # base repo
buildfile=".lastbuild"     # build counter file
# ---------------------------------------------------------------

# ensure .lastbuild exists
if [[ ! -f "$buildfile" ]]; then
    echo "0" > "$buildfile"
fi

# read + increment build number
build=$(cat "$buildfile")
next=$((build + 1))
echo "$next" > "$buildfile"

# create version tag
tag="0.0.${next}-alpha"

# construct full names
src="${app}:${scratch}"
dst_version="${repo}/${app}:${tag}"
dst_latest="${repo}/${app}:latest"

echo "📦 Building push:"
echo "  App:        ${app}"
echo "  Source:     ${src}"
echo "  VersionTag: ${dst_version}"
echo "  LatestTag:  ${dst_latest}"
echo

# tag operations
docker tag "$src" "$dst_version"
docker tag "$src" "$dst_latest"

# push operations
docker push "$dst_version"
docker push "$dst_latest"

echo
echo "✅ Pushed:"
echo "   ${dst_version}"
echo "   ${dst_latest}"

