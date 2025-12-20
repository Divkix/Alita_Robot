package db

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
)

// MigrationRunner handles automatic database migrations
type MigrationRunner struct {
	db             *gorm.DB
	migrationsPath string
	cleanSQL       bool
}

// SchemaMigration represents a migration record in the database
type SchemaMigration struct {
	Version    string    `gorm:"primaryKey;column:version"`
	ExecutedAt time.Time `gorm:"column:executed_at"`
}

// TableName returns the database table name for schema migrations
func (SchemaMigration) TableName() string {
	return "schema_migrations"
}

// NewMigrationRunner creates a new migration runner instance
func NewMigrationRunner(db *gorm.DB) *MigrationRunner {
	return &MigrationRunner{
		db:             db,
		migrationsPath: config.AppConfig.MigrationsPath,
		cleanSQL:       true, // Always clean Supabase-specific SQL
	}
}

// RunMigrations executes all pending database migrations
func (m *MigrationRunner) RunMigrations() error {
	log.Info("[Migrations] Starting automatic database migration...")

	// Ensure migrations table exists
	if err := m.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get migration files
	files, err := m.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	if len(files) == 0 {
		log.Info("[Migrations] No migration files found")
		return nil
	}

	log.Infof("[Migrations] Found %d migration files", len(files))

	// Track statistics
	applied := 0
	skipped := 0

	// Apply each migration
	for _, file := range files {
		version := filepath.Base(file)

		// Check if already applied
		if m.isMigrationApplied(version) {
			log.Debugf("[Migrations] Skipping %s (already applied)", version)
			skipped++
			continue
		}

		// Apply migration
		log.Infof("[Migrations] Applying %s...", version)
		if err := m.applyMigration(file, version); err != nil {
			// Note: We return immediately on failure, so failed count would always be 1
			// Keeping for potential future use where we might continue on certain errors
			return fmt.Errorf("failed to apply migration %s: %w", version, err)
		}
		applied++
		log.Infof("[Migrations] Successfully applied %s", version)
	}

	// Log summary
	log.Infof("[Migrations] Migration complete - Applied: %d, Skipped: %d",
		applied, skipped)

	// Log current migration status
	m.logMigrationStatus()

	// Verify indexes after successful migrations
	if applied > 0 {
		if err := m.verifyIndexes(); err != nil {
			log.Warnf("[Migrations] Index verification failed: %v", err)
			// Don't fail the migration, just warn
		}
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist
func (m *MigrationRunner) ensureMigrationsTable() error {
	sql := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	return m.db.Exec(sql).Error
}

// getMigrationFiles returns a sorted list of migration SQL files
func (m *MigrationRunner) getMigrationFiles() ([]string, error) {
	// Check if migrations path exists
	if _, err := os.Stat(m.migrationsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations path does not exist: %s", m.migrationsPath)
	}

	// Find all SQL files
	pattern := filepath.Join(m.migrationsPath, "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// Sort files to ensure consistent order
	slices.Sort(files)
	return files, nil
}

// isMigrationApplied checks if a migration version has already been applied
func (m *MigrationRunner) isMigrationApplied(version string) bool {
	var count int64
	m.db.Model(&SchemaMigration{}).Where("version = ?", version).Count(&count)
	return count > 0
}

// splitSQLStatements splits a SQL string into individual statements
// It handles various edge cases including:
// - Quoted strings (single quotes, double quotes)
// - Dollar-quoted strings (PostgreSQL specific)
// - Comments (single-line and multi-line)
// - Semicolons inside strings
func (m *MigrationRunner) splitSQLStatements(sql string) []string {
	var statements []string
	var currentStmt strings.Builder

	runes := []rune(sql)
	length := len(runes)

	inSingleQuote := false
	inDoubleQuote := false
	inDollarQuote := false
	inLineComment := false
	inBlockComment := false
	dollarQuoteTag := ""

	for i := 0; i < length; i++ {
		char := runes[i]
		nextChar := rune(0)
		if i+1 < length {
			nextChar = runes[i+1]
		}

		// Handle line comments
		if !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inBlockComment {
			if char == '-' && nextChar == '-' {
				inLineComment = true
				currentStmt.WriteRune(char)
				continue
			}
		}

		if inLineComment {
			currentStmt.WriteRune(char)
			if char == '\n' {
				inLineComment = false
			}
			continue
		}

		// Handle block comments
		if !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inLineComment {
			if char == '/' && nextChar == '*' {
				inBlockComment = true
				currentStmt.WriteRune(char)
				continue
			}
		}

		if inBlockComment {
			currentStmt.WriteRune(char)
			if char == '*' && nextChar == '/' {
				currentStmt.WriteRune(nextChar)
				i++
				inBlockComment = false
			}
			continue
		}

		// Handle dollar quotes (PostgreSQL)
		if !inSingleQuote && !inDoubleQuote && !inLineComment && !inBlockComment {
			if char == '$' {
				// Check if this is the start or end of a dollar quote
				tagEnd := i + 1
				for tagEnd < length && (runes[tagEnd] != '$' && runes[tagEnd] != ' ' && runes[tagEnd] != '\n' && runes[tagEnd] != ';') {
					tagEnd++
				}

				if tagEnd < length && runes[tagEnd] == '$' {
					tag := string(runes[i : tagEnd+1])
					if inDollarQuote {
						// Check if this closes the current dollar quote
						if tag == dollarQuoteTag {
							inDollarQuote = false
							dollarQuoteTag = ""
						}
					} else {
						// Start a new dollar quote
						inDollarQuote = true
						dollarQuoteTag = tag
					}

					// Add the entire tag to the current statement
					for j := i; j <= tagEnd; j++ {
						currentStmt.WriteRune(runes[j])
					}
					i = tagEnd
					continue
				}
			}
		}

		// Handle single quotes
		if !inDoubleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			if char == '\'' {
				// Check for escaped single quote
				if i+1 < length && runes[i+1] == '\'' {
					currentStmt.WriteRune(char)
					currentStmt.WriteRune(runes[i+1])
					i++
					continue
				}
				inSingleQuote = !inSingleQuote
			}
		}

		// Handle double quotes
		if !inSingleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			if char == '"' {
				// Check for escaped double quote
				if i+1 < length && runes[i+1] == '"' {
					currentStmt.WriteRune(char)
					currentStmt.WriteRune(runes[i+1])
					i++
					continue
				}
				inDoubleQuote = !inDoubleQuote
			}
		}

		// Handle semicolons (statement separator)
		if char == ';' && !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			// End of statement
			stmt := strings.TrimSpace(currentStmt.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStmt.Reset()
		} else {
			currentStmt.WriteRune(char)
		}
	}

	// Add any remaining statement
	if currentStmt.Len() > 0 {
		stmt := strings.TrimSpace(currentStmt.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// applyMigration reads, cleans, and applies a single migration file
func (m *MigrationRunner) applyMigration(filepath, version string) error {
	// Validate that the file path is within the migrations directory to prevent path traversal
	if !strings.HasPrefix(filepath, m.migrationsPath) {
		return fmt.Errorf("invalid migration file path: %s", filepath)
	}

	// Additional validation: ensure the path doesn't contain suspicious patterns
	cleanPath := path.Clean(filepath)
	if cleanPath != filepath || strings.Contains(filepath, "..") {
		return fmt.Errorf("potentially unsafe migration file path: %s", filepath)
	}

	// Read migration file
	content, err := os.ReadFile(filepath) // #nosec G304 - path validation performed above
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Clean SQL if needed
	sql := string(content)
	if m.cleanSQL {
		sql = m.cleanSupabaseSQL(sql)
	}

	// Split SQL into individual statements
	statements := m.splitSQLStatements(sql)
	if len(statements) == 0 {
		log.Warnf("[Migrations] No statements found in migration %s", version)
		return nil
	}

	log.Debugf("[Migrations] Migration %s contains %d statements", version, len(statements))

	// Begin transaction for atomicity
	tx := m.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Apply each statement individually
	for i, stmt := range statements {
		// Skip empty statements
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		// Log progress for large migrations
		if len(statements) > 50 && i%50 == 0 {
			log.Debugf("[Migrations] Progress: %d/%d statements executed", i, len(statements))
		}

		// Execute the statement
		if err := tx.Exec(stmt).Error; err != nil {
			if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
				log.Errorf("[Migrations] Failed to rollback transaction: %v", rollbackErr)
			}
			// Include statement preview in error for debugging
			preview := stmt
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			return fmt.Errorf("failed to execute statement %d/%d: %w\nStatement preview: %s",
				i+1, len(statements), err, preview)
		}
	}

	log.Debugf("[Migrations] All %d statements executed successfully", len(statements))

	// Record migration
	migration := SchemaMigration{
		Version:    version,
		ExecutedAt: time.Now().UTC(),
	}
	if err := tx.Create(&migration).Error; err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			log.Errorf("[Migrations] Failed to rollback transaction: %v", rollbackErr)
		}
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// cleanSupabaseSQL removes Supabase-specific SQL commands
func (m *MigrationRunner) cleanSupabaseSQL(sql string) string {
	// List of Supabase-specific extensions that are not available in standard PostgreSQL
	// These extensions are pre-installed in Supabase but will fail on regular PostgreSQL
	supabaseOnlyExtensions := []string{
		"hypopg",          // Hypothetical indexes extension
		"index_advisor",   // Index advisor that depends on hypopg
		"pg_graphql",      // GraphQL extension
		"pg_stat_monitor", // Enhanced monitoring (Supabase specific build)
		"pgaudit",         // Audit logging (may not be available)
		"plv8",            // JavaScript language (rarely available)
		"pgsodium",        // Encryption extension (Supabase specific)
		"vault",           // Secrets management (Supabase specific)
		"wrappers",        // Foreign data wrappers (Supabase specific)
	}

	// Pattern to match GRANT statements for Supabase roles (now handles quotes properly)
	grantPattern := regexp.MustCompile(`(?i)grant\s+[^;]+\s+to\s+["']?(anon|authenticated|service_role)["']?\s*;`)

	// Pattern to match policy creation for Supabase roles
	policyPattern := regexp.MustCompile(`(?i)create\s+policy\s+[^;]+\s+to\s+["']?(anon|authenticated|service_role)["']?\s*;`)

	// Clean the SQL
	cleaned := sql

	// === IDEMPOTENCY TRANSFORMATIONS ===
	// Make DDL statements idempotent to handle re-running migrations on existing databases

	// Make CREATE TABLE idempotent (handles optional schema prefix)
	// Matches: CREATE TABLE [IF NOT EXISTS] ["schema".]"table" or CREATE TABLE [IF NOT EXISTS] schema.table
	createTablePattern := regexp.MustCompile(`(?i)create\s+table\s+(?:if\s+not\s+exists\s+)?`)
	cleaned = createTablePattern.ReplaceAllString(cleaned, "CREATE TABLE IF NOT EXISTS ")

	// Make CREATE INDEX idempotent
	// Handles: CREATE INDEX, CREATE UNIQUE INDEX, CREATE INDEX CONCURRENTLY
	createIndexPattern := regexp.MustCompile(`(?i)create\s+(unique\s+)?index\s+(?:concurrently\s+)?(?:if\s+not\s+exists\s+)?`)
	cleaned = createIndexPattern.ReplaceAllStringFunc(cleaned, func(match string) string {
		hasUnique := regexp.MustCompile(`(?i)unique`).MatchString(match)
		hasConcurrently := regexp.MustCompile(`(?i)concurrently`).MatchString(match)
		result := "CREATE "
		if hasUnique {
			result += "UNIQUE "
		}
		result += "INDEX "
		if hasConcurrently {
			result += "CONCURRENTLY "
		}
		result += "IF NOT EXISTS "
		return result
	})

	// Make CREATE TYPE idempotent (for ENUMs) using DO block pattern
	// PostgreSQL doesn't support CREATE TYPE IF NOT EXISTS directly
	// Use WHEN OTHERS to catch all exception types
	createTypePattern := regexp.MustCompile(`(?i)create\s+type\s+(["']?[^"'\s(]+["']?)\s+as\s+enum\s*\(([^)]+)\)\s*;?`)
	cleaned = createTypePattern.ReplaceAllStringFunc(cleaned, func(match string) string {
		matches := createTypePattern.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		typeName := matches[1]
		enumValues := matches[2]
		return fmt.Sprintf(`DO $$ BEGIN
    CREATE TYPE %s AS ENUM (%s);
EXCEPTION
    WHEN OTHERS THEN null;
END $$`, typeName, enumValues)
	})

	// Make ALTER TABLE ADD CONSTRAINT idempotent
	// Wrap in DO block with exception handling
	// Use WHEN OTHERS to catch all error types (duplicate_object, index already associated, etc.)
	addConstraintPattern := regexp.MustCompile(`(?i)alter\s+table\s+(?:only\s+)?(["']?[^"'\s]+["']?)\s+add\s+constraint\s+(["']?[^"'\s]+["']?)\s+(.+?);`)
	cleaned = addConstraintPattern.ReplaceAllStringFunc(cleaned, func(match string) string {
		matches := addConstraintPattern.FindStringSubmatch(match)
		if len(matches) < 4 {
			return match
		}
		tableName := matches[1]
		constraintName := matches[2]
		constraintDef := strings.TrimSuffix(matches[3], ";")
		return fmt.Sprintf(`DO $$ BEGIN
    ALTER TABLE %s ADD CONSTRAINT %s %s;
EXCEPTION
    WHEN OTHERS THEN null;
END $$`, tableName, constraintName, constraintDef)
	})

	log.Debugf("[Migrations] SQL cleaning: Applied idempotency transformations")

	// === SUPABASE-SPECIFIC CLEANUP ===

	// Remove GRANT statements (handles both quoted and unquoted role names)
	cleaned = grantPattern.ReplaceAllString(cleaned, "")

	// Remove policy statements
	cleaned = policyPattern.ReplaceAllString(cleaned, "")

	// Remove "with schema extensions" clauses
	cleaned = strings.ReplaceAll(cleaned, ` with schema "extensions"`, "")
	cleaned = strings.ReplaceAll(cleaned, ` WITH SCHEMA "extensions"`, "")

	// Process line by line to handle CREATE EXTENSION statements
	lines := strings.Split(cleaned, "\n")
	var processedLines []string
	removedExtensions := []string{}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if this line creates a Supabase-specific extension
		isSupabaseExtension := false
		extensionPattern := regexp.MustCompile(`(?i)create\s+extension\s+(?:if\s+not\s+exists\s+)?["']?(\w+)["']?`)
		if matches := extensionPattern.FindStringSubmatch(trimmedLine); len(matches) > 1 {
			extensionName := strings.ToLower(matches[1])
			for _, supabaseExt := range supabaseOnlyExtensions {
				if extensionName == supabaseExt {
					isSupabaseExtension = true
					removedExtensions = append(removedExtensions, extensionName)
					// Add a comment explaining why this was removed
					processedLines = append(processedLines,
						fmt.Sprintf("-- Skipped Supabase-specific extension: %s (not available in standard PostgreSQL)", extensionName))
					break
				}
			}
		}

		// If it's not a Supabase-specific extension, process normally
		if !isSupabaseExtension {
			// Make other CREATE EXTENSION statements idempotent
			hasIfNotExistsPattern := regexp.MustCompile(`(?i)create\s+extension\s+if\s+not\s+exists\s+`)
			noIfNotExistsPattern := regexp.MustCompile(`(?i)create\s+extension\s+`)

			if hasIfNotExistsPattern.MatchString(line) {
				// Already has IF NOT EXISTS, just normalize to uppercase
				line = hasIfNotExistsPattern.ReplaceAllString(line, "CREATE EXTENSION IF NOT EXISTS ")
			} else if noIfNotExistsPattern.MatchString(line) {
				// Doesn't have IF NOT EXISTS, add it
				line = noIfNotExistsPattern.ReplaceAllString(line, "CREATE EXTENSION IF NOT EXISTS ")
			}

			processedLines = append(processedLines, line)
		}
	}

	// Log removed extensions if any
	if len(removedExtensions) > 0 {
		log.Debugf("[Migrations] Removed %d Supabase-specific extensions: %v",
			len(removedExtensions), removedExtensions)
	}

	cleaned = strings.Join(processedLines, "\n")

	// Remove empty lines created by cleaning (but keep comments)
	lines = strings.Split(cleaned, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" || strings.Contains(line, "--") { // Keep comments and non-empty lines
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	return strings.Join(nonEmptyLines, "\n")
}

// logMigrationStatus logs the current migration status
func (m *MigrationRunner) logMigrationStatus() {
	var migrations []SchemaMigration
	m.db.Order("executed_at DESC").Limit(5).Find(&migrations)

	if len(migrations) > 0 {
		log.Info("[Migrations] Recent migrations:")
		for _, migration := range migrations {
			log.Infof("  - %s (applied at %s)", migration.Version, migration.ExecutedAt.Format("2006-01-02 15:04:05"))
		}
	}

	// Count total migrations
	var count int64
	m.db.Model(&SchemaMigration{}).Count(&count)
	log.Infof("[Migrations] Total migrations applied: %d", count)
}

// GetAppliedMigrations returns a list of all applied migrations

// verifyIndexes checks that expected composite indexes are created
func (m *MigrationRunner) verifyIndexes() error {
	log.Info("[Migrations] Verifying database indexes...")

	// Define expected indexes (table -> index_name -> columns)
	expectedIndexes := map[string]map[string][]string{
		"users": {
			"idx_users_user_id":   {"user_id"},
			"idx_users_user_name": {"username"},
		},
		"chats": {
			"idx_chats_chat_id": {"chat_id"},
		},
		"filters": {
			"idx_filters_chat_keyword": {"chat_id", "keyword"},
		},
		"antiflood_settings": {
			"idx_antiflood_settings_chat_id": {"chat_id"},
		},
		"locks": {
			"idx_lock_chat_type": {"chat_id", "lock_type"},
		},
		"captcha_attempts": {
			"idx_captcha_user_chat":  {"user_id", "chat_id"},
			"idx_captcha_expires_at": {"expires_at"},
		},
	}

	verified := 0
	missing := 0

	for tableName, tableIndexes := range expectedIndexes {
		// Check if table exists first
		var tableExists bool
		err := m.db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = ?
			)
		`, tableName).Scan(&tableExists).Error
		if err != nil {
			log.Warnf("[Migrations] Failed to check if table %s exists: %v", tableName, err)
			continue
		}

		if !tableExists {
			log.Debugf("[Migrations] Table %s does not exist, skipping index verification", tableName)
			continue
		}

		for indexName, expectedColumns := range tableIndexes {
			// Check if index exists and has correct columns
			type IndexInfo struct {
				IndexName       string
				ColumnName      string
				OrdinalPosition int
			}

			var indexColumns []IndexInfo
			err := m.db.Raw(`
				SELECT
					i.relname as index_name,
					a.attname as column_name,
					array_position(ix.indkey, a.attnum) as ordinal_position
				FROM
					pg_class t,
					pg_class i,
					pg_index ix,
					pg_attribute a
				WHERE
					t.oid = ix.indrelid
					and i.oid = ix.indexrelid
					and a.attrelid = t.oid
					and a.attnum = ANY(ix.indkey)
					and t.relkind = 'r'
					and t.relname = ?
					and i.relname = ?
				ORDER BY array_position(ix.indkey, a.attnum)
			`, tableName, indexName).Find(&indexColumns).Error

			if err != nil {
				log.Warnf("[Migrations] Failed to check index %s on table %s: %v", indexName, tableName, err)
				continue
			}

			if len(indexColumns) == 0 {
				log.Warnf("[Migrations] Missing index: %s on table %s (expected columns: %v)", indexName, tableName, expectedColumns)
				missing++
				continue
			}

			// Verify columns match
			actualColumns := make([]string, len(indexColumns))
			for i, col := range indexColumns {
				actualColumns[i] = col.ColumnName
			}

			// Compare expected vs actual columns
			if len(expectedColumns) != len(actualColumns) {
				log.Warnf("[Migrations] Index %s on table %s has wrong number of columns: expected %v, got %v",
					indexName, tableName, expectedColumns, actualColumns)
				missing++
				continue
			}

			mismatch := false
			for i, expected := range expectedColumns {
				if actualColumns[i] != expected {
					log.Warnf("[Migrations] Index %s on table %s has wrong column at position %d: expected %s, got %s",
						indexName, tableName, i+1, expected, actualColumns[i])
					mismatch = true
				}
			}

			if mismatch {
				missing++
			} else {
				log.Debugf("[Migrations] Verified index: %s on table %s", indexName, tableName)
				verified++
			}
		}
	}

	log.Infof("[Migrations] Index verification complete - Verified: %d, Missing/Incorrect: %d", verified, missing)

	if missing > 0 {
		return fmt.Errorf("%d indexes are missing or incorrect", missing)
	}

	return nil
}
