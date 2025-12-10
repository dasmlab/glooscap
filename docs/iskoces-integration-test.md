# Testing Iskoces Integration with Glooscap

This guide explains how to test Glooscap with Iskoces as the translation service backend.

## Prerequisites

1. Iskoces running and accessible
2. Glooscap operator codebase
3. Access to the cluster/namespace where Glooscap will run

## Local Testing Setup

### 1. Start Iskoces Locally

```bash
cd /home/dasm/org-dasmlab/infra/iskoces
./runme.sh
```

This will:
- Start Iskoces in a Docker container
- Expose gRPC on port 50051
- Expose LibreTranslate on port 5000
- Create a volume for model persistence

### 2. Configure Glooscap to Use Iskoces

Edit `operator/config/manager/manager.yaml` or create an overlay:

```yaml
env:
  # Use Iskoces instead of Nanabush
  - name: TRANSLATION_SERVICE_ADDR
    value: "localhost:50051"  # For local testing
    # Or for Kubernetes: "iskoces-service.iskoces.svc:50051"
  - name: TRANSLATION_SERVICE_TYPE
    value: "iskoces"
  - name: TRANSLATION_SERVICE_SECURE
    value: "false"
  
  # Comment out or remove NANABUSH_GRPC_ADDR when using Iskoces
  # - name: NANABUSH_GRPC_ADDR
  #   value: "209.15.95.244:50051"
```

### 3. Run Glooscap Locally

```bash
cd /home/dasm/org-dasmlab/tools/glooscap/operator
make run
```

### 4. Verify Connection

Check the Glooscap logs for:

```
Translation service gRPC client initialized and registered
  service_type=iskoces
  address=localhost:50051
  client_id=iskoces-client-...
```

### 5. Test Translation

Create a TranslationJob in Glooscap and verify it uses Iskoces for translation.

## Kubernetes/OpenShift Testing

### 1. Deploy Iskoces

Deploy Iskoces to your cluster (create Service, Deployment, etc.)

### 2. Update Glooscap Configuration

Update the Glooscap deployment to use Iskoces:

```bash
kubectl set env deployment/controller-manager -n system \
  TRANSLATION_SERVICE_ADDR=iskoces-service.iskoces.svc:50051 \
  TRANSLATION_SERVICE_TYPE=iskoces \
  TRANSLATION_SERVICE_SECURE=false
```

Or edit the deployment directly:

```bash
kubectl edit deployment controller-manager -n system
```

### 3. Verify

```bash
# Check Glooscap logs
kubectl logs -f deployment/controller-manager -n system | grep -i "translation service"

# Check status endpoint
curl http://localhost:3000/api/v1/status/nanabush
```

## Switching Back to Nanabush

To switch back to Nanabush:

```bash
kubectl set env deployment/controller-manager -n system \
  TRANSLATION_SERVICE_ADDR=nanabush-service.nanabush.svc:50051 \
  TRANSLATION_SERVICE_TYPE=nanabush
```

Or use the backward-compatible variables:

```bash
kubectl set env deployment/controller-manager -n system \
  NANABUSH_GRPC_ADDR=nanabush-service.nanabush.svc:50051
```

## Troubleshooting

### Connection Refused

- Verify Iskoces is running: `docker ps` or `kubectl get pods -n iskoces`
- Check service address is correct
- Verify network policies allow communication

### Service Type Not Detected

- Set `TRANSLATION_SERVICE_TYPE` explicitly
- Check logs for auto-detection messages

### Translation Fails

- Verify Iskoces health: `curl http://localhost:5000/languages`
- Check Iskoces logs: `docker logs iskoces-server-instance`
- Verify language models are loaded

