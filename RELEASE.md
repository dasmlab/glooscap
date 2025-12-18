# Glooscap Release Process

This document outlines the steps for creating a Glooscap release.

## Pre-Release Checklist

### 1. Make Images Public

**Option A: Make images public (recommended for better UX)**

1. Run the script to make all images public:
   ```bash
   ./scripts/make-images-public.sh
   ```

2. Verify images are public:
   - Go to https://github.com/orgs/dasmlab/packages
   - Check that all packages show "Public" visibility

3. **Test public image pull** (without ImagePullSecrets):
   ```bash
   # Test pulling without authentication
   docker pull ghcr.io/dasmlab/glooscap-operator:released
   ```

4. If public images work, update install script to make ImagePullSecrets optional:
   - The install script should check if images are public
   - Only create secrets if images are private or if user provides token

**Option B: Keep images private (requires ImagePullSecrets)**

- Users must provide GitHub token during installation
- Install script will create ImagePullSecrets automatically
- Better for controlled access

**Recommendation**: Start with **Option A** (public) for v0.4.0 to improve user experience. We can always make them private later if needed.

### 2. Release Branch vs Tagging

**Modern Practice**: Tag the `main` branch directly (no release branch needed)

**Steps**:
1. Ensure all code is committed and pushed to `main`
2. Create a release tag:
   ```bash
   git tag -a v0.4.0 -m "Release v0.4.0: Working translation pipeline"
   git push origin v0.4.0
   ```

3. Create GitHub Release from the tag:
   - Go to https://github.com/dasmlab/glooscap/releases
   - Click "Draft a new release"
   - Select tag `v0.4.0`
   - Use the release notes template (see below)

**Alternative (if you prefer release branches)**:
- Create `release/v0.4.0` branch from main
- Tag that branch
- Keep branch for hotfixes if needed

**Recommendation**: Use **tagging main directly** - it's simpler and more common now.

### 3. Release Notes Template

Use this template for GitHub release notes:

```markdown
# Glooscap v0.4.0

## 🎉 What's New

This release marks a significant milestone with **working page translation functionality**. The system now successfully translates pages from source to target wikis with improved diagnostics and release management.

## ✨ Key Features

- ✅ **Working Page Translation**: End-to-end translation pipeline is now functional
- ✅ **Improved Diagnostics**: Single diagnostic page per target (no more duplicates!)
- ✅ **Release Management**: Easy installation with `:released` images
- ✅ **Multi-Architecture Support**: Images available for both ARM64 and AMD64

## 🚀 Quick Start

### Installation

1. **Install Glooscap**:
   ```bash
   git clone https://github.com/dasmlab/glooscap.git
   cd glooscap/infra/macos-foss
   ./install_glooscap.sh
   ```

2. **Install with Translation Service (Iskoces)**:
   ```bash
   ./install_glooscap.sh --plugins iskoces
   ```

3. **Access the UI**:
   - Open http://localhost:8080 (or http://glooscap-ui.testdev.dasmlab.org:8080)

### Next Steps

- 📖 **How-To Guide**: See [HOWTO.md](HOWTO.md) for step-by-step translation workflow
- 📚 **Installation Guide**: See [infra/macos-foss/README.md](infra/macos-foss/README.md)
- 🔧 **Developer Setup**: Use `dev_install_glooscap.sh` for building from source

## 📋 Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete list of changes.

## 🐛 Known Issues

- Some formatting and markdown issues (to be addressed in future releases)
- Page pulling investigation needed (hooks/polling pulling all pages)

## 📦 Images

All images are available at:
- `ghcr.io/dasmlab/glooscap-operator:released`
- `ghcr.io/dasmlab/glooscap-ui:released`
- `ghcr.io/dasmlab/glooscap-translation-runner:released`
- `ghcr.io/dasmlab/iskoces-server:released` (if using Iskoces plugin)

## 🙏 Thanks

Thank you for using Glooscap! For issues or questions, please open an issue on GitHub.
```

## Release Steps

1. **Final Testing**
   - [ ] Test installation with `install_glooscap.sh`
   - [ ] Test installation with `install_glooscap.sh --plugins iskoces`
   - [ ] Verify translation workflow end-to-end
   - [ ] Test with public images (no ImagePullSecrets)

2. **Build and Push Release Images**
   ```bash
   # From Glooscap repo
   ./release_images.sh
   
   # From Iskoces repo (if releasing plugin)
   cd ~/org-dasmlab/iskoces
   ./release_images.sh
   ```

3. **Make Images Public** (if choosing Option A)
   ```bash
   ./scripts/make-images-public.sh
   ```

4. **Create Release Tag**
   ```bash
   git tag -a v0.4.0 -m "Release v0.4.0: Working translation pipeline"
   git push origin v0.4.0
   ```

5. **Create GitHub Release**
   - Go to https://github.com/dasmlab/glooscap/releases/new
   - Select tag `v0.4.0`
   - Paste release notes from template above
   - Mark as "Latest release"
   - Publish

6. **Update Documentation**
   - [ ] Verify HOWTO.md is up to date
   - [ ] Verify README.md points to latest release
   - [ ] Update any version numbers in docs

## Post-Release

- Monitor for issues
- Prepare hotfix if needed
- Start planning next release

