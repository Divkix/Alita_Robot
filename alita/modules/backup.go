package modules

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/backup"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

// Module instance
var backupModule = moduleStruct{
	moduleName: "Backup",
}

type pendingImportState struct {
	backup    *backup.BackupFormat
	modules   []string
	token     string
	expiresAt time.Time
}

type pendingResetState struct {
	modules   []string
	token     string
	expiresAt time.Time
}

// Pending backup operations are stored in memory per chat. The callback token
// prevents an older confirmation button from acting on newer pending state.
// NOTE: single-instance requirement — pending maps are in-memory and lost on
// restart; not suitable for multi-instance deployments without shared storage.
var (
	pendingMu        sync.Mutex
	pendingImports   = make(map[int64]pendingImportState)
	pendingResets    = make(map[int64]pendingResetState)
	errNoValidModule = errors.New("no valid modules in arguments")

	backupDownloadBaseURL    = "https://api.telegram.org/file/bot"
	backupDownloadHTTPClient = &http.Client{}
)

const (
	maxBackupFileSize = 10 * 1024 * 1024
	pendingBackupTTL  = 10 * time.Minute
	// Short action values leave room for an int64 chat ID and nonce under
	// Telegram's 64-byte callback-data limit.
	backupActionConfirmImport = "ci"
	backupActionCancelImport  = "xi"
	backupActionConfirmReset  = "cr"
	backupActionCancelReset   = "xr"
)

func newPendingToken() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate pending backup token: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func storePendingImport(chatID int64, bkp *backup.BackupFormat, modules []string) (string, error) {
	token, err := newPendingToken()
	if err != nil {
		return "", err
	}

	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingImports[chatID] = pendingImportState{
		backup:    bkp,
		modules:   modules,
		token:     token,
		expiresAt: time.Now().Add(pendingBackupTTL),
	}
	return token, nil
}

func getPendingImport(chatID int64) (*backup.BackupFormat, []string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingImports[chatID]
	if !ok || !time.Now().Before(pending.expiresAt) {
		delete(pendingImports, chatID)
		return nil, nil, false
	}
	return pending.backup, pending.modules, true
}

func consumePendingImport(chatID int64, token string) (*backup.BackupFormat, []string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingImports[chatID]
	if !ok {
		return nil, nil, false
	}
	if !time.Now().Before(pending.expiresAt) {
		delete(pendingImports, chatID)
		return nil, nil, false
	}
	if pending.token != token {
		return nil, nil, false
	}
	delete(pendingImports, chatID)
	return pending.backup, pending.modules, true
}

func discardPendingImport(chatID int64, token string) bool {
	_, _, ok := consumePendingImport(chatID, token)
	return ok
}

func clearPendingImport(chatID int64) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	delete(pendingImports, chatID)
}

func storePendingReset(chatID int64, modules []string) (string, error) {
	token, err := newPendingToken()
	if err != nil {
		return "", err
	}

	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingResets[chatID] = pendingResetState{
		modules:   modules,
		token:     token,
		expiresAt: time.Now().Add(pendingBackupTTL),
	}
	return token, nil
}

func getPendingReset(chatID int64) ([]string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingResets[chatID]
	if !ok || !time.Now().Before(pending.expiresAt) {
		delete(pendingResets, chatID)
		return nil, false
	}
	return pending.modules, true
}

func consumePendingReset(chatID int64, token string) ([]string, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pending, ok := pendingResets[chatID]
	if !ok {
		return nil, false
	}
	if !time.Now().Before(pending.expiresAt) {
		delete(pendingResets, chatID)
		return nil, false
	}
	if pending.token != token {
		return nil, false
	}
	delete(pendingResets, chatID)
	return pending.modules, true
}

func discardPendingReset(chatID int64, token string) bool {
	_, ok := consumePendingReset(chatID, token)
	return ok
}

func clearPendingReset(chatID int64) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	delete(pendingResets, chatID)
}

func cleanupExpiredPending() {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	now := time.Now()
	for k, v := range pendingImports {
		if !now.Before(v.expiresAt) {
			delete(pendingImports, k)
		}
	}
	for k, v := range pendingResets {
		if !now.Before(v.expiresAt) {
			delete(pendingResets, k)
		}
	}
}

func startPendingCleanupTicker() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredPending()
		}
	}()
}

// exportHandler handles the /export command
func (m moduleStruct) exportHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)

	if user == nil {
		return ext.EndGroups
	}

	// Check if in a group
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	// Check if user is admin
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	// Check if bot is admin
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Parse module arguments
	var modules []string
	if msg.Text != "" {
		args := strings.Fields(msg.Text)
		if len(args) > 1 {
			var parseErr error
			modules, parseErr = parseModuleArgs(args[1:], backup.IsValidModule)
			if parseErr != nil {
				text, _ := tr.GetString("backup_export_no_modules")
				_, _ = msg.Reply(b, text, formatting.Shtml())
				return ext.EndGroups
			}
		}
	}

	// Reserve before starting work. The cooldown remains after a failure so an
	// expired operation cannot delete a newer caller's reservation.
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireExport(chat.Id); !allowed {
		cooldownStr := ratelimit.FormatCooldown(cooldown)
		text, _ := tr.GetString("backup_export_rate_limited", i18n.TranslationParams{
			"time": cooldownStr,
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Export data
	bkp, err := backup.ExportChatData(chat.Id, chat.Title, user.Id, modules)
	if err != nil {
		log.Errorf("[Backup] Export failed for chat %d: %v", chat.Id, err)
		text, _ := tr.GetString("backup_export_failed")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Check if any data was exported
	if len(bkp.Data) == 0 {
		text, _ := tr.GetString("backup_export_no_modules")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Convert to JSON
	jsonData, err := bkp.ToJSON()
	if err != nil {
		log.Errorf("[Backup] Failed to marshal backup: %v", err)
		text, _ := tr.GetString("backup_export_failed")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Send as document
	fileName := fmt.Sprintf("alita_backup_%d_%s.json", chat.Id, time.Now().Format("20060102_150405"))
	caption := buildExportCaption(tr, bkp)

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
			"modules": fmt.Sprintf("%d", len(bkp.Data)),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	log.Infof("[Backup] Chat %d exported %d modules", chat.Id, len(bkp.Data))
	return ext.EndGroups
}

// validateImportRequest checks all permissions and prerequisites for import
func validateImportRequest(b *gotgbot.Bot, ctx *ext.Context) (*gotgbot.Message, *gotgbot.Chat, *gotgbot.User, *i18n.Translator, bool) {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)

	if user == nil {
		return nil, nil, nil, nil, false
	}

	// Check if in a group
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return nil, nil, nil, nil, false
	}

	// Check if bot is admin
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return nil, nil, nil, nil, false
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check if user is the group creator
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		text, _ := tr.GetString("backup_import_creator_only")
		_, _ = msg.Reply(b, text, formatting.Shtml())
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

	if doc.FileSize > maxBackupFileSize {
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
	baseURL, err := url.Parse(backupDownloadBaseURL)
	if err != nil {
		log.Errorf("[Backup] Failed to parse download base URL: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}
	downloadURL, err := url.Parse(fmt.Sprintf("%s%s/%s", backupDownloadBaseURL, b.Token, file.FilePath))
	if err != nil {
		log.Errorf("[Backup] Failed to parse download URL: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}
	if downloadURL.Scheme != baseURL.Scheme || downloadURL.Host != baseURL.Host {
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

	resp, err := backupDownloadHTTPClient.Do(req)
	if err != nil {
		log.Errorf("[Backup] Failed to download file: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		log.Errorf("[Backup] Failed to download file: status %d", resp.StatusCode)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}

	fileData, err := io.ReadAll(io.LimitReader(resp.Body, maxBackupFileSize+1))
	if err != nil {
		log.Errorf("[Backup] Failed to read file: %v", err)
		text, _ := tr.GetString("backup_import_download_failed")
		return nil, text
	}
	if len(fileData) > maxBackupFileSize {
		text, _ := tr.GetString("backup_import_file_too_large")
		return nil, text
	}

	return fileData, ""
}

// parseImportModules parses module arguments from command text.
func parseImportModules(text string, backupData map[string]interface{}) ([]string, error) {
	if text != "" {
		args := strings.Fields(text)
		if len(args) > 1 {
			return parseModuleArgs(args[1:], func(module string) bool {
				_, ok := backupData[module]
				return ok
			})
		}
	}
	return nil, nil
}

func parseModuleArgs(args []string, valid func(string) bool) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	modules := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		module := strings.ToLower(arg)
		if module == "" || !valid(module) {
			continue
		}
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		modules = append(modules, module)
	}
	if len(modules) == 0 {
		return nil, errNoValidModule
	}
	return modules, nil
}

// importHandler handles the /import command
func (m moduleStruct) importHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg, chat, _, tr, ok := validateImportRequest(b, ctx)
	if !ok {
		return ext.EndGroups
	}

	// Check if this is a reply to a document (validate before burning cooldown)
	if msg.ReplyToMessage == nil || msg.ReplyToMessage.Document == nil {
		text, _ := tr.GetString("backup_import_no_reply")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	doc := msg.ReplyToMessage.Document

	// Download the backup file
	fileData, errText := downloadBackupFile(b, doc, tr)
	if fileData == nil {
		_, _ = msg.Reply(b, errText, formatting.Shtml())
		return ext.EndGroups
	}

	// Parse backup file
	bkp, err := backup.BackupFormatFromJSON(fileData)
	if err != nil {
		log.Errorf("[Backup] Failed to parse backup: %v", err)
		text, _ := tr.GetString("backup_import_invalid_file")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Validate backup
	if err := bkp.Validate(); err != nil {
		log.Errorf("[Backup] Invalid backup: %v", err)
		text, _ := tr.GetString("backup_import_invalid_file")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	if !bkp.IsCompatibleVersion() {
		text, _ := tr.GetString("backup_import_version_mismatch")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Parse module arguments
	importModules, parseErr := parseImportModules(msg.Text, bkp.Data)
	if parseErr != nil {
		text, _ := tr.GetString("backup_import_invalid_file")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// If no modules specified, use all from backup
	if len(importModules) == 0 {
		importModules = bkp.Modules
	}

	// Reserve cooldown after validation to avoid burning on malformed input.
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireImport(chat.Id); !allowed {
		text, _ := tr.GetString("backup_import_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(cooldown),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Store pending import
	token, err := storePendingImport(chat.Id, bkp, importModules)
	if err != nil {
		log.Errorf("[Backup] Failed to store pending import: %v", err)
		text, _ := tr.GetString("backup_import_failed", i18n.TranslationParams{"error": err.Error()})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Show confirmation with keyboard
	confirmText, _ := tr.GetString("backup_import_confirm", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(importModules)),
		"list":    buildModuleList(importModules),
	})

	keyboard := buildImportKeyboard(tr, chat.Id, token)

	_, err = msg.Reply(b, confirmText, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		discardPendingImport(chat.Id, token)
		log.Errorf("[Backup] Failed to send confirmation: %v", err)
	}

	return ext.EndGroups
}

// resetHandler handles the /reset command
func (m moduleStruct) resetHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)

	if user == nil {
		return ext.EndGroups
	}

	// Check if in a group
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	// Check if bot is admin
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check if user is the group creator
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	// Parse module arguments (validate before burning cooldown)
	var resetModules []string
	if msg.Text != "" {
		args := strings.Fields(msg.Text)
		if len(args) > 1 {
			var parseErr error
			resetModules, parseErr = parseModuleArgs(args[1:], backup.IsValidModule)
			if parseErr != nil {
				text, _ := tr.GetString("backup_export_no_modules")
				_, _ = msg.Reply(b, text, formatting.Shtml())
				return ext.EndGroups
			}
		}
	}

	// If no modules specified, reset all
	if len(resetModules) == 0 {
		resetModules = backup.AllExportableModules()
	}

	// Reserve after validation to avoid burning cooldown on malformed input.
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireReset(chat.Id); !allowed {
		text, _ := tr.GetString("backup_reset_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(cooldown),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Store pending reset
	token, err := storePendingReset(chat.Id, resetModules)
	if err != nil {
		log.Errorf("[Backup] Failed to store pending reset: %v", err)
		text, _ := tr.GetString("backup_reset_failed", i18n.TranslationParams{"error": err.Error()})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Show confirmation with keyboard
	confirmText, _ := tr.GetString("backup_reset_confirm", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(resetModules)),
		"list":    buildModuleList(resetModules),
	})

	keyboard := buildResetKeyboard(tr, chat.Id, token)

	_, err = msg.Reply(b, confirmText, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		discardPendingReset(chat.Id, token)
		log.Errorf("[Backup] Failed to send confirmation: %v", err)
	}

	return ext.EndGroups
}

// backupCallbackHandler handles callback queries for backup operations
func (m moduleStruct) backupCallbackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if chat == nil {
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	// Only creator can confirm import/reset
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
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
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	action, _ := decoded.Field("a")
	chatIDStr, _ := decoded.Field("c")
	token, _ := decoded.Field("t")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	validAction := action == backupActionConfirmImport ||
		action == backupActionCancelImport ||
		action == backupActionConfirmReset ||
		action == backupActionCancelReset
	if err != nil || chatID != chat.Id || token == "" || !validAction {
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	switch action {
	case backupActionConfirmImport:
		return m.handleConfirmImport(b, ctx, tr, chat, token)
	case backupActionCancelImport:
		return m.handleCancelImport(b, ctx, tr, query, token)
	case backupActionConfirmReset:
		return m.handleConfirmReset(b, ctx, tr, chat, token)
	case backupActionCancelReset:
		return m.handleCancelReset(b, ctx, tr, query, token)
	}
	return ext.EndGroups
}

func (m moduleStruct) handleConfirmImport(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat, token string) error {
	bkp, modules, ok := consumePendingImport(chat.Id, token)
	if !ok {
		text, _ := tr.GetString("backup_import_expired")
		_, err := b.SendMessage(chat.Id, text, formatting.Shtml())
		if err != nil {
			log.Errorf("[Backup] Failed to send message: %v", err)
		}
		return ext.EndGroups
	}

	// Rate limit already acquired at importHandler entry; no second Acquire here
	// to keep operation atomic (handler entry reserves the cooldown).
	// Perform import
	if err := backup.ImportChatData(chat.Id, bkp, modules); err != nil {
		log.Errorf("[Backup] Import failed for chat %d: %v", chat.Id, err)
		text, _ := tr.GetString("backup_import_failed", i18n.TranslationParams{
			"error": err.Error(),
		})
		_, _ = b.SendMessage(chat.Id, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Success message
	text, _ := tr.GetString("backup_import_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(modules)),
		"list":    buildModuleList(modules),
	})
	_, err := b.SendMessage(chat.Id, text, formatting.Shtml())
	if err != nil {
		log.Errorf("[Backup] Failed to send success message: %v", err)
	}

	log.Infof("[Backup] Chat %d imported %d modules", chat.Id, len(modules))
	return ext.EndGroups
}

// handleCancelPending atomically discards matching pending state and
// acknowledges the cancelled backup callback.
func (m moduleStruct) handleCancelPending(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, token string, discard func(int64, string) bool, cancelKey, expiredKey string) error {
	chat := ctx.EffectiveChat

	if !discard(chat.Id, token) {
		text, _ := tr.GetString(expiredKey)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	text, _ := tr.GetString(cancelKey)
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

func (m moduleStruct) handleCancelImport(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, token string) error {
	return m.handleCancelPending(b, ctx, tr, query, token, discardPendingImport, "backup_import_cancelled", "backup_import_expired")
}

func (m moduleStruct) handleConfirmReset(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat, token string) error {
	modules, ok := consumePendingReset(chat.Id, token)
	if !ok || len(modules) == 0 {
		text, _ := tr.GetString("backup_reset_expired")
		_, _ = b.SendMessage(chat.Id, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Rate limit already acquired at resetHandler entry.
	// Perform reset
	if err := backup.ClearChatData(chat.Id, modules); err != nil {
		log.Errorf("[Backup] Reset failed for chat %d: %v", chat.Id, err)
		text, _ := tr.GetString("backup_reset_failed", i18n.TranslationParams{
			"error": err.Error(),
		})
		_, _ = b.SendMessage(chat.Id, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Success message
	text, _ := tr.GetString("backup_reset_success", i18n.TranslationParams{
		"modules": fmt.Sprintf("%d", len(modules)),
		"list":    buildModuleList(modules),
	})
	_, err := b.SendMessage(chat.Id, text, formatting.Shtml())
	if err != nil {
		log.Errorf("[Backup] Failed to send success message: %v", err)
	}

	log.Infof("[Backup] Chat %d reset %d modules", chat.Id, len(modules))
	return ext.EndGroups
}

func (m moduleStruct) handleCancelReset(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, token string) error {
	return m.handleCancelPending(b, ctx, tr, query, token, discardPendingReset, "backup_reset_cancelled", "backup_reset_expired")
}

// Helper functions

func buildExportCaption(tr *i18n.Translator, backup *backup.BackupFormat) string {
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

func buildImportKeyboard(tr *i18n.Translator, chatID int64, token string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: trS(tr, "button_confirm_import"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionConfirmImport,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
				{
					Text: trS(tr, "button_cancel_import"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionCancelImport,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
			},
		},
	}
}

func buildResetKeyboard(tr *i18n.Translator, chatID int64, token string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: trS(tr, "button_confirm_reset"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionConfirmReset,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
			},
			{
				{
					Text: trS(tr, "button_cancel_reset"),
					CallbackData: encodeCallbackData("backup", map[string]string{
						"a": backupActionCancelReset,
						"c": fmt.Sprintf("%d", chatID),
						"t": token,
					}),
				},
			},
		},
	}
}

// LoadBackup registers all backup module handlers with the dispatcher.
func LoadBackup(dispatcher *ext.Dispatcher) {
	// Register module in enabled map
	DefaultHelpRegistry().AbleMap[backupModule.moduleName] = true

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

func init() {
	RegisterLegacyModule("Backup", 270, LoadBackup)
	startPendingCleanupTicker()
}
