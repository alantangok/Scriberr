#!/bin/bash
# Deploy script for Scriberr to likshing server
# Usage: ./deploy.sh [--binary-only|--frontend-only|--full]

set -e

REMOTE_HOST="likshing"
REMOTE_PATH="/opt/scriberr"
BINARY_NAME="scriberr"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Parse arguments
DEPLOY_MODE="${1:-full}"

case "$DEPLOY_MODE" in
    --binary-only)
        DEPLOY_BINARY=true
        DEPLOY_FRONTEND=false
        ;;
    --frontend-only)
        DEPLOY_BINARY=false
        DEPLOY_FRONTEND=true
        ;;
    --full|*)
        DEPLOY_BINARY=true
        DEPLOY_FRONTEND=true
        ;;
esac

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check SSH connectivity
log_info "Checking SSH connectivity to ${REMOTE_HOST}..."
if ! ssh -o ConnectTimeout=5 "${REMOTE_HOST}" "echo 'Connected'" &>/dev/null; then
    log_error "Cannot connect to ${REMOTE_HOST}. Check your SSH config."
    exit 1
fi

# Build and deploy binary
if [ "$DEPLOY_BINARY" = true ]; then
    log_info "Building Linux binary..."
    GOOS=linux GOARCH=amd64 go build -o "${BINARY_NAME}-linux" ./cmd/server/main.go

    log_info "Deploying binary to ${REMOTE_HOST}:${REMOTE_PATH}..."
    scp "${BINARY_NAME}-linux" "${REMOTE_HOST}:${REMOTE_PATH}/${BINARY_NAME}"

    # Clean up local binary
    rm -f "${BINARY_NAME}-linux"
    log_info "Binary deployed."
fi

# Deploy frontend
if [ "$DEPLOY_FRONTEND" = true ]; then
    # Check if frontend dist exists
    if [ -d "web/frontend/dist" ]; then
        log_info "Deploying frontend to ${REMOTE_HOST}..."
        rsync -avz --delete web/frontend/dist/ "${REMOTE_HOST}:${REMOTE_PATH}/web/frontend/dist/"
        log_info "Frontend deployed."
    else
        log_warn "Frontend dist not found. Run 'npm run build' in web/frontend first."
    fi
fi

# Restart service
log_info "Restarting scriberr service..."
ssh "${REMOTE_HOST}" "sudo systemctl restart scriberr"

# Check service status
log_info "Checking service status..."
ssh "${REMOTE_HOST}" "sudo systemctl status scriberr --no-pager | head -20"

echo ""
log_info "Deployment complete!"
echo "  URL: https://scriberr.hachitg4ever.com"
