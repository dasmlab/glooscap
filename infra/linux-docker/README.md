# Glooscap Linux Docker Setup

This directory contains everything needed to run Glooscap on Linux using Docker and k3s/k0s (lightweight Kubernetes).

## Overview

This setup provides a development stack for running Glooscap locally on Linux:

- **Docker**: Container runtime
- **k3s/k0s**: Lightweight Kubernetes distribution
- **kubectl**: Kubernetes CLI
- **Glooscap Operator**: Deployed via Kubernetes manifests
- **Glooscap UI**: Deployed via Kubernetes manifests

## Prerequisites

- Linux (tested on Ubuntu/Debian, but should work on most distributions)
- Docker installed and running
- Administrator/sudo access (for some installations)
- curl and wget (usually pre-installed)

## Quick Start

1. **Run the Linux environment setup:**
   ```bash
   ./scripts/setup-linux-env.sh
   ```

2. **Start the k3s cluster:**
   ```bash
   ./scripts/start-k3s.sh
   ```

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

The `setup-linux-env.sh` script will install:
- Docker (if not already installed)
- k3s (lightweight Kubernetes)
- kubectl (Kubernetes CLI)
- Helm (optional, for future use)

### Step 2: Start k3s Cluster

k3s can run in two modes:
- **Embedded mode**: Single binary with built-in containerd (default)
- **External mode**: Use Docker as the container runtime (requires additional configuration)

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
linux-docker/
├── README.md                 # This file
├── QUICKSTART.md             # Quick start guide
├── manifests/                # Kubernetes manifests
│   ├── namespace.yaml       # Namespace definition
│   ├── crd/                 # Custom Resource Definitions
│   ├── operator/            # Operator deployment
│   ├── ui/                  # UI deployment
│   └── rbac/                # RBAC resources
└── scripts/                 # Setup and deployment scripts
    ├── setup-linux-env.sh   # Linux environment setup
    ├── start-k3s.sh         # Start k3s cluster
    ├── stop-k3s.sh          # Stop k3s cluster
    ├── copy-crds.sh         # Copy CRDs from operator
    ├── deploy-glooscap.sh   # Deploy Glooscap
    └── undeploy-glooscap.sh # Remove Glooscap
```

## Configuration

### k3s Configuration

k3s configuration is stored in `/var/lib/rancher/k3s/` by default (requires sudo). The kubeconfig file is at:
```
~/.kube/config
```

### Docker Configuration

Docker stores images and containers in:
```
/var/lib/docker
```

## Troubleshooting

### k3s won't start
- Check if port 6443 is already in use: `sudo netstat -tlnp | grep 6443`
- Ensure you have sufficient disk space
- Check logs: `sudo journalctl -u k3s` (if using systemd) or `sudo k3s server --debug`

### Docker issues
- Ensure Docker daemon is running: `sudo systemctl status docker`
- Check Docker info: `docker info`
- Ensure user is in docker group: `sudo usermod -aG docker $USER` (requires logout/login)

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

