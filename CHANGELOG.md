# Changelog

All notable changes to Glooscap will be documented in this file.

## [0.4.0] - 2025-01-XX

### Summary

This release marks a significant milestone with working page translation functionality. The system now successfully translates pages from source to target wikis, with improved diagnostic capabilities and release management.

### Major Features

#### 🚀 Translation Pipeline
- **Working Page Translation**: End-to-end translation pipeline is now functional
- **Translation Jobs**: Successfully processing translation jobs from UI to target wiki
- **Background Translation**: Diagnostic jobs running regularly to test translation service

#### 🔧 Diagnostic Improvements
- **Single Diagnostic Page**: Fixed diagnostic controller to maintain a single diagnostic page per target
  - Deletes duplicate diagnostic pages automatically
  - Tracks and updates the most recent draft page
  - Prefers draft pages over published pages
- **Improved Error Handling**: Better detection and handling of ImagePullBackOff errors
- **Stuck Job Detection**: Diagnostic controller now detects and cleans up stuck jobs

#### 📦 Release Management
- **Release Images Script**: Added `release_images.sh` for building and pushing release images
- **User Installation**: Separated developer (`dev_install_glooscap.sh`) and user (`install_glooscap.sh`) installation scripts
- **Released Tag Support**: Images can now be tagged with `:released` for stable releases
- **Plugin Support**: User installation script supports `--plugins` flag with released images

#### 🏗️ Infrastructure Improvements
- **Image Pull Policy**: Changed to `PullIfNotPresent` for better offline/VPN operation
- **Pre-pull Script**: Added script to pre-pull images before isolated operation
- **TLS Certificate Handling**: Consistent `InsecureSkipTLSVerify` behavior across operator and runner
- **RBAC Permissions**: Added pod listing permissions for better error detection

### Bug Fixes

- Fixed diagnostic page creating multiple pages instead of updating a single one
- Fixed ImagePullBackOff errors when connected to VPN (now uses cached images)
- Fixed TLS certificate verification in translation-runner
- Fixed missing RBAC permissions for pod listing
- Fixed diagnostic controller getting stuck on jobs with empty status

### Known Issues

1. **Diagnostic Page Duplicates**: Still investigating edge cases where multiple diagnostic pages may be created (should be resolved in this release)
2. **Formatting and Markdown**: Some formatting and markdown issues need to be addressed in future releases
3. **Page Pulling**: Still catching hooks and pulling all pages in some scenarios - needs investigation

### Migration Notes

- For end-users: Use `install_glooscap.sh` which now uses `:released` images by default
- For developers: Use `dev_install_glooscap.sh` which builds from source
- To create a release: Run `./release_images.sh` to push images with `:released` tag

## [0.3.0] - Previous Release

### Summary

Previous release focused on macOS FOSS infrastructure setup with Podman and k3d support.

