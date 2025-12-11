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

# Install Docker CLI (for k3d compatibility)
# We use Podman as the actual container runtime/daemon
if ! check_command docker; then
    log_info "Installing Docker CLI (for k3d compatibility)..."
    brew install docker
    log_success "Docker CLI installed"
else
    log_success "Docker CLI is installed"
fi

# Install Podman (container runtime/daemon)
if ! check_command podman; then
    log_info "Installing Podman (container runtime)..."
    brew install podman
    log_success "Podman installed"
    
    # Initialize Podman machine in rootful mode (required for k3d)
    log_info "Initializing Podman machine in rootful mode..."
    log_info "Rootful mode is required for k3d to work properly (avoids rootless limitations)"
    
    # Check if machine already exists
    if podman machine list 2>/dev/null | grep -q "podman-machine"; then
        log_warn "Podman machine already exists"
        log_info "To recreate in rootful mode, run:"
        log_info "  podman machine stop"
        log_info "  podman machine rm podman-machine-default"
        log_info "  podman machine init --rootful podman-machine-default"
        log_info "  podman machine start podman-machine-default"
    else
        # Initialize in rootful mode
        podman machine init --rootful || {
            log_error "Failed to initialize Podman machine in rootful mode"
            log_info "You may need to run: podman machine init --rootful"
            exit 1
        }
        log_success "Podman machine initialized in rootful mode"
    fi
    
    # Start Podman machine
    log_info "Starting Podman machine..."
    podman machine start || log_warn "Podman machine may already be running"
    log_success "Podman machine started"
else
    log_info "Podman already installed: $(podman --version)"
    
    # Check if machine is in rootful mode
    MACHINE_NAME=$(podman machine list --format json 2>/dev/null | grep -o '"Name":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "podman-machine-default")
    if [ -n "${MACHINE_NAME}" ]; then
        # Check machine config for rootful
        MACHINE_CONFIG="${HOME}/.config/containers/podman/machine/applehv/${MACHINE_NAME}.json"
        if [ -f "${MACHINE_CONFIG}" ]; then
            if grep -q '"Rootful":true' "${MACHINE_CONFIG}" 2>/dev/null; then
                log_success "Podman machine is in rootful mode"
            else
                log_warn "Podman machine is in rootless mode"
                log_warn "k3d works better with rootful mode"
                log_info "To switch to rootful mode:"
                log_info "  1. podman machine stop"
                log_info "  2. podman machine rm ${MACHINE_NAME}"
                log_info "  3. podman machine init --rootful ${MACHINE_NAME}"
                log_info "  4. podman machine start ${MACHINE_NAME}"
            fi
        fi
    fi
    
    # Ensure Podman machine is running
    if ! podman machine list 2>/dev/null | grep -q "running"; then
        log_info "Starting Podman machine..."
        podman machine start || log_warn "Podman machine start failed"
    fi
fi

# Configure Docker CLI to use Podman socket
# k3d will use DOCKER_HOST to connect to Podman
PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
if [ -n "${PODMAN_SOCKET}" ]; then
    log_info "Configuring Docker CLI to use Podman socket..."
    export DOCKER_HOST="unix://${PODMAN_SOCKET}"
    log_info "DOCKER_HOST set to: ${DOCKER_HOST}"
    
    # Add to shell profile for persistence
    SHELL_PROFILE=""
    if [ -f "${HOME}/.zprofile" ]; then
        SHELL_PROFILE="${HOME}/.zprofile"
    elif [ -f "${HOME}/.zshrc" ]; then
        SHELL_PROFILE="${HOME}/.zshrc"
    fi
    
    if [ -n "${SHELL_PROFILE}" ]; then
        if ! grep -q "DOCKER_HOST.*podman\|DOCKER_HOST.*unix://" "${SHELL_PROFILE}" 2>/dev/null; then
            log_info "Adding DOCKER_HOST to ${SHELL_PROFILE} for persistence..."
            echo "" >> "${SHELL_PROFILE}"
            echo "# Configure Docker CLI to use Podman (for k3d)" >> "${SHELL_PROFILE}"
            echo "export DOCKER_HOST=\"unix://${PODMAN_SOCKET}\"" >> "${SHELL_PROFILE}"
            log_success "DOCKER_HOST added to ${SHELL_PROFILE}"
        else
            log_info "DOCKER_HOST already configured in ${SHELL_PROFILE}"
        fi
    else
        log_warn "Could not find ~/.zprofile or ~/.zshrc"
        log_info "Add this to your shell profile:"
        log_info "  export DOCKER_HOST=\"unix://${PODMAN_SOCKET}\""
    fi
else
    log_warn "Could not detect Podman socket"
    log_info "k3d will try to detect Podman automatically"
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
else
    log_error "✗ Docker CLI not found"
fi

if check_command podman; then
    log_success "✓ Podman: $(podman --version 2>/dev/null || echo 'installed')"
    if podman machine list 2>/dev/null | grep -q "running"; then
        log_success "✓ Podman machine is running"
        PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || echo "")
        if [ -n "${PODMAN_SOCKET}" ]; then
            log_info "  Podman socket: ${PODMAN_SOCKET}"
            log_info "  Set DOCKER_HOST=unix://${PODMAN_SOCKET} for k3d"
        fi
    else
        log_warn "⚠ Podman machine is not running"
        log_info "  Start it with: podman machine start"
    fi
else
    log_error "✗ Podman not found"
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
echo "  2. Ensure Podman machine is running: podman machine start"
echo "  3. Verify container runtime: ./scripts/check-docker.sh"
echo "  4. Start Kubernetes cluster:"
echo "     ./scripts/start-k3d.sh"
echo "  5. Run './scripts/deploy-glooscap.sh' to deploy Glooscap"
echo ""
log_info "k3d with Podman (FOSS approach):"
log_info "  - Uses Podman as container runtime (no Docker Desktop needed)"
log_info "  - Docker CLI connects to Podman via DOCKER_HOST"
log_info "  - Lightweight and fast startup"
log_info "  - Perfect for local development"

