#!/usr/bin/env bash
# check-docker.sh
# Checks Docker/Podman daemon accessibility for k3d

set -euo pipefail

echo "=== Container Runtime Check (for k3d) ==="
echo ""

# Check if docker command exists
if ! command -v docker &> /dev/null; then
    echo "❌ Docker CLI not found"
    echo "   Install with: brew install docker"
    exit 1
fi

echo "✓ Docker CLI found: $(docker --version)"
echo ""

# Check if Podman is installed and running
if command -v podman &> /dev/null; then
    echo "✓ Podman found: $(podman --version)"
    
    # Check Podman machine status
    if podman machine list 2>/dev/null | grep -q "running"; then
        echo "✓ Podman machine is running"
        
        # Get Podman socket
        PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
        if [ -n "${PODMAN_SOCKET}" ]; then
            echo "  Podman socket: ${PODMAN_SOCKET}"
            
            # Check if DOCKER_HOST is set
            if [ -n "${DOCKER_HOST:-}" ]; then
                echo "  DOCKER_HOST: ${DOCKER_HOST}"
            else
                echo "  ⚠ DOCKER_HOST not set"
                echo "  Set it with: export DOCKER_HOST=unix://${PODMAN_SOCKET}"
                echo "  Or add to ~/.zshrc: export DOCKER_HOST=unix://${PODMAN_SOCKET}"
            fi
        fi
    else
        echo "⚠ Podman machine is not running"
        echo "  Start it with: podman machine start"
    fi
    echo ""
fi

# Check Docker context
echo "Docker Context:"
docker context ls 2>/dev/null || echo "  (Could not list contexts)"
echo ""

# Check if Docker daemon is accessible (via Docker CLI, which may use Podman)
echo "Testing container runtime connection (via Docker CLI)..."
if docker ps &> /dev/null; then
    echo "✓ Container runtime is accessible via Docker CLI"
    echo ""
    echo "Runtime Info:"
    docker info 2>/dev/null | grep -E "(Server Version|Operating System|OSType|Architecture|Total Memory|CPUs|Backend)" || true
    
    # Check if it's actually Podman
    if docker info 2>/dev/null | grep -q "podman"; then
        echo ""
        echo "ℹ Using Podman as backend (via Docker CLI)"
    fi
else
    echo "❌ Cannot connect to container runtime"
    echo ""
    echo "Possible issues:"
    if command -v podman &> /dev/null; then
        echo "  1. Podman machine is not running: podman machine start"
        echo "  2. DOCKER_HOST not set to Podman socket"
        echo "  3. Docker CLI not configured to use Podman"
    else
        echo "  1. Docker Desktop is not running"
        echo "  2. Docker daemon is not accessible"
        echo "  3. Wrong Docker context"
    fi
    echo ""
    echo "Try:"
    if command -v podman &> /dev/null; then
        echo "  - Start Podman: podman machine start"
        PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
        if [ -n "${PODMAN_SOCKET}" ]; then
            echo "  - Set DOCKER_HOST: export DOCKER_HOST=unix://${PODMAN_SOCKET}"
        fi
    else
        echo "  - Start Docker Desktop: open -a Docker"
        echo "  - Check Docker context: docker context ls"
    fi
    exit 1
fi

echo ""
echo "=== Summary ==="
echo ""
if docker ps &> /dev/null; then
    echo "✓ Container runtime is working - you can create clusters with k3d"
else
    echo "✗ Container runtime is NOT accessible"
fi
