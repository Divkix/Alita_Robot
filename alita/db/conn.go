package db

import (
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/migrations"
)

var (
	DB *gorm.DB
)

func isCliModeActive() bool {
	testBinary := strings.TrimSuffix(os.Args[0], ".exe")
	if strings.HasSuffix(testBinary, ".test") &&
		!strings.EqualFold(strings.TrimSpace(os.Getenv("ALITA_TEST_DATABASE")), "true") {
		return true
	}
	if len(os.Args) < 2 {
		return false
	}
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version", "-version", "-v", "--health", "-health":
			return true
		}
	}
	return false
}

// newGormLogger logs slow and failed statements without row values, which
// carry chat and user content, as logs are exported over OTLP:
// ParameterizedQueries keeps the SQL as placeholders, and safeWriter drops
// the error text, which Postgres and pgx fill with values.
func newGormLogger(w logger.Writer) logger.Interface {
	return logger.New(safeWriter{w}, logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
}

// safeWriter writes each error argument as safeQueryError does. It wraps the
// writer, not the logger, so GORM still reports the caller's file and line.
type safeWriter struct {
	logger.Writer
}

func (w safeWriter) Printf(format string, args ...any) {
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			_, args[i] = safeQueryError(err)
		}
	}
	w.Writer.Printf(format, args...)
}

func init() {
	if isCliModeActive() {
		return
	}
	if os.Getenv("DATABASE_URL") == "" {
		return
	}

	var err error

	gormLogger := newGormLogger(log.StandardLogger())

	dsn := config.AppConfig.DatabaseURL
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}

	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger:      gormLogger,
			PrepareStmt: true,
			// Single statements are already atomic in Postgres; multi-statement writes use explicit db.DB.Transaction.
			SkipDefaultTransaction: true,
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		})
		if err == nil {
			break
		}

		log.WithFields(log.Fields{
			"attempt": attempt + 1,
			"error":   err,
		}).Warning("[Database][Connection] Failed to connect, retrying...")

		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	if err != nil || DB == nil {
		log.Fatalf("[Database][Connection] Failed after %d attempts: %v", maxRetries, err)
	}

	if err := registerQueryMetrics(DB); err != nil {
		log.Fatalf("[Database][Metrics] Failed to register query metrics: %v", err)
	}
	if err := registerQueryTracing(DB); err != nil {
		log.Fatalf("[Database][Tracing] Failed to register query tracing: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("[Database][SQL DB]: %v", err)
	}
	if err := registerPoolMetrics(sqlDB); err != nil {
		log.Fatalf("[Database][Metrics] Failed to register pool metrics: %v", err)
	}

	sqlDB.SetMaxIdleConns(config.AppConfig.DBMaxIdleConns)
	sqlDB.SetMaxOpenConns(config.AppConfig.DBMaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.AppConfig.DBConnMaxLifetimeMin) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(config.AppConfig.DBConnMaxIdleTimeMin) * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("[Database][Ping]: %v", err)
	}

	log.Info("Connected to PostgreSQL database successfully!")

	if config.AppConfig.AutoMigrate {
		log.Info("[Database] AUTO_MIGRATE is enabled, running database migrations...")
		runner := migrations.NewMigrationRunner(DB)
		if err := runner.RunMigrations(); err != nil {
			if config.AppConfig.AutoMigrateSilentFail {
				log.Errorf("[Database][AutoMigrate] Migration failed but continuing: %v", err)
			} else {
				log.Fatalf("[Database][AutoMigrate] Migration failed: %v", err)
			}
		} else {
			log.Info("[Database][AutoMigrate] All migrations applied successfully")
		}
	} else {
		log.Info("Database schema managed via SQL migrations - skipping auto-migration")
	}
	DB.DefaultContextTimeout = 30 * time.Second
}

func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying SQL DB: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}
