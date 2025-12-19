# Buildx Multi-Architecture Fix

## Problem Identified

From the diagnostic output, the issue is clear:

**The default buildx builder only supports `linux/amd64` - it does NOT support `linux/arm64`.**

```
docker buildx ls
NAME/NODE: default*
DRIVER/ENDPOINT: docker
PLATFORMS: linux/amd64, linux/amd64/v2, linux/amd64/v3
```

When the workflow tries to build for `platforms: linux/amd64,linux/arm64`, it fails because the builder can't handle ARM64.

## Solution

### 1. Install QEMU for Emulation

QEMU is required to emulate ARM64 on x86 hardware:

```yaml
- name: Install QEMU for multi-arch support
  run: |
    docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
```

### 2. Create Multi-Arch Builder

The default builder uses the `docker` driver which doesn't support multi-arch. We need to create a builder with the `docker-container` driver:

```yaml
- name: Create and bootstrap buildx builder
  run: |
    docker buildx rm ci-builder || true
    docker buildx create --name ci-builder --driver docker-container --use --platform linux/amd64,linux/arm64
    docker buildx inspect --bootstrap ci-builder
```

**Key points:**
- `--driver docker-container`: Required for multi-arch support
- `--platform linux/amd64,linux/arm64`: Explicitly declares supported platforms
- `--bootstrap`: Starts the BuildKit container

### 3. Use the Builder Explicitly

All build steps must specify the builder:

```yaml
- name: Build and push
  uses: docker/build-push-action@v5
  with:
    builder: ci-builder  # Use our multi-arch builder
    platforms: linux/amd64,linux/arm64
```

## Why This Works

1. **docker-container driver**: Creates a BuildKit container that supports QEMU emulation
2. **QEMU installation**: Registers ARM64 emulation with the kernel via binfmt
3. **Explicit platform declaration**: Ensures the builder knows it needs to support both architectures

## Verification

After the builder is created, verify it supports both platforms:

```bash
docker buildx inspect ci-builder | grep Platforms
```

Should show: `linux/amd64, linux/arm64`

## Runner Container Requirements

For this to work, your runner container needs:

1. **Privileged mode**: `--privileged` (for QEMU registration)
2. **Docker socket access**: `/var/run/docker.sock` mounted
3. **Cgroups access**: May need `-v /sys/fs/cgroup:/sys/fs/cgroup:rw` if BuildKit has issues

## Alternative: Build Only for Native Architecture

If multi-arch continues to be problematic, you can build only for the runner's native architecture:

```yaml
platforms: linux/amd64  # Only build for x86_64
```

Then add ARM64 builds later when you have an ARM64 runner.

