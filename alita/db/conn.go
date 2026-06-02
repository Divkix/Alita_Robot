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

// isCliModeActive returns true if the program is running with CLI flags
// that should skip database initialization (--version, --health, -v).
func isCliModeActive() bool {
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/go-build") {
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

func init() {
	if isCliModeActive() {
		return
	}
	if os.Getenv("DATABASE_URL") == "" {
		return
	}

	var err error

	gormLogger := logger.New(
		log.StandardLogger(),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	dsn := config.AppConfig.DatabaseURL
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}

	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger:      gormLogger,
			PrepareStmt: true,
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
	if err != nil {
		log.Fatalf("[Database][Connection] Failed after %d attempts: %v", maxRetries, err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("[Database][SQL DB]: %v", err)
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
}

// Close closes the database connection gracefully.
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
