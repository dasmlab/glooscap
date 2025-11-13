#!/usr/bin/env bash
set -euo pipefail

# --- config ----------------------------------------------------
app=glooscap-ui
scratch="scratch"
repo="ghcr.io/dasmlab"
buildfile=".lastbuild"
# ---------------------------------------------------------------

if [[ ! -f "$buildfile" ]]; then
  echo "0" >"$buildfile"
fi

build=$(cat "$buildfile")
next=$((build + 1))
echo "$next" >"$buildfile"

tag="0.1.${next}-alpha"

src="${app}:${scratch}"
dst_version="${repo}/${app}:${tag}"
dst_latest="${repo}/${app}:latest"

echo "📦 Publishing glooscap-ui"
echo "  Source:  ${src}"
echo "  Dest v:  ${dst_version}"
echo "  Dest latest: ${dst_latest}"
echo

docker tag "$src" "$dst_version"
docker tag "$src" "$dst_latest"

docker push "$dst_version"
docker push "$dst_latest"

echo
echo "✅ pushed ${dst_version} and ${dst_latest}"

