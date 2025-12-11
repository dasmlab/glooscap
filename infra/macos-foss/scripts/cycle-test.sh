#!/usr/bin/env bash
# cycle-test.sh
# Runs the complete dev cycle: setup -> start -> deploy -> undeploy -> stop -> remove
# Reports results for each step

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo ""
    echo "=========================================="
    echo -e "${BLUE}STEP $1: $2${NC}"
    echo "=========================================="
    echo ""
}

# Results tracking
RESULTS=()
STEP=0

run_step() {
    STEP=$((STEP + 1))
    STEP_NAME="$1"
    SCRIPT="$2"
    
    log_step "${STEP}" "${STEP_NAME}"
    
    if [ -f "${SCRIPT}" ]; then
        if bash "${SCRIPT}" 2>&1; then
            RESULTS+=("${STEP}. ${STEP_NAME}: ✓ SUCCESS")
            log_success "${STEP_NAME} completed successfully"
            return 0
        else
            RESULTS+=("${STEP}. ${STEP_NAME}: ✗ FAILED")
            log_error "${STEP_NAME} failed"
            return 1
        fi
    else
        log_error "Script not found: ${SCRIPT}"
        RESULTS+=("${STEP}. ${STEP_NAME}: ✗ SCRIPT NOT FOUND")
        return 1
    fi
}

# Start
echo ""
log_info "Starting complete dev cycle test..."
echo "This will run all 6 steps:"
echo "  1. Setup macOS environment"
echo "  2. Start k3d cluster"
echo "  3. Deploy Glooscap"
echo "  4. Undeploy Glooscap"
echo "  5. Stop k3d cluster"
echo "  6. Remove k3d cluster"
echo ""

# Step 1: Setup
run_step "Setup macOS Environment" "${SCRIPT_DIR}/setup-macos-env.sh"
SETUP_RESULT=$?

# Step 2: Start
run_step "Start k3d Cluster" "${SCRIPT_DIR}/start-k3d.sh"
START_RESULT=$?

# Step 3: Deploy
if [ ${START_RESULT} -eq 0 ]; then
    run_step "Deploy Glooscap" "${SCRIPT_DIR}/deploy-glooscap.sh"
    DEPLOY_RESULT=$?
else
    log_warn "Skipping deploy (start failed)"
    RESULTS+=("3. Deploy Glooscap: ⊘ SKIPPED (start failed)")
    DEPLOY_RESULT=1
fi

# Step 4: Undeploy
if [ ${DEPLOY_RESULT} -eq 0 ]; then
    run_step "Undeploy Glooscap" "${SCRIPT_DIR}/undeploy-glooscap.sh"
    UNDEPLOY_RESULT=$?
else
    log_warn "Skipping undeploy (deploy failed or skipped)")
    RESULTS+=("4. Undeploy Glooscap: ⊘ SKIPPED")
    UNDEPLOY_RESULT=0
fi

# Step 5: Stop
run_step "Stop k3d Cluster" "${SCRIPT_DIR}/stop-k3d.sh"
STOP_RESULT=$?

# Step 6: Remove
run_step "Remove k3d Cluster" "${SCRIPT_DIR}/remove-k3d.sh"
REMOVE_RESULT=$?

# Final Report
echo ""
echo "=========================================="
echo -e "${BLUE}FINAL REPORT${NC}"
echo "=========================================="
echo ""

for result in "${RESULTS[@]}"; do
    if [[ "${result}" == *"✓ SUCCESS"* ]]; then
        echo -e "${GREEN}${result}${NC}"
    elif [[ "${result}" == *"✗"* ]]; then
        echo -e "${RED}${result}${NC}"
    else
        echo -e "${YELLOW}${result}${NC}"
    fi
done

echo ""
TOTAL_STEPS=${#RESULTS[@]}
SUCCESS_COUNT=$(echo "${RESULTS[@]}" | grep -o "✓ SUCCESS" | wc -l | tr -d ' ')

if [ ${SUCCESS_COUNT} -eq ${TOTAL_STEPS} ]; then
    log_success "All steps completed successfully!"
    exit 0
else
    log_warn "Some steps failed or were skipped"
    log_info "Success: ${SUCCESS_COUNT}/${TOTAL_STEPS} steps"
    exit 1
fi

