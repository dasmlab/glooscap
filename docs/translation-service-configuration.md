# Translation Service Configuration

Glooscap supports two translation service backends that can be configured via environment variables:

1. **Nanabush** - Heavy vLLM-based service (GPU required, production workloads)
2. **Iskoces** - Lightweight MT service (CPU-only, LibreTranslate/Argos, development/testing)

Both services use the same gRPC proto interface, so switching between them is seamless - no code changes required.

## Configuration Options

### Option 1: Generic Environment Variables (Recommended)

Use these environment variables to configure either service:

```yaml
env:
  # Service address (required)
  - name: TRANSLATION_SERVICE_ADDR
    value: "iskoces-service.iskoces.svc:50051"  # or "nanabush-service.nanabush.svc:50051"
  
  # Service type (optional, helps with logging)
  - name: TRANSLATION_SERVICE_TYPE
    value: "iskoces"  # or "nanabush"
  
  # TLS/mTLS configuration (optional)
  - name: TRANSLATION_SERVICE_SECURE
    value: "false"  # Set to "true" if using TLS/mTLS
```

### Option 2: Nanabush-Specific Variables (Backward Compatible)

For backward compatibility, Nanabush-specific variables are still supported:

```yaml
env:
  - name: NANABUSH_GRPC_ADDR
    value: "nanabush-service.nanabush.svc:50051"
  - name: NANABUSH_SECURE
    value: "false"
```

**Priority**: If `TRANSLATION_SERVICE_ADDR` is set, it takes precedence over `NANABUSH_GRPC_ADDR`.

## Switching Between Services

### To Use Iskoces

1. Set `TRANSLATION_SERVICE_ADDR` to your Iskoces service address:
   ```yaml
   - name: TRANSLATION_SERVICE_ADDR
     value: "iskoces-service.iskoces.svc:50051"
   ```

2. (Optional) Set the service type for better logging:
   ```yaml
   - name: TRANSLATION_SERVICE_TYPE
     value: "iskoces"
   ```

3. Comment out or remove `NANABUSH_GRPC_ADDR` if it's set.

### To Use Nanabush

1. Either use the generic variables:
   ```yaml
   - name: TRANSLATION_SERVICE_ADDR
     value: "nanabush-service.nanabush.svc:50051"
   - name: TRANSLATION_SERVICE_TYPE
     value: "nanabush"
   ```

2. Or use the backward-compatible variables:
   ```yaml
   - name: NANABUSH_GRPC_ADDR
     value: "nanabush-service.nanabush.svc:50051"
   ```

## Service Addresses

### Kubernetes/OpenShift

- **Iskoces**: `iskoces-service.iskoces.svc:50051`
- **Nanabush**: `nanabush-service.nanabush.svc:50051`

### Local Development

- **Iskoces**: `localhost:50051` (when running `./runme.sh`)
- **Nanabush**: `localhost:50051` (when running locally)

## Verification

After configuring, check the Glooscap logs to verify the connection:

```bash
kubectl logs -f deployment/controller-manager -n system
```

You should see:
```
Translation service gRPC client initialized and registered
  service_type=iskoces  # or nanabush
  address=iskoces-service.iskoces.svc:50051
  client_id=iskoces-client-...
```

## Status Endpoints

Glooscap exposes status information via HTTP API:

- `/api/v1/status/nanabush` - Translation service status (works for both services)
- `/api/v1/status/translation` - Alias for the above

Both endpoints return the same status structure regardless of which service is configured.

## Differences Between Services

| Feature | Nanabush | Iskoces |
|---------|----------|---------|
| **Backend** | vLLM (GPU-based) | LibreTranslate/Argos (CPU-only) |
| **Resource Requirements** | GPU, high memory | CPU-only, lower memory |
| **Startup Time** | Slower (model loading) | Faster |
| **Translation Quality** | Higher (LLM-based) | Good (rule-based/statistical) |
| **Use Case** | Production, complex texts | Development, testing, simple texts |
| **gRPC Interface** | Same proto | Same proto |

## Troubleshooting

### Service Not Connecting

1. Verify the service address is correct:
   ```bash
   # For Kubernetes
   kubectl get svc -n iskoces iskoces-service
   kubectl get svc -n nanabush nanabush-service
   ```

2. Check network policies allow communication between namespaces.

3. Verify the service is running:
   ```bash
   kubectl get pods -n iskoces
   kubectl get pods -n nanabush
   ```

### Service Type Not Detected

If `TRANSLATION_SERVICE_TYPE` is not set, Glooscap will try to auto-detect from the address:
- If address contains "iskoces" → `iskoces`
- Otherwise → `nanabush` (default)

You can always set it explicitly for clarity.

