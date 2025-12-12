# Glooscap macOS FOSS Quick Start Guide

This guide will help you get Glooscap running on macOS in just a few steps.

## Prerequisites

- macOS 12 or later
- Administrator access (for some installations)
- Internet connection
- GitHub Personal Access Token with `write:packages` permission

## Quick Installation (Recommended)

For most users, the simplest way to install Glooscap:

```bash
cd infra/macos-foss
export DASMLAB_GHCR_PAT=your_github_token
./install_glooscap.sh
```

This single command will:
1. Install all dependencies (Docker CLI, Podman, k3d, kubectl, Go)
2. Start the Kubernetes cluster
3. Create registry credentials
4. Build and push architecture-specific images
5. Deploy Glooscap operator and UI

### Access the Services

After installation completes, services are accessible directly:

- **UI**: http://localhost:30080
- **Operator API**: http://localhost:30000
- **Operator Health**: http://localhost:30081/healthz

No port-forwarding needed!

### Uninstall

To remove everything:

```bash
./uninstall_glooscap.sh
```

---

## Manual Installation (For Developers)

If you prefer to run the steps individually or are developing Glooscap:

### Step 1: Setup macOS Environment

```bash
cd infra/macos-foss
./scripts/setup-macos-env.sh
```

This installs:
- Homebrew (if not already installed)
- Docker CLI (for k3d compatibility)
- Podman (container runtime)
- kubectl (Kubernetes CLI)
- k3d (runs k3s in containers)
- Go (for building operator)

### Step 2: Start Kubernetes Cluster

```bash
./scripts/start-k3d.sh
```

This creates a k3d cluster (k3s in Podman containers).

### Step 3: Create Registry Credentials

```bash
export DASMLAB_GHCR_PAT=your_github_token
./scripts/create-registry-secret.sh
```

### Step 4: Build and Push Images

```bash
./scripts/build-and-load-images.sh
```

This builds architecture-specific images and pushes them to `ghcr.io/dasmlab`.

### Step 5: Deploy Glooscap

```bash
./scripts/deploy-glooscap.sh
```

This deploys the operator and UI to the cluster.

### Step 6: Access the Services

Services are accessible directly on host ports:

- **UI**: http://localhost:30080
- **Operator API**: http://localhost:30000
- **Operator Health**: http://localhost:30081/healthz

## Step 7: Configure Translation Service

1. Open the Glooscap UI (http://localhost:8080)
2. Go to Settings → Translation tab
3. Configure your translation service:
   - **Address**: e.g., `iskoces-service.iskoces.svc:50051` (if Iskoces is deployed)
   - **Type**: `iskoces` or `nanabush`
   - **Secure**: `false` (for local development)
4. Click "Set Configuration"

## Step 8: Create WikiTargets

1. In the UI, go to Settings → WikiTargets tab
2. Click "Add WikiTarget"
3. Fill in the details:
   - **Name**: A unique name for your wiki target
   - **Namespace**: `glooscap-system` (default)
   - **Wiki URI**: The base URL of your Outline wiki
   - **Secret Name**: Name of the Kubernetes secret containing API credentials
   - **Secret Key**: Key in the secret (default: `token`)
   - **Mode**: `ReadOnly`, `ReadWrite`, or `PushOnly`
4. Click "Create"

## Troubleshooting

### k3d won't start

- Ensure Docker Desktop is running
- Check if ports are in use: `lsof -i :6443`
- Check k3d cluster status: `k3d cluster list`
- Check container logs: `docker logs k3d-${CLUSTER_NAME}-server-0` (replace CLUSTER_NAME)
- Try stopping and restarting: `./scripts/stop-k3d.sh && ./scripts/start-k3d.sh`

### Pods stuck in ImagePullBackOff

- Ensure images are built and loaded (see Step 4)
- Check image pull policy in deployment manifests
- For local development, use `imagePullPolicy: Never`

### Can't connect to cluster

- Verify k3d cluster is running: `k3d cluster list` and `kubectl cluster-info`
- Check kubeconfig: `kubectl config view`
- Ensure kubectl is using the correct context: `kubectl config current-context`

### Operator not starting

- Check operator logs: `kubectl logs -n glooscap-system deployment/glooscap-operator`
- Verify CRDs are installed: `kubectl get crd`
- Check RBAC: `kubectl get clusterrolebinding glooscap-operator-rolebinding`

## Cleanup

To remove Glooscap from the cluster:

```bash
./scripts/undeploy-glooscap.sh
```

To stop k3d:

```bash
./scripts/stop-k3d.sh
```

To completely clean up (including cluster):

```bash
DELETE_CLUSTER=true ./scripts/stop-k3d.sh
```

## Next Steps

- Deploy Iskoces for lightweight translation (see Iskoces documentation)
- Configure WikiTargets for your Outline wikis
- Create TranslationJobs to start translating content
- Explore the Glooscap UI features

## Getting Help

- Check the main [README.md](README.md) for more details
- Review [manifests/README.md](manifests/README.md) for manifest documentation
- Check operator logs for detailed error messages
- Open an issue in the repository if you encounter problems

