package modules

import (
	"fmt"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var disablingModule = moduleStruct{moduleName: "Disabling"}

/*
	To disable a command

# Connection - true, true

Only Admin can use this command to disable usage of a command in the chat
*/
// disable disables one or more bot commands in the current chat.
// Only admins can use this command. Accepts multiple command names as arguments.
func (moduleStruct) disable(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if len(args) >= 1 {
		toDisable := make([]string, 0)

		for _, i := range args {
			i = strings.ToLower(i)
			if string_handling.FindInStringSlice(misc.DisableCmds, i) {
				toDisable = append(toDisable, i)
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				temp, _ := tr.GetString("disabling_disabled_successfully")
				text := fmt.Sprintf(temp, strings.Join(toDisable, "\n - "))
				_, err := msg.Reply(b, text, helpers.Smarkdown())
				if err != nil {
					log.Error(err)
					return err
				}
			} else {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				temp, _ := tr.GetString("disabling_unknown_command")
				text := fmt.Sprintf(temp, i)
				_, err := msg.Reply(b, text, nil)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		}
		// finally disable all cmds
		for _, i := range toDisable {
			db.DisableCMD(chat.Id, i)
		}

	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("disabling_no_command_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

/*
	To check the disableable commands

Anyone can use this command to check the disableable commands
*/
// disableable shows a list of all commands that can be disabled in the chat.
// Any user can view this list to see which commands support disabling functionality.
func (moduleStruct) disableable(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("disabling_disableable_commands")
	var sb strings.Builder
	for _, cmds := range misc.DisableCmds {
		sb.WriteString(fmt.Sprintf("\n - `%s`", cmds))
	}
	text += sb.String()

	_, err := msg.Reply(b, text, helpers.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/*
	To list all disabled commands in chat

# Connection - false, true

Any user in can use this command to check the disabled commands in the current chat.
*/
// disabled displays all currently disabled commands in the chat.
// Any user can view the list of disabled commands for the current chat.
func (moduleStruct) disabled(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "adminlist") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	var replyMsgId int64

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	disabled := db.GetChatDisabledCMDs(chat.Id)

	if len(disabled) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("disabling_no_disabled_commands")
		_, err := msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("disabling_disabled_commands")
		slices.Sort(disabled)
		var sb strings.Builder
		for _, cmds := range disabled {
			sb.WriteString(fmt.Sprintf("\n - `%s`", cmds))
		}
		text += sb.String()
		_, err := msg.Reply(b, text, helpers.Smarkdown())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

/*
	To either delete or not to delete the disabled command in the chat

# Connection - true, true

Only admins can use this command to either choose to delete the disabled command
or not to. If no argument is given, the current chat setting is returned
*/
// disabledel toggles whether disabled commands should be automatically deleted.
// Only admins can use this. With no args, shows current setting; with args, changes it.
func (moduleStruct) disabledel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	var text string

	if len(args) >= 1 {
		param := strings.ToLower(args[0])
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		switch param {
		case "on", "true", "yes":
			go func() {
				defer error_handling.RecoverFromPanic("ToggleDel", "disabling")
				db.ToggleDel(chat.Id, true)
			}()
			text, _ = tr.GetString("disabling_delete_enabled")
		case "off", "false", "no":
			go func() {
				defer error_handling.RecoverFromPanic("ToggleDel", "disabling")
				db.ToggleDel(chat.Id, false)
			}()
			text, _ = tr.GetString("disabling_delete_disabled")
		default:
			text, _ = tr.GetString("disabling_invalid_option")
		}
	} else {
		currStatus := db.ShouldDel(chat.Id)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if currStatus {
			text, _ = tr.GetString("disabling_delete_current_enabled")
		} else {
			text, _ = tr.GetString("disabling_delete_current_disabled")
		}
	}
	_, err := msg.Reply(b, text, helpers.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/*
	To re-enable a command

# Connection - true, true

Only Admin can use this command to re-enable usage of a disabled command in the chat
*/
// enable re-enables one or more previously disabled bot commands in the chat.
// Only admins can use this command. Accepts multiple command names as arguments.
func (moduleStruct) enable(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if len(args) >= 1 {
		toEnable := make([]string, 0)

		for _, i := range args {
			i = strings.ToLower(i)
			if string_handling.FindInStringSlice(misc.DisableCmds, i) {
				toEnable = append(toEnable, i)
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				temp, _ := tr.GetString("disabling_enabled_successfully")
				text := fmt.Sprintf(temp, strings.Join(toEnable, "\n - "))
				_, err := msg.Reply(b, text, helpers.Smarkdown())
				if err != nil {
					log.Error(err)
					return err
				}
			} else {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				temp, _ := tr.GetString("disabling_unknown_reenable")
				text := fmt.Sprintf(temp, i)
				_, err := msg.Reply(b, text, nil)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		}
		// finally disable all cmds
		for _, i := range toEnable {
			db.EnableCMD(chat.Id, i)
		}

	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("disabling_no_command_reenable")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// LoadDisabling registers all disabling-related command handlers with the dispatcher.
// Sets up commands for managing which bot commands are enabled or disabled in chats.
func LoadDisabling(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(disablingModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("disable", disablingModule.disable))
	dispatcher.AddHandler(handlers.NewCommand("disableable", disablingModule.disableable))
	dispatcher.AddHandler(handlers.NewCommand("disabled", disablingModule.disabled))
	misc.AddCmdToDisableable("disabled")
	dispatcher.AddHandler(handlers.NewCommand("disabledel", disablingModule.disabledel))
	dispatcher.AddHandler(handlers.NewCommand("enable", disablingModule.enable))
}
