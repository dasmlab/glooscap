# Keycloak Redirect URI Configuration Check

## Problem

Outline is sending `http://` in the redirect_uri parameter, but Keycloak expects `https://`.

## Root Cause

Outline constructs the redirect_uri based on:
1. The `URL` environment variable
2. The incoming request protocol (if `FORCE_HTTPS` is not properly respected)
3. The OIDC callback path: `/auth/oidc/callback`

## Solution

### Step 1: Verify Keycloak Client Configuration

**CRITICAL**: Even if Outline sends `http://`, you can configure Keycloak to accept BOTH:

1. Go to Keycloak Admin Console: `https://keycloak.infra.dasmlab.org/admin`
2. Realm: `dasmlab`
3. Clients → `outline`
4. **Settings** tab → **Valid redirect URIs**

Add BOTH HTTP and HTTPS (for now, until Outline fully respects HTTPS):

```
http://wiki.infra.dasmlab.org/*
http://wiki.infra.dasmlab.org/auth/oidc/callback
https://wiki.infra.dasmlab.org/*
https://wiki.infra.dasmlab.org/auth/oidc/callback
```

5. **Web origins** should include:
```
http://wiki.infra.dasmlab.org
https://wiki.infra.dasmlab.org
```

6. **Root URL**:
```
https://wiki.infra.dasmlab.org
```

7. Click **Save**

### Step 2: Verify Outline Environment Variables

Check that Outline has the correct environment variables:

```bash
oc get deployment outline -n outline -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="URL")].value}'
oc get deployment outline -n outline -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FORCE_HTTPS")].value}'
```

Should show:
- `URL`: `https://wiki.infra.dasmlab.org`
- `FORCE_HTTPS`: `true`

### Step 3: Check HAProxy Configuration

If HAProxy is forwarding HTTP internally, Outline might construct redirect_uri based on the internal protocol.

Ensure HAProxy sets the `X-Forwarded-Proto: https` header:

```haproxy
http-request set-header X-Forwarded-Proto https if { ssl_fc }
```

And Outline has `TRUST_PROXY: true` (which we've already set).

### Step 4: Restart and Test

1. Restart Outline:
   ```bash
   oc rollout restart deployment/outline -n outline
   ```

2. Wait for pods to be ready:
   ```bash
   oc get pods -n outline -w
   ```

3. Clear browser cache/cookies

4. Test: `https://wiki.infra.dasmlab.org`

### Step 5: Verify Redirect URI in Browser

1. Open browser DevTools → Network tab
2. Click "Sign in with Keycloak"
3. Look for the `auth?response_type=code&redirect_uri=...` request
4. Check if `redirect_uri` now uses `https://` (URL decoded)

## Temporary Workaround

If Outline continues to send `http://`, configure Keycloak to accept both HTTP and HTTPS redirect URIs (as shown in Step 1). This allows authentication to work while we troubleshoot why Outline isn't using HTTPS.

## Long-term Fix

Once Outline is correctly sending `https://`, remove the HTTP redirect URIs from Keycloak for security.

