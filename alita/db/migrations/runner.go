package migrations

import (
	log "github.com/sirupsen/logrus"
)

// MigrationRunner is a dummy runner for MongoDB
type MigrationRunner struct {
}

// NewMigrationRunner creates a new dummy migration runner
func NewMigrationRunner(any interface{}) *MigrationRunner {
	return &MigrationRunner{}
}

// RunMigrations does nothing for MongoDB
func (m *MigrationRunner) RunMigrations() error {
	log.Info("[Migrations] SQL migrations skipped for MongoDB")
	return nil
}
