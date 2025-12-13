# Keycloak Realm Configuration for DasmLab

## Overview

This document outlines the steps to create the "dasmlab" realm in Keycloak and integrate it with Outline wiki.

## Prerequisites

- Keycloak is deployed and accessible
- Admin credentials from `setup-secrets.sh` output
- Access to Keycloak Admin Console

## Step 1: Access Keycloak Admin Console

1. Navigate to: `https://keycloak.infra.dasmlab.org/admin`
2. Login with admin credentials:
   - Username: `admin`
   - Password: (from setup-secrets.sh output)

## Step 2: Create "dasmlab" Realm

1. In the Keycloak Admin Console, click on the realm dropdown (top left, shows "master")
2. Click "Create Realm"
3. Enter realm name: `dasmlab`
4. Click "Create"

## Step 3: Configure Realm Settings

### General Settings
- Realm name: `dasmlab`
- Display name: `DasmLab`
- Enabled: `ON`

### Login Settings
- User registration: `ON` (if you want self-registration)
- Email as username: `OFF` (or `ON` if preferred)
- Remember me: `ON`
- Verify email: `ON` (recommended)

### Themes
- Login theme: `keycloak` (or custom if available)
- Account theme: `keycloak`
- Admin console theme: `keycloak`
- Email theme: `keycloak`

## Step 4: Create Client for Outline

1. Navigate to: **Clients** → **Create client**
2. Configure:
   - **Client type**: `OpenID Connect`
   - **Client ID**: `outline`
   - Click **Next**
3. **Capability config**:
   - **Client authentication**: `ON` (confidential client)
   - **Authorization**: `OFF` (unless needed)
   - **Standard flow**: `ON`
   - **Direct access grants**: `ON` (for API access)
   - **Implicit flow**: `OFF`
   - **Service accounts roles**: `ON` (for service account)
   - Click **Next**
4. **Login settings**:
   - **Root URL**: `https://wiki.infra.dasmlab.org`
   - **Home URL**: `https://wiki.infra.dasmlab.org`
   - **Valid redirect URIs**: 
     - `https://wiki.infra.dasmlab.org/*`
     - `https://wiki.infra.dasmlab.org/auth/oidc/callback`
   - **Web origins**: `https://wiki.infra.dasmlab.org`
   - Click **Save**

## Step 5: Get Client Credentials

1. In the Outline client settings, go to **Credentials** tab
2. Copy the **Client secret** (you'll need this for Outline configuration)
3. Note the **Client ID**: `outline`

## Step 6: Create Users (Optional)

1. Navigate to: **Users** → **Create new user**
2. Fill in:
   - **Username**: (desired username)
   - **Email**: (user email)
   - **First name**: (optional)
   - **Last name**: (optional)
   - **Email verified**: `ON`
   - Click **Create**
3. Go to **Credentials** tab for the user
4. Set a temporary password
5. Set **Temporary** to `OFF` (or leave `ON` to force password change on first login)

## Step 7: Configure Outline for Keycloak OIDC

Outline needs to be configured to use Keycloak as the OIDC provider. This requires:

1. **Keycloak OIDC Endpoints**:
   - Authorization URL: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/auth`
   - Token URL: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/token`
   - UserInfo URL: `https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/userinfo`

2. **Outline Environment Variables** (to be added to deployment):
   ```
   OIDC_CLIENT_ID=outline
   OIDC_CLIENT_SECRET=<client-secret-from-step-5>
   OIDC_AUTH_URI=https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/auth
   OIDC_TOKEN_URI=https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/token
   OIDC_USERINFO_URI=https://keycloak.infra.dasmlab.org/realms/dasmlab/protocol/openid-connect/userinfo
   OIDC_SCOPES=openid profile email
   OIDC_DISPLAY_NAME=Keycloak
   ```

## Step 8: Update Outline Deployment

The Outline deployment will need to be updated with the OIDC environment variables. This will be done via a configuration update.

## Notes

- Keycloak realm "dasmlab" will be the authentication provider for Outline
- Users will login to Outline using their Keycloak credentials
- User management can be done in Keycloak Admin Console
- Groups and roles can be configured in Keycloak and mapped to Outline

## Troubleshooting

- If Outline can't connect to Keycloak, verify the redirect URIs match exactly
- Check Keycloak logs: `oc logs -n keycloak deployment/keycloak`
- Verify OIDC endpoints are accessible from Outline pods
- Check Outline logs for OIDC connection errors

