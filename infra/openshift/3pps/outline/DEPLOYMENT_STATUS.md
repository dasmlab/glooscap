# Outline v0.83 Deployment Status

## Current Status

✅ **Namespace created**: `outline`
✅ **Secrets created**: `postgresql-credentials`, `outline-config`
✅ **PostgreSQL**: Running (postgres:15-alpine)
✅ **Redis**: Running (redis:7-alpine)
✅ **Storage**: PVCs created (lvms-vg1 storage class)
✅ **Route**: Created at `wiki.infra.dasmlab.org`
❌ **Outline**: CrashLoopBackOff - Connection refused to PostgreSQL

## Issue

Outline pods are failing to connect to PostgreSQL with error:
```
connect ECONNREFUSED 172.30.126.14:5432
```

However, connection test from a busybox pod works:
```bash
oc run -n outline test-db-connection --image=busybox:latest --rm -it --restart=Never -- sh -c "nc -zv postgresql.outline.svc 5432"
# Result: Connection successful!
```

## Configuration

- **DATABASE_URL**: `postgres://outline:PASSWORD@postgresql.outline.svc:5432/outline`
- **PGSSLMODE**: `disable`
- **Storage Class**: `lvms-vg1`
- **Init Containers**: Wait for PostgreSQL and Redis

## Next Steps

1. **Check if database needs initialization**: Outline may need to run migrations on first startup
2. **Add connection retry logic**: Outline might be connecting too quickly
3. **Check PostgreSQL logs**: Verify PostgreSQL is actually accepting connections
4. **Consider using a Job for migrations**: Run Outline migrations as a one-time job before starting the main deployment

## Manual Testing

Test database connection:
```bash
oc run -n outline test-db --image=busybox:latest --rm -it --restart=Never -- sh -c "nc -zv postgresql.outline.svc 5432"
```

Check Outline logs:
```bash
oc logs -n outline -l app=outline --tail=50
```

Check PostgreSQL:
```bash
oc get pods -n outline -l app=postgresql
oc logs -n outline -l app=postgresql --tail=20
```

