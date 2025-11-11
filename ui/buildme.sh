#!/usr/bin/env bash
set -euo pipefail

app=glooscap-ui
version=scratch

echo "[buildme] docker build -t ${app}:${version} ."
docker build -t "${app}:${version}" .

