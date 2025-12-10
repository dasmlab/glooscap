# Glooscap macOS FOSS Setup

This directory contains everything needed to run Glooscap on macOS using Podman and k3s/k0s (lightweight Kubernetes).

## Overview

This setup provides a fully FOSS (Free and Open Source Software) stack for running Glooscap locally on macOS:

- **Podman**: Container runtime (Docker alternative)
- **k0s**: Single binary Kubernetes distribution (recommended for macOS)
- **k3s**: Alternative Kubernetes (not recommended on macOS - requires systemd/openrc)
- **kubectl**: Kubernetes CLI
- **Glooscap Operator**: Deployed via Kubernetes manifests
- **Glooscap UI**: Deployed via Kubernetes manifests

## Prerequisites

- macOS (tested on macOS 12+)
- Homebrew (for package management)
- Administrator access (for some installations)

## Quick Start

1. **Run the macOS environment setup:**
   ```bash
   ./scripts/setup-macos-env.sh
   ```

2. **Start the k0s cluster (recommended for macOS):**
   ```bash
   ./scripts/start-k0s.sh
   ```
   
   **Note**: k3s is not recommended on macOS as it requires systemd/openrc which macOS doesn't provide.

3. **Deploy Glooscap:**
   ```bash
   ./scripts/deploy-glooscap.sh
   ```

4. **Access the UI:**
   ```bash
   kubectl port-forward -n glooscap-system svc/glooscap-ui 8080:80
   ```
   Then open http://localhost:8080 in your browser.

## Detailed Setup

### Step 1: Install Dependencies

The `setup-macos-env.sh` script will install:
- Podman Desktop (or Podman via Homebrew)
- k3s (lightweight Kubernetes)
- kubectl (Kubernetes CLI)
- Helm (optional, for future use)

### Step 2: Start k3s Cluster

k3s can run in two modes:
- **Embedded mode**: Single binary with built-in containerd
- **External mode**: Use Podman as the container runtime

We recommend starting with embedded mode for simplicity.

### Step 3: Deploy Glooscap

The deployment scripts will:
1. Create the `glooscap-system` namespace
2. Apply CRDs (Custom Resource Definitions)
3. Deploy the operator
4. Deploy the UI
5. Create necessary RBAC resources

## Directory Structure

```
macos-foss/
├── README.md                 # This file
├── manifests/                # Kubernetes manifests
│   ├── namespace.yaml       # Namespace definition
│   ├── crds/                # Custom Resource Definitions
│   ├── operator/            # Operator deployment
│   ├── ui/                  # UI deployment
│   └── rbac/                # RBAC resources
└── scripts/                 # Setup and deployment scripts
    ├── setup-macos-env.sh   # macOS environment setup
    ├── start-k3s.sh         # Start k3s cluster
    ├── stop-k3s.sh          # Stop k3s cluster
    ├── deploy-glooscap.sh   # Deploy Glooscap
    └── undeploy-glooscap.sh # Remove Glooscap
```

## Configuration

### k3s Configuration

k3s configuration is stored in `~/.k3s/` by default. The kubeconfig file is at:
```
~/.kube/config
```

### Podman Configuration

Podman stores images and containers in:
```
~/.local/share/containers/storage
```

## Troubleshooting

### k3s won't start
- Check if port 6443 is already in use
- Ensure you have sufficient disk space
- Check logs: `journalctl -u k3s` (if using systemd) or `k3s server --debug`

### Podman issues
- Ensure Podman machine is running: `podman machine start`
- Check Podman info: `podman info`

### Image pull errors
- Ensure images are built and available locally
- Check image pull secrets if using private registries
- For local development, use `imagePullPolicy: Never` or `IfNotPresent`

## Alternative: k0s

k0s is a single-binary Kubernetes distribution that can also be used. See `scripts/start-k0s.sh` for k0s setup.

## Next Steps

- Configure translation service (Iskoces or Nanabush)
- Create WikiTarget resources
- Set up Outline wiki connections
- Start translating!

## Support

For issues or questions, please refer to the main Glooscap documentation or open an issue in the repository.

