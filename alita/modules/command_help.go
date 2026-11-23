package modules

import (
	"html"
	"regexp"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// menuCommand is one command of the help menu: the module whose
// <module>_help_msg documents it, and whether only admins can use it.
type menuCommand struct {
	Module    string
	AdminOnly bool
}

// replyOnlyCommands act on the replied message, so an example without the
// reply would mislead; they are the only help commands without one.
var replyOnlyCommands = map[string]bool{"dban": true, "dkick": true, "dmute": true, "spam": true}

// commandHelpModules maps each command that answers a wrong call with its own
// help to the lowercase module that registered it. It is derived once from
// the command manifest WrapCommand fills at registration and the locale
// config.yml examples: a command belongs when locales/config.yml gives it a
// <cmd>_example — the sample call that makes the help actionable — or it is
// a replyOnlyCommands entry. Every such command needs a × /cmd line in all
// seven locales' <module>_help_msg, with continuation lines ending at a
// blank line or the next entry, no double quotes or unknown HTML tags, and
// /cmd in en.yml's <module>_extend_msg.
// TestEveryHelpCommandHasItsLineInEveryLocale enforces the locale contract.
// The warn commands (/warn, /swarn, /rmwarn, /resetwarns, /warns) answer
// with their help only from PM: in a group they answer a call without a
// user with common_no_user_specified alone, as before PM support.
//
// The map is built once and shared: callers must not mutate it.
var commandHelpModules = sync.OnceValue(func() map[string]string {
	out := make(map[string]string)
	for cmd, entry := range helpers.CommandManifest() {
		if commandExample(cmd) == "" && !replyOnlyCommands[cmd] {
			continue
		}
		out[cmd] = strings.ToLower(entry.Module)
	}
	return out
})

// menuCommands lists every command of every en.yml <module>_extend_msg, and
// nothing else. The module is the one the manifest says registered the
// command; AdminOnly comes from the descriptor's declared checks, so a
// command is member-visible only when some form of it works for a member:
// /warns without a user shows your own warns, /lang in PM sets your own
// language, /invitelink answers a member in a public group, and the
// federation commands are gated by federation roles, not group admin
// rights. Dev commands have no extend list, so they are not listed.
// Every listed module needs a one-line <module>_section_desc in all seven
// locales.
//
// The map is built once and shared: callers must not mutate it.
var menuCommands = sync.OnceValue(func() map[string]menuCommand {
	manifest := helpers.CommandManifest()
	en := i18n.MustNewTranslator("en")
	out := map[string]menuCommand{}
	seen := map[string]bool{}
	for _, entry := range manifest {
		module := strings.ToLower(entry.Module)
		if seen[module] {
			continue
		}
		seen[module] = true
		order, _ := extendLines(en, module)
		for _, cmd := range order {
			owner, ok := manifest[cmd]
			if !ok || strings.ToLower(owner.Module) != module {
				continue
			}
			out[cmd] = menuCommand{Module: module, AdminOnly: owner.AdminOnly}
		}
	}
	return out
})

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
// Each commandHelpModules command needs a <cmd>_example starting with "/<cmd> ",
// except the replyOnlyCommands /dban, /dkick, /dmute and /spam, which must have none.
// TestEveryHelpCommandHasAnExample enforces this contract.
func commandExample(cmd string) string {
	text, _ := i18n.MustNewTranslator("config").GetString(cmd + "_example")
	return text
}

// commandPMExample returns the sample call for cmd shown in PM help without a
// group. An optional <cmd>_pm_example names the group first and replaces
// commandExample; otherwise commandExample is used.
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
	module, ok := commandHelpModules()[cmd]
	if !ok {
		return errText
	}
	body := commandHelpBody(tr, module, cmd, example)
	if body == "" {
		return errText
	}
	return errText + "\n\n" + body
}

// commandHelpBody is the help for cmd (HTML) from module's help: its own entry
// and example when the chat's language documents it, else the whole module
// help. It is "" when the language has no help for module.
func commandHelpBody(tr *i18n.Translator, module, cmd, example string) string {
	helpMsg, _ := tr.GetString(module + "_help_msg")
	if helpMsg == "" {
		return ""
	}
	block := commandHelpBlock(helpMsg, cmd)
	if block == "" {
		header, _ := tr.GetString("helpers_module_help_header")
		return renderModuleHelp(header, strings.ToUpper(module[:1])+module[1:], helpMsg)
	}
	text := formatting.ToTelegramHTML(block)
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

// CommandModules maps each help-menu command to its module, the bounded
// command set metrics label updates with.
func CommandModules() map[string]string {
	menu := menuCommands()
	out := make(map[string]string, len(menu))
	for cmd, entry := range menu {
		out[cmd] = entry.Module
	}
	return out
}
