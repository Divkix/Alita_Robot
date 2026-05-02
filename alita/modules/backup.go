package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

// Module instance
var backupModule = moduleStruct{
	moduleName: "Backup",
}

// Pending imports storage (in-memory, per chat)
var (
	pendingImports = make(map[int64]*db.BackupFormat)
	pendingModules = make(map[int64][]string)
)

// exportHandler handles the /export command
func (m moduleStruct) exportHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)

	if user == nil {
		return ext.EndGroups
	}

	// Check if in a group
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// Check if user is admin
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	// Check if bot is admin
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check rate limiting
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.CanExport(chat.Id); !allowed {
		cooldownStr := ratelimit.FormatCooldown(cooldown)
		text, _ := tr.GetString("backup_export_rate_limited", i18n.TranslationParams{
			"time": cooldownStr,
		})
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Parse module arguments
	var modules []string
	if msg.Text != "" {
		args := strings.Fields(msg.Text)
		if len(args) > 1 {
			// User specified specific modules
			for _, arg := range args[1:] {
				if db.IsValidModule(strings.ToLower(arg)) {
					modules = append(modules, strings.ToLower(arg))
				}
			}
		}
	}

	// Export data
	backup, err := db.ExportChatData(chat.Id, chat.Title, user.Id, modules)
	if err != nil {
		log.Errorf("[Backup] Export failed for chat %d: %v", chat.Id, err)
		text, _ := tr.GetString("backup_export_failed")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Check if any data was exported
	if len(backup.Data) == 0 {
		text, _ := tr.GetString("backup_export_no_modules")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Convert to JSON
	jsonData, err := backup.ToJSON()
	if err != nil {
		log.Errorf("[Backup] Failed to marshal backup: %v", err)
		text, _ := tr.GetString("backup_export_failed")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Send as document
	fileName := fmt.Sprintf("alita_backup_%d_%s.json", chat.Id, time.Now().Format("20060102_150405"))
	caption := buildExportCaption(tr, backup)

	_, err = b.SendDocument(
		chat.Id,
		gotgbot.InputFileByReader(fileName, bytes.NewReader(jsonData)),
		&gotgbot.SendDocumentOpts{
			Caption:         caption,
			ParseMode:       "HTML",
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
		},
	)
	if err != nil {
		log.Errorf("[Backup] Failed to send document: %v", err)
		// Fallback: send as text
		text, _ := tr.GetString("backup_export_success_text", i18n.TranslationParams{
			"modules": fmt.Sprintf("%d", len(backup.Data)),
		})
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Record rate limit
	limiter.RecordExport(chat.Id)

	log.Infof("[Backup] Chat %d exported %d modules", chat.Id, len(backup.Data))
	return ext.EndGroups
}

// validateImportRequest checks all permissions and prerequisites for import
func validateImportRequest(b *gotgbot.Bot, ctx *ext.Context) (*gotgbot.Message, *gotgbot.Chat, *gotgbot.User, *i18n.Translator, bool) {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)

	if user == nil {
		return nil, nil, nil, nil, false
	}

	// Check if in a group
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return nil, nil, nil, nil, false
	}

	// Check if bot is admin
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return nil, nil, nil, nil, false
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check if user is the group creator
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		text, _ := tr.GetString("backup_import_creator_only")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return nil, nil, nil, nil, false
	}

	return msg, chat, user, tr, true
}

// checkImportRateLimit checks rate limit for import operation
func checkImportRateLimit(chatID int64, tr *i18n.Translator) (bool, string) {
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.CanImport(chatID); !allowed {
		cooldownStr := ratelimit.FormatCooldown(cooldown)
		text, _ := tr.GetString("backup_import_rate_limited", i18n.TranslationParams{
			"time": cooldownStr,
		})
		return false, text
	}
	return true, ""
}

// downloadBackupFile downloads the backup file from Telegram
func downloadBackupFile(b *gotgbot.Bot, doc *gotgbot.Document, tr *i18n.Translator) ([]byte, string) {
	// Check file type
	if !strings.HasSuffix(strings.ToLower(doc.FileName), ".json") {
		text, _ := tr.GetString("backup_import_invalid_file")
		return nil, text
	}

	// Max 10MB
	if doc.FileSize > 10*1024*1024 {
		text, _ := tr.GetString("backup_import_file_too_large")
		return nil, text
	}

	// Get the file
	file, err := b.GetFile(doc.FileId, &gotgbot.GetFileOpts{})
	if err != nil {
		log.Errorf("[Backup] Failed to get file: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}

	// Download file content
	downloadURL, err := url.Parse(
		fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.Token, file.FilePath),
	)
	if err != nil {
		log.Errorf("[Backup] Failed to parse download URL: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}
	if downloadURL.Scheme != "https" || downloadURL.Host != "api.telegram.org" {
		log.Errorf("[Backup] Unexpected download URL host: %s", downloadURL.Host)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		log.Errorf("[Backup] Failed to create download request: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("[Backup] Failed to download file: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}
	defer func() { _ = resp.Body.Close() }()

	fileData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[Backup] Failed to read file: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}

	return fileData, ""
}

// parseImportModules parses module arguments from command text
func parseImportModules(text string, backupData map[string]interface{}) []string {
	var importModules []string
	if text != "" {
		args := strings.Fields(text)
		if len(args) > 1 {
			for _, arg := range args[1:] {
				module := strings.ToLower(arg)
				if _, ok := backupData[module]; ok {
					importModules = append(importModules, module)
				}
			}
		}
	}
	return importModules
}

// importHandler handles the /import command
func (m moduleStruct) importHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg, chat, _, tr, ok := validateImportRequest(b, ctx)
	if !ok {
		return ext.EndGroups
	}

	// Check rate limiting
	if allowed, rateLimitText := checkImportRateLimit(chat.Id, tr); !allowed {
		_, _ = msg.Reply(b, rateLimitText, helpers.Shtml())
		return ext.EndGroups
	}

	// Check if this is a reply to a document
	if msg.ReplyToMessage == nil || msg.ReplyToMessage.Document == nil {
		text, _ := tr.GetString("backup_import_no_reply")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	doc := msg.ReplyToMessage.Document

	// Download the backup file
	fileData, errText := downloadBackupFile(b, doc, tr)
	if fileData == nil {
		_, _ = msg.Reply(b, errText, helpers.Shtml())
		return ext.EndGroups
	}

	// Parse backup file
	backup, err := db.BackupFormatFromJSON(fileData)
	if err != nil {
		log.Errorf("[Backup] Failed to parse backup: %v", err)
		text, _ := tr.GetString("backup_import_invalid_file")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Validate backup
	if err := backup.Validate(); err != nil {
		log.Errorf("[Backup] Invalid backup: %v", err)
		text, _ := tr.GetString("backup_import_invalid_file")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Check version compatibility
	versionWarning := ""
	if !backup.IsCompatibleVersion() {
		versionWarning, _ = tr.GetString("backup_import_version_mismatch")
	}

	// Parse module arguments
	importModules := parseImportModules(msg.Text, backup.Data)

	// If no modules specified, use all from backup
	if len(importModules) == 0 {
		importModules = backup.Modules
	}

	// Store pending import
	pendingImports[chat.Id] = backup
	pendingModules[chat.Id] = importModules

	// Show confirmation with keyboard
	confirmText, _ := tr.GetString("backup_import_confirm", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(importModules)),
		"list":    buildModuleList(importModules),
	})

	if versionWarning != "" {
		confirmText = versionWarning + "\n\n" + confirmText
	}

	keyboard := buildImportKeyboard(tr, chat.Id)

	_, err = msg.Reply(b, confirmText, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Errorf("[Backup] Failed to send confirmation: %v", err)
	}

	return ext.EndGroups
}

// resetHandler handles the /reset command
func (m moduleStruct) resetHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)

	if user == nil {
		return ext.EndGroups
	}

	// Check if in a group
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// Check if bot is admin
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check if user is the group creator
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		text, _ := tr.GetString("backup_reset_creator_only")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Check rate limiting
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.CanReset(chat.Id); !allowed {
		cooldownStr := ratelimit.FormatCooldown(cooldown)
		text, _ := tr.GetString("backup_reset_rate_limited", i18n.TranslationParams{
			"time": cooldownStr,
		})
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Parse module arguments
	var resetModules []string
	if msg.Text != "" {
		args := strings.Fields(msg.Text)
		if len(args) > 1 {
			// User specified specific modules to reset
			for _, arg := range args[1:] {
				if db.IsValidModule(strings.ToLower(arg)) {
					resetModules = append(resetModules, strings.ToLower(arg))
				}
			}
		}
	}

	// If no modules specified, reset all
	if len(resetModules) == 0 {
		resetModules = db.AllExportableModules()
	}

	// Store pending reset (using same maps for simplicity)
	pendingModules[chat.Id] = resetModules

	// Show confirmation with keyboard
	confirmText, _ := tr.GetString("backup_reset_confirm", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(resetModules)),
		"list":    buildModuleList(resetModules),
	})

	keyboard := buildResetKeyboard(tr, chat.Id)

	_, err := msg.Reply(b, confirmText, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Errorf("[Backup] Failed to send confirmation: %v", err)
	}

	return ext.EndGroups
}

// backupCallbackHandler handles callback queries for backup operations
func (m moduleStruct) backupCallbackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From
	chat := ctx.EffectiveChat

	// Only creator can confirm import/reset
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, true) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("backup_import_creator_only")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      text,
			ShowAlert: true,
		})
		return ext.EndGroups
	}

	// Decode callback data
	decoded, ok := decodeCallbackData(query.Data, "backup")
	if !ok {
		tempTr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tempTr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	action, _ := decoded.Field("a")
	chatIDStr, _ := decoded.Field("c")
	chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

	if chatID != chat.Id {
		// Wrong chat
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	switch action {
	case "confirm_import":
		return m.handleConfirmImport(b, ctx, tr, chat)
	case "cancel_import":
		return m.handleCancelImport(b, ctx, tr, query)
	case "confirm_reset":
		return m.handleConfirmReset(b, ctx, tr, chat)
	case "cancel_reset":
		return m.handleCancelReset(b, ctx, tr, query)
	}

	return ext.EndGroups
}

func (m moduleStruct) handleConfirmImport(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat) error {
	// Get pending import
	backup, ok := pendingImports[chat.Id]
	if !ok {
		text, _ := tr.GetString("backup_import_expired")
		_, err := b.SendMessage(chat.Id, text, helpers.Shtml())
		if err != nil {
			log.Errorf("[Backup] Failed to send message: %v", err)
		}
		return ext.EndGroups
	}

	modules := pendingModules[chat.Id]

	// Perform import
	if err := db.ImportChatData(chat.Id, backup, modules); err != nil {
		log.Errorf("[Backup] Import failed for chat %d: %v", chat.Id, err)
		text, _ := tr.GetString("backup_import_failed", i18n.TranslationParams{
			"error": err.Error(),
		})
		_, _ = b.SendMessage(chat.Id, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Record rate limit
	limiter := ratelimit.GetBackupRateLimiter()
	limiter.RecordImport(chat.Id)

	// Clean up
	delete(pendingImports, chat.Id)
	delete(pendingModules, chat.Id)

	// Success message
	text, _ := tr.GetString("backup_import_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(modules)),
		"list":    buildModuleList(modules),
	})
	_, err := b.SendMessage(chat.Id, text, helpers.Shtml())
	if err != nil {
		log.Errorf("[Backup] Failed to send success message: %v", err)
	}

	log.Infof("[Backup] Chat %d imported %d modules", chat.Id, len(modules))
	return ext.EndGroups
}

func (m moduleStruct) handleCancelImport(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery) error {
	chat := ctx.EffectiveChat

	// Clean up
	delete(pendingImports, chat.Id)
	delete(pendingModules, chat.Id)

	text, _ := tr.GetString("backup_import_cancelled")
	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: text,
	})

	msg := ctx.EffectiveMessage
	if msg != nil {
		_, _, _ = msg.EditText(b, text, &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
	}

	return ext.EndGroups
}

func (m moduleStruct) handleConfirmReset(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat) error {
	modules := pendingModules[chat.Id]
	if len(modules) == 0 {
		text, _ := tr.GetString("backup_reset_expired")
		_, _ = b.SendMessage(chat.Id, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Perform reset
	if err := db.ClearChatData(chat.Id, modules); err != nil {
		log.Errorf("[Backup] Reset failed for chat %d: %v", chat.Id, err)
		text, _ := tr.GetString("backup_reset_failed", i18n.TranslationParams{
			"error": err.Error(),
		})
		_, _ = b.SendMessage(chat.Id, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Record rate limit
	limiter := ratelimit.GetBackupRateLimiter()
	limiter.RecordReset(chat.Id)

	// Clean up
	delete(pendingModules, chat.Id)

	// Success message
	text, _ := tr.GetString("backup_reset_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(modules)),
		"list":    buildModuleList(modules),
	})
	_, err := b.SendMessage(chat.Id, text, helpers.Shtml())
	if err != nil {
		log.Errorf("[Backup] Failed to send success message: %v", err)
	}

	log.Infof("[Backup] Chat %d reset %d modules", chat.Id, len(modules))
	return ext.EndGroups
}

func (m moduleStruct) handleCancelReset(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery) error {
	chat := ctx.EffectiveChat

	// Clean up
	delete(pendingModules, chat.Id)

	text, _ := tr.GetString("backup_reset_cancelled")
	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: text,
	})

	msg := ctx.EffectiveMessage
	if msg != nil {
		_, _, _ = msg.EditText(b, text, &gotgbot.EditMessageTextOpts{
			ParseMode: "HTML",
		})
	}

	return ext.EndGroups
}

// Helper functions

func buildExportCaption(tr *i18n.Translator, backup *db.BackupFormat) string {
	modulesList := buildModuleList(backup.Modules)
	caption, _ := tr.GetString("backup_export_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(backup.Data)),
		"list":    modulesList,
		"chat":    backup.ChatName,
		"time":    backup.ExportedAt.Format("2006-01-02 15:04:05"),
	})
	return caption
}

func buildModuleList(modules []string) string {
	if len(modules) == 0 {
		return ""
	}
	return "• " + strings.Join(modules, "\n• ")
}

func buildImportKeyboard(tr *i18n.Translator, chatID int64) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: func() string { t, _ := tr.GetString("button_confirm_import"); return t }(),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": "confirm_import",
						"c": fmt.Sprintf("%d", chatID),
					}, "backup.confirm"),
				},
				{
					Text: func() string { t, _ := tr.GetString("button_cancel_import"); return t }(),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": "cancel_import",
						"c": fmt.Sprintf("%d", chatID),
					}, "backup.cancel"),
				},
			},
		},
	}
}

func buildResetKeyboard(tr *i18n.Translator, chatID int64) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: func() string { t, _ := tr.GetString("button_confirm_reset"); return t }(),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": "confirm_reset",
						"c": fmt.Sprintf("%d", chatID),
					}, "backup.confirm_reset"),
				},
			},
			{
				{
					Text: func() string { t, _ := tr.GetString("button_cancel_reset"); return t }(),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": "cancel_reset",
						"c": fmt.Sprintf("%d", chatID),
					}, "backup.cancel_reset"),
				},
			},
		},
	}
}

// LoadBackup registers all backup module handlers with the dispatcher.
func LoadBackup(dispatcher *ext.Dispatcher) {
	// Register module in enabled map
	HelpModule.AbleMap.Store(backupModule.moduleName, true)

	// Add help keyboard buttons
	HelpModule.helpableKb[backupModule.moduleName] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text: func() string {
					tr := i18n.MustNewTranslator("en")
					t, _ := tr.GetString("backup_export_button")
					return t
				}(),
				CallbackData: encodeCallbackData("backup", map[string]string{"a": "show_export"}, "backup.show_export"),
			},
			{
				Text: func() string {
					tr := i18n.MustNewTranslator("en")
					t, _ := tr.GetString("backup_import_button")
					return t
				}(),
				CallbackData: encodeCallbackData("backup", map[string]string{"a": "show_import"}, "backup.show_import"),
			},
		},
	}

	// Register command handlers
	dispatcher.AddHandler(handlers.NewCommand("export", backupModule.exportHandler))
	dispatcher.AddHandler(handlers.NewCommand("import", backupModule.importHandler))
	dispatcher.AddHandler(handlers.NewCommand("reset", backupModule.resetHandler))

	// Register callback query handlers
	dispatcher.AddHandler(handlers.NewCallback(
		callbackquery.Prefix("backup"),
		backupModule.backupCallbackHandler,
	))

	// Add disableable commands
	helpers.AddCmdToDisableable("export")
	helpers.AddCmdToDisableable("import")

	log.Info("[Backup] Module loaded successfully")
}

// init function to handle unused import
func init() {
	_ = json.Marshal
}