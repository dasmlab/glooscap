#!/usr/bin/env bash
# Setup script for Keycloak secrets
set -euo pipefail

NAMESPACE="keycloak"

echo "Setting up Keycloak secrets..."

# Check if namespace exists
if ! oc get namespace "${NAMESPACE}" >/dev/null 2>&1; then
  echo "Creating namespace ${NAMESPACE}..."
  oc create namespace "${NAMESPACE}"
fi

# Generate secrets
POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)

# Create PostgreSQL credentials secret
oc create secret generic postgresql-credentials \
  --namespace="${NAMESPACE}" \
  --from-literal=password="${POSTGRES_PASSWORD}" \
  --dry-run=client -o yaml | oc apply -f -

# Create Keycloak admin secret
oc create secret generic keycloak-admin \
  --namespace="${NAMESPACE}" \
  --from-literal=username="admin" \
  --from-literal=password="${KEYCLOAK_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | oc apply -f -

echo "Secrets created successfully!"
echo ""
echo "PostgreSQL password: ${POSTGRES_PASSWORD}"
echo "Keycloak admin password: ${KEYCLOAK_ADMIN_PASSWORD}"
echo ""
echo "Note: Save these values securely. They are stored in Kubernetes secrets."
echo ""
echo "Keycloak admin credentials:"
echo "  Username: admin"
echo "  Password: ${KEYCLOAK_ADMIN_PASSWORD}"

