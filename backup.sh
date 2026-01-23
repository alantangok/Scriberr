#!/bin/bash
# Backup script for Scriberr project
# Creates timestamped tarball backup excluding node_modules, .git, etc.

set -e

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PROJECT_NAME="scriberr"
BACKUP_DIR="${HOME}/backups/${PROJECT_NAME}"
BACKUP_FILE="${BACKUP_DIR}/${PROJECT_NAME}_${TIMESTAMP}.tar.gz"

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

# Get the project root directory (where this script is located)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create the backup
tar -czf "${BACKUP_FILE}" \
    --exclude='node_modules' \
    --exclude='.git' \
    --exclude='*.pyc' \
    --exclude='__pycache__' \
    --exclude='.venv' \
    --exclude='venv' \
    --exclude='*.egg-info' \
    --exclude='.DS_Store' \
    --exclude='tmp' \
    --exclude='dist' \
    --exclude='build' \
    -C "$(dirname "${SCRIPT_DIR}")" \
    "$(basename "${SCRIPT_DIR}")"

# Output for scripting (parseable)
echo "BACKUP_TIMESTAMP=${TIMESTAMP}"
echo "BACKUP_FILE=${BACKUP_FILE}"

# Human-readable output
echo ""
echo "Backup created successfully:"
echo "  Location: ${BACKUP_FILE}"
echo "  Size: $(du -h "${BACKUP_FILE}" | cut -f1)"

# Keep only last 10 backups
cd "${BACKUP_DIR}"
ls -t *.tar.gz 2>/dev/null | tail -n +11 | xargs -r rm -f

echo "  Retained: $(ls -1 *.tar.gz 2>/dev/null | wc -l | tr -d ' ') backups"
