# Cleanup Scripts

## Overview
These scripts help clean up accumulated TranslationJobs, Kubernetes Jobs, and Pods that can accumulate over time.

## Scripts

### 1. `cleanup-all-diagnostic-jobs.sh`
**Purpose**: Aggressively clean up ALL diagnostic jobs and pods immediately.

**Usage**:
```bash
# Preview what will be deleted (dry run)
./scripts/cleanup-all-diagnostic-jobs.sh DRY_RUN=true

# Actually delete everything
./scripts/cleanup-all-diagnostic-jobs.sh DRY_RUN=false
# or simply:
./scripts/cleanup-all-diagnostic-jobs.sh
```

**What it does**:
- Deletes ALL diagnostic TranslationJobs (CRs)
- Deletes ALL diagnostic Kubernetes Jobs
- Deletes ALL diagnostic Pods
- Deletes ALL completed/failed translation jobs (not just diagnostic)

**Use when**: You have thousands of accumulated diagnostic jobs and want to clean them all up immediately.

### 2. `cleanup-old-jobs.sh`
**Purpose**: Clean up old completed/failed jobs based on age.

**Usage**:
```bash
# Clean up jobs older than 24 hours (default)
./scripts/cleanup-old-jobs.sh

# Clean up jobs older than 12 hours
MAX_AGE_HOURS=12 ./scripts/cleanup-old-jobs.sh

# Preview what will be deleted
DRY_RUN=true ./scripts/cleanup-old-jobs.sh
```

**What it does**:
- Finds completed/failed Kubernetes Jobs older than specified age
- Deletes them (and their pods via cascade)

**Use when**: You want to clean up old jobs but keep recent ones.

## Automatic Cleanup (Built-in)

The operator now automatically cleans up TranslationJobs based on their state and type:

- **Diagnostic jobs**: Deleted after **1 hour** in Failed/Completed state
- **Failed jobs**: Deleted after **24 hours** in Failed state
- **Completed jobs**: Deleted after **48 hours** in Completed state

This happens automatically during reconciliation - no manual intervention needed!

## Quick Cleanup Commands

### Clean up everything diagnostic-related (immediate)
```bash
cd /home/dasm/org-dasmlab/glooscap/operator
./scripts/cleanup-all-diagnostic-jobs.sh
```

### Clean up old jobs (age-based)
```bash
cd /home/dasm/org-dasmlab/glooscap/operator
./scripts/cleanup-old-jobs.sh MAX_AGE_HOURS=1  # Delete jobs older than 1 hour
```

### Manual kubectl cleanup (if scripts don't work)
```bash
# Delete all diagnostic TranslationJobs
kubectl get translationjobs -n glooscap-system -l glooscap.dasmlab.org/diagnostic=true -o name | xargs kubectl delete -n glooscap-system

# Delete all diagnostic jobs
kubectl get jobs -n glooscap-system -l glooscap.dasmlab.org/diagnostic=true -o name | xargs kubectl delete -n glooscap-system

# Delete all completed/failed jobs
kubectl get jobs -n glooscap-system --field-selector status.successful=1 -o name | xargs kubectl delete -n glooscap-system
kubectl get jobs -n glooscap-system --field-selector status.failed=1 -o name | xargs kubectl delete -n glooscap-system
```

## After Cleanup

After running cleanup scripts, verify:
```bash
# Check remaining jobs
kubectl get jobs -n glooscap-system | wc -l

# Check remaining pods
kubectl get pods -n glooscap-system | grep -E "translation|diagnostic" | wc -l

# Check remaining TranslationJobs
kubectl get translationjobs -n glooscap-system | wc -l
```

## Notes

- Cleanup scripts use `--ignore-not-found=true` to avoid errors if resources are already deleted
- Cleanup scripts use `--cascade=orphan` for Jobs to avoid waiting for pod deletion
- Automatic cleanup in the operator will prevent future accumulation
- After deploying the new operator code, old jobs will be automatically cleaned up over time

