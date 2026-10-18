//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gopkg.in/yaml.v3"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

const extendTestLocale = `
help_extend_header: "<b>All commands</b>"
help_extend_pm: "Open the full command list in PM."
help_extend_button: "Command list"
admin_extend_msg: |
  /promote — make a user admin
  /demote — remove admin rights
bans_extend_msg: |
  /ban — ban a user
`

func withExtendRegistry(t *testing.T, modules ...string) {
	t.Helper()
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	for _, m := range modules {
		defaultHelpRegistry.AbleMap[m] = true
	}
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
}

func withExtendLocale(t *testing.T, yamlContent string) {
	t.Helper()
	restore, err := i18n.OverrideManagerForTest(yamlContent)
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restore)
}

func sentTexts(client *moduleBotClient) []string {
	var texts []string
	for _, call := range client.callsFor("sendMessage") {
		texts = append(texts, fmt.Sprint(call.Params["text"]))
	}
	return texts
}

func TestExtendInPrivateListsEveryEnabledModuleWithItsCommands(t *testing.T) {
	withExtendLocale(t, extendTestLocale)
	withExtendRegistry(t, "Admin", "Bans", "Formatting")

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4401, FirstName: "Reader"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Reader"}

	if err := DefaultHelpRegistry().extend(bot, newModuleMessageContext(bot, chat, user, "/extend")); err != ext.EndGroups {
		t.Fatalf("extend(private) error = %v, want EndGroups", err)
	}

	texts := sentTexts(client)
	if len(texts) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(texts))
	}
	got := texts[0]
	for _, want := range []string{"All commands", "<b>Admin</b>", "/promote — make a user admin", "/demote", "<b>Bans</b>", "/ban — ban a user"} {
		if !strings.Contains(got, want) {
			t.Errorf("extend text missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Formatting") {
		t.Errorf("module without commands must be skipped:\n%s", got)
	}
	if strings.Index(got, "<b>Admin</b>") > strings.Index(got, "<b>Bans</b>") {
		t.Errorf("modules must be listed in help order:\n%s", got)
	}
	if mode := client.callsFor("sendMessage")[0].Params["parse_mode"]; mode != "HTML" {
		t.Errorf("parse_mode = %v, want HTML", mode)
	}
}

func TestExtendSplitsAtModuleBoundariesWithinTelegramLimit(t *testing.T) {
	var locale strings.Builder
	locale.WriteString(extendTestLocale)
	var modules []string
	for i := range 40 {
		name := fmt.Sprintf("Mod%02d", i)
		modules = append(modules, name)
		fmt.Fprintf(&locale, "%s_extend_msg: |\n", strings.ToLower(name))
		for j := range 6 {
			fmt.Fprintf(&locale, "  /cmd%02d%d — описание команды с кириллицей и эмодзи 🛡️\n", i, j)
		}
	}
	withExtendLocale(t, locale.String())
	withExtendRegistry(t, modules...)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4402, FirstName: "Reader"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Reader"}

	if err := DefaultHelpRegistry().extend(bot, newModuleMessageContext(bot, chat, user, "/extend")); err != ext.EndGroups {
		t.Fatalf("extend(private) error = %v, want EndGroups", err)
	}

	texts := sentTexts(client)
	if len(texts) < 2 {
		t.Fatalf("sendMessage calls = %d, want the list split into several messages", len(texts))
	}
	joined := strings.Join(texts, "\n")
	for _, text := range texts {
		if n := len(utf16.Encode([]rune(text))); n > 4096 {
			t.Errorf("message is %d UTF-16 units, over Telegram's 4096 limit", n)
		}
	}
	for i, m := range modules {
		header := "<b>" + m + "</b>"
		if strings.Count(joined, header) != 1 {
			t.Fatalf("module %s header appears %d times, want 1", m, strings.Count(joined, header))
		}
		for j := range 6 {
			cmd := fmt.Sprintf("/cmd%02d%d", i, j)
			holder := slices.IndexFunc(texts, func(s string) bool { return strings.Contains(s, cmd) })
			if holder < 0 || !strings.Contains(texts[holder], header) {
				t.Fatalf("command %s is not in the same message as its module header", cmd)
			}
		}
	}
}

func TestExtendInGroupPointsToPrivateChat(t *testing.T) {
	withExtendLocale(t, extendTestLocale)
	withExtendRegistry(t, "Admin")

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4403, FirstName: "Reader"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Extend Chat"}

	if err := DefaultHelpRegistry().extend(bot, newModuleMessageContext(bot, group, user, "/extend")); err != ext.EndGroups {
		t.Fatalf("extend(group) error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if text := fmt.Sprint(calls[0].Params["text"]); strings.Contains(text, "/promote") {
		t.Fatalf("group reply must not dump the command list: %q", text)
	}
	markup := fmt.Sprintf("%v", calls[0].Params["reply_markup"])
	if !strings.Contains(markup, "https://t.me/AlitaTestBot?start=extend") {
		t.Fatalf("group reply markup = %s, want PM deep link", markup)
	}
}

func TestStartExtendDeepLinkSendsCommandList(t *testing.T) {
	withExtendLocale(t, extendTestLocale)
	withExtendRegistry(t, "Admin")

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4404, FirstName: "Reader"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Reader"}

	if err := DefaultHelpRegistry().start(bot, newModuleMessageContext(bot, chat, user, "/start extend")); err != ext.EndGroups {
		t.Fatalf("start(extend) error = %v, want EndGroups", err)
	}
	texts := sentTexts(client)
	if len(texts) != 1 || !strings.Contains(texts[0], "/promote") {
		t.Fatalf("deep link replies = %q, want the command list", texts)
	}
}

// staleHelpCommands are mentioned in a module's help text but are not registered
// under that name, so /extend lists the real command instead.
var staleHelpCommands = map[string]bool{
	"captchaattempts": true, // registered as /captchamaxattempts
}

// helpOnlyModules have help text but no button in the /help menu: Devs is
// owner-only, Help and Users never call SetModuleEnabled.
var helpOnlyModules = map[string]bool{"devs": true, "help": true, "users": true}

// Every module whose help documents commands must have a one-line-per-command
// summary in en and ru, and the summary must cover every command the help names.
func TestExtendLocalesCoverEveryDocumentedCommand(t *testing.T) {
	en := readLocaleFile(t, "en")
	ru := readLocaleFile(t, "ru")

	for key, value := range en {
		module, ok := strings.CutSuffix(key, "_help_msg")
		if !ok || helpOnlyModules[module] {
			continue
		}
		var documented []string
		for _, line := range helpCommandLine.FindAllStringSubmatch(value, -1) {
			for _, m := range helpCommandName.FindAllStringSubmatch(line[1], -1) {
				if !staleHelpCommands[m[1]] && !slices.Contains(documented, m[1]) {
					documented = append(documented, m[1])
				}
			}
		}
		if len(documented) == 0 {
			continue
		}
		for name, locale := range map[string]map[string]string{"en": en, "ru": ru} {
			summary := locale[module+"_extend_msg"]
			if summary == "" {
				t.Errorf("%s.yml: %s_extend_msg is missing", name, module)
				continue
			}
			for _, cmd := range documented {
				if !regexp.MustCompile(`(?m)/` + cmd + `\b`).MatchString(summary) {
					t.Errorf("%s.yml: %s_extend_msg does not list /%s", name, module, cmd)
				}
			}
			for line := range strings.SplitSeq(strings.TrimSpace(summary), "\n") {
				if !strings.HasPrefix(line, "/") || !strings.Contains(line, " — ") {
					t.Errorf("%s.yml: %s_extend_msg line %q is not \"/command — description\"", name, module, line)
				}
			}
		}
	}
}

func readLocaleFile(t *testing.T, code string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../../locales/" + code + ".yml")
	if err != nil {
		t.Fatalf("read %s.yml: %v", code, err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse %s.yml: %v", code, err)
	}
	out := make(map[string]string, len(parsed))
	for k, v := range parsed {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// failingSendClient fails the failAt-th sendMessage call (1-based) and records
// every attempt, so a test can see where the handler stopped.
type failingSendClient struct {
	*moduleBotClient
	failAt int
	sends  int
}

func (c *failingSendClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "sendMessage" {
		c.sends++
		if c.sends == c.failAt {
			c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts) //nolint:errcheck // record the attempt only
			return nil, errors.New("telegram: flood control")
		}
	}
	return c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
}

func TestExtendStopsAndReturnsErrorWhenAMiddlePartFailsToSend(t *testing.T) {
	var locale strings.Builder
	locale.WriteString(extendTestLocale)
	var modules []string
	for i := range 40 {
		name := fmt.Sprintf("Mod%02d", i)
		modules = append(modules, name)
		fmt.Fprintf(&locale, "%s_extend_msg: |\n", strings.ToLower(name))
		for j := range 6 {
			fmt.Fprintf(&locale, "  /cmd%02d%d — описание команды с кириллицей и эмодзи 🛡️\n", i, j)
		}
	}
	withExtendLocale(t, locale.String())
	withExtendRegistry(t, modules...)

	inner := newModuleBotClient()
	client := &failingSendClient{moduleBotClient: inner, failAt: 2}
	bot := newModuleTestBot(inner)
	bot.BotClient = client
	user := gotgbot.User{Id: 4405, FirstName: "Reader"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Reader"}

	err := DefaultHelpRegistry().extend(bot, newModuleMessageContext(bot, chat, user, "/extend"))
	if err == nil || errors.Is(err, ext.EndGroups) {
		t.Fatalf("extend error = %v, want the send failure", err)
	}
	if !strings.Contains(err.Error(), "flood control") {
		t.Fatalf("extend error = %v, want the send failure to be returned", err)
	}
	if got := len(inner.callsFor("sendMessage")); got != 2 {
		t.Fatalf("sendMessage attempts = %d, want 2 (stop at the failed part, send nothing after it)", got)
	}
}

func TestPackExtendBlocksSplitsAnOverlongLineWithoutBreakingSurrogatePairs(t *testing.T) {
	const limit = 4096
	// A leading ASCII char shifts every emoji (2 UTF-16 units each) onto an odd
	// offset, so a cut at exactly `limit` units would land inside a pair.
	line := "/cmd — a" + strings.Repeat("🛡", 3000)
	if utf16Len(line) <= limit {
		t.Fatalf("fixture is %d units, must exceed %d", utf16Len(line), limit)
	}

	parts := packExtendBlocks("", []string{line}, limit)

	if len(parts) < 2 {
		t.Fatalf("parts = %d, want the overlong line split", len(parts))
	}
	for i, p := range parts {
		if n := utf16Len(p); n > limit {
			t.Errorf("part %d is %d UTF-16 units, over %d", i, n, limit)
		}
		if !utf8.ValidString(p) || strings.ContainsRune(p, utf8.RuneError) {
			t.Errorf("part %d holds a broken rune", i)
		}
	}
	if got := strings.Join(parts, ""); got != line {
		t.Errorf("joined parts differ from the original line (len %d vs %d)", len(got), len(line))
	}
}
