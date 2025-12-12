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
# Docker CLI needs to be recent enough to support API 1.44+ (required by k3d)
if ! check_command docker; then
    log_info "Installing Docker CLI (for k3d compatibility)..."
    brew install docker
    log_success "Docker CLI installed"
else
    # Check Docker CLI version and warn if too old
    DOCKER_VERSION=$(docker --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo "0.0")
    DOCKER_MAJOR=$(echo "${DOCKER_VERSION}" | cut -d. -f1)
    DOCKER_MINOR=$(echo "${DOCKER_VERSION}" | cut -d. -f2)
    
    # Docker 24.0+ supports API 1.44+, but we'll check for 20.10+ as minimum
    if [ "${DOCKER_MAJOR}" -lt 20 ] || ([ "${DOCKER_MAJOR}" -eq 20 ] && [ "${DOCKER_MINOR}" -lt 10 ]); then
        log_warn "Docker CLI version ${DOCKER_VERSION} may be too old (needs API 1.44+)"
        log_info "Updating Docker CLI..."
        brew upgrade docker || {
            log_warn "Failed to upgrade Docker CLI, continuing anyway"
            log_info "If you see API version errors, try: brew upgrade docker"
        }
    else
        log_success "Docker CLI is installed: $(docker --version 2>/dev/null || echo 'installed')"
    fi
fi

# Set DOCKER_API_VERSION to 1.44+ if not set (for compatibility with newer runtimes)
if [ -z "${DOCKER_API_VERSION:-}" ]; then
    export DOCKER_API_VERSION=1.44
    log_info "Set DOCKER_API_VERSION=1.44 for compatibility"
    
    # Add to shell profile for persistence
    SHELL_PROFILE=""
    if [ -f "${HOME}/.zprofile" ]; then
        SHELL_PROFILE="${HOME}/.zprofile"
    elif [ -f "${HOME}/.zshrc" ]; then
        SHELL_PROFILE="${HOME}/.zshrc"
    fi
    
    if [ -n "${SHELL_PROFILE}" ]; then
        if ! grep -q "DOCKER_API_VERSION" "${SHELL_PROFILE}" 2>/dev/null; then
            log_info "Adding DOCKER_API_VERSION to ${SHELL_PROFILE} for persistence..."
            echo "" >> "${SHELL_PROFILE}"
            echo "# Docker API version for k3d compatibility (minimum 1.44)" >> "${SHELL_PROFILE}"
            echo "export DOCKER_API_VERSION=1.44" >> "${SHELL_PROFILE}"
            log_success "DOCKER_API_VERSION added to ${SHELL_PROFILE}"
        fi
    fi
fi

# Install Podman (container runtime/daemon)
# Podman 5.0+ supports API 1.44+ compatibility
if ! check_command podman; then
    log_info "Installing Podman (container runtime)..."
    brew install podman
    log_success "Podman installed"
else
    # Check Podman version and warn if too old
    PODMAN_VERSION=$(podman --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo "0.0")
    PODMAN_MAJOR=$(echo "${PODMAN_VERSION}" | cut -d. -f1)
    PODMAN_MINOR=$(echo "${PODMAN_VERSION}" | cut -d. -f2)
    
    # Podman 4.0+ should work, but 5.0+ is better for API 1.44+ compatibility
    if [ "${PODMAN_MAJOR}" -lt 4 ]; then
        log_warn "Podman version ${PODMAN_VERSION} may be too old (needs 4.0+ for API 1.44+ compatibility)"
        log_info "Updating Podman..."
        brew upgrade podman || {
            log_warn "Failed to upgrade Podman, continuing anyway"
            log_info "If you see API version errors, try: brew upgrade podman"
        }
    else
        log_info "Podman already installed: $(podman --version)"
    fi
fi

# Handle Podman machine setup
MACHINE_NAME="podman-machine-default"
MACHINE_EXISTS=false

# Check if machine exists
if podman machine list 2>/dev/null | grep -q "${MACHINE_NAME}"; then
    MACHINE_EXISTS=true
    log_info "Podman machine '${MACHINE_NAME}' already exists"
    
    # Check if it's rootful
    MACHINE_CONFIG="${HOME}/.config/containers/podman/machine/applehv/${MACHINE_NAME}.json"
    if [ -f "${MACHINE_CONFIG}" ] && grep -q '"Rootful":true' "${MACHINE_CONFIG}" 2>/dev/null; then
        log_success "Machine is in rootful mode (good for k3d)"
    else
        log_warn "Machine is in rootless mode - k3d may have issues"
        log_info "To convert to rootful: ./scripts/convert-podman-to-rootful.sh"
    fi
else
    log_info "No Podman machine found, creating new one in rootful mode..."
    log_info "Rootful mode is required for k3d to work properly"
    log_info "Setting resources: 6 CPUs, 8GB RAM (recommended for k3d and workloads)"
    
    # Create machine in rootful mode with explicit resources
    podman machine init --rootful --cpus 6 --memory 8192 "${MACHINE_NAME}" || {
        log_error "Failed to create Podman machine"
        log_info "Try manually: podman machine init --rootful --cpus 6 --memory 8192 ${MACHINE_NAME}"
        exit 1
    }
    log_success "Podman machine created in rootful mode (6 CPUs, 8GB RAM)"
    MACHINE_EXISTS=true
fi

# Start Podman machine if not running
if [ "${MACHINE_EXISTS}" = "true" ]; then
    if ! podman machine list 2>/dev/null | grep -q "${MACHINE_NAME}.*running"; then
        log_info "Starting Podman machine..."
        podman machine start "${MACHINE_NAME}" || {
            log_error "Failed to start Podman machine"
            log_info "Try manually: podman machine start ${MACHINE_NAME}"
            exit 1
        }
        log_success "Podman machine started"
    else
        log_success "Podman machine is already running"
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

# Install Go (required for building operator)
if ! check_command go; then
    log_info "Installing Go..."
    brew install go
    log_success "Go installed"
else
    log_info "Go already installed: $(go version 2>/dev/null || echo 'installed')"
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

if check_command go; then
    log_success "✓ Go: $(go version 2>/dev/null || echo 'installed')"
else
    log_error "✗ Go not found"
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

# Setup /etc/hosts entries for local development
log_info "Setting up /etc/hosts entries for local development..."
HOST_IP="127.0.0.1"
HOSTS_FILE="/etc/hosts"
HOSTS_ENTRIES=(
  "glooscap-ui.testdev.dasmlab.org"
  "glooscap-operator.testdev.dasmlab.org"
)

# Check if we need sudo
NEEDS_SUDO=false
if [ ! -w "${HOSTS_FILE}" ]; then
    NEEDS_SUDO=true
fi

for HOSTNAME in "${HOSTS_ENTRIES[@]}"; do
    if grep -q "${HOSTNAME}" "${HOSTS_FILE}" 2>/dev/null; then
        # Entry exists, check if it points to correct IP
        if grep -q "^${HOST_IP}.*${HOSTNAME}" "${HOSTS_FILE}" 2>/dev/null; then
            log_info "  ✓ ${HOSTNAME} already configured in /etc/hosts"
        else
            log_warn "  ⚠ ${HOSTNAME} exists in /etc/hosts but points to different IP"
            log_info "    You may need to manually update /etc/hosts"
            log_info "    Expected: ${HOST_IP} ${HOSTNAME}"
        fi
    else
        # Entry doesn't exist, add it
        log_info "  Adding ${HOSTNAME} to /etc/hosts..."
        if [ "${NEEDS_SUDO}" = "true" ]; then
            if sudo sh -c "echo '${HOST_IP} ${HOSTNAME}' >> ${HOSTS_FILE}"; then
                log_success "  ✓ Added ${HOSTNAME} to /etc/hosts"
            else
                log_warn "  ⚠ Failed to add ${HOSTNAME} to /etc/hosts (requires sudo)"
                log_info "    Manually add this line to /etc/hosts:"
                log_info "    ${HOST_IP} ${HOSTNAME}"
            fi
        else
            if echo "${HOST_IP} ${HOSTNAME}" >> "${HOSTS_FILE}"; then
                log_success "  ✓ Added ${HOSTNAME} to /etc/hosts"
            else
                log_warn "  ⚠ Failed to add ${HOSTNAME} to /etc/hosts"
            fi
        fi
    fi
done

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

