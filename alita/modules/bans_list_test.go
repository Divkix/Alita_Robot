//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// seedOpenBans records a ban in chatID for each of users, the last one
// newest.
func seedOpenBans(t *testing.T, chatID int64, users ...int64) {
	t.Helper()
	base := time.Now().Add(-time.Hour)
	for i, userID := range users {
		seedBanRecord(t, models.BanRecord{
			ChatId: chatID, UserId: userID, Source: models.BanSourceManual,
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		})
	}
}

// banListMessage runs /banlist through the bans dispatcher as user in chat
// and returns its one reply.
func banListMessage(t *testing.T, f adminChatFixture, chat gotgbot.Chat, user gotgbot.User, text string) moduleBotCall {
	t.Helper()
	before := len(f.client.callsFor("sendMessage"))
	update := newModuleMessageContext(f.bot, chat, user, text).Update
	if err := bansPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
	calls := f.client.callsFor("sendMessage")[before:]
	if len(calls) != 1 {
		t.Fatalf("/banlist sent %d messages, want 1", len(calls))
	}
	return calls[0]
}

// banListButtons returns the callback data of a list message's page
// buttons, by label.
func banListButtons(t *testing.T, call moduleBotCall) map[string]string {
	t.Helper()
	buttons := map[string]string{}
	markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		return buttons
	}
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			buttons[button.Text] = button.CallbackData
		}
	}
	return buttons
}

// pressBanListButton presses data on a list message in chat as user and
// returns the edits it made.
func pressBanListButton(t *testing.T, f adminChatFixture, chat gotgbot.Chat, user gotgbot.User, data string) []moduleBotCall {
	t.Helper()
	before := len(f.client.callsFor("editMessageText"))
	update := newModuleCallbackContext(f.bot, chat, user, data).Update
	if err := bansPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", data, err)
	}
	return f.client.callsFor("editMessageText")[before:]
}

func listedIDs(text string, ids ...int64) []int64 {
	var listed []int64
	for _, id := range ids {
		if strings.Contains(text, fmt.Sprintf("<code>%d</code>", id)) {
			listed = append(listed, id)
		}
	}
	return listed
}

func TestBanListShowsTheOpenBansNewestFirst(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	seedOpenBans(t, f.chat.Id, 501, 502, 503)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 504, Source: models.BanSourceManual})
	if err := bansModule.unban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/unban 504")); err != ext.EndGroups {
		t.Fatalf("unban() error = %v, want EndGroups", err)
	}
	ended := time.Now().Add(-time.Minute)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 505, Source: models.BanSourceAntiflood, Until: &ended})

	call := banListMessage(t, f, f.chat, f.admin, "/banlist")
	text := fmt.Sprint(call.Params["text"])

	if got := listedIDs(text, 501, 502, 503, 504, 505); fmt.Sprint(got) != "[501 502 503]" {
		t.Fatalf("/banlist listed %v, want the open bans 501, 502 and 503: %q", got, text)
	}
	if strings.Index(text, "<code>503</code>") > strings.Index(text, "<code>501</code>") {
		t.Fatalf("/banlist %q does not list the newest ban first", text)
	}
	if len(banListButtons(t, call)) != 0 {
		t.Fatal("a one-page list has page buttons")
	}
}

func TestBanListWithoutOpenBansSaysSo(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)

	text := fmt.Sprint(banListMessage(t, f, f.chat, f.admin, "/banlist").Params["text"])

	if want := translated(t, "bans_banlist_empty"); text != want {
		t.Fatalf("/banlist replied %q, want %q", text, want)
	}
}

func TestBanListPagesTenAtATime(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	users := make([]int64, 0, 12)
	for i := range 12 {
		users = append(users, int64(600+i))
	}
	seedOpenBans(t, f.chat.Id, users...)

	call := banListMessage(t, f, f.chat, f.admin, "/banlist")
	first := listedIDs(fmt.Sprint(call.Params["text"]), users...)
	if len(first) != 10 {
		t.Fatalf("first page listed %d bans, want 10", len(first))
	}
	next, ok := banListButtons(t, call)[translated(t, "bans_banlist_next")]
	if !ok {
		t.Fatal("first page has no next button")
	}

	edits := pressBanListButton(t, f, f.chat, f.admin, next)
	if len(edits) != 1 {
		t.Fatalf("next press made %d edits, want 1", len(edits))
	}
	if got := listedIDs(fmt.Sprint(edits[0].Params["text"]), users...); fmt.Sprint(got) != "[600 601]" {
		t.Fatalf("second page listed %v, want the two oldest bans", got)
	}
	buttons := banListButtons(t, edits[0])
	if _, ok := buttons[translated(t, "bans_banlist_prev")]; !ok || len(buttons) != 1 {
		t.Fatalf("last page buttons = %v, want only previous", buttons)
	}
}

func TestBanListSearchMatchesUsersAndKeepsAcrossPages(t *testing.T) {
	useEnglishLocale(t)
	withMiniredis(t)
	f := newAdminChatFixture(t)
	var spammers []int64
	for i := range 11 {
		id := int64(700 + i)
		setUserName(t, id, fmt.Sprintf("Spam Bot %d", i))
		spammers = append(spammers, id)
	}
	setUserName(t, 799, "Innocent")
	seedOpenBans(t, f.chat.Id, append(spammers, 799)...)

	call := banListMessage(t, f, f.chat, f.admin, "/banlist spam bot")
	text := fmt.Sprint(call.Params["text"])
	if got := listedIDs(text, 799); len(got) != 0 {
		t.Fatalf("search listed a user who does not match: %q", text)
	}
	if !strings.Contains(text, translatedWith(t, "bans_banlist_header_search", i18n.TranslationParams{
		"query": "spam bot", "count": 11, "page": 1, "pages": 2,
	})) {
		t.Fatalf("search header missing from %q", text)
	}

	edits := pressBanListButton(t, f, f.chat, f.admin, banListButtons(t, call)[translated(t, "bans_banlist_next")])
	if len(edits) != 1 {
		t.Fatalf("next press made %d edits, want 1", len(edits))
	}
	page := fmt.Sprint(edits[0].Params["text"])
	if got := listedIDs(page, append(spammers, 799)...); fmt.Sprint(got) != "[700]" {
		t.Fatalf("second page of the search listed %v, want only the oldest match: %q", got, page)
	}
}

func TestBanListSearchMatchesAnID(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	seedOpenBans(t, f.chat.Id, 801, 802)

	text := fmt.Sprint(banListMessage(t, f, f.chat, f.admin, "/banlist 802").Params["text"])

	if got := listedIDs(text, 801, 802); fmt.Sprint(got) != "[802]" {
		t.Fatalf("/banlist 802 listed %v, want only 802", got)
	}
}

// A search kept in Redis that has gone makes the page buttons say so
// instead of listing every ban.
func TestBanListExpiredSearchSaysSo(t *testing.T) {
	useEnglishLocale(t)
	withMiniredis(t)
	f := newAdminChatFixture(t)
	data := encodeCallbackData(banListNamespace, map[string]string{"p": "1", "t": "0123456789abcdef"})

	if edits := pressBanListButton(t, f, f.chat, f.admin, data); len(edits) != 0 {
		t.Fatalf("expired search edited the list: %v", edits)
	}
	answers := f.client.callsFor("answerCallbackQuery")
	if len(answers) == 0 || answers[len(answers)-1].Params["text"] != translated(t, "bans_banlist_expired") {
		t.Fatalf("answers = %v, want the expired notice", answers)
	}
}

// Members, and admins without the right to restrict, cannot list the bans
// or turn a list's page.
func TestBanListRefusesWhoCannotRestrict(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	users := make([]int64, 0, 11)
	for i := range 11 {
		users = append(users, int64(900+i))
	}
	seedOpenBans(t, f.chat.Id, users...)
	limited := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Limited"}
	f.addAdmins(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Limited"},"can_delete_messages":true}`, limited.Id))

	next := banListButtons(t, banListMessage(t, f, f.chat, f.admin, "/banlist"))[translated(t, "bans_banlist_next")]
	for name, user := range map[string]gotgbot.User{"member": f.member, "admin without restrict": limited} {
		t.Run(name, func(t *testing.T) {
			update := newModuleMessageContext(f.bot, f.chat, user, "/banlist").Update
			if err := bansPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate() error = %v", err)
			}
			if edits := pressBanListButton(t, f, f.chat, user, next); len(edits) != 0 {
				t.Fatalf("a %s turned the page: %v", name, edits)
			}
		})
	}
	// Only the admin's list was sent.
	shown := 0
	for _, text := range sentTexts(f.client) {
		if len(listedIDs(text, users...)) > 0 {
			shown++
		}
	}
	if shown != 1 {
		t.Fatalf("%d messages listed the bans, want only the admin's", shown)
	}
}

// From PM the group named first wins over the connection, and the page
// buttons carry that group.
func TestBanListFromPMListsTheNamedGroup(t *testing.T) {
	f, connected := pmModerationFixture(t)
	users := make([]int64, 0, 11)
	for i := range 11 {
		users = append(users, int64(1100+i))
	}
	seedOpenBans(t, f.chat.Id, users...)
	seedOpenBans(t, connected, 1199)
	pm := privateChatOf(f.admin)

	call := banListMessage(t, f, pm, f.admin, "/banlist @fixturegroup")
	text := fmt.Sprint(call.Params["text"])
	if len(listedIDs(text, users...)) != 10 || len(listedIDs(text, 1199)) != 0 {
		t.Fatalf("/banlist @fixturegroup replied %q, want the named group's bans only", text)
	}
	if !strings.Contains(text, groupLine(t, f.chat.Title, f.chat.Id)) {
		t.Fatalf("/banlist from PM replied %q, want the group named", text)
	}
	if calledFor(f, "sendMessage", f.chat.Id) {
		t.Fatal("/banlist from PM posted in the group")
	}

	var next string
	for _, data := range banListButtons(t, call) {
		next = data
	}
	edits := pressBanListButton(t, f, pm, f.admin, next)
	if len(edits) != 1 || fmt.Sprint(listedIDs(fmt.Sprint(edits[0].Params["text"]), users...)) != "[1100]" {
		t.Fatalf("next press from PM made %v, want the named group's second page", edits)
	}
}

func TestAnonymousAdminBanListIsRouted(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	seedOpenBans(t, f.chat.Id, 1201)

	if err := HandleAnonymousAdmin(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/banlist"), "banlist"); err != ext.EndGroups {
		t.Fatalf("HandleAnonymousAdmin(banlist) error = %v, want EndGroups", err)
	}
	if text := soleText(t, sentTexts(f.client)); len(listedIDs(text, 1201)) != 1 {
		t.Fatalf("anonymous /banlist replied %q, want the list", text)
	}
}
