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
// help to the module whose <module>_help_msg documents it.
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

// commandHelpText puts errText (HTML) above the help for cmd: its own entry
// and example when the chat's language documents it, else the whole module
// help.
func commandHelpText(tr *i18n.Translator, cmd, errText string) string {
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
	if example := commandExample(cmd); example != "" {
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
// (HTML) and the help that shows how to call it.
func replyCommandHelp(b *gotgbot.Bot, msg *gotgbot.Message, tr *i18n.Translator, cmd, errText string) error {
	_, err := msg.Reply(b, commandHelpText(tr, cmd, errText), formatting.Shtml())
	return err
}
