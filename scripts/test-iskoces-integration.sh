#!/usr/bin/env bash
set -euo pipefail

# test-iskoces-integration.sh - Test Glooscap integration with Iskoces
# This script helps verify that Glooscap can connect to Iskoces
# This works only in docker/podman on a localhost mode!

echo "=========================================="
echo "Testing Glooscap <-> Iskoces Integration"
echo "=========================================="
echo ""

# Check if Iskoces is running
echo "[1/4] Checking if Iskoces is running..."
if docker ps --format '{{.Names}}' | grep -q "iskoces-server-instance"; then
    echo "✅ Iskoces container is running"
    ISKOCES_RUNNING=true
else
    echo "⚠️  Iskoces container not found"
    echo "   Start it with: cd /home/dasm/org-dasmlab/infra/iskoces && ./runme.sh"
    ISKOCES_RUNNING=false
fi
echo ""

# Check if Iskoces gRPC port is accessible
echo "[2/4] Checking Iskoces gRPC endpoint..."
if nc -z localhost 50051 2>/dev/null; then
    echo "✅ Iskoces gRPC port 50051 is accessible"
    GRPC_ACCESSIBLE=true
else
    echo "❌ Iskoces gRPC port 50051 is not accessible"
    echo "   Make sure Iskoces is running and port is exposed"
    GRPC_ACCESSIBLE=false
fi
echo ""

# Check if we can connect with grpc_health_probe
echo "[3/4] Testing gRPC health check..."
if command -v grpc_health_probe >/dev/null 2>&1; then
    if grpc_health_probe -addr localhost:50051 2>/dev/null; then
        echo "✅ Iskoces gRPC health check passed"
        HEALTH_OK=true
    else
        echo "⚠️  Iskoces gRPC health check failed (may still be starting up)"
        HEALTH_OK=false
    fi
else
    echo "⚠️  grpc_health_probe not found, skipping health check"
    HEALTH_OK=unknown
fi
echo ""

# Test translation with test client
echo "[4/4] Testing translation via Iskoces..."
if [ -f /home/dasm/org-dasmlab/infra/iskoces/bin/test-client ]; then
    if /home/dasm/org-dasmlab/infra/iskoces/bin/test-client \
        -addr localhost:50051 \
        -source en \
        -target fr \
        -text "Hello, world!" 2>&1 | grep -q "TRANSLATED TEXT"; then
        echo "✅ Translation test successful"
        TRANSLATION_OK=true
    else
        echo "⚠️  Translation test failed or incomplete"
        TRANSLATION_OK=false
    fi
else
    echo "⚠️  Iskoces test client not found, skipping translation test"
    echo "   Build it with: cd /home/dasm/org-dasmlab/infra/iskoces && make build-test"
    TRANSLATION_OK=unknown
fi
echo ""

# Summary
echo "=========================================="
echo "Integration Test Summary"
echo "=========================================="
echo "Iskoces Running:     ${ISKOCES_RUNNING:-false}"
echo "gRPC Accessible:     ${GRPC_ACCESSIBLE:-false}"
echo "Health Check:        ${HEALTH_OK:-unknown}"
echo "Translation Test:    ${TRANSLATION_OK:-unknown}"
echo ""

if [ "${ISKOCES_RUNNING:-false}" = "true" ] && [ "${GRPC_ACCESSIBLE:-false}" = "true" ]; then
    echo "✅ Iskoces is ready for Glooscap integration"
    echo ""
    echo "To configure Glooscap to use Iskoces:"
    echo "  Set environment variable:"
    echo "    TRANSLATION_SERVICE_ADDR=localhost:50051"
    echo "    TRANSLATION_SERVICE_TYPE=iskoces"
    echo ""
    echo "Or for Kubernetes:"
    echo "    TRANSLATION_SERVICE_ADDR=iskoces-service.iskoces.svc:50051"
else
    echo "❌ Iskoces is not ready. Please start it first."
    exit 1
fi

