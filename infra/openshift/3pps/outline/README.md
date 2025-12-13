# Outline v0.83 Deployment on OpenShift

This directory contains the Kubernetes manifests for deploying Outline wiki v0.83 on OpenShift.

## Prerequisites

- OpenShift cluster with appropriate permissions
- Storage class available (default: `lvms-vg1`)
- DNS configured for `wiki.infra.dasmlab.org` (or update Route hostname)

## Quick Start

1. **Setup secrets:**
   ```bash
   cd infra/openshift/outline
   ./setup-secrets.sh
   ```

2. **Deploy all components:**
   ```bash
   oc apply -k .
   ```

3. **Wait for pods to be ready:**
   ```bash
   oc get pods -n outline -w
   ```

4. **Access Outline:**
   - URL: `https://wiki.infra.dasmlab.org` (or your configured Route hostname)
   - First-time setup: Follow the web UI to create an admin account

## Components

- **Namespace**: `outline`
- **PostgreSQL**: Database for Outline (persistent storage)
- **Redis**: Caching and session storage
- **Outline**: Main application (2 replicas)
- **Route**: External access via OpenShift Route

## Configuration

### Storage

Default storage class is `lvms-vg1`. If your cluster uses a different storage class, update:
- `postgresql.yaml` - `storageClassName`
- `redis.yaml` - `storageClassName`
- `storage.yaml` - `storageClassName`

### Database URL

The database URL is automatically constructed from the PostgreSQL service. It's stored in the `postgresql-credentials` secret as `database-url`.

### Application URL

Update the `URL` environment variable in `outline-deployment.yaml` to match your Route hostname.

### Email Configuration (Optional)

If you need email notifications, uncomment and configure the SMTP settings in `outline-deployment.yaml`.

## Troubleshooting

### Check pod logs:
```bash
oc logs -n outline deployment/outline
oc logs -n outline deployment/postgresql
oc logs -n outline deployment/redis
```

### Check database connection:
```bash
oc exec -n outline deployment/postgresql -- psql -U outline -d outline -c "SELECT version();"
```

### Check Route:
```bash
oc get route -n outline
```

### Scale Outline:
```bash
oc scale deployment/outline -n outline --replicas=3
```

## First-Time Setup

After deployment, access the Outline web UI and:
1. Create an admin account
2. Configure your workspace
3. Create your first document

## API Token for Glooscap

To use Outline with Glooscap, you'll need to create an API token:
1. Log into Outline as admin
2. Go to Settings → API
3. Create a new API token
4. Use this token in your `WikiTarget` CRD

## Notes

- Outline v0.83.0 is used (as specified)
- PostgreSQL 15 Alpine for lightweight deployment
- Redis 7 Alpine for caching
- Health checks configured for all services
- Init containers ensure dependencies are ready before starting Outline

