# Glooscap Kubernetes Manifests

This directory contains Kubernetes manifests for deploying Glooscap on any Kubernetes cluster (k3d for local development, or any other Kubernetes distribution).

## Directory Structure

```
manifests/
├── namespace.yaml          # Namespace definition
├── crd/                    # Custom Resource Definitions
│   ├── wiki.glooscap.dasmlab.org_wikitargets.yaml
│   └── wiki.glooscap.dasmlab.org_translationjobs.yaml
├── rbac/                   # RBAC resources
│   ├── service_account.yaml
│   ├── role.yaml
│   └── role_binding.yaml
├── operator/               # Operator deployment
│   ├── deployment.yaml
│   └── service.yaml
└── ui/                     # UI deployment
    ├── deployment.yaml
    └── service.yaml
```

## CRDs

The Custom Resource Definitions (CRDs) define the Glooscap API resources:
- `WikiTarget`: Defines a wiki source or destination
- `TranslationJob`: Defines a translation task

These CRDs should be generated from the operator source:
```bash
cd ../../operator
make manifests
cp config/crd/bases/*.yaml ../infra/macos-foss/manifests/crd/
```

## Deployment Order

Manifests should be applied in this order:

1. **Namespace**: Creates the `glooscap-system` namespace
2. **CRDs**: Registers the custom resource definitions
3. **RBAC**: Creates service account, roles, and bindings
4. **Operator**: Deploys the operator
5. **UI**: Deploys the UI

The `deploy-glooscap.sh` script handles this automatically.

## Image Configuration

For local development, you may need to:

1. **Build images locally** using Docker:
   ```bash
   # Build operator
   cd ../../operator
   docker build -t ghcr.io/dasmlab/glooscap-operator:latest .
   
   # Build UI
   cd ../ui
   docker build -t ghcr.io/dasmlab/glooscap-ui:latest .
   ```

2. **Load images into k3d**:
   ```bash
   # k3d uses Docker directly, so images are automatically available
   # Images built with Docker are accessible to k3d
   # Or import explicitly if needed:
   # k3d image import ghcr.io/dasmlab/glooscap-operator:latest -c glooscap
   # k3d image import ghcr.io/dasmlab/glooscap-ui:latest -c glooscap
   ```

3. **Update imagePullPolicy** in the deployment manifests:
   - Change `imagePullPolicy: Always` to `imagePullPolicy: Never` or `IfNotPresent`

## Customization

### Operator Configuration

Edit `operator/deployment.yaml` to customize:
- Environment variables
- Resource limits
- Replica count
- Image version

### UI Configuration

Edit `ui/deployment.yaml` to customize:
- API base URL
- Resource limits
- Replica count
- Image version

## Troubleshooting

### Images not found

If pods fail with `ImagePullBackOff`:
1. Ensure images are built and available in your local Docker registry
2. Check `imagePullPolicy` is set correctly
3. For k3d, images built with Docker are automatically available
4. Or import explicitly: `k3d image import <image> -c <cluster-name>`

### CRDs not found

If you see errors about CRDs:
1. Ensure CRDs are generated: `cd operator && make manifests`
2. Copy CRDs to `manifests/crd/`
3. Apply CRDs before deploying the operator

### RBAC errors

If you see permission errors:
1. Check service account exists: `kubectl get sa -n glooscap-system`
2. Check role binding: `kubectl get clusterrolebinding glooscap-operator-rolebinding`
3. Verify role permissions: `kubectl describe clusterrole glooscap-operator-role`

