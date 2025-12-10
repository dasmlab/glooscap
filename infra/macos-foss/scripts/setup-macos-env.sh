#!/usr/bin/env bash
# setup-macos-env.sh
# Sets up macOS environment for Glooscap FOSS development
# Installs Podman, k3d, kubectl, and other dependencies

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_command() {
    if command -v "$1" &> /dev/null; then
        return 0
    else
        return 1
    fi
}

# Check if running on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    log_error "This script is designed for macOS only"
    exit 1
fi

log_info "Starting macOS environment setup for Glooscap FOSS development..."

# Check for Homebrew
if ! check_command brew; then
    log_warn "Homebrew not found. Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    
    # Setup Homebrew shellenv for Apple Silicon or Intel
    CURRENT_USER=$(whoami)
    if [[ -d "/opt/homebrew" ]]; then
        # Apple Silicon
        BREW_PREFIX="/opt/homebrew"
    else
        # Intel
        BREW_PREFIX="/usr/local"
    fi
    
    log_info "Setting up Homebrew shell environment..."
    if [[ -f "${HOME}/.zprofile" ]]; then
        if ! grep -q "brew shellenv" "${HOME}/.zprofile"; then
            echo "" >> "${HOME}/.zprofile"
            echo 'eval "$('"${BREW_PREFIX}"'/bin/brew shellenv)"' >> "${HOME}/.zprofile"
        fi
    else
        echo 'eval "$('"${BREW_PREFIX}"'/bin/brew shellenv)"' > "${HOME}/.zprofile"
    fi
    
    # Also add to .zshrc if it exists
    if [[ -f "${HOME}/.zshrc" ]]; then
        if ! grep -q "brew shellenv" "${HOME}/.zshrc"; then
            echo "" >> "${HOME}/.zshrc"
            echo 'eval "$('"${BREW_PREFIX}"'/bin/brew shellenv)"' >> "${HOME}/.zshrc"
        fi
    fi
    
    # Evaluate shellenv for current session
    eval "$(${BREW_PREFIX}/bin/brew shellenv)"
    
    log_success "Homebrew installed and configured"
else
    log_info "Homebrew already installed: $(brew --version | head -n1)"
fi

# Update Homebrew
log_info "Updating Homebrew..."
brew update

# Install Podman
if ! check_command podman; then
    log_info "Installing Podman..."
    
    # Check if Podman Desktop is preferred
    if [[ "${PODMAN_DESKTOP:-false}" == "true" ]]; then
        log_info "Installing Podman Desktop via Homebrew Cask..."
        brew install --cask podman-desktop
        log_success "Podman Desktop installed"
        log_warn "Please start Podman Desktop and initialize the machine, then run this script again"
        exit 0
    else
        log_info "Installing Podman via Homebrew..."
        brew install podman
        log_success "Podman installed"
        
        # Initialize Podman machine
        log_info "Initializing Podman machine..."
        podman machine init || log_warn "Podman machine may already be initialized"
        podman machine start || log_warn "Podman machine may already be running"
        log_success "Podman machine initialized and started"
    fi
else
    log_info "Podman already installed: $(podman --version)"
    
    # Ensure Podman machine is running
    if ! podman machine list | grep -q "running"; then
        log_info "Starting Podman machine..."
        podman machine start
    fi
fi

# Install kubectl
if ! check_command kubectl; then
    log_info "Installing kubectl..."
    brew install kubectl
    log_success "kubectl installed"
else
    log_info "kubectl already installed: $(kubectl version --client --short 2>/dev/null || echo 'installed')"
fi

# Note: k3s doesn't work natively on macOS (requires systemd/openrc)
# We'll use k3d instead, which runs k3s inside Docker/Podman containers
log_info "Note: k3s doesn't work natively on macOS (requires systemd/openrc)"
log_info "We'll install k3d instead, which runs k3s inside containers (works on macOS)"

# Install k3d (k3s in Docker/Podman)
# Note: k3d has known issues with Podman on macOS, minikube is recommended
if ! check_command k3d; then
    log_info "Installing k3d..."
    brew install k3d
    log_success "k3d installed"
    log_warn "Note: k3d has known compatibility issues with Podman on macOS"
    log_info "If k3d hangs, consider using minikube instead (see below)"
else
    log_info "k3d already installed: $(k3d version 2>/dev/null || echo 'installed')"
fi

# Install minikube (recommended alternative for Podman on macOS)
if ! check_command minikube; then
    log_info "Installing minikube (recommended for Podman on macOS)..."
    brew install minikube
    log_success "minikube installed"
else
    log_info "minikube already installed: $(minikube version --short 2>/dev/null || echo 'installed')"
fi

# Install Helm (optional, for future use)
if [[ "${INSTALL_HELM:-false}" == "true" ]]; then
    if ! check_command helm; then
        log_info "Installing Helm..."
        brew install helm
        log_success "Helm installed"
    else
        log_info "Helm already installed: $(helm version --short 2>/dev/null || echo 'installed')"
    fi
fi

# k3d is now the default for macOS (installed above)

# Verify installations
log_info "Verifying installations..."

if check_command podman; then
    log_success "✓ Podman: $(podman --version)"
    podman info &> /dev/null && log_success "✓ Podman machine is running" || log_warn "⚠ Podman machine may not be running"
else
    log_error "✗ Podman not found"
fi

if check_command kubectl; then
    log_success "✓ kubectl: $(kubectl version --client --short 2>/dev/null | head -n1 || echo 'installed')"
else
    log_error "✗ kubectl not found"
fi

if check_command k3d; then
    log_success "✓ k3d: $(k3d version 2>/dev/null || echo 'installed')"
else
    log_warn "⚠ k3d not found"
fi

if check_command minikube; then
    log_success "✓ minikube: $(minikube version --short 2>/dev/null || echo 'installed')"
else
    log_warn "⚠ minikube not found"
fi

# Create kubeconfig directory
mkdir -p "${HOME}/.kube"

# Summary
log_success "macOS environment setup complete!"
echo ""
log_info "Next steps:"
echo "  1. Restart your terminal or run: source ~/.zprofile"
echo "  2. Start Kubernetes cluster:"
echo "     - RECOMMENDED (Podman): ./scripts/start-minikube.sh"
echo "     - Alternative: ./scripts/start-k3d.sh (may hang with Podman)"
echo "  3. Run './scripts/deploy-glooscap.sh' to deploy Glooscap"
echo ""
log_warn "IMPORTANT: k3d has known issues with Podman on macOS and may hang."
log_info "RECOMMENDED: Use minikube instead (works reliably with Podman):"
log_info "  ./scripts/start-minikube.sh"

