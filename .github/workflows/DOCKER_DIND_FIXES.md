# Docker-in-Docker (DinD) Fixes for Self-Hosted Runners

## Common Issues and Solutions

Based on troubleshooting guides, here are the fixes applied to the CI workflows:

### 1. Socket Permissions (Issue B)

**Symptom**: `docker info` fails with "permission denied" or "cannot connect"

**Solution**: Ensure the runner container has access to the Docker socket's GID.

**For your runner container**, you may need to:
- Run as root: `-u 0:0` 
- Or use: `--group-add $(stat -c %g /var/run/docker.sock)`

### 2. Buildx Builder Bootstrap (Issue C)

**Symptom**: `docker buildx inspect --bootstrap` fails with cgroups/overlayfs errors

**Solution**: 
- Mount cgroups in runner: `-v /sys/fs/cgroup:/sys/fs/cgroup:rw`
- Explicitly create builder with `docker-container` driver

**Applied in workflow**:
```yaml
- name: Create and bootstrap buildx builder
  run: |
    docker buildx rm ci-builder || true
    docker buildx create --name ci-builder --driver docker-container --use
    docker buildx inspect --bootstrap
```

### 3. Manual Builder Creation (Issue D)

**Why**: `docker/setup-buildx-action` can be unreliable in DinD environments

**Solution**: We now manually create and bootstrap the builder after the action runs

### 4. Build Context Verification (Issue E)

**Symptom**: "no such file" or "context" errors

**Solution**: Added verification step before builds:
```yaml
- name: Verify build context
  run: |
    pwd
    ls -la
    git rev-parse --show-toplevel
    test -f ./Dockerfile && echo "✅ Dockerfile found"
```

### 5. Explicit Builder Usage

**Why**: Ensure we use the manually created builder

**Solution**: Added `builder: ci-builder` to all `docker/build-push-action` steps

## Runner Container Recommendations

For your `build_and_run.sh`, consider adding:

```bash
# Mount cgroups for buildx
-v /sys/fs/cgroup:/sys/fs/cgroup:rw \

# Ensure proper GID access
--group-add $(stat -c %g /var/run/docker.sock) \
```

Or run as root (simpler but less secure):
```bash
-u 0:0 \
```

## Diagnostic Output

The workflow now includes comprehensive diagnostics that will show:
- User/group IDs
- Docker socket permissions
- Buildx status
- Cgroups mounts
- Builder containers

This will help identify the exact issue if problems persist.

