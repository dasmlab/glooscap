#!/bin/bash
#
# cleanup-all-jobs-fast.sh - Fast cleanup using kubectl label selectors
#
# This is a faster alternative that uses kubectl's built-in filtering
# instead of jq parsing, which is much faster for large numbers of resources.
#

set -euo pipefail

NAMESPACE="${NAMESPACE:-glooscap-system}"
DRY_RUN="${DRY_RUN:-false}"

echo "🚀 Fast cleanup of all diagnostic and completed/failed jobs"
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
    echo "   All completed/failed translation jobs:"
    kubectl get jobs -n "${NAMESPACE}" --field-selector status.successful=1 --no-headers 2>/dev/null | grep "^translation-" | wc -l || echo "0"
    kubectl get jobs -n "${NAMESPACE}" --field-selector status.failed=1 --no-headers 2>/dev/null | grep "^translation-" | wc -l || echo "0"
else
    echo "🗑️  Deleting diagnostic TranslationJobs..."
    kubectl delete translationjobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --ignore-not-found=true 2>/dev/null || true
    
    echo "🗑️  Deleting diagnostic Jobs..."
    kubectl delete jobs -n "${NAMESPACE}" -l glooscap.dasmlab.org/diagnostic=true --ignore-not-found=true --cascade=orphan 2>/dev/null || true
    
    echo "🗑️  Deleting all completed translation jobs..."
    kubectl get jobs -n "${NAMESPACE}" --field-selector status.successful=1 --no-headers 2>/dev/null | grep "^translation-" | awk '{print $1}' | xargs -r -P 10 -I {} kubectl delete job {} -n "${NAMESPACE}" --ignore-not-found=true --cascade=orphan 2>/dev/null || true
    
    echo "🗑️  Deleting all failed translation jobs..."
    kubectl get jobs -n "${NAMESPACE}" --field-selector status.failed=1 --no-headers 2>/dev/null | grep "^translation-" | awk '{print $1}' | xargs -r -P 10 -I {} kubectl delete job {} -n "${NAMESPACE}" --ignore-not-found=true --cascade=orphan 2>/dev/null || true
    
    echo ""
    echo "✅ Fast cleanup complete!"
    echo ""
    echo "💡 Remaining jobs:"
    kubectl get jobs -n "${NAMESPACE}" --no-headers 2>/dev/null | wc -l || echo "0"
fi

