package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	_ "embed"
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

// The advisory lock key must match scripts/migrate_psql.sh exactly so the bot
// and the standalone script serialize against each other.
const (
	migrationLockSQL   = "SELECT pg_try_advisory_lock(hashtextextended('alita:schema_migrations', 0))"
	migrationUnlockSQL = "SELECT pg_advisory_unlock(hashtextextended('alita:schema_migrations', 0))"

	migrationLockWait      = 10 * time.Minute
	migrationLockRetryMin  = 200 * time.Millisecond
	migrationLockRetryMax  = 5 * time.Second
	migrationUnlockTimeout = 5 * time.Second
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

	// Postgres needs a session-scoped advisory lock. Pin one connection for the
	// whole critical section: a pooled Exec can check out any connection, and a
	// deferred unlock could then run on a different session, report success with
	// no error while leaving the lock held for the pooled connection's lifetime.
	if m.db == nil || m.db.Name() != "postgres" {
		return m.runMigrationsLocked()
	}

	// The lock must stay pinned to one session so the deferred unlock runs on
	// the session that holds it, but the migration body keeps using the pool.
	// With MaxOpenConns(1) the pinned session is the pool's only slot, so the
	// body's first query would wait forever for a connection the lock never
	// releases. Add one slot for the critical section; 0 (unlimited) and >1
	// already have room. The restore defer is registered first so it runs after
	// the lock connection is unlocked and closed.
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database handle: %w", err)
	}
	if sqlDB.Stats().MaxOpenConnections == 1 {
		sqlDB.SetMaxOpenConns(2)
		defer sqlDB.SetMaxOpenConns(1)
	}

	conn, err := m.acquireMigrationLock()
	if err != nil {
		return err
	}
	// Deferred calls run last-in-first-out, so Close is registered before the
	// unlock and therefore runs after it: the lock is released on the same
	// session before the connection can return to the pool.
	defer func() {
		// ErrConnDone means releaseMigrationLock already discarded the connection.
		if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			log.Warnf("[Migrations] Failed to close migration lock connection: %v", err)
		}
	}()
	defer m.releaseMigrationLock(conn)

	return m.runMigrationsLocked()
}

// acquireMigrationLock checks out a dedicated connection and polls for the
// migration advisory lock until it is acquired or the wait budget expires.
// A non-blocking try is used so a stuck holder surfaces as a timeout instead
// of wedging the process in pg_advisory_lock.
func (m *MigrationRunner) acquireMigrationLock() (*sql.Conn, error) {
	sqlDB, err := m.db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database handle: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), migrationLockWait)
	defer cancel()

	backoff := migrationLockRetryMin
	for {
		conn, err := sqlDB.Conn(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("timed out waiting %s for the migration lock: %w", migrationLockWait, ctx.Err())
			}
			return nil, fmt.Errorf("failed to check out migration lock connection: %w", err)
		}

		var locked bool
		if err := conn.QueryRowContext(ctx, migrationLockSQL).Scan(&locked); err != nil {
			// The server may have granted the lock before the response was lost,
			// so this session must not go back to the pool either.
			discardMigrationLockConn(conn)
			return nil, fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		if locked {
			return conn, nil
		}
		_ = conn.Close()

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting %s for the migration lock: %w", migrationLockWait, ctx.Err())
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, migrationLockRetryMax)
	}
}

// releaseMigrationLock unlocks the migration advisory lock on the pinned
// session. If the unlock errors or reports that this session held no lock, the
// connection is marked bad so it cannot re-enter the pool still holding the
// lock; PostgreSQL then releases the lock when the session closes.
func (m *MigrationRunner) releaseMigrationLock(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), migrationUnlockTimeout)
	defer cancel()

	var unlocked bool
	err := conn.QueryRowContext(ctx, migrationUnlockSQL).Scan(&unlocked)
	if err == nil && unlocked {
		return
	}
	if err != nil {
		log.Warnf("[Migrations] Failed to release migration lock: %v", err)
	} else {
		log.Warn("[Migrations] Migration lock was not held by the releasing session")
	}

	// A connection that may still hold the lock must not return to the pool;
	// ending its session makes PostgreSQL release the lock.
	discardMigrationLockConn(conn)
}

// discardMigrationLockConn discards conn instead of returning it to the pool:
// returning driver.ErrBadConn from Conn.Raw makes database/sql close the
// session, which releases any advisory lock the session still holds.
func discardMigrationLockConn(conn *sql.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
}

// runMigrationsLocked performs the migration work. Callers on Postgres must
// hold the migration advisory lock; other drivers run it unconditionally.
func (m *MigrationRunner) runMigrationsLocked() error {
	if err := m.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return m.runMigrationFiles()
}

// runMigrationFiles verifies the already-applied migration files and applies the
// pending ones. The schema_migrations table must already exist.
func (m *MigrationRunner) runMigrationFiles() error {
	files, err := m.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}
	if len(files) == 0 {
		log.Info("[Migrations] No migration files found")
		return nil
	}

	log.Infof("[Migrations] Found %d migration files", len(files))

	records, allApplied, orphans, err := m.reconcileAppliedVersions(files)
	if err != nil {
		return fmt.Errorf("failed to compare applied migrations with files: %w", err)
	}
	if len(orphans) > 0 {
		log.Warnf("[Migrations] %d recorded migrations have no file (deleted or renamed after being applied): %s",
			len(orphans), strings.Join(orphans, ", "))
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

		if rec, isApplied := records[version]; isApplied {
			// Verify the content hasn't changed since it was applied.
			if err := m.verifyMigrationChecksum(rec, content); err != nil {
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

	if allApplied {
		log.Infof("[Migrations] All %d migrations already applied; nothing to do", len(files))
	} else {
		log.Infof("[Migrations] Migration complete - Applied: %d, Skipped: %d",
			applied, skipped)
	}

	m.logMigrationStatus()

	// Verify on every run, not just when something was applied: migration files
	// are immutable, so an index that changes state in a later migration is
	// invisible here. "Nothing new applied" is exactly when drift shows up.
	if err := m.verifyIndexes(); err != nil {
		log.Warnf("[Migrations] Index verification failed: %v", err)
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

// reconcileAppliedVersions reads every schema_migrations row once and compares
// the recorded versions with the given files. records maps each recorded version
// to its row; allApplied is true when every file has a record; orphans are the
// sorted records that have no file. Comparing row counts alone is not enough: a
// record whose file was deleted or renamed after it was applied offsets a new
// unapplied file and the count matches, so the new migration would be skipped
// silently.
func (m *MigrationRunner) reconcileAppliedVersions(files []string) (records map[string]SchemaMigration, allApplied bool, orphans []string, err error) {
	var rows []SchemaMigration
	if err := m.db.Find(&rows).Error; err != nil {
		return nil, false, nil, err
	}

	records = make(map[string]SchemaMigration, len(rows))
	for _, row := range rows {
		records[row.Version] = row
	}

	allApplied = true
	onDisk := make(map[string]struct{}, len(files))
	for _, file := range files {
		version := filepath.Base(file)
		onDisk[version] = struct{}{}
		if _, ok := records[version]; !ok {
			allApplied = false
		}
	}
	for version := range records {
		if _, ok := onDisk[version]; !ok {
			orphans = append(orphans, version)
		}
	}
	slices.Sort(orphans)
	return records, allApplied, orphans, nil
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

// verifyMigrationChecksum compares the stored checksum of an already-applied
// migration record against the SHA-256 of the current file content.
// When the stored checksum is empty (legacy row), the current checksum is
// written back as a trusted baseline instead of raising an error — this avoids
// false alarms on migrations that pre-date this feature.
// When AutoMigrateSilentFail is false a mismatch returns an error; otherwise
// it only logs a warning so that existing deployments are not blocked.
func (m *MigrationRunner) verifyMigrationChecksum(rec SchemaMigration, content []byte) error {
	version := rec.Version
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

// expectedIndexesFile is both the embedded golden and the file the chain test
// rewrites when UPDATE_INDEX_GOLDEN=1.
const expectedIndexesFile = "expected_indexes.txt"

// expectedIndexesGolden records every index on a public user table as
// "table|index|pg_get_indexdef", one sorted line per index. Generate it from a
// freshly migrated PostgreSQL database with:
//
//	ALITA_TEST_MIGRATION_CHAIN=true DATABASE_URL=... UPDATE_INDEX_GOLDEN=1 \
//	  go test ./alita/db/migrations -run TestRepositoryMigrationChain
//
//go:embed expected_indexes.txt
var expectedIndexesGolden string

// goldenIndex is one expected_indexes.txt row. The definition is stored whole
// so a redefined index (column order, predicate, method) is caught, not just a
// missing one.
type goldenIndex struct {
	Table string `gorm:"column:table_name"`
	Name  string `gorm:"column:index_name"`
	Def   string `gorm:"column:index_def"`
}

type indexKey struct {
	table string
	name  string
}

func (g goldenIndex) key() indexKey {
	return indexKey{table: g.Table, name: g.Name}
}

// indexCatalogSQL lists every index on a public user table, including indexes
// owned by primary keys and unique constraints (they back foreign keys too).
const indexCatalogSQL = `
SELECT c.relname AS table_name,
       i.relname AS index_name,
       pg_get_indexdef(x.indexrelid) AS index_def
FROM pg_index x
JOIN pg_class c ON c.oid = x.indrelid
JOIN pg_class i ON i.oid = x.indexrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind IN ('r', 'p')
ORDER BY c.relname, i.relname
`

func queryIndexCatalog(db *gorm.DB) ([]goldenIndex, error) {
	var indexes []goldenIndex
	if err := db.Raw(indexCatalogSQL).Scan(&indexes).Error; err != nil {
		return nil, err
	}
	return indexes, nil
}

func parseGoldenIndexes(data string) ([]goldenIndex, error) {
	var indexes []goldenIndex
	seen := make(map[indexKey]bool)
	for lineNo, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("%s:%d: want table|index|indexdef, got %q", expectedIndexesFile, lineNo+1, line)
		}

		index := goldenIndex{Table: parts[0], Name: parts[1], Def: parts[2]}
		if seen[index.key()] {
			return nil, fmt.Errorf("%s:%d: duplicate index %s on table %s", expectedIndexesFile, lineNo+1, index.Name, index.Table)
		}
		seen[index.key()] = true
		indexes = append(indexes, index)
	}
	return indexes, nil
}

func formatGoldenIndexes(indexes []goldenIndex) string {
	sorted := slices.Clone(indexes)
	slices.SortFunc(sorted, func(a, b goldenIndex) int {
		if n := strings.Compare(a.Table, b.Table); n != 0 {
			return n
		}
		return strings.Compare(a.Name, b.Name)
	})

	var builder strings.Builder
	for _, index := range sorted {
		builder.WriteString(index.Table)
		builder.WriteByte('|')
		builder.WriteString(index.Name)
		builder.WriteByte('|')
		builder.WriteString(index.Def)
		builder.WriteByte('\n')
	}
	return builder.String()
}

type indexDiffKind int

const (
	indexMissing indexDiffKind = iota
	indexChanged
	indexUnexpected
)

type indexDiff struct {
	Kind     indexDiffKind
	Table    string
	Name     string
	Expected string
	Actual   string
}

func (d indexDiff) String() string {
	switch d.Kind {
	case indexMissing:
		return fmt.Sprintf("missing index %s on table %s (expected %s)", d.Name, d.Table, d.Expected)
	case indexChanged:
		return fmt.Sprintf("index %s on table %s changed: expected %s, got %s", d.Name, d.Table, d.Expected, d.Actual)
	default:
		return fmt.Sprintf("unexpected index %s on table %s (%s); record it in %s or drop it", d.Name, d.Table, d.Actual, expectedIndexesFile)
	}
}

// diffIndexes compares in both directions: a golden index that is missing or
// was redefined, and a live index the golden does not know about.
func diffIndexes(expected, actual []goldenIndex) []indexDiff {
	actualByKey := make(map[indexKey]goldenIndex, len(actual))
	for _, index := range actual {
		actualByKey[index.key()] = index
	}

	var diffs []indexDiff
	expectedKeys := make(map[indexKey]bool, len(expected))
	for _, index := range expected {
		expectedKeys[index.key()] = true
		got, ok := actualByKey[index.key()]
		switch {
		case !ok:
			diffs = append(diffs, indexDiff{Kind: indexMissing, Table: index.Table, Name: index.Name, Expected: index.Def})
		case got.Def != index.Def:
			diffs = append(diffs, indexDiff{Kind: indexChanged, Table: index.Table, Name: index.Name, Expected: index.Def, Actual: got.Def})
		}
	}

	for _, index := range actual {
		if !expectedKeys[index.key()] {
			diffs = append(diffs, indexDiff{Kind: indexUnexpected, Table: index.Table, Name: index.Name, Actual: index.Def})
		}
	}
	return diffs
}

// verifyIndexes compares the live catalog with expected_indexes.txt in both
// directions. Runtime stays warn-only; the repository chain test fails on any
// diff, including an index that was silently reintroduced.
func (m *MigrationRunner) verifyIndexes() error {
	if m.db == nil || m.db.Name() != "postgres" {
		log.Debugf("[Migrations] Skipping index verification: not a PostgreSQL database")
		return nil
	}

	log.Info("[Migrations] Verifying database indexes...")

	expected, err := parseGoldenIndexes(expectedIndexesGolden)
	if err != nil {
		return fmt.Errorf("parse embedded %s: %w", expectedIndexesFile, err)
	}
	actual, err := queryIndexCatalog(m.db)
	if err != nil {
		return fmt.Errorf("query index catalog: %w", err)
	}

	diffs := diffIndexes(expected, actual)
	missing := 0
	for _, diff := range diffs {
		if diff.Kind != indexUnexpected {
			missing++
		}
		log.Warnf("[Migrations] Index verification: %s", diff)
	}

	log.Infof("[Migrations] Index verification complete - Verified: %d, Missing/Incorrect: %d, Unexpected: %d",
		len(expected)-missing, missing, len(diffs)-missing)

	if len(diffs) > 0 {
		return fmt.Errorf("%d indexes differ from %s (%d missing or incorrect, %d unexpected)",
			len(diffs), expectedIndexesFile, missing, len(diffs)-missing)
	}
	return nil
}

// foreignKeyCoverageSQL lists foreign keys that no valid, non-partial index
// leads with. int2vector is zero-based, so [0:n-1] is the first n index
// columns; the containment test accepts any column order among them.
const foreignKeyCoverageSQL = `
SELECT c.relname AS table_name,
       con.conname AS constraint_name
FROM pg_constraint con
JOIN pg_class c ON c.oid = con.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE con.contype = 'f'
  AND n.nspname = 'public'
  AND NOT EXISTS (
      SELECT 1
      FROM pg_index i
      WHERE i.indrelid = con.conrelid
        AND i.indisvalid
        AND i.indisready
        AND i.indpred IS NULL
        AND i.indnkeyatts >= cardinality(con.conkey)
        AND (i.indkey::int2[])[0:cardinality(con.conkey) - 1] @> con.conkey
  )
ORDER BY c.relname, con.conname
`

type foreignKeyWithoutIndex struct {
	Table      string `gorm:"column:table_name"`
	Constraint string `gorm:"column:constraint_name"`
}

// assertForeignKeyCoverage guards the indexes that FK enforcement needs.
// PostgreSQL does not create them, and the ones dropped as "unused" had to be
// restored: the chain test fails with the table and constraint names instead.
func assertForeignKeyCoverage(tx *gorm.DB) error {
	var missing []foreignKeyWithoutIndex
	if err := tx.Raw(foreignKeyCoverageSQL).Scan(&missing).Error; err != nil {
		return fmt.Errorf("query foreign key index coverage: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}

	names := make([]string, len(missing))
	for i, fk := range missing {
		names[i] = fk.Table + "." + fk.Constraint
	}
	return fmt.Errorf("%d foreign keys have no covering index: %s", len(missing), strings.Join(names, ", "))
}
