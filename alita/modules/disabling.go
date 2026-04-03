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
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var disablingModule = moduleStruct{moduleName: "Disabling"}

/*
	To disable or enable commands

# Connection - true, true

Only Admin can use this command to disable/enable usage of a command in the chat
*/

// toggleCmdConfig holds configuration for disable/enable command operations.
type toggleCmdConfig struct {
	emptyArgsMsgKey  string
	noCmdMsgKey      string
	successMsgKey    string
	unknownCmdMsgKey string
	dbOp             func(int64, string) error
	actionName       string // for logging: "disable" or "enable"
}

// toggleCommands handles both disabling and enabling commands.
// Only admins can use this command. Accepts multiple command names as arguments.
func (moduleStruct) toggleCommands(enable bool) func(*gotgbot.Bot, *ext.Context) error {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		msg := ctx.EffectiveMessage
		// connection status
		connectedChat := helpers.IsUserConnected(b, ctx, true, true)
		if connectedChat == nil {
			return ext.EndGroups
		}
		ctx.EffectiveChat = connectedChat
		chat := ctx.EffectiveChat
		args := ctx.Args()[1:]
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

		var cfg toggleCmdConfig
		if enable {
			cfg = toggleCmdConfig{
				emptyArgsMsgKey:  "disabling_no_command_reenable",
				noCmdMsgKey:      "disabling_no_command_reenable",
				successMsgKey:    "disabling_enabled_successfully",
				unknownCmdMsgKey: "disabling_unknown_reenable",
				dbOp:             db.EnableCMD,
				actionName:       "enable",
			}
		} else {
			cfg = toggleCmdConfig{
				emptyArgsMsgKey:  "disabling_no_command_specified",
				noCmdMsgKey:      "disabling_no_command_specified",
				successMsgKey:    "disabling_disabled_successfully",
				unknownCmdMsgKey: "disabling_unknown_command",
				dbOp:             db.DisableCMD,
				actionName:       "disable",
			}
		}

		if len(args) == 0 {
			text, _ := tr.GetString(cfg.emptyArgsMsgKey)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}

		// Collect valid and invalid commands
		toToggle := make([]string, 0, len(args))
		unknownCmds := make([]string, 0, len(args))

		for _, cmd := range args {
			cmd = strings.ToLower(cmd)
			if slices.Contains(helpers.DisableCmds, cmd) {
				toToggle = append(toToggle, cmd)
			} else {
				unknownCmds = append(unknownCmds, cmd)
			}
		}

		// First, toggle all valid commands in the database
		failedCmds := make([]string, 0, len(toToggle))
		for _, cmd := range toToggle {
			if err := cfg.dbOp(chat.Id, cmd); err != nil {
				failedCmds = append(failedCmds, cmd)
				log.Errorf("[Disabling] Failed to %s command '%s' in chat %d: %v", cfg.actionName, cmd, chat.Id, err)
			}
		}

		// Remove failed commands from success list
		successCmds := make([]string, 0, len(toToggle))
		for _, cmd := range toToggle {
			if !slices.Contains(failedCmds, cmd) {
				successCmds = append(successCmds, cmd)
			}
		}

		// Send success message for successfully toggled commands
		if len(successCmds) > 0 {
			temp, _ := tr.GetString(cfg.successMsgKey)
			text := fmt.Sprintf(temp, "\n - "+strings.Join(successCmds, "\n - "))
			_, err := msg.Reply(b, text, helpers.Smarkdown())
			if err != nil {
				log.Error(err)
				return err
			}
		}

		// Send error message for unknown commands
		for _, cmd := range unknownCmds {
			temp, _ := tr.GetString(cfg.unknownCmdMsgKey)
			text := fmt.Sprintf(temp, cmd)
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}

		return ext.EndGroups
	}
}

// disable disables one or more bot commands in the current chat.
// Only admins can use this command. Accepts multiple command names as arguments.
func (m moduleStruct) disable(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.toggleCommands(false)(b, ctx)
}

// enable re-enables one or more previously disabled bot commands in the chat.
// Only admins can use this command. Accepts multiple command names as arguments.
func (m moduleStruct) enable(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.toggleCommands(true)(b, ctx)
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
	for _, cmds := range helpers.DisableCmds {
		fmt.Fprintf(&sb, "\n - `%s`", cmds)
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
	if chat_status.CheckDisabledCmd(b, msg, "disabled") {
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
			fmt.Fprintf(&sb, "\n - `%s`", cmds)
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
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var text string

	if len(args) >= 1 {
		param := strings.ToLower(args[0])
		switch param {
		case "on", "true", "yes":
			// Execute DB operation synchronously before sending confirmation
			if err := db.ToggleDel(chat.Id, true); err != nil {
				log.Errorf("[Disabling] Failed to enable delete mode for chat %d: %v", chat.Id, err)
				text = "Failed to update setting. Please try again."
			} else {
				text, _ = tr.GetString("disabling_delete_enabled")
			}
		case "off", "false", "no":
			// Execute DB operation synchronously before sending confirmation
			if err := db.ToggleDel(chat.Id, false); err != nil {
				log.Errorf("[Disabling] Failed to disable delete mode for chat %d: %v", chat.Id, err)
				text = "Failed to update setting. Please try again."
			} else {
				text, _ = tr.GetString("disabling_delete_disabled")
			}
		default:
			text, _ = tr.GetString("disabling_invalid_option")
		}
	} else {
		currStatus := db.ShouldDel(chat.Id)
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

// LoadDisabling registers all disabling-related command handlers with the dispatcher.
// Sets up commands for managing which bot commands are enabled or disabled in chats.
func LoadDisabling(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(disablingModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("disable", disablingModule.disable))
	dispatcher.AddHandler(handlers.NewCommand("disableable", disablingModule.disableable))
	dispatcher.AddHandler(handlers.NewCommand("disabled", disablingModule.disabled))
	helpers.AddCmdToDisableable("disabled")
	dispatcher.AddHandler(handlers.NewCommand("disabledel", disablingModule.disabledel))
	dispatcher.AddHandler(handlers.NewCommand("enable", disablingModule.enable))
}
