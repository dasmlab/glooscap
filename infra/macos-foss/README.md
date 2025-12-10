# Glooscap macOS FOSS Setup

This directory contains everything needed to run Glooscap on macOS using Podman and k3d (k3s in containers).

## Overview

This setup provides a fully FOSS (Free and Open Source Software) stack for running Glooscap locally on macOS:

- **Podman**: Container runtime (Docker alternative)
- **k3d**: Runs k3s inside Docker/Podman containers (recommended for macOS)
- **kubectl**: Kubernetes CLI
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

2. **Start the k3d cluster (k3s in containers, recommended for macOS):**
   ```bash
   ./scripts/start-k3d.sh
   ```
   
   **Note**: k3d runs k3s inside Podman/Docker containers, avoiding the systemd requirement that native k3s has.

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

### Step 2: Start k3d Cluster

k3d runs k3s inside Docker/Podman containers, which:
- Works on macOS (no systemd requirement)
- Uses Podman automatically if available (or Docker as fallback)
- Creates a lightweight Kubernetes cluster in containers

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
    ├── start-k3d.sh         # Start k3d cluster (k3s in containers)
    ├── stop-k3d.sh          # Stop k3d cluster
    ├── deploy-glooscap.sh   # Deploy Glooscap
    └── undeploy-glooscap.sh # Remove Glooscap
```

## Configuration

### k3d Configuration

k3d stores cluster data in Docker/Podman containers. The kubeconfig file is at:
```
~/.kube/config
```

k3d automatically manages the kubeconfig when you create/start clusters.

### Podman Configuration

Podman stores images and containers in:
```
~/.local/share/containers/storage
```

## Troubleshooting

### k3d won't start
- Ensure Podman machine is running: `podman machine start`
- Check if port 6443 is already in use
- Check cluster status: `k3d cluster list`
- Check container logs: `podman logs k3d-glooscap-server-0`

### Podman issues
- Ensure Podman machine is running: `podman machine start`
- Check Podman info: `podman info`

### Image pull errors
- Ensure images are built and available locally
- Check image pull secrets if using private registries
- For local development, use `imagePullPolicy: Never` or `IfNotPresent`

## Why k3d?

k3d is the recommended solution for macOS because:
- k3s doesn't work natively on macOS (requires systemd/openrc)
- k0s doesn't support macOS (darwin)
- k3d runs k3s inside containers, avoiding these limitations
- Works seamlessly with Podman (FOSS-compliant)

## Next Steps

- Configure translation service (Iskoces or Nanabush)
- Create WikiTarget resources
- Set up Outline wiki connections
- Start translating!

## Support

For issues or questions, please refer to the main Glooscap documentation or open an issue in the repository.

