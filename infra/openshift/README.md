# Glooscap OpenShift Deployment

This directory contains OpenShift-specific manifests and configuration for deploying Glooscap to an OpenShift cluster.

## Overview

Glooscap consists of two main components:
- **Glooscap Operator**: Kubernetes operator for managing WikiTargets and TranslationJobs
- **Glooscap UI**: Web interface for managing Glooscap

This setup provides:
- **Production-ready deployment**: Uses production images from `ghcr.io/dasmlab`
- **OpenShift security**: Proper security contexts and SCC compliance
- **Service and Routes**: Internal services and external routes for UI and API access

## Prerequisites

- OpenShift cluster (4.x+)
- `dasmlab-ghcr-pull` image pull secret in the `glooscap-system` namespace
- `oc` CLI tool installed and configured

## Building and Pushing Images

Before deploying, you need to build and push the images to the registry.

### Option 1: Using Helper Scripts (Recommended)

From the project root:

```bash
# Build and push operator
cd operator
./buildme.sh
./pushme.sh

# Build and push UI
cd ../ui
./buildme.sh
./pushme.sh
```

Or use the convenience script:

```bash
# Operator
cd operator
./cycleme.sh

# UI
cd ../ui
./cycleme.sh  # if available
```

### Option 2: Using Make (Operator Only)

```bash
cd operator
make docker-build IMG=ghcr.io/dasmlab/glooscap:latest
make docker-push IMG=ghcr.io/dasmlab/glooscap:latest
```

### Image Tags

The helper scripts (`pushme.sh`) will:
- Tag images with version tags: `ghcr.io/dasmlab/glooscap:0.2.X-alpha`
- Tag images with `latest`: `ghcr.io/dasmlab/glooscap:latest`
- Automatically increment build numbers

For production deployments, update the manifests to use specific version tags instead of `latest`:

```yaml
image: ghcr.io/dasmlab/glooscap:0.2.42-alpha
```

## Creating Image Pull Secret

If not already created, create the image pull secret:

```bash
oc create secret docker-registry dasmlab-ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=lmcdasm \
  --docker-password=<your_github_token> \
  --namespace=glooscap-system
```

Or use the helper script:

```bash
cd operator
./create-registry-secret.sh
```

## Deployment

### 1. Create Namespace

```bash
oc create namespace glooscap-system
```

### 2. Deploy Operator

The operator is typically deployed using the operator-sdk or OLM. For manual deployment:

```bash
# Apply CRDs first
oc apply -f operator/config/crd/bases/

# Deploy operator
oc apply -f operator/config/default/
```

Or use the operator's install.yaml:

```bash
oc apply -f operator/dist/install.yaml
```

### 3. Deploy UI

```bash
oc apply -f glooscap-ui.yaml
```

### 4. Create API Route

```bash
oc apply -f operator-api-route.yaml
```

**Important**: Update the `host` field in `operator-api-route.yaml` with your cluster domain:

```yaml
spec:
  host: api-glooscap.apps.<your-cluster-domain>
```

Also update the `API_BASE_URL` environment variable in `glooscap-ui.yaml`:

```yaml
env:
  - name: API_BASE_URL
    value: https://api-glooscap.apps.<your-cluster-domain>/api/v1
```

## Verify Deployment

```bash
# Check operator pods
oc get pods -n glooscap-system

# Check UI pods
oc get pods -n glooscap-system -l app=glooscap-ui

# Check services
oc get svc -n glooscap-system

# Check routes
oc get route -n glooscap-system

# View operator logs
oc logs -f deployment/glooscap-controller-manager -n glooscap-system

# View UI logs
oc logs -f deployment/glooscap-ui -n glooscap-system
```

## Configuration

### Update Image Tags

To use a specific image version, edit the deployment manifests:

**Operator** (in `operator/config/default/manager_image_patch.yaml` or deployment):
```yaml
image: ghcr.io/dasmlab/glooscap:0.2.42-alpha
```

**UI** (in `glooscap-ui.yaml`):
```yaml
image: ghcr.io/dasmlab/glooscap-ui:0.2.42-alpha
```

### Update Cluster Domain

Edit the following files and update the cluster domain:

1. `operator-api-route.yaml`: Update `spec.host`
2. `glooscap-ui.yaml`: Update `env[].value` for `API_BASE_URL`

## Integration with Translation Services

After deploying Glooscap, configure it to use a translation service (e.g., Iskoces):

1. **Via Glooscap UI:**
   - Go to Settings → Translation Service
   - Set Address: `iskoces-service.iskoces.svc:50051`
   - Set Type: `iskoces` (or `nanabush`)
   - Set Secure: `false` (or `true` for TLS)

2. **Via Operator Config:**
   ```yaml
   env:
   - name: TRANSLATION_SERVICE_ADDR
     value: "iskoces-service.iskoces.svc:50051"
   - name: TRANSLATION_SERVICE_TYPE
     value: "iskoces"
   - name: TRANSLATION_SERVICE_SECURE
     value: "false"
   ```

## Troubleshooting

### Pods not starting
- Check image pull secret: `oc get secret dasmlab-ghcr-pull -n glooscap-system`
- Check pod events: `oc describe pod <pod-name> -n glooscap-system`
- Check image exists: `oc get imagestreamtag -n glooscap-system`

### UI not accessible
- Verify route: `oc get route glooscap-ui -n glooscap-system`
- Check service endpoints: `oc get endpoints glooscap-ui -n glooscap-system`
- Check UI logs for errors

### Operator not working
- Check operator logs: `oc logs -f deployment/glooscap-controller-manager -n glooscap-system`
- Verify CRDs are installed: `oc get crd | grep glooscap`
- Check operator status: `oc get deployment glooscap-controller-manager -n glooscap-system`

### API not accessible
- Verify API route: `oc get route glooscap-operator-api -n glooscap-system`
- Check CORS configuration if accessing from external UI
- Verify service: `oc get svc operator-glooscap-operator-api -n glooscap-system`

## Cleanup

To remove Glooscap:

```bash
# Delete UI
oc delete -f glooscap-ui.yaml

# Delete API route
oc delete -f operator-api-route.yaml

# Delete operator (if deployed manually)
oc delete -f operator/config/default/

# Delete CRDs (be careful - this removes all custom resources)
oc delete -f operator/config/crd/bases/

# Delete namespace (removes everything)
oc delete namespace glooscap-system
```

## Additional Resources

- **Keycloak**: OIDC authentication setup (see `keycloak/README.md`)
- **Outline**: Wiki integration (see `outline/README.md`)
- **WikiTargets**: Example WikiTarget CR (see `wikitarget-infra-dasmlab-org.yaml`)

