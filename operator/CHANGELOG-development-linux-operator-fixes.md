# Development Branch: linux-operator-fixes

## Overview
This branch contains fixes and improvements for the Linux/OpenShift operator deployment.

## Changes Made

### 1. Git Workflow Setup
- **Added**: `.git-branch-workflow.md` - Comprehensive branch workflow documentation
- **Added**: `scripts/git-branch-helper.sh` - Helper script for branch management
- **Purpose**: Enable safe multi-machine development without conflicts

### 2. Job Cleanup Script
- **Added**: `operator/scripts/cleanup-old-jobs.sh` - Script to clean up old completed/failed jobs
- **Features**:
  - Removes jobs older than specified age (default: 24 hours)
  - Supports dry-run mode
  - Handles both completed and failed jobs
- **Purpose**: Prevent accumulation of job pods in cluster

### 3. Deployment Fixes
- **Fixed**: `operator/cycleme.sh` - Explicitly set `IMG` environment variable
- **Change**: Added `export IMG="ghcr.io/dasmlab/glooscap:latest"` before `make deploy`
- **Purpose**: Ensure consistent image deployment

## Current Status

### Code Verification
✅ **Diagnostics**: Properly disabled in `cmd/main.go` (commented out)
✅ **TTL**: Set to 1 hour (3600 seconds) in `pkg/vllm/dispatcher.go`
✅ **Build Scripts**: `buildme.sh` and `pushme.sh` correctly tag and push images

### Issues to Address
1. **Old Image Running**: Current operator is using old image with diagnostics enabled
2. **Job Accumulation**: 1,347 completed jobs need cleanup
3. **Operator Restarts**: Recent restarts may be due to old image/config

## Next Steps

### 1. Clean Up Old Jobs (Optional)
```bash
cd operator
./scripts/cleanup-old-jobs.sh DRY_RUN=true  # Preview
./scripts/cleanup-old-jobs.sh                # Actually delete
```

### 2. Rebuild and Deploy
```bash
cd operator
./cycleme.sh
```

This will:
- Undeploy old operator
- Build new image with latest code (diagnostics disabled, TTL enabled)
- Push to `ghcr.io/dasmlab/glooscap:latest`
- Deploy new operator
- Deploy UI, WikiTarget, and TranslationService

### 3. Verify Deployment
```bash
# Check operator is running
kubectl get pods -n glooscap-system -l control-plane=controller-manager

# Check operator logs (should NOT see diagnostic job creation)
kubectl logs -n glooscap-system deployment/operator-controller-manager --tail=50

# Verify no new diagnostic jobs are being created
kubectl get jobs -n glooscap-system | grep diagnostic | wc -l
# Wait 5 minutes and check again - count should not increase
```

## Testing Checklist

- [ ] Operator deploys successfully
- [ ] Operator logs show "diagnostic runnable DISABLED"
- [ ] No new diagnostic jobs created after 5+ minutes
- [ ] New translation jobs have TTL set (check with `kubectl get job <name> -o yaml | grep ttlSecondsAfterFinished`)
- [ ] Old jobs are cleaned up after 1 hour
- [ ] Routes are accessible
- [ ] WikiTarget and TranslationService are healthy

## Merge to Main

When ready to merge:
1. Ensure all tests pass
2. Clean up old jobs if needed
3. Create PR or merge directly:
   ```bash
   git checkout main
   git pull origin main
   git merge development/linux-operator-fixes
   git push origin main
   ```

## Branch Information

- **Branch**: `development/linux-operator-fixes`
- **Base**: `main`
- **Last Updated**: $(date)

