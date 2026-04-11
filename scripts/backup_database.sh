#!/bin/bash
# Pre-migration database backup script
# Usage: ./scripts/backup_database.sh

set -eo pipefail

echo "💾 Database Backup for Migration Safety"
echo "========================================"

# Check if DATABASE_URL is set
if [ -z "$DATABASE_URL" ]; then
    echo "❌ ERROR: DATABASE_URL environment variable not set"
    exit 1
fi

# Generate backup filename with timestamp
BACKUP_DIR="./backups"
mkdir -p "$BACKUP_DIR"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/pre_migration_${TIMESTAMP}.sql.gz"

echo "📦 Creating backup..."
echo "   Destination: $BACKUP_FILE"

# Create compressed backup with pipe failure detection
pg_dump "$DATABASE_URL" | gzip > "$BACKUP_FILE"

# Check pipe exit status
if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "❌ ERROR: pg_dump failed"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Verify backup was created
if [ ! -f "$BACKUP_FILE" ]; then
    echo "❌ ERROR: Backup file was not created"
    exit 1
fi

# Verify backup integrity
echo "🔍 Verifying backup integrity..."
if ! gzip -t "$BACKUP_FILE" 2>/dev/null; then
    echo "❌ ERROR: Backup file is corrupted"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Get backup size
BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)

echo "✅ Backup created successfully!"
echo "   Size: $BACKUP_SIZE"
echo ""
echo "📋 To restore this backup if needed:"
echo "   gunzip -c $BACKUP_FILE | psql $DATABASE_URL"
echo ""
echo "⚠️  Keep this backup safe until migration is verified in production"
