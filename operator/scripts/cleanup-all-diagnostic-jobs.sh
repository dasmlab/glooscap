#!/bin/bash
#
# cleanup-all-diagnostic-jobs.sh - Aggressively clean up ALL diagnostic jobs and pods
#
# This script removes ALL diagnostic TranslationJobs and their associated pods/jobs
# regardless of age. Use with caution!
#

set -euo pipefail

NAMESPACE="${NAMESPACE:-glooscap-system}"
DRY_RUN="${DRY_RUN:-false}"

echo "🧹 Aggressively cleaning up ALL diagnostic jobs in namespace: ${NAMESPACE}"
echo "   Dry run: ${DRY_RUN}"
echo ""

# Delete all diagnostic TranslationJobs
echo "📋 Finding diagnostic TranslationJobs..."
DIAGNOSTIC_JOBS=$(kubectl get translationjobs -n "${NAMESPACE}" -o json 2>/dev/null | \
    jq -r '.items[] | select(
        (.metadata.labels."glooscap.dasmlab.org/diagnostic" == "true") or 
        (.metadata.name != null and (.metadata.name | type == "string") and (.metadata.name | startswith("diagnostic-")))
    ) | .metadata.name' 2>/dev/null || echo "")

if [ -n "${DIAGNOSTIC_JOBS}" ]; then
    COUNT=$(echo "${DIAGNOSTIC_JOBS}" | grep -v '^$' | wc -l)
    echo "   Found ${COUNT} diagnostic TranslationJobs"
    echo "${DIAGNOSTIC_JOBS}" | grep -v '^$' | while read -r job_name; do
        if [ -n "${job_name}" ]; then
            if [ "${DRY_RUN}" = "true" ]; then
                echo "   [DRY RUN] Would delete TranslationJob: ${job_name}"
            else
                echo "   🗑️  Deleting TranslationJob: ${job_name}"
                kubectl delete translationjob "${job_name}" -n "${NAMESPACE}" --ignore-not-found=true || true
            fi
        fi
    done
else
    echo "   No diagnostic TranslationJobs found"
fi

# Delete all diagnostic Kubernetes Jobs
echo ""
echo "📋 Finding diagnostic Kubernetes Jobs..."
DIAG_K8S_JOBS=$(kubectl get jobs -n "${NAMESPACE}" -o json 2>/dev/null | \
    jq -r '.items[] | select(
        (.metadata.labels."glooscap.dasmlab.org/diagnostic" == "true") or 
        (.metadata.name != null and (.metadata.name | type == "string") and (.metadata.name | startswith("translation-diagnostic-")))
    ) | .metadata.name' 2>/dev/null || echo "")

if [ -n "${DIAG_K8S_JOBS}" ]; then
    COUNT=$(echo "${DIAG_K8S_JOBS}" | grep -v '^$' | wc -l)
    echo "   Found ${COUNT} diagnostic Kubernetes Jobs"
    echo "${DIAG_K8S_JOBS}" | grep -v '^$' | while read -r job_name; do
        if [ -n "${job_name}" ]; then
            if [ "${DRY_RUN}" = "true" ]; then
                echo "   [DRY RUN] Would delete Job: ${job_name}"
            else
                echo "   🗑️  Deleting Job: ${job_name}"
                kubectl delete job "${job_name}" -n "${NAMESPACE}" --ignore-not-found=true --cascade=orphan || true
            fi
        fi
    done
else
    echo "   No diagnostic Kubernetes Jobs found"
fi

# Delete all diagnostic pods (orphaned or from deleted jobs)
echo ""
echo "📋 Finding diagnostic Pods..."
DIAG_PODS=$(kubectl get pods -n "${NAMESPACE}" -o json 2>/dev/null | \
    jq -r '.items[] | select(
        (.metadata.labels."glooscap.dasmlab.org/diagnostic" == "true") or 
        (.metadata.name != null and (.metadata.name | type == "string") and (.metadata.name | startswith("translation-diagnostic-")))
    ) | .metadata.name' 2>/dev/null || echo "")

if [ -n "${DIAG_PODS}" ]; then
    COUNT=$(echo "${DIAG_PODS}" | grep -v '^$' | wc -l)
    echo "   Found ${COUNT} diagnostic Pods"
    echo "${DIAG_PODS}" | grep -v '^$' | while read -r pod_name; do
        if [ -n "${pod_name}" ]; then
            if [ "${DRY_RUN}" = "true" ]; then
                echo "   [DRY RUN] Would delete Pod: ${pod_name}"
            else
                echo "   🗑️  Deleting Pod: ${pod_name}"
                kubectl delete pod "${pod_name}" -n "${NAMESPACE}" --ignore-not-found=true || true
            fi
        fi
    done
else
    echo "   No diagnostic Pods found"
fi

# Also clean up ALL completed/failed translation jobs (not just diagnostic)
echo ""
echo "📋 Finding ALL completed/failed translation jobs..."
ALL_FAILED_JOBS=$(kubectl get jobs -n "${NAMESPACE}" -o json 2>/dev/null | \
    jq -r '.items[] | select(.status.succeeded > 0 or .status.failed > 0) | select(.metadata.name | startswith("translation-")) | .metadata.name' || echo "")

if [ -n "${ALL_FAILED_JOBS}" ]; then
    COUNT=$(echo "${ALL_FAILED_JOBS}" | wc -l)
    echo "   Found ${COUNT} completed/failed translation jobs"
    if [ "${DRY_RUN}" = "true" ]; then
        echo "   [DRY RUN] Would delete ${COUNT} jobs"
    else
        echo "   🗑️  Deleting ${COUNT} completed/failed jobs..."
        echo "${ALL_FAILED_JOBS}" | xargs -I {} kubectl delete job {} -n "${NAMESPACE}" --ignore-not-found=true --cascade=orphan || true
    fi
else
    echo "   No completed/failed translation jobs found"
fi

echo ""
if [ "${DRY_RUN}" = "true" ]; then
    echo "✅ Dry run complete. Use DRY_RUN=false to actually delete."
else
    echo "✅ Aggressive cleanup complete!"
    echo ""
    echo "💡 Remaining jobs:"
    kubectl get jobs -n "${NAMESPACE}" --no-headers 2>/dev/null | wc -l || echo "0"
    echo "💡 Remaining pods:"
    kubectl get pods -n "${NAMESPACE}" --no-headers 2>/dev/null | grep -E "translation|diagnostic" | wc -l || echo "0"
fi

