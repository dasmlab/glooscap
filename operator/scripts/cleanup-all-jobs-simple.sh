#!/bin/bash
#
# cleanup-all-jobs-simple.sh - Simple, reliable cleanup using kubectl delete with selectors
#
# This script uses kubectl's native delete with label/field selectors
# which is the most efficient and reliable method.
#

set -euo pipefail

NAMESPACE="${NAMESPACE:-glooscap-system}"
DRY_RUN="${DRY_RUN:-false}"

echo "🧹 Simple cleanup of all diagnostic and completed/failed jobs"
echo "   Namespace: ${NAMESPACE}"
echo "   Dry run: ${DRY_RUN}"
echo ""

if [ "${DRY_RUN}" = "true" ]; then
    echo "📋 [DRY RUN] Would delete:"
    echo ""
    echo "   Diagnostic TranslationJobs:"
    kubectl get translationjobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --no-headers 2>/dev/null | wc -l || echo "0"
    echo ""
    echo "   Diagnostic Jobs:"
    kubectl get jobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --no-headers 2>/dev/null | wc -l || echo "0"
    echo ""
    echo "   All completed translation jobs:"
    kubectl get jobs -n "${NAMESPACE}" --field-selector status.successful=1 --no-headers 2>/dev/null | grep "^translation-" | wc -l || echo "0"
    echo ""
    echo "   All failed translation jobs:"
    kubectl get jobs -n "${NAMESPACE}" --field-selector status.failed=1 --no-headers 2>/dev/null | grep "^translation-" | wc -l || echo "0"
    exit 0
fi

# Delete diagnostic TranslationJobs by label
echo "🗑️  Step 1: Deleting diagnostic TranslationJobs..."
COUNT=$(kubectl get translationjobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --no-headers 2>/dev/null | wc -l || echo "0")
if [ "${COUNT}" -gt 0 ]; then
    echo "   Found ${COUNT} diagnostic TranslationJobs"
    kubectl delete translationjobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --ignore-not-found=true --wait=false 2>&1 | grep -v "^$" || true
    echo "   ✅ Deletion initiated"
else
    echo "   No diagnostic TranslationJobs found"
fi

# Delete diagnostic Jobs by label (use --cascade=orphan to avoid waiting for pods)
echo ""
echo "🗑️  Step 2: Deleting diagnostic Kubernetes Jobs..."
COUNT=$(kubectl get jobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --no-headers 2>/dev/null | wc -l || echo "0")
if [ "${COUNT}" -gt 0 ]; then
    echo "   Found ${COUNT} diagnostic Jobs"
    kubectl delete jobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --ignore-not-found=true --cascade=orphan --wait=false 2>&1 | grep -v "^$" || true
    echo "   ✅ Deletion initiated"
else
    echo "   No diagnostic Jobs found"
fi

# Delete all completed translation jobs
echo ""
echo "🗑️  Step 3: Deleting completed translation jobs..."
COMPLETED=$(kubectl get jobs -n "${NAMESPACE}" --field-selector status.successful=1 --no-headers 2>/dev/null | grep "^translation-" | awk '{print $1}' || echo "")
if [ -n "${COMPLETED}" ]; then
    COUNT=$(echo "${COMPLETED}" | wc -l)
    echo "   Found ${COUNT} completed translation jobs"
    echo "${COMPLETED}" | while read -r job; do
        [ -n "${job}" ] && kubectl delete job "${job}" -n "${NAMESPACE}" --ignore-not-found=true --cascade=orphan --wait=false 2>/dev/null || true
    done
    echo "   ✅ Deletion initiated"
else
    echo "   No completed translation jobs found"
fi

# Delete all failed translation jobs
echo ""
echo "🗑️  Step 4: Deleting failed translation jobs..."
FAILED=$(kubectl get jobs -n "${NAMESPACE}" --field-selector status.failed=1 --no-headers 2>/dev/null | grep "^translation-" | awk '{print $1}' || echo "")
if [ -n "${FAILED}" ]; then
    COUNT=$(echo "${FAILED}" | wc -l)
    echo "   Found ${COUNT} failed translation jobs"
    echo "${FAILED}" | while read -r job; do
        [ -n "${job}" ] && kubectl delete job "${job}" -n "${NAMESPACE}" --ignore-not-found=true --cascade=orphan --wait=false 2>/dev/null || true
    done
    echo "   ✅ Deletion initiated"
else
    echo "   No failed translation jobs found"
fi

echo ""
echo "✅ Cleanup commands issued!"
echo ""
echo "💡 Note: Deletions are running in background (--wait=false)."
echo "   Check progress with: kubectl get jobs -n ${NAMESPACE} | wc -l"
echo ""
echo "💡 Remaining jobs:"
kubectl get jobs -n "${NAMESPACE}" --no-headers 2>/dev/null | wc -l || echo "0"

