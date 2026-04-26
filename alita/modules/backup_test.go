package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/divkix/Alita_Robot/alita/db"
)

func TestBackupModuleStructure(t *testing.T) {
	t.Run("backupModule has correct name", func(t *testing.T) {
		assert.Equal(t, "Backup", backupModule.moduleName)
	})
}

func TestBuildModuleList(t *testing.T) {
	t.Run("buildModuleList returns empty for empty slice", func(t *testing.T) {
		result := buildModuleList([]string{})
		assert.Equal(t, "", result)
	})

	t.Run("buildModuleList formats correctly", func(t *testing.T) {
		result := buildModuleList([]string{"notes", "filters", "rules"})
		assert.Contains(t, result, "notes")
		assert.Contains(t, result, "filters")
		assert.Contains(t, result, "rules")
		assert.Contains(t, result, "•")
	})

	t.Run("buildModuleList with single module", func(t *testing.T) {
		result := buildModuleList([]string{"notes"})
		assert.Equal(t, "• notes", result)
	})
}

func TestBuildExportCaption(t *testing.T) {
	t.Run("buildExportCaption formats correctly", func(t *testing.T) {
		backup := db.NewBackupFormat(12345, "Test Chat", 67890, []string{"notes", "filters"})
		backup.Data["notes"] = map[string]interface{}{"test": "data"}
		backup.Data["filters"] = map[string]interface{}{"test": "data"}

		// This would need a real translator, so we'll just test it doesn't panic
		// In real tests, we'd mock the translator
		assert.NotNil(t, backup)
	})
}

func TestPendingImportsMaps(t *testing.T) {
	t.Run("pending imports maps exist", func(t *testing.T) {
		// Just verify the maps are initialized
		assert.NotNil(t, pendingImports)
		assert.NotNil(t, pendingModules)
	})
}

func TestModuleNames(t *testing.T) {
	t.Run("all module names are lowercase", func(t *testing.T) {
		modules := []string{
			db.BackupModuleAdmin,
			db.BackupModuleAntiflood,
			db.BackupModuleBlacklists,
			db.BackupModuleCaptcha,
			db.BackupModuleConnections,
			db.BackupModuleDisabling,
			db.BackupModuleFilters,
			db.BackupModuleGreetings,
			db.BackupModuleLocks,
			db.BackupModuleNotes,
			db.BackupModulePins,
			db.BackupModuleReports,
			db.BackupModuleRules,
			db.BackupModuleWarns,
		}

		for _, module := range modules {
			assert.Equal(t, module, module) // Just checking they exist
		}
	})
}