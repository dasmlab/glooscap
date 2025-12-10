#!/usr/bin/env bash
# setup-linux-env.sh
# Sets up Linux environment for Glooscap development
# Installs Docker, k3s/k0s, kubectl, and other dependencies

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

# Check if running on Linux
if [[ "$(uname)" != "Linux" ]]; then
    log_error "This script is designed for Linux only"
    exit 1
fi

# Detect Linux distribution
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
    DISTRO_VERSION=$VERSION_ID
else
    log_error "Cannot detect Linux distribution"
    exit 1
fi

log_info "Detected Linux distribution: ${DISTRO} ${DISTRO_VERSION}"
log_info "Starting Linux environment setup for Glooscap development..."

# Check for sudo access
if ! sudo -n true 2>/dev/null; then
    log_warn "This script requires sudo access for some operations"
fi

# Install Docker
if ! check_command docker; then
    log_info "Installing Docker..."
    
    case $DISTRO in
        ubuntu|debian)
            # Update package index
            sudo apt-get update
            
            # Install prerequisites
            sudo apt-get install -y \
                ca-certificates \
                curl \
                gnupg \
                lsb-release
            
            # Add Docker's official GPG key
            sudo install -m 0755 -d /etc/apt/keyrings
            curl -fsSL https://download.docker.com/linux/${DISTRO}/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            sudo chmod a+r /etc/apt/keyrings/docker.gpg
            
            # Set up repository
            echo \
              "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${DISTRO} \
              $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
            
            # Install Docker Engine
            sudo apt-get update
            sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            
            log_success "Docker installed"
            ;;
        fedora|rhel|centos)
            # Install using dnf/yum
            if command -v dnf &> /dev/null; then
                sudo dnf install -y docker
            elif command -v yum &> /dev/null; then
                sudo yum install -y docker
            fi
            
            # Start and enable Docker
            sudo systemctl start docker
            sudo systemctl enable docker
            
            log_success "Docker installed"
            ;;
        *)
            log_warn "Unsupported distribution. Please install Docker manually."
            log_info "See: https://docs.docker.com/engine/install/"
            ;;
    esac
    
    # Add current user to docker group (if not root)
    if [ "$EUID" -ne 0 ]; then
        log_info "Adding current user to docker group..."
        sudo usermod -aG docker $USER
        log_warn "You may need to log out and back in for docker group changes to take effect"
    fi
else
    log_info "Docker already installed: $(docker --version)"
    
    # Check if Docker daemon is running
    if ! docker info &> /dev/null; then
        log_warn "Docker daemon is not running. Starting it..."
        sudo systemctl start docker || log_warn "Could not start Docker. Please start it manually."
    fi
fi

# Install kubectl
if ! check_command kubectl; then
    log_info "Installing kubectl..."
    
    # Download kubectl
    KUBECTL_VERSION="${KUBECTL_VERSION:-latest}"
    if [[ "${KUBECTL_VERSION}" == "latest" ]]; then
        KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
    fi
    
    curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
    sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl
    
    log_success "kubectl installed"
else
    log_info "kubectl already installed: $(kubectl version --client --short 2>/dev/null || echo 'installed')"
fi

# Install k3s
if ! check_command k3s; then
    log_info "Installing k3s..."
    
    # Use the official k3s install script
    curl -sfL https://get.k3s.io | sh -
    
    log_success "k3s installed"
    
    # Add k3s to PATH if needed
    if [[ ":$PATH:" != *":/usr/local/bin:"* ]]; then
        log_info "k3s installed to /usr/local/bin (should be in PATH)"
    fi
else
    log_info "k3s already installed: $(k3s --version 2>/dev/null || echo 'installed')"
fi

# Install Helm (optional)
if [[ "${INSTALL_HELM:-false}" == "true" ]]; then
    if ! check_command helm; then
        log_info "Installing Helm..."
        curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
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
            K0S_URL=$(curl -s https://api.github.com/repos/k0sproject/k0s/releases/latest | grep "browser_download_url.*linux-amd64" | cut -d '"' -f 4)
        else
            K0S_URL="https://github.com/k0sproject/k0s/releases/download/v${K0S_VERSION}/k0s-v${K0S_VERSION}-linux-amd64"
        fi
        
        curl -L "${K0S_URL}" -o "${K0S_DIR}/k0s"
        chmod +x "${K0S_DIR}/k0s"
        
        # Add to PATH if not already there
        if [[ ":$PATH:" != *":${K0S_DIR}:"* ]]; then
            log_info "Adding ${K0S_DIR} to PATH in ~/.bashrc"
            echo "export PATH=\"${K0S_DIR}:\$PATH\"" >> "${HOME}/.bashrc"
            export PATH="${K0S_DIR}:${PATH}"
        fi
        
        log_success "k0s installed"
    else
        log_info "k0s already installed: $(k0s version 2>/dev/null || echo 'installed')"
    fi
fi

# Create kubeconfig directory
mkdir -p "${HOME}/.kube"

# Verify installations
log_info "Verifying installations..."

if check_command docker; then
    log_success "✓ Docker: $(docker --version)"
    if docker info &> /dev/null; then
        log_success "✓ Docker daemon is running"
    else
        log_warn "⚠ Docker daemon may not be running or user not in docker group"
    fi
else
    log_error "✗ Docker not found"
fi

if check_command kubectl; then
    log_success "✓ kubectl: $(kubectl version --client --short 2>/dev/null | head -n1 || echo 'installed')"
else
    log_error "✗ kubectl not found"
fi

if check_command k3s; then
    log_success "✓ k3s: $(k3s --version 2>/dev/null || echo 'installed')"
else
    log_warn "⚠ k3s not found"
fi

# Summary
log_success "Linux environment setup complete!"
echo ""
log_info "Next steps:"
echo "  1. If you were added to the docker group, log out and back in"
echo "  2. Run './scripts/start-k3s.sh' to start the k3s cluster"
echo "  3. Run './scripts/deploy-glooscap.sh' to deploy Glooscap"
echo ""
log_warn "Note: k3s requires sudo access to manage the cluster"

