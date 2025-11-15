#!/bin/bash
# Generate and display code coverage report

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPERATOR_DIR="$(dirname "$SCRIPT_DIR")"

cd "$OPERATOR_DIR"

echo "🔍 Generating code coverage report..."

# Ensure we have envtest
if ! command -v setup-envtest &> /dev/null; then
    echo "📦 Setting up envtest..."
    make setup-envtest
fi

# Generate manifests and code
echo "📝 Generating manifests..."
make manifests generate

# Run tests with coverage
echo "🧪 Running tests with coverage..."
KUBEBUILDER_ASSETS="$(make -s setup-envtest | grep KUBEBUILDER_ASSETS | cut -d'=' -f2 | tr -d '"')" \
go test $(go list ./... | grep -v /e2e) \
    -coverprofile=coverage.out \
    -covermode=atomic \
    -v

# Generate reports
echo "📊 Generating coverage reports..."
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out > coverage.txt

# Display summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📈 Coverage Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
tail -n 1 coverage.txt
echo ""
echo "📄 Detailed report: coverage.html"
echo "📄 Function-level report: coverage.txt"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Open HTML report if on macOS/Linux with GUI
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "🌐 Opening HTML report in browser..."
    open coverage.html
elif [[ "$OSTYPE" == "linux-gnu"* ]] && command -v xdg-open &> /dev/null; then
    echo "🌐 Opening HTML report in browser..."
    xdg-open coverage.html
fi

