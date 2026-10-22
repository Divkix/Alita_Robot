package modules

import (
	"errors"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// timezoneDesc is /timezone: the chat's IANA timezone, which features acting
// on the chat's local time read through chattz.Location. It is a chat
// setting like the title, so every form needs the change-info right. From PM
// it takes the group as its first argument or from the connection.
var timezoneDesc = helpers.CommandDescriptor{
	Name:       "timezone",
	PMGroupArg: helpers.PMGroupArgOwn,
	RequiredChecks: []helpers.CheckFunc{
		groupOnlyChatCheck,
		helpers.RequireGroup(),
		helpers.RequireUserAdmin(),
		helpers.CanUserChangeInfo(),
	},
}

// timezoneNow is the clock the replies show the local time from. Tests pin
// it; production never writes it after startup.
var timezoneNow = time.Now

// timezone handles /timezone. Without an argument it shows the chat's
// timezone; with one it stores that IANA name.
func (moduleStruct) timezone(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		name, err := chattz.Name(c.Chat.Id)
		if err != nil {
			log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
				Error("[Timezone] could not read the chat's timezone")
			return timezoneReply(c, "error_generic")
		}
		return timezoneReply(c, "timezone_status", timezoneParams(name))
	}

	input := strings.Join(args, " ")
	stored, err := chattz.Set(c.Chat.Id, input)
	if errors.Is(err, chattz.ErrUnknownTimezone) {
		text, _ := c.Tr.GetString("timezone_invalid", i18n.TranslationParams{"input": html.EscapeString(input)})
		if replyErr := replyCommandHelp(c.Bot, c.Msg, c.Tr, timezoneDesc.Name, text); replyErr != nil {
			log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": replyErr}).
				Warn("[Timezone] could not send the invalid-name reply")
		}
		return ext.EndGroups
	}
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Timezone] could not save the chat's timezone")
		return timezoneReply(c, "common_settings_save_failed")
	}
	return timezoneReply(c, "timezone_set", timezoneParams(stored))
}

// timezoneParams fills the replies: the name, its UTC offset now and the
// chat's local time now.
func timezoneParams(name string) i18n.TranslationParams {
	loc, err := chattz.Load(name)
	if err != nil {
		loc = time.UTC
	}
	now := timezoneNow().In(loc)
	return i18n.TranslationParams{
		"timezone": html.EscapeString(name),
		"offset":   now.Format("-07:00"),
		"time":     now.Format("15:04"),
	}
}

// timezoneReply answers in the chat the command came from; from PM the reply
// also names the group it acted on.
func timezoneReply(c *helpers.CommandContext, key string, params ...i18n.TranslationParams) error {
	text, _ := c.Tr.GetString(key, params...)
	if _, err := c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text), formatting.Shtml()); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Warn("[Timezone] could not reply")
		return err
	}
	return ext.EndGroups
}

// loadTimezone registers /timezone as part of the Admin module (LoadAdmin);
// its anonymous-admin handler is registered with the other chat-info
// commands in admin.go.
func loadTimezone(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, timezoneDesc, adminModule.timezone)
}
