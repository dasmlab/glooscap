# Outline + Keycloak OIDC Integration Guide

This guide walks you through configuring Outline wiki to use Keycloak as the identity provider.

## Prerequisites

- ✅ Keycloak is deployed and accessible at `https://keycloak.infra.dasmlab.org`
- ✅ Outline is deployed and accessible at `https://wiki.infra.dasmlab.org`
- ✅ You have admin access to Keycloak Admin Console

## Quick Setup (Automated)

Run the setup script:

```bash
cd infra/openshift/outline
./setup-keycloak-oidc.sh
```

The script will guide you through:
1. Creating the Keycloak client
2. Getting the client secret
3. Creating the Kubernetes secret
4. Updating the Outline deployment

## Manual Setup

### Step 1: Create "dasmlab" Realm in Keycloak (if not already done)

1. Access Keycloak Admin Console: `https://keycloak.infra.dasmlab.org/admin`
2. Login with admin credentials
3. Click realm dropdown (top left) → "Create Realm"
4. Realm name: `dasmlab`
5. Click "Create"

### Step 2: Create Outline Client in Keycloak

1. In the `dasmlab` realm, go to **Clients** → **Create client**
2. **General Settings**:
   - Client type: `OpenID Connect`
   - Client ID: `outline`
   - Click **Next**

3. **Capability config**:
   - Client authentication: `ON` (confidential client)
   - Authorization: `OFF`
   - Standard flow: `ON`
   - Direct access grants: `ON` (for API access)
   - Implicit flow: `OFF`
   - Service accounts roles: `ON` (optional, for service account)
   - Click **Next**

4. **Login settings**:
   - Root URL: `https://wiki.infra.dasmlab.org`
   - Home URL: `https://wiki.infra.dasmlab.org`
   - Valid redirect URIs:
     - `https://wiki.infra.dasmlab.org/*`
     - `https://wiki.infra.dasmlab.org/auth/oidc/callback`
   - Web origins: `https://wiki.infra.dasmlab.org`
   - **Valid post logout redirect URIs**:
     - `https://wiki.infra.dasmlab.org/*`
     - `https://wiki.infra.dasmlab.org/`
   - Click **Save**

### Step 3: Get Client Secret

1. In the Outline client settings, go to **Credentials** tab
2. Copy the **Client secret** (you'll need this in the next step)

### Step 4: Create Kubernetes Secret

Create a secret with the client secret:

```bash
oc create secret generic outline-oidc \
  --namespace=outline \
  --from-literal=client-secret="YOUR_CLIENT_SECRET_HERE"
```

Replace `YOUR_CLIENT_SECRET_HERE` with the client secret from Step 3.

### Step 5: Update Outline Deployment

The Outline deployment has already been updated with OIDC environment variables. Apply it:

```bash
oc apply -f infra/openshift/outline/outline-deployment.yaml
```

Or if you need to update it manually, the OIDC configuration includes:

```yaml
env:
  - name: OIDC_CLIENT_ID
    value: "outline"
  - name: OIDC_CLIENT_SECRET
    valueFrom:
      secretKeyRef:
        name: outline-oidc
        key: client-secret
  - name: OIDC_AUTH_URI
    value: "https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/auth"
  - name: OIDC_TOKEN_URI
    value: "https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/token"
  - name: OIDC_USERINFO_URI
    value: "https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/userinfo"
  - name: OIDC_SCOPES
    value: "openid profile email"
  - name: OIDC_DISPLAY_NAME
    value: "Keycloak"
  - name: OIDC_LOGOUT_URI
    value: "https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/logout"
```

### Step 6: Restart Outline

The deployment will automatically restart Outline pods. Monitor the restart:

```bash
oc get pods -n outline -w
```

Wait for pods to show `1/1 Ready`.

### Step 7: Test the Integration

1. Access Outline: `https://wiki.infra.dasmlab.org`
2. You should see a "Sign in with Keycloak" button
3. Click it and you'll be redirected to Keycloak login
4. Login with a Keycloak user from the `dasmlab` realm
5. You should be redirected back to Outline, authenticated

## Create Users in Keycloak

To create users that can login to Outline:

1. In Keycloak Admin Console, go to **Users** → **Create new user**
2. Fill in:
   - Username: (desired username)
   - Email: (user email)
   - First name: (optional)
   - Last name: (optional)
   - Email verified: `ON`
   - Click **Create**
3. Go to **Credentials** tab for the user
4. Set a password
5. Set **Temporary** to `OFF` (or `ON` to force password change)

## Troubleshooting

### Outline doesn't show "Sign in with Keycloak" button

- Check Outline logs: `oc logs -n outline -l app=outline`
- Verify OIDC environment variables are set: `oc get deployment outline -n outline -o yaml | grep OIDC`
- Ensure the secret exists: `oc get secret outline-oidc -n outline`

### Redirect URI mismatch error

- Verify the redirect URIs in Keycloak client match exactly:
  - `https://wiki.infra.dasmlab.org/*`
  - `https://wiki.infra.dasmlab.org/auth/oidc/callback`
- Check Outline logs for the exact redirect URI being used

### Can't connect to Keycloak

- Verify Keycloak is accessible: `curl -k https://keycloak.infra.dasmlab.org/realms/dasmlab`
- Check network policies allow Outline pods to reach Keycloak
- Verify the realm name is correct: `dasmlab`

### User not found after login

- Ensure the user exists in the `dasmlab` realm (not `master`)
- Check user email is verified
- Verify the user has a password set

## OIDC Endpoints Reference

For the `dasmlab` realm:

- **Authorization**: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/auth`
- **Token**: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/token`
- **UserInfo**: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/userinfo`
- **Logout**: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/logout`
- **JWKS**: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/certs`

## Notes

- Outline will use Keycloak as the primary authentication method
- Users must exist in the Keycloak `dasmlab` realm
- User management is done in Keycloak Admin Console
- Groups and roles can be configured in Keycloak and may be mapped to Outline permissions

