# Fixing "Invalid parameter: redirect_uri" Error

## Problem

When accessing Outline, you see: "Invalid parameter: redirect_uri" from Keycloak.

This happens when the redirect URI in the OIDC request doesn't match what's configured in the Keycloak client.

## Solution

### Step 1: Verify Keycloak Client Configuration

1. Go to Keycloak Admin Console: `https://keycloak.infra.dasmlab.org/admin`
2. Select realm: `dasmlab`
3. Go to: **Clients** → `outline`
4. Check **Valid redirect URIs** - they must match EXACTLY:
   - `https://wiki.infra.dasmlab.org/*`
   - `https://wiki.infra.dasmlab.org/auth/oidc/callback`

### Step 2: Common Issues

**Issue 1: HTTP vs HTTPS**
- Outline might be sending `http://` instead of `https://`
- **Fix**: Ensure `URL` environment variable uses `https://`
- **Fix**: Set `FORCE_HTTPS=true` in Outline deployment

**Issue 2: Trailing Slash**
- Redirect URI might be `https://wiki.infra.dasmlab.org/auth/oidc/callback/` (with trailing slash)
- **Fix**: In Keycloak, add both with and without trailing slash:
  - `https://wiki.infra.dasmlab.org/auth/oidc/callback`
  - `https://wiki.infra.dasmlab.org/auth/oidc/callback/`

**Issue 3: Wildcard Not Matching**
- Keycloak wildcard `*` might not match the exact callback path
- **Fix**: Add the exact callback URI explicitly:
  - `https://wiki.infra.dasmlab.org/auth/oidc/callback`

### Step 3: Check What Outline is Sending

Check Outline logs to see the exact redirect URI:

```bash
oc logs -n outline -l app=outline | grep -i redirect
```

Or check the browser's network tab to see the exact `redirect_uri` parameter in the OIDC request.

### Step 4: Update Keycloak Client

In Keycloak client settings, ensure:

1. **Valid redirect URIs** includes:
   ```
   https://wiki.infra.dasmlab.org/*
   https://wiki.infra.dasmlab.org/auth/oidc/callback
   ```

2. **Web origins** includes:
   ```
   https://wiki.infra.dasmlab.org
   ```

3. **Root URL** is set to:
   ```
   https://wiki.infra.dasmlab.org
   ```

### Step 5: Restart Outline

After updating Keycloak client settings, restart Outline:

```bash
oc rollout restart deployment/outline -n outline
```

## Verification

1. Clear browser cache/cookies for `wiki.infra.dasmlab.org`
2. Access: `https://wiki.infra.dasmlab.org`
3. Click "Sign in with Keycloak"
4. Should redirect to Keycloak login (not show redirect_uri error)

## Debugging

If still having issues, check:

1. **Keycloak logs**:
   ```bash
   oc logs -n keycloak -l app=keycloak | grep -i redirect
   ```

2. **Outline logs**:
   ```bash
   oc logs -n outline -l app=outline | grep -i oidc
   ```

3. **Browser network tab**: Check the exact `redirect_uri` parameter in the OIDC auth request

4. **Keycloak client settings**: Verify all URIs match exactly (case-sensitive, no extra spaces)

