//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// banInfoReply runs /baninfo through the bans dispatcher as user in chat and
// returns the replies it sent.
func banInfoReply(t *testing.T, f adminChatFixture, chat gotgbot.Chat, user gotgbot.User, text string) []string {
	t.Helper()
	before := len(f.client.callsFor("sendMessage"))
	update := newModuleMessageContext(f.bot, chat, user, text).Update
	if err := bansPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
	var texts []string
	for _, call := range f.client.callsFor("sendMessage")[before:] {
		texts = append(texts, fmt.Sprint(call.Params["text"]))
	}
	return texts
}

// soleText fails unless exactly one reply was sent and returns it.
func soleText(t *testing.T, replies []string) string {
	t.Helper()
	if len(replies) != 1 {
		t.Fatalf("replies = %q, want exactly one", replies)
	}
	return replies[0]
}

func TestBanInfoShowsTheLatestRecord(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	setUserName(t, f.member.Id, "Spammer <3")
	ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/tban 2h posted <links>")
	if err := bansModule.tBan(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("tBan() error = %v, want EndGroups", err)
	}
	record := singleBanRecord(t, f.chat.Id)

	text := soleText(t, banInfoReply(t, f, f.chat, f.admin, fmt.Sprintf("/baninfo %d", f.member.Id)))

	for _, want := range []string{
		html.EscapeString("Spammer <3"),
		html.EscapeString("posted <links>"),
		translated(t, "bans_baninfo_status_banned"),
		models.BanSourceManual,
		record.Until.UTC().Format(banRecordTimeFormat),
		record.CreatedAt.UTC().Format(banRecordTimeFormat),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("/baninfo reply %q lacks %q", text, want)
		}
	}
}

func TestBanInfoShowsWhoLiftedTheBan(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: f.member.Id, Source: models.BanSourceAntiflood})
	if err := bansModule.unban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, fmt.Sprintf("/unban %d", f.member.Id))); err != ext.EndGroups {
		t.Fatalf("unban() error = %v, want EndGroups", err)
	}
	record := singleBanRecord(t, f.chat.Id)

	text := soleText(t, banInfoReply(t, f, f.chat, f.admin, fmt.Sprintf("/baninfo %d", f.member.Id)))

	want := translatedWith(t, "bans_baninfo_status_lifted", i18n.TranslationParams{
		"admin": banRecordUser(f.admin.Id),
		"date":  record.LiftedAt.UTC().Format(banRecordTimeFormat),
	})
	if !strings.Contains(text, want) || !strings.Contains(text, translated(t, "bans_record_by_bot")) {
		t.Fatalf("/baninfo reply %q, want the lift %q and the bot as the banner", text, want)
	}
}

func TestBanInfoShowsAnEndedTemporaryBanAsEnded(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	ended := time.Now().Add(-time.Minute)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: f.member.Id, Source: models.BanSourceCaptcha, Until: &ended})

	text := soleText(t, banInfoReply(t, f, f.chat, f.admin, fmt.Sprintf("/baninfo %d", f.member.Id)))

	if !strings.Contains(text, translated(t, "bans_baninfo_status_expired")) {
		t.Fatalf("/baninfo reply %q, want the ban shown as ended", text)
	}
}

func TestBanInfoWithoutRecordSaysSo(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)

	text := soleText(t, banInfoReply(t, f, f.chat, f.admin, fmt.Sprintf("/baninfo %d", f.member.Id)))

	if want := translatedWith(t, "bans_baninfo_none", i18n.TranslationParams{"user": banRecordUser(f.member.Id)}); text != want {
		t.Fatalf("/baninfo reply %q, want %q", text, want)
	}
}

func TestBanInfoWithoutUserAnswersWithItsHelp(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)

	text := soleText(t, banInfoReply(t, f, f.chat, f.admin, "/baninfo"))

	assertCommandHelpReply(t, text, "baninfo", translated(t, "common_no_user_specified"))
}

// Members, and admins without the right to restrict, cannot read the records.
func TestBanInfoRefusesWhoCannotRestrict(t *testing.T) {
	useEnglishLocale(t)
	for name, user := range map[string]func(f adminChatFixture) gotgbot.User{
		"member": func(f adminChatFixture) gotgbot.User { return f.member },
		"admin without restrict": func(f adminChatFixture) gotgbot.User {
			limited := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Limited"}
			f.addAdmins(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Limited"},"can_delete_messages":true}`, limited.Id))
			return limited
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 43, Source: models.BanSourceManual, Reason: "secret reason"})

			for _, text := range banInfoReply(t, f, f.chat, user(f), "/baninfo 43") {
				if strings.Contains(text, "secret reason") {
					t.Fatalf("/baninfo showed the record to a %s: %q", name, text)
				}
			}
		})
	}
}

func TestBanInfoFromPMReadsTheConnectedGroup(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	pm := f.connectAdmin(t)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 43, Source: models.BanSourceManual, Reason: "connected reason"})

	text := soleText(t, banInfoReply(t, f, pm, f.admin, "/baninfo 43"))

	if !strings.Contains(text, "connected reason") {
		t.Fatalf("/baninfo from PM replied %q, want the connected group's record", text)
	}
	if calledFor(f, "sendMessage", f.chat.Id) {
		t.Fatal("/baninfo from PM posted in the group")
	}
}

// From PM the group named first wins over the connection.
func TestBanInfoFromPMReadsTheNamedGroup(t *testing.T) {
	f, connected := pmModerationFixture(t)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: victimID, Source: models.BanSourceManual, Reason: "named reason"})
	seedBanRecord(t, models.BanRecord{ChatId: connected, UserId: victimID, Source: models.BanSourceManual, Reason: "connected reason"})

	text := soleText(t, banInfoReply(t, f, privateChatOf(f.admin), f.admin, "/baninfo @fixturegroup @victim"))

	if !strings.Contains(text, "named reason") || strings.Contains(text, "connected reason") {
		t.Fatalf("/baninfo @fixturegroup replied %q, want the named group's record only", text)
	}
	if !strings.Contains(text, groupLine(t, f.chat.Title, f.chat.Id)) {
		t.Fatalf("/baninfo from PM replied %q, want the group named", text)
	}
}

func TestAnonymousAdminBanInfoIsRouted(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 43, Source: models.BanSourceManual, Reason: "anon reason"})

	if err := HandleAnonymousAdmin(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/baninfo 43"), "baninfo"); err != ext.EndGroups {
		t.Fatalf("HandleAnonymousAdmin(baninfo) error = %v, want EndGroups", err)
	}
	if text := soleText(t, sentTexts(f.client)); !strings.Contains(text, "anon reason") {
		t.Fatalf("anonymous /baninfo replied %q, want the record", text)
	}
}
