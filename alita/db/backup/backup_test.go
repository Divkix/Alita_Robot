package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
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
					Data:       map[string]interface{}{"notes": map[string]interface{}{}},
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
		assert.True(t, IsValidModule("rules"))
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
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.AntiRaidSettings{}).Error; err != nil {
		t.Errorf("cleanup failed deleting AntiRaidSettings: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ApprovedUsers{}).Error; err != nil {
		t.Errorf("cleanup failed deleting ApprovedUsers: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{}).Error; err != nil {
		t.Errorf("cleanup failed deleting Warns: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Reactions{}).Error; err != nil {
		t.Errorf("cleanup failed deleting Reactions: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error; err != nil {
		t.Errorf("cleanup failed deleting FederationChat: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.LogChannel{}).Error; err != nil {
		t.Errorf("cleanup failed deleting LogChannel: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
		t.Errorf("cleanup failed deleting Chat: %v", err)
	}
}

func TestImportAdminData_InvalidFormat(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_admin_invalid"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	err := ImportModuleData(chatID, BackupModuleAdmin, "not a map")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid admin data format")
}

func TestImportFiltersData_InvalidFormat(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_filters_invalid"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	err := ImportModuleData(chatID, BackupModuleFilters, "not a map")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filters data format")
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

func TestClearChatData_SpecificModules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_clear_specific"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	require.NoError(t, filters.AddFilter(chatID, "hello", "hi", "", nil, db.TEXT))
	require.NoError(t, antiflood.SetFlood(chatID, 5))
	rules.SetChatRules(chatID, "rules text")

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

	require.NoError(t, filters.AddFilter(chatID, "hello", "hi", "", nil, db.TEXT))
	require.NoError(t, blacklists.AddBlacklist(chatID, "bad"))
	require.NoError(t, antiflood.SetFlood(chatID, 5))
	rules.SetChatRules(chatID, "rules text")
	require.NoError(t, captcha.SetCaptchaEnabled(chatID, true))
	_ = pins.GetPinData(chatID)
	require.NoError(t, pins.SetAntiChannelPin(chatID, true))
	_ = reports.GetChatReportSettings(chatID)
	_ = admin.GetAdminSettings(chatID)

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
	assert.ErrorContains(t, err, "unknown module")
}

func TestClearModuleData_IndividualModules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_clear_individual"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	require.NoError(t, filters.AddFilter(chatID, "f", "r", "", nil, db.TEXT))
	require.NoError(t, ClearModuleData(chatID, BackupModuleFilters))
	assert.Empty(t, filters.GetFiltersList(chatID))

	require.NoError(t, blacklists.AddBlacklist(chatID, "badword"))
	require.NoError(t, ClearModuleData(chatID, BackupModuleBlacklists))
	assert.Empty(t, blacklists.GetBlacklistSettings(chatID))

	require.NoError(t, notes.AddNote(chatID, "n1", "c1", "", nil, db.TEXT, false, false, false, true, false, false))
	require.NoError(t, ClearModuleData(chatID, BackupModuleNotes))
	assert.Empty(t, notes.GetNotesList(chatID, true))

	rules.SetChatRules(chatID, "some rules")
	require.NoError(t, ClearModuleData(chatID, BackupModuleRules))
	assert.Equal(t, "", rules.GetChatRulesInfo(chatID).Rules)

	require.NoError(t, warns.SetWarnLimit(chatID, 10))
	require.NoError(t, warns.SetWarnMode(chatID, "ban"))
	require.NoError(t, ClearModuleData(chatID, BackupModuleWarns))
	assert.Equal(t, 3, warns.GetWarnSetting(chatID).WarnLimit)
	assert.Equal(t, "", warns.GetWarnSetting(chatID).WarnMode)

	require.NoError(t, locks.UpdateLock(chatID, " stickers", true))
	require.NoError(t, ClearModuleData(chatID, BackupModuleLocks))
	assert.False(t, locks.GetChatLocks(chatID)[" stickers"])

	require.NoError(t, greetings.SetWelcomeToggle(chatID, true))
	require.NoError(t, ClearModuleData(chatID, BackupModuleGreetings))
	settings := greetings.GetGreetingSettings(chatID)
	if settings != nil && settings.WelcomeSettings != nil {
		assert.False(t, settings.WelcomeSettings.ShouldWelcome)
	}

	_ = pins.GetPinData(chatID)
	require.NoError(t, pins.SetAntiChannelPin(chatID, true))
	require.NoError(t, ClearModuleData(chatID, BackupModulePins))
	assert.False(t, pins.GetPinData(chatID).AntiChannelPin)

	_ = reports.GetChatReportSettings(chatID)
	require.NoError(t, reports.SetChatReportStatus(chatID, false))
	require.NoError(t, ClearModuleData(chatID, BackupModuleReports))
	assert.True(t, reports.GetChatReportSettings(chatID).Enabled)

	_, _ = captcha.GetCaptchaSettings(chatID)
	_ = captcha.SetCaptchaEnabled(chatID, true)
	require.NoError(t, ClearModuleData(chatID, BackupModuleCaptcha))
	captchaSettings, _ := captcha.GetCaptchaSettings(chatID)
	if captchaSettings != nil {
		assert.False(t, captchaSettings.Enabled)
	}

	require.NoError(t, antiflood.SetFlood(chatID, 8))
	require.NoError(t, ClearModuleData(chatID, BackupModuleAntiflood))
	assert.Equal(t, 0, antiflood.GetFlood(chatID).Limit)

	require.NoError(t, antiraid.SetRaidTime(chatID, 60))
	require.NoError(t, antiraid.SetRaidActionTime(chatID, 30))
	require.NoError(t, antiraid.SetAutoAntiRaidThreshold(chatID, 5))
	require.NoError(t, ClearModuleData(chatID, BackupModuleAntiraid))
	cleared := antiraid.GetAntiRaidSettings(chatID)
	assert.Equal(t, 21600, cleared.RaidTime)
	assert.Equal(t, 3600, cleared.RaidActionTime)
	assert.Equal(t, 0, cleared.AutoAntiRaidThreshold)
}

func TestExportModuleData_EdgeCases(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_export_edge"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

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

	assert.NotNil(t, backup.Data[BackupModuleAdmin])
	assert.NotNil(t, backup.Data[BackupModuleFilters])
	assert.NotNil(t, backup.Data[BackupModuleRules])
	assert.NotNil(t, backup.Data[BackupModuleCaptcha])
}

func TestImportChatData_RejectsMissingLegacyModuleData(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "test_import_missing"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })

	backup := NewBackupFormat(chatID, "Test", 1, []string{BackupModuleFilters, BackupModuleNotes})
	backup.Version = legacyFormatVersion
	backup.Data[BackupModuleFilters] = map[string]interface{}{
		"filters": []map[string]interface{}{
			{"chat_id": float64(chatID), "keyword": "k", "filter_reply": "r", "msgtype": float64(db.TEXT)},
		},
	}

	err := ImportChatData(chatID, backup, nil)
	require.ErrorContains(t, err, "missing data for module: notes")
	assert.Empty(t, filters.GetFiltersList(chatID))
}
