# Changelog - macOS FOSS Setup

## Version 0.3.0 - macOS FOSS Infrastructure Complete

### Summary

This release completes the macOS FOSS (Free and Open Source Software) infrastructure setup for Glooscap, providing a fully functional local development and deployment environment using Podman and k3d.

### Major Features

#### 🚀 User-Friendly Installation
- **Simple Install/Uninstall Scripts**: Added `install_glooscap.sh` and `uninstall_glooscap.sh` for one-command setup and cleanup
- **Clear Documentation**: Separated user-facing documentation from developer documentation
- **Complete Automation**: Installation script handles all steps from dependency setup to deployment

#### 🏗️ Infrastructure Improvements
- **Podman Integration**: Full support for Podman as FOSS container runtime (replaces Docker Desktop)
- **Architecture-Specific Images**: Build and push images with architecture tags (`local-arm64`, `local-amd64`) for parallel development
- **Registry Integration**: Images pushed to `ghcr.io/dasmlab` with proper authentication
- **k3d Cluster Management**: Complete lifecycle management (create, start, stop, remove)

#### 🔧 Developer Experience
- **Complete Cycle Test**: `cycle-test.sh` runs full development cycle (8 steps) with reporting
- **Individual Scripts**: Modular scripts for each step (setup, start, build, deploy, etc.)
- **Error Handling**: Comprehensive error checking and helpful error messages
- **Architecture Detection**: Automatic detection and tagging for ARM64/AMD64

#### 🔐 Security & Permissions
- **Registry Credentials**: Automated creation of Kubernetes secrets for image pulling
- **RBAC Permissions**: Complete RBAC setup including leader election permissions
- **Service Account**: Proper service account configuration for operator

#### 🐛 Bug Fixes
- **Docker API Version**: Fixed API version compatibility (1.44+) for k3d
- **Go Installation**: Added Go to setup script (required for building operator)
- **Missing Imports**: Fixed missing `fmt` import in operator code
- **Leader Election**: Added RBAC permissions for leases resources
- **Podman Resources**: Explicit CPU (4) and memory (4GB) configuration for Podman machine
- **Docker Build**: Updated to use `buildx` with fallback for compatibility

### Technical Details

#### New Scripts
- `install_glooscap.sh` - One-command installation for end users
- `uninstall_glooscap.sh` - One-command cleanup for end users
- `build-and-load-images.sh` - Build and push architecture-specific images
- `create-registry-secret.sh` - Create Kubernetes secrets for registry access
- `cycle-test.sh` - Complete development cycle test (8 steps)

#### Updated Scripts
- `setup-macos-env.sh` - Added Go installation, Docker API version handling, Podman version checks
- `deploy-glooscap.sh` - Added image availability checks
- `start-k3d.sh` - Improved Podman compatibility and error handling

#### New Manifests
- `manifests/rbac/leader_election_role.yaml` - Leader election permissions
- `manifests/rbac/leader_election_role_binding.yaml` - Leader election role binding

#### Updated Manifests
- `manifests/operator/deployment.yaml` - Uses architecture-specific registry images
- `manifests/ui/deployment.yaml` - Uses architecture-specific registry images

### Documentation
- Updated `README.md` with clear user vs developer distinction
- Updated `QUICKSTART.md` with simple installation path
- Added comprehensive troubleshooting sections

### Dependencies
- Docker CLI (via Homebrew)
- Podman 4.0+ (via Homebrew)
- k3d (via Homebrew)
- kubectl (via Homebrew)
- Go 1.24+ (via Homebrew)
- GitHub Personal Access Token with `write:packages` permission

### Breaking Changes
None - this is a new infrastructure setup.

### Migration Notes
For existing users:
- Use `install_glooscap.sh` for fresh installation
- Existing manual setups can continue using individual scripts
- Images now use architecture-specific tags in registry

### Known Issues
None at this time.

### Contributors
- Infrastructure setup and automation
- Podman integration and compatibility
- Architecture-specific image builds
- Complete documentation

---

## Previous Versions

### Version 0.2.x
- Operator and UI version alignment
- Translation service configuration API
- WikiTargets management UI

### Version 0.1.x
- Initial Iskoces integration
- Basic macOS setup scripts

