//go:build testtools

package modules

import (
	"fmt"
	"html"
	"os"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gopkg.in/yaml.v3"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// enHelpEntry returns the rendered en help line that documents cmd, found by
// its "× /cmd" prefix rather than by commandHelpBlock.
func enHelpEntry(t *testing.T, module, cmd string) string {
	t.Helper()
	for line := range strings.SplitSeq(readLocaleFile(t, "en")[module+"_help_msg"], "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "×" && strings.TrimSuffix(fields[1], ":") == "/"+cmd {
			return formatting.ToTelegramHTML(line)
		}
	}
	t.Fatalf("en %s_help_msg has no × /%s line", module, cmd)
	return ""
}

// onlyReply returns the one sendMessage the handler made, after checking it
// went to chatID as HTML.
func onlyReply(t *testing.T, client *moduleBotClient, chatID int64) string {
	t.Helper()
	call := client.onlyCall(t, "sendMessage")
	assertParams(t, call, map[string]any{"chat_id": chatID, "parse_mode": "HTML"})
	return fmt.Sprint(call.Params["text"])
}

// assertCommandHelpReply fails unless text gives the failure errText, then
// cmd's own help line and, for commands that take a target, its example.
func assertCommandHelpReply(t *testing.T, text, cmd, errText string) {
	t.Helper()
	module := commandHelpModules[cmd]
	entry := enHelpEntry(t, module, cmd)
	if !strings.HasPrefix(text, errText+"\n\n") {
		t.Errorf("/%s reply = %q, want it to start with the error %q", cmd, text, errText)
	}
	if !strings.Contains(text, entry) {
		t.Errorf("/%s reply = %q, want its help line %q", cmd, text, entry)
	}
	if example := readLocaleFile(t, "config")[cmd+"_example"]; example != "" {
		if want := "<code>" + html.EscapeString(example) + "</code>"; !strings.Contains(text, want) {
			t.Errorf("/%s reply = %q, want example %q", cmd, text, want)
		}
	}
	header := strings.SplitN(translated(t, "helpers_module_help_header"), "%s", 2)[0]
	if strings.Contains(text, formatting.ToTelegramHTML(header)) {
		t.Errorf("/%s reply = %q carries the whole module help", cmd, text)
	}
}

func TestModerationCommandsCalledWithoutTargetReplyWithTheirHelp(t *testing.T) {
	useRealLocales(t)
	tests := []struct {
		cmd    string
		text   string
		reply  bool
		errKey string
		run    func(moduleStruct) func(*gotgbot.Bot, *ext.Context) error
	}{
		{cmd: "ban", text: "/ban", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.ban }},
		{cmd: "ban", text: "/ban notauser", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.ban }},
		{cmd: "sban", text: "/sban", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.sBan }},
		{cmd: "tban", text: "/tban", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.tBan }},
		{cmd: "dban", text: "/dban 42", errKey: "bans_ban_dban_no_reply", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.dBan }},
		{cmd: "unban", text: "/unban", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.unban }},
		{cmd: "kick", text: "/kick", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.kick }},
		{cmd: "skick", text: "/skick", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.sKick }},
		{cmd: "dkick", text: "/dkick", errKey: "bans_dkick_no_reply", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.dkick }},
		{cmd: "restrict", text: "/restrict", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.restrict }},
		{cmd: "unrestrict", text: "/unrestrict", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.unrestrict }},
		{cmd: "mute", text: "/mute", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.mute }},
		{cmd: "smute", text: "/smute", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.sMute }},
		{cmd: "tmute", text: "/tmute", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.tMute }},
		{cmd: "dmute", text: "/dmute 42", errKey: "mute_reply_to_dmute", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.dMute }},
		{cmd: "unmute", text: "/unmute", errKey: "common_no_user_specified", run: func(m moduleStruct) func(*gotgbot.Bot, *ext.Context) error { return m.unmute }},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			f := newAdminChatFixture(t)
			module := bansModule
			if commandHelpModules[tt.cmd] == "mutes" {
				module = mutesModule
			}
			ctx := newModuleMessageContext(f.bot, f.chat, f.admin, tt.text)
			if err := tt.run(module)(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.text, err)
			}
			assertCommandHelpReply(t, onlyReply(t, f.client, f.chat.Id), tt.cmd, translated(t, tt.errKey))
		})
	}
}

func TestBanHelpDoesNotIncludeBanme(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := bansModule.ban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/ban")); err != ext.EndGroups {
		t.Fatalf("ban error = %v, want EndGroups", err)
	}
	if text := onlyReply(t, f.client, f.chat.Id); strings.Contains(text, enHelpEntry(t, "bans", "banme")) {
		t.Fatalf("/ban reply = %q, want only the /ban entry", text)
	}
}

func TestTimedCommandsWithoutTimeReplyWithTheirHelp(t *testing.T) {
	useRealLocales(t)
	for _, tt := range []struct {
		cmd string
		run func(*gotgbot.Bot, *ext.Context) error
	}{
		{cmd: "tban", run: bansModule.tBan},
		{cmd: "tmute", run: mutesModule.tMute},
	} {
		t.Run(tt.cmd, func(t *testing.T) {
			f := newAdminChatFixture(t)
			ctx := newBanReplyContext(f.bot, f.chat, f.admin, f.member, "/"+tt.cmd)
			if err := tt.run(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("/%s error = %v, want EndGroups", tt.cmd, err)
			}
			if calls := f.client.callsFor("banChatMember"); len(calls) != 0 {
				t.Fatalf("banChatMember calls = %d, want none without a time", len(calls))
			}
			if calls := f.client.callsFor("restrictChatMember"); len(calls) != 0 {
				t.Fatalf("restrictChatMember calls = %d, want none without a time", len(calls))
			}
			errText := html.EscapeString(translated(t, "extraction_no_time_specified"))
			assertCommandHelpReply(t, onlyReply(t, f.client, f.chat.Id), tt.cmd, errText)
		})
	}
}

// A malformed time keeps the extractor's own reply, which already names the
// accepted units.
func TestTimedCommandWithBadTimeKeepsTheTimeError(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	ctx := newBanReplyContext(f.bot, f.chat, f.admin, f.member, "/tban 5x")
	if err := bansModule.tBan(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("/tban 5x error = %v, want EndGroups", err)
	}
	text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
	if strings.Contains(text, enHelpEntry(t, "bans", "tban")) {
		t.Fatalf("/tban 5x reply = %q, want the time error alone", text)
	}
}

func TestCommandHelpFallsBackToModuleHelp(t *testing.T) {
	raw, err := os.ReadFile("../../locales/en.yml")
	if err != nil {
		t.Fatalf("read en.yml: %v", err)
	}
	var en map[string]any
	if err := yaml.Unmarshal(raw, &en); err != nil {
		t.Fatalf("parse en.yml: %v", err)
	}
	const helpWithoutBan = "Bans help that lists no commands."
	en["bans_help_msg"] = helpWithoutBan
	patched, err := yaml.Marshal(en)
	if err != nil {
		t.Fatalf("marshal en.yml: %v", err)
	}
	config, err := os.ReadFile("../../locales/config.yml")
	if err != nil {
		t.Fatalf("read config.yml: %v", err)
	}
	restore, err := i18n.OverrideLocalesForTest(map[string]string{"en": string(patched), "config": string(config)})
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restore)

	f := newAdminChatFixture(t)
	if err := bansModule.ban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/ban")); err != ext.EndGroups {
		t.Fatalf("ban error = %v, want EndGroups", err)
	}
	text := onlyReply(t, f.client, f.chat.Id)
	errText := translated(t, "common_no_user_specified")
	header := formatting.ToTelegramHTML(fmt.Sprintf(translated(t, "helpers_module_help_header"), "Bans"))
	if want := errText + "\n\n" + header + helpWithoutBan; text != want {
		t.Fatalf("/ban reply = %q, want the error and the whole module help %q", text, want)
	}
}

func TestAnonymousAdminBanWithoutTargetGetsHelp(t *testing.T) {
	useRealLocales(t)
	initBanDescs()
	f := newAdminChatFixture(t)
	handler := anonPipelineHandler(banDesc, bansModule.ban)
	if err := handler(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/ban")); err != ext.EndGroups {
		t.Fatalf("anonymous /ban error = %v, want EndGroups", err)
	}
	assertCommandHelpReply(t, onlyReply(t, f.client, f.chat.Id), "ban", translated(t, "common_no_user_specified"))
}
