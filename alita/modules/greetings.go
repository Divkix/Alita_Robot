package modules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/chatjoinrequest"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/media"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var recentJoinProcessTTL = 5 * time.Second

// membershipHandlerGroup holds membership intake's join and leave handlers.
// It runs after the federation watcher (-6) and the users tracker (-1), and
// its handlers end the update, so later groups never see a join.
const membershipHandlerGroup = 0

var (
	greetingsModule      = moduleStruct{moduleName: "Greetings"}
	recentJoinProcessing sync.Map
)

type joinClaim struct{ exp time.Time }

type greetingType int

const (
	greetingWelcome greetingType = iota
	greetingGoodbye
)

type greetingConfig struct {
	gType       greetingType
	setting     *chatsettings.Declaration
	statusKey   string
	enabledKey  string
	disabledKey string
	invalidKey  string
}

var welcomeConfig = greetingConfig{
	gType:       greetingWelcome,
	setting:     chatsettings.Welcome,
	statusKey:   "greetings_welcome_status",
	enabledKey:  "greetings_welcome_enabled",
	disabledKey: "greetings_welcome_disabled",
	invalidKey:  "greetings_welcome_invalid_option",
}

var goodbyeConfig = greetingConfig{
	gType:       greetingGoodbye,
	setting:     chatsettings.Goodbye,
	statusKey:   "greetings_goodbye_status",
	enabledKey:  "greetings_goodbye_enable",
	disabledKey: "greetings_goodbye_disable",
	invalidKey:  "greetings_goodbye_invalid",
}

func recentJoinProcessingKey(chatID, userID int64) string {
	return fmt.Sprintf("alita:recentJoinProcessing:%d:%d", chatID, userID)
}

func claimRecentJoinProcessing(chatID, userID int64) bool {
	claimed, err := tryClaimRecentJoinProcessing(chatID, userID)
	if err != nil {
		// Genuine Redis error — fail closed to avoid double welcome/captcha in multi-instance.
		log.Debugf("[Greetings] Redis SETNX failed for join dedupe key %s, failing closed: %v", recentJoinProcessingKey(chatID, userID), err)
		return false
	}
	return claimed
}

// claimJoin is membership intake's join claim. A Redis error falls back to a
// claim local to this replica, which lets the guard stages run but not the
// challenge or welcome, so an outage neither lets a banned user in nor greets
// anyone twice.
func claimJoin(chatID, userID int64) JoinClaim {
	claimed, err := tryClaimRecentJoinProcessing(chatID, userID)
	if err != nil {
		log.Debugf("[Greetings] Redis SETNX failed for join dedupe key %s, using the local claim: %v", recentJoinProcessingKey(chatID, userID), err)
		if claimRecentJoinLocally(recentJoinProcessingKey(chatID, userID)) {
			return ClaimLocal
		}
		return ClaimLost
	}
	if claimed {
		return ClaimShared
	}
	return ClaimLost
}

// markJoinHandled claims userID's join for a guard that removed them on
// another update, so a later update of that join does nothing.
func markJoinHandled(chatID, userID int64) {
	claimJoin(chatID, userID)
}

// tryClaimRecentJoinProcessing takes the join claim in Redis, or in this
// replica without Redis. It returns the Redis error instead of a decision.
func tryClaimRecentJoinProcessing(chatID, userID int64) (bool, error) {
	key := recentJoinProcessingKey(chatID, userID)

	if rdb := cache.GetRedisClient(); rdb != nil {
		_, err := rdb.SetArgs(cache.Context, key, true, redis.SetArgs{
			Mode: "NX",
			TTL:  recentJoinProcessTTL,
		}).Result()
		if err == nil {
			return true, nil
		}
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return claimRecentJoinLocally(key), nil
}

func claimRecentJoinLocally(key string) bool {
	now := time.Now()
	for {
		newClaim := joinClaim{exp: now.Add(recentJoinProcessTTL)}
		actual, loaded := recentJoinProcessing.LoadOrStore(key, newClaim)
		if !loaded {
			expireRecentJoinClaim(key, newClaim)
			return true
		}
		if c, ok := actual.(joinClaim); ok && now.Before(c.exp) {
			return false
		}
		newClaim = joinClaim{exp: time.Now().Add(recentJoinProcessTTL)}
		if recentJoinProcessing.CompareAndSwap(key, actual, newClaim) {
			expireRecentJoinClaim(key, newClaim)
			return true
		}
		now = time.Now()
	}
}

func expireRecentJoinClaim(key string, claim joinClaim) {
	time.AfterFunc(recentJoinProcessTTL, func() {
		recentJoinProcessing.CompareAndDelete(key, claim)
	})
}

func clearRecentJoinProcessing(chatID, userID int64) {
	key := recentJoinProcessingKey(chatID, userID)

	if rdb := cache.GetRedisClient(); rdb != nil {
		if err := rdb.Del(cache.Context, key).Err(); err != nil {
			log.Debugf("[Greetings] Failed to clear shared join dedupe key %s: %v", key, err)
		}
	}

	recentJoinProcessing.Delete(key)
}

func (moduleStruct) displayGreeting(c *helpers.CommandContext, config greetingConfig) error {
	bot, ctx, msg, chat, user := c.Bot, c.Ctx, c.Msg, c.Chat, c.User
	args := ctx.Args()[1:]

	var greetingText string

	if len(args) == 0 || strings.ToLower(args[0]) == "noformat" {
		noformat := len(args) > 0 && strings.ToLower(args[0]) == "noformat"
		greetPrefs := chatsettings.ReadGreetings(chat.Id)

		var buttons []db.Button
		var fileID string
		var greetingDataType int
		var shouldGreet bool
		var cleanGreet bool

		// ReadGreetings supplies both embedded message configurations.
		if config.gType == greetingWelcome {
			greetingText = greetPrefs.WelcomeSettings.WelcomeText
			buttons = greetings.GetWelcomeButtons(chat.Id)
			fileID = greetPrefs.WelcomeSettings.FileID
			greetingDataType = greetPrefs.WelcomeSettings.WelcomeType
			shouldGreet = greetPrefs.WelcomeSettings.ShouldWelcome
			cleanGreet = greetPrefs.WelcomeSettings.CleanWelcome
		} else {
			greetingText = greetPrefs.GoodbyeSettings.GoodbyeText
			buttons = greetings.GetGoodbyeButtons(chat.Id)
			fileID = greetPrefs.GoodbyeSettings.FileID
			greetingDataType = greetPrefs.GoodbyeSettings.GoodbyeType
			shouldGreet = greetPrefs.GoodbyeSettings.ShouldGoodbye
			cleanGreet = greetPrefs.GoodbyeSettings.CleanGoodbye
		}

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString(config.statusKey)
		_, err := msg.Reply(bot, fmt.Sprintf(text,
			strconv.FormatBool(shouldGreet),
			strconv.FormatBool(cleanGreet),
			strconv.FormatBool(greetPrefs.ShouldCleanService)), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		// The preview goes with the status reply, to the chat the command was
		// sent in, not the connected group a PM command swapped in.
		if noformat {
			greetingText += content.RevertButtons(buttons)
			_, err := media.SendGreeting(bot, msg.Chat.Id, greetingText, fileID, greetingDataType, &gotgbot.InlineKeyboardMarkup{InlineKeyboard: nil}, ctx.EffectiveMessage.MessageThreadId)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			greetingText, buttons = formatting.FormattingReplacer(bot, chat, user, greetingText, buttons)
			keyb := keyboard.BuildKeyboard(buttons)
			keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}
			_, err := media.SendGreeting(bot, msg.Chat.Id, greetingText, fileID, greetingDataType, &keyboard, ctx.EffectiveMessage.MessageThreadId)
			if err != nil {
				log.Error(err)
				return err
			}
		}

	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		var on bool
		key := config.disabledKey
		switch strings.ToLower(args[0]) {
		case "on", "yes":
			on, key = true, config.enabledKey
		case "off", "no":
		default:
			text, _ := tr.GetString(config.invalidKey)
			_, err := msg.Reply(bot, text, formatting.Shtml())
			if err != nil {
				return err
			}
			return ext.EndGroups
		}
		if err := config.setting.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, on); err != nil {
			log.Errorf("[Greetings] %s failed for chat %d: %v", config.setting.Key, chat.Id, err)
			key = "common_settings_save_failed"
		}
		text, _ := tr.GetString(key)
		if _, err := msg.Reply(bot, text, formatting.Shtml()); err != nil {
			return err
		}
	}
	return ext.EndGroups
}

//nolint:dupl // welcome delegates to displayGreeting with different config
func (m moduleStruct) welcome(c *helpers.CommandContext) error {
	return m.displayGreeting(c, welcomeConfig)
}

//nolint:dupl // setWelcome is similar to setGoodbye but uses different DB calls and translation keys
func (moduleStruct) setWelcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	result := content.ExtractWelcome(msg, "welcome", lang.GetLanguage(ctx))
	text, dataType, content, buttons, errorMsg := result.Text, result.DataType, result.FileID, result.Buttons, result.ErrorMsg
	if dataType == -1 {
		_, err := msg.Reply(bot, errorMsg, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if dbErr := greetings.SetWelcomeText(chat.Id, text, content, buttons, dataType); dbErr != nil {
		log.Errorf("[Greetings] SetWelcomeText failed for chat %d: %v", chat.Id, dbErr)
		errText, _ := tr.GetString("common_settings_save_failed")
		_, _ = msg.Reply(bot, errText, formatting.Shtml())
		return ext.EndGroups
	}
	successText, _ := tr.GetString("greetings_welcome_set_success")
	_, err := msg.Reply(bot, successText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

//nolint:dupl // resetGreeting has symmetric welcome/goodbye logic by design
func (moduleStruct) resetGreeting(bot *gotgbot.Bot, ctx *ext.Context, isWelcome bool) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if isWelcome {
		if dbErr := greetings.SetWelcomeText(chat.Id, db.DefaultWelcome, "", nil, db.TEXT); dbErr != nil {
			log.Errorf("[Greetings] SetWelcomeText failed for chat %d: %v", chat.Id, dbErr)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, errText, formatting.Shtml())
			return ext.EndGroups
		}
	} else {
		if dbErr := greetings.SetGoodbyeText(chat.Id, db.DefaultGoodbye, "", nil, db.TEXT); dbErr != nil {
			log.Errorf("[Greetings] SetGoodbyeText failed for chat %d: %v", chat.Id, dbErr)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, errText, formatting.Shtml())
			return ext.EndGroups
		}
	}
	translationKey := "greetings_welcome_reset_success"
	if !isWelcome {
		translationKey = "greetings_goodbye_reset"
	}
	successText, _ := tr.GetString(translationKey)
	_, err := msg.Reply(bot, successText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) resetWelcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.resetGreeting(bot, ctx, true)
}

//nolint:dupl // goodbye delegates to displayGreeting with different config
func (m moduleStruct) goodbye(c *helpers.CommandContext) error {
	return m.displayGreeting(c, goodbyeConfig)
}

//nolint:dupl // setGoodbye is similar to setWelcome but uses different DB calls and translation keys
func (moduleStruct) setGoodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	result := content.ExtractWelcome(msg, "goodbye", lang.GetLanguage(ctx))
	text, dataType, content, buttons, errorMsg := result.Text, result.DataType, result.FileID, result.Buttons, result.ErrorMsg
	if dataType == -1 {
		_, err := msg.Reply(bot, errorMsg, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if dbErr := greetings.SetGoodbyeText(chat.Id, text, content, buttons, dataType); dbErr != nil {
		log.Errorf("[Greetings] SetGoodbyeText failed for chat %d: %v", chat.Id, dbErr)
		errText, _ := tr.GetString("common_settings_save_failed")
		_, _ = msg.Reply(bot, errText, formatting.Shtml())
		return ext.EndGroups
	}
	successText, _ := tr.GetString("greetings_goodbye_set_success")
	_, err := msg.Reply(bot, successText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) resetGoodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.resetGreeting(bot, ctx, false)
}

type greetingToggleConfig struct {
	setting              *chatsettings.Declaration
	showEnabled          string
	showDisabled         string
	setEnabled           string
	setDisabled          string
	invalid              string
	useMarkdownOnEnabled bool
}

var cleanWelcomeToggleConfig = greetingToggleConfig{
	setting:              chatsettings.CleanWelcome,
	showEnabled:          "greetings_clean_welcome_should",
	showDisabled:         "greetings_clean_welcome_not",
	setEnabled:           "greetings_clean_welcome_enable",
	setDisabled:          "greetings_clean_welcome_disable",
	invalid:              "greetings_clean_welcome_invalid_option",
	useMarkdownOnEnabled: false,
}

var cleanGoodbyeToggleConfig = greetingToggleConfig{
	setting:              chatsettings.CleanGoodbye,
	showEnabled:          "greetings_clean_goodbye_should",
	showDisabled:         "greetings_clean_goodbye_not",
	setEnabled:           "greetings_clean_goodbye_enable",
	setDisabled:          "greetings_clean_goodbye_disable",
	invalid:              "greetings_clean_goodbye_invalid_option",
	useMarkdownOnEnabled: false,
}

var delJoinedToggleConfig = greetingToggleConfig{
	setting:              chatsettings.CleanService,
	showEnabled:          "greetings_clean_service_should",
	showDisabled:         "greetings_clean_service_not",
	setEnabled:           "greetings_clean_service_enable",
	setDisabled:          "greetings_clean_service_disable",
	invalid:              "greetings_clean_service_invalid_option",
	useMarkdownOnEnabled: true,
}

var autoApproveToggleConfig = greetingToggleConfig{
	setting:              chatsettings.AutoApprove,
	showEnabled:          "greetings_auto_approve_enabled",
	showDisabled:         "greetings_auto_approve_disabled",
	setEnabled:           "greetings_auto_approve_enable",
	setDisabled:          "greetings_auto_approve_disable",
	invalid:              "greetings_auto_approve_invalid_option",
	useMarkdownOnEnabled: true,
}

func (moduleStruct) greetingToggle(bot *gotgbot.Bot, ctx *ext.Context, cfg greetingToggleConfig) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	var err error

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if len(args) == 0 {
		if cfg.setting.ReadContext(tracing.UpdateContext(ctx), chat.Id).Value.(bool) {
			text, _ := tr.GetString(cfg.showEnabled)
			if cfg.useMarkdownOnEnabled {
				_, err = msg.Reply(bot, text, formatting.Smarkdown())
			} else {
				_, err = msg.Reply(bot, text, formatting.Shtml())
			}
		} else {
			text, _ := tr.GetString(cfg.showDisabled)
			_, err = msg.Reply(bot, text, formatting.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	switch strings.ToLower(args[0]) {
	case "off", "no":
		if dbErr := cfg.setting.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, false); dbErr != nil {
			log.Errorf("[Greetings] %s failed for chat %d: %v", cfg.setting.Key, chat.Id, dbErr)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, errText, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString(cfg.setDisabled)
		_, err = msg.Reply(bot, text, formatting.Shtml())
	case "on", "yes":
		if dbErr := cfg.setting.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, true); dbErr != nil {
			log.Errorf("[Greetings] %s failed for chat %d: %v", cfg.setting.Key, chat.Id, dbErr)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, errText, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString(cfg.setEnabled)
		_, err = msg.Reply(bot, text, formatting.Shtml())
	default:
		text, _ := tr.GetString(cfg.invalid)
		_, err = msg.Reply(bot, text, formatting.Shtml())
	}

	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) cleanWelcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.greetingToggle(bot, ctx, cleanWelcomeToggleConfig)
}

func (m moduleStruct) cleanGoodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.greetingToggle(bot, ctx, cleanGoodbyeToggleConfig)
}

func (m moduleStruct) delJoined(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.greetingToggle(bot, ctx, delJoinedToggleConfig)
}

// SendWelcomeMessage sends the configured welcome message for a user in a chat.
func SendWelcomeMessage(bot *gotgbot.Bot, ctx *ext.Context, userID int64, firstName string) error {
	defer error_handling.RecoverFromPanic("SendWelcomeMessage", "Greetings")
	chat := ctx.EffectiveChat
	greetPrefs := chatsettings.ReadGreetings(chat.Id)

	if greetPrefs.WelcomeSettings.ShouldWelcome {
		user := &gotgbot.User{
			Id:        userID,
			FirstName: firstName,
			IsBot:     false,
		}

		buttons := greetings.GetWelcomeButtons(chat.Id)
		res, buttons := formatting.FormattingReplacer(bot, chat, user,
			greetPrefs.WelcomeSettings.WelcomeText,
			buttons,
		)
		kb := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard.BuildKeyboard(buttons)}

		var threadID int64
		if ctx.EffectiveMessage != nil {
			threadID = ctx.EffectiveMessage.MessageThreadId
		}
		sent, err := media.SendGreeting(bot, chat.Id, res, greetPrefs.WelcomeSettings.FileID, greetPrefs.WelcomeSettings.WelcomeType, kb, threadID)
		if err != nil {
			logJoinOperationError("[Greetings][SendWelcomeMessage] Failed to send welcome message", err)
			return err
		}
		if greetPrefs.WelcomeSettings.CleanWelcome {
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, greetPrefs.WelcomeSettings.LastMsgId)
			if err := greetings.SetCleanWelcomeMsgId(chat.Id, sent.MessageId); err != nil {
				log.Warnf("[Greetings] Failed to store clean welcome msg ID for chat %d: %v", chat.Id, err)
			}
		}
	}
	return nil
}

func (moduleStruct) newMember(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	newMember := ctx.ChatMember.NewChatMember.MergeChatMember().User
	captchaSettings, err := chatsettings.ReadCaptchaContext(tracing.UpdateContext(ctx), chat.Id)
	if err != nil {
		log.Errorf("[Greetings][newMember] Failed to get captcha settings for chat %d: %v", chat.Id, err)
		captchaSettings = &db.CaptchaSettings{Enabled: false}
	}
	if err := processSingleNewMember(bot, ctx, newMember, captchaSettings != nil && captchaSettings.Enabled); err != nil {
		return err
	}
	return ext.EndGroups
}

func (moduleStruct) leftMember(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	leftMember := ctx.ChatMember.OldChatMember.MergeChatMember().User
	greetPrefs := chatsettings.ReadGreetings(chat.Id)

	if leftMember.Id == bot.Id {
		return ext.EndGroups
	}

	// A leave arrives only as this ChatMemberUpdated: cleanService matches the
	// LeftChatMember service message but never processes it. Counting here is
	// therefore exactly one increment per membership change, and delivering
	// the service message too does not add another.
	stats.RecordLeave(chat.Id)

	// A ban keeps the join claim: a guard that banned the newcomer on one
	// update of the join must not see the join again on the other.
	if ctx.ChatMember.NewChatMember.GetStatus() != "kicked" {
		clearRecentJoinProcessing(chat.Id, leftMember.Id)
	}

	captchaAttempt, err := captcha.GetCaptchaAttemptIncludingExpired(leftMember.Id, chat.Id)
	if err != nil {
		log.Errorf("Failed to get captcha attempt for leaving user %d: %v", leftMember.Id, err)
	} else if captchaAttempt != nil {
		if captchaAttempt.MessageID > 0 {
			if delErr := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, captchaAttempt.MessageID); delErr != nil {
				log.Debugf("Failed to delete captcha message for leaving user %d: %v", leftMember.Id, delErr)
			}
		}
		if _, delErr := captcha.DeleteCaptchaAttemptByIDAtomic(captchaAttempt.ID, leftMember.Id, chat.Id); delErr != nil {
			log.Errorf("Failed to delete captcha attempt for leaving user %d: %v", leftMember.Id, delErr)
		}
	}
	if err := captcha.DeleteMutedUser(leftMember.Id, chat.Id); err != nil {
		log.Errorf("Failed to delete scheduled captcha unmute for leaving user %d: %v", leftMember.Id, err)
	}
	endProbationOnLeave(tracing.UpdateContext(ctx), chat.Id, leftMember.Id)

	if greetPrefs.GoodbyeSettings == nil {
		log.Warnf("[Greetings][leftMember] GoodbyeSettings is nil for chat %d, skipping goodbye message", chat.Id)
		return ext.EndGroups
	}

	if greetPrefs.GoodbyeSettings.ShouldGoodbye {
		buttons := greetings.GetGoodbyeButtons(chat.Id)
		res, buttons := formatting.FormattingReplacer(bot, chat, &leftMember, greetPrefs.GoodbyeSettings.GoodbyeText, buttons)
		kb := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard.BuildKeyboard(buttons)}
		var threadID int64
		if ctx.EffectiveMessage != nil {
			threadID = ctx.EffectiveMessage.MessageThreadId
		}
		sent, err := media.SendGreeting(bot, chat.Id, res, greetPrefs.GoodbyeSettings.FileID, greetPrefs.GoodbyeSettings.GoodbyeType, kb, threadID)
		if err != nil {
			logJoinOperationError("[Greetings][leftMember] Failed to send goodbye message", err)
			return err
		}

		if greetPrefs.GoodbyeSettings.CleanGoodbye {
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, greetPrefs.GoodbyeSettings.LastMsgId)
			if err := greetings.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId); err != nil {
				log.Warnf("[Greetings] Failed to store clean goodbye msg ID for chat %d: %v", chat.Id, err)
			}
		}
	}
	return ext.EndGroups
}

// membershipDepsForJoin adapts membership intake to the join in ctx, whose
// effective sender is the member who added the newcomer.
func membershipDepsForJoin(bot *gotgbot.Bot, ctx *ext.Context) MembershipDeps {
	return MembershipDeps{
		Claim: claimJoin,
		IsApproved: func(chatID, userID int64) bool {
			return chat_status.IsApproved(bot, chatID, userID)
		},
		Count: func(user gotgbot.User) { antiRaidModule.countJoin(bot, ctx, user.Id) },
		// The guards in the order a newcomer meets them.
		Stages: []JoinStage{
			func(user gotgbot.User) StageAnswer { return federationsModule.fedBanStage(bot, ctx, user) },
			func(user gotgbot.User) StageAnswer { return antiRaidModule.antiRaidStage(bot, ctx, user) },
			func(user gotgbot.User) StageAnswer { return spamDBModule.spamDBStage(bot, ctx, user) },
			func(user gotgbot.User) StageAnswer { return locksModule.botsLockStage(bot, ctx, user) },
		},
		Challenge: func(user gotgbot.User) error {
			ctxCopy := *ctx
			if err := SendCaptcha(bot, &ctxCopy, user.Id, user.FirstName); err != nil {
				// Only unenforceable challenges or fully rolled-back delivery
				// failures fall through to the newcomer's welcome.
				if errors.Is(err, errCaptchaDisabled) || errors.Is(err, errCaptchaCannotRestrict) || errors.Is(err, errCaptchaDeliverySkipped) {
					return ErrChallengeDisabled
				}
				return err
			}
			return nil
		},
		Probation: func(chatID int64, user gotgbot.User) {
			startProbation(context.Background(), chatID, user)
		},
		Welcome: func(user gotgbot.User) error {
			ctxCopy := *ctx
			return SendWelcomeMessage(bot, &ctxCopy, user.Id, user.FirstName)
		},
	}
}

func processSingleNewMember(bot *gotgbot.Bot, ctx *ext.Context, newMember gotgbot.User, captchaEnabled bool) error {
	chat := ctx.EffectiveChat
	outcome, err := ProcessSingleJoin(chat.Id, bot.Id, newMember, captchaEnabled, membershipDepsForJoin(bot, ctx))
	if err != nil {
		return err
	}
	if outcome == JoinIgnore && newMember.Id != bot.Id {
		log.Debugf("[Greetings][cleanService] Skipping duplicate join processing for user %d in chat %d", newMember.Id, chat.Id)
	}
	return nil
}

func (moduleStruct) cleanService(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return ext.EndGroups
	}

	if user.Id == bot.Id {
		return ext.EndGroups
	}

	if msg.NewChatMembers != nil {
		captchaSettings, err := chatsettings.ReadCaptchaContext(tracing.UpdateContext(ctx), chat.Id)
		if err != nil {
			log.Errorf("[Greetings][cleanService] Failed to get captcha settings for chat %d: %v", chat.Id, err)
			captchaSettings = &db.CaptchaSettings{Enabled: false}
		}
		captchaEnabled := captchaSettings != nil && captchaSettings.Enabled

		var joinErr error
		for _, member := range msg.NewChatMembers {
			joinErr = errors.Join(joinErr, processSingleNewMember(bot, ctx, member, captchaEnabled))
		}
		if joinErr != nil {
			return joinErr
		}
	}

	greetPrefs := chatsettings.ReadGreetings(chat.Id)

	if greetPrefs.ShouldCleanService {
		_, err := msg.Delete(bot, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

func (m moduleStruct) pendingJoins(bot *gotgbot.Bot, ctx *ext.Context) error {
	defer error_handling.RecoverFromPanic("Greetings", "pendingJoins")

	handled, captchaErr := handleJoinRequestCaptcha(bot, ctx)
	if handled {
		if captchaErr != nil {
			return captchaErr
		}
		return ext.ContinueGroups
	}

	chat := ctx.ChatJoinRequest.Chat
	user := ctx.ChatJoinRequest.From

	if !m.loadPendingJoins(chat.Id, user.Id) {

		if chatsettings.AutoApprove.ReadContext(tracing.UpdateContext(ctx), chat.Id).Value.(bool) {
			if _, err := bot.ApproveChatJoinRequest(chat.Id, user.Id, nil); err != nil {
				if isJoinRequestGone(err) || helpers.IsExpectedTelegramError(err) {
					log.Debugf("[Greetings] Expected error auto-approving join for user %d in chat %d: %v", user.Id, chat.Id, err)
				} else {
					log.Error(err)
					return errors.Join(captchaErr, err)
				}
			}
			if captchaErr != nil {
				return captchaErr
			}
			return ext.ContinueGroups
		}

		if err := sendJoinRequestAdminNotice(bot, &chat, user, i18n.MustNewTranslator(lang.GetLanguage(ctx))); err != nil {
			return errors.Join(captchaErr, err)
		}
	}

	if captchaErr != nil {
		return captchaErr
	}
	return ext.ContinueGroups
}

// sendJoinRequestAdminNotice preserves the manual join-request controls for
// delivery failures and approvals Telegram could not complete.
func sendJoinRequestAdminNotice(bot *gotgbot.Bot, chat *gotgbot.Chat, user gotgbot.User, tr *i18n.Translator) error {
	newUserText, _ := tr.GetString("greetings_join_request_new")
	approveText, _ := tr.GetString("greetings_join_request_approve_btn")
	declineText, _ := tr.GetString("greetings_join_request_decline_btn")
	banText, _ := tr.GetString("greetings_join_request_ban_btn")
	userInfoTemplate, _ := tr.GetString("format_user_info")
	userIdTemplate, _ := tr.GetString("format_user_id")
	const namespace = "join_request"

	_, err := helpers.SendMessageWithErrorHandling(bot, chat.Id,
		fmt.Sprint(newUserText,
			"\n"+fmt.Sprintf(userInfoTemplate, formatting.MentionHtml(user.Id, user.FirstName)),
			"\n"+fmt.Sprintf(userIdTemplate, user.Id)),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{Text: approveText, CallbackData: encodeCallbackData(namespace, map[string]string{"a": "accept", "u": fmt.Sprint(user.Id)})},
						{Text: declineText, CallbackData: encodeCallbackData(namespace, map[string]string{"a": "decline", "u": fmt.Sprint(user.Id)})},
					},
					{{Text: banText, CallbackData: encodeCallbackData(namespace, map[string]string{"a": "ban", "u": fmt.Sprint(user.Id)})}},
				},
			},
		})
	if err != nil {
		return err
	}
	greetingsModule.setPendingJoins(chat.Id, user.Id)
	return nil
}

//nolint:gocyclo // Validation and one-shot action handling stay together to prevent partial flows.
func (m moduleStruct) joinRequestHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	defer error_handling.RecoverFromPanic("Greetings", "joinRequestHandler")

	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query.Message == nil {
		return ext.EndGroups
	}
	user := query.From
	chat := ctx.EffectiveChat
	msg := query.Message

	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	response := ""
	joinUserIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "join_request"); ok {
		response, _ = decoded.Field("a")
		joinUserIDRaw, _ = decoded.Field("u")
	}
	if response == "" || joinUserIDRaw == "" {
		log.Warnf("[Greetings] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	if response != "accept" && response != "decline" && response != "ban" {
		log.Warnf("[Greetings] Invalid join request action: %s", response)
		return answerInvalidCallback(b, ctx, query)
	}
	joinUserId, err := strconv.ParseInt(joinUserIDRaw, 10, 64)
	if err != nil {
		log.Errorf("[Greetings] Failed to parse join user ID '%s': %v", joinUserIDRaw, err)
		return answerInvalidCallback(b, ctx, query)
	}

	if response == "ban" {
		if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
			return ext.EndGroups
		}
		if !chat_status.CanBotRestrict(b, ctx, chat) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_error", "chat_status_bot_restrict_error")
			return ext.EndGroups
		}
	}
	if response == "accept" || response == "decline" || response == "ban" {
		if !chat_status.CanUserInvite(b, ctx, chat, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_invite_link_user_error", "chat_status_invite_link_user_error")
			return ext.EndGroups
		}
		if !chat_status.CanBotInvite(b, ctx, chat) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_invite_link_bot_error", "chat_status_invite_link_bot_error")
			return ext.EndGroups
		}
	}

	joinUser, err := b.GetChat(joinUserId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	var helpText string
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// A request another admin already handled, the user withdrew, or that
	// joined some other way is gone; report that instead of the requested action.
	switch response {
	case "accept":
		helpText, _ = tr.GetString("greetings_join_request_accepted")
		if _, err = b.ApproveChatJoinRequest(chat.Id, joinUser.Id, nil); err != nil {
			if isJoinRequestGone(err) {
				helpText, _ = tr.GetString("greetings_join_request_already_handled")
			} else if helpers.IsExpectedTelegramError(err) {
				log.Debugf("[Greetings] Expected error approving join for user %d in chat %d: %v", joinUser.Id, chat.Id, err)
			} else {
				log.Error(err)
				return err
			}
		}
	case "decline":
		helpText, _ = tr.GetString("greetings_join_request_declined")
		if _, err = b.DeclineChatJoinRequest(chat.Id, joinUser.Id, nil); err != nil {
			if isJoinRequestGone(err) {
				helpText, _ = tr.GetString("greetings_join_request_already_handled")
			} else if helpers.IsExpectedTelegramError(err) {
				log.Debugf("[Greetings] Expected error declining join for user %d in chat %d: %v", joinUser.Id, chat.Id, err)
			} else {
				log.Error(err)
				return err
			}
		}
	case "ban":
		if _, banErr := punishment.Apply(tracing.UpdateContext(ctx), b, punishment.Request{
			ChatID: chat.Id, Target: punishment.Target{ID: joinUser.Id}, Mode: punishment.Ban,
			Source: models.BanSourceJoinRequest, ActorID: user.Id,
		}); banErr != nil {
			if helpers.IsExpectedTelegramError(banErr) {
				log.Debugf("[Greetings] Expected error banning user %d in chat %d: %v", joinUser.Id, chat.Id, banErr)
			} else {
				log.Error(banErr)
				return banErr
			}
		} else {
			recordModerationEvent(chat.Id, user.Id, joinUser.Id, models.ModerationActionBan, "")
		}
		// The ban stands even when the request itself is already gone.
		if _, err = b.DeclineChatJoinRequest(chat.Id, joinUser.Id, nil); err != nil && !isJoinRequestGone(err) {
			if helpers.IsExpectedTelegramError(err) {
				log.Debugf("[Greetings] Expected error declining join after ban for user %d in chat %d: %v", joinUser.Id, chat.Id, err)
			} else {
				log.Error(err)
				return err
			}
		}
		helpText, _ = tr.GetString("greetings_join_request_banned")
	}
	m.clearPendingJoins(chat.Id, joinUser.Id)

	_, _, err = msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: fmt.Sprintf(helpText, formatting.MentionHtml(joinUser.Id, joinUser.FirstName)), ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: fmt.Sprintf(helpText, joinUser.FirstName),
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// isJoinRequestGone reports Telegram rejecting a join-request action because the
// request no longer exists.
func isJoinRequestGone(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "HIDE_REQUESTER_MISSING") || strings.Contains(s, "USER_ALREADY_PARTICIPANT")
}

func (m moduleStruct) autoApprove(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.greetingToggle(bot, ctx, autoApproveToggleConfig)
}

func pendingJoinsKey(chatId, userId int64) string {
	return fmt.Sprintf("alita:pendingJoins:%d:%d", chatId, userId)
}

func (moduleStruct) loadPendingJoins(chatId, userId int64) bool {
	m := cache.GetMarshal()
	if m == nil {
		return false
	}
	alreadyAsked, err := m.Get(cache.Context, pendingJoinsKey(chatId, userId), new(bool))
	if err != nil || alreadyAsked == nil {
		return false
	}
	if boolVal, ok := alreadyAsked.(*bool); ok && boolVal != nil {
		return *boolVal
	}
	return false
}

func (moduleStruct) setPendingJoins(chatId, userId int64) {
	m := cache.GetMarshal()
	if m == nil {
		return
	}
	_ = m.Set(cache.Context, pendingJoinsKey(chatId, userId), true, store.WithExpiration(5*time.Minute))
}

func (moduleStruct) clearPendingJoins(chatId, userId int64) {
	if m := cache.GetMarshal(); m != nil {
		_ = m.Delete(cache.Context, pendingJoinsKey(chatId, userId))
	}
}

func LoadGreetings(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(greetingsModule.moduleName, true)

	SetModuleHelpBuilder(greetingsModule.moduleName, func(l string) [][]gotgbot.InlineKeyboardButton {
		return [][]gotgbot.InlineKeyboardButton{{{
			Text:         trS(i18n.MustNewTranslator(l), "button_formatting"),
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Formatting"}),
		}}}
	})

	dispatcher.AddHandler(
		handlers.NewChatJoinRequest(
			chatjoinrequest.All, greetingsModule.pendingJoins,
		),
	)

	dispatcher.AddHandlerToGroup(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				wasMember, isMember := chat_status.ExtractJoinLeftStatusChange(u)
				return !wasMember && isMember
			},
			greetingsModule.newMember,
		), membershipHandlerGroup,
	)

	dispatcher.AddHandlerToGroup(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				wasMember, isMember := chat_status.ExtractJoinLeftStatusChange(u)
				return wasMember && !isMember
			},
			greetingsModule.leftMember,
		), membershipHandlerGroup,
	)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return msg.LeftChatMember != nil || msg.NewChatMembers != nil
			},
			greetingsModule.cleanService,
		), membershipHandlerGroup,
	)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "welcome", PMGroupArg: helpers.PMGroupArgOwn, RequiredChecks: declaredSettingChecks(chatsettings.Welcome),
	}, greetingsModule.welcome)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "goodbye", PMGroupArg: helpers.PMGroupArgOwn, RequiredChecks: declaredSettingChecks(chatsettings.Goodbye),
	}, greetingsModule.goodbye)
	changeInfoChecks := []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), helpers.CanUserChangeInfo()}
	for _, cfg := range []struct {
		name    string
		arg     helpers.PMGroupArg
		handler func(*gotgbot.Bot, *ext.Context) error
		checks  []helpers.CheckFunc
	}{
		{"setwelcome", helpers.PMGroupArgOwn, greetingsModule.setWelcome, changeInfoChecks},
		{"resetwelcome", helpers.PMGroupArgNone, greetingsModule.resetWelcome, changeInfoChecks},
		{"setgoodbye", helpers.PMGroupArgOwn, greetingsModule.setGoodbye, changeInfoChecks},
		{"resetgoodbye", helpers.PMGroupArgNone, greetingsModule.resetGoodbye, changeInfoChecks},
		{"cleanwelcome", helpers.PMGroupArgOwn, greetingsModule.cleanWelcome, declaredSettingChecks(chatsettings.CleanWelcome)},
		{"cleangoodbye", helpers.PMGroupArgOwn, greetingsModule.cleanGoodbye, declaredSettingChecks(chatsettings.CleanGoodbye)},
		{"cleanservice", helpers.PMGroupArgOwn, greetingsModule.delJoined, declaredSettingChecks(chatsettings.CleanService)},
		{"autoapprove", helpers.PMGroupArgOwn, greetingsModule.autoApprove, declaredSettingChecks(chatsettings.AutoApprove)},
	} {
		desc := helpers.CommandDescriptor{
			Name:           cfg.name,
			PMGroupArg:     cfg.arg,
			RequiredChecks: cfg.checks,
		}
		helpers.WrapCommand(dispatcher, desc, pipelineHandler(cfg.handler))
	}
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("join_request"), greetingsModule.joinRequestHandler))
}

func init() {
	RegisterLegacyModule("Greetings", 210, LoadGreetings)
}
