package modules

import (
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
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/media"
)

// Concurrency limit for processing multiple new members
const (
	maxConcurrentMemberProcessing = 5 // Maximum concurrent member welcome/captcha processing
)

var greetingsModule = moduleStruct{moduleName: "Greetings"}

type greetingType int

const (
	greetingWelcome greetingType = iota
	greetingGoodbye
)

type greetingConfig struct {
	gType            greetingType
	logContext       string
	notConfiguredKey string
	statusKey        string
	enabledKey       string
	disabledKey      string
	invalidKey       string
}

var welcomeConfig = greetingConfig{
	gType:            greetingWelcome,
	logContext:       "welcome",
	notConfiguredKey: "greetings_welcome_not_configured",
	statusKey:        "greetings_welcome_status",
	enabledKey:       "greetings_welcome_enabled",
	disabledKey:      "greetings_welcome_disabled",
	invalidKey:       "greetings_welcome_invalid_option",
}

var goodbyeConfig = greetingConfig{
	gType:            greetingGoodbye,
	logContext:       "goodbye",
	notConfiguredKey: "greetings_goodbye_not_configured",
	statusKey:        "greetings_goodbye_status",
	enabledKey:       "greetings_goodbye_enable",
	disabledKey:      "greetings_goodbye_disable",
	invalidKey:       "greetings_goodbye_invalid",
}

// displayGreeting is a shared helper function that handles both welcome and goodbye greeting display/toggling.
// It consolidates common logic between welcome() and goodbye() commands.
func (moduleStruct) displayGreeting(bot *gotgbot.Bot, ctx *ext.Context, config greetingConfig) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	var greetingText string

	if len(args) == 0 || strings.ToLower(args[0]) == "noformat" {
		noformat := len(args) > 0 && strings.ToLower(args[0]) == "noformat"
		greetPrefs := db.GetGreetingSettings(chat.Id)

		// Get the appropriate settings based on greeting type
		var buttons []db.Button
		var fileID string
		var greetingDataType int
		var shouldGreet bool
		var cleanGreet bool

		if config.gType == greetingWelcome {
			if greetPrefs.WelcomeSettings == nil {
				log.Warnf("[Greetings][%s] WelcomeSettings is nil for chat %d, using defaults", config.logContext, chat.Id)
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString(config.notConfiguredKey)
				_, err := msg.Reply(bot, text, helpers.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return ext.EndGroups
			}
			greetingText = greetPrefs.WelcomeSettings.WelcomeText
			buttons = db.GetWelcomeButtons(chat.Id)
			fileID = greetPrefs.WelcomeSettings.FileID
			greetingDataType = greetPrefs.WelcomeSettings.WelcomeType
			shouldGreet = greetPrefs.WelcomeSettings.ShouldWelcome
			cleanGreet = greetPrefs.WelcomeSettings.CleanWelcome
		} else {
			if greetPrefs.GoodbyeSettings == nil {
				log.Warnf("[Greetings][%s] GoodbyeSettings is nil for chat %d, using defaults", config.logContext, chat.Id)
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString(config.notConfiguredKey)
				_, err := msg.Reply(bot, text, helpers.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return ext.EndGroups
			}
			greetingText = greetPrefs.GoodbyeSettings.GoodbyeText
			buttons = db.GetGoodbyeButtons(chat.Id)
			fileID = greetPrefs.GoodbyeSettings.FileID
			greetingDataType = greetPrefs.GoodbyeSettings.GoodbyeType
			shouldGreet = greetPrefs.GoodbyeSettings.ShouldGoodbye
			cleanGreet = greetPrefs.GoodbyeSettings.CleanGoodbye
		}

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString(config.statusKey)
		_, err := msg.Reply(bot, fmt.Sprintf(text,
			shouldGreet,
			cleanGreet,
			greetPrefs.ShouldCleanService), helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		if noformat {
			greetingText += helpers.RevertButtons(buttons)
			_, err := media.SendGreeting(bot, ctx.EffectiveChat.Id, greetingText, fileID, greetingDataType, &gotgbot.InlineKeyboardMarkup{InlineKeyboard: nil}, ctx.EffectiveMessage.MessageThreadId)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			greetingText, buttons = helpers.FormattingReplacer(bot, chat, user, greetingText, buttons)
			keyb := helpers.BuildKeyboard(buttons)
			keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}
			_, err := media.SendGreeting(bot, ctx.EffectiveChat.Id, greetingText, fileID, greetingDataType, &keyboard, ctx.EffectiveMessage.MessageThreadId)
			if err != nil {
				log.Error(err)
				return err
			}
		}

	} else if len(args) >= 1 {
		var err error
		switch strings.ToLower(args[0]) {
		case "on", "yes":
			if config.gType == greetingWelcome {
				db.SetWelcomeToggle(chat.Id, true)
			} else {
				db.SetGoodbyeToggle(chat.Id, true)
			}
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString(config.enabledKey)
			_, err = msg.Reply(bot, text, helpers.Shtml())
		case "off", "no":
			if config.gType == greetingWelcome {
				db.SetWelcomeToggle(chat.Id, false)
			} else {
				db.SetGoodbyeToggle(chat.Id, false)
			}
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString(config.disabledKey)
			_, err = msg.Reply(bot, text, helpers.Shtml())
		default:
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString(config.invalidKey)
			_, err = msg.Reply(bot, text, helpers.Shtml())
		}

		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// welcome manages welcome message settings and displays current welcome configuration.
// Admins can toggle welcome messages on/off or view current settings with 'noformat' option.
func (m moduleStruct) welcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.displayGreeting(bot, ctx, welcomeConfig)
}

// setWelcome allows admins to set a custom welcome message for new chat members.
// Supports text, media, and inline buttons with formatting and placeholder variables.
func (moduleStruct) setWelcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	text, dataType, content, buttons, errorMsg := helpers.GetWelcomeType(msg, "welcome", db.GetLanguage(ctx))
	if dataType == -1 {
		_, err := msg.Reply(bot, errorMsg, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	db.SetWelcomeText(chat.Id, text, content, buttons, dataType)
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	successText, _ := tr.GetString("greetings_welcome_set_success")
	_, err := msg.Reply(bot, successText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetGreeting is a shared helper for resetting welcome or goodbye messages to defaults.
// It consolidates the common logic between resetWelcome and resetGoodbye.
func (moduleStruct) resetGreeting(bot *gotgbot.Bot, ctx *ext.Context, isWelcome bool) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	// Reset greeting text synchronously to ensure DB write completes before sending success
	if isWelcome {
		db.SetWelcomeText(chat.Id, db.DefaultWelcome, "", nil, db.TEXT)
	} else {
		db.SetGoodbyeText(chat.Id, db.DefaultGoodbye, "", nil, db.TEXT)
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	translationKey := "greetings_welcome_reset_success"
	if !isWelcome {
		translationKey = "greetings_goodbye_reset"
	}
	successText, _ := tr.GetString(translationKey)
	_, err := msg.Reply(bot, successText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetWelcome resets the welcome message back to the default bot welcome message.
// Only admins can use this command to restore the original welcome text.
func (m moduleStruct) resetWelcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.resetGreeting(bot, ctx, true)
}

// goodbye manages goodbye message settings and displays current goodbye configuration.
// Admins can toggle goodbye messages on/off or view current settings with 'noformat' option.
func (m moduleStruct) goodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.displayGreeting(bot, ctx, goodbyeConfig)
}

// setGoodbye allows admins to set a custom goodbye message for members leaving the chat.
// Supports text, media, and inline buttons with formatting and placeholder variables.
func (moduleStruct) setGoodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	text, dataType, content, buttons, errorMsg := helpers.GetWelcomeType(msg, "goodbye", db.GetLanguage(ctx))
	if dataType == -1 {
		_, err := msg.Reply(bot, errorMsg, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	db.SetGoodbyeText(chat.Id, text, content, buttons, dataType)
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	successText, _ := tr.GetString("greetings_goodbye_set_success")
	_, err := msg.Reply(bot, successText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// resetGoodbye resets the goodbye message back to the default bot goodbye message.
// Only admins can use this command to restore the original goodbye text.
func (m moduleStruct) resetGoodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	return m.resetGreeting(bot, ctx, false)
}

// cleanWelcome toggles automatic deletion of old welcome messages.
// Admins can enable/disable cleanup or check current setting. Helps keep chats tidy.
func (moduleStruct) cleanWelcome(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	var err error
	user := ctx.EffectiveSender.User
	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		var err error
		greetSettings := db.GetGreetingSettings(chat.Id)

		// Nil check for WelcomeSettings
		if greetSettings.WelcomeSettings == nil {
			log.Warnf("[Greetings][cleanWelcome] WelcomeSettings is nil for chat %d, using default (false)", chat.Id)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("greetings_clean_welcome_should")
			_, err = msg.Reply(bot, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}

		cleanPref := greetSettings.WelcomeSettings.CleanWelcome
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if !cleanPref {
			text, _ := tr.GetString("greetings_clean_welcome_should")
			_, err = msg.Reply(bot, text, helpers.Shtml())
		} else {
			text, _ := tr.GetString("greetings_clean_welcome_not")
			_, err = msg.Reply(bot, text, helpers.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	switch strings.ToLower(args[0]) {
	case "off", "no":
		db.SetCleanWelcomeSetting(chat.Id, false)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_welcome_disable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	case "on", "yes":
		db.SetCleanWelcomeSetting(chat.Id, true)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_welcome_enable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	default:
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_welcome_invalid_option")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	}

	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// cleanGoodbye toggles automatic deletion of old goodbye messages.
// Admins can enable/disable cleanup or check current setting. Helps keep chats tidy.
func (moduleStruct) cleanGoodbye(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]
	var err error
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		var err error
		greetSettings := db.GetGreetingSettings(chat.Id)

		// Nil check for GoodbyeSettings
		if greetSettings.GoodbyeSettings == nil {
			log.Warnf("[Greetings][cleanGoodbye] GoodbyeSettings is nil for chat %d, using default (false)", chat.Id)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("greetings_clean_goodbye_should")
			_, err = msg.Reply(bot, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}

		cleanPref := greetSettings.GoodbyeSettings.CleanGoodbye
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if !cleanPref {
			text, _ := tr.GetString("greetings_clean_goodbye_should")
			_, err = msg.Reply(bot, text, helpers.Shtml())
		} else {
			text, _ := tr.GetString("greetings_clean_goodbye_not")
			_, err = msg.Reply(bot, text, helpers.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	switch strings.ToLower(args[0]) {
	case "off", "no":
		db.SetCleanGoodbyeSetting(chat.Id, false)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_goodbye_disable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	case "on", "yes":
		db.SetCleanGoodbyeSetting(chat.Id, true)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_goodbye_enable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	default:
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_goodbye_invalid_option")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	}

	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// delJoined toggles automatic deletion of service messages when users join the chat.
// Admins can enable/disable cleanup of 'user joined' messages or check current setting.
func (moduleStruct) delJoined(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]
	var err error
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		delPref := db.GetGreetingSettings(chat.Id).ShouldCleanService
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if delPref {
			text, _ := tr.GetString("greetings_clean_service_should")
			_, err = msg.Reply(bot, text, helpers.Smarkdown())
		} else {
			text, _ := tr.GetString("greetings_clean_service_not")
			_, err = msg.Reply(bot, text, helpers.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	switch strings.ToLower(args[0]) {
	case "off", "no":
		db.SetShouldCleanService(chat.Id, false)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_service_disable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	case "on", "yes":
		db.SetShouldCleanService(chat.Id, true)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_service_enable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	default:
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_clean_service_invalid_option")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	}

	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// SendWelcomeMessage sends the configured welcome message for a user in a chat.
// This is extracted as a separate function to be reusable after captcha verification.
func SendWelcomeMessage(bot *gotgbot.Bot, ctx *ext.Context, userID int64, firstName string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Greetings][SendWelcomeMessage] Recovered from panic: %v", r)
		}
	}()
	chat := ctx.EffectiveChat
	greetPrefs := db.GetGreetingSettings(chat.Id)

	// Nil check for WelcomeSettings
	if greetPrefs.WelcomeSettings == nil {
		log.Warnf("[Greetings][SendWelcomeMessage] WelcomeSettings is nil for chat %d, skipping welcome message", chat.Id)
		return nil
	}

	if greetPrefs.WelcomeSettings.ShouldWelcome {
		// Create a user object for formatting
		user := &gotgbot.User{
			Id:        userID,
			FirstName: firstName,
			IsBot:     false,
		}

		buttons := db.GetWelcomeButtons(chat.Id)
		res, buttons := helpers.FormattingReplacer(bot, chat, user,
			greetPrefs.WelcomeSettings.WelcomeText,
			buttons,
		)
		keyboard := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: helpers.BuildKeyboard(buttons)}

		var threadID int64
		if ctx.EffectiveMessage != nil {
			threadID = ctx.EffectiveMessage.MessageThreadId
		}
		sent, err := media.SendGreeting(bot, chat.Id, res, greetPrefs.WelcomeSettings.FileID, greetPrefs.WelcomeSettings.WelcomeType, keyboard, threadID)
		if err != nil {
			log.Error(err)
			return err
		}
		if greetPrefs.WelcomeSettings.CleanWelcome {
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, greetPrefs.WelcomeSettings.LastMsgId)
			db.SetCleanWelcomeMsgId(chat.Id, sent.MessageId)
		}
	}
	return nil
}

// newMember handles welcome messages when new members join the chat.
// Automatically sends welcome message and manages cleanup based on chat settings.
func (moduleStruct) newMember(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	newMember := ctx.ChatMember.NewChatMember.MergeChatMember().User

	// when bot joins stop all updates of the groups
	// we use bot_updates for this
	if newMember.Id == bot.Id {
		return ext.EndGroups
	}

	// Check if captcha is enabled
	captchaSettings, err := db.GetCaptchaSettings(chat.Id)
	if err != nil {
		log.Errorf("[Greetings][newMember] Failed to get captcha settings for chat %d: %v", chat.Id, err)
		// Continue with welcome message on error (captcha disabled by default)
		captchaSettings = &db.CaptchaSettings{Enabled: false}
	}
	if captchaSettings != nil && captchaSettings.Enabled {
		// Mute the new member immediately
		_, err := chat.RestrictMember(bot, newMember.Id, gotgbot.ChatPermissions{
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

		if err != nil {
			log.Errorf("Failed to mute user %d for captcha: %v", newMember.Id, err)
			// Continue with normal welcome if muting fails
		} else {
			// Send captcha instead of welcome message
			err = SendCaptcha(bot, ctx, newMember.Id, newMember.FirstName)
			if err != nil {
				log.Errorf("Failed to send captcha to user %d: %v", newMember.Id, err)
				// Unmute the user if captcha sending fails
				_, _ = chat.RestrictMember(bot, newMember.Id, gotgbot.ChatPermissions{
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
			} else {
				// Captcha sent successfully, don't send welcome message yet
				return ext.EndGroups
			}
		}
	}

	// Send welcome message if captcha is disabled or failed
	if err := SendWelcomeMessage(bot, ctx, newMember.Id, newMember.FirstName); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// leftMember handles goodbye messages when members leave the chat.
// Automatically sends goodbye message and manages cleanup based on chat settings.
func (moduleStruct) leftMember(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	leftMember := ctx.ChatMember.OldChatMember.MergeChatMember().User
	greetPrefs := db.GetGreetingSettings(chat.Id)

	// when bot leaves stop all updates of the groups
	if leftMember.Id == bot.Id {
		return ext.EndGroups
	}

	// Clean up any pending captcha for the leaving user
	captchaAttempt, err := db.GetCaptchaAttempt(leftMember.Id, chat.Id)
	if err != nil {
		log.Errorf("Failed to get captcha attempt for leaving user %d: %v", leftMember.Id, err)
	} else if captchaAttempt != nil {
		// Delete the captcha message if it exists
		if captchaAttempt.MessageID > 0 {
			if delErr := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, captchaAttempt.MessageID); delErr != nil {
				log.Debugf("Failed to delete captcha message for leaving user %d: %v", leftMember.Id, delErr)
			}
		}
		// Delete the captcha attempt from database
		if delErr := db.DeleteCaptchaAttempt(leftMember.Id, chat.Id); delErr != nil {
			log.Errorf("Failed to delete captcha attempt for leaving user %d: %v", leftMember.Id, delErr)
		}
	}

	// Nil check for GoodbyeSettings
	if greetPrefs.GoodbyeSettings == nil {
		log.Warnf("[Greetings][leftMember] GoodbyeSettings is nil for chat %d, skipping goodbye message", chat.Id)
		return ext.EndGroups
	}

	if greetPrefs.GoodbyeSettings.ShouldGoodbye {
		buttons := db.GetGoodbyeButtons(chat.Id)
		res, buttons := helpers.FormattingReplacer(bot, chat, &leftMember, greetPrefs.GoodbyeSettings.GoodbyeText, buttons)
		keyboard := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: helpers.BuildKeyboard(buttons)}
		var threadID int64
		if ctx.EffectiveMessage != nil {
			threadID = ctx.EffectiveMessage.MessageThreadId
		}
		sent, err := media.SendGreeting(bot, chat.Id, res, greetPrefs.GoodbyeSettings.FileID, greetPrefs.GoodbyeSettings.GoodbyeType, keyboard, threadID)
		if err != nil {
			log.Error(err)
			return err
		}

		if greetPrefs.GoodbyeSettings.CleanGoodbye {
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, greetPrefs.GoodbyeSettings.LastMsgId)
			db.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId)
			// if err.Error() == "unable to deleteMessage: Bad Request: message to delete not found" {
			// 	log.WithFields(
			// 		log.Fields{
			// 			"chat": chat.Id,
			// 		},
			// 	).Error("error deleting message")
			// 	return ext.EndGroups
			// }
		}
	}
	return ext.EndGroups
}

// processSingleNewMember handles a single new member joining (mute, captcha, welcome).
// This is extracted to enable concurrent processing of multiple members.
func processSingleNewMember(bot *gotgbot.Bot, ctx *ext.Context, newMember gotgbot.User, captchaEnabled bool) {
	chat := ctx.EffectiveChat

	if newMember.Id == bot.Id {
		return
	}

	if captchaEnabled {
		// Mute the new member immediately
		_, err := chat.RestrictMember(bot, newMember.Id, gotgbot.ChatPermissions{
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

		if err != nil {
			log.Errorf("Failed to mute user %d for captcha: %v", newMember.Id, err)
			// Send welcome if muting fails
			if err := SendWelcomeMessage(bot, ctx, newMember.Id, newMember.FirstName); err != nil {
				log.Error(err)
			}
		} else {
			// Send captcha instead of welcome message
			err = SendCaptcha(bot, ctx, newMember.Id, newMember.FirstName)
			if err != nil {
				log.Errorf("Failed to send captcha to user %d: %v", newMember.Id, err)
				// Unmute the user if captcha sending fails
				_, _ = chat.RestrictMember(bot, newMember.Id, gotgbot.ChatPermissions{
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
				// Send welcome if captcha fails
				if err := SendWelcomeMessage(bot, ctx, newMember.Id, newMember.FirstName); err != nil {
					log.Error(err)
				}
			}
		}
	} else {
		// Captcha is disabled, send welcome message
		if err := SendWelcomeMessage(bot, ctx, newMember.Id, newMember.FirstName); err != nil {
			log.Error(err)
		}
	}
}

// cleanService automatically deletes service messages about members joining/leaving.
// Runs when service messages are posted and deletes them if cleanup is enabled.
// Also handles captcha for users joining via invite links or being added.
func (moduleStruct) cleanService(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	if user.Id == bot.Id {
		return ext.EndGroups
	}

	// Handle new members joining via invite links or being added
	if msg.NewChatMembers != nil {
		captchaSettings, err := db.GetCaptchaSettings(chat.Id)
		if err != nil {
			log.Errorf("[Greetings][cleanService] Failed to get captcha settings for chat %d: %v", chat.Id, err)
			// Default to disabled captcha on error
			captchaSettings = &db.CaptchaSettings{Enabled: false}
		}
		captchaEnabled := captchaSettings != nil && captchaSettings.Enabled

		// Process multiple members concurrently for better performance
		numMembers := len(msg.NewChatMembers)
		if numMembers > 1 {
			// Use goroutines for multiple members
			var wg sync.WaitGroup
			// Limit concurrent processing to prevent overwhelming the API
			sem := make(chan struct{}, maxConcurrentMemberProcessing)

			for _, newMember := range msg.NewChatMembers {
				if newMember.Id == bot.Id {
					continue
				}

				wg.Add(1)
				sem <- struct{}{} // Acquire semaphore

				go func(member gotgbot.User) {
					defer wg.Done()
					defer func() { <-sem }() // Release semaphore

					processSingleNewMember(bot, ctx, member, captchaEnabled)
				}(newMember)
			}

			wg.Wait()
		} else if numMembers == 1 {
			// For single member, process directly without goroutine
			processSingleNewMember(bot, ctx, msg.NewChatMembers[0], captchaEnabled)
		}
	}

	greetPrefs := db.GetGreetingSettings(chat.Id)

	if greetPrefs.ShouldCleanService {
		_, err := msg.Delete(bot, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// pendingJoins handles chat join requests and creates approval buttons for admins.
// Auto-approves if enabled, otherwise presents approve/decline/ban options to admins.
func (m moduleStruct) pendingJoins(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.ChatJoinRequest.Chat
	user := ctx.ChatJoinRequest.From
	joinReqStr := "join_request"

	if !m.loadPendingJoins(chat.Id, user.Id) {

		// auto approve join requests
		if db.GetGreetingSettings(chat.Id).ShouldAutoApprove {
			_, _ = bot.ApproveChatJoinRequest(chat.Id, user.Id, nil)
			return ext.ContinueGroups
		}

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		newUserText, _ := tr.GetString("greetings_join_request_new")
		approveText, _ := tr.GetString("greetings_join_request_approve_btn")
		declineText, _ := tr.GetString("greetings_join_request_decline_btn")
		banText, _ := tr.GetString("greetings_join_request_ban_btn")
		userInfoTemplate, _ := tr.GetString("format_user_info")
		userIdTemplate, _ := tr.GetString("format_user_id")

		_, err := helpers.SendMessageWithErrorHandling(
			bot,
			chat.Id,
			fmt.Sprint(
				newUserText,
				"\n"+fmt.Sprintf(userInfoTemplate, helpers.MentionHtml(user.Id, user.FirstName)),
				"\n"+fmt.Sprintf(userIdTemplate, user.Id),
			),
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         approveText,
								CallbackData: fmt.Sprintf("%s.accept.%d", joinReqStr, user.Id),
							},
							{
								Text:         declineText,
								CallbackData: fmt.Sprintf("%s.decline.%d", joinReqStr, user.Id),
							},
						},
						{
							{
								Text:         banText,
								CallbackData: fmt.Sprintf("%s.ban.%d", joinReqStr, user.Id),
							},
						},
					},
				},
			},
		)
		m.setPendingJoins(chat.Id, user.Id)

		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.ContinueGroups
}

// joinRequestHandler processes admin responses to join request approval buttons.
// Handles accept, decline, and ban actions for pending chat join requests.
func (moduleStruct) joinRequestHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From
	chat := ctx.EffectiveChat
	msg := query.Message

	// permission checks
	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	response := args[1]
	joinUserId, _ := strconv.ParseInt(args[2], 10, 64)
	joinUser, err := b.GetChat(joinUserId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	var helpText string
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	switch response {
	case "accept":
		_, _ = b.ApproveChatJoinRequest(chat.Id, joinUser.Id, nil)
		helpText, _ = tr.GetString("greetings_join_request_accepted")
		_ = cache.Marshal.Delete(cache.Context, fmt.Sprintf("alita:pendingJoins:%d:%d", chat.Id, joinUser.Id))
	case "decline":
		_, _ = b.DeclineChatJoinRequest(chat.Id, joinUser.Id, nil)
		helpText, _ = tr.GetString("greetings_join_request_declined")
	case "ban":
		_, _ = chat.BanMember(b, joinUser.Id, nil)
		_, _ = b.DeclineChatJoinRequest(chat.Id, joinUser.Id, nil)
		helpText, _ = tr.GetString("greetings_join_request_banned")
	}

	_, _, err = msg.EditText(b,
		fmt.Sprintf(helpText, helpers.MentionHtml(joinUser.Id, joinUser.FirstName)),
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
		},
	)
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

// autoApprove toggles automatic approval of chat join requests.
// Admins can enable/disable auto-approval or check current setting for new join requests.
func (moduleStruct) autoApprove(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]
	var err error
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	// check permission
	if !chat_status.CanUserChangeInfo(bot, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		delPref := db.GetGreetingSettings(chat.Id).ShouldAutoApprove
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if delPref {
			text, _ := tr.GetString("greetings_auto_approve_enabled")
			_, err = msg.Reply(bot, text, helpers.Smarkdown())
		} else {
			text, _ := tr.GetString("greetings_auto_approve_disabled")
			_, err = msg.Reply(bot, text, helpers.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	switch strings.ToLower(args[0]) {
	case "off", "no":
		db.SetShouldAutoApprove(chat.Id, false)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_auto_approve_disable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	case "on", "yes":
		db.SetShouldAutoApprove(chat.Id, true)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_auto_approve_enable")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	default:
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("greetings_auto_approve_invalid_option")
		_, err = msg.Reply(bot, text, helpers.Shtml())
	}

	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// loadPendingJoins checks if a join request notification has already been sent for a user.
// Prevents duplicate join request messages by checking cache for recent requests.
func (moduleStruct) loadPendingJoins(chatId, userId int64) bool {
	if cache.Marshal == nil {
		return false
	}
	alreadyAsked, err := cache.Marshal.Get(cache.Context, fmt.Sprintf("alita:pendingJoins:%d:%d", chatId, userId), new(bool))
	if err != nil || alreadyAsked == nil {
		return false
	}
	// Safe type assertion
	if boolVal, ok := alreadyAsked.(*bool); ok && boolVal != nil {
		return *boolVal
	}
	return false
}

// setPendingJoins marks a join request as processed in cache with expiration.
// Stores request info for 5 minutes to prevent duplicate approval notifications.
func (moduleStruct) setPendingJoins(chatId, userId int64) {
	if cache.Marshal == nil {
		return
	}
	_ = cache.Marshal.Set(cache.Context, fmt.Sprintf("alita:pendingJoins:%d:%d", chatId, userId), true, store.WithExpiration(5*time.Minute))
}

// LoadGreetings registers all greeting-related handlers with the dispatcher.
// Sets up welcome/goodbye messages, join requests, and service message cleanup.
func LoadGreetings(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(greetingsModule.moduleName, true)

	// Adds Formatting kb button to Greetings Menu
	HelpModule.helpableKb[greetingsModule.moduleName] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         func() string { tr := i18n.MustNewTranslator("en"); t, _ := tr.GetString("button_formatting"); return t }(),
				CallbackData: fmt.Sprintf("helpq.%s", "Formatting"),
			},
		},
	}

	// this is used when user join, and creates a join request
	dispatcher.AddHandler(
		handlers.NewChatJoinRequest(
			chatjoinrequest.All, greetingsModule.pendingJoins,
		),
	)

	// this is for chat member joined the chat
	dispatcher.AddHandler(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				wasMember, isMember := helpers.ExtractJoinLeftStatusChange(u)
				return !wasMember && isMember
			},
			greetingsModule.newMember,
		),
	)

	// this is for chat member left the chat
	dispatcher.AddHandler(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				wasMember, isMember := helpers.ExtractJoinLeftStatusChange(u)
				return wasMember && !isMember
			},
			greetingsModule.leftMember,
		),
	)

	// for cleaning service messages
	dispatcher.AddHandler(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return msg.LeftChatMember != nil || msg.NewChatMembers != nil
			},
			greetingsModule.cleanService,
		),
	)

	dispatcher.AddHandler(handlers.NewCommand("welcome", greetingsModule.welcome))
	dispatcher.AddHandler(handlers.NewCommand("setwelcome", greetingsModule.setWelcome))
	dispatcher.AddHandler(handlers.NewCommand("resetwelcome", greetingsModule.resetWelcome))
	dispatcher.AddHandler(handlers.NewCommand("goodbye", greetingsModule.goodbye))
	dispatcher.AddHandler(handlers.NewCommand("setgoodbye", greetingsModule.setGoodbye))
	dispatcher.AddHandler(handlers.NewCommand("resetgoodbye", greetingsModule.resetGoodbye))
	dispatcher.AddHandler(handlers.NewCommand("cleanwelcome", greetingsModule.cleanWelcome))
	dispatcher.AddHandler(handlers.NewCommand("cleangoodbye", greetingsModule.cleanGoodbye))
	dispatcher.AddHandler(handlers.NewCommand("cleanservice", greetingsModule.delJoined))
	dispatcher.AddHandler(handlers.NewCommand("autoapprove", greetingsModule.autoApprove))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("join_request."), greetingsModule.joinRequestHandler))
}
