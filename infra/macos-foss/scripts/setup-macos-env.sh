#!/usr/bin/env bash
# setup-macos-env.sh
# Sets up macOS environment for Glooscap development
# Installs Docker, k3d, kubectl, and other dependencies

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

# Install Docker Desktop (required for k3d - includes daemon)
# NOTE: brew install docker only installs CLI, not daemon!
# We need Docker Desktop (--cask) which includes the daemon
if ! check_command docker; then
    log_info "Installing Docker Desktop (includes daemon)..."
    brew install --cask docker
    log_success "Docker Desktop installed"
    log_warn "Please start Docker Desktop before continuing:"
    log_info "  open -a Docker"
    log_info "  Wait for Docker to start, then run this script again"
else
    log_success "Docker CLI is installed"
    # Check if it's Docker Desktop (has daemon) or just CLI
    if [ -d "/Applications/Docker.app" ]; then
        log_info "Docker Desktop is installed (includes daemon)"
        if pgrep -f "Docker Desktop" &> /dev/null; then
            log_success "Docker Desktop is running"
        else
            log_warn "Docker Desktop is not running"
            log_info "Start it with: open -a Docker"
        fi
    else
        log_warn "Docker CLI found but Docker Desktop not detected"
        log_warn "You may have installed 'docker' via 'brew install docker' (CLI only)"
        log_info "k3d needs Docker Desktop (includes daemon):"
        log_info "  brew install --cask docker"
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

# Install k3d (k3s in Docker containers)
# Note: k3s doesn't work natively on macOS (requires systemd/openrc)
# k3d runs k3s inside Docker containers, which works perfectly on macOS
if ! check_command k3d; then
    log_info "Installing k3d (k3s in Docker containers)..."
    brew install k3d
    log_success "k3d installed"
else
    log_info "k3d already installed: $(k3d version 2>/dev/null | head -n1 || echo 'installed')"
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

if check_command docker; then
    log_success "✓ Docker CLI: $(docker --version 2>/dev/null || echo 'installed')"
    log_info "  (Docker daemon status will be checked when starting cluster)"
else
    log_error "✗ Docker CLI not found"
fi

if check_command kubectl; then
    log_success "✓ kubectl: $(kubectl version --client --short 2>/dev/null | head -n1 || echo 'installed')"
else
    log_error "✗ kubectl not found"
fi

if check_command k3d; then
    log_success "✓ k3d: $(k3d version 2>/dev/null | head -n1 || echo 'installed')"
else
    log_warn "⚠ k3d not found"
fi

# Create kubeconfig directory
mkdir -p "${HOME}/.kube"

# Summary
log_success "macOS environment setup complete!"
echo ""
log_info "Next steps:"
echo "  1. Restart your terminal or run: source ~/.zprofile"
echo "  2. Ensure Docker Desktop is running"
echo "  3. Start Kubernetes cluster:"
echo "     ./scripts/start-k3d.sh"
echo "  4. Run './scripts/deploy-glooscap.sh' to deploy Glooscap"
echo ""
log_info "k3d is the recommended solution:"
log_info "  - Lightweight (k3s in Docker containers, no VM overhead)"
log_info "  - Works reliably with Docker"
log_info "  - Fast startup and simple architecture"
log_info "  - Perfect for local development"

