#!/bin/bash
#
# cleanup-old-jobs.sh - Clean up old completed/failed Kubernetes Jobs
#
# This script removes completed and failed jobs that are older than a specified age
# to prevent accumulation of job pods in the cluster.
#

set -euo pipefail

NAMESPACE="${NAMESPACE:-glooscap-system}"
MAX_AGE_HOURS="${MAX_AGE_HOURS:-24}"  # Default: delete jobs older than 24 hours
DRY_RUN="${DRY_RUN:-false}"

echo "🧹 Cleaning up old jobs in namespace: ${NAMESPACE}"
echo "   Max age: ${MAX_AGE_HOURS} hours"
echo "   Dry run: ${DRY_RUN}"
echo ""

# Get all completed and failed jobs
COMPLETED_JOBS=$(kubectl get jobs -n "${NAMESPACE}" -o json | \
    jq -r '.items[] | select(.status.completionTime != null) | select(.status.succeeded > 0 or .status.failed > 0) | "\(.metadata.name)|\(.status.completionTime)"')

FAILED_JOBS=$(kubectl get jobs -n "${NAMESPACE}" -o json | \
    jq -r '.items[] | select(.status.failed > 0 and .status.completionTime == null) | "\(.metadata.name)|\(.metadata.creationTimestamp)"')

if [ -z "${COMPLETED_JOBS}" ] && [ -z "${FAILED_JOBS}" ]; then
    echo "✅ No old jobs found to clean up"
    exit 0
fi

CUTOFF_TIME=$(date -u -d "${MAX_AGE_HOURS} hours ago" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || \
    date -u -v-${MAX_AGE_HOURS}H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || \
    echo "error")

if [ "${CUTOFF_TIME}" = "error" ]; then
    echo "❌ Error: Unable to calculate cutoff time. This script requires GNU date or macOS date."
    exit 1
fi

DELETED=0
SKIPPED=0

# Process completed jobs
if [ -n "${COMPLETED_JOBS}" ]; then
    echo "${COMPLETED_JOBS}" | while IFS='|' read -r job_name completion_time; do
        if [ "${completion_time}" \< "${CUTOFF_TIME}" ]; then
            if [ "${DRY_RUN}" = "true" ]; then
                echo "   [DRY RUN] Would delete completed job: ${job_name} (completed: ${completion_time})"
            else
                echo "   🗑️  Deleting completed job: ${job_name} (completed: ${completion_time})"
                kubectl delete job "${job_name}" -n "${NAMESPACE}" --ignore-not-found=true || true
                DELETED=$((DELETED + 1))
            fi
        else
            SKIPPED=$((SKIPPED + 1))
        fi
    done
fi

# Process failed jobs (without completion time)
if [ -n "${FAILED_JOBS}" ]; then
    echo "${FAILED_JOBS}" | while IFS='|' read -r job_name creation_time; do
        if [ "${creation_time}" \< "${CUTOFF_TIME}" ]; then
            if [ "${DRY_RUN}" = "true" ]; then
                echo "   [DRY RUN] Would delete failed job: ${job_name} (created: ${creation_time})"
            else
                echo "   🗑️  Deleting failed job: ${job_name} (created: ${creation_time})"
                kubectl delete job "${job_name}" -n "${NAMESPACE}" --ignore-not-found=true || true
                DELETED=$((DELETED + 1))
            fi
        else
            SKIPPED=$((SKIPPED + 1))
        fi
    done
fi

echo ""
if [ "${DRY_RUN}" = "true" ]; then
    echo "✅ Dry run complete. Use DRY_RUN=false to actually delete jobs."
else
    echo "✅ Cleanup complete. Deleted: ${DELETED} jobs, Skipped: ${SKIPPED} jobs"
fi

