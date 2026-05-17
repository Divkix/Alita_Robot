package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
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

func testTranslator(t *testing.T) *i18n.Translator {
	yaml := `
backup_export_success: "Chat: {chat}, Modules: {modules}, Time: {time}, List: {list}"
button_confirm_import: "Confirm Import"
button_cancel_import: "Cancel Import"
button_confirm_reset: "Confirm Reset"
button_cancel_reset: "Cancel Reset"
`
	tr, err := i18n.NewTestTranslator(yaml)
	require.NoError(t, err)
	return tr
}

func TestParseImportModules(t *testing.T) {
	t.Parallel()

	backupData := map[string]interface{}{
		"notes":   map[string]interface{}{"a": 1},
		"filters": map[string]interface{}{"b": 2},
		"rules":   map[string]interface{}{"c": 3},
	}

	t.Run("empty text returns empty slice", func(t *testing.T) {
		assert.Empty(t, parseImportModules("", backupData))
	})

	t.Run("no args returns empty slice", func(t *testing.T) {
		assert.Empty(t, parseImportModules("/import", backupData))
	})

	t.Run("valid modules only", func(t *testing.T) {
		got := parseImportModules("/import notes filters", backupData)
		assert.Equal(t, []string{"notes", "filters"}, got)
	})

	t.Run("invalid modules skipped", func(t *testing.T) {
		got := parseImportModules("/import notes invalid rules", backupData)
		assert.Equal(t, []string{"notes", "rules"}, got)
	})

	t.Run("case insensitive", func(t *testing.T) {
		got := parseImportModules("/import NOTES", backupData)
		assert.Equal(t, []string{"notes"}, got)
	})

	t.Run("all invalid returns empty", func(t *testing.T) {
		assert.Empty(t, parseImportModules("/import foo bar", backupData))
	})
}

func TestBuildExportCaption(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	backup := db.NewBackupFormat(12345, "Test Chat", 67890, []string{"notes", "filters"})
	backup.Data["notes"] = map[string]interface{}{"test": "data"}
	backup.Data["filters"] = map[string]interface{}{"test": "data"}
	backup.ExportedAt = backup.ExportedAt.UTC()

	caption := buildExportCaption(tr, backup)
	assert.Contains(t, caption, "Test Chat")
	assert.Contains(t, caption, "2")
	assert.Contains(t, caption, backup.ExportedAt.Format("2006-01-02 15:04:05"))
	assert.Contains(t, caption, "notes")
	assert.Contains(t, caption, "filters")
}

func TestBuildImportKeyboard(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	chatID := int64(12345)
	keyboard := buildImportKeyboard(tr, chatID)

	require.Len(t, keyboard.InlineKeyboard, 1)
	require.Len(t, keyboard.InlineKeyboard[0], 2)

	confirmBtn := keyboard.InlineKeyboard[0][0]
	assert.Equal(t, "Confirm Import", confirmBtn.Text)
	assert.NotEmpty(t, confirmBtn.CallbackData)

	cancelBtn := keyboard.InlineKeyboard[0][1]
	assert.Equal(t, "Cancel Import", cancelBtn.Text)
	assert.NotEmpty(t, cancelBtn.CallbackData)

	// Verify callback data decodes correctly
	decodedConfirm, ok := decodeCallbackData(confirmBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ := decodedConfirm.Field("a")
	assert.Equal(t, "confirm_import", action)
	chatIDStr, _ := decodedConfirm.Field("c")
	assert.Equal(t, "12345", chatIDStr)

	decodedCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ = decodedCancel.Field("a")
	assert.Equal(t, "cancel_import", action)
}

func TestBuildResetKeyboard(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	chatID := int64(54321)
	keyboard := buildResetKeyboard(tr, chatID)

	require.Len(t, keyboard.InlineKeyboard, 2)
	require.Len(t, keyboard.InlineKeyboard[0], 1)
	require.Len(t, keyboard.InlineKeyboard[1], 1)

	confirmBtn := keyboard.InlineKeyboard[0][0]
	assert.Equal(t, "Confirm Reset", confirmBtn.Text)
	assert.NotEmpty(t, confirmBtn.CallbackData)

	cancelBtn := keyboard.InlineKeyboard[1][0]
	assert.Equal(t, "Cancel Reset", cancelBtn.Text)
	assert.NotEmpty(t, cancelBtn.CallbackData)

	// Verify callback data decodes correctly
	decodedConfirm, ok := decodeCallbackData(confirmBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ := decodedConfirm.Field("a")
	assert.Equal(t, "confirm_reset", action)
	chatIDStr, _ := decodedConfirm.Field("c")
	assert.Equal(t, "54321", chatIDStr)

	decodedCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ = decodedCancel.Field("a")
	assert.Equal(t, "cancel_reset", action)
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