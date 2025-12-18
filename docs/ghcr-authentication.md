# GitHub Container Registry (GHCR) Authentication Guide

This guide explains the different authentication options for pulling Glooscap images from GHCR.

## Option 1: Public Images (Recommended for v0.4.0)

**Best User Experience** - No authentication required!

### Making Images Public

1. Run the script to make all images public:
   ```bash
   ./scripts/make-images-public.sh
   ```

2. Verify images are public:
   - Go to https://github.com/orgs/dasmlab/packages
   - Check that packages show "Public" visibility

### Benefits
- ✅ No tokens needed
- ✅ No ImagePullSecrets required
- ✅ Simplest installation experience
- ✅ Works out of the box

### Drawbacks
- ⚠️ Anyone can pull images (but that's usually fine for open source)
- ⚠️ Can't track who is using the images

---

## Option 2: Private Images with Read-Only Token (Best Practice for Private)

**Secure and Controlled** - Requires authentication but read-only tokens are safe.

### Creating a Read-Only GitHub Token

1. Go to https://github.com/settings/tokens
2. Click "Generate new token" → "Generate new token (classic)"
3. Set expiration (recommend 90 days or 1 year)
4. **Select only these scopes:**
   - ✅ `read:packages` (read packages from GitHub Container Registry)
   - ❌ Do NOT select `write:packages` (not needed for pulling)
5. Generate token and copy it

### Using the Token

**During Installation:**
```bash
export DASMLAB_GHCR_PAT=your_read_only_token
./install_glooscap.sh
```

The install script will automatically:
- Detect that images are private
- Create ImagePullSecrets using your token
- Configure Kubernetes to use the secret

**Manual Secret Creation:**
```bash
export DASMLAB_GHCR_PAT=your_read_only_token
./infra/macos-foss/scripts/create-registry-secret.sh
```

### Benefits
- ✅ Secure - read-only, can't push or modify
- ✅ Can be scoped to specific packages (if needed)
- ✅ Can track usage (if needed)
- ✅ Can revoke easily if compromised

### Drawbacks
- ⚠️ Users need to create and manage tokens
- ⚠️ Tokens expire and need renewal
- ⚠️ Slightly more complex installation

---

## Option 3: Organization-Level Access (For Teams)

If you're part of the `dasmlab` organization, you can:

1. Use your personal read-only token (as above)
2. Or use organization-level package access (if configured)

This is useful for teams where you want centralized access management.

---

## Security Best Practices

### ✅ DO:
- Use read-only tokens (`read:packages` only)
- Set reasonable expiration dates
- Rotate tokens periodically
- Store tokens securely (environment variables, not in code)
- Use different tokens for different purposes

### ❌ DON'T:
- Share tokens in documentation or code
- Use write tokens (`write:packages`) for pulling images
- Commit tokens to git repositories
- Use the same token for multiple services
- Set tokens to never expire (unless absolutely necessary)

---

## Token Scoping

GitHub PATs can be scoped, but **package-level scoping is limited**:

- ✅ You can scope to `read:packages` (all packages) or `write:packages` (all packages)
- ❌ You **cannot** scope to a specific package (e.g., "only glooscap-operator")
- ⚠️ The token will have access to all packages in the organization

**This is a GitHub limitation** - package-level scoping isn't available in PATs.

**Workaround**: If you need package-level access control, consider:
- Making specific packages public
- Using organization-level access controls
- Creating separate GitHub accounts for different packages (not recommended)

---

## Recommendation for Glooscap v0.4.0

**For initial release (v0.4.0):**
- **Make images public** for best user experience
- No authentication needed
- Simplest installation

**For future releases (if needed):**
- Keep images public (recommended for open source)
- Or switch to private with read-only tokens if you need:
  - Usage tracking
  - Access control
  - Compliance requirements

---

## Testing Your Setup

### Test Public Images
```bash
# Logout first to ensure no cached credentials
docker logout ghcr.io

# Try pulling without authentication
docker pull ghcr.io/dasmlab/glooscap-operator:released
```

### Test Private Images with Token
```bash
# Login with your read-only token
echo "your_read_only_token" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin

# Try pulling
docker pull ghcr.io/dasmlab/glooscap-operator:released
```

### Automated Test
```bash
# Test if images are public
./scripts/test-public-images.sh
```

---

## Troubleshooting

### "unauthorized: authentication required"
- Images are private and you need a token
- Create a read-only token with `read:packages` scope
- Set `DASMLAB_GHCR_PAT` environment variable

### "pull access denied"
- Token doesn't have `read:packages` scope
- Token has expired
- Token is for wrong GitHub account/organization

### "ImagePullBackOff" in Kubernetes
- ImagePullSecret not created or incorrect
- Secret in wrong namespace
- Token in secret is invalid or expired

---

## Summary

| Option | Authentication | Best For |
|--------|---------------|----------|
| **Public Images** | None needed | Open source, easy installation |
| **Read-Only Token** | `read:packages` PAT | Private images, controlled access |
| **Write Token** | `write:packages` PAT | Building/pushing images (not for users) |

**For Glooscap users**: Read-only tokens are perfectly fine and actually recommended if images are private. The install script handles this automatically!

