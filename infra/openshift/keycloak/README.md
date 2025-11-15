# Keycloak Deployment on OpenShift

This directory contains the Kubernetes manifests for deploying Keycloak on OpenShift.

## Prerequisites

- OpenShift cluster with appropriate permissions
- Storage class available (default: `lvms-vg1`)
- DNS configured for `keycloak.infra.dasmlab.org` (or update Route hostname)

## Quick Start

1. **Setup secrets:**
   ```bash
   cd infra/openshift/keycloak
   ./setup-secrets.sh
   ```

2. **Deploy all components:**
   ```bash
   oc apply -k .
   ```

3. **Wait for pods to be ready:**
   ```bash
   oc get pods -n keycloak -w
   ```

4. **Access Keycloak:**
   - URL: `https://keycloak.infra.dasmlab.org` (or your configured Route hostname)
   - Admin Console: `https://keycloak.infra.dasmlab.org/admin`
   - Username: `admin` (password from setup-secrets.sh output)

## Components

- **Namespace**: `keycloak`
- **PostgreSQL**: Database for Keycloak (persistent storage)
- **Keycloak**: Main application (2 replicas)
- **Route**: External access via OpenShift Route

## Configuration

### Storage

Default storage class is `lvms-vg1`. If your cluster uses a different storage class, update:
- `postgresql.yaml` - `storageClassName`
- `storage.yaml` - `storageClassName`

### Application URL

Update the `KC_HOSTNAME` environment variable in `keycloak-deployment.yaml` to match your Route hostname.

### HTTP Configuration

Keycloak is configured for HTTP internally (port 8080), with HAProxy handling SSL termination externally:
- `KC_HTTP_ENABLED=true`
- `KC_PROXY=edge` (for HAProxy edge proxy)
- Route uses `edge` termination

## Troubleshooting

### Check pod logs:
```bash
oc logs -n keycloak deployment/keycloak
oc logs -n keycloak deployment/postgresql
```

### Check database connection:
```bash
oc exec -n keycloak deployment/postgresql -- psql -U keycloak -d keycloak -c "SELECT version();"
```

### Check Route:
```bash
oc get route -n keycloak
```

### Scale Keycloak:
```bash
oc scale deployment/keycloak -n keycloak --replicas=3
```

## First-Time Setup

After deployment, access the Keycloak admin console:
1. Navigate to `https://keycloak.infra.dasmlab.org/admin`
2. Login with admin credentials (from setup-secrets.sh output)
3. Create realms, clients, and users as needed

## Notes

- Using latest Keycloak image from `quay.io/keycloak/keycloak:latest`
- PostgreSQL 15 Alpine for lightweight deployment
- Health checks configured for all services
- Init containers ensure dependencies are ready before starting Keycloak
- Optimized startup mode for faster initialization

