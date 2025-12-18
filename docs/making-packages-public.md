# Making GitHub Container Registry Packages Public

If packages are in an organization, making them public can be tricky. Here are the steps:

## Step 1: Check Organization Package Settings

Organization-level settings might be restricting visibility changes:

1. Go to: **https://github.com/orgs/dasmlab/settings/packages**
2. Check the "Package creation" and "Package visibility" settings
3. If visibility changes are restricted, you'll need organization admin to change it

## Step 2: Find the Package Settings

The "Package settings" button location can vary:

### Option A: From Organization Packages Page
1. Go to: **https://github.com/orgs/dasmlab/packages**
2. Click on a package (e.g., `glooscap-operator`)
3. Look for:
   - **"Package settings"** button (usually top right, might be a gear icon)
   - Or scroll down to find settings/visibility options

### Option B: Direct Package URL
1. Go directly to: **https://github.com/orgs/dasmlab/packages/container/glooscap-operator**
2. Look for settings/gear icon
3. If you see "Package settings", click it
4. Scroll to "Danger Zone" section
5. Click "Change visibility"

### Option C: If "Package settings" is Missing

If you don't see "Package settings", try:

1. **Check your permissions**: You need to be an organization owner/admin
2. **Check organization settings**: Go to https://github.com/orgs/dasmlab/settings/packages
   - Look for "Package visibility" restrictions
   - Organization might have a policy preventing public packages

## Step 3: Alternative - Organization Settings

If individual package settings don't work:

1. Go to: **https://github.com/orgs/dasmlab/settings/packages**
2. Look for organization-wide package visibility settings
3. You might need to change the default visibility policy

## Step 4: Using GitHub CLI (If You Have Admin Access)

If you have organization admin permissions, try:

```bash
# First, check current visibility
gh api "orgs/dasmlab/packages/container/glooscap-operator" --jq .visibility

# Try to change visibility (requires admin permissions)
gh api \
  -X PATCH \
  "orgs/dasmlab/packages/container/glooscap-operator" \
  -f visibility=public
```

## Common Issues

### "Package settings" button not visible
- **Cause**: You don't have admin permissions for the organization
- **Solution**: Ask an organization owner to make packages public, or grant you admin access

### Visibility option is grayed out
- **Cause**: Organization has a policy restricting public packages
- **Solution**: Organization owner needs to change the policy at: https://github.com/orgs/dasmlab/settings/packages

### Packages show as "Internal"
- **Cause**: Organization default visibility is "Internal"
- **Solution**: Change individual package visibility, or change organization default

## Quick Test

After making packages public, test:

```bash
# Logout from Docker
docker logout ghcr.io

# Try pulling without authentication
docker pull ghcr.io/dasmlab/glooscap-operator:released
```

If this works, packages are public!

## If All Else Fails

If you can't make packages public due to organization restrictions:

1. **Keep packages private** - Users will need read-only tokens (this is fine!)
2. **Document token creation** - See `docs/ghcr-authentication.md`
3. **Install script handles it** - The install script auto-detects and creates secrets if needed

Private packages with read-only tokens are actually a valid and secure approach!

