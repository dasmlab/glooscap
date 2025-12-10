# Glooscap macOS FOSS Quick Start Guide

This guide will help you get Glooscap running on macOS using Podman and k3s in just a few steps.

## Prerequisites

- macOS 12 or later
- Administrator access (for some installations)
- Internet connection

## Step 1: Setup macOS Environment

Run the setup script to install all required dependencies:

```bash
cd infra/macos-foss
./scripts/setup-macos-env.sh
```

This will install:
- Homebrew (if not already installed)
- Podman (container runtime)
- kubectl (Kubernetes CLI)
- k3s (lightweight Kubernetes)

**Note**: After installation, you may need to restart your terminal or run:
```bash
export PATH="${HOME}/.local/bin:$PATH"
```

## Step 2: Start k3d Cluster

**Important**: k3s doesn't work natively on macOS (requires systemd/openrc). Use k3d instead, which runs k3s inside containers.

Start a local k3d cluster:

```bash
./scripts/start-k3d.sh
```

This will:
- Create a k3d cluster (k3s running in Podman/Docker containers)
- Configure kubectl to use the cluster
- Wait for the cluster to be ready
- Set up port mappings for services

**Note**: k3d is the recommended option for macOS as it runs k3s inside containers, avoiding the systemd requirement.

**To stop k3d later:**
```bash
./scripts/stop-k3d.sh
```

**To delete the cluster completely:**
```bash
DELETE_CLUSTER=true ./scripts/stop-k3d.sh
```

## Step 3: Prepare CRDs

Copy the Custom Resource Definitions from the operator:

```bash
./scripts/copy-crds.sh
```

This copies the CRD YAML files from `operator/config/crd/bases/` to `manifests/crd/`.

## Step 4: Build and Load Images (Optional)

If you're building images locally, you'll need to build and load them into k3s:

### Build Operator Image

```bash
cd ../../operator
podman build -t ghcr.io/dasmlab/glooscap-operator:latest .
```

### Build UI Image

```bash
cd ../ui
podman build -t ghcr.io/dasmlab/glooscap-ui:latest .
```

### Load Images into k3s

k3s uses containerd, so you need to import images. The easiest way is to use a local registry or import directly:

```bash
# Save images
podman save ghcr.io/dasmlab/glooscap-operator:latest -o /tmp/operator.tar
podman save ghcr.io/dasmlab/glooscap-ui:latest -o /tmp/ui.tar

# Import into k3s (k3s must be running)
sudo k3s ctr images import /tmp/operator.tar
sudo k3s ctr images import /tmp/ui.tar
```

**Note**: For development, you can also set `imagePullPolicy: Never` in the deployment manifests to use local images.

## Step 5: Deploy Glooscap

Deploy the operator and UI:

```bash
./scripts/deploy-glooscap.sh
```

This will:
1. Create the `glooscap-system` namespace
2. Apply CRDs
3. Deploy RBAC resources
4. Deploy the operator
5. Deploy the UI

Wait for all pods to be ready:
```bash
kubectl get pods -n glooscap-system -w
```

## Step 6: Access the UI

Port-forward the UI service:

```bash
kubectl port-forward -n glooscap-system svc/glooscap-ui 8080:80
```

Then open http://localhost:8080 in your browser.

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

- Ensure Podman machine is running: `podman machine start`
- Check if ports are in use: `lsof -i :6443`
- Check k3d cluster status: `k3d cluster list`
- Check container logs: `podman logs k3d-${CLUSTER_NAME}-server-0` (replace CLUSTER_NAME)
- Try stopping and restarting: `./scripts/stop-k3d.sh && ./scripts/start-k3d.sh`

### Pods stuck in ImagePullBackOff

- Ensure images are built and loaded (see Step 4)
- Check image pull policy in deployment manifests
- For local development, use `imagePullPolicy: Never`

### Can't connect to cluster

- Verify k3s is running: `kubectl cluster-info`
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

To stop k3s:

```bash
./scripts/stop-k3s.sh
```

To completely clean up (including data):

```bash
CLEAN_DATA=true ./scripts/stop-k3s.sh
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

