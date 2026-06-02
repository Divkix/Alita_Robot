package backup

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/pins"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestBackupTypes(t *testing.T) {
	t.Run("NewBackupFormat creates valid backup", func(t *testing.T) {
		backup := NewBackupFormat(12345, "Test Chat", 67890, []string{"notes", "filters"})

		assert.Equal(t, BackupFormatVersion, backup.Version)
		assert.Equal(t, "AlitaRobot", backup.BotName)
		assert.Equal(t, int64(12345), backup.ChatID)
		assert.Equal(t, "Test Chat", backup.ChatName)
		assert.Equal(t, int64(67890), backup.ExportedBy)
		assert.Equal(t, []string{"notes", "filters"}, backup.Modules)
		assert.NotNil(t, backup.Data)
		assert.WithinDuration(t, time.Now().UTC(), backup.ExportedAt, time.Second)
	})

	t.Run("BackupFormat validation", func(t *testing.T) {
		tests := []struct {
			name    string
			backup  *BackupFormat
			wantErr bool
		}{
			{
				name: "valid backup",
				backup: &BackupFormat{
					Version:    "1.0",
					BotName:    "AlitaRobot",
					ChatID:     12345,
					Modules:    []string{"notes"},
					Data:       make(map[string]interface{}),
					ExportedAt: time.Now(),
				},
				wantErr: false,
			},
			{
				name: "missing version",
				backup: &BackupFormat{
					BotName: "AlitaRobot",
					ChatID:  12345,
					Modules: []string{"notes"},
					Data:    make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "missing bot name",
				backup: &BackupFormat{
					Version: "1.0",
					ChatID:  12345,
					Modules: []string{"notes"},
					Data:    make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "missing chat ID",
				backup: &BackupFormat{
					Version: "1.0",
					BotName: "AlitaRobot",
					Modules: []string{"notes"},
					Data:    make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "empty modules",
				backup: &BackupFormat{
					Version: "1.0",
					BotName: "AlitaRobot",
					ChatID:  12345,
					Modules: []string{},
					Data:    make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "nil data",
				backup: &BackupFormat{
					Version: "1.0",
					BotName: "AlitaRobot",
					ChatID:  12345,
					Modules: []string{"notes"},
					Data:    nil,
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.backup.Validate()
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("IsCompatibleVersion checks version", func(t *testing.T) {
		compatible := &BackupFormat{Version: BackupFormatVersion}
		assert.True(t, compatible.IsCompatibleVersion())

		incompatible := &BackupFormat{Version: "0.9"}
		assert.False(t, incompatible.IsCompatibleVersion())
	})

	t.Run("ToJSON marshals correctly", func(t *testing.T) {
		backup := NewBackupFormat(12345, "Test", 67890, []string{"notes"})
		backup.Data["notes"] = []models.Notes{{NoteName: "test", NoteContent: "reply"}}

		jsonData, err := backup.ToJSON()
		require.NoError(t, err)
		assert.NotNil(t, jsonData)
		assert.Contains(t, string(jsonData), "AlitaRobot")
		assert.Contains(t, string(jsonData), "notes")
	})

	t.Run("BackupFormatFromJSON unmarshals correctly", func(t *testing.T) {
		jsonData := `{
			"version": "1.0",
			"bot_name": "AlitaRobot",
			"chat_id": 12345,
			"chat_name": "Test Chat",
			"exported_by": 67890,
			"modules": ["notes", "filters"],
			"data": {"notes": [{"note_name": "welcome", "note_content": "Hello!"}]},
			"exported_at": "2024-01-01T00:00:00Z"
		}`

		backup, err := BackupFormatFromJSON([]byte(jsonData))
		require.NoError(t, err)
		assert.Equal(t, "1.0", backup.Version)
		assert.Equal(t, "AlitaRobot", backup.BotName)
		assert.Equal(t, int64(12345), backup.ChatID)
		assert.Equal(t, []string{"notes", "filters"}, backup.Modules)
	})

	t.Run("BackupFormatFromJSON returns error on invalid JSON", func(t *testing.T) {
		_, err := BackupFormatFromJSON([]byte("invalid json"))
		assert.Error(t, err)
	})
}

func TestModuleValidation(t *testing.T) {
	t.Run("AllExportableModules returns expected modules", func(t *testing.T) {
		modules := AllExportableModules()
		assert.NotEmpty(t, modules)
		assert.Contains(t, modules, BackupModuleAdmin)
		assert.Contains(t, modules, BackupModuleNotes)
		assert.Contains(t, modules, BackupModuleFilters)
		assert.Contains(t, modules, BackupModuleRules)
	})

	t.Run("IsValidModule validates correctly", func(t *testing.T) {
		assert.True(t, IsValidModule("notes"))
		assert.True(t, IsValidModule("filters"))
		assert.False(t, IsValidModule("invalid"))
		assert.False(t, IsValidModule(""))
	})

	t.Run("FilterValidModules filters correctly", func(t *testing.T) {
		input := []string{"notes", "filters", "invalid", "rules"}
		filtered := FilterValidModules(input)
		assert.Contains(t, filtered, "notes")
		assert.Contains(t, filtered, "filters")
		assert.Contains(t, filtered, "rules")
		assert.NotContains(t, filtered, "invalid")
	})
}

func TestExportModuleData(t *testing.T) {
	t.Run("ExportModuleData for invalid module", func(t *testing.T) {
		_, err := ExportModuleData(12345, "invalid_module")
		assert.Error(t, err)
	})

	t.Run("ImportModuleData with invalid module", func(t *testing.T) {
		err := ImportModuleData(12345, "invalid_module", map[string]interface{}{})
		assert.Error(t, err)
	})

	t.Run("ClearModuleData with invalid module", func(t *testing.T) {
		err := ClearModuleData(12345, "invalid_module")
		assert.Error(t, err)
	})
}

func TestImportModuleDataRejectsMalformedPayloadForEveryModule(t *testing.T) {
	for _, module := range AllExportableModules() {
		t.Run(module, func(t *testing.T) {
			err := ImportModuleData(12345, module, "not a backup object")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid")
			assert.Contains(t, err.Error(), "data format")
		})
	}
}

func TestClearModuleDataConnectionsDisablesAllowConnect(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_backup_connections_clear"))
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{}).Error; err != nil {
			t.Fatalf("cleanup Delete(ConnectionChatSettings) error: %v", err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
			t.Fatalf("cleanup Delete(Chat) error: %v", err)
		}
	})

	_ = connections.GetChatConnectionSetting(chatID)
	connections.ToggleAllowConnect(chatID, true)
	require.True(t, connections.GetChatConnectionSetting(chatID).AllowConnect)

	require.NoError(t, ClearModuleData(chatID, BackupModuleConnections))

	assert.False(t, connections.GetChatConnectionSetting(chatID).AllowConnect)
}

func TestExportChatData(t *testing.T) {
	t.Run("ExportChatData with no valid modules", func(t *testing.T) {
		_, err := ExportChatData(12345, "Test", 67890, []string{"invalid_module"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid modules specified")
	})

	t.Run("ExportChatData with empty modules exports all", func(t *testing.T) {
		// Just verify it doesn't error with nil modules
		backup := NewBackupFormat(12345, "Test", 67890, AllExportableModules())
		assert.NotNil(t, backup)
	})
}

func TestBackupDataStructures(t *testing.T) {
	t.Run("AdminBackup struct", func(t *testing.T) {
		backup := &AdminBackup{
			AdminSettings: &models.AdminSettings{
				ChatId:    12345,
				AnonAdmin: true,
			},
			BlacklistMode: "ban",
		}
		assert.Equal(t, int64(12345), backup.AdminSettings.ChatId)
		assert.True(t, backup.AdminSettings.AnonAdmin)
		assert.Equal(t, "ban", backup.BlacklistMode)
	})

	t.Run("AntifloodBackup struct", func(t *testing.T) {
		backup := &AntifloodBackup{
			Settings: &models.AntifloodSettings{
				ChatId: 12345,
				Limit:  5,
				Action: "mute",
			},
		}
		assert.Equal(t, 5, backup.Settings.Limit)
		assert.Equal(t, "mute", backup.Settings.Action)
	})

	t.Run("NotesBackup struct", func(t *testing.T) {
		backup := &NotesBackup{
			Notes: []models.Notes{
				{
					ChatId:      12345,
					NoteName:    "welcome",
					NoteContent: "Hello!",
				},
			},
		}
		assert.Len(t, backup.Notes, 1)
		assert.Equal(t, "welcome", backup.Notes[0].NoteName)
	})
}

// cleanupChat removes all test data for a chatID across known backup-related tables.
// cleanupBackupChat removes all test data for a chatID across known backup-related tables.
// Uses t.Errorf not t.Fatalf so a failure for one table still attempts the others.
func cleanupBackupChat(t *testing.T, chatID int64) {
	t.Helper()
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.AdminSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting AdminSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.AntifloodSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting AntifloodSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.BlacklistSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting BlacklistSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.CaptchaSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting CaptchaSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting ConnectionChatSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.DisableSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting DisableSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.DisableChatSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting DisableChatSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatFilters{}).Error; err != nil {
		t.Errorf("cleanup failed deleting ChatFilters: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.GreetingSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting GreetingSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.LockSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting LockSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.NotesSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting NotesSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Notes{}).Error; err != nil {
		t.Errorf("cleanup failed deleting Notes: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.PinSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting PinSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting ReportChatSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting RulesSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting WarnSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
		t.Errorf("cleanup failed deleting Chat: %v", err)
	}
}

func TestExportAdminData(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_export_admin"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Configure admin-related settings
	require.NoError(t, admin.SetAnonAdminMode(chatID, true))
	require.NoError(t, antiflood.SetFlood(chatID, 7))
	require.NoError(t, antiflood.SetFloodMode(chatID, "ban"))
	require.NoError(t, captcha.SetCaptchaEnabled(chatID, true))
	require.NoError(t, captcha.SetCaptchaMode(chatID, "text"))

	backup, err := exportAdminData(chatID)
	require.NoError(t, err)
	require.NotNil(t, backup)

	require.NotNil(t, backup.AdminSettings)
	assert.Equal(t, chatID, backup.AdminSettings.ChatId)
	assert.True(t, backup.AdminSettings.AnonAdmin)

	require.NotNil(t, backup.AntifloodSettings)
	assert.Equal(t, 7, backup.AntifloodSettings.Limit)
	assert.Equal(t, "ban", backup.AntifloodSettings.Action)

	require.NotNil(t, backup.CaptchaSettings)
	assert.True(t, backup.CaptchaSettings.Enabled)
	assert.Equal(t, "text", backup.CaptchaSettings.CaptchaMode)
}

func TestImportAdminData(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_admin"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Ensure admin settings record exists before import
	_ = admin.GetAdminSettings(chatID)

	// Build import payload as map (mimics JSON round-trip)
	payload := map[string]interface{}{
		"admin_settings": map[string]interface{}{
			"chat_id":    float64(chatID),
			"anon_admin": true,
		},
		"antiflood_settings": map[string]interface{}{
			"chat_id": float64(chatID),
			"limit":   float64(10),
			"action":  "kick",
		},
		"captcha_settings": map[string]interface{}{
			"chat_id":        float64(chatID),
			"enabled":        true,
			"captcha_mode":   "math",
			"timeout":        float64(5),
			"max_attempts":   float64(3),
			"failure_action": "ban",
		},
	}

	require.NoError(t, importAdminData(chatID, payload))

	adminSettings := admin.GetAdminSettings(chatID)
	require.NotNil(t, adminSettings)
	assert.True(t, adminSettings.AnonAdmin)

	flood := antiflood.GetFlood(chatID)
	require.NotNil(t, flood)
	assert.Equal(t, 10, flood.Limit)
	assert.Equal(t, "kick", flood.Action)

	captchaSettings, err := captcha.GetCaptchaSettings(chatID)
	require.NoError(t, err)
	require.NotNil(t, captchaSettings)
	assert.True(t, captchaSettings.Enabled)
	assert.Equal(t, "math", captchaSettings.CaptchaMode)
}

func TestImportAdminData_InvalidFormat(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_admin_invalid"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	err := importAdminData(chatID, "not a map")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid admin data format")
}

func TestExportFiltersData(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_export_filters"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Empty filters → returns empty backup
	backup, err := exportFiltersData(chatID)
	require.NoError(t, err)
	require.NotNil(t, backup)
	assert.Empty(t, backup.Filters)

	// Add filters
	require.NoError(t, filters.AddFilter(chatID, "hello", "hi there", "", nil, db.TEXT))
	require.NoError(t, filters.AddFilter(chatID, "bye", "see ya", "", nil, db.TEXT))

	backup, err = exportFiltersData(chatID)
	require.NoError(t, err)
	require.NotNil(t, backup)
	assert.Len(t, backup.Filters, 2)

	names := make([]string, len(backup.Filters))
	for i, f := range backup.Filters {
		names[i] = f.KeyWord
	}
	assert.Contains(t, names, "hello")
	assert.Contains(t, names, "bye")
}

func TestImportFiltersData(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_filters"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	payload := map[string]interface{}{
		"filters": []map[string]interface{}{
			{
				"chat_id":      float64(chatID),
				"keyword":      "spam",
				"filter_reply": "no spam",
				"msgtype":      float64(db.TEXT),
			},
			{
				"chat_id":      float64(chatID),
				"keyword":      "ad",
				"filter_reply": "no ads",
				"msgtype":      float64(db.TEXT),
			},
		},
	}

	require.NoError(t, importFiltersData(chatID, payload))

	list := filters.GetFiltersList(chatID)
	assert.Len(t, list, 2)
	assert.Contains(t, list, "spam")
	assert.Contains(t, list, "ad")
}

func TestImportFiltersData_InvalidFormat(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_filters_invalid"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	err := importFiltersData(chatID, "not a map")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filters data format")
}

func TestExportImportNotesRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_notes"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_notes"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	// Add notes to source chat
	require.NoError(t, notes.AddNote(srcChat, "welcome", "Welcome!", "", nil, db.TEXT, false, false, false, true, false, false))
	require.NoError(t, notes.AddNote(srcChat, "rules", "Follow the rules", "", nil, db.TEXT, false, false, false, true, false, false))

	// Export
	exported, err := exportNotesData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	assert.Len(t, exported.Notes, 2)

	// Convert to map for import
	payload := map[string]interface{}{
		"notes": exported.Notes,
	}

	// Import into destination
	require.NoError(t, importNotesData(dstChat, payload))

	list := notes.GetNotesList(dstChat, true)
	assert.Len(t, list, 2)
	assert.Contains(t, list, "welcome")
	assert.Contains(t, list, "rules")

	note := notes.GetNote(dstChat, "welcome")
	require.NotNil(t, note)
	assert.Equal(t, "Welcome!", note.NoteContent)
}

func TestExportImportRulesRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_rules"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_rules"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	rules.SetChatRules(srcChat, "Be nice")
	rules.SetChatRulesButton(srcChat, "Read Rules")
	rules.SetPrivateRules(srcChat, true)

	exported, err := exportRulesData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	assert.Equal(t, "Be nice", exported.Settings.Rules)
	assert.Equal(t, "Read Rules", exported.Settings.RulesBtn)
	assert.True(t, exported.Settings.Private)

	payload := map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":   float64(dstChat),
			"rules":     "Be nice",
			"rules_btn": "Read Rules",
			"private":   true,
		},
	}

	require.NoError(t, importRulesData(dstChat, payload))

	settings := rules.GetChatRulesInfo(dstChat)
	require.NotNil(t, settings)
	assert.Equal(t, "Be nice", settings.Rules)
	assert.Equal(t, "Read Rules", settings.RulesBtn)
	assert.True(t, settings.Private)
}

func TestExportImportLocksRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_locks"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_locks"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, locks.UpdateLock(srcChat, " stickers", true))
	require.NoError(t, locks.UpdateLock(srcChat, " url", false))

	// Export
	exported, err := exportLocksData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	assert.Len(t, exported.Locks, 2)

	// Convert to map for import
	payload := map[string]interface{}{
		"locks": exported.Locks,
	}

	// Import into destination
	require.NoError(t, importLocksData(dstChat, payload))

	lockMap := locks.GetChatLocks(dstChat)
	assert.True(t, lockMap[" stickers"])
	assert.False(t, lockMap[" url"])
}

func TestExportImportWarnsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_warns"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_warns"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, warns.SetWarnLimit(srcChat, 5))
	require.NoError(t, warns.SetWarnMode(srcChat, "ban"))

	exported, err := exportWarnsData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.WarnSettings)
	assert.Equal(t, 5, exported.WarnSettings.WarnLimit)
	assert.Equal(t, "ban", exported.WarnSettings.WarnMode)

	payload := map[string]interface{}{
		"warn_settings": map[string]interface{}{
			"chat_id":    float64(dstChat),
			"warn_limit": float64(5),
			"warn_mode":  "ban",
		},
	}

	require.NoError(t, importWarnsData(dstChat, payload))

	settings := warns.GetWarnSetting(dstChat)
	require.NotNil(t, settings)
	assert.Equal(t, 5, settings.WarnLimit)
	assert.Equal(t, "ban", settings.WarnMode)
}

func TestExportImportBlacklistsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_blacklists"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_blacklists"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, blacklists.AddBlacklist(srcChat, "spam"))
	require.NoError(t, blacklists.AddBlacklist(srcChat, "scam"))
	require.NoError(t, blacklists.SetBlacklistAction(srcChat, "ban"))

	exported, err := exportBlacklistsData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	assert.Equal(t, "ban", exported.BlacklistMode)
	assert.Len(t, exported.Entries, 2)

	payload := map[string]interface{}{
		"entries": exported.Entries,
	}

	require.NoError(t, importBlacklistsData(dstChat, payload))

	settings := blacklists.GetBlacklistSettings(dstChat)
	assert.Len(t, settings, 2)
	assert.Equal(t, "ban", settings.Action())
}

func TestExportImportDisablingRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_disabling"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_disabling"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, disabling.DisableCMD(srcChat, "kick"))
	require.NoError(t, disabling.DisableCMD(srcChat, "ban"))
	require.NoError(t, disabling.ToggleDel(srcChat, true))

	exported, err := exportDisablingData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	assert.Len(t, exported.Commands, 2)
	require.NotNil(t, exported.ChatSettings)
	assert.True(t, exported.ChatSettings.DeleteCommands)

	payload := map[string]interface{}{
		"chat_settings": map[string]interface{}{
			"chat_id":         float64(dstChat),
			"delete_commands": true,
		},
		"commands": []map[string]interface{}{
			{"chat_id": float64(dstChat), "command": "kick", "disabled": true},
			{"chat_id": float64(dstChat), "command": "ban", "disabled": true},
		},
	}

	require.NoError(t, importDisablingData(dstChat, payload))

	disabled := disabling.GetChatDisabledCMDs(dstChat)
	assert.Len(t, disabled, 2)
	assert.Contains(t, disabled, "kick")
	assert.Contains(t, disabled, "ban")
	assert.True(t, disabling.ShouldDel(dstChat))
}

func TestExportImportConnectionsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_connections"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_connections"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	connections.ToggleAllowConnect(srcChat, true)

	exported, err := exportConnectionsData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	assert.True(t, exported.Settings.AllowConnect)

	payload := map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":       float64(dstChat),
			"allow_connect": true,
		},
	}

	require.NoError(t, importConnectionsData(dstChat, payload))

	settings := connections.GetChatConnectionSetting(dstChat)
	assert.True(t, settings.AllowConnect)
}

func TestExportImportCaptchaRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_captcha"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_captcha"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, captcha.SetCaptchaEnabled(srcChat, true))
	require.NoError(t, captcha.SetCaptchaMode(srcChat, "text"))
	require.NoError(t, captcha.SetCaptchaTimeout(srcChat, 7))
	require.NoError(t, captcha.SetCaptchaMaxAttempts(srcChat, 5))

	exported, err := exportCaptchaData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	assert.True(t, exported.Settings.Enabled)
	assert.Equal(t, "text", exported.Settings.CaptchaMode)
	assert.Equal(t, 7, exported.Settings.Timeout)
	assert.Equal(t, 5, exported.Settings.MaxAttempts)

	payload := map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":        float64(dstChat),
			"enabled":        true,
			"captcha_mode":   "text",
			"timeout":        float64(7),
			"max_attempts":   float64(5),
			"failure_action": "kick",
		},
	}

	require.NoError(t, importCaptchaData(dstChat, payload))

	settings, err := captcha.GetCaptchaSettings(dstChat)
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.True(t, settings.Enabled)
	assert.Equal(t, "text", settings.CaptchaMode)
	assert.Equal(t, 7, settings.Timeout)
	assert.Equal(t, 5, settings.MaxAttempts)
}

func TestExportImportAntifloodRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_antiflood"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_antiflood"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, antiflood.SetFlood(srcChat, 3))
	require.NoError(t, antiflood.SetFloodMode(srcChat, "mute"))
	require.NoError(t, antiflood.SetFloodMsgDel(srcChat, true))

	exported, err := exportAntifloodData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	assert.Equal(t, 3, exported.Settings.Limit)
	assert.Equal(t, "mute", exported.Settings.Action)
	assert.True(t, exported.Settings.DeleteAntifloodMessage)

	payload := map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":                  float64(dstChat),
			"limit":                    float64(3),
			"action":                   "mute",
			"delete_antiflood_message": true,
		},
	}

	require.NoError(t, importAntifloodData(dstChat, payload))

	settings := antiflood.GetFlood(dstChat)
	require.NotNil(t, settings)
	assert.Equal(t, 3, settings.Limit)
	assert.Equal(t, "mute", settings.Action)
	assert.True(t, settings.DeleteAntifloodMessage)
}

func TestExportImportGreetingsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_greetings"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_greetings"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	require.NoError(t, greetings.SetWelcomeText(srcChat, "Hello {first}!", "", nil, db.TEXT))
	require.NoError(t, greetings.SetWelcomeToggle(srcChat, true))
	require.NoError(t, greetings.SetGoodbyeText(srcChat, "Bye {first}!", "", nil, db.TEXT))

	exported, err := exportGreetingsData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	require.NotNil(t, exported.Settings.WelcomeSettings)
	assert.Equal(t, "Hello {first}!", exported.Settings.WelcomeSettings.WelcomeText)
	assert.True(t, exported.Settings.WelcomeSettings.ShouldWelcome)
	require.NotNil(t, exported.Settings.GoodbyeSettings)
	assert.Equal(t, "Bye {first}!", exported.Settings.GoodbyeSettings.GoodbyeText)

	// Ensure greeting record exists in dst before import
	_ = greetings.GetGreetingSettings(dstChat)

	// Build payload from exported JSON so keys match struct tags exactly
	exportedJSON, err := json.Marshal(exported)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(exportedJSON, &payload))

	require.NoError(t, importGreetingsData(dstChat, payload))

	settings := greetings.GetGreetingSettings(dstChat)
	require.NotNil(t, settings)
	require.NotNil(t, settings.WelcomeSettings)
	assert.Equal(t, "Hello {first}!", settings.WelcomeSettings.WelcomeText)
	assert.True(t, settings.WelcomeSettings.ShouldWelcome)
	require.NotNil(t, settings.GoodbyeSettings)
	assert.Equal(t, "Bye {first}!", settings.GoodbyeSettings.GoodbyeText)
}

func TestExportImportPinsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_pins"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_pins"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	_ = pins.GetPinData(srcChat)
	require.NoError(t, pins.SetAntiChannelPin(srcChat, true))

	// Ensure dst has record before import
	_ = pins.GetPinData(dstChat)

	exported, err := exportPinsData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	assert.True(t, exported.Settings.AntiChannelPin)

	payload := map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":          float64(dstChat),
			"anti_channel_pin": true,
		},
	}

	require.NoError(t, importPinsData(dstChat, payload))

	settings := pins.GetPinData(dstChat)
	require.NotNil(t, settings)
	assert.True(t, settings.AntiChannelPin)
}

func TestExportImportReportsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	require.NoError(t, chats.EnsureChatInDb(srcChat, "src_reports"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "dst_reports"))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
	})

	// Ensure src record exists, then disable
	_ = reports.GetChatReportSettings(srcChat)
	require.NoError(t, reports.SetChatReportStatus(srcChat, false))

	exported, err := exportReportsData(srcChat)
	require.NoError(t, err)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Settings)
	assert.False(t, exported.Settings.Enabled)

	// Ensure dst record exists before import (create if missing)
	_ = reports.GetChatReportSettings(dstChat)

	payload := map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id": float64(dstChat),
			"enabled": false,
		},
	}

	require.NoError(t, importReportsData(dstChat, payload))

	settings := reports.GetChatReportSettings(dstChat)
	require.NotNil(t, settings)
	assert.False(t, settings.Enabled)
}

func TestImportChatData_Validation(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_chat_data"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	invalidBackup := &BackupFormat{
		Version: "", // empty version triggers validation error
		BotName: "OtherBot",
		ChatID:  chatID,
		Modules: []string{"notes"},
		Data:    map[string]interface{}{},
	}

	err := ImportChatData(chatID, invalidBackup, []string{"notes"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid backup")
}

func TestImportChatData_SingleModule(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_chat_single"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	backup := NewBackupFormat(chatID, "Test", 1, []string{BackupModuleWarns})
	backup.Data[BackupModuleWarns] = map[string]interface{}{
		"warn_settings": map[string]interface{}{
			"chat_id":    float64(chatID),
			"warn_limit": float64(7),
			"warn_mode":  "kick",
		},
	}

	require.NoError(t, ImportChatData(chatID, backup, []string{BackupModuleWarns}))

	settings := warns.GetWarnSetting(chatID)
	require.NotNil(t, settings)
	assert.Equal(t, 7, settings.WarnLimit)
	assert.Equal(t, "kick", settings.WarnMode)
}

func TestImportChatData_AllModulesFromBackup(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_chat_all"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	backup := NewBackupFormat(chatID, "Test", 1, []string{BackupModuleFilters, BackupModuleRules})
	backup.Data[BackupModuleFilters] = map[string]interface{}{
		"filters": []map[string]interface{}{
			{"chat_id": float64(chatID), "keyword": "test", "filter_reply": "reply", "msgtype": float64(db.TEXT)},
		},
	}
	backup.Data[BackupModuleRules] = map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id": float64(chatID),
			"rules":   "test rules",
		},
	}

	require.NoError(t, ImportChatData(chatID, backup, nil))

	assert.Len(t, filters.GetFiltersList(chatID), 1)
	assert.Equal(t, "test rules", rules.GetChatRulesInfo(chatID).Rules)
}

func TestClearChatData_SpecificModules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_clear_specific"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Set up data for multiple modules
	require.NoError(t, filters.AddFilter(chatID, "hello", "hi", "", nil, db.TEXT))
	require.NoError(t, antiflood.SetFlood(chatID, 5))
	rules.SetChatRules(chatID, "rules text")

	// Clear only filters
	require.NoError(t, ClearChatData(chatID, []string{BackupModuleFilters}))

	assert.Empty(t, filters.GetFiltersList(chatID))
	assert.Equal(t, 5, antiflood.GetFlood(chatID).Limit)
	assert.Equal(t, "rules text", rules.GetChatRulesInfo(chatID).Rules)
}

func TestClearChatData_AllModules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_clear_all"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Set up data
	require.NoError(t, filters.AddFilter(chatID, "hello", "hi", "", nil, db.TEXT))
	require.NoError(t, blacklists.AddBlacklist(chatID, "bad"))
	require.NoError(t, antiflood.SetFlood(chatID, 5))
	rules.SetChatRules(chatID, "rules text")
	require.NoError(t, captcha.SetCaptchaEnabled(chatID, true))
	_ = pins.GetPinData(chatID)
	require.NoError(t, pins.SetAntiChannelPin(chatID, true))
	_ = reports.GetChatReportSettings(chatID)
	_ = admin.GetAdminSettings(chatID)

	// Clear all (empty modules)
	require.NoError(t, ClearChatData(chatID, nil))

	assert.Empty(t, filters.GetFiltersList(chatID))
	assert.Len(t, blacklists.GetBlacklistSettings(chatID), 0)
	assert.Equal(t, 0, antiflood.GetFlood(chatID).Limit)
	assert.Equal(t, "", rules.GetChatRulesInfo(chatID).Rules)

	captchaSettings, _ := captcha.GetCaptchaSettings(chatID)
	if captchaSettings != nil {
		assert.False(t, captchaSettings.Enabled)
	}

	pin := pins.GetPinData(chatID)
	if pin != nil {
		assert.False(t, pin.AntiChannelPin)
	}
}

func TestClearChatData_InvalidModule(t *testing.T) {
	skipIfNoDb(t)

	err := ClearChatData(12345, []string{"invalid_module"})
	assert.NoError(t, err) // FilterValidModules strips invalid, leaving empty → no-op loop
}

func TestClearModuleData_IndividualModules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_clear_individual"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// --- Filters ---
	require.NoError(t, filters.AddFilter(chatID, "f", "r", "", nil, db.TEXT))
	require.NoError(t, ClearModuleData(chatID, BackupModuleFilters))
	assert.Empty(t, filters.GetFiltersList(chatID))

	// --- Blacklists ---
	require.NoError(t, blacklists.AddBlacklist(chatID, "badword"))
	require.NoError(t, ClearModuleData(chatID, BackupModuleBlacklists))
	assert.Empty(t, blacklists.GetBlacklistSettings(chatID))

	// --- Notes ---
	require.NoError(t, notes.AddNote(chatID, "n1", "c1", "", nil, db.TEXT, false, false, false, true, false, false))
	require.NoError(t, ClearModuleData(chatID, BackupModuleNotes))
	assert.Empty(t, notes.GetNotesList(chatID, true))

	// --- Rules ---
	rules.SetChatRules(chatID, "some rules")
	require.NoError(t, ClearModuleData(chatID, BackupModuleRules))
	assert.Equal(t, "", rules.GetChatRulesInfo(chatID).Rules)

	// --- Warns ---
	require.NoError(t, warns.SetWarnLimit(chatID, 10))
	require.NoError(t, warns.SetWarnMode(chatID, "ban"))
	require.NoError(t, ClearModuleData(chatID, BackupModuleWarns))
	assert.Equal(t, 3, warns.GetWarnSetting(chatID).WarnLimit)
	assert.Equal(t, "", warns.GetWarnSetting(chatID).WarnMode)

	// --- Locks ---
	require.NoError(t, locks.UpdateLock(chatID, " stickers", true))
	require.NoError(t, ClearModuleData(chatID, BackupModuleLocks))
	assert.False(t, locks.GetChatLocks(chatID)[" stickers"])

	// --- Greetings ---
	require.NoError(t, greetings.SetWelcomeToggle(chatID, true))
	require.NoError(t, ClearModuleData(chatID, BackupModuleGreetings))
	settings := greetings.GetGreetingSettings(chatID)
	if settings != nil && settings.WelcomeSettings != nil {
		assert.False(t, settings.WelcomeSettings.ShouldWelcome)
	}

	// --- Pins ---
	_ = pins.GetPinData(chatID)
	require.NoError(t, pins.SetAntiChannelPin(chatID, true))
	require.NoError(t, ClearModuleData(chatID, BackupModulePins))
	assert.False(t, pins.GetPinData(chatID).AntiChannelPin)

	// --- Reports ---
	_ = reports.GetChatReportSettings(chatID)
	require.NoError(t, reports.SetChatReportStatus(chatID, false))
	require.NoError(t, ClearModuleData(chatID, BackupModuleReports))
	assert.True(t, reports.GetChatReportSettings(chatID).Enabled)

	// --- Captcha ---
	_, _ = captcha.GetCaptchaSettings(chatID)
	_ = captcha.SetCaptchaEnabled(chatID, true)
	require.NoError(t, ClearModuleData(chatID, BackupModuleCaptcha))
	captchaSettings, _ := captcha.GetCaptchaSettings(chatID)
	if captchaSettings != nil {
		assert.False(t, captchaSettings.Enabled)
	}

	// --- Antiflood ---
	require.NoError(t, antiflood.SetFlood(chatID, 8))
	require.NoError(t, ClearModuleData(chatID, BackupModuleAntiflood))
	assert.Equal(t, 0, antiflood.GetFlood(chatID).Limit)
}

func TestExportModuleData_EdgeCases(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_export_edge"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// No data exists yet → exports should return non-nil empty structs
	adminData, err := exportAdminData(chatID)
	require.NoError(t, err)
	require.NotNil(t, adminData)

	antifloodData, err := exportAntifloodData(chatID)
	require.NoError(t, err)
	require.NotNil(t, antifloodData)

	blacklistsData, err := exportBlacklistsData(chatID)
	require.NoError(t, err)
	require.NotNil(t, blacklistsData)
	assert.Empty(t, blacklistsData.Entries)

	captchaData, err := exportCaptchaData(chatID)
	require.NoError(t, err)
	require.NotNil(t, captchaData)

	connectionsData, err := exportConnectionsData(chatID)
	require.NoError(t, err)
	require.NotNil(t, connectionsData)

	disablingData, err := exportDisablingData(chatID)
	require.NoError(t, err)
	require.NotNil(t, disablingData)

	filtersData, err := exportFiltersData(chatID)
	require.NoError(t, err)
	require.NotNil(t, filtersData)

	greetingsData, err := exportGreetingsData(chatID)
	require.NoError(t, err)
	require.NotNil(t, greetingsData)

	locksData, err := exportLocksData(chatID)
	require.NoError(t, err)
	require.NotNil(t, locksData)

	notesData, err := exportNotesData(chatID)
	require.NoError(t, err)
	require.NotNil(t, notesData)

	pinsData, err := exportPinsData(chatID)
	require.NoError(t, err)
	require.NotNil(t, pinsData)

	reportsData, err := exportReportsData(chatID)
	require.NoError(t, err)
	require.NotNil(t, reportsData)

	rulesData, err := exportRulesData(chatID)
	require.NoError(t, err)
	require.NotNil(t, rulesData)

	warnsData, err := exportWarnsData(chatID)
	require.NoError(t, err)
	require.NotNil(t, warnsData)
}

func TestExportChatData_Full(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_export_chat_full"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	// Populate multiple modules
	require.NoError(t, admin.SetAnonAdminMode(chatID, true))
	require.NoError(t, antiflood.SetFlood(chatID, 4))
	require.NoError(t, filters.AddFilter(chatID, "hi", "hello", "", nil, db.TEXT))
	rules.SetChatRules(chatID, "Be kind")
	require.NoError(t, captcha.SetCaptchaEnabled(chatID, true))

	backup, err := ExportChatData(chatID, "Test Chat", 1, []string{
		BackupModuleAdmin,
		BackupModuleFilters,
		BackupModuleRules,
		BackupModuleCaptcha,
	})
	require.NoError(t, err)
	require.NotNil(t, backup)
	assert.Equal(t, chatID, backup.ChatID)
	assert.Equal(t, "Test Chat", backup.ChatName)
	assert.Len(t, backup.Modules, 4)

	// Verify data is present
	assert.NotNil(t, backup.Data[BackupModuleAdmin])
	assert.NotNil(t, backup.Data[BackupModuleFilters])
	assert.NotNil(t, backup.Data[BackupModuleRules])
	assert.NotNil(t, backup.Data[BackupModuleCaptcha])
}

func TestExportChatData_EmptyModulesExportsAll(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_export_chat_empty"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	require.NoError(t, antiflood.SetFlood(chatID, 2))
	require.NoError(t, filters.AddFilter(chatID, "a", "b", "", nil, db.TEXT))

	backup, err := ExportChatData(chatID, "Test", 1, nil)
	require.NoError(t, err)
	require.NotNil(t, backup)

	// Should contain all modules (even if some have empty data)
	assert.Equal(t, len(AllExportableModules()), len(backup.Modules))
}

func TestImportChatData_MissingModuleDataSkipped(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_missing"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	backup := NewBackupFormat(chatID, "Test", 1, []string{BackupModuleFilters, BackupModuleNotes})
	// Only provide data for filters
	backup.Data[BackupModuleFilters] = map[string]interface{}{
		"filters": []map[string]interface{}{
			{"chat_id": float64(chatID), "keyword": "k", "filter_reply": "r", "msgtype": float64(db.TEXT)},
		},
	}
	// Notes module has no data in backup

	require.NoError(t, ImportChatData(chatID, backup, nil))
	assert.Len(t, filters.GetFiltersList(chatID), 1)
}
