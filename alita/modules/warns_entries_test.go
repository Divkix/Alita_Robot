//go:build testtools

package modules

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// warnsListText runs /warns for userID and returns the bot's last reply.
func warnsListText(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat, userID int64) string {
	t.Helper()
	viewer := gotgbot.User{Id: 43, FirstName: "Viewer"}
	ctx := newModuleMessageContext(bot, chat, viewer, fmt.Sprintf("/warns %d", userID))
	if err := warnsModule.warns(bot, ctx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) == 0 {
		t.Fatal("sendMessage calls = 0, want /warns reply")
	}
	return calls[len(calls)-1].Params["text"].(string)
}

// useEnglishLocale serves the real locales/en.yml for the rest of the test.
func useEnglishLocale(t *testing.T) {
	t.Helper()
	content, err := os.ReadFile("../../locales/en.yml")
	if err != nil {
		t.Fatalf("read en.yml: %v", err)
	}
	restore, err := i18n.OverrideManagerForTest(string(content))
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restore)
}

func translated(t *testing.T, key string) string {
	t.Helper()
	text, _ := i18n.MustNewTranslator("en").GetString(key)
	if text == "" {
		t.Fatalf("locale key %s is missing", key)
	}
	return text
}

func setUserName(t *testing.T, userID int64, name string) {
	t.Helper()
	if err := user.UpdateUser(userID, "", name); err != nil {
		t.Fatalf("UpdateUser(%d) error = %v", userID, err)
	}
	dbcache.DeleteCache(dbcache.CacheKey("user", userID))
}

// ensureWarnParents creates the chats and users rows that warns_users and
// warn_entries reference, for fixtures that insert those rows directly.
func ensureWarnParents(t *testing.T, chatID, userID int64) {
	t.Helper()
	if err := chats.EnsureChatInDb(chatID, "Warn Chat"); err != nil {
		t.Fatalf("EnsureChatInDb(%d) error = %v", chatID, err)
	}
	if err := user.EnsureUserInDb(userID, "", "Member"); err != nil {
		t.Fatalf("EnsureUserInDb(%d) error = %v", userID, err)
	}
}

func TestWarnsListShowsIssuerAndTime(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	client, bot, chat, issuer := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	setUserName(t, issuer.Id, "Mod Squad")

	if err := warnsModule.warnUser(bot, newWarnReplyContext(bot, chat, issuer, target, "/warn too noisy")); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}

	text := warnsListText(t, client, bot, chat, target.Id)
	if !strings.Contains(text, "too noisy") || !strings.Contains(text, "Mod Squad") {
		t.Fatalf("/warns text = %q, want reason and issuer name", text)
	}
	// Compare with the stored warned_at rather than time.Now(), which can fall
	// on the next day when the test runs across midnight UTC.
	_, entries := warns.GetWarns(target.Id, chat.Id)
	if len(entries) != 1 || entries[0].WarnedAt == nil {
		t.Fatalf("stored entries = %+v, want one with warned_at", entries)
	}
	if stamp := entries[0].WarnedAt.UTC().Format("2006-01-02 15:04 UTC"); !strings.Contains(text, stamp) {
		t.Fatalf("/warns text = %q, want warn time %s", text, stamp)
	}
}

func TestWarnsListKeepsLegacyReasonsPlain(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	ensureWarnParents(t, chat.Id, 42)
	if err := db.DB.Create(&models.Warns{
		UserId: 42, ChatId: chat.Id, NumWarns: 1, Reasons: models.StringArray{"old reason"},
	}).Error; err != nil {
		t.Fatalf("create legacy warns: %v", err)
	}

	text := warnsListText(t, client, bot, chat, 42)
	if !strings.HasSuffix(text, "\n - old reason") {
		t.Fatalf("/warns text = %q, want the legacy reason without author or time", text)
	}
}

func TestWarnByAnonymousAdminHidesIssuer(t *testing.T) {
	useEnglishLocale(t)
	anonymous := translated(t, "warns_issuer_anonymous")
	groupAnonymousBot := gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	t.Run("anonymous admins allowed", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Chat"}
		if err := admin.SetAnonAdminMode(chat.Id, true); err != nil {
			t.Fatalf("SetAnonAdminMode() error = %v", err)
		}
		ctx := newWarnReplyContext(bot, chat, groupAnonymousBot, target, "/warn anon spam")
		ctx.EffectiveMessage.SenderChat = &chat
		ctx.EffectiveSender = ctx.EffectiveMessage.GetSender()
		if !ctx.EffectiveSender.IsAnonymousAdmin() {
			t.Fatal("fixture is not an anonymous admin message")
		}
		if err := warnsModule.warnUser(bot, ctx); err != ext.EndGroups {
			t.Fatalf("warnUser() error = %v, want EndGroups", err)
		}

		text := warnsListText(t, client, bot, chat, target.Id)
		if !strings.Contains(text, "anon spam") || !strings.Contains(text, anonymous) {
			t.Fatalf("/warns text = %q, want reason marked %q", text, anonymous)
		}
	})

	t.Run("confirmed through the button", func(t *testing.T) {
		m := cache.GetMarshal()
		if m == nil {
			t.Skip("requires initialized cache marshaler")
		}
		f := newAdminChatFixture(t)
		client, bot, chat, clicker := f.client, f.bot, f.chat, f.admin
		setUserName(t, clicker.Id, "Hidden Mod")

		command := &gotgbot.Message{
			MessageId:  505,
			Date:       1,
			Chat:       chat,
			From:       &groupAnonymousBot,
			SenderChat: &chat,
			Text:       "/warn button spam",
			ReplyToMessage: &gotgbot.Message{
				MessageId: 504, Date: 1, Chat: chat, From: &target, Text: "spam",
			},
		}
		key := fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, command.MessageId)
		if err := m.Set(cache.Context, key, command); err != nil {
			t.Fatalf("cache set: %v", err)
		}
		t.Cleanup(func() { _ = m.Delete(cache.Context, key) })

		// The real button path: verifyAnonymousAdmin replays the cached command
		// through the anonymous-admin handler LoadWarns registers.
		LoadWarns(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))
		data := encodeCallbackData("anon_admin", map[string]string{
			"c": fmt.Sprint(chat.Id),
			"m": fmt.Sprint(command.MessageId),
		})
		if err := verifyAnonymousAdmin(bot, newModuleCallbackContext(bot, chat, clicker, data)); err != ext.EndGroups {
			t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
		}

		text := warnsListText(t, client, bot, chat, target.Id)
		if !strings.Contains(text, "button spam") || !strings.Contains(text, anonymous) {
			t.Fatalf("/warns text = %q, want reason marked %q", text, anonymous)
		}
		if strings.Contains(text, "Hidden Mod") || strings.Contains(text, fmt.Sprint(clicker.Id)) {
			t.Fatalf("/warns text = %q reveals the anonymous admin", text)
		}
	})
}

func TestWarnIssuerMarksMessageSentAsTheGroup(t *testing.T) {
	chat := gotgbot.Chat{Id: -1001234567890, Type: "supergroup", Title: "Anon Chat"}
	// Sent on behalf of the group with no GroupAnonymousBot sender: only the
	// sender chat tells that an anonymous admin issued the warn.
	msg := &gotgbot.Message{MessageId: 1, Date: 1, Chat: chat, SenderChat: &chat, Text: "/warn spam"}
	ctx := &ext.Context{EffectiveMessage: msg, EffectiveChat: &chat, EffectiveSender: msg.GetSender()}

	if issuer := warnIssuer(ctx); !issuer.Anonymous {
		t.Fatalf("warnIssuer() = %+v, want an anonymous issuer", issuer)
	}

	mod := gotgbot.User{Id: 501, FirstName: "Admin"}
	msg = &gotgbot.Message{MessageId: 2, Date: 1, Chat: chat, From: &mod, Text: "/warn spam"}
	ctx = &ext.Context{EffectiveMessage: msg, EffectiveChat: &chat, EffectiveUser: &mod, EffectiveSender: msg.GetSender()}
	if issuer := warnIssuer(ctx); issuer.Anonymous || issuer.WarnedBy != mod.Id {
		t.Fatalf("warnIssuer() = %+v, want admin %d shown by name", issuer, mod.Id)
	}
}

func TestBlacklistWarnIsMarkedAutomatic(t *testing.T) {
	useEnglishLocale(t)
	automatic := translated(t, "warns_issuer_auto")
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(chat.Id, "warn"); err != nil {
		t.Fatalf("SetBlacklistAction() error = %v", err)
	}

	if err := blacklistsModule.blacklistWatcher(bot, newModuleMessageContext(bot, chat, member, "this has spam inside")); err != ext.EndGroups {
		t.Fatalf("blacklistWatcher() error = %v, want EndGroups", err)
	}

	if text := warnsListText(t, client, bot, chat, member.Id); !strings.Contains(text, automatic) {
		t.Fatalf("/warns text = %q, want blacklist warn marked %q", text, automatic)
	}
}

func TestImportedBotWarnIsMarkedAutomatic(t *testing.T) {
	useEnglishLocale(t)
	automatic := translated(t, "warns_issuer_auto")
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	// What a backup from another bot account restores for its blacklist warn.
	now := time.Now().UTC()
	ensureWarnParents(t, chat.Id, 42)
	if err := db.DB.Create(&models.Warns{UserId: 42, ChatId: chat.Id, NumWarns: 1, Reasons: models.StringArray{"blacklisted"}}).Error; err != nil {
		t.Fatalf("create warns: %v", err)
	}
	if err := db.DB.Create(&models.WarnEntry{UserId: 42, ChatId: chat.Id, Reason: "blacklisted", WarnedBy: models.WarnedByBot, WarnedAt: &now}).Error; err != nil {
		t.Fatalf("create entry: %v", err)
	}

	if text := warnsListText(t, client, bot, chat, 42); !strings.Contains(text, "("+automatic+",") {
		t.Fatalf("/warns text = %q, want the imported bot warn marked %q", text, automatic)
	}
}

func TestWarnLimitListsIssuerOfEachWarn(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	client, bot, chat, issuer := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	setUserName(t, issuer.Id, "Limit <Mod>")
	limitCtx := newModuleMessageContext(bot, chat, issuer, "/setwarnlimit 1")
	if err := warnsModule.setWarnLimit(bot, limitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit() error = %v, want EndGroups", err)
	}

	if err := warnsModule.warnUser(bot, newWarnReplyContext(bot, chat, issuer, target, "/warn last straw")); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	text := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(text, "last straw") || !strings.Contains(text, "Limit &lt;Mod&gt;") {
		t.Fatalf("limit text = %q, want reason with HTML-escaped issuer", text)
	}
}

func TestWarnLimitSplitsLongList(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	client, bot, chat, issuer := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	for i := range 2 {
		reason := fmt.Sprintf("%d %s", i, strings.Repeat("x", 2500))
		if _, _, err := warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: reason, WarnedBy: issuer.Id}); err != nil {
			t.Fatalf("WarnUser() error = %v", err)
		}
	}
	before := len(client.callsFor("sendMessage"))

	if err := warnsModule.warnUser(bot, newWarnReplyContext(bot, chat, issuer, target, "/warn last straw")); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}

	sent := client.callsFor("sendMessage")[before:]
	if len(sent) < 2 {
		t.Fatalf("sendMessage calls = %d, want the limit notice split in several", len(sent))
	}
	var full strings.Builder
	for i, call := range sent {
		text := call.Params["text"].(string)
		if n := utf8.RuneCountInString(text); n > formatting.MaxMessageLength {
			t.Fatalf("chunk %d has %d chars, over the Telegram limit", i, n)
		}
		_, replies := call.Params["reply_parameters"]
		_, markup := call.Params["reply_markup"]
		if replies != (i == 0) || markup != (i == 0) {
			t.Fatalf("chunk %d reply_parameters=%v reply_markup=%v, want them on the first chunk only", i, replies, markup)
		}
		full.WriteString(text)
	}
	if text := full.String(); !strings.Contains(text, "0 xxx") || !strings.Contains(text, "last straw") {
		t.Fatalf("limit notice = %q, want every warn listed", text)
	}
}

// telegramHTMLToken matches a whole entity or a whole tag at a '&' or '<'.
var telegramHTMLToken = regexp.MustCompile(`^(?:&(?:amp|lt|gt|quot|#[0-9]+);|<(/?)([a-z]+)(?:\s[^<>]*)?>)`)

// checkTelegramHTML fails unless text parses on its own as Telegram HTML:
// no entity or tag cut short, no stray '>', every tag closed in order.
func checkTelegramHTML(t *testing.T, part int, text string) {
	t.Helper()
	var open []string
	for pos := 0; pos < len(text); {
		switch text[pos] {
		case '&', '<':
			m := telegramHTMLToken.FindStringSubmatch(text[pos:])
			if m == nil {
				t.Fatalf("part %d: cut entity or tag at byte %d: %q", part, pos, text[pos:min(pos+12, len(text))])
			}
			switch {
			case m[2] == "":
			case m[1] == "":
				open = append(open, m[2])
			case len(open) == 0 || open[len(open)-1] != m[2]:
				t.Fatalf("part %d: </%s> closes nothing open (open: %v)", part, m[2], open)
			default:
				open = open[:len(open)-1]
			}
			pos += len(m[0])
		case '>':
			t.Fatalf("part %d: stray '>' at byte %d", part, pos)
		default:
			pos++
		}
	}
	if len(open) > 0 {
		t.Fatalf("part %d: tags left open: %v", part, open)
	}
}

// Escaping turns one character of a reason into up to five, so a long reason
// of '&' or '"' becomes a line longer than a Telegram message. Each part sent
// must still be valid HTML on its own.
func TestWarnReplyWithLongEscapedReasonSendsValidHTML(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limit  int
		reason string
	}{
		{"below the limit", 3, strings.Repeat("&", 4000)},
		{"at the limit", 1, strings.Repeat(`"`, 3000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useEnglishLocale(t)
			f := newAdminChatFixture(t)
			client, bot, chat, issuer := f.client, f.bot, f.chat, f.admin
			target := gotgbot.User{Id: 42, FirstName: "Member"}
			if tc.limit != 3 {
				limitCtx := newModuleMessageContext(bot, chat, issuer, fmt.Sprintf("/setwarnlimit %d", tc.limit))
				if err := warnsModule.setWarnLimit(bot, limitCtx); err != ext.EndGroups {
					t.Fatalf("setWarnLimit() error = %v, want EndGroups", err)
				}
			}
			before := len(client.callsFor("sendMessage"))

			if err := warnsModule.warnUser(bot, newWarnReplyContext(bot, chat, issuer, target, "/warn "+tc.reason)); err != ext.EndGroups {
				t.Fatalf("warnUser() error = %v, want EndGroups", err)
			}

			sent := client.callsFor("sendMessage")[before:]
			if len(sent) == 0 {
				t.Fatal("sendMessage calls = 0, want the warn reply")
			}
			var full strings.Builder
			for i, call := range sent {
				text := call.Params["text"].(string)
				if n := utf8.RuneCountInString(text); n > formatting.MaxMessageLength {
					t.Fatalf("part %d has %d chars, over the Telegram limit", i, n)
				}
				checkTelegramHTML(t, i, text)
				full.WriteString(text)
			}
			if escaped := html.EscapeString(tc.reason[:10]); !strings.Contains(full.String(), escaped) {
				t.Fatalf("reply = %q, want the start of the reason", full.String())
			}
		})
	}
}
