# CI/CD Pipeline Documentation

## Overview

This repository uses GitHub Actions for continuous integration and deployment. The pipeline is designed to:

1. **Build only what changed** - Conditional builds based on file changes
2. **Scan containers** - DAST scanning before promotion (permissive mode)
3. **Smoke test** - Full cluster install and health checks
4. **Code coverage** - Never fails on syntax errors (permissive)
5. **Manual publish** - Optional GHCR publishing via workflow dispatch
6. **Auto cleanup** - Keeps only 3 most recent dev packages

## Pipeline Flow

### 1. Commit Triggers
- **All branches**: Build and test
- **Pull requests**: Build and test (no push to GHCR)
- **Main branch**: Additional system tests + publish as `latest`

### 2. Change Detection
The pipeline uses `dorny/paths-filter` to detect what changed:
- `operator/**` → Build operator
- `ui/**` → Build UI
- `translation-runner/**` → Build translation-runner
- `infra/**` → Rebuild all (infra changes affect everything)

### 3. Build Jobs (Parallel)
- **build-operator**: Builds operator container
- **build-ui**: Builds UI container
- **build-translation-runner**: Builds translation-runner container

All builds:
- Run on `self-hosted` runner
- Use Docker Buildx for multi-arch (currently amd64, arm64 when available)
- Tag with: `branch-name`, `dev-<commit-sha>`, `pr-<number>` (for PRs)
- Push to GHCR only if not a PR

### 4. Container Scanning
- Uses Trivy for vulnerability scanning
- **Permissive mode**: Logs failures but doesn't block
- Uploads results to GitHub Security tab
- Scans all built containers

### 5. Smoke Test
- Runs on `self-hosted` runner (Docker-in-Docker)
- Executes `install_glooscap.sh --plugins iskoces`
- Verifies:
  - All pods start successfully
  - Operator is running
  - Iskoces is running
  - Health checks pass
- Cleans up cluster after test

### 6. Code Coverage
- Runs on `ubuntu-latest` (GitHub-hosted)
- **Never fails on syntax errors** - uses `continue-on-error: true`
- Generates coverage report
- Uploads to Codecov (non-blocking)

### 7. Manual Publish (Optional)
- Triggered via `workflow_dispatch` with `publish: true` input
- Tags images as `dev-<commit-sha>`
- Publishes to GHCR
- **Auto-cleanup**: Deletes old dev packages (keeps 3 most recent)

### 8. Main Branch Special Handling
- **System tests**: Comprehensive API tests (TODO: implement)
- **Publish as `latest`**: Tags main branch builds as `latest`

## Usage

### Manual Trigger with Publish

1. Go to Actions tab
2. Select "CI Pipeline"
3. Click "Run workflow"
4. Check "Publish to GHCR"
5. Click "Run workflow"

### Viewing Results

- **Build status**: Check Actions tab
- **Security scans**: Check Security tab → Code scanning alerts
- **Coverage**: Check Codecov or Actions artifacts
- **Container images**: Check `ghcr.io/dasmlab/*` packages

## Image Tagging Strategy

### Development Builds
- `dev-<commit-sha>`: Specific commit builds (manual publish)
- `<branch-name>`: Branch builds (auto on push)
- `pr-<number>`: PR builds (not pushed to GHCR)

### Main Branch
- `main`: Latest main branch build
- `latest`: Latest successful main build (after system tests)

### Release Branch (Future)
- `v<version>`: Semantic version tags
- `released`: Latest release (manual promotion)

## Cleanup Policy

- **Dev packages**: Automatically keeps only 3 most recent `dev-*` tags
- **Old packages**: Deleted via GitHub API during manual publish
- **PR packages**: Not pushed to GHCR (no cleanup needed)

## Self-Hosted Runner Requirements

The pipeline requires a self-hosted runner with:
- Docker installed and running
- Docker-in-Docker support (for smoke tests)
- Kubernetes/k3d available (for smoke tests)
- GitHub CLI (`gh`) installed (for package cleanup)
- Sufficient resources (CPU, memory, disk)

## Troubleshooting

### Build Fails
- Check Actions logs
- Verify self-hosted runner is online
- Check Docker is running on runner

### Smoke Test Fails
- Check cluster logs: `kubectl get pods -A`
- Verify `install_glooscap.sh` works locally
- Check runner has sufficient resources

### Coverage Fails
- Coverage never fails the pipeline (permissive mode)
- Check logs for actual errors
- Verify test files exist

### Publish Fails
- Verify `GITHUB_TOKEN` has package write permissions
- Check runner has `gh` CLI installed
- Verify manual trigger was used with `publish: true`

## Future Enhancements

- [ ] ARM64 builds when ARM runner available
- [ ] System test suite implementation
- [ ] Release branch promotion workflow
- [ ] Multi-arch builds (amd64 + arm64)
- [ ] Performance benchmarks
- [ ] Integration with external test wiki

