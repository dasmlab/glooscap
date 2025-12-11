#!/usr/bin/env bash
# check-docker.sh
# Diagnoses Docker setup on macOS

set -euo pipefail

echo "=== Docker Diagnosis ==="
echo ""

# Check Docker CLI
if command -v docker &> /dev/null; then
    echo "✓ Docker CLI is installed: $(docker --version 2>/dev/null || echo 'version unknown')"
else
    echo "✗ Docker CLI not found"
    exit 1
fi

echo ""
echo "=== Docker Daemon Access ==="
echo ""

# Check Docker socket locations
SOCKET_LOCATIONS=(
    "/var/run/docker.sock"
    "$HOME/.docker/run/docker.sock"
    "/tmp/docker.sock"
)

FOUND_SOCKET=""
for socket in "${SOCKET_LOCATIONS[@]}"; do
    if [ -S "${socket}" ] 2>/dev/null; then
        echo "✓ Found Docker socket: ${socket}"
        FOUND_SOCKET="${socket}"
    fi
done

if [ -z "${FOUND_SOCKET}" ]; then
    echo "✗ No Docker socket found in common locations"
fi

echo ""
echo "=== Docker Context ==="
echo ""

# Check Docker context
if docker context ls &> /dev/null 2>&1; then
    echo "Docker contexts:"
    docker context ls 2>&1 || echo "  (docker context ls failed)"
    echo ""
    CURRENT_CONTEXT=$(docker context show 2>/dev/null || echo "unknown")
    echo "Current context: ${CURRENT_CONTEXT}"
else
    echo "Cannot list Docker contexts"
fi

echo ""
echo "=== Docker Info Test ==="
echo ""

# Try docker info
if docker info &> /dev/null 2>&1; then
    echo "✓ docker info works - Docker daemon is accessible"
    echo ""
    echo "Docker daemon info:"
    docker info 2>&1 | head -20 || true
else
    echo "✗ docker info failed - Docker daemon not accessible"
    echo ""
    echo "Error details:"
    docker info 2>&1 | head -10 || true
fi

echo ""
echo "=== Docker PS Test ==="
echo ""

# Try docker ps
if docker ps &> /dev/null 2>&1; then
    echo "✓ docker ps works"
    echo ""
    echo "Running containers:"
    docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" 2>&1 | head -20 || true
else
    echo "✗ docker ps failed"
    docker ps 2>&1 | head -5 || true
fi

echo ""
echo "=== Environment Variables ==="
echo ""

# Check DOCKER_HOST
if [ -n "${DOCKER_HOST:-}" ]; then
    echo "DOCKER_HOST is set: ${DOCKER_HOST}"
else
    echo "DOCKER_HOST is not set (using default socket)"
fi

# Check DOCKER_CONTEXT
if [ -n "${DOCKER_CONTEXT:-}" ]; then
    echo "DOCKER_CONTEXT is set: ${DOCKER_CONTEXT}"
else
    echo "DOCKER_CONTEXT is not set"
fi

echo ""
echo "=== Docker Desktop Status (macOS) ==="
echo ""

# Check if Docker Desktop is running (macOS)
if pgrep -f "Docker Desktop" &> /dev/null; then
    echo "✓ Docker Desktop process is running"
else
    echo "✗ Docker Desktop process not found"
    echo ""
    echo "To start Docker Desktop:"
    echo "  open -a Docker"
    echo ""
    echo "Or check if it's installed:"
    echo "  ls -la /Applications/Docker.app"
fi

echo ""
echo "=== Summary ==="
echo ""
if docker ps &> /dev/null 2>&1; then
    echo "✓ Docker is working - you can create clusters"
else
    echo "✗ Docker is NOT accessible"
    echo ""
    echo "To fix:"
    echo "  1. Start Docker Desktop: open -a Docker"
    echo "  2. Wait for Docker to start (check system tray)"
    echo "  3. Run this script again to verify"
fi

