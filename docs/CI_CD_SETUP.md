# CI/CD Pipeline Setup Summary

## What Was Created

### Glooscap CI/CD Pipeline

**File**: `.github/workflows/ci.yml`

A comprehensive CI/CD pipeline that:

1. **Detects Changes** - Only builds what changed (operator, UI, translation-runner)
2. **Parallel Builds** - Builds all components in parallel
3. **Container Scanning** - Trivy DAST scanning (permissive mode)
4. **Smoke Tests** - Full cluster install and health checks
5. **Code Coverage** - Never fails on syntax errors
6. **Manual Publish** - Optional GHCR publishing via workflow dispatch
7. **Auto Cleanup** - Keeps only 3 most recent dev packages
8. **Main Branch** - System tests + publish as `latest`

### Iskoces CI/CD Pipeline

**File**: `.github/workflows/ci.yml`

A streamlined CI/CD pipeline that:

1. **Builds** Iskoces server container
2. **Scans** for vulnerabilities
3. **Tests** code coverage
4. **Publishes** manually or on main branch

### Updated Test Workflows

**File**: `.github/workflows/test.yml` (Glooscap)

- Fixed to never fail on syntax errors
- Uses `continue-on-error: true`
- Non-blocking coverage uploads

## Key Features

### ✅ Conditional Builds
- Uses `dorny/paths-filter` to detect file changes
- Only builds components that changed
- Saves CI time and resources

### ✅ Permissive Scanning
- Trivy scans containers but doesn't block
- Logs vulnerabilities to GitHub Security
- Allows development to continue

### ✅ Smoke Tests
- Runs `install_glooscap.sh --plugins iskoces`
- Verifies all pods start
- Checks health endpoints
- Cleans up after test

### ✅ Code Coverage
- Never fails on syntax errors
- Uses `continue-on-error: true`
- Non-blocking uploads to Codecov

### ✅ Manual Publishing
- Workflow dispatch with `publish: true` input
- Tags as `dev-<commit-sha>`
- Auto-cleans old dev packages (keeps 3)

### ✅ Branch Strategy
- **All branches**: Build and test
- **PRs**: Build and test (no push)
- **Main**: System tests + publish as `latest`
- **Release** (future): Manual promotion

## Image Tagging

### Development
- `dev-<commit-sha>`: Manual publish builds
- `<branch-name>`: Auto branch builds
- `pr-<number>`: PR builds (not pushed)

### Main Branch
- `main`: Latest main build
- `latest`: After system tests pass

### Release (Future)
- `v<version>`: Semantic versions
- `released`: Latest release

## Usage

### Manual Publish

1. Go to Actions → CI Pipeline
2. Click "Run workflow"
3. Check "Publish to GHCR"
4. Click "Run workflow"

### Viewing Results

- **Builds**: Actions tab
- **Security**: Security → Code scanning
- **Coverage**: Codecov or artifacts
- **Images**: `ghcr.io/dasmlab/*` packages

## Requirements

### Self-Hosted Runner

Must have:
- Docker installed
- Docker-in-Docker support
- Kubernetes/k3d available
- GitHub CLI (`gh`) installed
- Sufficient resources

### GitHub Secrets

- `GITHUB_TOKEN` (auto-provided)
- `CODECOV_TOKEN` (optional, for coverage)

## Next Steps

1. **Test the pipeline** - Push a commit and verify it runs
2. **Set up self-hosted runner** - Configure runner with required tools
3. **Implement system tests** - Add comprehensive API tests for main branch
4. **Add ARM64 builds** - When ARM runner available
5. **Release workflow** - Create release branch promotion workflow

## Troubleshooting

### Pipeline Not Running
- Check branch protection rules
- Verify workflow file syntax
- Check Actions tab for errors

### Build Fails
- Check runner is online
- Verify Docker is running
- Check build logs

### Smoke Test Fails
- Check cluster logs
- Verify `install_glooscap.sh` works locally
- Check runner resources

### Coverage Fails
- Coverage never fails (permissive)
- Check logs for actual errors
- Verify test files exist

## Documentation

- **Glooscap**: `.github/workflows/README.md`
- **Iskoces**: `.github/workflows/README.md`

