# Docker Buildx Driver Explanation

## Why `docker-container` instead of `docker`?

### `docker` Driver (Default)
- **What it is**: Uses the BuildKit library built into the Docker daemon
- **Pros**: 
  - Simple, no extra containers
  - Images automatically loaded into local Docker
  - Fast for native platform builds
- **Cons**: 
  - **Only supports the native platform** (amd64 on x86, arm64 on ARM)
  - Cannot do multi-arch builds
  - Limited cache export options

### `docker-container` Driver
- **What it is**: Runs BuildKit inside a dedicated Docker container
- **Pros**:
  - **Supports multi-architecture builds** via QEMU emulation
  - Can specify custom BuildKit versions
  - Better cache import/export
  - More advanced features
- **Cons**:
  - Requires an extra container running
  - Images not automatically loaded (need `--load` flag)
  - Slightly more complex setup

## Why We Need `docker-container` for Multi-Arch

Since we're building for **both `linux/amd64` and `linux/arm64`** on an x86 runner:

1. **Native builds (amd64)**: The `docker` driver could handle this
2. **Cross-platform builds (arm64)**: Requires QEMU emulation, which only works with `docker-container` driver

The `docker-container` driver runs BuildKit in a container that can use QEMU to emulate ARM64 on x86 hardware.

## Network Host Option

The `--driver-opt network=host` allows the BuildKit container to:
- Access the host network directly
- Connect to registries (GHCR) without network issues
- Avoid Docker bridge network complications

This is especially important in DinD (Docker-in-Docker) environments where network routing can be tricky.

## Alternative: Single-Arch Builds

If you only need to build for the native architecture (amd64), you could use:

```bash
docker buildx create --name ci-builder --driver docker --use
```

This would be simpler but wouldn't support ARM64 builds.

## Summary

- **`docker` driver**: Native platform only, simpler
- **`docker-container` driver**: Multi-arch support, required for ARM64 on x86
- **`network=host`**: Helps with registry access in DinD environments

For our use case (multi-arch builds), `docker-container` is the right choice.

