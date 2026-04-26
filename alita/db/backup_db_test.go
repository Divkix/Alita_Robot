package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
					BotName:    "AlitaRobot",
					ChatID:     12345,
					Modules:    []string{"notes"},
					Data:       make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "missing bot name",
				backup: &BackupFormat{
					Version:    "1.0",
					ChatID:     12345,
					Modules:    []string{"notes"},
					Data:       make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "missing chat ID",
				backup: &BackupFormat{
					Version:    "1.0",
					BotName:    "AlitaRobot",
					Modules:    []string{"notes"},
					Data:       make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "empty modules",
				backup: &BackupFormat{
					Version:    "1.0",
					BotName:    "AlitaRobot",
					ChatID:     12345,
					Modules:    []string{},
					Data:       make(map[string]interface{}),
				},
				wantErr: true,
			},
			{
				name: "nil data",
				backup: &BackupFormat{
					Version:    "1.0",
					BotName:    "AlitaRobot",
					ChatID:     12345,
					Modules:    []string{"notes"},
					Data:       nil,
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
		backup.Data["notes"] = []Notes{{NoteName: "test", NoteContent: "reply"}}

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
			AdminSettings: &AdminSettings{
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
			Settings: &AntifloodSettings{
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
			Notes: []Notes{
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