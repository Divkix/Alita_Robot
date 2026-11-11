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
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
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
	// The check is a stage of membership intake, which greetings runs.
	LoadGreetings(dispatcher)
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

// spamDBChat returns a supergroup with the join check set to action, a log
// channel linked, so a notice has somewhere to go, and no welcome, so the
// group hears from the bot only about the check.
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
	if err := greetings.SetWelcomeToggle(chat.Id, false); err != nil {
		t.Fatalf("SetWelcomeToggle() error = %v", err)
	}
	return chat
}

var spamDBUpdateID atomic.Int64

// withSpamDBReputation turns the shared network on with explicit thresholds and
// restores the whole process config when the test ends.
func withSpamDBReputation(t *testing.T, minChats, windowDays int) {
	t.Helper()
	previous := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = previous })
	config.AppConfig.EnableReputation = true
	config.AppConfig.ReputationMinChats = minChats
	config.AppConfig.ReputationWindowDays = windowDays
}

// seedSpamDBEvent records one human moderation event, the only kind the shared
// network aggregates.
func seedSpamDBEvent(t *testing.T, chatID, userID int64, action string) {
	t.Helper()
	if err := moderation.Record(models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      42,
		TargetUserId: userID,
		Action:       action,
	}); err != nil {
		t.Fatalf("Record(%s) error = %v", action, err)
	}
}

// spamDBSharedOnlyChat returns a chat whose only spam database is Shared bans.
func spamDBSharedOnlyChat(t *testing.T, action string) gotgbot.Chat {
	t.Helper()
	chat := spamDBChat(t, true, action)
	if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseShared, true); err != nil {
		t.Fatalf("SetSpamDBDatabase(shared, true) error = %v", err)
	}
	if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseCAS, false); err != nil {
		t.Fatalf("SetSpamDBDatabase(cas, false) error = %v", err)
	}
	return chat
}

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

// The check is an intake stage: it looks up a newcomer once however Telegram
// reports the join, a ban removes them and a mute holds them (no challenge,
// no welcome either way), and anything it does not act on reaches the
// captcha.
func TestSpamDBStageGivesEveryJoinWayOneVerdict(t *testing.T) {
	type want struct{ lookups, challenges, welcomes, bans, mutes, logs int }
	for _, tc := range []struct {
		name      string
		action    string
		casStatus int
		banErr    bool
		want      want
	}{
		{name: "ban", action: spamdbsettings.ActionBan, casStatus: http.StatusOK, want: want{lookups: 1, bans: 1, logs: 1}},
		{name: "mute", action: spamdbsettings.ActionMute, casStatus: http.StatusOK, want: want{lookups: 1, mutes: 1, logs: 1}},
		{name: "log only", action: spamdbsettings.ActionLog, casStatus: http.StatusOK, want: want{lookups: 1, challenges: 1, logs: 1}},
		{name: "lookup failed", action: spamdbsettings.ActionBan, casStatus: http.StatusBadGateway, want: want{lookups: 1, challenges: 1}},
		{name: "ban failed", action: spamdbsettings.ActionBan, casStatus: http.StatusOK, banErr: true, want: want{lookups: 1, challenges: 1, bans: 1, logs: 1}},
	} {
		for _, way := range joinWays {
			t.Run(tc.name+"/"+strings.Join(way, " then "), func(t *testing.T) {
				_, cas := withSpamDBCAS(t, true, tc.casStatus, spamDBCASListed)
				j := newIntakeJoin(t, false)
				require.NoError(t, spamdbsettings.SetSpamDBEnabled(j.chat.Id, true))
				require.NoError(t, spamdbsettings.SetSpamDBAction(j.chat.Id, tc.action))
				require.NoError(t, logchannels.Set(j.chat.Id, j.chat.Title, spamDBLogChannel))
				require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
				if tc.banErr {
					j.client.errors["banChatMember"] = errors.New("Bad Request: not enough rights to restrict/unrestrict chat member")
				}

				j.deliver(t, way)

				challenges := j.challenges(t)
				mutes := len(j.client.callsFor("restrictChatMember"))
				if challenges > 0 {
					mutes = 0
				}
				logs := callsToChat(j.client, "sendMessage", spamDBLogChannel)
				got := want{int(cas.hits.Load()), challenges, j.welcomes(), j.bans(), mutes, len(logs)}
				require.Equal(t, tc.want, got)
				if tc.want.challenges == 0 {
					muted, err := captcha.GetMutedUsersForChat(j.chat.Id)
					require.NoError(t, err)
					require.Empty(t, muted, "no captcha unmute may lift the check's mute")
				}
				if tc.want.bans == 1 && !tc.banErr {
					require.Equal(t, models.BanSourceSpamDB, singleBanRecord(t, j.chat.Id).Source)
					require.Contains(t, fmt.Sprint(logs[0].Params["reply_markup"]), punishment.LogUnbanNamespace+"|", "the log entry keeps its Unban button")
				}
			})
		}
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

// A settings entry cached by a release that predates the database choice has
// neither check field. It must not read as "no database", which would let
// every joiner in until the entry expires.
func TestSpamDBReadsCacheEntryWithoutDatabaseFieldsAsCAS(t *testing.T) {
	dispatcher, cas, lols := withSpamDBServices(t, true, http.StatusOK, spamDBCASNotFound, true)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := spamDBChat(t, true, spamdbsettings.ActionBan)

	type settingsBeforeDatabaseChoice struct {
		ChatID  int64  `json:"chat_id,omitempty"`
		Enabled bool   `json:"enabled,omitempty"`
		Action  string `json:"action,omitempty"`
	}
	key := dbcache.CacheKey("spam_db_settings", chat.Id)
	stale := settingsBeforeDatabaseChoice{ChatID: chat.Id, Enabled: true, Action: spamdbsettings.ActionBan}
	if err := cache.GetMarshal().Set(cache.Context, key, stale); err != nil {
		t.Fatalf("seed stale cache entry: %v", err)
	}
	dbcache.ResetLocalForTest()
	if got, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), chat.Id); err != nil || got.CheckCAS || got.CheckLols || got.CheckShared {
		t.Fatalf("stale entry not served from cache: %+v, %v", got, err)
	}
	dbcache.ResetLocalForTest()

	spamDBJoin(t, dispatcher, bot, chat, gotgbot.User{Id: 5550030, FirstName: "Spammer"})

	if cas.hits.Load() != 1 || lols.hits.Load() != 0 {
		t.Fatalf("lookups CAS=%d lols=%d, want 1/0", cas.hits.Load(), lols.hits.Load())
	}
}

func TestSpamDBCommandParsesDatabaseNamesAndToggles(t *testing.T) {
	useRealLocales(t)
	withSpamDBReputation(t, 3, 30)
	for _, command := range []string{"db SHARED ON", "db shared yes", "db shared enable"} {
		t.Run(command, func(t *testing.T) {
			f := newAdminChatFixture(t)
			contractDispatch(t, f, f.admin, "/spamdb "+command)
			row, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), f.chat.Id)
			if err != nil || !row.CheckShared {
				t.Fatalf("parsed choice = %+v, %v", row, err)
			}
		})
	}
	for _, command := range []string{"db spamwatch on", "db cas maybe", "db"} {
		t.Run(command, func(t *testing.T) {
			f := newAdminChatFixture(t)
			contractDispatch(t, f, f.admin, "/spamdb "+command)
			if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != translated(t, "spamdb_db_usage") {
				t.Fatalf("database usage = %q", reply)
			}
		})
	}
}

// A user several distinct chats removed is acted on through the shared
// source, the log notice names it, and the block writes no moderation event of
// its own: it is not a human action.
func TestSpamDBActsOnSharedListedJoiner(t *testing.T) {
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
			withSpamDBReputation(t, 3, 30)
			dispatcher, cas, _ := withSpamDBServices(t, true, http.StatusOK, spamDBCASNotFound, false)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := spamDBSharedOnlyChat(t, tc.action)
			spammer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Spammer"}
			seedSpamDBEvent(t, chat.Id-1, spammer.Id, models.ModerationActionBan)
			seedSpamDBEvent(t, chat.Id-2, spammer.Id, models.ModerationActionKick)
			seedSpamDBEvent(t, chat.Id-3, spammer.Id, models.ModerationActionBan)

			spamDBJoin(t, dispatcher, bot, chat, spammer)

			if got := cas.hits.Load(); got != 0 {
				t.Fatalf("CAS lookups = %d, want 0 for a shared-only chat", got)
			}
			if got := len(client.callsFor("banChatMember")); got != tc.wantBan {
				t.Fatalf("banChatMember calls = %d, want %d", got, tc.wantBan)
			}
			if got := len(client.callsFor("restrictChatMember")); got != tc.wantRestrict {
				t.Fatalf("restrictChatMember calls = %d, want %d", got, tc.wantRestrict)
			}
			notices := spamDBMessagesTo(client, spamDBLogChannel)
			if len(notices) != 1 {
				t.Fatalf("log channel messages = %d, want 1", len(notices))
			}
			if !strings.Contains(notices[0], "Shared bans") || !strings.Contains(notices[0], tc.action) {
				t.Fatalf("log notice %q does not name Shared bans and the action %q", notices[0], tc.action)
			}

			events, err := moderation.ListForTarget(spammer.Id, time.Now().Add(-time.Hour))
			if err != nil {
				t.Fatalf("ListForTarget() error = %v", err)
			}
			if len(events) != 3 {
				t.Fatalf("moderation events = %d, want the 3 seeded ones and no row from the block", len(events))
			}
		})
	}
}

// The shared verdict is cached for ten minutes: once a user is known clean,
// adding enough events to list them does not change the second join.
func TestSpamDBSharedLookupServedFromCache(t *testing.T) {
	withSpamDBReputation(t, 3, 30)
	dispatcher, _, _ := withSpamDBServices(t, true, http.StatusOK, spamDBCASNotFound, false)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	first := spamDBSharedOnlyChat(t, spamdbsettings.ActionBan)
	second := spamDBSharedOnlyChat(t, spamdbsettings.ActionBan)
	joiner := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Member"}
	// Two distinct chats is below the threshold.
	seedSpamDBEvent(t, first.Id-1, joiner.Id, models.ModerationActionBan)
	seedSpamDBEvent(t, first.Id-2, joiner.Id, models.ModerationActionBan)

	spamDBJoin(t, dispatcher, bot, first, joiner)
	if got := spamDBMessagesTo(client, spamDBLogChannel); len(got) != 0 {
		t.Fatalf("first join produced %v, want the member let in", got)
	}

	// A fresh lookup would now list the user; the cached clean verdict must
	// still let them in on the second join.
	seedSpamDBEvent(t, first.Id-3, joiner.Id, models.ModerationActionBan)
	spamDBJoin(t, dispatcher, bot, second, joiner)

	if got := len(client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want 0: the second join queried the database instead of the shared cache", got)
	}
	if got := spamDBMessagesTo(client, spamDBLogChannel); len(got) != 0 {
		t.Fatalf("second join produced %v, want the cached clean verdict", got)
	}
}

// Switching a database on says members are looked up there, and lols.bot that
// their IDs are sent, only while the check runs: it does not with the chat's
// switch off or with the operator's off.
func TestSpamDBDatabaseOnReplyFollowsTheCheck(t *testing.T) {
	useEnglishLocale(t)
	tr := i18n.MustNewTranslator("en")
	text := func(key string, params i18n.TranslationParams) string {
		t.Helper()
		got, _ := tr.GetString(key, params)
		if got == "" {
			t.Fatalf("locale key %s is missing", key)
		}
		return got
	}
	lols := i18n.TranslationParams{"database": "lols.bot"}
	cas := i18n.TranslationParams{"database": "CAS"}

	cases := []struct {
		name               string
		chatOn, operatorOn bool
		wantCAS, wantLols  string
	}{
		{
			name: "check running", chatOn: true, operatorOn: true,
			wantCAS:  text("spamdb_db_enabled", cas),
			wantLols: text("spamdb_db_enabled", lols) + "\n\n" + text("spamdb_db_lols_warning", nil),
		},
		{
			name: "chat switch off", chatOn: false, operatorOn: true,
			wantCAS:  text("spamdb_db_enabled_idle", cas),
			wantLols: text("spamdb_db_enabled_idle", lols) + "\n\n" + text("spamdb_db_lols_warning_idle", nil),
		},
		{
			name: "operator switch off", chatOn: true, operatorOn: false,
			wantCAS:  text("spamdb_db_enabled_idle", cas),
			wantLols: text("spamdb_db_enabled_idle", lols) + "\n\n" + text("spamdb_db_lols_warning_idle", nil),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, _ := withSpamDBCAS(t, tc.operatorOn, http.StatusOK, spamDBCASNotFound)
			client := aispamAdminClient()
			bot := newModuleTestBot(client)
			chat := spamDBChat(t, tc.chatOn, spamdbsettings.ActionBan)
			admin := gotgbot.User{Id: 42, FirstName: "Admin"}

			// CAS is on by default, so switch lols on first, then CAS off and on.
			spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb db lols on")
			spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb db cas off")
			spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb db cas on")

			replies := spamDBMessagesTo(client, chat.Id)
			if len(replies) != 3 {
				t.Fatalf("replies = %d, want 3", len(replies))
			}
			if replies[0] != tc.wantLols {
				t.Errorf("db lols on replied %q, want %q", replies[0], tc.wantLols)
			}
			if replies[2] != tc.wantCAS {
				t.Errorf("db cas on replied %q, want %q", replies[2], tc.wantCAS)
			}
		})
	}
}

// The status lists every database with its state, not only the selected ones.
func TestSpamDBStatusNamesTheChatsDatabases(t *testing.T) {
	useEnglishLocale(t)
	previousConfig := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = previousConfig })
	config.AppConfig.EnableReputation = true

	dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Status Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	tr := i18n.MustNewTranslator("en")
	state := func(key, name string) string {
		t.Helper()
		text, _ := tr.GetString(key, i18n.TranslationParams{"database": name})
		if text == "" {
			t.Fatalf("locale key %s is missing", key)
		}
		return text
	}

	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb")
	if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseLols, true); err != nil {
		t.Fatalf("SetSpamDBDatabase(lols) error = %v", err)
	}
	if err := spamdbsettings.SetSpamDBDatabase(chat.Id, spamdbsettings.DatabaseShared, true); err != nil {
		t.Fatalf("SetSpamDBDatabase(shared) error = %v", err)
	}
	spamDBCommand(t, dispatcher, bot, chat, admin, "/spamdb")

	replies := spamDBMessagesTo(client, chat.Id)
	if len(replies) != 2 {
		t.Fatalf("replies = %d, want 2", len(replies))
	}
	for _, want := range []string{
		state("spamdb_status_db_on", "CAS"),
		state("spamdb_status_db_off", "lols.bot"),
		state("spamdb_status_db_off", "Shared bans"),
	} {
		if !strings.Contains(replies[0], want) {
			t.Fatalf("CAS-only status %q does not contain %q", replies[0], want)
		}
	}
	for _, want := range []string{
		state("spamdb_status_db_on", "CAS"),
		state("spamdb_status_db_on", "lols.bot"),
		state("spamdb_status_db_on", "Shared bans"),
	} {
		if !strings.Contains(replies[1], want) {
			t.Fatalf("all-on status %q does not contain %q", replies[1], want)
		}
	}

	sharedNotice, _ := tr.GetString("spamdb_shared_data_notice")
	if sharedNotice == "" {
		t.Fatal("spamdb_shared_data_notice is missing from en.yml")
	}
	if strings.Contains(replies[0], sharedNotice) {
		t.Fatalf("status mentions the shared data notice while the source is off: %q", replies[0])
	}
	if !strings.Contains(replies[1], sharedNotice) {
		t.Fatalf("status with Shared bans on does not explain it sends nothing: %q", replies[1])
	}
}

// A chat that selected Shared bans must not read it as running while the
// operator has the shared reputation network switched off.
func TestSpamDBStatusMarksSharedInertWhenReputationDisabled(t *testing.T) {
	cases := []struct {
		name         string
		reputationOn bool
		wantKey      string
		unwantedKey  string
	}{
		{"operator switch off", false, "spamdb_status_db_on_inactive", "spamdb_status_db_on"},
		{"operator switch on", true, "spamdb_status_db_on", "spamdb_status_db_on_inactive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useEnglishLocale(t)
			previousConfig := *config.AppConfig
			t.Cleanup(func() { *config.AppConfig = previousConfig })
			config.AppConfig.EnableReputation = tc.reputationOn

			dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASNotFound)
			client := aispamAdminClient()
			bot := newModuleTestBot(client)
			chat := spamDBSharedOnlyChat(t, spamdbsettings.ActionBan)

			tr := i18n.MustNewTranslator("en")
			params := i18n.TranslationParams{"database": "Shared bans"}
			want, _ := tr.GetString(tc.wantKey, params)
			if want == "" {
				t.Fatalf("locale key %s is missing from en.yml", tc.wantKey)
			}
			unwanted, _ := tr.GetString(tc.unwantedKey, params)
			if unwanted == "" {
				t.Fatalf("locale key %s is missing from en.yml", tc.unwantedKey)
			}

			spamDBCommand(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Admin"}, "/spamdb")

			replies := spamDBMessagesTo(client, chat.Id)
			if len(replies) != 1 {
				t.Fatalf("replies = %d, want 1", len(replies))
			}
			if !strings.Contains(replies[0], want) {
				t.Fatalf("status %q does not contain %q", replies[0], want)
			}
			if strings.Contains(replies[0], unwanted) {
				t.Fatalf("status %q contains %q", replies[0], unwanted)
			}
		})
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
