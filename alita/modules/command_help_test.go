//go:build testtools

package modules

import (
	"os"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// useRealLocales serves every file in locales/, config.yml included, for the
// rest of the test.
func useRealLocales(t *testing.T) {
	t.Helper()
	locales := map[string]string{}
	for _, code := range append([]string{"config"}, localeCodes...) {
		raw, err := os.ReadFile("../../locales/" + code + ".yml")
		if err != nil {
			t.Fatalf("read %s.yml: %v", code, err)
		}
		locales[code] = string(raw)
	}
	restore, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restore)
}

const syntheticCommandHelp = `Intro text that mentions /ban in prose.

*Ban Commands:*
× /ban <userhandle>: bans a user.
× /banme: bans yourself.
× /tban <userhandle> x(m/h/d): bans a user for x time.
  m = minutes, h = hours.

× /fban /unfban: federation bans.
- /unban: unbans a user.`

func TestCommandHelpBlockCutsOnlyThatCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{cmd: "ban", want: "× /ban <userhandle>: bans a user."},
		{cmd: "banme", want: "× /banme: bans yourself."},
		{cmd: "tban", want: "× /tban <userhandle> x(m/h/d): bans a user for x time.\nm = minutes, h = hours."},
		{cmd: "fban", want: "× /fban /unfban: federation bans."},
		{cmd: "unfban", want: "× /fban /unfban: federation bans."},
		{cmd: "unban", want: "- /unban: unbans a user."},
		{cmd: "kick", want: ""},
		{cmd: "ba", want: ""},
	}
	for _, tt := range tests {
		if got := commandHelpBlock(syntheticCommandHelp, tt.cmd); got != tt.want {
			t.Errorf("commandHelpBlock(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}

// Every command whose wrong call answers with its own help must find its line
// in the module help of every locale, or users of that locale get the whole
// module help instead.
func TestEveryHelpCommandHasItsLineInEveryLocale(t *testing.T) {
	en := readLocaleFile(t, "en")
	for _, code := range localeCodes {
		locale := readLocaleFile(t, code)
		for cmd, module := range commandHelpModules {
			if code == "en" && !extendListsCommand(en[module+"_extend_msg"], cmd) {
				t.Errorf("en.yml: %s_extend_msg does not list /%s", module, cmd)
			}
			block := commandHelpBlock(locale[module+"_help_msg"], cmd)
			if block == "" {
				t.Errorf("%s.yml: %s_help_msg has no line for /%s", code, module, cmd)
				continue
			}
			if strings.Contains(block, `"`) {
				t.Errorf("%s.yml: /%s line %q carries a stray quote", code, cmd, block)
			}
			if bad := unknownTags(formatting.ToTelegramHTML(block)); len(bad) > 0 {
				t.Errorf("%s.yml: /%s line renders to HTML with %q", code, cmd, bad)
			}
		}
	}
}

// replyOnlyCommands act on the replied message, so an example without the
// reply would mislead.
var replyOnlyCommands = map[string]bool{"dkick": true, "dban": true, "dmute": true}

func TestEveryHelpCommandHasAnExample(t *testing.T) {
	useRealLocales(t)
	for cmd := range commandHelpModules {
		example := commandExample(cmd)
		if replyOnlyCommands[cmd] {
			if example != "" {
				t.Errorf("/%s acts on a reply but has example %q", cmd, example)
			}
			continue
		}
		if !strings.HasPrefix(example, "/"+cmd+" ") {
			t.Errorf("commandExample(%q) = %q, want it to start with %q", cmd, example, "/"+cmd+" ")
		}
	}
}

func extendListsCommand(extendMsg, cmd string) bool {
	for line := range strings.SplitSeq(extendMsg, "\n") {
		name, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		if name == "/"+cmd {
			return true
		}
	}
	return false
}

// extendCommands returns the commands each en.yml <module>_extend_msg lists,
// by module.
func extendCommands(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for key, extend := range readLocaleFile(t, "en") {
		module, ok := strings.CutSuffix(key, "_extend_msg")
		if !ok {
			continue
		}
		for line := range strings.SplitSeq(strings.TrimSpace(extend), "\n") {
			if cmd, _, found := strings.Cut(line, " — "); found && strings.HasPrefix(cmd, "/") {
				out[module] = append(out[module], strings.TrimPrefix(cmd, "/"))
			}
		}
	}
	return out
}

// The menu map and the extend lists name the same commands under the same
// modules, so a command added to one is added to the other.
func TestMenuCommandsMatchExtendLists(t *testing.T) {
	listed := map[string]bool{}
	for module, commands := range extendCommands(t) {
		for _, cmd := range commands {
			listed[cmd] = true
			entry, ok := menuCommands[cmd]
			if !ok {
				t.Errorf("/%s is in en.yml %s_extend_msg but not in menuCommands", cmd, module)
				continue
			}
			if entry.Module != module {
				t.Errorf("menuCommands puts /%s in %q; en.yml lists it in %s_extend_msg", cmd, entry.Module, module)
			}
		}
	}
	for cmd, entry := range menuCommands {
		if !listed[cmd] {
			t.Errorf("menuCommands has /%s, but en.yml %s_extend_msg does not list it", cmd, entry.Module)
		}
	}
}

func TestMenuCommandsAgreeWithCommandHelpModules(t *testing.T) {
	for cmd, module := range commandHelpModules {
		if entry := menuCommands[cmd]; entry.Module != module {
			t.Errorf("commandHelpModules puts /%s in %q, menuCommands in %q", cmd, module, entry.Module)
		}
	}
}

// Every section of the menu has a one-line description in every locale.
func TestEveryMenuSectionHasADescriptionInEveryLocale(t *testing.T) {
	modules := map[string]bool{}
	for _, entry := range menuCommands {
		modules[entry.Module] = true
	}
	for _, code := range localeCodes {
		locale := readLocaleFile(t, code)
		for module := range modules {
			desc := strings.TrimSpace(locale[module+"_section_desc"])
			if desc == "" {
				t.Errorf("%s.yml: %s_section_desc is missing", code, module)
				continue
			}
			if strings.Contains(desc, "\n") {
				t.Errorf("%s.yml: %s_section_desc spans several lines", code, module)
			}
			if bad := unknownTags(desc); len(bad) > 0 {
				t.Errorf("%s.yml: %s_section_desc has %q", code, module, bad)
			}
		}
	}
}
