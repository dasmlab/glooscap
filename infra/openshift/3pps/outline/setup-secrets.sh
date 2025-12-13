#!/usr/bin/env bash
# Setup script for Outline secrets
set -euo pipefail

NAMESPACE="outline"

echo "Setting up Outline secrets..."

# Check if namespace exists
if ! oc get namespace "${NAMESPACE}" >/dev/null 2>&1; then
  echo "Creating namespace ${NAMESPACE}..."
  oc create namespace "${NAMESPACE}"
fi

# Generate secrets
SECRET_KEY=$(openssl rand -hex 32)
UTILS_SECRET=$(openssl rand -hex 32)
POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)

# Create PostgreSQL credentials secret with full DATABASE_URL
# Add ?sslmode=disable to the connection string for Outline compatibility
DATABASE_URL="postgres://outline:${POSTGRES_PASSWORD}@postgresql.${NAMESPACE}.svc:5432/outline?sslmode=disable"
oc create secret generic postgresql-credentials \
  --namespace="${NAMESPACE}" \
  --from-literal=password="${POSTGRES_PASSWORD}" \
  --from-literal=database-url="${DATABASE_URL}" \
  --dry-run=client -o yaml | oc apply -f -

# Create Outline config secret
oc create secret generic outline-config \
  --namespace="${NAMESPACE}" \
  --from-literal=SECRET_KEY="${SECRET_KEY}" \
  --from-literal=UTILS_SECRET="${UTILS_SECRET}" \
  --dry-run=client -o yaml | oc apply -f -

echo "Secrets created successfully!"
echo ""
echo "PostgreSQL password: ${POSTGRES_PASSWORD}"
echo "SECRET_KEY: ${SECRET_KEY}"
echo "UTILS_SECRET: ${UTILS_SECRET}"
echo ""
echo "Note: Save these values securely. They are stored in Kubernetes secrets."

