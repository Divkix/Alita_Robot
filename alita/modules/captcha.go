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
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/mojocn/base64Captcha"
	log "github.com/sirupsen/logrus"
)

var captchaModule = moduleStruct{moduleName: "Captcha"}

// fixedStringCaptchaDriver wraps DriverString so captcha.Generate renders
// the exact provided content instead of randomly sampling from Source.
type fixedStringCaptchaDriver struct {
	*base64Captcha.DriverString
	content string
}

func (d *fixedStringCaptchaDriver) GenerateIdQuestionAnswer() (id, q, a string) {
	id = base64Captcha.RandomId()
	return id, d.content, d.content
}

// noopCaptchaStore avoids persisting unused answers for image-only generation paths.
type noopCaptchaStore struct{}

func (noopCaptchaStore) Set(id string, value string) error {
	return nil
}

func (noopCaptchaStore) Get(id string, clear bool) string {
	return ""
}

func (noopCaptchaStore) Verify(id, answer string, clear bool) bool {
	return false
}

func newMathImageCaptchaDriver(question string) *fixedStringCaptchaDriver {
	return &fixedStringCaptchaDriver{
		DriverString: base64Captcha.NewDriverString(
			80,            // height
			240,           // width (wider for math expression)
			0,             // noiseCount
			0,             // showLineOptions - keep math operators readable
			len(question), // source length
			question,      // source string (the question itself)
			nil,           // bgColor
			nil,           // fonts
			[]string{},    // fontsArray
		),
		content: question,
	}
}

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
	case db.VIDEO_NOTE:
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

// Process-wide captcha worker lifecycle.
var (
	captchaLifecycleOnce sync.Once
	captchaLifecycleErr  error
	captchaLifecycleMu   sync.Mutex
	captchaLifecycleWG   sync.WaitGroup
	captchaLifecycleCtx  context.Context
	captchaLifecycleStop context.CancelFunc
	captchaLifecycleDone bool
	errCaptchaDisabled   = errors.New("captcha disabled")
	errCaptchaStopped    = errors.New("captcha lifecycle already stopped")
)

// StartCaptchaLifecycle restores persisted attempts and starts the cleanup workers.
// It must run during process startup, before updates are accepted.
func StartCaptchaLifecycle(bot *gotgbot.Bot) error {
	if bot == nil {
		return errors.New("captcha lifecycle requires a bot")
	}
	captchaLifecycleOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		captchaLifecycleMu.Lock()
		if captchaLifecycleDone {
			captchaLifecycleErr = errCaptchaStopped
			captchaLifecycleMu.Unlock()
			cancel()
			return
		}
		captchaLifecycleCtx = ctx
		captchaLifecycleStop = cancel
		captchaLifecycleMu.Unlock()

		captchaLifecycleErr = runOrphanedCaptchaRecovery(bot)
		if captchaLifecycleErr == nil {
			startCaptchaWorkers(bot)
		} else {
			StopCaptchaLifecycle()
		}
	})

	captchaLifecycleMu.Lock()
	defer captchaLifecycleMu.Unlock()
	if captchaLifecycleErr != nil {
		return captchaLifecycleErr
	}
	if captchaLifecycleDone {
		return errCaptchaStopped
	}
	return nil
}

// StopCaptchaLifecycle stops and joins periodic workers and challenge timers.
func StopCaptchaLifecycle() {
	captchaLifecycleMu.Lock()
	captchaLifecycleDone = true
	stop := captchaLifecycleStop
	captchaLifecycleMu.Unlock()
	if stop != nil {
		stop()
	}
	captchaLifecycleWG.Wait()
}

func startCaptchaLifecycleTask(task func(context.Context)) bool {
	captchaLifecycleMu.Lock()
	if captchaLifecycleCtx == nil || captchaLifecycleDone {
		captchaLifecycleMu.Unlock()
		return false
	}
	ctx := captchaLifecycleCtx
	captchaLifecycleWG.Add(1)
	captchaLifecycleMu.Unlock()

	go func() {
		defer captchaLifecycleWG.Done()
		task(ctx)
	}()
	return true
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

// isPermanentUnmuteError reports errors where permission restoration is no
// longer needed. Access and permission failures remain queued for retry.
func isPermanentUnmuteError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	permanentErrors := []string{
		"user not found",
		"user_not_participant",
		"group chat was deactivated",
		"user is an administrator",
	}
	for _, pe := range permanentErrors {
		if strings.Contains(errStr, pe) {
			return true
		}
	}
	return false
}

// runOrphanedCaptchaRecovery resumes valid attempts and finalizes expired ones.
func runOrphanedCaptchaRecovery(bot *gotgbot.Bot) error {
	log.Info("[CaptchaRecovery] Starting orphaned captcha recovery...")

	attempts, err := captcha.GetAllPendingCaptchaAttempts()
	if err != nil {
		return fmt.Errorf("load pending captcha attempts: %w", err)
	}

	if len(attempts) == 0 {
		log.Info("[CaptchaRecovery] No orphaned captchas found")
		return nil
	}

	log.Infof("[CaptchaRecovery] Found %d orphaned captcha attempts to process", len(attempts))

	var (
		expiredCount  int
		resumedCount  int
		releasedCount int
	)

	for _, attempt := range attempts {
		if attempt.MessageID <= 0 {
			released, err := releaseIncompleteCaptchaAttempt(bot, attempt)
			if err != nil {
				log.Warnf("[CaptchaRecovery] Failed to release incomplete attempt %d: %v", attempt.ID, err)
			}
			if released {
				releasedCount++
			}
			continue
		}
		if time.Now().Before(attempt.ExpiresAt) {
			scheduleCaptchaTimeout(bot, attempt)
			resumedCount++
			continue
		}

		handled, err := expireCaptchaAttempt(bot, attempt)
		if err != nil {
			// Keep the attempt so the periodic worker can retry it. A single
			// inaccessible chat must not prevent the bot from starting.
			log.Warnf("[CaptchaRecovery] Failed to expire attempt %d; will retry: %v", attempt.ID, err)
		} else if handled {
			expiredCount++
		}
	}

	log.Infof("[CaptchaRecovery] Completed: %d expired, %d resumed, %d incomplete released", expiredCount, resumedCount, releasedCount)
	return nil
}

func releaseIncompleteCaptchaAttempt(bot *gotgbot.Bot, attempt *db.CaptchaAttempts) (bool, error) {
	claimed, deleteErr := captcha.ReleaseCaptchaAttemptAtomic(attempt.ID, attempt.UserID, attempt.ChatID)
	if deleteErr != nil || !claimed {
		return false, deleteErr
	}
	unmuteErr := unmuteCaptchaUser(bot, attempt.ChatID, attempt.UserID)
	if unmuteErr == nil || isPermanentUnmuteError(unmuteErr) {
		return true, errors.Join(unmuteErr, captcha.DeleteMutedUser(attempt.UserID, attempt.ChatID))
	}
	return true, unmuteErr
}

// secureIntn returns a cryptographically secure random integer in [0, max).
// If max <= 0, it returns 0.
func secureIntn(max int) int {
	if max <= 0 {
		return 0
	}
	// Use crypto/rand.Int for unbiased secure random selection
	// Bounded retry to avoid CPU starvation if entropy source fails persistently.
	const maxRetries = 10
	for i := 0; i < maxRetries; i++ {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
		if err == nil {
			return int(n.Int64())
		}
	}
	log.Error("[Captcha] secureIntn: exhausted retries for crypto/rand.Int, returning 0")
	return 0
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
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return ext.EndGroups
	}

	// Check admin permissions
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	// Parse target user from command
	if len(ctx.Args()) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_pending_usage")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	// Get user ID from mention or ID
	targetUserID := extraction.ExtractUser(bot, ctx)
	if targetUserID == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	// Get stored messages for user
	messages, err := captcha.GetStoredMessagesForUser(targetUserID, chat.Id)
	if err != nil || len(messages) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_no_pending_messages")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	// Build response
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var response strings.Builder
	headerText, _ := tr.GetString("captcha_pending_messages_header")
	fmt.Fprintf(&response, headerText, targetUserID)

	for i, msg := range messages {
		typeText, _ := tr.GetString("captcha_pending_message_type")
		fmt.Fprintf(&response, typeText, i+1, messageTypeToString(tr, msg.MessageType))
		if msg.Caption != "" {
			captionText, _ := tr.GetString("captcha_pending_message_caption")
			fmt.Fprintf(&response, captionText, html.EscapeString(msg.Caption))
		}
		if msg.Content != "" && msg.MessageType == db.TEXT {
			preview := msg.Content
			if runes := []rune(preview); len(runes) > 100 {
				preview = string(runes[:100]) + "..."
			}
			contentText, _ := tr.GetString("captcha_pending_message_content")
			fmt.Fprintf(&response, contentText, html.EscapeString(preview))
		}
		timeText, _ := tr.GetString("captcha_pending_message_time")
		fmt.Fprintf(&response, timeText, msg.CreatedAt.Format("15:04:05"))
	}

	_, err = msg.Reply(bot, response.String(), formatting.Shtml())
	return err
}

// clearPendingMessages handles the /captchaclear command to clear stored messages for a user.
// Admins can use this to manually clear pending messages if needed.
func (moduleStruct) clearPendingMessages(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return ext.EndGroups
	}

	// Check admin permissions
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	// Parse target user
	args := ctx.Args()[1:]
	if len(args) < 1 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_clear_usage")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	targetUserID := extraction.ExtractUser(bot, ctx)
	if targetUserID == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	// Delete messages
	err := captcha.DeleteStoredMessagesForUser(targetUserID, chat.Id)
	if err != nil {
		log.Errorf("Failed to delete stored messages for user %d in chat %d: %v", targetUserID, chat.Id, err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_clear_failed")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_clear_success",
		i18n.TranslationParams{"user_id": targetUserID})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

// captchaAdminGate runs the shared permission preamble for captcha setting
// commands: RequireUser → RequireGroup → RequireUserAdmin (and RequireBotAdmin
// when requireBotAdmin is set). Each gate sends its own responder message on
// failure; ok is false the instant any gate fails.
func captchaAdminGate(bot *gotgbot.Bot, ctx *ext.Context, requireBotAdmin bool) (*gotgbot.User, bool) {
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return nil, false
	}
	if !chat_status.RequireGroup(bot, ctx, nil) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return nil, false
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return nil, false
	}
	if requireBotAdmin && !chat_status.RequireBotAdmin(bot, ctx, nil) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return nil, false
	}
	return user, true
}

// captchaCommand handles the /captcha command to enable/disable captcha verification.
// Admins can use this to toggle captcha protection for their group.
func (moduleStruct) captchaCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if _, ok := captchaAdminGate(bot, ctx, true); !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		// Show current status
		settings, err := captcha.GetCaptchaSettings(chat.Id)
		if err != nil {
			log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
			return ext.EndGroups
		}
		status := "disabled"
		if settings.Enabled {
			status = "enabled"
		}

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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

		_, err = msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	switch strings.ToLower(args[0]) {
	case "on", "enable", "yes":
		err := captcha.SetCaptchaEnabled(chat.Id, true)
		if err != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_enable_failed")
			if _, replyErr := msg.Reply(bot, text, nil); replyErr != nil {
				log.Warnf("[Captcha] Failed to send enable error message in chat %d: %v", chat.Id, replyErr)
			}
			return err
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_enabled_success")
		_, err = msg.Reply(bot, text, formatting.Shtml())
		return err

	case "off", "disable", "no":
		err := captcha.SetCaptchaEnabled(chat.Id, false)
		if err != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_disable_failed")
			if _, replyErr := msg.Reply(bot, text, nil); replyErr != nil {
				log.Warnf("[Captcha] Failed to send disable error message in chat %d: %v", chat.Id, replyErr)
			}
			return err
		}
		if err = disableCaptchaForChat(bot, chat.Id); err != nil {
			log.Errorf("[Captcha] Failed to clean up chat %d after disabling captcha: %v", chat.Id, err)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_disable_failed")
			_, _ = msg.Reply(bot, text, nil)
			return err
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_disabled_success")
		_, err = msg.Reply(bot, text, formatting.Shtml())
		return err

	default:
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_usage")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}
}

func disableCaptchaForChat(bot *gotgbot.Bot, chatID int64) error {
	attempts, err := captcha.GetCaptchaAttemptsForChat(chatID)
	if err != nil {
		return err
	}
	mutedUsers, err := captcha.GetMutedUsersForChat(chatID)
	if err != nil {
		return err
	}

	usersToRestore := make(map[int64]struct{}, len(attempts)+len(mutedUsers))
	var cleanupErr error
	for _, attempt := range attempts {
		claimed, err := captcha.ReleaseCaptchaAttemptAtomic(attempt.ID, attempt.UserID, chatID)
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if !claimed {
			continue
		}
		if attempt.MessageID > 0 {
			if err := helpers.DeleteMessageWithErrorHandling(bot, chatID, attempt.MessageID); err != nil && !isPermanentTelegramError(err) {
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
		usersToRestore[attempt.UserID] = struct{}{}
	}
	for _, user := range mutedUsers {
		usersToRestore[user.UserID] = struct{}{}
	}
	for userID := range usersToRestore {
		if err := restoreCaptchaPermissions(bot, chatID, userID); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

// captchaModeCommand handles the /captchamode command to set captcha type.
// Admins can choose between math and text captcha modes.
func (moduleStruct) captchaModeCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if _, ok := captchaAdminGate(bot, ctx, false); !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_mode_specify")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	mode := strings.ToLower(args[0])
	if mode != "math" && mode != "text" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_mode_invalid")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	err := captcha.SetCaptchaMode(chat.Id, mode)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		var text string
		if errors.Is(err, captcha.ErrInvalidCaptchaMode) {
			text, _ = tr.GetString("captcha_invalid_mode_error")
		} else {
			text, _ = tr.GetString("captcha_mode_failed")
		}
		if _, replyErr := msg.Reply(bot, text, formatting.Shtml()); replyErr != nil {
			log.Warnf("[Captcha] Failed to send mode error message in chat %d: %v", chat.Id, replyErr)
		}
		return err
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	modeDesc, _ := tr.GetString("captcha_mode_math_desc")
	if mode == "text" {
		modeDesc, _ = tr.GetString("captcha_mode_text_desc")
	}

	textTemplate, _ := tr.GetString("captcha_mode_set_formatted")
	text := fmt.Sprintf(textTemplate, mode, modeDesc)
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

// captchaTimeCommand handles the /captchatime command to set verification timeout.
// Admins can set how long users have to complete the captcha (1-10 minutes).
func (moduleStruct) captchaTimeCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if _, ok := captchaAdminGate(bot, ctx, false); !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_timeout_specify")
		_, err := msg.Reply(bot, text, nil)
		return err
	}

	timeout64, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || timeout64 < 1 || timeout64 > 10 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_timeout_invalid")
		_, err = msg.Reply(bot, text, nil)
		return err
	}
	timeout := int(timeout64)

	err = captcha.SetCaptchaTimeout(chat.Id, timeout)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		var text string
		if errors.Is(err, captcha.ErrInvalidTimeout) {
			text, _ = tr.GetString("captcha_timeout_range_error")
		} else {
			text, _ = tr.GetString("captcha_timeout_failed")
		}
		if _, replyErr := msg.Reply(bot, text, formatting.Shtml()); replyErr != nil {
			log.Warnf("[Captcha] Failed to send timeout error message in chat %d: %v", chat.Id, replyErr)
		}
		return err
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_timeout_set_success", i18n.TranslationParams{"d": timeout})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

// captchaActionCommand handles the /captchaaction command to set failure action.
// Admins can choose what happens when users fail the captcha: kick, ban, or mute.
func (moduleStruct) captchaActionCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if _, ok := captchaAdminGate(bot, ctx, false); !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_action_specify")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	action := strings.ToLower(args[0])
	if action != "kick" && action != "ban" && action != "mute" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_action_invalid")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	err := captcha.SetCaptchaFailureAction(chat.Id, action)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		var text string
		if errors.Is(err, captcha.ErrInvalidFailureAction) {
			text, _ = tr.GetString("captcha_invalid_action_error")
		} else {
			text, _ = tr.GetString("captcha_action_failed")
		}
		if _, replyErr := msg.Reply(bot, text, formatting.Shtml()); replyErr != nil {
			log.Warnf("[Captcha] Failed to send action error message in chat %d: %v", chat.Id, replyErr)
		}
		return err
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_action_set_success", i18n.TranslationParams{"s": action})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

// captchaMaxAttemptsCommand handles the /captchamaxattempts command to set max verification attempts.
// Admins can set how many wrong answers are allowed before taking action (1-10 attempts).
func (moduleStruct) captchaMaxAttemptsCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if _, ok := captchaAdminGate(bot, ctx, true); !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if len(args) == 0 {
		settings, err := captcha.GetCaptchaSettings(chat.Id)
		if err != nil {
			log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
			return ext.EndGroups
		}
		text, _ := tr.GetString("captcha_max_attempts_current", map[string]any{
			"attempts": settings.MaxAttempts,
		})
		_, err = msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	attempts64, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || attempts64 < 1 || attempts64 > 10 {
		text, _ := tr.GetString("captcha_max_attempts_invalid")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}
	attempts := int(attempts64)

	if err := captcha.SetCaptchaMaxAttempts(chat.Id, attempts); err != nil {
		log.Errorf("Failed to set captcha max attempts: %v", err)
		text, _ := tr.GetString("captcha_internal_error")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	text, _ := tr.GetString("captcha_max_attempts_set", map[string]any{
		"attempts": attempts,
	})
	_, err = msg.Reply(bot, text, formatting.Shtml())
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
		question = formatMathQuestion(a, b, operation)
	case "-":
		a = secureIntn(50) + 20
		b = secureIntn(a) + 1
		answer = a - b
		question = formatMathQuestion(a, b, operation)
	case "*":
		a = secureIntn(12) + 1
		b = secureIntn(12) + 1
		answer = a * b
		question = formatMathQuestion(a, b, operation)
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

func formatMathQuestion(a, b int, operation string) string {
	operator := operation
	if operation == "*" {
		// Use ASCII x instead of the multiplication symbol to avoid missing glyphs
		// in some embedded captcha fonts.
		operator = "x"
	}
	return fmt.Sprintf("%d %s %d", a, operator, b)
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
		log.Error("[Captcha] Generated text answer was missing from its options, regenerating")
		return generateTextCaptcha() // Retry
	}

	return answer, imageBytes, options, nil
}

// generateMathImageCaptcha generates a math captcha image using custom math generation
// for reliable answer matching. Uses the existing generateMathCaptcha logic.
func generateMathImageCaptcha() (string, []byte, []string, error) {
	// Use our reliable math generation
	question, answer, options := generateMathCaptcha()

	// DriverString normally samples random characters from Source on Generate.
	// Wrap it so the rendered image always matches the computed math question.
	driver := newMathImageCaptchaDriver(question)

	captcha := base64Captcha.NewCaptcha(driver, noopCaptchaStore{})
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
		log.Error("[Captcha] Generated math answer was missing from its options, regenerating")
		return generateMathImageCaptcha() // Retry
	}

	return answer, imageBytes, options, nil
}

// buildCaptchaKeyboard builds the inline keyboard for a captcha challenge:
// one button per answer option (captcha_verify) and, when includeRefresh is
// set, a trailing refresh button (captcha_refresh) labelled refreshBtnText.
func buildCaptchaKeyboard(attemptID uint, userID int64, refreshCount int, options []string, includeRefresh bool, refreshBtnText string) gotgbot.InlineKeyboardMarkup {
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, option := range options {
		button := gotgbot.InlineKeyboardButton{
			Text: option,
			CallbackData: encodeCallbackData(
				"captcha_verify",
				map[string]string{
					"a": fmt.Sprint(attemptID),
					"r": fmt.Sprint(refreshCount),
					"u": fmt.Sprint(userID),
					"s": option,
				},
			),
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{button})
	}
	if includeRefresh {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
			Text: refreshBtnText,
			CallbackData: encodeCallbackData(
				"captcha_refresh",
				map[string]string{
					"a": fmt.Sprint(attemptID),
					"r": fmt.Sprint(refreshCount),
					"u": fmt.Sprint(userID),
				},
			),
		}})
	}
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// SendCaptcha sends a captcha challenge to a new member.
// Called when a new member joins a group with captcha enabled.
//
//nolint:gocyclo // The ordered mute/send/persist rollback state machine is intentionally cohesive.
func SendCaptcha(bot *gotgbot.Bot, ctx *ext.Context, userID int64, userName string) (err error) {
	var preAttempt *db.CaptchaAttempts
	muted := false
	defer func() {
		if r := recover(); r != nil {
			if preAttempt != nil {
				if muted {
					_ = rollbackCaptchaAttempt(bot, preAttempt)
				} else {
					_, _ = captcha.DeleteCaptchaAttemptByIDAtomic(preAttempt.ID, userID, preAttempt.ChatID)
				}
			}
			err = fmt.Errorf("send captcha: %v", r)
		}
	}()
	chat := ctx.EffectiveChat
	settings, err := captcha.GetCaptchaSettings(chat.Id)
	if err != nil {
		log.Errorf("[Captcha][SendCaptcha] Failed to get settings for chat %d: %v", chat.Id, err)
		return err
	}

	if !settings.Enabled {
		return errCaptchaDisabled
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
	if err := user.EnsureUserInDb(userID, validatedUsername, validatedUserName); err != nil {
		log.Errorf("Failed to ensure user in database: %v", err)
		return err
	}
	if err := chats.EnsureChatInDb(chat.Id, chat.Title); err != nil {
		log.Errorf("Failed to ensure chat in database: %v", err)
		return err
	}

	preAttempt, err = captcha.CreateCaptchaAttemptPreMessageIfEnabled(userID, chat.Id, answer, settings.Timeout)
	if err != nil || preAttempt == nil {
		if errors.Is(err, captcha.ErrCaptchaDisabled) {
			return errCaptchaDisabled
		}
		if err == nil {
			err = errors.New("captcha attempt was not created")
		}
		log.Errorf("Failed to pre-create captcha attempt: %v", err)
		return err
	}

	if _, err = chat.RestrictMember(bot, userID, MutedPermissions, nil); err != nil {
		_, _ = captcha.DeleteCaptchaAttemptByIDAtomic(preAttempt.ID, userID, chat.Id)
		return fmt.Errorf("mute user for captcha: %w", err)
	}
	muted = true

	// Create inline keyboard with options including attempt ID.
	// Add refresh button for image-based captcha (text or math) with attempt ID.
	includeRefresh := isImage && imageBytes != nil
	refreshBtnText := ""
	if includeRefresh {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		refreshBtnText, _ = tr.GetString("captcha_refresh_button")
	}
	keyboard := buildCaptchaKeyboard(preAttempt.ID, userID, preAttempt.RefreshCount, options, includeRefresh, refreshBtnText)

	// Prepare message text/caption
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var msgText string
	if isImage {
		if settings.CaptchaMode == "math" {
			text, _ := tr.GetString("captcha_welcome_math_image", i18n.TranslationParams{
				"first":  formatting.MentionHtml(userID, userName),
				"number": settings.Timeout,
			})
			msgText = text
		} else {
			text, _ := tr.GetString("captcha_welcome_text_image", i18n.TranslationParams{
				"first":  formatting.MentionHtml(userID, userName),
				"number": settings.Timeout,
			})
			msgText = text
		}
	} else {
		// Text-based fallback for math
		text, _ := tr.GetString("captcha_welcome_math_text", i18n.TranslationParams{
			"first":    formatting.MentionHtml(userID, userName),
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
			ParseMode:   formatting.HTML,
			ReplyMarkup: keyboard,
		})
	} else {
		// Send text message for math captcha
		sent, err = helpers.SendMessageWithErrorHandling(bot, chat.Id, msgText, &gotgbot.SendMessageOpts{
			ParseMode:   formatting.HTML,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil || sent == nil || sent.MessageId <= 0 {
		if err == nil {
			err = errors.New("telegram returned no captcha message")
		}
		log.Errorf("Failed to send captcha: %v", err)
		return errors.Join(err, rollbackCaptchaAttempt(bot, preAttempt))
	}

	// Update the attempt with the sent message ID
	err = captcha.UpdateCaptchaAttemptMessageID(preAttempt.ID, sent.MessageId)
	if err != nil {
		log.Errorf("Failed to set captcha attempt message ID: %v", err)
		// Delete the message if we can't track it
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, sent.MessageId)
		return errors.Join(err, rollbackCaptchaAttempt(bot, preAttempt))
	}

	preAttempt.MessageID = sent.MessageId
	if preAttempt.PreviousMessageID > 0 && preAttempt.PreviousMessageID != sent.MessageId {
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, preAttempt.PreviousMessageID)
	}
	scheduleCaptchaTimeout(bot, preAttempt)

	return nil
}

func rollbackCaptchaAttempt(bot *gotgbot.Bot, attempt *db.CaptchaAttempts) error {
	claimed, err := captcha.ReleaseCaptchaAttemptAtomic(attempt.ID, attempt.UserID, attempt.ChatID)
	if err != nil || !claimed {
		return err
	}
	return restoreCaptchaPermissions(bot, attempt.ChatID, attempt.UserID)
}

func scheduleCaptchaTimeout(bot *gotgbot.Bot, attempt *db.CaptchaAttempts) {
	delay := time.Until(attempt.ExpiresAt)
	if delay < 0 {
		delay = 0
	}
	startCaptchaLifecycleTask(func(ctx context.Context) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			defer error_handling.RecoverFromPanic("captchaTimeout", "captcha")
			if _, err := expireCaptchaAttempt(bot, attempt); err != nil {
				log.Errorf("[Captcha] Failed to expire attempt %d: %v", attempt.ID, err)
			}
		case <-ctx.Done():
		}
	})
}

func expireCaptchaAttempt(bot *gotgbot.Bot, attempt *db.CaptchaAttempts) (bool, error) {
	if chat_status.IsApproved(bot, attempt.ChatID, attempt.UserID) {
		if attempt.MessageID > 0 {
			_ = helpers.DeleteMessageWithErrorHandling(bot, attempt.ChatID, attempt.MessageID)
		}
		return releaseIncompleteCaptchaAttempt(bot, attempt)
	}
	settings, err := captcha.GetCaptchaSettings(attempt.ChatID)
	if err != nil {
		return false, fmt.Errorf("load captcha settings for chat %d: %w", attempt.ChatID, err)
	}
	if !settings.Enabled {
		if attempt.MessageID > 0 {
			_ = helpers.DeleteMessageWithErrorHandling(bot, attempt.ChatID, attempt.MessageID)
		}
		return releaseIncompleteCaptchaAttempt(bot, attempt)
	}
	claimed, _ := handleCaptchaTimeout(
		bot,
		attempt.ChatID,
		attempt.UserID,
		attempt.ID,
		attempt.MessageID,
		settings.FailureAction,
	)
	return claimed, nil
}

// handleCaptchaTimeout handles when a user fails to complete captcha in time.
func handleCaptchaTimeout(bot *gotgbot.Bot, chatID, userID int64, attemptID uint, fallbackMessageID int64, action string) (bool, bool) {
	// Fetch and validate the specific attempt targeted by this timeout event.
	attempt, err := captcha.GetCaptchaAttemptByID(attemptID)
	if err != nil || attempt == nil {
		log.Debugf("[Captcha] Timeout handler skipped - attempt not found for attempt_id=%d", attemptID)
		return false, false
	}
	if attempt.UserID != userID || attempt.ChatID != chatID {
		log.WithFields(log.Fields{
			"attempt_id":   attemptID,
			"attempt_user": attempt.UserID,
			"attempt_chat": attempt.ChatID,
			"user_id":      userID,
			"chat_id":      chatID,
		}).Warn("[Captcha] Timeout handler skipped - attempt identity mismatch")
		return false, false
	}

	storedMsgCount, _ := captcha.CountStoredMessagesForAttempt(attemptID)

	// Claim the attempt and persist an immediate unmute fallback in one
	// transaction. If the process dies before applying the failure action, the
	// unmute worker will not leave the user captcha-muted forever.
	deleted, err := captcha.ReleaseCaptchaAttemptAtomic(attemptID, userID, chatID)
	if err != nil || !deleted {
		log.Debugf("[Captcha] Timeout handler skipped - attempt already handled for attempt_id=%d", attemptID)
		return false, false
	}

	// Delete the captcha message
	messageID := attempt.MessageID
	if messageID == 0 {
		messageID = fallbackMessageID
	}
	if messageID > 0 {
		_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, messageID)
	}

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

	if err := executeCaptchaFailureAction(bot, chatID, userID, action); err != nil {
		log.Errorf("[Captcha] Failed to apply %s to user %d in chat %d: %v", action, userID, chatID, err)
		if action == "kick" {
			_, _ = bot.UnbanChatMember(chatID, userID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: false})
		}
		if restoreErr := restoreCaptchaPermissions(bot, chatID, userID); restoreErr != nil {
			log.Errorf("[Captcha] Failed to restore permissions after action failure: %v", restoreErr)
		}
		return true, false
	}
	if action != "mute" {
		if err := captcha.DeleteMutedUser(userID, chatID); err != nil {
			log.Errorf("[Captcha] Failed to clear fallback unmute for user %d: %v", userID, err)
		}
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: &gotgbot.Chat{Id: chatID}}))
	failureMsg := buildCaptchaFailureMessage(tr, action, userID, userName, storedMsgCount)
	sent, err := helpers.SendMessageWithErrorHandling(bot, chatID, failureMsg, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
	if err != nil {
		log.Errorf("Failed to send captcha failure message: %v", err)
	}

	// Delete the failure message after 30 seconds
	if sent != nil {
		time.AfterFunc(30*time.Second, func() {
			defer error_handling.RecoverFromPanic("captchaFailureMsgDelete", "captcha")
			_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, sent.MessageId)
		})
	}

	return true, true
}

// buildCaptchaFailureMessage builds the user-facing captcha failure message,
// including stored pending-message counts when present. Extracted from
// handleCaptchaTimeout to keep its cyclomatic complexity under the gocyclo limit.
func buildCaptchaFailureMessage(tr *i18n.Translator, action string, userID int64, userName string, storedMsgCount int64) string {
	if storedMsgCount > 0 {
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
		return fmt.Sprintf(template, formatting.MentionHtml(userID, userName), actionKey, storedMsgCount)
	}

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
	return fmt.Sprintf(template, formatting.MentionHtml(userID, userName))
}

// executeCaptchaFailureAction applies the configured captcha failure action
// (kick/ban/mute) to a user. Extracted from handleCaptchaTimeout.
func executeCaptchaFailureAction(bot *gotgbot.Bot, chatID, userID int64, action string) error {
	switch action {
	case "kick":
		if err := kickMember(bot, chatID, userID); err != nil {
			return fmt.Errorf("kick: %w", err)
		}
	case "ban":
		if _, err := bot.BanChatMember(chatID, userID, nil); err != nil {
			return fmt.Errorf("ban: %w", err)
		}
	case "mute":
		unmuteAt := time.Now().Add(24 * time.Hour)
		if err := captcha.CreateMutedUser(userID, chatID, unmuteAt); err != nil {
			return fmt.Errorf("store automatic unmute: %w", err)
		}
		if _, muteErr := bot.RestrictChatMember(chatID, userID, MutedPermissions, nil); muteErr != nil {
			return errors.Join(fmt.Errorf("mute: %w", muteErr), captcha.DeleteMutedUser(userID, chatID))
		}
		log.Infof("User %d muted in chat %d, will be unmuted at %s", userID, chatID, unmuteAt.Format(time.RFC3339))
	}
	return nil
}

func unmuteCaptchaUser(bot *gotgbot.Bot, chatID, userID int64) error {
	chat, err := bot.GetChat(chatID, nil)
	if err != nil {
		return err
	}
	_, err = bot.RestrictChatMember(chatID, userID, resolveUnmutePermissions(chat), nil)
	return err
}

func restoreCaptchaPermissions(bot *gotgbot.Bot, chatID, userID int64) error {
	err := unmuteCaptchaUser(bot, chatID, userID)
	if err != nil {
		scheduleErr := captcha.CreateMutedUser(userID, chatID, time.Now())
		return errors.Join(err, scheduleErr)
	}
	return captcha.DeleteMutedUser(userID, chatID)
}

// captchaVerifyCallback handles captcha answer button clicks.
// Verifies if the selected answer is correct and takes appropriate action.
func (moduleStruct) captchaVerifyCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := query.From

	attemptIDRaw := ""
	targetUserIDRaw := ""
	selectedAnswer := ""
	refreshCountRaw := "0"
	if decoded, ok := decodeCallbackData(query.Data, "captcha_verify"); ok {
		attemptIDRaw, _ = decoded.Field("a")
		targetUserIDRaw, _ = decoded.Field("u")
		selectedAnswer, _ = decoded.Field("s")
		if value, exists := decoded.Field("r"); exists {
			refreshCountRaw = value
		}
	}
	if attemptIDRaw == "" || targetUserIDRaw == "" || selectedAnswer == "" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_data")
		_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	attemptID64, err := strconv.ParseUint(attemptIDRaw, 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_attempt")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	targetUserID, err := strconv.ParseInt(targetUserIDRaw, 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	expectedRefreshCount, err := strconv.Atoi(refreshCountRaw)
	if err != nil || expectedRefreshCount < 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_data")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Check if this is the correct user
	if user.Id != targetUserID {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_not_for_you")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Get the captcha attempt and ensure IDs match
	attempt, err := captcha.GetCaptchaAttempt(targetUserID, chat.Id)
	if err != nil || attempt == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_expired_or_not_found")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	if attempt.ID != uint(attemptID64) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_attempt_not_valid")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	if attempt.RefreshCount != expectedRefreshCount {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_attempt_not_valid")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	settings, err := captcha.GetCaptchaSettings(chat.Id)
	if err != nil {
		log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_error_processing")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Check if answer is correct
	if selectedAnswer == attempt.Answer {
		// Read the summary before the atomic delete; PostgreSQL cascades stored
		// messages as soon as the attempt is claimed.
		storedMessages, storedErr := captcha.GetStoredMessagesForAttempt(attempt.ID)
		if storedErr != nil {
			log.Warnf("[Captcha] Failed to read stored messages for attempt %d: %v", attempt.ID, storedErr)
		}

		// Claim the attempt first to prevent timeout workers from acting after success.
		claimed, claimErr := captcha.CompleteCaptchaAttemptAtomic(
			attempt.ID,
			targetUserID,
			chat.Id,
			selectedAnswer,
			attempt.MessageID,
			expectedRefreshCount,
		)
		if claimErr != nil || !claimed {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_expired_or_not_found")
			_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}

		if err = restoreCaptchaPermissions(bot, chat.Id, targetUserID); err != nil {
			// The attempt is already claimed (single-winner), so the timeout
			// worker will not act. The user answered correctly, so do NOT apply
			// the failure action. restoreCaptchaPermissions persists a retry.
			log.Errorf("Failed to unmute user %d on verify: %v", targetUserID, err)
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, attempt.MessageID)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_failed_verify")
			_, _ = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}

		if storedErr == nil && len(storedMessages) > 0 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
					"user":  formatting.MentionHtml(targetUserID, user.FirstName),
					"count": len(storedMessages),
					"types": strings.Join(messageTypes, ", "),
				})

			// Send summary message that auto-deletes after 30 seconds
			summaryMsg, _ := helpers.SendMessageWithErrorHandling(bot, chat.Id, summaryText, &gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
			})

			// Auto-delete the summary after 30 seconds
			if summaryMsg != nil {
				time.AfterFunc(30*time.Second, func() {
					defer error_handling.RecoverFromPanic("captchaSummaryMsgDelete", "captcha")
					_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, summaryMsg.MessageId)
				})
			}
		}

		// Delete the captcha message
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, attempt.MessageID)

		// Send success message
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		msgTemplate, _ := tr.GetString("greetings_captcha_verified_success")
		successMsg := fmt.Sprintf(msgTemplate, formatting.MentionHtml(targetUserID, user.FirstName))
		sent, _ := helpers.SendMessageWithErrorHandling(bot, chat.Id, successMsg, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})

		// Delete success message after 5 seconds
		if sent != nil {
			time.AfterFunc(5*time.Second, func() {
				defer error_handling.RecoverFromPanic("captchaSuccessMsgDelete", "captcha")
				_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, sent.MessageId)
			})
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
		attempt, err = captcha.IncrementCaptchaAttempts(
			attempt.ID,
			targetUserID,
			chat.Id,
			attempt.Answer,
			attempt.MessageID,
			attempt.RefreshCount,
		)
		if err != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			key := "captcha_error_processing"
			if errors.Is(err, captcha.ErrNoActiveCaptcha) {
				key = "captcha_attempt_not_valid"
			}
			text, _ := tr.GetString(key)
			_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}

		if attempt.Attempts >= settings.MaxAttempts {
			// Max attempts reached - execute failure action. handleCaptchaTimeout
			// claims the attempt atomically; only this call wins it (another wrong
			// answer or the timeout goroutine may have already claimed it). Send the
			// final alert only when this call won, to avoid duplicate/contradictory
			// "you were kicked/banned" popups.
			claimed, applied := handleCaptchaTimeout(bot, chat.Id, targetUserID, attempt.ID, attempt.MessageID, settings.FailureAction)
			if applied {
				tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
			if claimed {
				tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
				text, _ := tr.GetString("captcha_error_processing")
				_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				return err
			}
			// Another actor already claimed this attempt; answer quietly.
			return ext.EndGroups
		}

		remainingAttempts := settings.MaxAttempts - attempt.Attempts
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := query.From

	attemptIDRaw := ""
	targetUserIDRaw := ""
	refreshCountRaw := "0"
	if decoded, ok := decodeCallbackData(query.Data, "captcha_refresh"); ok {
		attemptIDRaw, _ = decoded.Field("a")
		targetUserIDRaw, _ = decoded.Field("u")
		if value, exists := decoded.Field("r"); exists {
			refreshCountRaw = value
		}
	}
	if attemptIDRaw == "" || targetUserIDRaw == "" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_refresh")
		_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	attemptID64, err := strconv.ParseUint(attemptIDRaw, 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_attempt")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	targetUserID, err := strconv.ParseInt(targetUserIDRaw, 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_user")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	expectedRefreshCount, err := strconv.Atoi(refreshCountRaw)
	if err != nil || expectedRefreshCount < 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_refresh")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Check if this is the correct user
	if user.Id != targetUserID {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_not_for_you")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Cooldown: block rapid refreshes per user+chat
	cooldownKey := fmt.Sprintf("alita:captcha:refresh:cooldown:%d:%d", chat.Id, targetUserID)
	if m := cache.GetMarshal(); m != nil {
		if exists, _ := m.Get(cache.Context, cooldownKey, new(bool)); exists != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_wait_refresh")
			_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}
	}

	// Get the existing attempt and verify attempt ID
	attempt, err := captcha.GetCaptchaAttempt(targetUserID, chat.Id)
	if err != nil || attempt == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_expired_or_not_found")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	if attempt.ID != uint(attemptID64) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_attempt_not_valid")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	if attempt.RefreshCount != expectedRefreshCount {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_invalid_refresh")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Enforce per-attempt refresh cap
	if attempt.RefreshCount >= captchaMaxRefreshes {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_refresh_limit_reached")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Store old message ID for cleanup later (send-first pattern)
	oldMessageID := attempt.MessageID

	// Determine current mode and whether image flow applies
	settings, err := captcha.GetCaptchaSettings(chat.Id)
	if err != nil {
		log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
		// Fall through with nil settings — the nil guard below handles it safely
	}

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
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_failed_generate")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Build keyboard with new options and refresh button
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	refreshBtnText, _ := tr.GetString("captcha_refresh_button")
	keyboard := buildCaptchaKeyboard(attempt.ID, targetUserID, attempt.RefreshCount+1, options, true, refreshBtnText)

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
			formatting.MentionHtml(targetUserID, user.FirstName), remainingMinutes,
		)
	} else {
		template, _ := tr.GetString("captcha_welcome_math_detailed")
		caption = fmt.Sprintf(
			template,
			formatting.MentionHtml(targetUserID, user.FirstName), remainingMinutes,
		)
	}

	// Step 1: Send new message FIRST (before any deletion) - atomic refresh pattern
	sent, sendErr := bot.SendPhoto(chat.Id, gotgbot.InputFileByReader("captcha.png", bytes.NewReader(imageBytes)), &gotgbot.SendPhotoOpts{
		Caption:     caption,
		ParseMode:   formatting.HTML,
		ReplyMarkup: keyboard,
	})
	if sendErr != nil || sent == nil || sent.MessageId <= 0 {
		if sendErr == nil {
			sendErr = errors.New("telegram returned no refreshed captcha message")
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_failed_send")
		if _, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text}); err != nil {
			return errors.Join(sendErr, err)
		}
		return sendErr
	}

	// Step 2: Update DB with new answer and message ID
	updated, err := captcha.UpdateCaptchaAttemptOnRefreshByID(
		attempt.ID,
		attempt.Answer,
		attempt.MessageID,
		attempt.RefreshCount,
		newAnswer,
		sent.MessageId,
	)
	if err != nil || updated == nil {
		if err != nil {
			log.Errorf("Failed to update captcha attempt on refresh: %v", err)
		}
		// Rollback: delete the new message since DB update failed
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, sent.MessageId)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		key := "captcha_internal_update_error"
		if err == nil {
			key = "captcha_invalid_refresh"
		}
		text, _ := tr.GetString(key)
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	// Step 3: Delete old message LAST (best effort).
	if oldMessageID > 0 {
		if err := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, oldMessageID); err != nil {
			log.Warnf("[CaptchaRefresh] Failed to delete old message %d in chat %d: %v", oldMessageID, chat.Id, err)
		}
	}

	// Set cooldown
	if m := cache.GetMarshal(); m != nil {
		if err := m.Set(cache.Context, cooldownKey, true, store.WithExpiration(time.Duration(captchaRefreshCooldownS)*time.Second)); err != nil {
			log.Errorf("[CaptchaRefresh] Failed to set refresh cooldown for chat %d user %d: %v", chat.Id, targetUserID, err)
		}
	}

	tr = i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("captcha_refresh_success")
	_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
	return err
}

// handlePendingCaptchaMessage intercepts messages from users with pending captcha verification.
// Stores their messages and deletes them to prevent spam while they complete verification.
func (moduleStruct) handlePendingCaptchaMessage(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return ext.ContinueGroups
	}

	// Skip if this is not a group chat
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.ContinueGroups
	}

	// Skip if user is an admin
	if chat_status.IsUserAdmin(bot, chat.Id, user.Id) {
		return ext.ContinueGroups
	}
	if chat_status.IsApproved(bot, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	// Check if user has a pending captcha attempt
	attempt, err := captcha.GetCaptchaAttempt(user.Id, chat.Id)
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
		messageType = db.VIDEO_NOTE
		fileID = msg.VideoNote.FileId
	default:
		// Unknown message type, skip storing but still delete
		messageType = db.TEXT
		content = "[Unsupported message type]"
	}

	// Store the message
	err = captcha.StoreMessageForCaptcha(user.Id, chat.Id, attempt.ID, messageType, content, fileID, caption)
	if err != nil {
		log.Errorf("Failed to store message for user %d with pending captcha: %v", user.Id, err)
	}

	// Delete the message to prevent spam
	_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId)

	// End processing - don't let this message continue through other handlers
	return ext.EndGroups
}

func cleanupExpiredCaptchaAttempts(ctx context.Context, bot *gotgbot.Bot) error {
	defer error_handling.RecoverFromPanic("CaptchaCleanupExpiredAttempts", "captcha")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get expired attempts with message IDs before cleanup
	attempts, err := captcha.GetExpiredCaptchaAttempts()
	if err != nil {
		return err
	}

	if len(attempts) == 0 {
		return nil
	}
	if bot == nil {
		return errors.New("captcha cleanup has no bot")
	}

	log.Infof("[CaptchaCleanup] Processing %d expired captcha attempts", len(attempts))

	var cleanupErr error
	for _, attempt := range attempts {
		select {
		case <-ctx.Done():
			log.Warn("[CaptchaCleanup] Cleanup cancelled due to timeout")
			return ctx.Err()
		default:
		}

		if attempt.MessageID <= 0 {
			_, err = releaseIncompleteCaptchaAttempt(bot, attempt)
		} else {
			_, err = expireCaptchaAttempt(bot, attempt)
		}
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func runCaptchaCleanupTick(baseCtx context.Context, bot *gotgbot.Bot) {
	defer error_handling.RecoverFromPanic("CaptchaCleanup", "captcha")

	ctx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
	defer cancel()

	if err := cleanupExpiredCaptchaAttempts(ctx, bot); err != nil {
		log.Errorf("[CaptchaCleanup] Cleanup error: %v", err)
	}
}

func unmuteExpiredCaptchaUsers(bot *gotgbot.Bot) {
	if bot == nil {
		return
	}

	users, err := captcha.GetUsersToUnmute()
	if err != nil {
		log.Errorf("[CaptchaUnmute] Failed to get users to unmute: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	log.Infof("[CaptchaUnmute] Processing %d users to unmute", len(users))

	for _, user := range users {
		err := unmuteCaptchaUser(bot, user.ChatID, user.UserID)
		if err != nil {
			if isPermanentUnmuteError(err) {
				// Permanent error - user left, chat deleted, etc. - remove from DB
				log.Infof("[CaptchaUnmute] User %d no longer in chat %d, removing from muted list: %v",
					user.UserID, user.ChatID, err)
				if _, deleteErr := captcha.DeleteMutedUserIfUnchanged(user.ID, user.UnmuteAt); deleteErr != nil {
					log.Errorf("[CaptchaUnmute] Failed to delete permanent schedule %d: %v", user.ID, deleteErr)
				}
			} else {
				// Transient error - will retry on next tick
				log.Warnf("[CaptchaUnmute] Failed to unmute user %d in chat %d (will retry): %v",
					user.UserID, user.ChatID, err)
			}
			continue
		}

		deleted, deleteErr := captcha.DeleteMutedUserIfUnchanged(user.ID, user.UnmuteAt)
		if deleteErr != nil {
			log.Errorf("[CaptchaUnmute] Failed to delete muted user record %d: %v", user.ID, deleteErr)
			continue
		}
		if deleted {
			log.Infof("[CaptchaUnmute] Unmuted user %d in chat %d", user.UserID, user.ChatID)
			continue
		}

		// A concurrent captcha mute replaced the schedule while Telegram was
		// unmuting. Re-apply the mute so the newer schedule remains effective.
		current, currentErr := captcha.GetMutedUser(user.UserID, user.ChatID)
		if currentErr != nil {
			log.Errorf("[CaptchaUnmute] Failed to reload schedule for user %d: %v", user.UserID, currentErr)
		} else if current != nil && current.UnmuteAt.After(user.UnmuteAt) {
			if _, remuteErr := bot.RestrictChatMember(user.ChatID, user.UserID, MutedPermissions, nil); remuteErr != nil {
				log.Errorf("[CaptchaUnmute] Failed to restore newer mute for user %d: %v", user.UserID, remuteErr)
			}
		}
	}
}

func startCaptchaWorkers(bot *gotgbot.Bot) {
	startCaptchaLifecycleTask(func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runCaptchaCleanupTick(ctx, bot)
			case <-ctx.Done():
				return
			}
		}
	})

	startCaptchaLifecycleTask(func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				func() {
					defer error_handling.RecoverFromPanic("CaptchaUnmute", "captcha")
					unmuteExpiredCaptchaUsers(bot)
				}()
			case <-ctx.Done():
				return
			}
		}
	})
}

// LoadCaptcha registers all captcha module handlers with the dispatcher.
func LoadCaptcha(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[captchaModule.moduleName] = true

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
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("captcha_verify"), captchaModule.captchaVerifyCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("captcha_refresh"), captchaModule.captchaRefreshCallback))
}

func init() {
	RegisterLegacyModule("Captcha", 220, LoadCaptcha)
}
