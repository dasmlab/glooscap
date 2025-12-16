#!/bin/bash
#
# git-branch-helper.sh - Helper script for managing branches in multi-machine development
#
# Usage:
#   ./scripts/git-branch-helper.sh status          - Show current branch status
#   ./scripts/git-branch-helper.sh create <name>   - Create and switch to new development branch
#   ./scripts/git-branch-helper.sh switch <name>   - Switch to existing branch
#   ./scripts/git-branch-helper.sh update          - Update current branch with main
#   ./scripts/git-branch-helper.sh list             - List all branches
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${PROJECT_ROOT}"

case "${1:-status}" in
    status)
        echo "📊 Current Git Status"
        echo "===================="
        echo ""
        echo "Current branch:"
        git branch --show-current
        echo ""
        echo "Branch status:"
        git status -sb
        echo ""
        echo "Recent commits:"
        git log --oneline -5
        ;;
    
    create)
        if [ -z "${2:-}" ]; then
            echo "❌ Error: Branch name required"
            echo "Usage: $0 create <branch-name>"
            exit 1
        fi
        BRANCH_NAME="development/${2}"
        echo "🌿 Creating branch: ${BRANCH_NAME}"
        git checkout -b "${BRANCH_NAME}"
        echo "✅ Created and switched to ${BRANCH_NAME}"
        echo ""
        echo "💡 Next steps:"
        echo "   1. Make your changes"
        echo "   2. git add . && git commit -m 'your message'"
        echo "   3. git push -u origin ${BRANCH_NAME}"
        ;;
    
    switch)
        if [ -z "${2:-}" ]; then
            echo "❌ Error: Branch name required"
            echo "Usage: $0 switch <branch-name>"
            echo ""
            echo "Available branches:"
            git branch -a | grep -E "development|feature|fix" | head -10
            exit 1
        fi
        BRANCH_NAME="${2}"
        # If no prefix, assume development/
        if [[ ! "${BRANCH_NAME}" =~ ^(development|feature|fix)/ ]]; then
            BRANCH_NAME="development/${BRANCH_NAME}"
        fi
        echo "🔄 Switching to branch: ${BRANCH_NAME}"
        git fetch origin
        git checkout "${BRANCH_NAME}" 2>/dev/null || {
            echo "⚠️  Branch not found locally, trying to checkout from remote..."
            git checkout -b "${BRANCH_NAME}" "origin/${BRANCH_NAME}" || {
                echo "❌ Error: Branch ${BRANCH_NAME} not found"
                exit 1
            }
        }
        echo "✅ Switched to ${BRANCH_NAME}"
        ;;
    
    update)
        CURRENT_BRANCH=$(git branch --show-current)
        echo "🔄 Updating ${CURRENT_BRANCH} with latest from main..."
        git fetch origin
        git merge origin/main || {
            echo "⚠️  Merge conflicts detected. Resolve them and commit."
            exit 1
        }
        echo "✅ Branch updated"
        ;;
    
    list)
        echo "📋 Available Branches"
        echo "===================="
        echo ""
        echo "Local branches:"
        git branch
        echo ""
        echo "Remote branches:"
        git branch -r | grep -E "development|feature|fix" | head -20
        ;;
    
    *)
        echo "Git Branch Helper"
        echo "================="
        echo ""
        echo "Usage: $0 <command> [args]"
        echo ""
        echo "Commands:"
        echo "  status          - Show current branch status"
        echo "  create <name>   - Create and switch to new development branch"
        echo "  switch <name>   - Switch to existing branch"
        echo "  update          - Update current branch with main"
        echo "  list             - List all branches"
        echo ""
        echo "Examples:"
        echo "  $0 status"
        echo "  $0 create linux-operator-fixes"
        echo "  $0 switch development/linux-operator-fixes"
        echo "  $0 update"
        exit 1
        ;;
esac

