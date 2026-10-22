package modules

import (
	"html"
	"regexp"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// commandHelpModules maps each command that answers a wrong call with its own
// help to the module whose <module>_help_msg documents it. The warn commands
// (/warn, /swarn, /rmwarn, /resetwarns, /warns) do so only from PM: in a group
// they answer a call without a user with common_no_user_specified alone, as
// before PM support.
var commandHelpModules = map[string]string{
	"ban":        "bans",
	"sban":       "bans",
	"tban":       "bans",
	"dban":       "bans",
	"unban":      "bans",
	"kick":       "bans",
	"dkick":      "bans",
	"skick":      "bans",
	"restrict":   "bans",
	"unrestrict": "bans",
	"mute":       "mutes",
	"smute":      "mutes",
	"tmute":      "mutes",
	"dmute":      "mutes",
	"unmute":     "mutes",
	"digest":     "digest",
	"warn":       "warns",
	"swarn":      "warns",
	"rmwarn":     "warns",
	"resetwarns": "warns",
	"warns":      "warns",
	"warnings":   "warns",
	"rules":      "rules",
	"notes":      "notes",
	"saved":      "notes",
	"filters":    "filters",
	"connect":    "connections",
	"setlog":     "logchannels",
	"unsetlog":   "logchannels",
	"logchannel": "logchannels",
	"log":        "logchannels",
	"nolog":      "logchannels",

	"locks":            "locks",
	"locktypes":        "locks",
	"lock":             "locks",
	"unlock":           "locks",
	"flood":            "antiflood",
	"setflood":         "antiflood",
	"setfloodmode":     "antiflood",
	"delflood":         "antiflood",
	"disabled":         "disabling",
	"disable":          "disabling",
	"enable":           "disabling",
	"disabledel":       "disabling",
	"reactions":        "reactions",
	"addreaction":      "reactions",
	"removereaction":   "reactions",
	"resetreactions":   "reactions",
	"antiraid":         "antiraid",
	"raidtime":         "antiraid",
	"raidactiontime":   "antiraid",
	"autoantiraid":     "antiraid",
	"antichannelpin":   "pins",
	"cleanlinked":      "pins",
	"allowconnect":     "connections",
	"setrules":         "rules",
	"resetrules":       "rules",
	"privaterules":     "rules",
	"rulesbtn":         "rules",
	"resetrulesbutton": "rules",
	"blacklists":       "blacklists",
	"addblacklist":     "blacklists",
	"rmblacklist":      "blacklists",
	"blaction":         "blacklists",
	"approved":         "approvals",
	"approve":          "approvals",
	"unapprove":        "approvals",
	"approval":         "approvals",
	// WrapCommand settings commands, with the group from PM.
	"welcome":            "greetings",
	"setwelcome":         "greetings",
	"resetwelcome":       "greetings",
	"goodbye":            "greetings",
	"setgoodbye":         "greetings",
	"resetgoodbye":       "greetings",
	"cleanwelcome":       "greetings",
	"cleangoodbye":       "greetings",
	"cleanservice":       "greetings",
	"autoapprove":        "greetings",
	"captcha":            "captcha",
	"captchamode":        "captcha",
	"captchatime":        "captcha",
	"captchaaction":      "captcha",
	"captchamaxattempts": "captcha",
	"captchapending":     "captcha",
	"captchaclear":       "captcha",
	"setwarnlimit":       "warns",
	"setwarnmode":        "warns",
	"setwarntime":        "warns",
	"aispam":             "aispam",
	"spamdb":             "spamdb",
	"anonadmin":          "admin",
	// Group resets, with the group from PM.
	"privnote":      "notes",
	"clearall":      "notes",
	"stopall":       "filters",
	"remallbl":      "blacklists",
	"unapproveall":  "approvals",
	"resetallwarns": "warns",
	"unbanall":      "bans",
	// Group-only admin commands, with the group from PM.
	"adminlist":       "admin",
	"invitelink":      "admin",
	"setgtitle":       "admin",
	"setgdesc":        "admin",
	"admincache":      "admin",
	"clearadmincache": "admin",
}

// helpCommandLine matches a help line that defines commands: an optional
// bullet followed by one or more /commands ("× /fban /unfban: ..."). Commands
// mentioned inside prose ("use the /id command") are cross-references.
var (
	helpCommandLine = regexp.MustCompile(`(?m)^\s*(?:[×•-]>?\s*)?((?:/[a-z][a-z_]*\s*)+)`)
	helpCommandName = regexp.MustCompile(`/([a-z][a-z_]*)`)
)

// commandHelpBlock returns the entry for cmd in a module's help: the line that
// defines it and the unmarked lines that continue it, up to a blank line, a
// heading, or the next entry. It returns "" when the help has no such entry.
func commandHelpBlock(helpMsg, cmd string) string {
	lines := strings.Split(helpMsg, "\n")
	for i, line := range lines {
		if !definesCommand(line, cmd) {
			continue
		}
		block := []string{strings.TrimSpace(line)}
		for _, next := range lines[i+1:] {
			next = strings.TrimSpace(next)
			if next == "" || startsHelpEntry(next) {
				break
			}
			block = append(block, next)
		}
		return strings.Join(block, "\n")
	}
	return ""
}

func definesCommand(line, cmd string) bool {
	m := helpCommandLine.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	for _, name := range helpCommandName.FindAllStringSubmatch(m[1], -1) {
		if name[1] == cmd {
			return true
		}
	}
	return false
}

func startsHelpEntry(line string) bool {
	return helpCommandLine.MatchString(line) || strings.HasPrefix(line, "*") ||
		strings.HasPrefix(line, "×") || strings.HasPrefix(line, "•") || strings.HasPrefix(line, "- ")
}

// commandExample returns the sample call for cmd from locales/config.yml, or ""
// when the command has none.
func commandExample(cmd string) string {
	text, _ := i18n.MustNewTranslator("config").GetString(cmd + "_example")
	return text
}

// commandPMExample returns the sample call for cmd shown in PM help without a
// group: <cmd>_pm_example when the group's call (commandExample) names no
// group, else commandExample.
func commandPMExample(cmd string) string {
	if text, _ := i18n.MustNewTranslator("config").GetString(cmd + "_pm_example"); text != "" {
		return text
	}
	return commandExample(cmd)
}

// commandHelpTextWith puts errText (HTML) above the help for cmd: its own
// entry and example, the sample call, when the chat's language documents it,
// else the whole module help.
func commandHelpTextWith(tr *i18n.Translator, cmd, errText, example string) string {
	module, ok := commandHelpModules[cmd]
	if !ok {
		return errText
	}
	helpMsg, _ := tr.GetString(module + "_help_msg")
	if helpMsg == "" {
		return errText
	}
	block := commandHelpBlock(helpMsg, cmd)
	if block == "" {
		header, _ := tr.GetString("helpers_module_help_header")
		return errText + "\n\n" + renderModuleHelp(header, strings.ToUpper(module[:1])+module[1:], helpMsg)
	}
	text := errText + "\n\n" + formatting.ToTelegramHTML(block)
	if example != "" {
		line, _ := tr.GetString("command_help_example", i18n.TranslationParams{
			"example": "<code>" + html.EscapeString(example) + "</code>",
		})
		if line != "" {
			text += "\n\n" + line
		}
	}
	return text
}

// replyCommandHelp answers a call that cmd cannot run as given with errText
// (HTML) and the help that shows how to call it: from PM, with the PM example
// (commandPMExample).
func replyCommandHelp(b *gotgbot.Bot, msg *gotgbot.Message, tr *i18n.Translator, cmd, errText string) error {
	example := commandExample(cmd)
	if inPM(msg) {
		example = commandPMExample(cmd)
	}
	_, err := msg.Reply(b, commandHelpTextWith(tr, cmd, errText, example), formatting.Shtml())
	return err
}
