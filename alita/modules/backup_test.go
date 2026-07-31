//go:build testtools

package modules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/backup"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

type backupRoundTripFunc func(*http.Request) (*http.Response, error)

func (f backupRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBackupModuleStructure(t *testing.T) {
	t.Run("backupModule has correct name", func(t *testing.T) {
		assert.Equal(t, "Backup", backupModule.moduleName)
	})
}

func TestLoadBackupDoesNotRegisterDeadHelpButtons(t *testing.T) {
	registry := DefaultHelpRegistry()
	previousButtons, hadButtons := registry.helpableKb[backupModule.moduleName]
	previousEnabled, hadEnabled := registry.AbleMap[backupModule.moduleName]
	delete(registry.helpableKb, backupModule.moduleName)
	delete(registry.AbleMap, backupModule.moduleName)
	t.Cleanup(func() {
		if hadButtons {
			registry.helpableKb[backupModule.moduleName] = previousButtons
		} else {
			delete(registry.helpableKb, backupModule.moduleName)
		}
		if hadEnabled {
			registry.AbleMap[backupModule.moduleName] = previousEnabled
		} else {
			delete(registry.AbleMap, backupModule.moduleName)
		}
	})

	LoadBackup(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))

	assert.True(t, registry.AbleMap[backupModule.moduleName])
	_, ok := registry.helpableKb[backupModule.moduleName]
	assert.False(t, ok)
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
		got, err := parseImportModules("", backupData)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("no args returns empty slice", func(t *testing.T) {
		got, err := parseImportModules("/import", backupData)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("valid modules only", func(t *testing.T) {
		got, err := parseImportModules("/import notes filters", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", "filters"}, got)
	})

	t.Run("invalid modules skipped", func(t *testing.T) {
		got, err := parseImportModules("/import notes invalid rules", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", "rules"}, got)
	})

	t.Run("case insensitive", func(t *testing.T) {
		got, err := parseImportModules("/import NOTES", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes"}, got)
	})

	t.Run("deduplicates valid module args while preserving first occurrence", func(t *testing.T) {
		got, err := parseImportModules("/import notes filters NOTES rules filters", backupData)
		require.NoError(t, err)
		assert.Equal(t, []string{"notes", "filters", "rules"}, got)
	})

	t.Run("all invalid returns error", func(t *testing.T) {
		got, err := parseImportModules("/import foo bar", backupData)
		require.ErrorIs(t, err, errNoValidModule)
		assert.Nil(t, got)
	})
}

func TestParseModuleArgs(t *testing.T) {
	t.Parallel()

	valid := func(module string) bool {
		return module == "notes" || module == "filters"
	}

	got, err := parseModuleArgs([]string{"NOTES", "invalid", "filters", "notes", ""}, valid)
	require.NoError(t, err)
	assert.Equal(t, []string{"notes", "filters"}, got)

	_, err = parseModuleArgs([]string{"invalid", ""}, valid)
	require.ErrorIs(t, err, errNoValidModule)
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

func TestDownloadBackupFileLimitsResponseBody(t *testing.T) {
	tr := testTranslator(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxBackupFileSize+1))
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
	assert.Equal(t, "File is too large", msg)
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
	backup := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules", "notes"})
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
		clearPendingImport(chat.Id)
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
	gotBackup, gotModules, ok := getPendingImport(chat.Id)
	require.True(t, ok)
	assert.Equal(t, []string{"rules"}, gotModules)
	assert.Equal(t, backup.Version, gotBackup.Version)
	assert.Contains(t, gotBackup.Data, "notes")
	assert.Len(t, client.callsFor("getFile"), 1)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestImportHandlerClearsPendingWhenConfirmationFails(t *testing.T) {
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	bkp := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"})
	bkp.Data["rules"] = map[string]interface{}{"settings": map[string]interface{}{"rules": "test"}}
	backupData, err := bkp.ToJSON()
	require.NoError(t, err)

	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = "https://example.invalid/file/bot"
	backupDownloadHTTPClient = &http.Client{Transport: backupRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(backupData)),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
		clearPendingImport(chat.Id)
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"backup-file-id","file_path":"backups/chat.json"}`,
	)
	client.errors["sendMessage"] = errors.New("confirmation send failed")
	bot := newModuleTestBot(client)
	ctx := newModuleMessageContext(bot, chat, owner, "/import")
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
	_, _, ok := getPendingImport(chat.Id)
	assert.False(t, ok)
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
	backup := backup.NewBackupFormat(12345, "Test Chat", 67890, []string{"notes", "filters"})
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
	chatID := int64(-9223372036854775807 - 1)
	token := "0123456789abcdef"
	keyboard := buildImportKeyboard(tr, chatID, token)

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
	assert.Equal(t, "ci", action)
	chatIDStr, _ := decodedConfirm.Field("c")
	assert.Equal(t, "-9223372036854775808", chatIDStr)
	gotToken, _ := decodedConfirm.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(confirmBtn.CallbackData), 64)

	decodedCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ = decodedCancel.Field("a")
	assert.Equal(t, "xi", action)
	gotToken, _ = decodedCancel.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(cancelBtn.CallbackData), 64)
}

func TestBuildResetKeyboard(t *testing.T) {
	t.Parallel()

	tr := testTranslator(t)
	chatID := int64(-9223372036854775807 - 1)
	token := "0123456789abcdef"
	keyboard := buildResetKeyboard(tr, chatID, token)

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
	assert.Equal(t, "cr", action)
	chatIDStr, _ := decodedConfirm.Field("c")
	assert.Equal(t, "-9223372036854775808", chatIDStr)
	gotToken, _ := decodedConfirm.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(confirmBtn.CallbackData), 64)

	decodedCancel, ok := decodeCallbackData(cancelBtn.CallbackData, "backup")
	require.True(t, ok)
	action, _ = decodedCancel.Field("a")
	assert.Equal(t, "xr", action)
	gotToken, _ = decodedCancel.Field("t")
	assert.Equal(t, token, gotToken)
	assert.LessOrEqual(t, len(cancelBtn.CallbackData), 64)
}

func TestPendingImportsMaps(t *testing.T) {
	t.Run("pending imports maps exist", func(t *testing.T) {
		// Just verify the maps are initialized
		assert.NotNil(t, pendingImports)
		assert.NotNil(t, pendingResets)
	})
}

func TestPendingImportRejectsStaleTokenAndConsumesOnce(t *testing.T) {
	chatID := uniqueModuleChatID()
	t.Cleanup(func() { clearPendingImport(chatID) })

	oldToken, err := storePendingImport(chatID, &backup.BackupFormat{}, []string{"rules"})
	require.NoError(t, err)
	current := &backup.BackupFormat{}
	currentToken, err := storePendingImport(chatID, current, []string{"notes"})
	require.NoError(t, err)
	require.NotEqual(t, oldToken, currentToken)

	_, _, ok := consumePendingImport(chatID, oldToken)
	assert.False(t, ok)

	var consumed atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, modules, ok := consumePendingImport(chatID, currentToken)
			if ok {
				assert.Same(t, current, got)
				assert.Equal(t, []string{"notes"}, modules)
				consumed.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), consumed.Load())

	expiredToken, err := storePendingImport(chatID, current, []string{"notes"})
	require.NoError(t, err)
	pendingMu.Lock()
	expired := pendingImports[chatID]
	expired.expiresAt = time.Now().Add(-time.Second)
	pendingImports[chatID] = expired
	pendingMu.Unlock()
	_, _, ok = consumePendingImport(chatID, expiredToken)
	assert.False(t, ok)
}

func TestPendingResetRejectsStaleAndExpiredTokens(t *testing.T) {
	chatID := uniqueModuleChatID()
	t.Cleanup(func() { clearPendingReset(chatID) })

	oldToken, err := storePendingReset(chatID, []string{"rules"})
	require.NoError(t, err)
	currentToken, err := storePendingReset(chatID, []string{"notes"})
	require.NoError(t, err)
	require.NotEqual(t, oldToken, currentToken)

	assert.False(t, discardPendingReset(chatID, oldToken))
	modules, ok := getPendingReset(chatID)
	require.True(t, ok)
	assert.Equal(t, []string{"notes"}, modules)

	pendingMu.Lock()
	pending := pendingResets[chatID]
	pending.expiresAt = time.Now().Add(-time.Second)
	pendingResets[chatID] = pending
	pendingMu.Unlock()

	_, ok = consumePendingReset(chatID, currentToken)
	assert.False(t, ok)
	_, ok = getPendingReset(chatID)
	assert.False(t, ok)
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
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	require.NoError(t, notes.AddNote(chat.Id, "welcome", "hello", "", nil, db.TEXT, false, false, false, true, false, false))

	ctx := newModuleMessageContext(bot, chat, owner, "/export notes invalid notes")
	err := backupModule.exportHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendDocument"), 1)
	assert.Empty(t, client.callsFor("sendMessage"))
}

func TestExportHandlerFallsBackToTextWhenDocumentSendFails(t *testing.T) {
	client := newModuleBotClient()
	client.errors["sendDocument"] = fmt.Errorf("document upload failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	require.NoError(t, notes.AddNote(chat.Id, "fallback", "hello", "", nil, db.TEXT, false, false, false, true, false, false))

	ctx := newModuleMessageContext(bot, chat, owner, "/export notes")
	err := backupModule.exportHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendDocument"), 1)
	assert.Len(t, client.callsFor("sendMessage"), 1)
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
		clearPendingReset(chat.Id)
	})

	err := backupModule.resetHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	gotModules, ok := getPendingReset(chat.Id)
	assert.True(t, ok)
	assert.Equal(t, []string{"rules", "notes"}, gotModules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestResetHandlerClearsPendingWhenConfirmationFails(t *testing.T) {
	client := newModuleBotClient()
	client.errors["sendMessage"] = errors.New("confirmation send failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, owner, "/reset rules")
	t.Cleanup(func() { clearPendingReset(chat.Id) })

	err := backupModule.resetHandler(bot, ctx)

	require.Equal(t, ext.EndGroups, err)
	_, ok := getPendingReset(chat.Id)
	assert.False(t, ok)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerConfirmsPendingImport(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))

	backup := backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"})
	backup.Data["rules"] = map[string]interface{}{
		"settings": map[string]interface{}{
			"chat_id":   chat.Id,
			"rules":     "imported rules",
			"rules_btn": "Read rules",
			"private":   true,
		},
	}
	token, err := storePendingImport(chat.Id, backup, []string{"rules"})
	require.NoError(t, err)
	t.Cleanup(func() {
		clearPendingImport(chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionConfirmImport, "c": strconv.FormatInt(chat.Id, 10), "t": token},
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err = backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	_, _, ok := getPendingImport(chat.Id)
	assert.False(t, ok)
	assert.Equal(t, "imported rules", rules.GetChatRulesInfo(chat.Id).Rules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerConfirmsPendingReset(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	rules.SetChatRules(chat.Id, "rules before reset")
	token, err := storePendingReset(chat.Id, []string{"rules"})
	require.NoError(t, err)
	t.Cleanup(func() {
		clearPendingReset(chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionConfirmReset, "c": strconv.FormatInt(chat.Id, 10), "t": token},
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err = backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	_, ok := getPendingReset(chat.Id)
	assert.False(t, ok)
	assert.Empty(t, rules.GetChatRulesInfo(chat.Id).Rules)
	assert.Len(t, client.callsFor("sendMessage"), 1)
}

func TestBackupCallbackHandlerIgnoresWrongChatConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	token, err := storePendingReset(chat.Id, []string{"rules"})
	require.NoError(t, err)
	t.Cleanup(func() {
		clearPendingReset(chat.Id)
	})

	callback := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionConfirmReset, "c": strconv.FormatInt(chat.Id+1, 10), "t": token},
	)
	ctx := newModuleCallbackContext(bot, chat, owner, callback)
	err = backupModule.backupCallbackHandler(bot, ctx)
	assert.Equal(t, ext.EndGroups, err)
	_, ok := getPendingReset(chat.Id)
	assert.True(t, ok)
	assert.Empty(t, client.callsFor("sendMessage"))
	assert.Len(t, client.callsFor("answerCallbackQuery"), 1)
}

func TestBackupCallbackHandlerRejectsInvalidCallbackData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	for _, data := range []string{
		"not-a-backup-callback",
		encodeCallbackData("backup", map[string]string{
			"a": "crafted",
			"c": strconv.FormatInt(chat.Id, 10),
			"t": "token",
		}),
		encodeCallbackData("backup", map[string]string{
			"a": backupActionConfirmReset,
			"c": "not-a-chat",
			"t": "token",
		}),
		encodeCallbackData("backup", map[string]string{
			"a": backupActionConfirmReset,
			"c": strconv.FormatInt(chat.Id, 10),
		}),
	} {
		ctx := newModuleCallbackContext(bot, chat, owner, data)
		assert.Equal(t, ext.EndGroups, backupModule.backupCallbackHandler(bot, ctx))
	}
	assert.Len(t, client.callsFor("answerCallbackQuery"), 4)
}

func TestBackupConfirmHandlersReportExpiredState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := &gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	tr := testTranslator(t)
	ctx := &ext.Context{}
	t.Cleanup(func() {
		clearPendingImport(chat.Id)
		clearPendingReset(chat.Id)
	})

	err := backupModule.handleConfirmImport(bot, ctx, tr, chat, "")
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 1)

	err = backupModule.handleConfirmReset(bot, ctx, tr, chat, "")
	assert.Equal(t, ext.EndGroups, err)
	assert.Len(t, client.callsFor("sendMessage"), 2)
}

func TestBackupCallbackCancelImportAndResetCleanup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Backup Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	importToken, err := storePendingImport(chat.Id, backup.NewBackupFormat(chat.Id, chat.Title, owner.Id, []string{"rules"}), []string{"rules"})
	require.NoError(t, err)
	cancelImport := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionCancelImport, "c": strconv.FormatInt(chat.Id, 10), "t": importToken},
	)
	importCtx := newModuleCallbackContext(bot, chat, owner, cancelImport)
	err = backupModule.handleCancelImport(bot, importCtx, testTranslator(t), importCtx.CallbackQuery, importToken)
	assert.Equal(t, ext.EndGroups, err)
	_, _, ok := getPendingImport(chat.Id)
	assert.False(t, ok)

	resetToken, err := storePendingReset(chat.Id, []string{"rules"})
	require.NoError(t, err)
	cancelReset := encodeCallbackData(
		"backup",
		map[string]string{"a": backupActionCancelReset, "c": strconv.FormatInt(chat.Id, 10), "t": resetToken},
	)
	resetCtx := newModuleCallbackContext(bot, chat, owner, cancelReset)
	err = backupModule.handleCancelReset(bot, resetCtx, testTranslator(t), resetCtx.CallbackQuery, resetToken)
	assert.Equal(t, ext.EndGroups, err)
	_, ok = getPendingReset(chat.Id)
	assert.False(t, ok)

	assert.Len(t, client.callsFor("answerCallbackQuery"), 2)
}

func TestModuleNames(t *testing.T) {
	t.Run("all module names are lowercase", func(t *testing.T) {
		modules := []string{
			backup.BackupModuleAdmin,
			backup.BackupModuleAntiflood,
			backup.BackupModuleAntiraid,
			backup.BackupModuleApprovals,
			backup.BackupModuleBlacklists,
			backup.BackupModuleCaptcha,
			backup.BackupModuleConnections,
			backup.BackupModuleDisabling,
			backup.BackupModuleFilters,
			backup.BackupModuleGreetings,
			backup.BackupModuleLocks,
			backup.BackupModuleNotes,
			backup.BackupModulePins,
			backup.BackupModuleReports,
			backup.BackupModuleRules,
			backup.BackupModuleWarns,
		}

		for _, module := range modules {
			assert.Equal(t, module, module) // Just checking they exist
		}
	})
}
