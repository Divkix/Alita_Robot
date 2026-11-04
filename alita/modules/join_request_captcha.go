package modules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/joinrequestcaptchas"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/jobs"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const joinRequestCaptchaPassTTL = 5 * time.Minute

// The local fallback is swept with the persisted attempts when Redis is absent.
var joinRequestCaptchaPasses sync.Map

func joinRequestCaptchaPassKey(chatID, userID int64) string {
	return fmt.Sprintf("alita:joinCaptcha:passed:%d:%d", chatID, userID)
}

func markJoinRequestCaptchaPassed(chatID, userID int64) {
	key := joinRequestCaptchaPassKey(chatID, userID)
	joinRequestCaptchaPasses.Store(key, time.Now().Add(joinRequestCaptchaPassTTL))
	if redis := cache.GetRedisClient(); redis != nil {
		if err := redis.Set(cache.Context, key, 1, joinRequestCaptchaPassTTL).Err(); err != nil {
			log.Warnf("[JoinCaptcha] Could not persist pass marker: %v", err)
		}
	}
}

func clearJoinRequestCaptchaPassed(chatID, userID int64) {
	key := joinRequestCaptchaPassKey(chatID, userID)
	joinRequestCaptchaPasses.Delete(key)
	if redis := cache.GetRedisClient(); redis != nil {
		if err := redis.Del(cache.Context, key).Err(); err != nil {
			log.Warnf("[JoinCaptcha] Could not clear pass marker: %v", err)
		}
	}
}

func joinRequestCaptchaPassed(chatID, userID int64) bool {
	key := joinRequestCaptchaPassKey(chatID, userID)
	if expiry, ok := joinRequestCaptchaPasses.Load(key); ok {
		if time.Now().Before(expiry.(time.Time)) {
			return true
		}
		joinRequestCaptchaPasses.Delete(key)
	}
	if redis := cache.GetRedisClient(); redis != nil {
		exists, err := redis.Exists(cache.Context, key).Result()
		return err == nil && exists > 0
	}
	return false
}

func joinCaptchaTranslator(chatID int64) *i18n.Translator {
	return i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: &gotgbot.Chat{Id: chatID}}))
}

// handleJoinRequestCaptcha returns false only when today's admin/autoapprove
// path must handle the request: the setting is off or the PM could not be sent.
func handleJoinRequestCaptcha(bot *gotgbot.Bot, ctx *ext.Context) (bool, error) {
	request := ctx.ChatJoinRequest
	chat, applicant := request.Chat, request.From
	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).Warnf("[JoinCaptcha] Cannot read settings; using normal join-request handling: %v", err)
		return false, nil
	}
	if !settings.JoinRequests {
		return false, nil
	}
	if chat_status.IsApproved(bot, chat.Id, applicant.Id) {
		_, err = bot.ApproveChatJoinRequestWithContext(tracing.UpdateContext(ctx), chat.Id, applicant.Id, nil)
		if isJoinRequestGone(err) {
			return true, nil
		}
		if err != nil {
			log.WithContext(tracing.UpdateContext(ctx)).Warnf("[JoinCaptcha] Approved-user request could not be approved: %v", err)
			return true, errors.Join(err, joinRequestApprovalFallback(bot, &chat, applicant))
		}
		actionlog.Automated(bot, &chat, "JOIN_REQUEST_APPROVED", applicant.Id, applicant.FirstName,
			actionlog.Field{Name: "Reason", Value: "approved user"})
		return true, nil
	}
	if handled, err := checkJoinRequestSpamDB(bot, ctx); handled || err != nil {
		return handled, err
	}
	if pending, err := joinrequestcaptchas.GetJoinRequestCaptcha(chat.Id, applicant.Id); err != nil {
		return true, err
	} else if pending != nil && time.Now().Before(pending.ExpiresAt) {
		return true, nil
	}
	return sendJoinRequestCaptcha(bot, ctx, settings)
}

func joinRequestApprovalFallback(bot *gotgbot.Bot, chat *gotgbot.Chat, applicant gotgbot.User) error {
	if greetingsModule.loadPendingJoins(chat.Id, applicant.Id) {
		return nil
	}
	return sendJoinRequestAdminNotice(bot, chat, applicant, joinCaptchaTranslator(chat.Id))
}

func checkJoinRequestSpamDB(bot *gotgbot.Bot, ctx *ext.Context) (bool, error) {
	if config.AppConfig == nil || !config.AppConfig.EnableSpamDB || spamDBChecker == nil {
		return false, nil
	}
	request := ctx.ChatJoinRequest
	chat, applicant := request.Chat, request.From
	reqCtx := tracing.UpdateContext(ctx)
	settings, err := spamdbsettings.GetSpamDBSettingsContext(reqCtx, chat.Id)
	if err != nil || !settings.Enabled {
		return false, nil
	}
	source, err := spamDBChecker.Check(reqCtx, applicant.Id, spamDBSources(settings))
	if err != nil || source == "" {
		return false, nil // External lookups fail open, as they do on normal joins.
	}
	var banRecordID int64
	outcome := settings.Action
	if settings.Action == spamdbsettings.ActionBan {
		if _, err = bot.BanChatMemberWithContext(reqCtx, chat.Id, applicant.Id, nil); err != nil {
			if isJoinRequestGone(err) {
				return true, nil
			}
			log.WithContext(reqCtx).WithFields(log.Fields{"chat_id": chat.Id, "user_id": applicant.Id}).
				Warnf("[JoinCaptcha] Spam DB ban failed; declining the request: %v", err)
			outcome += " (failed)"
		} else {
			banRecordID = recordBan(banRecord{chatID: chat.Id, userID: applicant.Id, source: models.BanSourceSpamDB, detail: source.Name()})
		}
	}
	handled := settings.Action != spamdbsettings.ActionLog
	if handled {
		if _, err = bot.DeclineChatJoinRequestWithContext(reqCtx, chat.Id, applicant.Id, nil); err != nil && !isJoinRequestGone(err) {
			return true, err
		}
	}
	actionlog.AutomatedWithMarkup(bot, &chat, unbanLogMarkup(chat.Id, banRecordID), "SPAMDB", applicant.Id, applicant.FirstName,
		actionlog.Field{Name: "Listed in", Value: source.Name()},
		actionlog.Field{Name: "Action", Value: outcome},
		actionlog.Field{Name: "Stage", Value: "join request"})
	return handled, nil
}

func generateJoinRequestCaptcha(mode string) (string, []byte, []string, error) {
	if mode == "math" {
		return generateMathImageCaptcha()
	}
	return generateTextCaptcha()
}

// The shared keyboard supplies layout, options and refresh availability; only
// its callback payload changes because this challenge lives in a private chat.
func buildJoinRequestCaptchaKeyboard(attempt *models.JoinRequestCaptcha, options []string, tr *i18n.Translator) gotgbot.InlineKeyboardMarkup {
	refreshText, _ := tr.GetString("captcha_refresh_button")
	keyboard := buildCaptchaKeyboard(attempt.ID, attempt.UserID, attempt.RefreshCount, options, attempt.RefreshCount < models.CaptchaMaxRefreshes, refreshText)
	for row := range keyboard.InlineKeyboard {
		for column := range keyboard.InlineKeyboard[row] {
			button := &keyboard.InlineKeyboard[row][column]
			fields := map[string]string{"a": fmt.Sprint(attempt.ID), "c": fmt.Sprint(attempt.ChatID), "r": fmt.Sprint(attempt.RefreshCount)}
			if decoded, ok := decodeCallbackData(button.CallbackData, "captcha_verify"); ok {
				fields["s"], _ = decoded.Field("s")
			}
			button.CallbackData = encodeCallbackData("joincaptcha", fields)
		}
	}
	return keyboard
}

func joinRequestCaptchaPrompt(tr *i18n.Translator, chatTitle string, settings *db.CaptchaSettings, timeout int) string {
	text, _ := tr.GetString("captcha_join_request_prompt", i18n.TranslationParams{
		"group": html.EscapeString(chatTitle), "timeout": timeout, "attempts": settings.MaxAttempts,
	})
	return text
}

func sendJoinRequestCaptcha(bot *gotgbot.Bot, ctx *ext.Context, settings *db.CaptchaSettings) (bool, error) {
	request := ctx.ChatJoinRequest
	answer, image, options, err := generateJoinRequestCaptcha(settings.CaptchaMode)
	if err != nil {
		return true, err
	}
	timeout := min(settings.Timeout, 5)
	attempt, err := joinrequestcaptchas.CreateJoinRequestCaptcha(request.Chat.Id, request.From.Id, 0, request.UserChatId, request.From.FirstName, answer, int64(timeout))
	if err != nil {
		return true, err
	}
	tr := joinCaptchaTranslator(request.Chat.Id)
	var sent *gotgbot.Message
	err = ratelimit.GlobalSendLimiter().Do(tracing.UpdateContext(ctx), func() error {
		var sendErr error
		sent, sendErr = bot.SendPhotoWithContext(tracing.UpdateContext(ctx), request.UserChatId,
			gotgbot.InputFileByReader("captcha.png", bytes.NewReader(image)), &gotgbot.SendPhotoOpts{
				Caption: joinRequestCaptchaPrompt(tr, request.Chat.Title, settings, timeout), ParseMode: formatting.HTML,
				ReplyMarkup: buildJoinRequestCaptchaKeyboard(attempt, options, tr),
			})
		return sendErr
	})
	if err != nil || sent == nil || sent.MessageId <= 0 {
		deleteErr := joinrequestcaptchas.DeleteJoinRequestCaptcha(attempt.ID)
		if deleteErr != nil {
			return false, deleteErr
		}
		if isJoinRequestGone(err) {
			return true, nil
		}
		log.WithContext(tracing.UpdateContext(ctx)).Debugf("[JoinCaptcha] PM unavailable; using normal join-request handling: %v", err)
		return false, nil
	}
	if err := joinrequestcaptchas.UpdateJoinRequestCaptchaMessageID(attempt.ID, sent.MessageId); err != nil {
		_, deleteErr := bot.DeleteMessageWithContext(tracing.UpdateContext(ctx), request.UserChatId, sent.MessageId, nil)
		return true, errors.Join(err, deleteErr, joinrequestcaptchas.DeleteJoinRequestCaptcha(attempt.ID))
	}
	attempt.MessageID = sent.MessageId
	scheduleJoinRequestCaptchaTimeout(bot, attempt)
	return true, nil
}

func answerJoinCaptcha(bot *gotgbot.Bot, ctx *ext.Context, chatID int64, key string, params i18n.TranslationParams) error {
	text, _ := joinCaptchaTranslator(chatID).GetString(key, params)
	_, err := ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text, ShowAlert: true})
	return err
}

func joinCaptchaError(bot *gotgbot.Bot, ctx *ext.Context, chatID int64, err error) error {
	return errors.Join(err, answerJoinCaptcha(bot, ctx, chatID, "captcha_error_processing", nil))
}

func joinCaptchaCallbackAttempt(bot *gotgbot.Bot, ctx *ext.Context) (*models.JoinRequestCaptcha, string, error) {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return nil, "", ext.EndGroups
	}
	decoded, ok := decodeCallbackData(query.Data, "joincaptcha")
	if !ok {
		return nil, "", answerInvalidCallback(bot, ctx, query)
	}
	idRaw, _ := decoded.Field("a")
	chatRaw, _ := decoded.Field("c")
	refreshRaw, _ := decoded.Field("r")
	id, idErr := strconv.ParseUint(idRaw, 10, 64)
	chatID, chatErr := strconv.ParseInt(chatRaw, 10, 64)
	refresh, refreshErr := strconv.Atoi(refreshRaw)
	if idErr != nil || chatErr != nil || refreshErr != nil || refresh < 0 {
		return nil, "", answerInvalidCallback(bot, ctx, query)
	}
	attempt, err := joinrequestcaptchas.GetJoinRequestCaptchaByID(uint(id))
	if err != nil {
		return nil, "", joinCaptchaError(bot, ctx, chatID, err)
	}
	if attempt == nil || attempt.ChatID != chatID || attempt.RefreshCount != refresh {
		return nil, "", answerJoinCaptcha(bot, ctx, chatID, "captcha_join_request_expired", nil)
	}
	if !joinCaptchaCallbackMatches(ctx, attempt) {
		return nil, "", answerJoinCaptcha(bot, ctx, chatID, "captcha_join_request_wrong_user", nil)
	}
	if !time.Now().Before(attempt.ExpiresAt) {
		return nil, "", errors.Join(expireJoinRequestCaptcha(tracing.UpdateContext(ctx), bot, attempt), answerJoinCaptcha(bot, ctx, chatID, "captcha_join_request_expired", nil))
	}
	selected, _ := decoded.Field("s")
	return attempt, selected, nil
}

func joinCaptchaCallbackMatches(ctx *ext.Context, attempt *models.JoinRequestCaptcha) bool {
	query := ctx.CallbackQuery
	return query != nil && query.From.Id == attempt.UserID && ctx.EffectiveChat != nil &&
		ctx.EffectiveChat.Id == attempt.UserChatID && query.Message != nil && query.Message.GetMessageId() == attempt.MessageID
}

func (moduleStruct) joinCaptchaCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	attempt, selected, err := joinCaptchaCallbackAttempt(bot, ctx)
	if err != nil || attempt == nil {
		return err
	}
	if selected == "" {
		return refreshJoinRequestCaptcha(bot, ctx, attempt, joinCaptchaTranslator(attempt.ChatID))
	}
	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), attempt.ChatID)
	if err != nil {
		return joinCaptchaError(bot, ctx, attempt.ChatID, err)
	}
	if selected == attempt.Answer {
		return approveJoinRequestCaptcha(bot, ctx, attempt, settings.MaxAttempts)
	}
	attempts, err := joinrequestcaptchas.IncrementJoinRequestCaptchaAttempts(attempt.ID, attempt.RefreshCount, settings.MaxAttempts)
	if errors.Is(err, joinrequestcaptchas.ErrNoActiveCaptcha) {
		return answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_expired", nil)
	}
	if err != nil {
		return joinCaptchaError(bot, ctx, attempt.ChatID, err)
	}
	if attempts >= settings.MaxAttempts {
		if err := finishJoinRequestCaptcha(tracing.UpdateContext(ctx), bot, attempt, "captcha_join_request_declined", "wrong answers"); err != nil {
			return joinCaptchaError(bot, ctx, attempt.ChatID, err)
		}
		return answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_declined_alert", nil)
	}
	return answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_wrong", i18n.TranslationParams{"remaining": settings.MaxAttempts - attempts})
}

func approveJoinRequestCaptcha(bot *gotgbot.Bot, ctx *ext.Context, attempt *models.JoinRequestCaptcha, maxAttempts int) error {
	claimed, err := joinrequestcaptchas.CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, attempt.RefreshCount, maxAttempts)
	if err != nil {
		return joinCaptchaError(bot, ctx, attempt.ChatID, err)
	}
	if !claimed {
		return answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_expired", nil)
	}
	markJoinRequestCaptchaPassed(attempt.ChatID, attempt.UserID)
	if _, err = bot.ApproveChatJoinRequestWithContext(tracing.UpdateContext(ctx), attempt.ChatID, attempt.UserID, nil); err != nil {
		clearJoinRequestCaptchaPassed(attempt.ChatID, attempt.UserID)
		if isJoinRequestGone(err) {
			return answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_expired", nil)
		}
		log.WithContext(tracing.UpdateContext(ctx)).Warnf("[JoinCaptcha] Verified request could not be approved: %v", err)
		return joinCaptchaError(bot, ctx, attempt.ChatID,
			errors.Join(err, joinRequestApprovalFallback(bot, captchaLogChat(attempt.ChatID), ctx.CallbackQuery.From)))
	}
	actionlog.Automated(bot, captchaLogChat(attempt.ChatID), "JOIN_REQUEST_APPROVED", attempt.UserID, ctx.CallbackQuery.From.FirstName,
		actionlog.Field{Name: "Reason", Value: "captcha passed"})
	return errors.Join(editJoinRequestCaptcha(tracing.UpdateContext(ctx), bot, attempt, "captcha_join_request_approved"), answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_verified_success_msg", nil))
}

func refreshJoinRequestCaptcha(bot *gotgbot.Bot, ctx *ext.Context, attempt *models.JoinRequestCaptcha, tr *i18n.Translator) error {
	if attempt.RefreshCount >= models.CaptchaMaxRefreshes || time.Since(attempt.UpdatedAt) < time.Duration(captchaRefreshCooldownS)*time.Second {
		text, _ := tr.GetString("captcha_wait_refresh")
		_, err := ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return err
	}
	settings, err := captcha.GetCaptchaSettingsContext(tracing.UpdateContext(ctx), attempt.ChatID)
	if err != nil {
		return joinCaptchaError(bot, ctx, attempt.ChatID, err)
	}
	answer, image, options, err := generateJoinRequestCaptcha(settings.CaptchaMode)
	if err != nil {
		return joinCaptchaError(bot, ctx, attempt.ChatID, err)
	}
	next := *attempt
	next.RefreshCount++
	keyboard := buildJoinRequestCaptchaKeyboard(&next, options, tr)
	reqCtx := tracing.UpdateContext(ctx)
	var sent *gotgbot.Message
	err = ratelimit.GlobalSendLimiter().Do(reqCtx, func() error {
		var sendErr error
		sent, sendErr = bot.SendPhotoWithContext(reqCtx, attempt.UserChatID,
			gotgbot.InputFileByReader("captcha.png", bytes.NewReader(image)), &gotgbot.SendPhotoOpts{
				Caption:   joinRequestCaptchaPrompt(tr, captchaLogChat(attempt.ChatID).Title, settings, max(1, int(time.Until(attempt.ExpiresAt).Minutes()))),
				ParseMode: formatting.HTML, ReplyMarkup: keyboard,
			})
		return sendErr
	})
	if isJoinRequestGone(err) {
		return errors.Join(joinrequestcaptchas.DeleteJoinRequestCaptcha(attempt.ID),
			answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_expired", nil))
	}
	if err != nil {
		return joinCaptchaError(bot, ctx, attempt.ChatID, err)
	}
	if sent == nil || sent.MessageId <= 0 {
		return joinCaptchaError(bot, ctx, attempt.ChatID, errors.New("telegram returned no refreshed join-request captcha"))
	}
	fresh, err := joinrequestcaptchas.RefreshJoinRequestCaptcha(attempt.ID, answer, attempt.RefreshCount, sent.MessageId)
	if err != nil || fresh == nil {
		_, deleteErr := bot.DeleteMessageWithContext(reqCtx, attempt.UserChatID, sent.MessageId, nil)
		if err != nil {
			return joinCaptchaError(bot, ctx, attempt.ChatID, errors.Join(err, deleteErr))
		}
		return errors.Join(deleteErr, answerJoinCaptcha(bot, ctx, attempt.ChatID, "captcha_join_request_expired", nil))
	}
	if attempt.MessageID != sent.MessageId {
		if _, err := bot.DeleteMessageWithContext(reqCtx, attempt.UserChatID, attempt.MessageID, nil); err != nil {
			return joinCaptchaError(bot, ctx, attempt.ChatID, err)
		}
	}
	_, err = ctx.CallbackQuery.Answer(bot, nil)
	return err
}

func editJoinRequestCaptcha(ctx context.Context, bot *gotgbot.Bot, attempt *models.JoinRequestCaptcha, key string) error {
	text, _ := joinCaptchaTranslator(attempt.ChatID).GetString(key, i18n.TranslationParams{"group": html.EscapeString(captchaLogChat(attempt.ChatID).Title)})
	_, _, err := bot.EditMessageCaptionWithContext(ctx, &gotgbot.EditMessageCaptionOpts{
		ChatId: attempt.UserChatID, MessageId: attempt.MessageID,
		Caption: text, ParseMode: formatting.HTML, ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{}},
	})
	if isJoinRequestGone(err) {
		return nil
	}
	return err
}

func finishJoinRequestCaptcha(ctx context.Context, bot *gotgbot.Bot, attempt *models.JoinRequestCaptcha, key, reason string) error {
	claimed, err := joinrequestcaptchas.ReleaseJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, attempt.RefreshCount)
	if err != nil || !claimed {
		return err
	}
	if _, err = bot.DeclineChatJoinRequestWithContext(ctx, attempt.ChatID, attempt.UserID, nil); err != nil {
		if isJoinRequestGone(err) {
			return nil
		}
		return err
	}
	name := attempt.FirstName
	if name == "" {
		name = strconv.FormatInt(attempt.UserID, 10)
	}
	actionlog.Automated(bot, captchaLogChat(attempt.ChatID), "JOIN_REQUEST_DECLINED", attempt.UserID, name,
		actionlog.Field{Name: "Reason", Value: reason})
	return editJoinRequestCaptcha(ctx, bot, attempt, key)
}

func expireJoinRequestCaptcha(ctx context.Context, bot *gotgbot.Bot, attempt *models.JoinRequestCaptcha) error {
	if time.Now().Before(attempt.ExpiresAt) {
		return nil
	}
	if attempt.MessageID <= 0 {
		return joinrequestcaptchas.DeleteJoinRequestCaptcha(attempt.ID)
	}
	return finishJoinRequestCaptcha(ctx, bot, attempt, "captcha_join_request_timeout", "timeout")
}

func scheduleJoinRequestCaptchaTimeout(bot *gotgbot.Bot, attempt *models.JoinRequestCaptcha) {
	startCaptchaLifecycleTask(func(ctx context.Context) {
		defer error_handling.RecoverFromPanic("joinRequestCaptchaTimeout", "captcha")
		timer := time.NewTimer(max(time.Until(attempt.ExpiresAt), 0))
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			jobs.Run(ctx, "joinRequestCaptchaTimeout", func(ctx context.Context) error {
				fresh, err := joinrequestcaptchas.GetJoinRequestCaptchaByID(attempt.ID)
				if err != nil || fresh == nil {
					return err
				}
				return expireJoinRequestCaptcha(ctx, bot, fresh)
			})
		}
	})
}

func recoverJoinRequestCaptchas(bot *gotgbot.Bot) error {
	attempts, err := joinrequestcaptchas.GetPendingJoinRequestCaptchas()
	if err != nil {
		return err
	}
	for _, attempt := range attempts {
		scheduleJoinRequestCaptchaTimeout(bot, attempt)
	}
	return nil
}

func cleanupExpiredJoinRequestCaptchas(ctx context.Context, bot *gotgbot.Bot) error {
	joinRequestCaptchaPasses.Range(func(key, value any) bool {
		if time.Now().After(value.(time.Time)) {
			joinRequestCaptchaPasses.Delete(key)
		}
		return true
	})
	attempts, err := joinrequestcaptchas.GetExpiredJoinRequestCaptchas()
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return errors.Join(cleanupErr, err)
		}
		cleanupErr = errors.Join(cleanupErr, expireJoinRequestCaptcha(ctx, bot, attempt))
	}
	return cleanupErr
}
