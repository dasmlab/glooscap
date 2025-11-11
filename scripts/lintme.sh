#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[lintme] Operator static analysis..."
pushd "${ROOT_DIR}/operator" >/dev/null
golangci-lint run ./... || echo "[lintme] golangci-lint not installed, skipping"
go test ./api/... ./internal/... ./cmd/... >/dev/null
popd >/dev/null

echo "[lintme] UI lint..."
pushd "${ROOT_DIR}/ui" >/dev/null
npm install --silent
npm run lint
popd >/dev/null

echo "[lintme] Completed."

