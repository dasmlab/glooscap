# Publishing GitHub Container Registry Packages

GitHub Container Registry packages need to be "published" before they can be made public. Here's how to do it:

## What Does "Publish" Mean?

When you push a Docker image to GHCR, it creates a package, but the package might not be "published" yet. Publishing makes the package:
- Visible in the packages list
- Available for visibility changes (public/private)
- Accessible to others (if public)

## How to Publish Packages

### Method 1: Via GitHub Web UI

1. **Go to your package page**:
   - https://github.com/orgs/dasmlab/packages/container/glooscap-operator
   - (or any other package)

2. **Look for a "Publish" or "Release" button**:
   - This might be at the top of the package page
   - Or in the package settings
   - Or in a "Versions" or "Releases" section

3. **If you see versions/tags listed**:
   - Each version/tag might need to be published individually
   - Look for a "Publish" button next to each version

### Method 2: Publish via Package Settings

1. Go to package settings:
   - https://github.com/orgs/dasmlab/packages/container/glooscap-operator/settings

2. Look for:
   - "Publish package" option
   - "Make available" option
   - "Release" option

### Method 3: Publish via GitHub CLI

Try publishing via API:

```bash
# List packages to see their status
gh api "orgs/dasmlab/packages" --jq '.[] | select(.package_type=="container") | {name: .name, visibility: .visibility}'

# Publish a specific package version (if API supports it)
# Note: This might not be available via API
```

### Method 4: Re-push with Explicit Publishing

Sometimes re-pushing with a specific tag helps:

```bash
# Make sure you're logged in
echo "${DASMLAB_GHCR_PAT}" | docker login ghcr.io -u lmcdasm --password-stdin

# Re-tag and push (this might trigger publishing)
docker tag ghcr.io/dasmlab/glooscap-operator:released ghcr.io/dasmlab/glooscap-operator:released
docker push ghcr.io/dasmlab/glooscap-operator:released
```

## What to Look For

On the package page, you might see:
- "This package is not published" message
- "Publish package" button
- Versions listed but marked as "unpublished"
- Settings option to "Publish" or "Release"

## After Publishing

Once packages are published:
1. They should appear in the packages list
2. You should be able to change visibility to public
3. The "Package settings" → "Change visibility" option should become available

## Quick Check

To see if packages are published:
1. Go to: https://github.com/orgs/dasmlab/packages
2. Check if packages show up in the list
3. Click on a package - if it says "not published", that's the issue

## Alternative: Use GitHub Releases

If packages are tied to a repository:
1. Create a GitHub Release (this sometimes publishes packages)
2. Tag the release with the same tag as your images
3. This might automatically publish the packages

## Still Having Issues?

If you can't find a "Publish" button:
- The package might already be published but visibility is restricted
- Organization settings might prevent publishing
- You might need organization owner permissions

In that case, keeping packages private with read-only tokens is a perfectly valid approach!

