# Outline Wiki Setup & Installation

This document covers the installation and configuration of Outline wiki instances for use with Glooscap.

## Overview

[Outline](https://www.getoutline.com/) is a modern, open-source wiki platform that Glooscap uses as both source and destination for translation workflows. This guide covers installation options and integration with Glooscap.

## Prerequisites

- Docker/Podman or Kubernetes/OpenShift cluster
- PostgreSQL database (version 12+)
- Redis (for caching and sessions)
- Storage for file uploads (S3-compatible or local filesystem)
- Domain name and SSL certificate (for production)

## Installation Options

### Option 1: Docker Compose (Recommended for Development/Testing)

**Quick Start**:
```bash
# Clone Outline repository
git clone https://github.com/outline/outline.git
cd outline

# Copy environment template
cp .env.sample .env

# Edit .env with your configuration
# Required: DATABASE_URL, REDIS_URL, SECRET_KEY, UTILS_SECRET

# Start services
docker-compose up -d
```

**Required Services**:
- PostgreSQL (via docker-compose or external)
- Redis (via docker-compose or external)
- Outline application container

### Option 2: Kubernetes/OpenShift Deployment

**Components Needed**:
- Outline application (Deployment)
- PostgreSQL database (StatefulSet or external managed service)
- Redis (Deployment or external)
- Storage (PVC for uploads or S3-compatible storage)
- Service and Route/Ingress for external access

**Manifests**: See `infra/openshift/outline/` (to be created)

### Option 3: Self-Hosted with Docker

**Single Container**:
```bash
docker run -d \
  --name outline \
  -p 3000:3000 \
  -e DATABASE_URL=postgres://... \
  -e REDIS_URL=redis://... \
  -e SECRET_KEY=... \
  -e UTILS_SECRET=... \
  -e URL=https://wiki.example.com \
  outlinewiki/outline:latest
```

## Configuration

### Environment Variables

Key environment variables for Outline:

```bash
# Database (Required)
DATABASE_URL=postgres://user:password@postgres-host:5432/outline
DATABASE_URL_SSL=require  # or "disable" for local

# Redis (Required)
REDIS_URL=redis://redis-host:6379

# Application Secrets (Required - generate with: openssl rand -hex 32)
SECRET_KEY=your-secret-key-here
UTILS_SECRET=your-utils-secret-here

# Application URL (Required)
URL=https://wiki.example.com

# Storage Configuration
# Option A: S3-compatible storage
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
AWS_REGION=us-east-1
AWS_S3_UPLOAD_BUCKET_NAME=outline-uploads
AWS_S3_UPLOAD_BUCKET_URL=https://s3.amazonaws.com/outline-uploads
AWS_S3_ACL=private
AWS_S3_FORCE_PATH_STYLE=false

# Option B: Local filesystem storage
FILE_STORAGE=local
FILE_STORAGE_LOCAL_ROOT_DIR=/var/lib/outline/uploads

# Email (Optional, for notifications)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_FROM_EMAIL=noreply@example.com
SMTP_REPLY_EMAIL=noreply@example.com

# Additional Options
FORCE_HTTPS=true
TRUST_PROXY=true
```

### Generating Secrets

```bash
# Generate SECRET_KEY
openssl rand -hex 32

# Generate UTILS_SECRET
openssl rand -hex 32
```

## Database Setup

### PostgreSQL

**Using OpenShift Template**:
```bash
oc new-app postgresql-persistent \
  -p POSTGRESQL_DATABASE=outline \
  -p POSTGRESQL_USER=outline \
  -p POSTGRESQL_PASSWORD=$(openssl rand -base64 24) \
  -p VOLUME_CAPACITY=50Gi \
  -n outline
```

**Manual Setup**:
```sql
CREATE DATABASE outline;
CREATE USER outline WITH PASSWORD 'secure-password';
GRANT ALL PRIVILEGES ON DATABASE outline TO outline;
```

### Redis

**Using OpenShift Template**:
```bash
oc new-app redis-persistent \
  -p REDIS_PASSWORD=$(openssl rand -base64 24) \
  -n outline
```

**Or use external Redis service**

## Storage Setup

### Option 1: S3-Compatible Storage (Recommended for Production)

**Using MinIO on OpenShift**:
```bash
# Install MinIO operator or use existing S3 service
# Configure bucket: outline-uploads
# Set up access keys
```

**Using AWS S3**:
- Create S3 bucket: `outline-uploads`
- Create IAM user with S3 access
- Configure credentials in Outline environment

### Option 2: Local Storage (PVC)

**Create PVC**:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: outline-uploads
  namespace: outline
spec:
  accessModes:
  - ReadWriteMany  # Preferred for multiple replicas
  resources:
    requests:
      storage: 100Gi
  storageClassName: nfs-client  # or your storage class
```

## OpenShift Deployment

### Complete Deployment Manifest

Create `infra/openshift/outline/outline-deployment.yaml`:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: outline
---
apiVersion: v1
kind: Secret
metadata:
  name: outline-config
  namespace: outline
type: Opaque
stringData:
  DATABASE_URL: "postgres://outline:password@postgres.outline.svc:5432/outline"
  REDIS_URL: "redis://redis.outline.svc:6379"
  SECRET_KEY: "generate-with-openssl-rand-hex-32"
  UTILS_SECRET: "generate-with-openssl-rand-hex-32"
  URL: "https://wiki.example.com"
  AWS_ACCESS_KEY_ID: "..."
  AWS_SECRET_ACCESS_KEY: "..."
  AWS_REGION: "us-east-1"
  AWS_S3_UPLOAD_BUCKET_NAME: "outline-uploads"
  AWS_S3_UPLOAD_BUCKET_URL: "https://s3.amazonaws.com/outline-uploads"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: outline
  namespace: outline
spec:
  replicas: 2
  selector:
    matchLabels:
      app: outline
  template:
    metadata:
      labels:
        app: outline
    spec:
      containers:
      - name: outline
        image: outlinewiki/outline:latest
        envFrom:
        - secretRef:
            name: outline-config
        ports:
        - containerPort: 3000
          name: http
        volumeMounts:
        - name: uploads
          mountPath: /var/lib/outline/uploads
          readOnly: false
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /api/health
            port: 3000
          initialDelaySeconds: 60
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /api/health
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: uploads
        persistentVolumeClaim:
          claimName: outline-uploads
---
apiVersion: v1
kind: Service
metadata:
  name: outline
  namespace: outline
spec:
  selector:
    app: outline
  ports:
  - port: 3000
    targetPort: 3000
    protocol: TCP
  type: ClusterIP
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: outline
  namespace: outline
spec:
  to:
    kind: Service
    name: outline
  port:
    targetPort: 3000
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
```

## API Token Creation

To create an API token for Glooscap integration:

1. **Log into Outline** as an administrator
2. Navigate to **Settings** → **API** (or **Integrations**)
3. Click **Create API Token**
4. Configure permissions:
   - **Read**: For read-only WikiTargets
   - **Read & Write**: For read-write WikiTargets  
   - **Write**: For push-only WikiTargets
5. Copy the generated token
6. Store it in a Kubernetes Secret

**Secret Creation**:
```bash
kubectl create secret generic outline-api-token \
  --from-literal=token='your-api-token-here' \
  -n glooscap-system
```

Or via YAML:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: outline-api-token
  namespace: glooscap-system
type: Opaque
stringData:
  token: "outline_api_token_here"
```

## Integration with Glooscap

### Creating WikiTarget

Once Outline is deployed and you have an API token, create a `WikiTarget` resource:

```yaml
apiVersion: wiki.glooscap.dasmlab.org/v1alpha1
kind: WikiTarget
metadata:
  name: outline-infra-dasmlab-org
  namespace: glooscap-system
spec:
  uri: https://wiki.infra.dasmlab.org
  serviceAccountSecretRef:
    name: outline-api-token
    key: token
  mode: ReadWrite
  sync:
    interval: 15s
  translationDefaults:
    targetRef: outline-infra-dasmlab-org
    languageTag: fr-CA
```

See `infra/openshift/wikitarget-infra-dasmlab-org.yaml` for a complete example.

## Verification

### Test Outline API

```bash
# List pages
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://wiki.infra.dasmlab.org/api/documents.list

# Get page info
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://wiki.infra.dasmlab.org/api/documents.info?id=page-id

# Export page content
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id": "page-id-here"}' \
  https://wiki.infra.dasmlab.org/api/documents.export
```

### Test from Glooscap

1. Create `WikiTarget` resource:
   ```bash
   kubectl apply -f infra/openshift/wikitarget-infra-dasmlab-org.yaml
   ```

2. Check controller logs:
   ```bash
   kubectl logs -n glooscap-system -l control-plane=controller-manager --tail=50
   ```

3. Verify pages appear in UI:
   - Open Glooscap UI
   - Check Catalogue page
   - Verify pages are listed

## Troubleshooting

### Common Issues

1. **API Token Invalid**
   - Verify token in Outline Settings → API
   - Check token hasn't expired
   - Ensure token has correct permissions
   - Verify token format (should start with `ol_api_`)

2. **Connection Refused**
   - Verify Outline service is running: `kubectl get pods -n outline`
   - Check service endpoint: `kubectl get svc -n outline`
   - Verify Route/Ingress is configured
   - Check network policies aren't blocking

3. **Database Connection Failed**
   - Verify PostgreSQL is running
   - Check DATABASE_URL format
   - Verify network connectivity between pods
   - Check database credentials

4. **Storage Issues**
   - Verify PVC is bound: `kubectl get pvc -n outline`
   - Check storage class exists
   - Verify pod has mount permissions
   - Check disk space

5. **Authentication Errors**
   - Verify SECRET_KEY and UTILS_SECRET are set
   - Check they're different values
   - Ensure they're not empty

## Security Considerations

- **API Tokens**: Use least-privilege tokens (read-only when possible)
- **Network Policies**: Restrict access to Outline API from Glooscap namespace only
- **TLS**: Always use HTTPS in production (configured via Route)
- **Secrets**: Store tokens in Kubernetes Secrets, never in code or config files
- **RBAC**: Limit who can create/modify WikiTargets
- **Database**: Use strong passwords, enable SSL connections
- **Storage**: Use encrypted storage classes for sensitive data

## Production Checklist

- [ ] PostgreSQL database with backups configured
- [ ] Redis with persistence enabled
- [ ] S3-compatible storage or encrypted PVC
- [ ] SSL/TLS certificate configured
- [ ] API tokens created with minimal permissions
- [ ] Network policies configured
- [ ] Monitoring and alerting set up
- [ ] Backup strategy for database and uploads
- [ ] Resource limits configured
- [ ] Health checks configured

## References

- [Outline GitHub Repository](https://github.com/outline/outline)
- [Outline Documentation](https://www.getoutline.com/developers)
- [Outline API Reference](https://www.getoutline.com/developers)
- [Docker Hub - Outline](https://hub.docker.com/r/outlinewiki/outline)

## Next Steps

1. Deploy Outline instance (Docker, Kubernetes, or OpenShift)
2. Configure database and storage
3. Create initial admin user
4. Generate API token for Glooscap
5. Store token in Kubernetes Secret
6. Create WikiTarget resource
7. Verify discovery in Glooscap UI
