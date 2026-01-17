package modules

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/mojocn/base64Captcha"
	log "github.com/sirupsen/logrus"
)

var captchaModule = moduleStruct{moduleName: "Captcha"}

// messageTypeToString converts message type constants to human-readable strings
func messageTypeToString(tr *i18n.Translator, messageType int) string {
	var key string
	switch messageType {
	case db.TEXT:
		key = "message_type_text"
	case db.STICKER:
		key = "message_type_sticker"
	case db.DOCUMENT:
		key = "message_type_document"
	case db.PHOTO:
		key = "message_type_photo"
	case db.AUDIO:
		key = "message_type_audio"
	case db.VOICE:
		key = "message_type_voice"
	case db.VIDEO:
		key = "message_type_video"
	case db.VideoNote:
		key = "message_type_video_note"
	default:
		key = "message_type_unknown"
	}
	text, _ := tr.GetString(key)
	return text
}

// Refresh controls
const (
	captchaMaxRefreshes     = 3
	captchaRefreshCooldownS = 5 // seconds
)

// Cleanup and recovery constants
const (
	captchaFailureMessageTTL = 30 * time.Second
	captchaCleanupRetries    = 3
)

// Module-level bot reference for cleanup operations
var (
	captchaBotRef       *gotgbot.Bot
	captchaBotRefOnce   sync.Once
	captchaRecoveryOnce sync.Once
)

// setBotRef captures the bot reference on first handler call and triggers recovery
func setBotRef(bot *gotgbot.Bot) {
	captchaBotRefOnce.Do(func() {
		captchaBotRef = bot
		// Run startup recovery in background
		go recoverOrphanedCaptchas(bot)
	})
}

// isPermanentTelegramError checks if an error is permanent and shouldn't be retried
func isPermanentTelegramError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	permanentErrors := []string{
		"message to delete not found",
		"message can't be deleted",
		"bot was kicked",
		"chat not found",
		"group chat was deactivated",
		"bot is not a member",
		"CHAT_NOT_FOUND",
		"PEER_ID_INVALID",
	}
	for _, pe := range permanentErrors {
		if strings.Contains(errStr, pe) {
			return true
		}
	}
	return false
}

// isPermanentUnmuteError checks if an unmute error is permanent and the record should be deleted
func isPermanentUnmuteError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	permanentErrors := []string{
		"user not found",
		"USER_NOT_PARTICIPANT",
		"bot was kicked",
		"chat not found",
		"group chat was deactivated",
		"bot is not a member",
		"CHAT_NOT_FOUND",
		"PEER_ID_INVALID",
		"user is an administrator",
		"not enough rights",
	}
	for _, pe := range permanentErrors {
		if strings.Contains(errStr, pe) {
			return true
		}
	}
	return false
}

// recoverOrphanedCaptchas handles captcha attempts left over from bot restart.
// For expired attempts: delete message and DB record.
// For still-valid attempts: delete message (user must rejoin to get new captcha).
func recoverOrphanedCaptchas(bot *gotgbot.Bot) {
	captchaRecoveryOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("[CaptchaRecovery] Recovery panic: %v", r)
			}
		}()

		log.Info("[CaptchaRecovery] Starting orphaned captcha recovery...")

		// Get ALL pending attempts (both expired and valid)
		attempts, err := db.GetAllPendingCaptchaAttempts()
		if err != nil {
			log.Errorf("[CaptchaRecovery] Failed to get pending attempts: %v", err)
			return
		}

		if len(attempts) == 0 {
			log.Info("[CaptchaRecovery] No orphaned captchas found")
			return
		}

		log.Infof("[CaptchaRecovery] Found %d orphaned captcha attempts to process", len(attempts))

		var (
			expiredCount int
			cleanedCount int
			failedCount  int
		)

		for _, attempt := range attempts {
			// Delete the Telegram message (best effort)
			if attempt.MessageID > 0 {
				if err := helpers.DeleteMessageWithErrorHandling(bot, attempt.ChatID, attempt.MessageID); err != nil {
					if !isPermanentTelegramError(err) {
						log.Warnf("[CaptchaRecovery] Failed to delete message %d in chat %d: %v",
							attempt.MessageID, attempt.ChatID, err)
						failedCount++
					}
				}
			}

			// Delete stored messages
			_ = db.DeleteStoredMessagesForAttempt(attempt.ID)

			// Get settings to apply failure action for expired attempts
			if time.Now().After(attempt.ExpiresAt) {
				// Expired - apply failure action
				settings, _ := db.GetCaptchaSettings(attempt.ChatID)
				if settings == nil {
					settings = &db.CaptchaSettings{FailureAction: "kick"}
				}

				// Apply failure action
				switch settings.FailureAction {
				case "kick":
					_, err := bot.BanChatMember(attempt.ChatID, attempt.UserID, nil)
					if err == nil {
						_, _ = bot.UnbanChatMember(attempt.ChatID, attempt.UserID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: false})
					}
				case "ban":
					_, _ = bot.BanChatMember(attempt.ChatID, attempt.UserID, nil)
				case "mute":
					// User remains muted
				}
				expiredCount++
			} else {
				// Still valid but orphaned - clean up, user must rejoin
				cleanedCount++
			}

			// Delete attempt from DB
			_ = db.DeleteCaptchaAttempt(attempt.UserID, attempt.ChatID)

			// Small delay to avoid rate limiting
			time.Sleep(50 * time.Millisecond)
		}

		log.Infof("[CaptchaRecovery] Completed: %d expired (action applied), %d cleaned, %d failed",
			expiredCount, cleanedCount, failedCount)
	})
}

// secureIntn returns a cryptographically secure random integer in [0, max).
// If max <= 0, it returns 0.
func secureIntn(max int) int {
	if max <= 0 {
		return 0
	}
	// Use crypto/rand.Int for unbiased secure random selection
	// Retry on the extremely unlikely error case.
	for {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
		if err == nil {
			return int(n.Int64())
		}
	}
}

// secureShuffleStrings shuffles a slice of strings using Fisher-Yates with crypto-grade randomness.
func secureShuffleStrings(values []string) {
	for i := len(values) - 1; i > 0; i-- {
		j := secureIntn(i + 1)
		values[i], values[j] = values[j], values[i]
	}
}

// viewPendingMessages handles the /captchapending command to view stored messages for a user.
// Admins can use this to see what messages a user tried to send before verification.
func (moduleStruct) viewPendingMessages(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	// Check admin permissions
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	// Parse target user from command
	args := ctx.Args()[1:]
	if len(args) < 1 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_pending_usage")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	// Get user ID from mention or ID
	targetUserID := extraction.ExtractUser(bot, ctx)
	if targetUserID == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	// Get stored messages for user
	messages, err := db.GetStoredMessagesForUser(targetUserID, chat.Id)
	if err != nil || len(messages) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_no_pending_messages")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	// Build response
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var response strings.Builder
	headerText, _ := tr.GetString("captcha_pending_messages_header")
	response.WriteString(fmt.Sprintf(headerText, targetUserID))

	for i, msg := range messages {
		typeText, _ := tr.GetString("captcha_pending_message_type")
		response.WriteString(fmt.Sprintf(typeText, i+1, messageTypeToString(tr, msg.MessageType)))
		if msg.Caption != "" {
			captionText, _ := tr.GetString("captcha_pending_message_caption")
			response.WriteString(fmt.Sprintf(captionText, html.EscapeString(msg.Caption)))
		}
		if msg.Content != "" && msg.MessageType == db.TEXT {
			preview := msg.Content
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			contentText, _ := tr.GetString("captcha_pending_message_content")
			response.WriteString(fmt.Sprintf(contentText, html.EscapeString(preview)))
		}
		timeText, _ := tr.GetString("captcha_pending_message_time")
		response.WriteString(fmt.Sprintf(timeText, msg.CreatedAt.Format("15:04:05")))
	}

	_, err = msg.Reply(bot, response.String(), helpers.Shtml())
	return err
}

// clearPendingMessages handles the /captchaclear command to clear stored messages for a user.
// Admins can use this to manually clear pending messages if needed.
func (moduleStruct) clearPendingMessages(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	// Check admin permissions
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	// Parse target user
	args := ctx.Args()[1:]
	if len(args) < 1 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_clear_usage")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	targetUserID := extraction.ExtractUser(bot, ctx)
	if targetUserID == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	// Delete messages
	err := db.DeleteStoredMessagesForUser(targetUserID, chat.Id)
	if err != nil {
		log.Errorf("Failed to delete stored messages for user %d in chat %d: %v", targetUserID, chat.Id, err)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_clear_failed")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_clear_success",
		i18n.TranslationParams{"user_id": targetUserID})
	_, err = msg.Reply(bot, text, helpers.Shtml())
	return err
}

// captchaCommand handles the /captcha command to enable/disable captcha verification.
// Admins can use this to toggle captcha protection for their group.
func (moduleStruct) captchaCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	setBotRef(bot)

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// Check permissions
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		// Show current status
		settings, _ := db.GetCaptchaSettings(chat.Id)
		status := "disabled"
		if settings.Enabled {
			status = "enabled"
		}

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		statusUsage, _ := tr.GetString("captcha_status_usage")
		header, _ := tr.GetString("captcha_settings_header")
		statusLine, _ := tr.GetString("captcha_settings_status", i18n.TranslationParams{"s": status})
		modeLine, _ := tr.GetString("captcha_settings_mode", i18n.TranslationParams{"s": settings.CaptchaMode})
		timeoutLine, _ := tr.GetString("captcha_settings_timeout", i18n.TranslationParams{"d": settings.Timeout})
		actionLine, _ := tr.GetString("captcha_settings_failure_action", i18n.TranslationParams{"s": settings.FailureAction})
		attemptsLine, _ := tr.GetString("captcha_settings_max_attempts", i18n.TranslationParams{"d": settings.MaxAttempts})

		text := fmt.Sprintf(
			"%s\n%s\n%s\n%s\n%s\n%s\n\n%s",
			header, statusLine, modeLine, timeoutLine, actionLine, attemptsLine, statusUsage,
		)

		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	switch strings.ToLower(args[0]) {
	case "on", "enable", "yes":
		err := db.SetCaptchaEnabled(chat.Id, true)
		if err != nil {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_enable_failed")
			_, _ = msg.Reply(bot, text, nil)
			return err
		}
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_enabled_success")
		_, err = msg.Reply(bot, text, helpers.Shtml())
		return err

	case "off", "disable", "no":
		err := db.SetCaptchaEnabled(chat.Id, false)
		if err != nil {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_disable_failed")
			_, _ = msg.Reply(bot, text, nil)
			return err
		}
		// Clean up any pending captcha attempts
		go func(chatID int64) {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("Panic in captcha disable cleanup goroutine")
				}
			}()

			// Add timeout context for cleanup operation
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Use a channel to signal completion
			done := make(chan struct{})
			go func() {
				defer close(done)
				if err := db.DeleteAllCaptchaAttempts(chatID); err != nil {
					log.Errorf("Failed to delete captcha attempts for chat %d: %v", chatID, err)
				} else {
					log.Infof("Successfully cleaned up captcha attempts for chat %d", chatID)
				}
			}()

			select {
			case <-done:
				// Operation completed successfully
				log.Debugf("Captcha cleanup completed for chat %d", chatID)
			case <-ctx.Done():
				log.Warnf("Captcha cleanup timed out for chat %d", chatID)
				return
			}
		}(chat.Id)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_disabled_success")
		_, err = msg.Reply(bot, text, helpers.Shtml())
		return err

	default:
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_usage")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}
}

// captchaModeCommand handles the /captchamode command to set captcha type.
// Admins can choose between math and text captcha modes.
func (moduleStruct) captchaModeCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// Check permissions
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_mode_specify")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	mode := strings.ToLower(args[0])
	if mode != "math" && mode != "text" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_mode_invalid")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	err := db.SetCaptchaMode(chat.Id, mode)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		var text string
		if errors.Is(err, db.ErrInvalidCaptchaMode) {
			text, _ = tr.GetString("captcha_invalid_mode_error")
		} else {
			text, _ = tr.GetString("captcha_mode_failed")
		}
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	modeDesc, _ := tr.GetString("captcha_mode_math_desc")
	if mode == "text" {
		modeDesc, _ = tr.GetString("captcha_mode_text_desc")
	}

	textTemplate, _ := tr.GetString("captcha_mode_set_formatted")
	text := fmt.Sprintf(textTemplate, mode, modeDesc)
	_, err = msg.Reply(bot, text, helpers.Shtml())
	return err
}

// captchaTimeCommand handles the /captchatime command to set verification timeout.
// Admins can set how long users have to complete the captcha (1-10 minutes).
func (moduleStruct) captchaTimeCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// Check permissions
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_timeout_specify")
		_, err := msg.Reply(bot, text, nil)
		return err
	}

	timeout, err := strconv.Atoi(args[0])
	if err != nil || timeout < 1 || timeout > 10 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_timeout_invalid")
		_, err = msg.Reply(bot, text, nil)
		return err
	}

	err = db.SetCaptchaTimeout(chat.Id, timeout)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		var text string
		if errors.Is(err, db.ErrInvalidTimeout) {
			text, _ = tr.GetString("captcha_timeout_range_error")
		} else {
			text, _ = tr.GetString("captcha_timeout_failed")
		}
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_timeout_set_success", i18n.TranslationParams{"d": timeout})
	_, err = msg.Reply(bot, text, helpers.Shtml())
	return err
}

// captchaActionCommand handles the /captchaaction command to set failure action.
// Admins can choose what happens when users fail the captcha: kick, ban, or mute.
func (moduleStruct) captchaActionCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// Check permissions
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_action_specify")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	action := strings.ToLower(args[0])
	if action != "kick" && action != "ban" && action != "mute" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_action_invalid")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	err := db.SetCaptchaFailureAction(chat.Id, action)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		var text string
		if errors.Is(err, db.ErrInvalidFailureAction) {
			text, _ = tr.GetString("captcha_invalid_action_error")
		} else {
			text, _ = tr.GetString("captcha_action_failed")
		}
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_action_set_success", i18n.TranslationParams{"s": action})
	_, err = msg.Reply(bot, text, helpers.Shtml())
	return err
}

// captchaMaxAttemptsCommand handles the /captchamaxattempts command to set max verification attempts.
// Admins can set how many wrong answers are allowed before taking action (1-10 attempts).
func (moduleStruct) captchaMaxAttemptsCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// Check permissions
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if len(args) == 0 {
		settings, _ := db.GetCaptchaSettings(chat.Id)
		text, _ := tr.GetString("captcha_max_attempts_current", map[string]interface{}{
			"attempts": settings.MaxAttempts,
		})
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	attempts, err := strconv.Atoi(args[0])
	if err != nil || attempts < 1 || attempts > 10 {
		text, _ := tr.GetString("captcha_max_attempts_invalid")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	if err := db.SetCaptchaMaxAttempts(chat.Id, attempts); err != nil {
		log.Errorf("Failed to set captcha max attempts: %v", err)
		text, _ := tr.GetString("captcha_internal_error")
		_, err := msg.Reply(bot, text, helpers.Shtml())
		return err
	}

	text, _ := tr.GetString("captcha_max_attempts_set", map[string]interface{}{
		"attempts": attempts,
	})
	_, err = msg.Reply(bot, text, helpers.Shtml())
	return err
}

// generateMathCaptcha generates a random math problem and returns the question and answer.
func generateMathCaptcha() (string, string, []string) {
	operations := []string{"+", "-", "*"}
	operation := operations[secureIntn(len(operations))]

	var a, b, answer int
	var question string

	switch operation {
	case "+":
		a = secureIntn(50) + 1
		b = secureIntn(50) + 1
		answer = a + b
		question = fmt.Sprintf("%d + %d", a, b)
	case "-":
		a = secureIntn(50) + 20
		b = secureIntn(a) + 1
		answer = a - b
		question = fmt.Sprintf("%d - %d", a, b)
	case "*":
		a = secureIntn(12) + 1
		b = secureIntn(12) + 1
		answer = a * b
		question = fmt.Sprintf("%d × %d", a, b)
	}

	// Generate wrong answers
	options := []string{strconv.Itoa(answer)}
	for len(options) < 4 {
		// Generate a wrong answer within reasonable range
		wrongAnswer := answer + secureIntn(20) - 10
		if wrongAnswer != answer && wrongAnswer > 0 {
			wrongStr := strconv.Itoa(wrongAnswer)
			// Check if this option already exists
			if !slices.Contains(options, wrongStr) {
				options = append(options, wrongStr)
			}
		}
	}

	// Shuffle options
	secureShuffleStrings(options)

	return question, strconv.Itoa(answer), options
}

// generateTextCaptcha generates a captcha image with random text.
func generateTextCaptcha() (string, []byte, []string, error) {
	// Create captcha store (using memory store)
	store := base64Captcha.DefaultMemStore

	// Create a string driver for text captcha
	driver := base64Captcha.NewDriverString(
		80,                                 // height
		160,                                // width
		0,                                  // noiseCount
		2,                                  // showLineOptions
		4,                                  // length
		"234567890abcdefghjkmnpqrstuvwxyz", // source characters
		nil,                                // bgColor
		nil,                                // fonts
		[]string{},
	)

	// Create captcha
	captcha := base64Captcha.NewCaptcha(driver, store)

	// Generate the captcha
	id, b64s, answer, err := captcha.Generate()
	if err != nil {
		return "", nil, nil, err
	}
	_ = id // We don't use the ID

	// Decode base64 image
	// Remove data:image/png;base64, prefix if present
	if strings.HasPrefix(b64s, "data:image/") {
		parts := strings.Split(b64s, ",")
		if len(parts) > 1 {
			b64s = parts[1]
		}
	}

	imageBytes, err := base64.StdEncoding.DecodeString(b64s)
	if err != nil {
		return "", nil, nil, err
	}

	// Generate decoy answers
	options := []string{answer}
	characters := "234567890abcdefghjkmnpqrstuvwxyz"
	for len(options) < 4 {
		// Generate a random string of same length as answer
		decoy := ""
		for range len(answer) {
			decoy += string(characters[secureIntn(len(characters))])
		}
		// Check if this option already exists
		if !slices.Contains(options, decoy) {
			options = append(options, decoy)
		}
	}

	// Shuffle options
	secureShuffleStrings(options)

	// Verify answer is in options (defensive check)
	if !slices.Contains(options, answer) {
		log.Errorf("[Captcha] BUG: Text answer %q not in options %v, regenerating", answer, options)
		return generateTextCaptcha() // Retry
	}

	return answer, imageBytes, options, nil
}

// generateMathImageCaptcha generates a math captcha image using custom math generation
// for reliable answer matching. Uses the existing generateMathCaptcha logic.
func generateMathImageCaptcha() (string, []byte, []string, error) {
	// Use our reliable math generation
	question, answer, options := generateMathCaptcha()

	// Create a character driver to render the question as an image
	driver := base64Captcha.NewDriverString(
		80,            // height
		240,           // width (wider for math expression)
		0,             // noiseCount
		2,             // showLineOptions
		len(question), // source length
		question,      // source string (the question itself)
		nil,           // bgColor
		nil,           // fonts
		[]string{},    // fontsArray
	)

	captcha := base64Captcha.NewCaptcha(driver, base64Captcha.DefaultMemStore)
	_, b64s, _, err := captcha.Generate()
	if err != nil {
		return "", nil, nil, err
	}

	// Decode base64 image
	if strings.HasPrefix(b64s, "data:image/") {
		parts := strings.Split(b64s, ",")
		if len(parts) > 1 {
			b64s = parts[1]
		}
	}
	imageBytes, err := base64.StdEncoding.DecodeString(b64s)
	if err != nil {
		return "", nil, nil, err
	}

	// Verify answer is in options (defensive check)
	if !slices.Contains(options, answer) {
		log.Errorf("[Captcha] BUG: Answer %s not in options %v, regenerating", answer, options)
		return generateMathImageCaptcha() // Retry
	}

	return answer, imageBytes, options, nil
}

// SendCaptcha sends a captcha challenge to a new member.
// Called when a new member joins a group with captcha enabled.
func SendCaptcha(bot *gotgbot.Bot, ctx *ext.Context, userID int64, userName string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Captcha][SendCaptcha] Recovered from panic: %v", r)
		}
	}()
	chat := ctx.EffectiveChat
	settings, _ := db.GetCaptchaSettings(chat.Id)

	if !settings.Enabled {
		return nil
	}

	var question string
	var answer string
	var options []string
	var imageBytes []byte
	isImage := false

	if settings.CaptchaMode == "math" {
		// Prefer image captcha for math mode
		var err error
		answer, imageBytes, options, err = generateMathImageCaptcha()
		if err != nil || imageBytes == nil {
			log.Errorf("Failed to generate math image captcha: %v", err)
			// Fallback to text-based math question
			question, answer, options = generateMathCaptcha()
			isImage = false
		} else {
			isImage = true
		}
	} else {
		// Text mode: image captcha with text content
		var err error
		answer, imageBytes, options, err = generateTextCaptcha()
		if err != nil {
			log.Errorf("Failed to generate text captcha: %v", err)
			// Fallback to text-based math question
			question, answer, options = generateMathCaptcha()
			isImage = false
		} else {
			isImage = true
		}
	}

	// Debug logging for captcha generation
	log.Debugf("[Captcha] Generated %s captcha for user %d in chat %d: answer=%q, options=%v",
		settings.CaptchaMode, userID, chat.Id, answer, options)

	// Validate user and chat exist in Telegram before creating DB records
	// This prevents FK constraint violations for non-existent entities

	// Validate user exists via Telegram API
	userMember, err := bot.GetChatMember(chat.Id, userID, nil)
	if err != nil {
		log.Errorf("Failed to validate user %d via Telegram API: %v", userID, err)
		return fmt.Errorf("user %d does not exist or is not accessible: %w", userID, err)
	}

	// Extract validated user info from API response
	validatedUser := userMember.GetUser()
	validatedUserName := userName
	if validatedUser.FirstName != "" {
		validatedUserName = validatedUser.FirstName
	}
	validatedUsername := validatedUser.Username

	// Validate chat exists (already have chat object from context, but verify it's valid)
	if chat.Id == 0 || chat.Title == "" {
		log.Errorf("Invalid chat data: ID=%d, Title=%s", chat.Id, chat.Title)
		return fmt.Errorf("invalid chat data")
	}

	// Now that we've validated via Telegram API, ensure records exist in database
	if err := db.EnsureUserInDb(userID, validatedUsername, validatedUserName); err != nil {
		log.Errorf("Failed to ensure user in database: %v", err)
		return err
	}
	if err := db.EnsureChatInDb(chat.Id, chat.Title); err != nil {
		log.Errorf("Failed to ensure chat in database: %v", err)
		return err
	}

	preAttempt, preErr := db.CreateCaptchaAttemptPreMessage(userID, chat.Id, answer, settings.Timeout)
	if preErr != nil || preAttempt == nil {
		log.Errorf("Failed to pre-create captcha attempt: %v", preErr)
		return preErr
	}

	// Create inline keyboard with options including attempt ID
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, option := range options {
		button := gotgbot.InlineKeyboardButton{
			Text:         option,
			CallbackData: fmt.Sprintf("captcha_verify.%d.%d.%s", preAttempt.ID, userID, option),
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{button})
	}

	// Add refresh button for image-based captcha (text or math) with attempt ID
	if isImage && imageBytes != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		buttonText, _ := tr.GetString("captcha_refresh_button")
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{
				Text:         buttonText,
				CallbackData: fmt.Sprintf("captcha_refresh.%d.%d", preAttempt.ID, userID),
			},
		})
	}

	keyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	// Prepare message text/caption
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var msgText string
	if isImage {
		if settings.CaptchaMode == "math" {
			text, _ := tr.GetString("captcha_welcome_math_image", i18n.TranslationParams{
				"first":  helpers.MentionHtml(userID, userName),
				"number": settings.Timeout,
			})
			msgText = text
		} else {
			text, _ := tr.GetString("captcha_welcome_text_image", i18n.TranslationParams{
				"first":  helpers.MentionHtml(userID, userName),
				"number": settings.Timeout,
			})
			msgText = text
		}
	} else {
		// Text-based fallback for math
		text, _ := tr.GetString("captcha_welcome_math_text", i18n.TranslationParams{
			"first":    helpers.MentionHtml(userID, userName),
			"question": question,
			"number":   settings.Timeout,
		})
		msgText = text
	}

	// Send the captcha message
	var sent *gotgbot.Message

	if isImage && imageBytes != nil {
		// Send photo with text captcha
		sent, err = bot.SendPhoto(chat.Id, gotgbot.InputFileByReader("captcha.png", bytes.NewReader(imageBytes)), &gotgbot.SendPhotoOpts{
			Caption:     msgText,
			ParseMode:   helpers.HTML,
			ReplyMarkup: keyboard,
		})
	} else {
		// Send text message for math captcha
		sent, err = helpers.SendMessageWithErrorHandling(bot, chat.Id, msgText, &gotgbot.SendMessageOpts{
			ParseMode:   helpers.HTML,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil {
		log.Errorf("Failed to send captcha: %v", err)
		// Clean up the pre-created attempt to prevent orphaned records
		_ = db.DeleteCaptchaAttempt(userID, chat.Id)
		return err
	}

	// Update the attempt with the sent message ID
	err = db.UpdateCaptchaAttemptMessageID(preAttempt.ID, sent.MessageId)
	if err != nil {
		log.Errorf("Failed to set captcha attempt message ID: %v", err)
		// Delete the message if we can't track it
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, sent.MessageId)
		return err
	}

	// Schedule cleanup after timeout with proper context and cancellation
	go func(originalMessageID int64, chatID int64, uID int64) {
		// Create a context with the timeout duration plus a small buffer
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.Timeout)*time.Minute+30*time.Second)
		defer cancel()

		// Use a timer instead of Sleep for better control
		timer := time.NewTimer(time.Duration(settings.Timeout) * time.Minute)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Check if attempt still exists (not completed)
			attempt, _ := db.GetCaptchaAttempt(uID, chatID)
			if attempt != nil {
				// Use the latest message ID from the attempt to avoid leaving a stale message after refresh
				handleCaptchaTimeout(bot, chatID, uID, attempt.MessageID, settings.FailureAction)
			}
		case <-ctx.Done():
			log.Warnf("Captcha timeout handler cancelled for user %d in chat %d", uID, chatID)
			return
		}
	}(sent.MessageId, chat.Id, userID)

	return nil
}

// handleCaptchaTimeout handles when a user fails to complete captcha in time.
func handleCaptchaTimeout(bot *gotgbot.Bot, chatID, userID int64, messageID int64, action string) {
	// Get the attempt first to check for stored messages before deletion
	attempt, _ := db.GetCaptchaAttempt(userID, chatID)
	var storedMsgCount int64
	var attemptID uint
	if attempt != nil {
		attemptID = attempt.ID
		storedMsgCount, _ = db.CountStoredMessagesForAttempt(attempt.ID)
	}

	// Atomic delete - if this returns 0 rows, another handler already processed it
	deleted, err := db.DeleteCaptchaAttemptAtomic(userID, chatID)
	if err != nil || !deleted {
		log.Debugf("[Captcha] Timeout handler skipped - attempt already handled for user %d in chat %d", userID, chatID)
		return
	}

	// Clean up stored messages if we had an attempt
	if attemptID > 0 {
		_ = db.DeleteStoredMessagesForAttempt(attemptID)
	}

	// Delete the captcha message
	_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, messageID)

	// Get user info for the failure message
	member, err := bot.GetChatMember(chatID, userID, nil)
	var userName string
	if err == nil {
		user := member.GetUser()
		if user.FirstName != "" {
			userName = user.FirstName
		} else {
			userName = "User"
		}
	} else {
		userName = "User"
	}

	// Send failure message with action taken and stored message info
	tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: &gotgbot.Chat{Id: chatID}}))

	var failureMsg string
	if storedMsgCount > 0 {
		// Get the action-specific translation key
		var actionKey string
		switch action {
		case "ban":
			actionKey, _ = tr.GetString("captcha_action_banned")
		case "mute":
			actionKey, _ = tr.GetString("captcha_action_muted")
		default:
			actionKey, _ = tr.GetString("captcha_action_kicked")
		}

		template, _ := tr.GetString("captcha_timeout_with_messages")
		failureMsg = fmt.Sprintf(template, helpers.MentionHtml(userID, userName), actionKey, storedMsgCount)
	} else {
		// Use action-specific failure message
		var msgKey string
		switch action {
		case "ban":
			msgKey = "captcha_timeout_failure_banned"
		case "mute":
			msgKey = "captcha_timeout_failure_muted"
		default:
			msgKey = "captcha_timeout_failure_kicked"
		}

		template, _ := tr.GetString(msgKey)
		failureMsg = fmt.Sprintf(template, helpers.MentionHtml(userID, userName))
	}

	// Send the failure message
	sent, err := helpers.SendMessageWithErrorHandling(bot, chatID, failureMsg, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Errorf("Failed to send captcha failure message: %v", err)
	}

	// Delete the failure message after 30 seconds with context
	if sent != nil {
		go func(msgID int64, cID int64) {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()

			timer := time.NewTimer(30 * time.Second)
			defer timer.Stop()

			select {
			case <-timer.C:
				_ = helpers.DeleteMessageWithErrorHandling(bot, cID, msgID)
			case <-ctx.Done():
				// Context cancelled, clean up and exit
				return
			}
		}(sent.MessageId, chatID)
	}

	// Execute the failure action
	switch action {
	case "kick":
		// First ban the user
		_, err := bot.BanChatMember(chatID, userID, nil)
		if err != nil {
			log.Errorf("Failed to ban user %d for kick: %v", userID, err)
			return
		}
		// Then immediately unban to achieve "kick" effect
		_, err = bot.UnbanChatMember(chatID, userID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: false})
		if err != nil {
			log.Errorf("Failed to unban user %d after kick: %v", userID, err)
		}
	case "ban":
		_, err := bot.BanChatMember(chatID, userID, nil)
		if err != nil {
			log.Errorf("Failed to ban user %d: %v", userID, err)
		}
	case "mute":
		// Explicitly mute the user (don't rely on initial mute from greetings)
		_, muteErr := bot.RestrictChatMember(chatID, userID, gotgbot.ChatPermissions{
			CanSendMessages:       false,
			CanSendPhotos:         false,
			CanSendVideos:         false,
			CanSendAudios:         false,
			CanSendDocuments:      false,
			CanSendVideoNotes:     false,
			CanSendVoiceNotes:     false,
			CanAddWebPagePreviews: false,
			CanChangeInfo:         false,
			CanInviteUsers:        false,
			CanPinMessages:        false,
			CanManageTopics:       false,
			CanSendPolls:          false,
			CanSendOtherMessages:  false,
		}, nil)
		if muteErr != nil {
			log.Errorf("[Captcha] Failed to mute user %d in chat %d: %v", userID, chatID, muteErr)
		}
		// Store for auto-unmute in 24 hours
		unmuteAt := time.Now().Add(24 * time.Hour)
		if err := db.CreateMutedUser(userID, chatID, unmuteAt); err != nil {
			log.Errorf("[Captcha] Failed to store muted user for auto-unmute: %v", err)
		}
		log.Infof("User %d muted in chat %d, will be unmuted at %s", userID, chatID, unmuteAt.Format(time.RFC3339))
	}
}

// captchaVerifyCallback handles captcha answer button clicks.
// Verifies if the selected answer is correct and takes appropriate action.
func (moduleStruct) captchaVerifyCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := query.From

	// Parse callback data: captcha_verify.{attempt_id}.{user_id}.{answer}
	parts := strings.Split(query.Data, ".")
	if len(parts) != 4 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_data")
		_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	attemptID64, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_attempt")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	targetUserID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Check if this is the correct user
	if user.Id != targetUserID {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_not_for_you")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	selectedAnswer := parts[3]

	// Get the captcha attempt and ensure IDs match
	attempt, err := db.GetCaptchaAttempt(targetUserID, chat.Id)
	if err != nil || attempt == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_expired_or_not_found")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	if attempt.ID != uint(attemptID64) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_attempt_not_valid")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	settings, _ := db.GetCaptchaSettings(chat.Id)

	// Check if answer is correct
	if selectedAnswer == attempt.Answer {
		// Correct answer - unmute the user
		_, err = chat.RestrictMember(bot, targetUserID, gotgbot.ChatPermissions{
			CanSendMessages:       true,
			CanSendPhotos:         true,
			CanSendVideos:         true,
			CanSendAudios:         true,
			CanSendDocuments:      true,
			CanSendVideoNotes:     true,
			CanSendVoiceNotes:     true,
			CanAddWebPagePreviews: true,
			CanChangeInfo:         false,
			CanInviteUsers:        true,
			CanPinMessages:        false,
			CanManageTopics:       false,
			CanSendPolls:          true,
			CanSendOtherMessages:  true,
		}, nil)

		if err != nil {
			log.Errorf("Failed to unmute user %d: %v", targetUserID, err)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_failed_verify")
			_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}

		// Retrieve and display stored messages before deletion
		storedMessages, err := db.GetStoredMessagesForAttempt(attempt.ID)
		if err == nil && len(storedMessages) > 0 {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			// Create a summary of what the user tried to send
			var messageTypes []string
			for _, msg := range storedMessages {
				msgTypeStr := messageTypeToString(tr, msg.MessageType)
				if !slices.Contains(messageTypes, msgTypeStr) {
					messageTypes = append(messageTypes, msgTypeStr)
				}
			}
			summaryText, _ := tr.GetString("captcha_stored_messages_summary",
				i18n.TranslationParams{
					"user":  helpers.MentionHtml(targetUserID, user.FirstName),
					"count": len(storedMessages),
					"types": strings.Join(messageTypes, ", "),
				})

			// Send summary message that auto-deletes after 30 seconds
			summaryMsg, _ := helpers.SendMessageWithErrorHandling(bot, chat.Id, summaryText, &gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
			})

			// Auto-delete the summary after 30 seconds
			if summaryMsg != nil {
				go func(chatID, msgID int64) {
					// Create context with timeout
					ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
					defer cancel()

					timer := time.NewTimer(30 * time.Second)
					defer timer.Stop()

					select {
					case <-timer.C:
						_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, msgID)
					case <-ctx.Done():
						log.Debugf("Summary message deletion cancelled for message %d", msgID)
						return
					}
				}(chat.Id, summaryMsg.MessageId)
			}
		}

		// Clean up stored messages
		_ = db.DeleteStoredMessagesForAttempt(attempt.ID)

		// Delete the captcha message
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, attempt.MessageID)

		// Delete the attempt from database
		_ = db.DeleteCaptchaAttempt(targetUserID, chat.Id)

		// Send success message
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		msgTemplate, _ := tr.GetString("greetings_captcha_verified_success")
		successMsg := fmt.Sprintf(msgTemplate, helpers.MentionHtml(targetUserID, user.FirstName))
		sent, _ := helpers.SendMessageWithErrorHandling(bot, chat.Id, successMsg, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})

		// Delete success message after 5 seconds with timeout
		if sent != nil {
			go func(msgID int64, cID int64) {
				// Create context with timeout
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				timer := time.NewTimer(5 * time.Second)
				defer timer.Stop()

				select {
				case <-timer.C:
					_ = helpers.DeleteMessageWithErrorHandling(bot, cID, msgID)
				case <-ctx.Done():
					log.Debugf("Success message deletion cancelled for message %d", msgID)
					return
				}
			}(sent.MessageId, chat.Id)
		}

		// Send welcome message after successful verification
		if err = SendWelcomeMessage(bot, ctx, targetUserID, user.FirstName); err != nil {
			log.Errorf("Failed to send welcome message after captcha verification: %v", err)
		}

		text, _ := tr.GetString("captcha_verified_success_msg")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err

	} else {
		// Wrong answer - increment attempts
		log.Debugf("[Captcha] Wrong answer for user %d in chat %d: selected=%q, expected=%q",
			targetUserID, chat.Id, selectedAnswer, attempt.Answer)
		attempt, err = db.IncrementCaptchaAttempts(targetUserID, chat.Id)
		if err != nil {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_error_processing")
			_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}

		if attempt.Attempts >= settings.MaxAttempts {
			// Max attempts reached - execute failure action
			handleCaptchaTimeout(bot, chat.Id, targetUserID, attempt.MessageID, settings.FailureAction)

			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			actionText, _ := tr.GetString("captcha_action_kicked")
			switch settings.FailureAction {
			case "ban":
				actionText, _ = tr.GetString("captcha_action_banned")
			case "mute":
				actionText, _ = tr.GetString("captcha_action_muted")
			}

			text, _ := tr.GetString("captcha_wrong_answer_final", i18n.TranslationParams{"s": actionText})
			_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
				Text:      text,
				ShowAlert: true,
			})
			return err
		}

		remainingAttempts := settings.MaxAttempts - attempt.Attempts
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_wrong_answer_remaining", i18n.TranslationParams{"d": remainingAttempts})
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
			Text:      text,
			ShowAlert: true,
		})
		return err
	}
}

// captchaRefreshCallback handles the refresh button for text captchas.
// Generates a new captcha image when users can't read the current one.
// Uses send-first pattern for atomic refresh to prevent stuck states.
func (moduleStruct) captchaRefreshCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := query.From

	// Parse callback data: captcha_refresh.{attempt_id}.{user_id}
	parts := strings.Split(query.Data, ".")
	if len(parts) != 3 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_refresh")
		_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	attemptID64, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_attempt")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	targetUserID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Check if this is the correct user
	if user.Id != targetUserID {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_not_for_you")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Cooldown: block rapid refreshes per user+chat
	cooldownKey := fmt.Sprintf("alita:captcha:refresh:cooldown:%d:%d", chat.Id, targetUserID)
	if cache.Marshal != nil {
		if exists, _ := cache.Marshal.Get(cache.Context, cooldownKey, new(bool)); exists != nil {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_wait_refresh")
			_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}
	}

	// Get the existing attempt and verify attempt ID
	attempt, err := db.GetCaptchaAttempt(targetUserID, chat.Id)
	if err != nil || attempt == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_expired_or_not_found")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	if attempt.ID != uint(attemptID64) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_attempt_not_valid")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Enforce per-attempt refresh cap
	if attempt.RefreshCount >= captchaMaxRefreshes {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_refresh_limit_reached")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Store old message ID for cleanup later (send-first pattern)
	oldMessageID := attempt.MessageID

	// Determine current mode and whether image flow applies
	settings, _ := db.GetCaptchaSettings(chat.Id)

	// Generate a new image/options based on current mode
	var newAnswer string
	var imageBytes []byte
	var options []string
	var genErr error
	if settings != nil && settings.CaptchaMode == "text" {
		newAnswer, imageBytes, options, genErr = generateTextCaptcha()
	} else {
		newAnswer, imageBytes, options, genErr = generateMathImageCaptcha()
	}
	if genErr != nil || imageBytes == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_failed_generate")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Build keyboard with new options and refresh button
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, option := range options {
		button := gotgbot.InlineKeyboardButton{
			Text:         option,
			CallbackData: fmt.Sprintf("captcha_verify.%d.%d.%s", attempt.ID, targetUserID, option),
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{button})
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	refreshBtnText, _ := tr.GetString("captcha_refresh_button")
	buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
		Text:         refreshBtnText,
		CallbackData: fmt.Sprintf("captcha_refresh.%d.%d", attempt.ID, targetUserID),
	}})

	keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}

	// Build caption for the new message
	remainingMinutes := int(time.Until(attempt.ExpiresAt).Minutes())
	if remainingMinutes < 0 {
		remainingMinutes = 0
	}
	var caption string
	if settings != nil && settings.CaptchaMode == "text" {
		template, _ := tr.GetString("captcha_welcome_text_detailed")
		caption = fmt.Sprintf(
			template,
			helpers.MentionHtml(targetUserID, user.FirstName), remainingMinutes,
		)
	} else {
		template, _ := tr.GetString("captcha_welcome_math_detailed")
		caption = fmt.Sprintf(
			template,
			helpers.MentionHtml(targetUserID, user.FirstName), remainingMinutes,
		)
	}

	// Step 1: Send new message FIRST (before any deletion) - atomic refresh pattern
	sent, sendErr := bot.SendPhoto(chat.Id, gotgbot.InputFileByReader("captcha.png", bytes.NewReader(imageBytes)), &gotgbot.SendPhotoOpts{
		Caption:     caption,
		ParseMode:   helpers.HTML,
		ReplyMarkup: keyboard,
	})
	if sendErr != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_failed_send")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Step 2: Update DB with new answer and message ID
	if _, err := db.UpdateCaptchaAttemptOnRefreshByID(attempt.ID, newAnswer, sent.MessageId); err != nil {
		log.Errorf("Failed to update captcha attempt on refresh: %v", err)
		// Rollback: delete the new message since DB update failed
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, sent.MessageId)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_internal_update_error")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Step 3: Delete old message LAST (best effort, in background)
	go func(chatID, msgID int64) {
		if err := helpers.DeleteMessageWithErrorHandling(bot, chatID, msgID); err != nil {
			log.Warnf("[CaptchaRefresh] Failed to delete old message %d in chat %d: %v", msgID, chatID, err)
		}
	}(chat.Id, oldMessageID)

	// Set cooldown
	if cache.Marshal != nil {
		_ = cache.Marshal.Set(cache.Context, cooldownKey, true, store.WithExpiration(time.Duration(captchaRefreshCooldownS)*time.Second))
	}

	tr = i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_refresh_success")
	_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
	return err
}

// handlePendingCaptchaMessage intercepts messages from users with pending captcha verification.
// Stores their messages and deletes them to prevent spam while they complete verification.
func (moduleStruct) handlePendingCaptchaMessage(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	// Skip if this is not a group chat
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.ContinueGroups
	}

	// Skip if user is an admin
	if chat_status.IsUserAdmin(bot, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	// Check if user has a pending captcha attempt
	attempt, err := db.GetCaptchaAttempt(user.Id, chat.Id)
	if err != nil {
		log.Errorf("Failed to check captcha attempt for user %d: %v", user.Id, err)
		return ext.ContinueGroups
	}

	// If no pending captcha, continue normal processing
	if attempt == nil {
		return ext.ContinueGroups
	}

	// Store the message content based on type
	var messageType int
	var content, fileID, caption string

	switch {
	case msg.Text != "":
		messageType = db.TEXT
		content = msg.Text
	case msg.Sticker != nil:
		messageType = db.STICKER
		fileID = msg.Sticker.FileId
	case msg.Document != nil:
		messageType = db.DOCUMENT
		fileID = msg.Document.FileId
		caption = msg.Caption
	case msg.Photo != nil:
		messageType = db.PHOTO
		if len(msg.Photo) > 0 {
			fileID = msg.Photo[len(msg.Photo)-1].FileId // Get highest resolution
		}
		caption = msg.Caption
	case msg.Audio != nil:
		messageType = db.AUDIO
		fileID = msg.Audio.FileId
		caption = msg.Caption
	case msg.Voice != nil:
		messageType = db.VOICE
		fileID = msg.Voice.FileId
		caption = msg.Caption
	case msg.Video != nil:
		messageType = db.VIDEO
		fileID = msg.Video.FileId
		caption = msg.Caption
	case msg.VideoNote != nil:
		messageType = db.VideoNote
		fileID = msg.VideoNote.FileId
	default:
		// Unknown message type, skip storing but still delete
		messageType = db.TEXT
		content = "[Unsupported message type]"
	}

	// Store the message
	err = db.StoreMessageForCaptcha(user.Id, chat.Id, attempt.ID, messageType, content, fileID, caption)
	if err != nil {
		log.Errorf("Failed to store message for user %d with pending captcha: %v", user.Id, err)
	}

	// Delete the message to prevent spam
	_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId)

	// End processing - don't let this message continue through other handlers
	return ext.EndGroups
}

// LoadCaptcha registers all captcha module handlers with the dispatcher.
func LoadCaptcha(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(captchaModule.moduleName, true)

	// Message handler for users with pending captcha (high priority to intercept early)
	dispatcher.AddHandlerToGroup(handlers.NewMessage(nil, captchaModule.handlePendingCaptchaMessage), -10)

	// Commands
	dispatcher.AddHandler(handlers.NewCommand("captcha", captchaModule.captchaCommand))
	dispatcher.AddHandler(handlers.NewCommand("captchamode", captchaModule.captchaModeCommand))
	dispatcher.AddHandler(handlers.NewCommand("captchatime", captchaModule.captchaTimeCommand))
	dispatcher.AddHandler(handlers.NewCommand("captchaaction", captchaModule.captchaActionCommand))
	dispatcher.AddHandler(handlers.NewCommand("captchamaxattempts", captchaModule.captchaMaxAttemptsCommand))

	// Admin commands for managing stored messages
	dispatcher.AddHandler(handlers.NewCommand("captchapending", captchaModule.viewPendingMessages))
	dispatcher.AddHandler(handlers.NewCommand("captchaclear", captchaModule.clearPendingMessages))

	// Callbacks
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("captcha_verify."), captchaModule.captchaVerifyCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("captcha_refresh."), captchaModule.captchaRefreshCallback))

	// Start periodic cleanup of expired attempts with proper context management
	go func() {
		// Create base context that can be cancelled when the module shuts down
		baseCtx, baseCancel := context.WithCancel(context.Background())
		defer baseCancel()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				func() {
					ctx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
					defer cancel()

					done := make(chan error, 1)
					go func() {
						// Get expired attempts with message IDs before cleanup
						attempts, err := db.GetExpiredCaptchaAttempts()
						if err != nil {
							done <- err
							return
						}

						if len(attempts) == 0 {
							done <- nil
							return
						}

						log.Infof("[CaptchaCleanup] Processing %d expired captcha attempts", len(attempts))

						// Delete Telegram messages first (with retry for transient errors)
						var cleanedIDs []uint
						for _, attempt := range attempts {
							select {
							case <-ctx.Done():
								log.Warn("[CaptchaCleanup] Cleanup cancelled due to timeout")
								done <- ctx.Err()
								return
							default:
							}

							if attempt.MessageID > 0 && captchaBotRef != nil {
								// Retry up to captchaCleanupRetries times
								deleted := false
								for retry := 0; retry < captchaCleanupRetries; retry++ {
									err := helpers.DeleteMessageWithErrorHandling(captchaBotRef, attempt.ChatID, attempt.MessageID)
									if err == nil || isPermanentTelegramError(err) {
										deleted = true
										break
									}
									log.Warnf("[CaptchaCleanup] Retry %d/%d deleting message %d in chat %d: %v",
										retry+1, captchaCleanupRetries, attempt.MessageID, attempt.ChatID, err)
									time.Sleep(time.Second * time.Duration(retry+1))
								}
								if !deleted {
									log.Errorf("[CaptchaCleanup] Failed to delete message %d in chat %d after %d retries",
										attempt.MessageID, attempt.ChatID, captchaCleanupRetries)
								}
							}

							// Delete stored messages for this attempt
							_ = db.DeleteStoredMessagesForAttempt(attempt.ID)
							cleanedIDs = append(cleanedIDs, attempt.ID)
						}

						// Delete DB records in batch
						if len(cleanedIDs) > 0 {
							count, err := db.DeleteCaptchaAttemptsByIDs(cleanedIDs)
							if err != nil {
								log.Errorf("[CaptchaCleanup] Failed to delete DB records: %v", err)
								done <- err
								return
							}
							if count > 0 {
								log.Infof("[CaptchaCleanup] Cleaned up %d expired captcha attempts", count)
							}
						}
						done <- nil
					}()

					select {
					case err := <-done:
						if err != nil {
							log.Errorf("[CaptchaCleanup] Cleanup error: %v", err)
						}
					case <-ctx.Done():
						log.Warn("[CaptchaCleanup] Cleanup operation timed out")
					}
				}()
			case <-baseCtx.Done():
				// Module shutdown, exit gracefully
				log.Info("Captcha cleanup goroutine shutting down")
				return
			}
		}
	}()

	// Start periodic unmute job for users muted due to captcha failure
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if captchaBotRef == nil {
				continue
			}

			users, err := db.GetUsersToUnmute()
			if err != nil {
				log.Errorf("[CaptchaUnmute] Failed to get users to unmute: %v", err)
				continue
			}

			if len(users) == 0 {
				continue
			}

			log.Infof("[CaptchaUnmute] Processing %d users to unmute", len(users))

			var unmuteIDs []uint
			for _, user := range users {
				// Unmute the user by granting standard member permissions (matching success unmute)
				_, err := captchaBotRef.RestrictChatMember(user.ChatID, user.UserID, gotgbot.ChatPermissions{
					CanSendMessages:       true,
					CanSendAudios:         true,
					CanSendDocuments:      true,
					CanSendPhotos:         true,
					CanSendVideos:         true,
					CanSendVideoNotes:     true,
					CanSendVoiceNotes:     true,
					CanSendPolls:          true,
					CanSendOtherMessages:  true,
					CanAddWebPagePreviews: true,
					CanChangeInfo:         false, // Match success unmute permissions
					CanInviteUsers:        true,
					CanPinMessages:        false, // Match success unmute permissions
					CanManageTopics:       false, // Match success unmute permissions
				}, nil)
				if err != nil {
					if isPermanentUnmuteError(err) {
						// Permanent error - user left, chat deleted, etc. - remove from DB
						log.Infof("[CaptchaUnmute] User %d no longer in chat %d, removing from muted list: %v",
							user.UserID, user.ChatID, err)
						unmuteIDs = append(unmuteIDs, user.ID)
					} else {
						// Transient error - will retry on next tick
						log.Warnf("[CaptchaUnmute] Failed to unmute user %d in chat %d (will retry): %v",
							user.UserID, user.ChatID, err)
						// Don't add to unmuteIDs - will retry on next tick
					}
				} else {
					// Success - add to cleanup list
					unmuteIDs = append(unmuteIDs, user.ID)
				}
			}

			// Clean up DB records
			if len(unmuteIDs) > 0 {
				count, err := db.DeleteMutedUsersByIDs(unmuteIDs)
				if err != nil {
					log.Errorf("[CaptchaUnmute] Failed to delete muted user records: %v", err)
				} else {
					log.Infof("[CaptchaUnmute] Unmuted %d users", count)
				}
			}
		}
	}()
}
