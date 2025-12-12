# Glooscap macOS FOSS Setup

This directory contains everything needed to run Glooscap on macOS using Podman and k3d (k3s in containers).

## Overview

This setup provides a fully FOSS (Free and Open Source Software) stack for running Glooscap locally on macOS:

- **Docker CLI**: Container CLI (for k3d compatibility)
- **Podman**: Container runtime/daemon (FOSS alternative to Docker Desktop)
- **k3d**: k3s in Podman containers (lightweight, no Docker Desktop needed)
- **kubectl**: Kubernetes CLI
- **Glooscap Operator**: Deployed via Kubernetes manifests
- **Glooscap UI**: Deployed via Kubernetes manifests

## Prerequisites

- macOS (tested on macOS 12+)
- Homebrew (for package management)
- Administrator access (for some installations)
- GitHub Personal Access Token with `write:packages` permission (for pushing images)

## For End Users: Simple Installation

If you just want to **install and use Glooscap**, use these simple scripts:

### Install Glooscap

```bash
cd infra/macos-foss
export DASMLAB_GHCR_PAT=your_github_token
./install_glooscap.sh
```

This will:
1. Set up all dependencies (Docker CLI, Podman, k3d, kubectl, Go)
2. Start the Kubernetes cluster
3. Create registry credentials
4. Build and push architecture-specific images
5. Deploy Glooscap operator and UI

### Access the Services

After installation, services are accessible directly on host ports:

- **UI**: http://localhost:30080
- **Operator API**: http://localhost:30000
- **Operator Health**: http://localhost:30081/healthz

No port-forwarding needed!

### Uninstall Glooscap

To remove everything:

```bash
./uninstall_glooscap.sh
```

This will:
1. Remove Glooscap deployment
2. Stop the Kubernetes cluster
3. Remove the cluster

---

## For Developers: Advanced Usage

If you're **developing or testing Glooscap**, you can use the individual scripts or the full cycle test:

### Individual Scripts

See the [scripts/](scripts/) directory for individual scripts:
- `setup-macos-env.sh` - Install dependencies
- `start-k3d.sh` - Start cluster
- `build-and-load-images.sh` - Build and push images
- `deploy-glooscap.sh` - Deploy Glooscap
- `undeploy-glooscap.sh` - Remove Glooscap
- `stop-k3d.sh` - Stop cluster
- `remove-k3d.sh` - Remove cluster

### Full Cycle Test

Run the complete development cycle (setup → start → build → deploy → undeploy → stop → remove):

```bash
export DASMLAB_GHCR_PAT=your_github_token
./scripts/cycle-test.sh
```

This is useful for:
- Testing the complete setup process
- Verifying all scripts work correctly
- CI/CD validation

## Architecture

### Container Runtime

- **Podman**: FOSS container runtime (replaces Docker Desktop)
- **Docker CLI**: Used by k3d, connects to Podman via `DOCKER_HOST`
- Podman runs in a lightweight VM (managed automatically)

### Kubernetes

- **k3d**: Runs k3s inside Podman containers
- No systemd requirement (works on macOS)
- Lightweight and fast startup
- Perfect for local development

### Image Management

- Images are built locally for your architecture (ARM64/AMD64)
- Tagged with architecture-specific tags: `local-arm64`, `local-amd64`
- Pushed to `ghcr.io/dasmlab` for cluster to pull
- Allows parallel development on different architectures

## Directory Structure

```
macos-foss/
├── README.md                 # This file (user/developer guide)
├── QUICKSTART.md            # Quick start guide
├── install_glooscap.sh      # Simple install script (for users)
├── uninstall_glooscap.sh    # Simple uninstall script (for users)
├── manifests/                # Kubernetes manifests
│   ├── namespace.yaml       # Namespace definition
│   ├── crds/                # Custom Resource Definitions
│   ├── operator/            # Operator deployment
│   ├── ui/                  # UI deployment
│   └── rbac/                # RBAC resources
└── scripts/                 # Individual scripts (for developers)
    ├── setup-macos-env.sh   # macOS environment setup
    ├── start-k3d.sh         # Start k3d cluster
    ├── stop-k3d.sh          # Stop k3d cluster
    ├── remove-k3d.sh        # Remove k3d cluster
    ├── build-and-load-images.sh  # Build and push images
    ├── create-registry-secret.sh  # Create registry credentials
    ├── deploy-glooscap.sh   # Deploy Glooscap
    ├── undeploy-glooscap.sh # Remove Glooscap
    └── cycle-test.sh        # Full cycle test (for developers)
```

## Configuration

### k3d Configuration

k3d stores cluster data in Docker containers. The kubeconfig file is at:
```
~/.kube/config
```

k3d automatically manages the kubeconfig when you create/start clusters.

### Podman Configuration

Podman stores images and containers in:
```
~/.local/share/containers/storage
```

The Docker CLI connects to Podman via `DOCKER_HOST` environment variable, which is automatically configured by the setup scripts.

## Troubleshooting

### k3d won't start
- Ensure Podman machine is running: `podman machine start`
- Check if `DOCKER_HOST` is set: `echo $DOCKER_HOST` (should point to Podman socket)
- Verify container runtime: `./scripts/check-docker.sh`
- Check if port 6443 is already in use
- Check cluster status: `k3d cluster list`
- Check Podman containers: `podman ps`

### Container runtime issues
- Ensure Podman machine is running: `podman machine start`
- Check if `DOCKER_HOST` is set correctly
- Verify with: `./scripts/check-docker.sh`
- Check Docker info: `docker info` (should show Podman backend)

### Image pull errors
- Ensure images are built and available locally
- Check image pull secrets if using private registries
- For local development, use `imagePullPolicy: Never` or `IfNotPresent`

## Why k3d?

k3d is the recommended solution for macOS because:
- k3s doesn't work natively on macOS (requires systemd/openrc)
- k0s doesn't support macOS (darwin)
- k3d runs k3s inside containers, avoiding these limitations
- Works seamlessly with Docker

## Next Steps

- Configure translation service (Iskoces or Nanabush)
- Create WikiTarget resources
- Set up Outline wiki connections
- Start translating!

## Support

For issues or questions, please refer to the main Glooscap documentation or open an issue in the repository.

