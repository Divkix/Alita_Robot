package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

type MigrationRunner struct {
	db             *gorm.DB
	migrationsPath string
}

type SchemaMigration struct {
	Version    string    `gorm:"primaryKey;column:version"`
	ExecutedAt time.Time `gorm:"column:executed_at"`
	Checksum   string    `gorm:"column:checksum"`
}

func (SchemaMigration) TableName() string {
	return "schema_migrations"
}

func NewMigrationRunner(db *gorm.DB) *MigrationRunner {
	return &MigrationRunner{
		db:             db,
		migrationsPath: config.AppConfig.MigrationsPath,
	}
}

func (m *MigrationRunner) RunMigrations() error {
	log.Info("[Migrations] Starting automatic database migration...")

	if err := m.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	files, err := m.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	if len(files) == 0 {
		log.Info("[Migrations] No migration files found")
		return nil
	}

	log.Infof("[Migrations] Found %d migration files", len(files))

	var appliedCount int64
	if err := m.db.Model(&SchemaMigration{}).Count(&appliedCount).Error; err != nil {
		return fmt.Errorf("failed to count applied migrations: %w", err)
	}
	if appliedCount == int64(len(files)) {
		log.Infof("[Migrations] All %d migrations already applied; nothing to do", len(files))
		m.logMigrationStatus()
		return nil
	}

	applied := 0
	skipped := 0

	for _, file := range files {
		version := filepath.Base(file)

		// Read the raw file content once so we can checksum it regardless of
		// whether the migration is pending or already applied.
		content, err := os.ReadFile(file) // #nosec G304 - path comes from getMigrationFiles which validates the directory
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", version, err)
		}

		isApplied, err := m.isMigrationApplied(version)
		if err != nil {
			return fmt.Errorf("failed to check migration %s status: %w", version, err)
		}
		if isApplied {
			// Verify the content hasn't changed since it was applied.
			if err := m.verifyMigrationChecksum(version, content); err != nil {
				return err
			}
			log.Debugf("[Migrations] Skipping %s (already applied)", version)
			skipped++
			continue
		}

		log.Infof("[Migrations] Applying %s...", version)
		if err := m.applyMigration(file, version); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", version, err)
		}
		applied++
		log.Infof("[Migrations] Successfully applied %s", version)
	}

	log.Infof("[Migrations] Migration complete - Applied: %d, Skipped: %d",
		applied, skipped)

	m.logMigrationStatus()

	if applied > 0 {
		if err := m.verifyIndexes(); err != nil {
			log.Warnf("[Migrations] Index verification failed: %v", err)
		}
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist,
// and idempotently adds the checksum column so existing deployments are upgraded
// without re-running migrations.
func (m *MigrationRunner) ensureMigrationsTable() error {
	createSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	if err := m.db.Exec(createSQL).Error; err != nil {
		return err
	}
	// Add the checksum column to existing tables.  IF NOT EXISTS prevents
	// errors on fresh tables that were just created above.
	alterSQL := `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum VARCHAR(64)`
	return m.db.Exec(alterSQL).Error
}

func (m *MigrationRunner) getMigrationFiles() ([]string, error) {
	if _, err := os.Stat(m.migrationsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations path does not exist: %s", m.migrationsPath)
	}

	pattern := filepath.Join(m.migrationsPath, "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	files = slices.DeleteFunc(files, func(file string) bool {
		return strings.HasSuffix(filepath.Base(file), ".rollback.sql")
	})

	slices.Sort(files)
	return files, nil
}

func (m *MigrationRunner) isMigrationApplied(version string) (bool, error) {
	var count int64
	if err := m.db.Model(&SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *MigrationRunner) getMigrationRecord(version string) (*SchemaMigration, error) {
	var rec SchemaMigration
	if err := m.db.Where("version = ?", version).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// verifyMigrationChecksum compares the stored checksum for an already-applied
// migration against the SHA-256 of the current file content.
// When the stored checksum is empty (legacy row), the current checksum is
// written back as a trusted baseline instead of raising an error — this avoids
// false alarms on migrations that pre-date this feature.
// When AutoMigrateSilentFail is false a mismatch returns an error; otherwise
// it only logs a warning so that existing deployments are not blocked.
func (m *MigrationRunner) verifyMigrationChecksum(version string, content []byte) error {
	rec, err := m.getMigrationRecord(version)
	if err != nil {
		return fmt.Errorf("failed to read migration record %s: %w", version, err)
	}
	if rec == nil {
		return fmt.Errorf("migration record %s not found", version)
	}

	sum := sha256.Sum256(content)
	current := hex.EncodeToString(sum[:])

	if rec.Checksum == "" {
		// Legacy row with no checksum — backfill and treat as trusted.
		if err := m.db.Model(&SchemaMigration{}).
			Where("version = ?", version).
			Update("checksum", current).Error; err != nil {
			return fmt.Errorf("failed to backfill checksum for %s: %w", version, err)
		}
		log.Debugf("[Migrations] Backfilled checksum for legacy migration %s", version)
		return nil
	}

	if rec.Checksum == current {
		return nil // content unchanged
	}

	log.Warnf("[Migrations] Checksum mismatch for %s: file content changed since it was applied", version)
	if !config.AppConfig.AutoMigrateSilentFail {
		return fmt.Errorf("migration %s has been modified after it was applied (checksum mismatch); "+
			"migrations are immutable once applied — create a new migration file instead", version)
	}
	return nil
}

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

		if !inSingleQuote && !inDoubleQuote && !inLineComment && !inBlockComment {
			if char == '$' {
				tagEnd := i + 1
				for tagEnd < length && (runes[tagEnd] != '$' && runes[tagEnd] != ' ' && runes[tagEnd] != '\n' && runes[tagEnd] != ';') {
					tagEnd++
				}

				if tagEnd < length && runes[tagEnd] == '$' {
					tag := string(runes[i : tagEnd+1])
					if inDollarQuote {
						if tag == dollarQuoteTag {
							inDollarQuote = false
							dollarQuoteTag = ""
						}
					} else {
						inDollarQuote = true
						dollarQuoteTag = tag
					}

					for j := i; j <= tagEnd; j++ {
						currentStmt.WriteRune(runes[j])
					}
					i = tagEnd
					continue
				}
			}
		}

		if !inDoubleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			if char == '\'' {
				if i+1 < length && runes[i+1] == '\'' {
					currentStmt.WriteRune(char)
					currentStmt.WriteRune(runes[i+1])
					i++
					continue
				}
				inSingleQuote = !inSingleQuote
			}
		}

		if !inSingleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			if char == '"' {
				if i+1 < length && runes[i+1] == '"' {
					currentStmt.WriteRune(char)
					currentStmt.WriteRune(runes[i+1])
					i++
					continue
				}
				inDoubleQuote = !inDoubleQuote
			}
		}

		if char == ';' && !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			stmt := strings.TrimSpace(currentStmt.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStmt.Reset()
		} else {
			currentStmt.WriteRune(char)
		}
	}

	if currentStmt.Len() > 0 {
		stmt := strings.TrimSpace(currentStmt.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}

func isTransactionControlStatement(stmt string) bool {
	stmt = strings.TrimSpace(stmt)
	for {
		switch {
		case strings.HasPrefix(stmt, "--"):
			if newline := strings.IndexByte(stmt, '\n'); newline >= 0 {
				stmt = strings.TrimSpace(stmt[newline+1:])
				continue
			}
			return false
		case strings.HasPrefix(stmt, "/*"):
			if end := strings.Index(stmt[2:], "*/"); end >= 0 {
				stmt = strings.TrimSpace(stmt[end+4:])
				continue
			}
			return false
		}
		break
	}

	switch strings.ToUpper(strings.Join(strings.Fields(stmt), " ")) {
	case "BEGIN", "BEGIN WORK", "BEGIN TRANSACTION",
		"START TRANSACTION",
		"COMMIT", "COMMIT WORK", "COMMIT TRANSACTION",
		"END", "END WORK", "END TRANSACTION":
		return true
	default:
		return false
	}
}

func (m *MigrationRunner) applyMigration(filepath, version string) error {
	if !strings.HasPrefix(filepath, m.migrationsPath) {
		return fmt.Errorf("invalid migration file path: %s", filepath)
	}

	cleanPath := path.Clean(filepath)
	if cleanPath != filepath || strings.Contains(filepath, "..") {
		return fmt.Errorf("potentially unsafe migration file path: %s", filepath)
	}

	content, err := os.ReadFile(filepath) // #nosec G304 - path validation performed above
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	sql := cleanSupabaseSQL(string(content))

	statements := m.splitSQLStatements(sql)
	statements = slices.DeleteFunc(statements, isTransactionControlStatement)
	if len(statements) == 0 {
		log.Warnf("[Migrations] No statements found in migration %s", version)
		return nil
	}

	log.Debugf("[Migrations] Migration %s contains %d statements", version, len(statements))

	tx := m.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	for i, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		if len(statements) > 50 && i%50 == 0 {
			log.Debugf("[Migrations] Progress: %d/%d statements executed", i, len(statements))
		}

		if err := tx.Exec(stmt).Error; err != nil {
			if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
				log.Errorf("[Migrations] Failed to rollback transaction: %v", rollbackErr)
			}
			preview := stmt
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			return fmt.Errorf("failed to execute statement %d/%d: %w\nStatement preview: %s",
				i+1, len(statements), err, preview)
		}
	}

	log.Debugf("[Migrations] All %d statements executed successfully", len(statements))

	// Compute checksum over raw file bytes (before any Supabase cleaning) so
	// the stored value is stable and matches a simple sha256sum of the source file.
	sum := sha256.Sum256(content)
	checksum := hex.EncodeToString(sum[:])

	migration := SchemaMigration{
		Version:    version,
		ExecutedAt: time.Now().UTC(),
		Checksum:   checksum,
	}
	if err := tx.Create(&migration).Error; err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			log.Errorf("[Migrations] Failed to rollback transaction: %v", rollbackErr)
		}
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// cleanSupabaseSQL removes Supabase-specific SQL commands
func cleanSupabaseSQL(sql string) string {
	// List of Supabase-specific extensions that are not available in standard PostgreSQL
	// These extensions are pre-installed in Supabase but will fail on regular PostgreSQL
	supabaseOnlyExtensions := []string{
		"hypopg",
		"index_advisor",
		"pg_graphql",
		"pg_stat_monitor",
		"pgaudit",
		"plv8",
		"pgsodium",
		"vault",
		"wrappers",
	}

	grantPattern := regexp.MustCompile(`(?i)grant\s+[^;]+\s+to\s+[\"']?(anon|authenticated|service_role)[\"']?\s*;`)

	policyPattern := regexp.MustCompile(`(?i)create\s+policy\s+[^;]+\s+to\s+[\"']?(anon|authenticated|service_role)[\"']?\s*;`)

	cleaned := sql

	createTablePattern := regexp.MustCompile(`(?i)create\s+table\s+(?:if\s+not\s+exists\s+)?`)
	cleaned = createTablePattern.ReplaceAllString(cleaned, "CREATE TABLE IF NOT EXISTS ")

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
	// Ignore only the duplicate-object case; invalid enum definitions must fail.
	createTypePattern := regexp.MustCompile(`(?i)create\s+type\s+([\"']?[^\"'\s(]+[\"']?)\s+as\s+enum\s*\(([^)]+)\)\s*;?`)
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
    WHEN duplicate_object THEN NULL;
END $$;`, typeName, enumValues)
	})

	// Make ALTER TABLE ADD CONSTRAINT idempotent — wrap in a DO block that only
	// ignores a constraint which already exists. Every other error must abort the
	// migration so a broken schema is never recorded as applied.
	// Skips ALTER TABLE statements already inside dollar-quoted blocks
	// (e.g., hand-written DO $$ blocks in migration files) to avoid
	// creating nested DO blocks that PostgreSQL cannot parse.
	addConstraintPattern := regexp.MustCompile(`(?is)alter\s+table\s+(?:only\s+)?([\"']?[^\"'\s]+[\"']?)\s+add\s+constraint\s+([\"']?[^\"'\s]+[\"']?)\s+(.+?);`)

	// Find dollar-quote block boundaries so we can skip matches inside them.
	dollarBlocks := findDollarQuoteBlocks(cleaned)

	allMatches := addConstraintPattern.FindAllStringSubmatchIndex(cleaned, -1)
	if len(allMatches) > 0 {
		var result strings.Builder
		lastEnd := 0
		for _, matchIdx := range allMatches {
			matchStart := matchIdx[0]
			matchEnd := matchIdx[1]

			insideBlock := false
			for _, block := range dollarBlocks {
				if matchStart >= block.start && matchEnd <= block.end {
					insideBlock = true
					break
				}
			}

			result.WriteString(cleaned[lastEnd:matchStart])

			if insideBlock {
				result.WriteString(cleaned[matchStart:matchEnd])
			} else {
				submatches := addConstraintPattern.FindStringSubmatch(cleaned[matchStart:matchEnd])
				if len(submatches) >= 4 {
					tableName := submatches[1]
					constraintName := submatches[2]
					constraintDef := strings.TrimSuffix(submatches[3], ";")
					fmt.Fprintf(&result, `DO $$ BEGIN
    ALTER TABLE %s ADD CONSTRAINT %s %s;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`, tableName, constraintName, constraintDef)
				} else {
					result.WriteString(cleaned[matchStart:matchEnd])
				}
			}
			lastEnd = matchEnd
		}
		result.WriteString(cleaned[lastEnd:])
		cleaned = result.String()
	}

	// One legacy migration dynamically drops every index before dropping its
	// table, including the constraint-owned primary-key index. PostgreSQL requires
	// the owning constraint/table to remove that index, so leave it for the
	// following DROP TABLE while still propagating every unrelated error.
	dynamicDropIndexPattern := regexp.MustCompile(`(?i)execute\s+'drop\s+index\s+if\s+exists\s+'\s*\|\|\s*([^;]+);`)
	cleaned = dynamicDropIndexPattern.ReplaceAllString(cleaned, `BEGIN
    EXECUTE 'DROP INDEX IF EXISTS ' || $1;
EXCEPTION
    WHEN dependent_objects_still_exist THEN NULL;
END;`)

	log.Debugf("[Migrations] SQL cleaning: Applied idempotency transformations")

	cleaned = grantPattern.ReplaceAllString(cleaned, "")

	cleaned = policyPattern.ReplaceAllString(cleaned, "")

	cleaned = strings.ReplaceAll(cleaned, ` with schema "extensions"`, "")
	cleaned = strings.ReplaceAll(cleaned, ` WITH SCHEMA "extensions"`, "")

	lines := strings.Split(cleaned, "\n")
	var processedLines []string
	removedExtensions := []string{}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		isSupabaseExtension := false
		extensionPattern := regexp.MustCompile(`(?i)create\s+extension\s+(?:if\s+not\s+exists\s+)?[\"']?(\w+)[\"']?`)
		if matches := extensionPattern.FindStringSubmatch(trimmedLine); len(matches) > 1 {
			extensionName := strings.ToLower(matches[1])
			for _, supabaseExt := range supabaseOnlyExtensions {
				if extensionName == supabaseExt {
					isSupabaseExtension = true
					removedExtensions = append(removedExtensions, extensionName)
					processedLines = append(processedLines,
						fmt.Sprintf("-- Skipped Supabase-specific extension: %s (not available in standard PostgreSQL)", extensionName))
					break
				}
			}
		}

		if !isSupabaseExtension {
			hasIfNotExistsPattern := regexp.MustCompile(`(?i)create\s+extension\s+if\s+not\s+exists\s+`)
			noIfNotExistsPattern := regexp.MustCompile(`(?i)create\s+extension\s+`)

			if hasIfNotExistsPattern.MatchString(line) {
				line = hasIfNotExistsPattern.ReplaceAllString(line, "CREATE EXTENSION IF NOT EXISTS ")
			} else if noIfNotExistsPattern.MatchString(line) {
				line = noIfNotExistsPattern.ReplaceAllString(line, "CREATE EXTENSION IF NOT EXISTS ")
			}

			processedLines = append(processedLines, line)
		}
	}

	if len(removedExtensions) > 0 {
		log.Debugf("[Migrations] Removed %d Supabase-specific extensions: %v",
			len(removedExtensions), removedExtensions)
	}

	cleaned = strings.Join(processedLines, "\n")

	lines = strings.Split(cleaned, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" || strings.Contains(line, "--") {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	return strings.Join(nonEmptyLines, "\n")
}

type dqBlock struct {
	start int // byte offset of opening $tag$
	end   int // byte offset of closing $tag$ (inclusive of closing $)
}

// findDollarQuoteBlocks finds all dollar-quoted block boundaries in the given SQL.
// It handles single quotes, double quotes, line comments, and block comments to
// avoid false positives inside string literals or comments.
//
//nolint:gocyclo // State machine parser with many states - complexity is inherent
func findDollarQuoteBlocks(sql string) []dqBlock {
	var blocks []dqBlock
	runes := []rune(sql)
	length := len(runes)

	inSingleQuote := false
	inDoubleQuote := false
	inDollarQuote := false
	inLineComment := false
	inBlockComment := false
	dollarQuoteTag := ""
	dollarQuoteStart := -1

	for i := 0; i < length; i++ {
		char := runes[i]
		nextChar := rune(0)
		if i+1 < length {
			nextChar = runes[i+1]
		}

		if !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inBlockComment {
			if char == '-' && nextChar == '-' {
				inLineComment = true
				continue
			}
		}

		if inLineComment {
			if char == '\n' {
				inLineComment = false
			}
			continue
		}

		if !inSingleQuote && !inDoubleQuote && !inDollarQuote && !inLineComment {
			if char == '/' && nextChar == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		if inBlockComment {
			if char == '*' && nextChar == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if !inSingleQuote && !inDoubleQuote && !inLineComment && !inBlockComment {
			if char == '$' {
				tagEnd := i + 1
				for tagEnd < length && runes[tagEnd] != '$' && runes[tagEnd] != ' ' && runes[tagEnd] != '\n' && runes[tagEnd] != ';' {
					tagEnd++
				}

				if tagEnd < length && runes[tagEnd] == '$' {
					tag := string(runes[i : tagEnd+1])
					if inDollarQuote {
						if tag == dollarQuoteTag {
							blocks = append(blocks, dqBlock{
								start: dollarQuoteStart,
								end:   tagEnd,
							})
							inDollarQuote = false
							dollarQuoteTag = ""
						}
					} else {
						inDollarQuote = true
						dollarQuoteTag = tag
						dollarQuoteStart = i
					}
					i = tagEnd
					continue
				}
			}
		}

		if !inDoubleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			if char == '\'' {
				if i+1 < length && runes[i+1] == '\'' {
					i++
				} else {
					inSingleQuote = !inSingleQuote
				}
			}
		}

		if !inSingleQuote && !inDollarQuote && !inLineComment && !inBlockComment {
			if char == '"' {
				if i+1 < length && runes[i+1] == '"' {
					i++
				} else {
					inDoubleQuote = !inDoubleQuote
				}
			}
		}
	}

	// Convert rune offsets to byte offsets for compatibility with FindAllStringSubmatchIndex
	for i := range blocks {
		blocks[i].start = len(string(runes[:blocks[i].start]))
		blocks[i].end = len(string(runes[:blocks[i].end]))
	}

	return blocks
}

func (m *MigrationRunner) logMigrationStatus() {
	var migrations []SchemaMigration
	m.db.Order("executed_at DESC").Limit(5).Find(&migrations)

	if len(migrations) > 0 {
		log.Info("[Migrations] Recent migrations:")
		for _, migration := range migrations {
			log.Infof("  - %s (applied at %s)", migration.Version, migration.ExecutedAt.Format("2006-01-02 15:04:05"))
		}
	}

	var count int64
	m.db.Model(&SchemaMigration{}).Count(&count)
	log.Infof("[Migrations] Total migrations applied: %d", count)
}

func (m *MigrationRunner) verifyIndexes() error {
	log.Info("[Migrations] Verifying database indexes...")

	// Note: Some indexes were intentionally dropped as unused - see migration history:
	// - idx_users_user_id → replaced by uk_users_user_id (unique constraint)
	// - idx_chats_chat_id → replaced by uk_chats_chat_id (unique constraint)
	// - idx_filters_chat_keyword → dropped (app loads all filters per chat)
	// - idx_lock_chat_type → dropped (app loads all locks per chat)
	// - idx_captcha_expires_at → dropped (0 scans in production)
	expectedIndexes := map[string]map[string][]string{
		"users": {
			"uk_users_user_id":    {"user_id"}, // Unique constraint (replaces idx_users_user_id)
			"idx_users_user_name": {"username"},
		},
		"chats": {
			"uk_chats_chat_id": {"chat_id"}, // Unique constraint (replaces idx_chats_chat_id)
		},
		"antiflood_settings": {
			"idx_antiflood_settings_chat_id": {"chat_id"},
		},
		"captcha_attempts": {
			"uk_captcha_user_chat": {"user_id", "chat_id"},
		},
		"approved_users": {
			"idx_approved_users_chat_id": {"chat_id"}, // Restored by 20260831230000; dropped once as "unused" and missed
		},
	}

	verified := 0
	missing := 0

	for tableName, tableIndexes := range expectedIndexes {
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

			actualColumns := make([]string, len(indexColumns))
			for i, col := range indexColumns {
				actualColumns[i] = col.ColumnName
			}

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
