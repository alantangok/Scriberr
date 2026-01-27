#!/bin/bash

# Scriberr Deployment Script for likshing Server
# This script builds and deploys the frontend to the production server

set -e  # Exit on any error

echo "🚀 Starting Scriberr deployment..."

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
REMOTE_SERVER="likshing"
REMOTE_PATH="/opt/scriberr"
BUILD_DIR="web/frontend/dist"

# Step 1: Build Frontend
echo -e "${BLUE}📦 Building frontend...${NC}"
cd web/frontend
npm run build
cd ../..

# Step 2: Deploy Frontend
echo -e "${BLUE}🚀 Deploying frontend to ${REMOTE_SERVER}...${NC}"
rsync -avz --delete --progress \
  ${BUILD_DIR}/ \
  ${REMOTE_SERVER}:${REMOTE_PATH}/web/frontend/dist/

# Step 3: Restart Service
echo -e "${BLUE}🔄 Restarting scriberr service...${NC}"
ssh ${REMOTE_SERVER} "sudo systemctl restart scriberr"

# Step 4: Wait for service to start
echo -e "${BLUE}⏳ Waiting for service to start...${NC}"
sleep 3

# Step 5: Verify deployment
echo -e "${BLUE}✅ Checking service status...${NC}"
ssh ${REMOTE_SERVER} "sudo systemctl status scriberr --no-pager | head -10"

echo -e "${GREEN}✅ Deployment complete!${NC}"
echo -e "${GREEN}🌐 Visit: https://scriberr.hachitg4ever.com${NC}"
echo ""
echo -e "${BLUE}💡 Tip: Force refresh browser (Cmd+Shift+R / Ctrl+Shift+F5) to clear cache${NC}"
