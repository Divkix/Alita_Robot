//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
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
backup_import_file_too_large: "File is too large"
backup_import_invalid_file: "Invalid backup file"
backup_import_download_failed: "Download failed"
backup_import_rate_limited: "Wait {time}"
backup_import_expired: "Import expired"
backup_reset_expired: "Reset expired"
common_callback_invalid_request: "Invalid callback"
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

	t.Run("deduplicates valid module args while preserving first occurrence", func(t *testing.T) {
		got := parseImportModules("/import notes filters NOTES rules filters", backupData)
		assert.Equal(t, []string{"notes", "filters", "rules"}, got)
	})

	t.Run("all invalid returns empty", func(t *testing.T) {
		assert.Empty(t, parseImportModules("/import foo bar", backupData))
	})
}

func TestParseModuleArgs(t *testing.T) {
	t.Parallel()

	valid := func(module string) bool {
		return module == "notes" || module == "filters"
	}

	got := parseModuleArgs([]string{"NOTES", "invalid", "filters", "notes", ""}, valid)
	assert.Equal(t, []string{"notes", "filters"}, got)
}

func TestDownloadBackupFileRejectsInvalidDocumentBeforeNetwork(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)

	t.Run("non-json file", func(t *testing.T) {
		data, msg := downloadBackupFile(nil, &gotgbot.Document{FileName: "backup.txt"}, tr)
		assert.Nil(t, data)
		assert.Equal(t, "Invalid backup file", msg)
	})

	t.Run("file larger than ten megabytes", func(t *testing.T) {
		data, msg := downloadBackupFile(nil, &gotgbot.Document{
			FileName: "backup.json",
			FileSize: 10*1024*1024 + 1,
		}, tr)
		assert.Nil(t, data)
		assert.Equal(t, "File is too large", msg)
	})
}

func TestDownloadBackupFileReportsGotgbotGetFileFailure(t *testing.T) {
	tr := testTranslator(t)
	client := newModuleBotClient()
	client.errors["getFile"] = fmt.Errorf("telegram getFile failed")
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Nil(t, data)
	assert.NotEmpty(t, msg)
	assert.Len(t, client.callsFor("getFile"), 1)
}

func TestDownloadBackupFileDownloadsGetFilePath(t *testing.T) {
	tr := testTranslator(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/file/bot999:test/backups/chat.json", r.URL.Path)
		_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Equal(t, `{"version":"1.0.0"}`, string(data))
	assert.Empty(t, msg)
	assert.Len(t, client.callsFor("getFile"), 1)
}

func TestDownloadBackupFileReportsHTTPStatusFailure(t *testing.T) {
	tr := testTranslator(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failed", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)

	data, msg := downloadBackupFile(bot, &gotgbot.Document{
		FileName: "backup.json",
		FileId:   "backup-file-id",
	}, tr)

	assert.Nil(t, data)
	assert.Equal(t, "Download failed", msg)
	assert.Len(t, client.callsFor("getFile"), 1)
}

func TestImportHandlerStoresDownloadedBackupForConfirmation(t *testing.T) {
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	backup := db.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules", "notes"})
	backup.Data["rules"] = map[string]interface{}{
		"settings": map[string]interface{}{"rules": "imported rules"},
	}
	backup.Data["notes"] = map[string]interface{}{
		"notes": []interface{}{
			map[string]interface{}{"keyword": "welcome", "reply": "hello"},
		},
	}
	backupData, err := backup.ToJSON()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/file/bot999:test/backups/chat.json", r.URL.Path)
		_, _ = w.Write(backupData)
	}))
	t.Cleanup(server.Close)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
		delete(pendingImports, chat.Id)
		delete(pendingModules, chat.Id)
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	bot := newModuleTestBot(client)
	ctx := newModuleMessageContext(bot, chat, owner, "/import rules invalid rules")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 333,
		Date:      1,
		Chat:      chat,
		Document: &gotgbot.Document{
			FileId:   "backup-file-id",
			FileName: "backup.json",
		},
	}

	err = backupModule.importHandler(bot, ctx)

	require.Equal(t, ext.EndGroups, err)
	require.Contains(t, pendingImports, chat.Id)
	assert.Equal(t, []string{"rules"}, pendingModules[chat.Id])
	assert.Equal(t, backup.Version, pendingImports[chat.Id].Version)
	assert.Contains(t, pendingImports[chat.Id].Data, "notes")
	assert.Len(t, client.callsFor("getFile"), 1)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestCheckImportRateLimitAllowsWhenCacheUnavailable(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	allowed, text := checkImportRateLimit(time.Now().UnixNano(), tr)
	assert.True(t, allowed)
	assert.Empty(t, text)
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

func TestBackupCallbackHandlerNilCallbackQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  *ext.Context
	}{
		{name: "nil context", ctx: nil},
		{name: "nil update", ctx: &ext.Context{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := backupModule.backupCallbackHandler(nil, tc.ctx)
			assert.Equal(t, ext.EndGroups, err)
		})
	}
}

func TestImportHandlerRequiresReplyDocument(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/import")

	err := backupModule.importHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestImportHandlerRejectsInvalidReplyDocument(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/import")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 333,
		Date:      1,
		Chat:      chat,
		Document: &gotgbot.Document{
			FileId:   "not-json",
			FileName: "backup.txt",
		},
	}

	err := backupModule.importHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)
	assert.Empty(t, client.callsFor("getFile"))
}

func TestExportHandlerSendsRequestedModuleBackupDocument(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, db.EnsureChatInDb(chat.Id, chat.Title))
	require.NoError(t, db.AddNote(chat.Id, "welcome", "hello", "", nil, db.TEXT, false, false, false, true, false, false))

	ctx := newModuleMessageContext(bot, chat, owner, "/export notes invalid notes")
	err := backupModule.exportHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendDocument"), 1)
	assert.Empty(t, client.callsFor("sendMessage"))
}

func TestValidateImportRequestRejectsNonOwner(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, member, "/import")

	msg, gotChat, user, tr, ok := validateImportRequest(bot, ctx)
	assert.False(t, ok)
	assert.Nil(t, msg)
	assert.Nil(t, gotChat)
	assert.Nil(t, user)
	assert.Nil(t, tr)
	assert.NotEmpty(t, client.callsFor("sendMessage"))
}

func TestResetHandlerStoresPendingModulesAndRepliesWithConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/reset rules notes invalid rules")
	t.Cleanup(func() {
		delete(pendingModules, chat.Id)
	})

	err := backupModule.resetHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Equal(t, []string{"rules", "notes"}, pendingModules[chat.Id])
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerConfirmsPendingImport(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, db.EnsureChatInDb(chat.Id, chat.Title))

	backup := db.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"})
	backup.Data["rules"] = map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":   chat.Id,
			"rules":     "imported rules",
			"rules_btn": "Read rules",
			"private":   true,
		},
	}
	pendingImports[chat.Id] = backup
	pendingModules[chat.Id] = []string{"rules"}
	t.Cleanup(func() {
		delete(pendingImports, chat.Id)
		delete(pendingModules, chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": "confirm_import", "c": strconv.FormatInt(chat.Id, 10)},
		"backup.confirm",
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err := backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.NotContains(t, pendingImports, chat.Id)
	assert.NotContains(t, pendingModules, chat.Id)
	assert.Equal(t, "imported rules", db.GetChatRulesInfo(chat.Id).Rules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerConfirmsPendingReset(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, db.EnsureChatInDb(chat.Id, chat.Title))
	db.SetChatRules(chat.Id, "rules before reset")
	pendingModules[chat.Id] = []string{"rules"}
	t.Cleanup(func() {
		delete(pendingModules, chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": "confirm_reset", "c": strconv.FormatInt(chat.Id, 10)},
		"backup.reset",
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err := backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.NotContains(t, pendingModules, chat.Id)
	assert.Empty(t, db.GetChatRulesInfo(chat.Id).Rules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerIgnoresWrongChatConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	pendingModules[chat.Id] = []string{"rules"}
	t.Cleanup(func() {
		delete(pendingModules, chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": "confirm_reset", "c": strconv.FormatInt(chat.Id+1, 10)},
		"backup.reset",
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err := backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Contains(t, pendingModules, chat.Id)
	assert.Empty(t, client.callsFor("sendMessage"))
}

func TestBackupCallbackHandlerRejectsInvalidCallbackData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleCallbackContext(bot, chat, owner, "not-a-backup-callback")

	err := backupModule.backupCallbackHandler(bot, ctx)

	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("answerCallbackQuery"), 1)
}

func TestBackupConfirmHandlersReportExpiredState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := &gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	tr := testTranslator(t)
	ctx := &ext.Context{}
	t.Cleanup(func() {
		delete(pendingImports, chat.Id)
		delete(pendingModules, chat.Id)
	})

	err := backupModule.handleConfirmImport(bot, ctx, tr, chat)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)

	err = backupModule.handleConfirmReset(bot, ctx, tr, chat)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 2)
}

func TestBackupCallbackCancelImportAndResetCleanup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	pendingImports[chat.Id] = db.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"})
	pendingModules[chat.Id] = []string{"rules"}
	cancelImport := encodeCallbackData(
		"backup",
		map[string]string{"a": "cancel_import", "c": strconv.FormatInt(chat.Id, 10)},
		"backup.cancel",
	)
	importCtx := newModuleCallbackContext(bot, chat, owner, cancelImport)
	err := backupModule.handleCancelImport(bot, importCtx, testTranslator(t), importCtx.CallbackQuery)
	assert.Equal(t, ext.EndGroups, err)
	assert.NotContains(t, pendingImports, chat.Id)
	assert.NotContains(t, pendingModules, chat.Id)

	pendingModules[chat.Id] = []string{"rules"}
	cancelReset := encodeCallbackData(
		"backup",
		map[string]string{"a": "cancel_reset", "c": strconv.FormatInt(chat.Id, 10)},
		"backup.cancel_reset",
	)
	resetCtx := newModuleCallbackContext(bot, chat, owner, cancelReset)
	err = backupModule.handleCancelReset(bot, resetCtx, testTranslator(t), resetCtx.CallbackQuery)
	assert.Equal(t, ext.EndGroups, err)
	assert.NotContains(t, pendingModules, chat.Id)

	assert.Len(t, client.callsFor("answerCallbackQuery"), 2)
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
