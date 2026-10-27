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
	"github.com/eko/gocache/lib/v4/store"
	"github.com/mojocn/base64Captcha"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/jobs"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

var captchaMemberRetryDelay = 300 * time.Millisecond

func isTransientMemberError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "user not found") || strings.Contains(s, "user_id_invalid") || strings.Contains(s, "participant_id_invalid")
}

var captchaModule = moduleStruct{moduleName: "Captcha"}

type fixedStringCaptchaDriver struct {
	*base64Captcha.DriverString
	content string
}

func (d *fixedStringCaptchaDriver) GenerateIdQuestionAnswer() (id, q, a string) {
	id = base64Captcha.RandomId()
	return id, d.content, d.content
}

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
			80,
			240,
			0,
			0,
			len(question),
			question,
			nil,
			nil,
			[]string{},
		),
		content: question,
	}
}

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

const (
	captchaMaxRefreshes     = 3
	captchaRefreshCooldownS = 5
)

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
	// errCaptchaCannotRestrict means the bot may not mute the newcomer, so the
	// join gets the welcome instead of a challenge it could not enforce.
	errCaptchaCannotRestrict = errors.New("captcha skipped: bot cannot restrict members")
)

// captchaRightsNoticeCooldown is how long a chat goes without a repeat of the
// notice that captcha needs the bot's "Ban users" right.
const captchaRightsNoticeCooldown = 6 * time.Hour

// captchaRightsNoticeLocal holds the notice cooldown expiry per chat when
// Redis is not configured.
var captchaRightsNoticeLocal sync.Map

func captchaRightsNoticeKey(chatID int64) string {
	return fmt.Sprintf("alita:captcha:rightsNotice:%d", chatID)
}

// claimCaptchaRightsNotice reports whether this call may send the missing-right
// notice in chatID, at most once per captchaRightsNoticeCooldown across
// replicas. A Redis error sends nothing: a missed notice comes back on a later
// join, a repeated one is noise.
func claimCaptchaRightsNotice(chatID int64) bool {
	if rdb := cache.GetRedisClient(); rdb != nil {
		claimed, err := rdb.SetNX(cache.Context, captchaRightsNoticeKey(chatID), 1, captchaRightsNoticeCooldown).Result()
		if err != nil {
			log.Debugf("[Captcha] Failed to claim the rights notice for chat %d: %v", chatID, err)
			return false
		}
		return claimed
	}
	for {
		now := time.Now()
		prev, loaded := captchaRightsNoticeLocal.LoadOrStore(chatID, now.Add(captchaRightsNoticeCooldown))
		if !loaded {
			return true
		}
		if expiry, ok := prev.(time.Time); ok && now.Before(expiry) {
			return false
		}
		if captchaRightsNoticeLocal.CompareAndSwap(chatID, prev, now.Add(captchaRightsNoticeCooldown)) {
			return true
		}
	}
}

// captchaBotCanRestrict reports whether the bot holds the right to mute
// members in chat, from the admin cache or one live lookup. It sends nothing.
// A failed lookup returns its error, so the caller can tell it from a missing
// right and still try the mute.
func captchaBotCanRestrict(bot *gotgbot.Bot, chat *gotgbot.Chat) (bool, error) {
	if found, member := cache.GetAdminCacheUser(chat.Id, bot.Id); found {
		return member.CanRestrictMembers, nil
	}
	member, err := chat.GetMember(bot, bot.Id, nil)
	if err != nil {
		return false, err
	}
	return member.MergeChatMember().CanRestrictMembers, nil
}

// isMissingRestrictRightError reports whether a restrictChatMember error means
// the bot lacks the right to restrict members.
func isMissingRestrictRightError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "not enough rights to restrict") ||
		strings.Contains(s, "CHAT_ADMIN_REQUIRED") ||
		strings.Contains(s, "need administrator rights")
}

// skipCaptchaWithoutRestrictRight tells the chat's admins, at most once per
// cooldown, that captcha cannot run until the bot gets the "Ban users" right,
// and returns errCaptchaCannotRestrict so the join falls through to the
// welcome.
func skipCaptchaWithoutRestrictRight(bot *gotgbot.Bot, ctx *ext.Context, cause error) error {
	chat := ctx.EffectiveChat
	log.Debugf("[Captcha] Bot cannot restrict members in chat %d, skipping captcha: %v", chat.Id, cause)
	if claimCaptchaRightsNotice(chat.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_bot_cannot_restrict")
		opts := &gotgbot.SendMessageOpts{ParseMode: formatting.HTML}
		if ctx.EffectiveMessage != nil {
			opts.MessageThreadId = ctx.EffectiveMessage.MessageThreadId
		}
		if _, err := helpers.SendMessageWithErrorHandling(bot, chat.Id, text, opts); err != nil {
			log.Warnf("[Captcha] Failed to send the missing-right notice in chat %d: %v", chat.Id, err)
		}
	}
	if cause == nil {
		return errCaptchaCannotRestrict
	}
	return fmt.Errorf("%w: %w", errCaptchaCannotRestrict, cause)
}

// StartCaptchaLifecycle restores persisted attempts and starts the cleanup workers.
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
// If max <= 0, it returns 0. On persistent entropy failure it returns an error
// instead of falling back to a predictable PRNG.
func secureIntn(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	const maxRetries = 10
	for i := 0; i < maxRetries; i++ {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
		if err == nil {
			return int(n.Int64()), nil
		}
	}
	return 0, fmt.Errorf("secureIntn: exhausted retries for crypto/rand.Int (max=%d)", max)
}

func secureShuffleStrings(values []string) error {
	for i := len(values) - 1; i > 0; i-- {
		j, err := secureIntn(i + 1)
		if err != nil {
			return err
		}
		values[i], values[j] = values[j], values[i]
	}
	return nil
}

// viewPendingMessages lets admins list a user's stored messages with /captchapending.
func (moduleStruct) viewPendingMessages(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.RequireUser(bot, ctx) == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chat := resolveAdminChat(bot, ctx, false)
	if chat == nil {
		return ext.EndGroups
	}

	if len(ctx.Args()) < 2 {
		text, _ := tr.GetString("captcha_pending_usage")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	targetUserID := extraction.ExtractUser(bot, ctx)
	if targetUserID == 0 {
		text, _ := tr.GetString("captcha_invalid_user")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	messages, err := captcha.GetStoredMessagesForUser(targetUserID, chat.Id)
	if err != nil || len(messages) == 0 {
		text, _ := tr.GetString("captcha_no_pending_messages")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

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

// clearPendingMessages lets admins delete a user's stored messages with /captchaclear.
func (moduleStruct) clearPendingMessages(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.RequireUser(bot, ctx) == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chat := resolveAdminChat(bot, ctx, false)
	if chat == nil {
		return ext.EndGroups
	}

	args := ctx.Args()[1:]
	if len(args) < 1 {
		text, _ := tr.GetString("captcha_clear_usage")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	targetUserID := extraction.ExtractUser(bot, ctx)
	if targetUserID == 0 {
		text, _ := tr.GetString("captcha_invalid_user")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	err := captcha.DeleteStoredMessagesForUser(targetUserID, chat.Id)
	if err != nil {
		log.Errorf("Failed to delete stored messages for user %d in chat %d: %v", targetUserID, chat.Id, err)
		text, _ := tr.GetString("captcha_clear_failed")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	text, _ := tr.GetString("captcha_clear_success",
		i18n.TranslationParams{"user_id": targetUserID})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

func captchaAdminGate(bot *gotgbot.Bot, ctx *ext.Context, requireBotAdmin bool) (*gotgbot.User, *gotgbot.Chat, bool) {
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return nil, nil, false
	}
	chat := resolveAdminChat(bot, ctx, requireBotAdmin)
	if chat == nil {
		return nil, nil, false
	}
	return user, chat, true
}

// captchaJoinNamespace is the callback namespace of the "Turn on captcha"
// button on the message the bot sends when it joins a group. The button
// carries no fields: it acts on the chat it was pressed in.
const captchaJoinNamespace = "captcha_on"

// captchaJoinOfferMarkup returns the "Turn on captcha" keyboard for the join
// message in chatID, or false when captcha is already on there or the button
// cannot be encoded. It reads the settings without creating a row.
func captchaJoinOfferMarkup(ctx *ext.Context, chatID int64, buttonText string) (gotgbot.InlineKeyboardMarkup, bool) {
	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chatID)
	if err == nil && settings != nil && settings.Enabled {
		return gotgbot.InlineKeyboardMarkup{}, false
	}
	data, ok := mustCallbackData(captchaJoinNamespace, nil)
	if !ok {
		return gotgbot.InlineKeyboardMarkup{}, false
	}
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
		{Text: buttonText, CallbackData: data},
	}}}, true
}

// captchaJoinEnableCallback turns captcha on from the join message's button.
// It takes the same rights as /captcha on (the presser and the bot are admins
// of the chat) and the same write, answering every press in the chat's
// language.
func (moduleStruct) captchaJoinEnableCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if _, ok := decodeCallbackData(query.Data, captchaJoinNamespace); !ok || query.Message == nil {
		return answerInvalidCallback(bot, ctx, query)
	}
	chat := query.Message.GetChat()
	if chat.Type == "private" {
		return answerInvalidCallback(bot, ctx, query)
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	answer := func(key string) {
		text, _ := tr.GetString(key)
		if _, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text}); err != nil {
			log.Warnf("[Captcha] Failed to answer captcha button press in chat %d: %v", chat.Id, err)
		}
	}
	removeButton := func(confirmation string) {
		var err error
		if msg, ok := query.Message.(gotgbot.Message); ok && msg.Text != "" && confirmation != "" {
			_, _, err = msg.EditText(bot, &gotgbot.EditMessageTextOpts{Text: msg.Text + "\n\n" + confirmation})
		} else {
			_, _, err = query.Message.EditReplyMarkup(bot, &gotgbot.EditMessageReplyMarkupOpts{})
		}
		if err != nil {
			log.Debugf("[Captcha] Failed to remove the captcha button in chat %d: %v", chat.Id, err)
		}
	}

	if !chat_status.IsUserAdmin(bot, chat.Id, query.From.Id) {
		answer("connections_is_user_connected_user_not_admin")
		return ext.EndGroups
	}
	if !chat_status.IsUserAdmin(bot, chat.Id, bot.Id) {
		answer("connections_is_user_connected_bot_not_admin")
		return ext.EndGroups
	}

	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		answer("captcha_enable_failed")
		return err
	}
	if settings.Enabled {
		answer("captcha_join_already_enabled")
		removeButton("")
		return ext.EndGroups
	}
	if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
		answer("captcha_enable_failed")
		return err
	}
	answer("captcha_join_enabled")
	confirmation, _ := tr.GetString("captcha_join_enabled")
	removeButton(confirmation)
	return ext.EndGroups
}

func (moduleStruct) captchaCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	_, chat, ok := captchaAdminGate(bot, ctx, true)
	if !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
		if err != nil {
			log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
			return ext.EndGroups
		}
		status, _ := tr.GetString("captcha_status_disabled")
		if settings.Enabled {
			status, _ = tr.GetString("captcha_status_enabled")
		}

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
			text, _ := tr.GetString("captcha_enable_failed")
			if _, replyErr := msg.Reply(bot, text, nil); replyErr != nil {
				log.Warnf("[Captcha] Failed to send enable error message in chat %d: %v", chat.Id, replyErr)
			}
			return err
		}
		text, _ := tr.GetString("captcha_enabled_success")
		_, err = msg.Reply(bot, text, formatting.Shtml())
		return err

	case "off", "disable", "no":
		err := captcha.SetCaptchaEnabled(chat.Id, false)
		if err != nil {
			text, _ := tr.GetString("captcha_disable_failed")
			if _, replyErr := msg.Reply(bot, text, nil); replyErr != nil {
				log.Warnf("[Captcha] Failed to send disable error message in chat %d: %v", chat.Id, replyErr)
			}
			return err
		}
		if err = DisableCaptchaForChat(bot, chat.Id); err != nil {
			log.Errorf("[Captcha] Failed to clean up chat %d after disabling captcha: %v", chat.Id, err)
			text, _ := tr.GetString("captcha_disable_failed")
			_, _ = msg.Reply(bot, text, nil)
			return err
		}
		text, _ := tr.GetString("captcha_disabled_success")
		_, err = msg.Reply(bot, text, formatting.Shtml())
		return err

	default:
		text, _ := tr.GetString("captcha_usage")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}
}

// DisableCaptchaForChat clears the chat's pending captcha attempts and unmutes
// the users they held, after captcha has been turned off. The panel runs it too.
func DisableCaptchaForChat(bot *gotgbot.Bot, chatID int64) error {
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

func (moduleStruct) captchaModeCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	_, chat, ok := captchaAdminGate(bot, ctx, false)
	if !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		text, _ := tr.GetString("captcha_mode_specify")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	mode := strings.ToLower(args[0])
	if mode != "math" && mode != "text" {
		text, _ := tr.GetString("captcha_mode_invalid")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	err := captcha.SetCaptchaMode(chat.Id, mode)
	if err != nil {
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

	modeDesc, _ := tr.GetString("captcha_mode_math_desc")
	if mode == "text" {
		modeDesc, _ = tr.GetString("captcha_mode_text_desc")
	}

	textTemplate, _ := tr.GetString("captcha_mode_set_formatted")
	text := fmt.Sprintf(textTemplate, mode, modeDesc)
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

func (moduleStruct) captchaTimeCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	_, chat, ok := captchaAdminGate(bot, ctx, false)
	if !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		text, _ := tr.GetString("captcha_timeout_specify")
		_, err := msg.Reply(bot, text, nil)
		return err
	}

	timeout64, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || timeout64 < 1 || timeout64 > 10 {
		text, _ := tr.GetString("captcha_timeout_invalid")
		_, err = msg.Reply(bot, text, nil)
		return err
	}
	timeout := int(timeout64)

	err = captcha.SetCaptchaTimeout(chat.Id, timeout)
	if err != nil {
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

	text, _ := tr.GetString("captcha_timeout_set_success", i18n.TranslationParams{"d": timeout})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

func (moduleStruct) captchaActionCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	_, chat, ok := captchaAdminGate(bot, ctx, false)
	if !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		text, _ := tr.GetString("captcha_action_specify")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	action := strings.ToLower(args[0])
	if action != "kick" && action != "ban" && action != "mute" {
		text, _ := tr.GetString("captcha_action_invalid")
		_, err := msg.Reply(bot, text, formatting.Shtml())
		return err
	}

	err := captcha.SetCaptchaFailureAction(chat.Id, action)
	if err != nil {
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

	text, _ := tr.GetString("captcha_action_set_success", i18n.TranslationParams{"s": action})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

func (moduleStruct) captchaMaxAttemptsCommand(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	_, chat, ok := captchaAdminGate(bot, ctx, true)
	if !ok {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if len(args) == 0 {
		settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
		if err != nil {
			log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
			return ext.EndGroups
		}
		text, _ := tr.GetString("captcha_max_attempts_current", i18n.TranslationParams{"d": settings.MaxAttempts})
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

	text, _ := tr.GetString("captcha_max_attempts_set", i18n.TranslationParams{"d": attempts})
	_, err = msg.Reply(bot, text, formatting.Shtml())
	return err
}

func generateMathCaptcha() (string, string, []string, error) {
	opIdx, err := secureIntn(3)
	if err != nil {
		return "", "", nil, err
	}
	operations := []string{"+", "-", "*"}
	operation := operations[opIdx]

	var a, b, answer int
	var question string

	switch operation {
	case "+":
		a0, err := secureIntn(50)
		if err != nil {
			return "", "", nil, err
		}
		b0, err := secureIntn(50)
		if err != nil {
			return "", "", nil, err
		}
		a = a0 + 1
		b = b0 + 1
		answer = a + b
		question = formatMathQuestion(a, b, operation)
	case "-":
		a0, err := secureIntn(50)
		if err != nil {
			return "", "", nil, err
		}
		a = a0 + 20
		bRaw, err := secureIntn(a)
		if err != nil {
			return "", "", nil, err
		}
		b = bRaw + 1
		answer = a - b
		question = formatMathQuestion(a, b, operation)
	case "*":
		a0, err := secureIntn(12)
		if err != nil {
			return "", "", nil, err
		}
		b0, err := secureIntn(12)
		if err != nil {
			return "", "", nil, err
		}
		a = a0 + 1
		b = b0 + 1
		answer = a * b
		question = formatMathQuestion(a, b, operation)
	}

	options := []string{strconv.Itoa(answer)}
	for len(options) < 4 {
		off, err := secureIntn(20)
		if err != nil {
			return "", "", nil, err
		}
		wrongAnswer := answer + off - 10
		if wrongAnswer != answer && wrongAnswer > 0 {
			wrongStr := strconv.Itoa(wrongAnswer)
			if !slices.Contains(options, wrongStr) {
				options = append(options, wrongStr)
			}
		}
	}

	if err := secureShuffleStrings(options); err != nil {
		return "", "", nil, err
	}

	return question, strconv.Itoa(answer), options, nil
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

func generateTextCaptcha() (string, []byte, []string, error) {
	store := base64Captcha.DefaultMemStore

	driver := base64Captcha.NewDriverString(
		80,
		160,
		0,
		2,
		4,
		"234567890abcdefghjkmnpqrstuvwxyz",
		nil,
		nil,
		[]string{},
	)

	captcha := base64Captcha.NewCaptcha(driver, store)

	id, b64s, answer, err := captcha.Generate()
	if err != nil {
		return "", nil, nil, err
	}
	_ = id

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

	options := []string{answer}
	characters := "234567890abcdefghjkmnpqrstuvwxyz"
	for len(options) < 4 {
		decoy := ""
		for range len(answer) {
			idx, err := secureIntn(len(characters))
			if err != nil {
				return "", nil, nil, err
			}
			decoy += string(characters[idx])
		}
		if !slices.Contains(options, decoy) {
			options = append(options, decoy)
		}
	}

	if err := secureShuffleStrings(options); err != nil {
		return "", nil, nil, err
	}

	if !slices.Contains(options, answer) {
		log.Error("[Captcha] Generated text answer was missing from its options, regenerating")
		return generateTextCaptcha()
	}

	return answer, imageBytes, options, nil
}

func generateMathImageCaptcha() (string, []byte, []string, error) {
	question, answer, options, err := generateMathCaptcha()
	if err != nil {
		return "", nil, nil, err
	}

	// DriverString normally samples random characters from Source on Generate.
	// Wrap it so the rendered image always matches the computed math question.
	driver := newMathImageCaptchaDriver(question)

	captcha := base64Captcha.NewCaptcha(driver, noopCaptchaStore{})
	_, b64s, _, err := captcha.Generate()
	if err != nil {
		return "", nil, nil, err
	}

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

	if !slices.Contains(options, answer) {
		log.Error("[Captcha] Generated math answer was missing from its options, regenerating")
		return generateMathImageCaptcha()
	}

	return answer, imageBytes, options, nil
}

func buildCaptchaKeyboard(attemptID uint, userID int64, refreshCount int, options []string, includeRefresh bool, refreshBtnText string) gotgbot.InlineKeyboardMarkup {
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, option := range options {
		data, ok := mustCallbackData(
			"captcha_verify",
			map[string]string{
				"a": fmt.Sprint(attemptID),
				"r": fmt.Sprint(refreshCount),
				"u": fmt.Sprint(userID),
				"s": option,
			},
		)
		if !ok {
			log.WithFields(log.Fields{
				"attemptID": attemptID,
				"option":    option,
			}).Warn("[Captcha] Failed to encode verify button, omitting")
			continue
		}
		button := gotgbot.InlineKeyboardButton{
			Text:         option,
			CallbackData: data,
		}
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{button})
	}
	if includeRefresh {
		if data, ok := mustCallbackData(
			"captcha_refresh",
			map[string]string{
				"a": fmt.Sprint(attemptID),
				"r": fmt.Sprint(refreshCount),
				"u": fmt.Sprint(userID),
			},
		); ok {
			buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
				Text:         refreshBtnText,
				CallbackData: data,
			}})
		} else {
			log.WithField("attemptID", attemptID).Warn("[Captcha] Failed to encode refresh button, omitting")
		}
	}
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// SendCaptcha sends a captcha challenge to a new member.
// There is one pending attempt per (user, chat); creating a challenge replaces the old one.
//
//nolint:gocyclo // The ordered mute/send/persist rollback state machine is intentionally cohesive.
func SendCaptcha(bot *gotgbot.Bot, ctx *ext.Context, userID int64, userName string) (err error) {
	var preAttempt *db.CaptchaAttempts
	muted := false
	defer func() {
		if r := recover(); r != nil {
			if preAttempt != nil {
				var rbErr error
				if muted {
					rbErr = rollbackCaptchaAttempt(bot, preAttempt)
				} else {
					_, rbErr = captcha.DeleteCaptchaAttemptByIDAtomic(preAttempt.ID, userID, preAttempt.ChatID)
				}
				if rbErr != nil {
					log.Errorf("[Captcha] Failed to roll back attempt %d after panic: %v", preAttempt.ID, rbErr)
				}
			}
			err = fmt.Errorf("send captcha: %v", r)
		}
	}()
	chat := ctx.EffectiveChat
	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		log.Errorf("[Captcha][SendCaptcha] Failed to get settings for chat %d: %v", chat.Id, err)
		return err
	}

	if !settings.Enabled {
		return errCaptchaDisabled
	}

	// Check the mute right before any work: without it the challenge cannot be
	// enforced, and a failed join would also cost the newcomer the welcome.
	if canRestrict, rightErr := captchaBotCanRestrict(bot, chat); rightErr != nil {
		log.Debugf("[Captcha] Could not check the restrict right in chat %d, trying the mute: %v", chat.Id, rightErr)
	} else if !canRestrict {
		return skipCaptchaWithoutRestrictRight(bot, ctx, nil)
	}

	var question string
	var answer string
	var options []string
	var imageBytes []byte
	isImage := false

	if settings.CaptchaMode == "math" {
		var err error
		answer, imageBytes, options, err = generateMathImageCaptcha()
		if err != nil || imageBytes == nil {
			log.Errorf("Failed to generate math image captcha: %v", err)
			var fbErr error
			question, answer, options, fbErr = generateMathCaptcha()
			if fbErr != nil {
				log.Errorf("Failed to generate fallback math captcha: %v", fbErr)
				return fmt.Errorf("generate captcha: %w", fbErr)
			}
			isImage = false
		} else {
			isImage = true
		}
	} else {
		var err error
		answer, imageBytes, options, err = generateTextCaptcha()
		if err != nil {
			log.Errorf("Failed to generate text captcha: %v", err)
			var fbErr error
			question, answer, options, fbErr = generateMathCaptcha()
			if fbErr != nil {
				log.Errorf("Failed to generate fallback math captcha: %v", fbErr)
				return fmt.Errorf("generate captcha: %w", fbErr)
			}
			isImage = false
		} else {
			isImage = true
		}
	}

	userMember, err := bot.GetChatMember(chat.Id, userID, nil)
	if err != nil {
		if isTransientMemberError(err) {
			time.Sleep(captchaMemberRetryDelay)
			userMember, err = bot.GetChatMember(chat.Id, userID, nil)
		}
		if err != nil {
			log.Errorf("Failed to validate user %d via Telegram API: %v", userID, err)
			return fmt.Errorf("user %d does not exist or is not accessible: %w", userID, err)
		}
	}
	// The admin cache can miss a fresh admin; Telegram refuses to mute one, so
	// greet them as if captcha were off.
	if s := userMember.GetStatus(); s == "administrator" || s == "creator" {
		return errCaptchaDisabled
	}

	validatedUser := userMember.GetUser()
	validatedUserName := userName
	if validatedUser.FirstName != "" {
		validatedUserName = validatedUser.FirstName
	}
	validatedUsername := validatedUser.Username

	if chat.Id == 0 || chat.Title == "" {
		log.Errorf("Invalid chat data: ID=%d, Title=%s", chat.Id, chat.Title)
		return fmt.Errorf("invalid chat data")
	}

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
		if _, delErr := captcha.DeleteCaptchaAttemptByIDAtomic(preAttempt.ID, userID, chat.Id); delErr != nil {
			log.Errorf("[Captcha] Failed to delete attempt %d after mute failure: %v", preAttempt.ID, delErr)
		} else if isMissingRestrictRightError(err) {
			// The admin cache said the bot could restrict: drop it so the
			// next join checks the right afresh.
			cache.InvalidateAdminCache(chat.Id)
			return skipCaptchaWithoutRestrictRight(bot, ctx, err)
		}
		return fmt.Errorf("mute user for captcha: %w", err)
	}
	muted = true

	includeRefresh := isImage && imageBytes != nil
	refreshBtnText := ""
	if includeRefresh {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		refreshBtnText, _ = tr.GetString("captcha_refresh_button")
	}
	keyboard := buildCaptchaKeyboard(preAttempt.ID, userID, preAttempt.RefreshCount, options, includeRefresh, refreshBtnText)
	verifyCount := len(keyboard.InlineKeyboard)
	if includeRefresh && verifyCount > 0 {
		verifyCount--
	}
	if verifyCount == 0 {
		log.Errorf("[Captcha] No verify buttons could be encoded for attempt %d", preAttempt.ID)
		return errors.Join(errors.New("captcha keyboard empty: no encodable options"), rollbackCaptchaAttempt(bot, preAttempt))
	}

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
		text, _ := tr.GetString("captcha_welcome_math_text", i18n.TranslationParams{
			"first":    formatting.MentionHtml(userID, userName),
			"question": question,
			"number":   settings.Timeout,
		})
		msgText = text
	}

	var sent *gotgbot.Message

	if isImage && imageBytes != nil {
		sent, err = bot.SendPhoto(chat.Id, gotgbot.InputFileByReader("captcha.png", bytes.NewReader(imageBytes)), &gotgbot.SendPhotoOpts{
			Caption:     msgText,
			ParseMode:   formatting.HTML,
			ReplyMarkup: keyboard,
		})
	} else {
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

	err = captcha.UpdateCaptchaAttemptMessageID(preAttempt.ID, sent.MessageId)
	if err != nil {
		log.Errorf("Failed to set captcha attempt message ID: %v", err)
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
	capturedRefreshCount := attempt.RefreshCount
	capturedExpiresAt := attempt.ExpiresAt
	capturedID := attempt.ID
	startCaptchaLifecycleTask(func(ctx context.Context) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			defer error_handling.RecoverFromPanic("captchaTimeout", "captcha")
			if fresh, err := captcha.GetCaptchaAttemptByID(capturedID); err == nil && fresh != nil {
				if fresh.RefreshCount != capturedRefreshCount || !fresh.ExpiresAt.Equal(capturedExpiresAt) {
					log.Debugf("[Captcha] Stale timer for attempt %d ignored (refreshCount %d->%d, expires %v->%v)", capturedID, capturedRefreshCount, fresh.RefreshCount, capturedExpiresAt, fresh.ExpiresAt)
					return
				}
			}
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
		captchaFailedTimeout,
	)
	return claimed, nil
}

// Why a captcha failed, as the log channel entry names it.
const (
	captchaFailedTimeout      = "timeout"
	captchaFailedWrongAnswers = "wrong answers"
)

func handleCaptchaTimeout(bot *gotgbot.Bot, chatID, userID int64, attemptID uint, fallbackMessageID int64, action, cause string) (bool, bool) {
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

	deleted, err := captcha.ReleaseCaptchaAttemptAtomic(attemptID, userID, chatID)
	if err != nil || !deleted {
		log.Debugf("[Captcha] Timeout handler skipped - attempt already handled for attempt_id=%d", attemptID)
		return false, false
	}

	messageID := attempt.MessageID
	if messageID == 0 {
		messageID = fallbackMessageID
	}
	if messageID > 0 {
		_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, messageID)
	}

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

	// Another guard (spamdb, antiraid, fed-ban) banned the newcomer while the
	// captcha was pending. The kick is unbanChatMember, which would lift that
	// ban, so leave the member as it is and drop any pending unmute.
	if err == nil && member.GetStatus() == "kicked" {
		if err := captcha.DeleteMutedUser(userID, chatID); err != nil {
			log.Errorf("[Captcha] Failed to clear fallback unmute for banned user %d: %v", userID, err)
		}
		log.Infof("[Captcha] User %d is already banned in chat %d; skipping %s", userID, chatID, action)
		return true, false
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
	var banRecordID int64
	if action == "ban" {
		banRecordID = recordBan(banRecord{chatID: chatID, userID: userID, source: models.BanSourceCaptcha, detail: cause})
	}
	if action != "mute" {
		if err := captcha.DeleteMutedUser(userID, chatID); err != nil {
			log.Errorf("[Captcha] Failed to clear fallback unmute for user %d: %v", userID, err)
		}
	}
	reportCaptchaFailure(bot, chatID, userID, userName, action, cause, storedMsgCount, unbanLogMarkup(chatID, banRecordID))
	return true, true
}

// reportCaptchaFailure logs an applied failure action to the log channel and
// posts the group notice, which deletes itself after 30 seconds. The log
// entry carries unban, the Unban button of a ban (nil for none).
func reportCaptchaFailure(bot *gotgbot.Bot, chatID, userID int64, userName, action, cause string, storedMsgCount int64, unban *gotgbot.InlineKeyboardMarkup) {
	actionlog.AutomatedWithMarkup(bot, captchaLogChat(chatID), unban, "CAPTCHA_FAILED", userID, userName,
		actionlog.Field{Name: "Action", Value: action},
		actionlog.Field{Name: "Cause", Value: cause},
	)

	tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: &gotgbot.Chat{Id: chatID}}))
	failureMsg := buildCaptchaFailureMessage(tr, action, userID, userName, storedMsgCount)
	sent, err := helpers.SendMessageWithErrorHandling(bot, chatID, failureMsg, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
	if err != nil {
		log.Errorf("Failed to send captcha failure message: %v", err)
	}

	if sent != nil {
		time.AfterFunc(30*time.Second, func() {
			defer error_handling.RecoverFromPanic("captchaFailureMsgDelete", "captcha")
			_ = helpers.DeleteMessageWithErrorHandling(bot, chatID, sent.MessageId)
		})
	}
}

// captchaLogChat is the chat for a log entry when only its ID is at hand; the
// entry header takes its title from the chats table.
func captchaLogChat(chatID int64) *gotgbot.Chat {
	chat := &gotgbot.Chat{Id: chatID}
	if info, err := chats.GetChatBasicInfoCached(chatID); err == nil && info != nil {
		chat.Title = info.ChatName
	}
	return chat
}

func buildCaptchaFailureMessage(tr *i18n.Translator, action string, userID int64, userName string, storedMsgCount int64) string {
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
	msg := fmt.Sprintf(template, formatting.MentionHtml(userID, userName))

	if storedMsgCount > 0 {
		note, _ := tr.GetString("captcha_timeout_stored_note")
		msg += "\n\n" + fmt.Sprintf(note, storedMsgCount)
	}

	return msg
}

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

// captchaVerifyCallback completes a correct answer by deleting the attempt and its stored rows.
// It never replays stored messages: their only replacement is a type/count summary using
// captcha_stored_messages_summary, removed after 30 seconds.
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

	if user.Id != targetUserID {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_not_for_you")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

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

	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_error_processing")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	if selectedAnswer == attempt.Answer {
		storedMessages, storedErr := captcha.GetStoredMessagesForAttempt(attempt.ID)
		if storedErr != nil {
			log.Warnf("[Captcha] Failed to read stored messages for attempt %d: %v", attempt.ID, storedErr)
		}

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
			log.Errorf("Failed to unmute user %d on verify: %v", targetUserID, err)
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, attempt.MessageID)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_failed_verify")
			_, _ = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}
		// The correct answer counts as an attempt of its own.
		actionlog.Join(bot, chat, "CAPTCHA_PASSED", targetUserID, user.FirstName,
			actionlog.Field{Name: "Attempts", Value: fmt.Sprintf("%d/%d", attempt.Attempts+1, settings.MaxAttempts)},
		)

		if storedErr == nil && len(storedMessages) > 0 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			var messageTypes []string
			for _, msg := range storedMessages {
				msgTypeStr := messageTypeToString(tr, msg.MessageType)
				if !slices.Contains(messageTypes, msgTypeStr) {
					messageTypes = append(messageTypes, msgTypeStr)
				}
			}
			summaryText, _ := tr.GetString("captcha_stored_messages_summary",
				captchaStoredSummaryParams(formatting.MentionHtml(targetUserID, user.FirstName), len(storedMessages), strings.Join(messageTypes, ", ")))

			summaryMsg, _ := helpers.SendMessageWithErrorHandling(bot, chat.Id, summaryText, &gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
			})

			if summaryMsg != nil {
				time.AfterFunc(30*time.Second, func() {
					defer error_handling.RecoverFromPanic("captchaSummaryMsgDelete", "captcha")
					_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, summaryMsg.MessageId)
				})
			}
		}

		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, attempt.MessageID)

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		msgTemplate, _ := tr.GetString("greetings_captcha_verified_success")
		successMsg := fmt.Sprintf(msgTemplate, formatting.MentionHtml(targetUserID, user.FirstName))
		sent, _ := helpers.SendMessageWithErrorHandling(bot, chat.Id, successMsg, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})

		if sent != nil {
			time.AfterFunc(5*time.Second, func() {
				defer error_handling.RecoverFromPanic("captchaSuccessMsgDelete", "captcha")
				_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, sent.MessageId)
			})
		}

		if err = SendWelcomeMessage(bot, ctx, targetUserID, user.FirstName); err != nil {
			log.Errorf("Failed to send welcome message after captcha verification: %v", err)
		}

		text, _ := tr.GetString("captcha_verified_success_msg")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err

	} else {
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
			claimed, applied := handleCaptchaTimeout(bot, chat.Id, targetUserID, attempt.ID, attempt.MessageID, settings.FailureAction, captchaFailedWrongAnswers)
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

	if user.Id != targetUserID {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_not_for_you")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	cooldownKey := fmt.Sprintf("alita:captcha:refresh:cooldown:%d:%d", chat.Id, targetUserID)
	if m := cache.GetMarshal(); m != nil {
		if exists, _ := m.Get(cache.Context, cooldownKey, new(bool)); exists != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("captcha_wait_refresh")
			_, err := query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return err
		}
	}

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

	if attempt.RefreshCount >= captchaMaxRefreshes {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("captcha_refresh_limit_reached")
		_, err = query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}

	oldMessageID := attempt.MessageID

	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
	}

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

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	refreshBtnText, _ := tr.GetString("captcha_refresh_button")
	keyboard := buildCaptchaKeyboard(attempt.ID, targetUserID, attempt.RefreshCount+1, options, true, refreshBtnText)

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

	if oldMessageID > 0 {
		if err := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, oldMessageID); err != nil {
			log.Warnf("[CaptchaRefresh] Failed to delete old message %d in chat %d: %v", oldMessageID, chat.Id, err)
		}
	}

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

type captchaAttemptMemoKey struct{ chatID, userID int64 }

type captchaAttemptResult struct {
	attempt *models.CaptchaAttempts
	err     error
}

// captchaAttemptForUpdate is captcha.GetCaptchaAttempt memoised for the
// current update, so the pending-message watcher and aispam share one query.
// Attempts are created only from join events, which carry no text for aispam
// to check, so the result cannot go stale between the two.
func captchaAttemptForUpdate(ctx *ext.Context, userID, chatID int64) (*models.CaptchaAttempts, error) {
	r := updatememo.Get(updatememo.From(ctx), captchaAttemptMemoKey{chatID, userID}, func() captchaAttemptResult {
		attempt, err := captcha.GetCaptchaAttempt(userID, chatID)
		return captchaAttemptResult{attempt, err}
	})
	return r.attempt, r.err
}

// handlePendingCaptchaMessage runs in dispatcher group -10, stores pending users' message
// types, text, file IDs and captions, and deletes the originals rather than replaying them.
func (moduleStruct) handlePendingCaptchaMessage(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return ext.ContinueGroups
	}

	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.ContinueGroups
	}

	// On lookup error fall through to the checks below, so a cache or DB
	// failure cannot weaken enforcement.
	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		log.Errorf("[Captcha] Failed to get settings for chat %d: %v", chat.Id, err)
	} else if settings != nil && !settings.Enabled {
		return ext.ContinueGroups
	}

	// No unexpired attempt in the chat means the per-user lookup below cannot
	// find one either. On error fall through, as for the settings lookup.
	if pending, err := captcha.HasPendingCaptchaAttemptsContext(tracing.UpdateContext(ctx), chat.Id); err == nil && !pending {
		return ext.ContinueGroups
	}

	if chat_status.IsUserAdminForUpdate(bot, ctx, chat.Id, user.Id) {
		return ext.ContinueGroups
	}
	if chat_status.IsApprovedForUpdate(bot, ctx, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	attempt, err := captchaAttemptForUpdate(ctx, user.Id, chat.Id)
	if err != nil {
		log.Errorf("Failed to check captcha attempt for user %d: %v", user.Id, err)
		return ext.ContinueGroups
	}

	if attempt == nil {
		return ext.ContinueGroups
	}

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
			fileID = msg.Photo[len(msg.Photo)-1].FileId
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
		messageType = db.TEXT
		content = "[Unsupported message type]"
	}

	err = captcha.StoreMessageForCaptcha(user.Id, chat.Id, attempt.ID, messageType, content, fileID, caption)
	if err != nil {
		log.Errorf("Failed to store message for user %d with pending captcha: %v", user.Id, err)
	}

	_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId)

	return ext.EndGroups
}

func cleanupExpiredCaptchaAttempts(ctx context.Context, bot *gotgbot.Bot) error {
	defer error_handling.RecoverFromPanic("CaptchaCleanupExpiredAttempts", "captcha")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

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

func runCaptchaCleanupTick(baseCtx context.Context, bot *gotgbot.Bot) error {
	ctx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
	defer cancel()
	return cleanupExpiredCaptchaAttempts(ctx, bot)
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
				log.Infof("[CaptchaUnmute] User %d no longer in chat %d, removing from muted list: %v",
					user.UserID, user.ChatID, err)
				if _, deleteErr := captcha.DeleteMutedUserIfUnchanged(user.ID, user.UnmuteAt); deleteErr != nil {
					log.Errorf("[CaptchaUnmute] Failed to delete permanent schedule %d: %v", user.ID, deleteErr)
				}
			} else {
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
				jobs.Run(ctx, "captchaCleanup", func(ctx context.Context) error {
					return runCaptchaCleanupTick(ctx, bot)
				})
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
				jobs.Run(ctx, "captchaUnmute", func(context.Context) error {
					unmuteExpiredCaptchaUsers(bot)
					return nil
				})
			case <-ctx.Done():
				return
			}
		}
	})
}

func LoadCaptcha(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(captchaModule.moduleName, true)

	dispatcher.AddHandlerToGroup(handlers.NewMessage(nil, captchaModule.handlePendingCaptchaMessage), -10)

	// Admin and bot-admin checks run inside the handlers, against the group
	// the user is connected to when the command comes from PM.
	for _, cfg := range []struct {
		name    string
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{"captcha", captchaModule.captchaCommand},
		{"captchamode", captchaModule.captchaModeCommand},
		{"captchatime", captchaModule.captchaTimeCommand},
		{"captchaaction", captchaModule.captchaActionCommand},
		{"captchamaxattempts", captchaModule.captchaMaxAttemptsCommand},
		{"captchapending", captchaModule.viewPendingMessages},
		{"captchaclear", captchaModule.clearPendingMessages},
	} {
		desc := helpers.CommandDescriptor{
			Name:           cfg.name,
			RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled(cfg.name)},
			PMGroupArg:     helpers.PMGroupArgOwn,
		}
		helpers.WrapCommand(dispatcher, desc, pipelineHandler(cfg.handler))
	}

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("captcha_verify"), captchaModule.captchaVerifyCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("captcha_refresh"), captchaModule.captchaRefreshCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(captchaJoinNamespace+"|"), captchaModule.captchaJoinEnableCallback))
}

func init() {
	RegisterLegacyModule("Captcha", 220, LoadCaptcha)
}

// captchaStoredSummaryParams fills captcha_stored_messages_summary by name and,
// under "0".."2", in the order of the "%s %d %s" verbs the locales use: the
// positional fallback does not know "types" and would leave the verbs raw.
func captchaStoredSummaryParams(mention string, count int, types string) i18n.TranslationParams {
	return i18n.TranslationParams{
		"user": mention, "count": count, "types": types,
		"0": mention, "1": count, "2": types,
	}
}
