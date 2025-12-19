# Multi-Architecture Build Support

## Overview

The CI/CD pipeline now supports building multi-architecture container images using Docker Buildx with QEMU emulation. This allows building for both `linux/amd64` and `linux/arm64` on a single x86 runner.

## How It Works

### Buildx with QEMU Emulation

1. **Buildx Setup**: The workflow sets up Docker Buildx with the `moby/buildkit:latest` image
2. **QEMU Emulation**: Buildx automatically uses QEMU to emulate ARM64 builds on x86 hardware
3. **Multi-Platform Builds**: All images are built for both `linux/amd64` and `linux/arm64`

### Supported Architectures

- **linux/amd64**: Native build on x86 runner
- **linux/arm64**: Emulated build using QEMU

## Configuration

### Workflow Configuration

The `docker/setup-buildx-action@v3` is configured with:

```yaml
- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3
  with:
    driver-opts: |
      image=moby/buildkit:latest
```

### Build Configuration

All build steps specify multi-platform support:

```yaml
- name: Build and push
  uses: docker/build-push-action@v5
  with:
    platforms: linux/amd64,linux/arm64
```

## Performance Considerations

### Build Time

- **AMD64 builds**: Fast (native)
- **ARM64 builds**: Slower (emulated via QEMU)
- **Total time**: ~2x single-arch build time

### Caching

- Buildx uses GitHub Actions cache (`type=gha`)
- Caches are architecture-specific
- Subsequent builds are faster due to layer caching

## Testing Multi-Arch Builds

### Manual Test

1. Trigger workflow manually:
   ```bash
   gh workflow run ci.yml
   ```

2. Check build logs for multi-arch output:
   ```
   #1 [linux/amd64 builder 1/5] FROM docker.io/library/golang:1.24
   #2 [linux/arm64 builder 1/5] FROM docker.io/library/golang:1.24
   ```

3. Verify images in GHCR:
   ```bash
   docker manifest inspect ghcr.io/dasmlab/glooscap-operator:main
   ```

### Expected Output

The manifest should show both architectures:

```json
{
  "manifests": [
    {
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "platform": {
        "architecture": "arm64",
        "os": "linux"
      }
    }
  ]
}
```

## Troubleshooting

### Build Fails on ARM64

- **QEMU not available**: Ensure buildx is properly set up
- **Out of memory**: ARM64 emulation requires more RAM
- **Timeout**: ARM64 builds take longer, may need timeout adjustment

### Performance Issues

- **Slow builds**: Normal for emulated ARM64
- **Cache misses**: First build is slower, subsequent builds use cache
- **Resource limits**: Ensure runner has sufficient CPU/RAM

### Verification

Check if multi-arch images were created:

```bash
# List architectures
docker buildx imagetools inspect ghcr.io/dasmlab/glooscap-operator:main

# Pull specific architecture
docker pull --platform linux/arm64 ghcr.io/dasmlab/glooscap-operator:main
```

## Future Enhancements

- [ ] Native ARM64 runner for faster ARM64 builds
- [ ] Additional architectures (s390x, ppc64le)
- [ ] Architecture-specific optimizations
- [ ] Build matrix for parallel architecture builds

