#!/usr/bin/env bash
set -euo pipefail

# buildme.sh - Build Glooscap operator Docker image
# This script builds the Docker image with a local "scratch" tag

app=glooscap
version=scratch

echo "[buildme] Building ${app}:${version}..."
docker build -t "${app}:${version}" .

echo "[buildme] ✅ Build complete: ${app}:${version}"
