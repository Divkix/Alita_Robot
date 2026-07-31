#!/bin/bash

# PostgreSQL Migration Script for Alita Robot (vendor-agnostic)
# Uses migrations/ as source-of-truth and auto-cleans for plain PostgreSQL

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Load .env file if it exists (optional)
if [[ -f "$SCRIPT_DIR/.env" ]]; then
  echo -e "${BLUE}Loading configuration from ${SCRIPT_DIR}/.env...${NC}"
  set -a
  # shellcheck source=/dev/null
  source "$SCRIPT_DIR/.env"
  set +a
fi

# Database configuration from environment variables
DB_HOST="${PSQL_DB_HOST:-}"
DB_PORT="${PSQL_DB_PORT:-5432}"
DB_NAME="${PSQL_DB_NAME:-}"
DB_USER="${PSQL_DB_USER:-}"
DB_PASSWORD="${PSQL_DB_PASSWORD:-}"
DB_SSLMODE="${PSQL_DB_SSLMODE:-prefer}"

# Migration directory resolution
# Priority:
# 1) Respect MIGRATIONS_DIR if provided
# 2) Use local migrations directory relative to this script (rare)
# 3) Auto-clean from migrations/ into a temp dir

DEFAULT_MIGRATIONS_DIR="$SCRIPT_DIR/migrations"
SRC_MIGRATIONS_DIR="$SCRIPT_DIR/../migrations"
AUTO_TEMP_DIR=""
SOURCE_MIGRATIONS_DIR=""

cleanup_temp_dir() {
  if [[ -n "$AUTO_TEMP_DIR" && -d "$AUTO_TEMP_DIR" ]]; then
    rm -rf "$AUTO_TEMP_DIR" || true
  fi
}

prepare_clean_migrations_from_supabase() {
  local source_dir="$1"
  local dest_dir="$2"
  mkdir -p "$dest_dir"

  # Supabase-only extensions that don't exist in standard PostgreSQL
  local supabase_extensions="hypopg|index_advisor|pg_graphql|pg_stat_monitor|pgaudit|plv8|pgsodium|vault|wrappers"

  for file in "$source_dir"/*.sql; do
    [[ -e "$file" ]] || continue
    local filename
    filename=$(basename "$file")
    [[ "$filename" == *.rollback.sql ]] && continue
    # Remove Supabase-specific GRANT statements and make DDL idempotent
    # Use perl -0777 for multi-line matching (ALTER TABLE...ADD CONSTRAINT can span lines)
    sed -E '/(grant|GRANT).*(anon|authenticated|service_role)/d' "$file" \
      | sed 's/ with schema "extensions"//g' \
      | perl -pe "s/.*create extension.*($supabase_extensions).*/-- SKIPPED: Supabase-only extension/gi" \
      | sed 's/create extension if not exists/CREATE EXTENSION IF NOT EXISTS/g' \
      | sed 's/create extension/CREATE EXTENSION IF NOT EXISTS/g' \
      | perl -pe 's/create\s+table\s+(?!if\s+not\s+exists)/CREATE TABLE IF NOT EXISTS /gi' \
      | perl -pe 's/create\s+unique\s+index\s+(?!if\s+not\s+exists)/CREATE UNIQUE INDEX IF NOT EXISTS /gi' \
      | perl -pe 's/create\s+index\s+(?!if\s+not\s+exists)/CREATE INDEX IF NOT EXISTS /gi' \
      | perl -0777 -pe 's/^ALTER\s+TABLE\s+(\S+)\s+ADD\s+CONSTRAINT\s+(\S+)\s+([^;]+);/DO \$\$ BEGIN ALTER TABLE $1 ADD CONSTRAINT $2 $3; EXCEPTION WHEN duplicate_object THEN NULL; END \$\$;/gims' \
      | perl -0777 -pe 's/CREATE\s+TRIGGER\s+(\w+)\s+(BEFORE|AFTER|INSTEAD\s+OF)\s+(\w+)\s+ON\s+(\S+)\s+FOR\s+EACH\s+(\w+)\s+EXECUTE\s+(FUNCTION|PROCEDURE)\s+([^;]+);/DO \$\$ BEGIN CREATE TRIGGER $1 $2 $3 ON $4 FOR EACH $5 EXECUTE $6 $7; EXCEPTION WHEN duplicate_object THEN NULL; END \$\$;/gis' \
      | perl -pe "s/EXECUTE\s+'DROP INDEX IF EXISTS '\s*\|\|\s*(\S+);/BEGIN EXECUTE 'DROP INDEX IF EXISTS ' || \$1; EXCEPTION WHEN dependent_objects_still_exist THEN NULL; END;/gi" \
      > "$dest_dir/$filename"
  done
}

if [[ -n "${MIGRATIONS_DIR:-}" ]]; then
  SOURCE_MIGRATIONS_DIR="$MIGRATIONS_DIR"
elif [[ -d "$DEFAULT_MIGRATIONS_DIR" ]]; then
  MIGRATIONS_DIR="$DEFAULT_MIGRATIONS_DIR"
  SOURCE_MIGRATIONS_DIR="$DEFAULT_MIGRATIONS_DIR"
elif [[ -d "$SRC_MIGRATIONS_DIR" ]]; then
  AUTO_TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/alita-migrations.XXXXXX")
  trap cleanup_temp_dir EXIT
  echo -e "${BLUE}No local migrations found. Auto-preparing from migrations/...${NC}"
  prepare_clean_migrations_from_supabase "$SRC_MIGRATIONS_DIR" "$AUTO_TEMP_DIR"
  MIGRATIONS_DIR="$AUTO_TEMP_DIR"
  SOURCE_MIGRATIONS_DIR="$SRC_MIGRATIONS_DIR"
else
  echo -e "${RED}Error: Could not locate migrations.${NC}"
  echo -e "${YELLOW}Ensure migrations/ exists, or set MIGRATIONS_DIR to a directory with .sql files.${NC}"
  exit 1
fi

print_color() {
  local color=$1; shift
  echo -e "${color}$*${NC}"
}

execute_sql() {
  PGPASSWORD="${DB_PASSWORD}" PGSSLMODE="${DB_SSLMODE}" psql \
    -h "${DB_HOST}" \
    -p "${DB_PORT}" \
    -U "${DB_USER}" \
    -d "${DB_NAME}" \
    -v ON_ERROR_STOP=1 \
    "$@" 2>&1
}

file_checksum() {
  local file=$1
  if command -v sha256sum > /dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum > /dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    print_color "$RED" "Error: sha256sum or shasum is required"
    return 1
  fi
}

is_migration_applied() {
  local version=$1
  local escaped_version
  local result
  escaped_version=${version//\'/\'\'}
  result=$(execute_sql -t -c "SELECT COUNT(*) FROM schema_migrations WHERE version = '${escaped_version}';" 2>/dev/null | tr -d ' ') || {
    print_color "$RED" "Error: Could not read migration status"
    exit 1
  }
  [[ "$result" == "1" ]]
}

verify_migration_checksum() {
  local version=$1
  local escaped_version
  local current
  local stored
  escaped_version=${version//\'/\'\'}
  current=$(file_checksum "$SOURCE_MIGRATIONS_DIR/$version") || return 1
  stored=$(execute_sql -At -c "SELECT COALESCE(checksum, '') FROM schema_migrations WHERE version = '${escaped_version}';") || return 1

  if [[ -z "$stored" ]]; then
    execute_sql -c "UPDATE schema_migrations SET checksum = '${current}' WHERE version = '${escaped_version}';" > /dev/null || return 1
  elif [[ "$stored" != "$current" ]]; then
    print_color "$RED" "  ✗ ${version} was modified after it was applied"
    return 1
  fi
}

apply_migration() {
  local migration_file=$1
  local version
  local execution_file
  local escaped_version
  local checksum
  version=$(basename "$migration_file")
  escaped_version=${version//\'/\'\'}
  checksum=$(file_checksum "$SOURCE_MIGRATIONS_DIR/$version") || return 1
  execution_file=$(mktemp) || return 1

  if grep -Eiq '^[[:space:]]*ROLLBACK([[:space:]]+(WORK|TRANSACTION))?[[:space:]]*;' "$migration_file"; then
    rm -f "$execution_file"
    print_color "$RED" "    ✗ Top-level ROLLBACK is not allowed in a migration"
    return 1
  fi

  # The script owns the transaction. Remove only top-level transaction
  # boundaries embedded in legacy files; PL/pgSQL BEGIN/END blocks do not end
  # with a semicolon on their own line and remain untouched.
  if ! perl -ne 'print unless /^\s*(?:BEGIN|COMMIT)(?:\s+(?:WORK|TRANSACTION))?\s*;\s*(?:--.*)?$/i' \
    "$migration_file" > "$execution_file"; then
    rm -f "$execution_file"
    print_color "$RED" "    ✗ Failed to prepare migration"
    return 1
  fi
  printf "\nINSERT INTO schema_migrations (version, checksum) VALUES ('%s', '%s');\n" \
    "$escaped_version" "$checksum" >> "$execution_file" || return 1

  print_color "$BLUE" "  → Applying ${version}..."
  if execute_sql --single-transaction -f "$execution_file"; then
    rm -f "$execution_file"
    print_color "$GREEN" "    ✓ Applied successfully"
    return 0
  else
    rm -f "$execution_file"
    print_color "$RED" "    ✗ Failed to apply migration"
    return 1
  fi
}

main() {
  print_color "$BLUE" "=========================================="
  print_color "$BLUE" "PostgreSQL Migration Tool"
  print_color "$BLUE" "=========================================="
  echo

  if [[ -z "$DB_HOST" || -z "$DB_NAME" || -z "$DB_USER" ]]; then
    print_color "$RED" "Error: Required environment variables not set"
    print_color "$YELLOW" "Set: PSQL_DB_HOST, PSQL_DB_NAME, PSQL_DB_USER (and PSQL_DB_PASSWORD if required)"
    exit 1
  fi

  if [[ ! -d "$MIGRATIONS_DIR" ]]; then
    print_color "$RED" "Error: Migrations directory not found: $MIGRATIONS_DIR"
    print_color "$YELLOW" "Ensure migrations/ exists, or set MIGRATIONS_DIR to a directory with .sql files"
    exit 1
  fi

  print_color "$BLUE" "Testing database connection..."
  if ! execute_sql -c "SELECT 1;" > /dev/null; then
    print_color "$RED" "Error: Cannot connect to database"
    print_color "$YELLOW" "Host: $DB_HOST:$DB_PORT | DB: $DB_NAME | User: $DB_USER"
    exit 1
  fi
  print_color "$GREEN" "✓ Connected to database"
  echo

  print_color "$BLUE" "Ensuring migrations table exists..."
  execute_sql <<EOF
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(64)
);
ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum VARCHAR(64);
EOF
  print_color "$GREEN" "✓ Migrations table ready"
  echo

  print_color "$BLUE" "Scanning for migrations..."
  migration_files=()
  for f in "$MIGRATIONS_DIR"/*.sql; do
    [[ -f "$f" && "$(basename "$f")" != *.rollback.sql ]] && migration_files+=("$f")
  done
  if [[ ${#migration_files[@]} -eq 0 ]]; then
    print_color "$YELLOW" "No migration files found in $MIGRATIONS_DIR"
    exit 0
  fi
  print_color "$GREEN" "Found ${#migration_files[@]} migration files"
  echo

  print_color "$BLUE" "Applying migrations..."
  applied_count=0
  skipped_count=0
  failed_count=0
  for migration_file in "${migration_files[@]}"; do
    version=$(basename "$migration_file")
    if is_migration_applied "$version"; then
      if ! verify_migration_checksum "$version"; then
        print_color "$RED" "Migration checksum verification failed. Stopping execution."
        exit 1
      fi
      print_color "$YELLOW" "  ○ Skipping ${version} (already applied)"
      skipped_count=$((skipped_count + 1))
    else
      if apply_migration "$migration_file"; then
        applied_count=$((applied_count + 1))
      else
        failed_count=$((failed_count + 1))
        print_color "$RED" "Migration failed. Stopping execution."
        exit 1
      fi
    fi
  done

  echo
  print_color "$BLUE" "=========================================="
  print_color "$GREEN" "Migration Summary:"
  echo "  • Applied: $applied_count"
  echo "  • Skipped: $skipped_count"
  echo "  • Failed: $failed_count"
  print_color "$BLUE" "=========================================="

  echo
  print_color "$BLUE" "Current migration status:"
  execute_sql -c "SELECT version, executed_at FROM schema_migrations ORDER BY executed_at DESC LIMIT 5;"
  echo
  table_count=$(execute_sql -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE';" | tr -d ' ')
  print_color "$GREEN" "✓ Database has $table_count tables"
}

main "$@"
