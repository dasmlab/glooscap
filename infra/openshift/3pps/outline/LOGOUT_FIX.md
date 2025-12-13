# Outline Logout Fix

## Problem

When clicking logout in Outline, users are redirected back to the same page and remain logged in. The session is not properly terminated.

## Root Cause

The logout flow requires:
1. Outline to redirect to Keycloak's logout endpoint
2. Keycloak to clear the session
3. Keycloak to redirect back to Outline with a cleared session
4. Outline to recognize the cleared session and show the login page

The `OIDC_LOGOUT_URI` needs to include the `post_logout_redirect_uri` parameter so Keycloak knows where to redirect after logout.

## Solution

### Step 1: Update Outline Deployment

The `OIDC_LOGOUT_URI` environment variable now includes the `post_logout_redirect_uri` parameter:

```yaml
- name: OIDC_LOGOUT_URI
  value: "https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/logout?post_logout_redirect_uri=https://wiki.infra.dasmlab.org"
```

### Step 2: Verify Keycloak Client Configuration

In Keycloak Admin Console:

1. Go to: `https://keycloak.infra.dasmlab.org/admin`
2. Realm: `dasmlab`
3. Clients → `outline`
4. **Settings** tab → Scroll to **Valid post logout redirect URIs**
5. Ensure these are configured:
   - `https://wiki.infra.dasmlab.org/*`
   - `https://wiki.infra.dasmlab.org/`
6. Click **Save**

### Step 3: Restart Outline

```bash
oc rollout restart deployment/outline -n outline
```

Wait for pods to be ready:

```bash
oc get pods -n outline -w
```

### Step 4: Test Logout

1. Log in to Outline
2. Click logout
3. Should redirect to Keycloak logout endpoint
4. Keycloak clears the session
5. Redirects back to `https://wiki.infra.dasmlab.org`
6. Outline should show the login page (not auto-login)

## How It Works

1. User clicks logout in Outline
2. Outline redirects to: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/logout?post_logout_redirect_uri=https://wiki.infra.dasmlab.org`
3. Keycloak receives the logout request, clears the session
4. Keycloak redirects to `https://wiki.infra.dasmlab.org` (the post_logout_redirect_uri)
5. Outline receives the request without a valid session, shows login page

## Troubleshooting

### Still auto-logging in after logout

- Check browser cookies - clear all cookies for `wiki.infra.dasmlab.org` and `keycloak.infra.dasmlab.org`
- Verify Keycloak client has the correct post-logout redirect URIs
- Check Outline logs: `oc logs -n outline -l app=outline | grep -i logout`
- Check Keycloak logs: `oc logs -n keycloak -l app=keycloak | grep -i logout`

### Logout redirects to wrong page

- Verify `post_logout_redirect_uri` in `OIDC_LOGOUT_URI` matches a valid post-logout redirect URI in Keycloak
- Ensure the URL uses HTTPS (not HTTP)

