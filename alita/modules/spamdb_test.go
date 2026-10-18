//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/spamdb"
)

const (
	spamDBCASListed   = `{"ok":true,"result":{"reasons":[1],"offenses":1,"messages":["spam"],"time_added":"2026-10-02T15:26:20.000Z"}}`
	spamDBCASNotFound = `{"ok":false,"description":"Record not found."}`
	spamDBLogChannel  = int64(-1009876543210)
)

// spamDBCAS is a fake CAS API that answers every lookup with one body and
// counts the lookups.
type spamDBCAS struct {
	hits atomic.Int32
}

// withSpamDBCAS loads the module into a fresh dispatcher and points it at a
// fake CAS answering status/body and a fake lols.bot answering "not banned",
// with the operator switch set to enabled.
func withSpamDBCAS(t *testing.T, enabled bool, status int, body string) (*ext.Dispatcher, *spamDBCAS) {
	t.Helper()
	dispatcher, cas, _ := withSpamDBServices(t, enabled, status, body, false)
	return dispatcher, cas
}

// withSpamDBServices is withSpamDBCAS with a fake lols.bot that answers
// lolsBanned for whichever user is asked about.
func withSpamDBServices(t *testing.T, enabled bool, casStatus int, casBody string, lolsBanned bool) (*ext.Dispatcher, *spamDBCAS, *spamDBCAS) {
	t.Helper()
	cache.SetupTestMemoryMarshaler(t)

	previousEnabled := config.AppConfig.EnableSpamDB
	config.AppConfig.EnableSpamDB = enabled

	cas := &spamDBCAS{}
	casSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cas.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(casStatus)
		_, _ = w.Write([]byte(casBody))
	}))
	lols := &spamDBCAS{}
	lolsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lols.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"ok":true,"user_id":%s,"banned":%t}`, r.URL.Query().Get("id"), lolsBanned)
	}))

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadSpamDB(dispatcher)
	previousChecker := spamDBChecker
	spamDBChecker = spamdb.New(spamdb.Options{
		CASURL:  casSrv.URL + "/check",
		LolsURL: lolsSrv.URL + "/account",
		Timeout: time.Second,
	})

	t.Cleanup(func() {
		casSrv.Close()
		lolsSrv.Close()
		spamDBChecker = previousChecker
		config.AppConfig.EnableSpamDB = previousEnabled
	})
	return dispatcher, cas, lols
}

// spamDBChat returns a supergroup with the join check set to action and a log
// channel linked, so a notice has somewhere to go.
func spamDBChat(t *testing.T, enabled bool, action string) gotgbot.Chat {
	t.Helper()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Spam DB Chat"}
	if err := spamdbsettings.SetSpamDBEnabled(chat.Id, enabled); err != nil {
		t.Fatalf("SetSpamDBEnabled() error = %v", err)
	}
	if err := spamdbsettings.SetSpamDBAction(chat.Id, action); err != nil {
		t.Fatalf("SetSpamDBAction() error = %v", err)
	}
	if err := logchannels.Set(chat.Id, chat.Title, spamDBLogChannel); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	return chat
}

var spamDBUpdateID atomic.Int64

func spamDBJoin(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, chat gotgbot.Chat, members ...gotgbot.User) {
	t.Helper()
	id := spamDBUpdateID.Add(1)
	update := &gotgbot.Update{
		UpdateId: id,
		Message: &gotgbot.Message{
			MessageId:      id,
			Date:           1,
			Chat:           chat,
			From:           &members[0],
			NewChatMembers: members,
		},
	}
	if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
}

func spamDBCommand(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, chat gotgbot.Chat, from gotgbot.User, text string) {
	t.Helper()
	aispamProcessUpdate(t, dispatcher, bot, chat, from, spamDBUpdateID.Add(1), text)
}

// spamDBMessagesTo returns the texts the bot sent to chatID.
func spamDBMessagesTo(client *moduleBotClient, chatID int64) []string {
	var texts []string
	for _, call := range client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			texts = append(texts, fmt.Sprint(call.Params["text"]))
		}
	}
	return texts
}

func TestSpamDBActsOnListedJoinerAndLogsToLogChannel(t *testing.T) {
	cases := []struct {
		action       string
		wantBan      int
		wantRestrict int
	}{
		{spamdbsettings.ActionBan, 1, 0},
		{spamdbsettings.ActionMute, 0, 1},
		{spamdbsettings.ActionLog, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			dispatcher, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := spamDBChat(t, true, tc.action)
			spammer := gotgbot.User{Id: 5550001, FirstName: "Spammer"}

			spamDBJoin(t, dispatcher, bot, chat, spammer)

			if got := cas.hits.Load(); got != 1 {
				t.Fatalf("CAS lookups = %d, want 1", got)
			}
			if got := len(client.callsFor("banChatMember")); got != tc.wantBan {
				t.Fatalf("banChatMember calls = %d, want %d", got, tc.wantBan)
			}
			if got := len(client.callsFor("restrictChatMember")); got != tc.wantRestrict {
				t.Fatalf("restrictChatMember calls = %d, want %d", got, tc.wantRestrict)
			}
			for _, method := range []string{"banChatMember", "restrictChatMember"} {
				for _, call := range client.callsFor(method) {
					if fmt.Sprint(call.Params["user_id"]) != fmt.Sprint(spammer.Id) {
						t.Fatalf("%s user_id = %v, want %d", method, call.Params["user_id"], spammer.Id)
					}
				}
			}

			notices := spamDBMessagesTo(client, spamDBLogChannel)
			if len(notices) != 1 {
				t.Fatalf("log channel messages = %d, want 1", len(notices))
			}
			if !strings.Contains(notices[0], fmt.Sprint(spammer.Id)) || !strings.Contains(notices[0], tc.action) {
				t.Fatalf("log notice %q does not name the user and the action %q", notices[0], tc.action)
			}
			if got := spamDBMessagesTo(client, chat.Id); len(got) != 0 {
				t.Fatalf("group got %d messages, want none: %v", len(got), got)
			}
		})
	}
}

func TestSpamDBLeavesCleanJoinerAlone(t *testing.T) {
	dispatcher, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := spamDBChat(t, true, spamdbsettings.ActionBan)

	spamDBJoin(t, dispatcher, bot, chat, gotgbot.User{Id: 5550002, FirstName: "Clean"})

	if got := cas.hits.Load(); got != 1 {
		t.Fatalf("CAS lookups = %d, want 1", got)
	}
	if got := len(client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want 0", got)
	}
	if got := spamDBMessagesTo(client, spamDBLogChannel); len(got) != 0 {
		t.Fatalf("log channel got %v, want nothing", got)
	}
}

// withSpamDBGreetings loads greetings next to the join check and turns on
// both the welcome and the captcha in chat, so a join that slips past the
// check would post in the group.
func withSpamDBGreetings(t *testing.T, dispatcher *ext.Dispatcher, chat gotgbot.Chat) {
	t.Helper()
	LoadGreetings(dispatcher)
	if err := greetings.SetWelcomeToggle(chat.Id, true); err != nil {
		t.Fatalf("SetWelcomeToggle() error = %v", err)
	}
	if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
}

// spamDBGroupPosts counts the messages and photos the bot posted in chatID.
func spamDBGroupPosts(client *moduleBotClient, chatID int64) int {
	posts := 0
	for _, method := range []string{"sendMessage", "sendPhoto"} {
		for _, call := range client.callsFor(method) {
			if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
				posts++
			}
		}
	}
	return posts
}

// spamDBMemberJoin delivers the chat_member update Telegram sends for the
// same join alongside the service message.
func spamDBMemberJoin(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, chat gotgbot.Chat, member gotgbot.User) {
	t.Helper()
	update := &gotgbot.Update{
		UpdateId: spamDBUpdateID.Add(1),
		ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          chat,
			From:          member,
			Date:          1,
			OldChatMember: gotgbot.ChatMemberLeft{User: member},
			NewChatMember: gotgbot.ChatMemberMember{User: member},
		},
	}
	if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(chat_member) error = %v", err)
	}
}

// A member the check banned or muted gets no welcome and no captcha, from
// either update Telegram sends for the join. A captcha would lift the mute
// once solved or expired, and its timeout kick would undo the ban.
func TestSpamDBKeepsGreetingsAndCaptchaAwayFromActedJoiner(t *testing.T) {
	for _, tc := range []struct {
		action       string
		wantRestrict int
	}{
		{spamdbsettings.ActionBan, 0},
		{spamdbsettings.ActionMute, 1},
	} {
		t.Run(tc.action, func(t *testing.T) {
			dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := spamDBChat(t, true, tc.action)
			withSpamDBGreetings(t, dispatcher, chat)
			spammer := gotgbot.User{Id: 5550101, FirstName: "Spammer"}

			spamDBJoin(t, dispatcher, bot, chat, spammer)
			spamDBMemberJoin(t, dispatcher, bot, chat, spammer)

			if got := spamDBGroupPosts(client, chat.Id); got != 0 {
				t.Fatalf("group posts = %d, want none", got)
			}
			if got := len(client.callsFor("restrictChatMember")); got != tc.wantRestrict {
				t.Fatalf("restrictChatMember calls = %d, want %d (the check's own)", got, tc.wantRestrict)
			}
			attempt, err := captcha.GetCaptchaAttemptIncludingExpired(spammer.Id, chat.Id)
			if err != nil {
				t.Fatalf("GetCaptchaAttemptIncludingExpired() error = %v", err)
			}
			if attempt != nil {
				t.Fatalf("captcha attempt = %#v, want none: solving or expiring it restores the member", attempt)
			}
			muted, err := captcha.GetMutedUsersForChat(chat.Id)
			if err != nil {
				t.Fatalf("GetMutedUsersForChat() error = %v", err)
			}
			if len(muted) != 0 {
				t.Fatalf("scheduled captcha unmutes = %d, want none", len(muted))
			}
		})
	}
}

// A member the check did not act on is still free in the chat, so the join
// goes through greetings and captcha as usual: when Telegram refuses the ban,
// and when the chat only logs listed members.
func TestSpamDBLeavesJoinToCaptchaWhenMemberStaysFree(t *testing.T) {
	for _, tc := range []struct {
		name    string
		action  string
		banFail bool
	}{
		{"ban refused", spamdbsettings.ActionBan, true},
		{"log only", spamdbsettings.ActionLog, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
			client := newModuleBotClient()
			if tc.banFail {
				client.errors["banChatMember"] = errors.New("Bad Request: not enough rights to restrict/unrestrict chat member")
			}
			bot := newModuleTestBot(client)
			chat := spamDBChat(t, true, tc.action)
			withSpamDBGreetings(t, dispatcher, chat)
			spammer := gotgbot.User{Id: 5550102, FirstName: "Spammer"}

			spamDBJoin(t, dispatcher, bot, chat, spammer)

			attempt, err := captcha.GetCaptchaAttemptIncludingExpired(spammer.Id, chat.Id)
			if err != nil {
				t.Fatalf("GetCaptchaAttemptIncludingExpired() error = %v", err)
			}
			if attempt == nil {
				t.Fatal("captcha attempt = nil, want the usual captcha for a member still in the chat")
			}
			if got := spamDBGroupPosts(client, chat.Id); got != 1 {
				t.Fatalf("group posts = %d, want the captcha prompt", got)
			}
		})
	}
}

// User IDs leave the bot only for chats that opted in, and never when the
// operator has switched the feature off.
func TestSpamDBSendsNothingToCASUnlessEnabled(t *testing.T) {
	cases := []struct {
		name     string
		operator bool
		chat     bool
	}{
		{"chat not opted in", true, false},
		{"operator switch off", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, cas := withSpamDBCAS(t, tc.operator, http.StatusOK, spamDBCASListed)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := spamDBChat(t, tc.chat, spamdbsettings.ActionBan)

			spamDBJoin(t, dispatcher, bot, chat, gotgbot.User{Id: 5550003, FirstName: "Spammer"})

			if got := cas.hits.Load(); got != 0 {
				t.Fatalf("CAS lookups = %d, want 0", got)
			}
			if got := len(client.callsFor("banChatMember")); got != 0 {
				t.Fatalf("banChatMember calls = %d, want 0", got)
			}
		})
	}
}

func TestSpamDBSkipsAdminsApprovedUsersAndBots(t *testing.T) {
	dispatcher, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_restrict_members":true},` +
			`{"status":"administrator","user":{"id":5550010,"is_bot":false,"first_name":"Admin"},"can_restrict_members":true}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := spamDBChat(t, true, spamdbsettings.ActionBan)
	approved := gotgbot.User{Id: 5550011, FirstName: "Approved"}
	if err := approvals.AddApprovedUser(chat.Id, approved.Id, 5550010, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	spamDBJoin(t, dispatcher, bot, chat,
		gotgbot.User{Id: 5550010, FirstName: "Admin"},
		approved,
		gotgbot.User{Id: 5550012, FirstName: "Helper", IsBot: true},
		bot.User,
	)

	if got := cas.hits.Load(); got != 0 {
		t.Fatalf("CAS lookups = %d, want 0", got)
	}
	if got := len(client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want 0", got)
	}
}

// A CAS outage must let the join through without a notice.
func TestSpamDBFailsOpenWhenCASFails(t *testing.T) {
	dispatcher, cas := withSpamDBCAS(t, true, http.StatusBadGateway, `upstream down`)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := spamDBChat(t, true, spamdbsettings.ActionBan)

	spamDBJoin(t, dispatcher, bot, chat, gotgbot.User{Id: 5550004, FirstName: "Unknown"})

	if got := cas.hits.Load(); got != 1 {
		t.Fatalf("CAS lookups = %d, want 1", got)
	}
	if got := len(client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want 0", got)
	}
	if got := spamDBMessagesTo(client, spamDBLogChannel); len(got) != 0 {
		t.Fatalf("log channel got %v, want nothing", got)
	}
}

func TestSpamDBCommandSetsSwitchAndAction(t *testing.T) {
	dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Command Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	get := func() (bool, string) {
		t.Helper()
		settings, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), chat.Id)
		if err != nil {
			t.Fatalf("GetSpamDBSettingsContext() error = %v", err)
		}
		return settings.Enabled, settings.Action
	}

	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb on")
	if enabled, _ := get(); !enabled {
		t.Fatal("/spamdb on did not enable the check")
	}

	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb action mute")
	if enabled, action := get(); !enabled || action != spamdbsettings.ActionMute {
		t.Fatalf("after /spamdb action mute: enabled=%v action=%q, want true, mute", enabled, action)
	}

	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb action kick")
	if _, action := get(); action != spamdbsettings.ActionMute {
		t.Fatalf("/spamdb action kick changed the action to %q", action)
	}

	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb off")
	if enabled, action := get(); enabled || action != spamdbsettings.ActionMute {
		t.Fatalf("after /spamdb off: enabled=%v action=%q, want false, mute", enabled, action)
	}

	if got := len(spamDBMessagesTo(client, chat.Id)); got != 4 {
		t.Fatalf("replies = %d, want one per command", got)
	}
}

func TestSpamDBCommandRefusesEnableWhenOperatorSwitchedOff(t *testing.T) {
	dispatcher, _ := withSpamDBCAS(t, false, http.StatusOK, spamDBCASNotFound)
	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Switched Off Chat"}

	spamDBCommand(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Admin"}, "/spamdb on")

	settings, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), chat.Id)
	if err != nil {
		t.Fatalf("GetSpamDBSettingsContext() error = %v", err)
	}
	if settings.Enabled {
		t.Fatal("check was enabled while the operator switch was off")
	}
	if got := len(spamDBMessagesTo(client, chat.Id)); got != 1 {
		t.Fatalf("replies = %d, want the refusal", got)
	}
}

func TestSpamDBCommandRejectsNonAdmin(t *testing.T) {
	dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
	bot := newModuleTestBot(newModuleBotClient())
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Non Admin Chat"}

	spamDBCommand(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "/spamdb on")

	settings, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), chat.Id)
	if err != nil {
		t.Fatalf("GetSpamDBSettingsContext() error = %v", err)
	}
	if settings.Enabled {
		t.Fatal("a non-admin was able to enable the check")
	}
}

// A new member's ID goes only to the databases the chat chose, and the notice
// names the database that listed them.
func TestSpamDBAsksOnlyTheChatsDatabases(t *testing.T) {
	cases := []struct {
		name               string
		lolsOn, casOff     bool
		wantCAS, wantLols  int32
		wantBan            int
		wantNoticeDatabase string
	}{
		{"default is CAS only", false, false, 1, 0, 0, ""},
		{"CAS and lols.bot", true, false, 1, 1, 1, "lols.bot"},
		{"lols.bot only", true, true, 0, 1, 1, "lols.bot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, cas, lols := withSpamDBServices(t, true, http.StatusOK, spamDBCASNotFound, true)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := spamDBChat(t, true, spamdbsettings.ActionBan)
			if tc.lolsOn {
				if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseLols, true); err != nil {
					t.Fatalf("SetSpamDBDatabase(lols) error = %v", err)
				}
			}
			if tc.casOff {
				if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseCAS, false); err != nil {
					t.Fatalf("SetSpamDBDatabase(cas off) error = %v", err)
				}
			}
			spammer := gotgbot.User{Id: 5550020, FirstName: "Spammer"}

			spamDBJoin(t, dispatcher, bot, chat, spammer)

			if cas.hits.Load() != tc.wantCAS || lols.hits.Load() != tc.wantLols {
				t.Fatalf("lookups CAS=%d lols=%d, want %d/%d", cas.hits.Load(), lols.hits.Load(), tc.wantCAS, tc.wantLols)
			}
			if got := len(client.callsFor("banChatMember")); got != tc.wantBan {
				t.Fatalf("banChatMember calls = %d, want %d", got, tc.wantBan)
			}
			notices := spamDBMessagesTo(client, spamDBLogChannel)
			if tc.wantNoticeDatabase == "" {
				if len(notices) != 0 {
					t.Fatalf("log channel got %v, want nothing", notices)
				}
				return
			}
			if len(notices) != 1 || !strings.Contains(notices[0], tc.wantNoticeDatabase) {
				t.Fatalf("log notices = %v, want one naming %s", notices, tc.wantNoticeDatabase)
			}
		})
	}
}

func TestSpamDBCommandSelectsDatabases(t *testing.T) {
	dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Databases Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	steps := []struct {
		command   string
		cas, lols bool
	}{
		{"/spamdb db lols on", true, true},
		{"/spamdb db cas off", false, true},
		// The last database cannot be switched off.
		{"/spamdb db lols off", false, true},
		{"/spamdb db spamwatch on", false, true},
		{"/spamdb db cas maybe", false, true},
		{"/spamdb db", false, true},
		{"/spamdb db CAS ON", true, true},
		{"/spamdb db lols no", true, false},
	}
	for i, s := range steps {
		spamDBCommand(t, dispatcher, bot, chat, admin, s.command)
		settings, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), chat.Id)
		if err != nil {
			t.Fatalf("GetSpamDBSettingsContext() error = %v", err)
		}
		if settings.CheckCAS != s.cas || settings.CheckLols != s.lols {
			t.Fatalf("after %q: cas %v lols %v, want cas %v lols %v", s.command, settings.CheckCAS, settings.CheckLols, s.cas, s.lols)
		}
		if got := len(spamDBMessagesTo(client, chat.Id)); got != i+1 {
			t.Fatalf("after %q: replies = %d, want one per command", s.command, got)
		}
	}
}

// The chat's databases decide what /spamdb says is sent, and to whom.
func TestSpamDBStatusNamesTheChatsDatabases(t *testing.T) {
	useEnglishLocale(t)
	dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Status Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb")
	if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseLols, true); err != nil {
		t.Fatalf("SetSpamDBDatabase(lols) error = %v", err)
	}
	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb")

	replies := spamDBMessagesTo(client, chat.Id)
	if len(replies) != 2 {
		t.Fatalf("replies = %d, want 2", len(replies))
	}
	if strings.Contains(replies[0], "lols.bot") {
		t.Fatalf("CAS-only status mentions lols.bot: %q", replies[0])
	}
	if !strings.Contains(replies[1], "lols.bot") {
		t.Fatalf("status with lols.bot on does not name it: %q", replies[1])
	}
}

func TestSpamDBIsReachableFromHelp(t *testing.T) {
	registry := DefaultHelpRegistry()
	previousEnabled, hadEnabled := registry.AbleMap[spamDBModule.moduleName]
	t.Cleanup(func() {
		if hadEnabled {
			registry.AbleMap[spamDBModule.moduleName] = previousEnabled
			return
		}
		delete(registry.AbleMap, spamDBModule.moduleName)
	})

	LoadSpamDB(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))

	var button bool
	for _, row := range initHelpButtonsFrom(registry, "en").InlineKeyboard {
		for _, each := range row {
			button = button || each.Text == spamDBModule.moduleName
		}
	}
	if !button {
		t.Fatalf("%s is missing from the /help menu", spamDBModule.moduleName)
	}
	if got := getModuleNameFromAltName("spamdb", registry); got != spamDBModule.moduleName {
		t.Fatalf("/help spamdb resolves to %q, want %q", got, spamDBModule.moduleName)
	}
}
