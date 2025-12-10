#!/usr/bin/env bash
# setup-macos-env.sh
# Sets up macOS environment for Glooscap FOSS development
# Installs Podman, k3s/k0s, kubectl, and other dependencies

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
    log_success "Homebrew installed"
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

# Install k3s (we'll download and install it)
if ! check_command k3s; then
    log_info "Installing k3s..."
    
    # Create k3s directory
    K3S_DIR="${HOME}/.local/bin"
    mkdir -p "${K3S_DIR}"
    
    # Download k3s
    K3S_VERSION="${K3S_VERSION:-latest}"
    if [[ "${K3S_VERSION}" == "latest" ]]; then
        K3S_URL="https://get.k3s.io"
    else
        K3S_URL="https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s"
    fi
    
    # For macOS, we'll use the install script which handles everything
    curl -sfL https://get.k3s.io | INSTALL_K3S_BIN_DIR="${K3S_DIR}" sh -s -
    
    # Add to PATH if not already there
    if [[ ":$PATH:" != *":${K3S_DIR}:"* ]]; then
        log_info "Adding ${K3S_DIR} to PATH in ~/.zshrc (or ~/.bash_profile)"
        if [[ -f "${HOME}/.zshrc" ]]; then
            echo "export PATH=\"${K3S_DIR}:\$PATH\"" >> "${HOME}/.zshrc"
        elif [[ -f "${HOME}/.bash_profile" ]]; then
            echo "export PATH=\"${K3S_DIR}:\$PATH\"" >> "${HOME}/.bash_profile"
        fi
        export PATH="${K3S_DIR}:${PATH}"
    fi
    
    log_success "k3s installed"
else
    log_info "k3s already installed: $(k3s --version 2>/dev/null || echo 'installed')"
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

# Install k0s (alternative to k3s, single binary)
if [[ "${INSTALL_K0S:-false}" == "true" ]]; then
    if ! check_command k0s; then
        log_info "Installing k0s..."
        K0S_VERSION="${K0S_VERSION:-latest}"
        K0S_DIR="${HOME}/.local/bin"
        mkdir -p "${K0S_DIR}"
        
        if [[ "${K0S_VERSION}" == "latest" ]]; then
            K0S_URL=$(curl -s https://api.github.com/repos/k0sproject/k0s/releases/latest | grep "browser_download_url.*darwin" | cut -d '"' -f 4)
        else
            K0S_URL="https://github.com/k0sproject/k0s/releases/download/v${K0S_VERSION}/k0s-v${K0S_VERSION}-darwin-amd64"
        fi
        
        curl -L "${K0S_URL}" -o "${K0S_DIR}/k0s"
        chmod +x "${K0S_DIR}/k0s"
        
        log_success "k0s installed"
    else
        log_info "k0s already installed: $(k0s version 2>/dev/null || echo 'installed')"
    fi
fi

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

if check_command k3s; then
    log_success "✓ k3s: $(k3s --version 2>/dev/null || echo 'installed')"
else
    log_warn "⚠ k3s not found (may need to add to PATH)"
fi

# Create kubeconfig directory
mkdir -p "${HOME}/.kube"

# Summary
log_success "macOS environment setup complete!"
echo ""
log_info "Next steps:"
echo "  1. Run './scripts/start-k3s.sh' to start the k3s cluster"
echo "  2. Run './scripts/deploy-glooscap.sh' to deploy Glooscap"
echo ""
log_info "Note: If you installed k3s, you may need to restart your terminal or run:"
echo "  export PATH=\"\${HOME}/.local/bin:\$PATH\""

