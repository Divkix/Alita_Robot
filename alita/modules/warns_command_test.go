package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

func newWarnReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	target gotgbot.User,
	text string,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 202,
		Date:      1,
		Chat:      chat,
		From:      &target,
		Text:      "message being warned",
	}
	return ctx
}

// From PM the commands act on the connected group, so the admin checks must
// run there too, not on the PM.
func TestWarnSettingsCommandsFromPMActOnConnectedGroup(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, admin := f.bot, f.chat, f.admin
	pm := f.connectAdmin(t)

	if err := warnsModule.setWarnLimit(bot, newModuleMessageContext(bot, pm, admin, "/setwarnlimit 7")); err != ext.EndGroups {
		t.Fatalf("setWarnLimit from PM error = %v, want EndGroups", err)
	}
	if got := warns.GetWarnSetting(chat.Id).WarnLimit; got != 7 {
		t.Fatalf("connected group warn limit = %d, want 7", got)
	}

	if err := warnsModule.setWarnMode(bot, newModuleMessageContext(bot, pm, admin, "/setwarnmode kick")); err != ext.EndGroups {
		t.Fatalf("setWarnMode from PM error = %v, want EndGroups", err)
	}
	if got := warns.GetWarnSetting(chat.Id).WarnMode; got != "kick" {
		t.Fatalf("connected group warn mode = %q, want kick", got)
	}
}

func TestWarnSettingsCommandsUpdateAndDisplay(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	limitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 5")
	if err := warnsModule.setWarnLimit(bot, limitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit() error = %v, want EndGroups", err)
	}
	if got := warns.GetWarnSetting(chat.Id).WarnLimit; got != 5 {
		t.Fatalf("warn limit = %d, want 5", got)
	}

	invalidCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 0")
	if err := warnsModule.setWarnLimit(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit invalid error = %v, want EndGroups", err)
	}
	if got := warns.GetWarnSetting(chat.Id).WarnLimit; got != 5 {
		t.Fatalf("invalid warn limit changed setting to %d", got)
	}

	modeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode ban")
	if err := warnsModule.setWarnMode(bot, modeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode() error = %v, want EndGroups", err)
	}
	if got := warns.GetWarnSetting(chat.Id).WarnMode; got != "ban" {
		t.Fatalf("warn mode = %q, want ban", got)
	}

	for _, mode := range []string{"kick", "mute"} {
		modeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode "+mode)
		if err := warnsModule.setWarnMode(bot, modeCtx); err != ext.EndGroups {
			t.Fatalf("setWarnMode(%s) error = %v, want EndGroups", mode, err)
		}
		if got := warns.GetWarnSetting(chat.Id).WarnMode; got != mode {
			t.Fatalf("warn mode = %q, want %q", got, mode)
		}
	}

	missingLimitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit")
	if err := warnsModule.setWarnLimit(bot, missingLimitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit missing error = %v, want EndGroups", err)
	}
	badLimitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit nope")
	if err := warnsModule.setWarnLimit(bot, badLimitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit bad number error = %v, want EndGroups", err)
	}
	bigLimitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 101")
	if err := warnsModule.setWarnLimit(bot, bigLimitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit range error = %v, want EndGroups", err)
	}

	missingModeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode")
	if err := warnsModule.setWarnMode(bot, missingModeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode missing error = %v, want EndGroups", err)
	}
	unknownModeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode freeze")
	if err := warnsModule.setWarnMode(bot, unknownModeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode unknown error = %v, want EndGroups", err)
	}

	displayCtx := newModuleMessageContext(bot, chat, admin, "/warnings")
	if err := warnsModule.warnings(bot, displayCtx); err != ext.EndGroups {
		t.Fatalf("warnings() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) < 4 {
		t.Fatalf("sendMessage calls = %d, want at least 4", len(calls))
	}
}

func TestWarnsCommandHandlesNoWarningsAndMissingTargets(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	noWarnsCtx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, noWarnsCtx); err != ext.EndGroups {
		t.Fatalf("warns no-warning error = %v, want EndGroups", err)
	}

	missingWarnCtx := newModuleMessageContext(bot, chat, admin, "/warn")
	if err := warnsModule.warnUser(bot, missingWarnCtx); err != ext.EndGroups {
		t.Fatalf("warn missing target error = %v, want EndGroups", err)
	}
	missingSWarnCtx := newModuleMessageContext(bot, chat, admin, "/swarn")
	if err := warnsModule.sWarnUser(bot, missingSWarnCtx); err != ext.EndGroups {
		t.Fatalf("swarn missing target error = %v, want EndGroups", err)
	}
	missingDWarnCtx := newModuleMessageContext(bot, chat, admin, "/dwarn")
	if err := warnsModule.dWarnUser(bot, missingDWarnCtx); err != ext.EndGroups {
		t.Fatalf("dwarn missing target error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) < 4 {
		t.Fatalf("sendMessage calls = %d, want replies for no warns and missing targets", len(calls))
	}
}

func TestWarnCommandsRejectChannelTargets(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "warn channel", text: "/warn -1001234567890 spam", run: warnsModule.warnUser},
		{name: "swarn channel", text: "/swarn -1001234567890", run: warnsModule.sWarnUser},
		{name: "dwarn channel", text: "/dwarn -1001234567890", run: warnsModule.dWarnUser},
		{name: "warns channel", text: "/warns -1001234567890", run: warnsModule.warns},
		{name: "remove channel", text: "/rmwarn -1001234567890", run: warnsModule.removeWarn},
		{name: "reset channel", text: "/resetwarns -1001234567890", run: warnsModule.resetWarns},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected channel targets", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for rejected channel targets", len(calls))
	}
}

func TestWarnReplyStoresReasonAndWarnsListsIt(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn too noisy")
	if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	numWarns, reasons := warns.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0].Reason != "too noisy" {
		t.Fatalf("reasons = %v, want [too noisy]", reasons)
	}
	if reasons[0].WarnedBy != admin.Id {
		t.Fatalf("warn issued by %d, want admin %d", reasons[0].WarnedBy, admin.Id)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want warn and list replies", len(calls))
	}
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "too noisy") {
		t.Fatalf("warn list text = %q, want reason", lastText)
	}
}

func TestWarnsListsCountWhenReasonsAreEmpty(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	if err := db.DB.Create(&models.Warns{
		UserId:   42,
		ChatId:   chat.Id,
		NumWarns: 2,
		Reasons:  models.StringArray{},
	}).Error; err != nil {
		t.Fatalf("create warns fixture: %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, ctx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want warning count reply", len(calls))
	}
}

func TestWarnLimitPunishesAndResetsWarnings(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := warns.SetWarnMode(chat.Id, "ban", 0); err != nil {
		t.Fatalf("SetWarnMode() error = %v", err)
	}

	warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn limit reached")
	if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "banChatMember"), targetParams(chat.Id, target.Id))
	if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
	}
}

func TestWarnLimitKickAndMuteModes(t *testing.T) {
	tests := []struct {
		mode       string
		wantMethod string
	}{
		{mode: "kick", wantMethod: "unbanChatMember"},
		{mode: "mute", wantMethod: "restrictChatMember"},
	}

	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			f := newAdminChatFixture(t)
			client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
			target := gotgbot.User{Id: 42, FirstName: "Member"}
			if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
				t.Fatalf("SetWarnLimit() error = %v", err)
			}
			if err := warns.SetWarnMode(chat.Id, tc.mode, 0); err != nil {
				t.Fatalf("SetWarnMode() error = %v", err)
			}

			warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn limit reached")
			if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
				t.Fatalf("warnUser() error = %v, want EndGroups", err)
			}
			assertParams(t, client.onlyCall(t, tc.wantMethod), targetParams(chat.Id, target.Id))
			if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 0 {
				t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
			}
		})
	}
}

func TestSetWarnModeStoresTimedModeDuration(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	setMode := func(text string) {
		t.Helper()
		if err := warnsModule.setWarnMode(bot, newModuleMessageContext(bot, chat, admin, text)); err != ext.EndGroups {
			t.Fatalf("%s error = %v, want EndGroups", text, err)
		}
	}
	assertSetting := func(text, wantMode string, wantTime int) {
		t.Helper()
		s := warns.GetWarnSetting(chat.Id)
		if s.WarnMode != wantMode || s.WarnActionTime != wantTime {
			t.Fatalf("after %q: mode=%q time=%d, want %q %d", text, s.WarnMode, s.WarnActionTime, wantMode, wantTime)
		}
	}

	setMode("/setwarnmode tban 1d")
	assertSetting("/setwarnmode tban 1d", "tban", 86400)
	setMode("/setwarnmode TMUTE 2h")
	assertSetting("/setwarnmode TMUTE 2h", "tmute", 7200)

	for _, text := range []string{"/setwarnmode tban", "/setwarnmode tban 5x", "/setwarnmode tmute 0h", "/setwarnmode tban 400d", "/setwarnmode tban 1d extra"} {
		before := len(client.callsFor("sendMessage"))
		setMode(text)
		assertSetting(text, "tmute", 7200)
		if got := len(client.callsFor("sendMessage")); got != before+1 {
			t.Fatalf("%q sent %d replies, want one error reply", text, got-before)
		}
	}

	displayCtx := newModuleMessageContext(bot, chat, admin, "/warnings")
	if err := warnsModule.warnings(bot, displayCtx); err != ext.EndGroups {
		t.Fatalf("warnings() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if text := calls[len(calls)-1].Params["text"].(string); !strings.Contains(text, "tmute 2h") {
		t.Fatalf("/warnings text = %q, want mode with duration", text)
	}

	setMode("/setwarnmode ban")
	assertSetting("/setwarnmode ban", "ban", 0)
}

func untilDateParam(t *testing.T, call moduleBotCall) int64 {
	t.Helper()
	raw, ok := call.Params["until_date"]
	if !ok {
		t.Fatalf("%s params = %#v, want until_date", call.Method, call.Params)
	}
	until, err := strconv.ParseInt(fmt.Sprint(raw), 10, 64)
	if err != nil {
		t.Fatalf("until_date = %v: %v", raw, err)
	}
	return until
}

func TestWarnLimitTimedModesPunishUntilDuration(t *testing.T) {
	tests := []struct {
		name       string
		settings   models.WarnSettings
		wantMethod string
		wantTime   int64
		wantText   string
	}{
		{name: "tban", settings: models.WarnSettings{WarnMode: "tban", WarnActionTime: 86400}, wantMethod: "banChatMember", wantTime: 86400, wantText: "1d"},
		{name: "tmute", settings: models.WarnSettings{WarnMode: "tmute", WarnActionTime: 7200}, wantMethod: "restrictChatMember", wantTime: 7200, wantText: "2h"},
		// A backup taken before warn_action_time existed restores tmute with 0:
		// that must not become a permanent mute.
		{name: "legacy zero duration", settings: models.WarnSettings{WarnMode: "tmute"}, wantMethod: "restrictChatMember", wantTime: 3600, wantText: "1h"},
		// 30 s reaches Telegram as less than 30 s after rounding and latency,
		// which it reads as forever.
		{name: "too short duration", settings: models.WarnSettings{WarnMode: "tban", WarnActionTime: 30}, wantMethod: "banChatMember", wantTime: 3600, wantText: "1h"},
		{name: "longest duration", settings: models.WarnSettings{WarnMode: "tban", WarnActionTime: 366 * 86400}, wantMethod: "banChatMember", wantTime: 366 * 86400, wantText: "366d"},
		// Telegram reads an until_date more than 366 days away as forever.
		{name: "too long duration", settings: models.WarnSettings{WarnMode: "tmute", WarnActionTime: 366*86400 + 1}, wantMethod: "restrictChatMember", wantTime: 3600, wantText: "1h"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
			target := gotgbot.User{Id: 42, FirstName: "Member"}
			settings := tc.settings
			settings.ChatId = chat.Id
			settings.WarnLimit = 1
			if err := db.DB.Create(&settings).Error; err != nil {
				t.Fatalf("create warn settings: %v", err)
			}

			before := time.Now().Unix()
			warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn limit reached")
			if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
				t.Fatalf("warnUser() error = %v, want EndGroups", err)
			}
			after := time.Now().Unix()

			call := client.onlyCall(t, tc.wantMethod)
			assertParams(t, call, targetParams(chat.Id, target.Id))
			if until := untilDateParam(t, call); until < before+tc.wantTime || until > after+tc.wantTime {
				t.Fatalf("until_date = %d, want now+%d (%d..%d)", until, tc.wantTime, before+tc.wantTime, after+tc.wantTime)
			}
			if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 0 {
				t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
			}
			replies := client.callsFor("sendMessage")
			if len(replies) == 0 || !strings.Contains(replies[len(replies)-1].Params["text"].(string), tc.wantText) {
				t.Fatalf("punishment replies = %#v, want duration %q", replies, tc.wantText)
			}
		})
	}
}

func TestWarnLimitTimedModesKeepWarnsWhenPunishmentFails(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	for _, tc := range []struct {
		mode   string
		method string
	}{
		{mode: "tban", method: "banChatMember"},
		{mode: "tmute", method: "restrictChatMember"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			f := newAdminChatFixture(t)
			bot, chat, admin := f.bot, f.chat, f.admin
			f.client.errors[tc.method] = requestErr
			target := gotgbot.User{Id: 42, FirstName: "Member"}
			if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
				t.Fatalf("SetWarnLimit() error = %v", err)
			}
			if err := warns.SetWarnMode(chat.Id, tc.mode, 3600); err != nil {
				t.Fatalf("SetWarnMode() error = %v", err)
			}

			err := warnsModule.warnUser(bot, newWarnReplyContext(bot, chat, admin, target, "/warn limit reached"))
			if !errors.Is(err, requestErr) {
				t.Fatalf("warnUser() error = %v, want request error", err)
			}
			if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 1 {
				t.Fatalf("numWarns after failed %s = %d, want warn kept", tc.mode, numWarns)
			}
		})
	}
}

func TestSilentWarnDeletesCommandAndStoresReason(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newWarnReplyContext(bot, chat, admin, target, "/swarn quiet reason")
	if err := warnsModule.sWarnUser(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sWarnUser() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": chat.Id, "message_id": ctx.EffectiveMessage.MessageId})
	numWarns, reasons := warns.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0].Reason != "quiet reason" {
		t.Fatalf("reasons = %v, want [quiet reason]", reasons)
	}
}

func TestDeleteWarnDeletesReplyAndStoresReason(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newWarnReplyContext(bot, chat, admin, target, "/dwarn remove this")
	if err := warnsModule.dWarnUser(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dWarnUser() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": chat.Id, "message_id": ctx.EffectiveMessage.ReplyToMessage.MessageId})
	numWarns, reasons := warns.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0].Reason != "remove this" {
		t.Fatalf("reasons = %v, want [remove this]", reasons)
	}
}

// /dwarn and /swarn delete a message, so like /dban and /sban they need delete
// rights: an admin who can restrict but not delete is refused before the warn
// is stored or anything is deleted.
func TestDeletingWarnsRequireDeleteRights(t *testing.T) {
	useEnglishLocale(t)
	for _, command := range []string{"/dwarn", "/swarn"} {
		for _, tt := range []struct {
			name      string
			canDelete bool
			wantWarns int
		}{
			{name: "without delete rights", canDelete: false, wantWarns: 0},
			{name: "with delete rights", canDelete: true, wantWarns: 1},
		} {
			t.Run(command+" "+tt.name, func(t *testing.T) {
				f := newAdminChatFixture(t)
				client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
				target := gotgbot.User{Id: 42, FirstName: "Member"}
				adminJSON := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},`+
					`"can_manage_chat":true,"can_restrict_members":true,"can_delete_messages":%t}`, admin.Id, tt.canDelete)
				botJSON := `{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},` + fixtureAdminRights + `}`
				client.adminsByChat[fmt.Sprint(chat.Id)] = json.RawMessage("[" + botJSON + "," + adminJSON + "]")
				client.membersByUser[fmt.Sprint(admin.Id)] = json.RawMessage(adminJSON)

				dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadWarns(dispatcher)
				ctx := newWarnReplyContext(bot, chat, admin, target, command+" spam")
				if err := dispatcher.ProcessUpdate(bot, &gotgbot.Update{UpdateId: 1, Message: ctx.EffectiveMessage}, nil); err != nil {
					t.Fatalf("ProcessUpdate(%s) error = %v", command, err)
				}

				if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != tt.wantWarns {
					t.Fatalf("%s numWarns = %d, want %d", command, numWarns, tt.wantWarns)
				}
				deletes := client.callsFor("deleteMessage")
				if tt.canDelete {
					if len(deletes) != 1 {
						t.Fatalf("%s deleteMessage calls = %d, want 1", command, len(deletes))
					}
					return
				}
				if len(deletes) != 0 {
					t.Fatalf("%s deleteMessage calls = %d, want none without delete rights", command, len(deletes))
				}
				refusal := translated(t, "chat_status_delete_cmd_error")
				for _, call := range client.callsFor("sendMessage") {
					if fmt.Sprint(call.Params["text"]) == refusal {
						return
					}
				}
				t.Fatalf("%s sent no delete-rights refusal", command)
			})
		}
	}
}

func TestRemoveWarnAndResetWarnsCommands(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, admin := f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "first"})

	removeCtx := newModuleMessageContext(bot, chat, admin, "/rmwarn 42")
	if err := warnsModule.removeWarn(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeWarn() error = %v, want EndGroups", err)
	}
	if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after remove = %d, want 0", numWarns)
	}

	_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "one"})
	_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "two"})
	resetCtx := newModuleMessageContext(bot, chat, admin, "/resetwarns 42")
	if err := warnsModule.resetWarns(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetWarns() error = %v, want EndGroups", err)
	}
	if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after reset = %d, want 0", numWarns)
	}

	removeMissingCtx := newModuleMessageContext(bot, chat, admin, "/rmwarn 43")
	if err := warnsModule.removeWarn(bot, removeMissingCtx); err != ext.EndGroups {
		t.Fatalf("removeWarn missing warning error = %v, want EndGroups", err)
	}
	resetMissingCtx := newModuleMessageContext(bot, chat, admin, "/resetwarns")
	if err := warnsModule.resetWarns(bot, resetMissingCtx); err != ext.EndGroups {
		t.Fatalf("resetWarns missing target error = %v, want EndGroups", err)
	}
}

func TestWarnCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	f := newAdminChatFixture(t)
	f.makeOwner()
	admin := f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name  string
		text  string
		setup func(t *testing.T, chat gotgbot.Chat)
		run   func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "set warn mode help reply", text: "/setwarnmode", run: warnsModule.setWarnMode},
		{name: "set warn limit help reply", text: "/setwarnlimit", run: warnsModule.setWarnLimit},
		{name: "warnings display reply", text: "/warnings", run: warnsModule.warnings},
		{name: "warn missing target reply", text: "/warn", run: warnsModule.warnUser},
		{name: "silent warn missing target reply", text: "/swarn", run: warnsModule.sWarnUser},
		{name: "delete warn missing target reply", text: "/dwarn", run: warnsModule.dWarnUser},
		{name: "warns no warnings reply", text: "/warns 42", run: warnsModule.warns},
		{name: "remove warn missing target reply", text: "/rmwarn", run: warnsModule.removeWarn},
		{name: "remove warn no warnings reply", text: "/rmwarn 42", run: warnsModule.removeWarn},
		{name: "reset warns missing target reply", text: "/resetwarns", run: warnsModule.resetWarns},
		{name: "reset warns success reply", text: "/resetwarns 42", run: warnsModule.resetWarns},
		{
			name: "reset all empty reply",
			text: "/resetallwarns",
			run:  warnsModule.resetAllWarns,
		},
		{
			name: "reset all confirmation reply",
			text: "/resetallwarns",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "first"})
			},
			run: warnsModule.resetAllWarns,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.membersByUser = f.client.membersByUser
			client.responses["getChatAdministrators"] = f.client.adminsByChat[fmt.Sprint(f.chat.Id)]
			client.errors["sendMessage"] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			if tt.setup != nil {
				tt.setup(t, chat)
			}
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestWarnThisUserPropagatesGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name         string
		method       string
		mode         string
		callWarnThis bool
	}{
		{name: "member lookup failure", method: "getChatMember", callWarnThis: true},
		{name: "warning reply failure", method: "sendMessage"},
		{name: "ban limit failure", method: "banChatMember", mode: "ban"},
		{name: "kick limit failure", method: "unbanChatMember", mode: "kick"},
		{name: "mute limit failure", method: "restrictChatMember", mode: "mute"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			if tt.mode != "" {
				if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
					t.Fatalf("SetWarnLimit() error = %v", err)
				}
				if err := warns.SetWarnMode(chat.Id, tt.mode, 0); err != nil {
					t.Fatalf("SetWarnMode() error = %v", err)
				}
			}
			ctx := newWarnReplyContext(bot, chat, admin, target, "/warn too noisy")

			var err error
			if tt.callWarnThis {
				err = warnsModule.warnThisUser(bot, ctx, target.Id, "too noisy", "warn", warnIssuer(ctx))
			} else {
				err = warnsModule.warnUser(bot, ctx)
			}
			if !errors.Is(err, requestErr) {
				t.Fatalf("warnUser() error = %v, want request error", err)
			}
		})
	}
}

func TestRmWarnButtonRemovesWarning(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "button"})
	data := encodeCallbackData("rmWarn", map[string]string{"u": "42"})

	ctx := newModuleCallbackContext(bot, chat, admin, data)
	if err := warnsModule.rmWarnButton(bot, ctx); err != ext.EndGroups {
		t.Fatalf("rmWarnButton() error = %v, want EndGroups", err)
	}
	if numWarns, _ := warns.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after callback remove = %d, want 0", numWarns)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestRmWarnButtonRejectsMalformedAndInvalidUserCallbacks(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	for _, data := range []string{"rmWarn", "rmWarn.not-a-number"} {
		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := warnsModule.rmWarnButton(bot, ctx); err != ext.EndGroups {
			t.Fatalf("rmWarnButton(%q) error = %v, want EndGroups", data, err)
		}
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want malformed and invalid answers", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want none for invalid callbacks", len(calls))
	}
}

func TestWarnCallbackHandlersPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	f := newAdminChatFixture(t)
	f.makeOwner()
	admin := f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	t.Run("remove warn edit failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.membersByUser = f.client.membersByUser
		client.responses["getChatAdministrators"] = f.client.adminsByChat[fmt.Sprint(f.chat.Id)]
		client.errors["editMessageText"] = requestErr
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
		_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "button"})
		data := encodeCallbackData("rmWarn", map[string]string{"u": "42"})
		ctx := newModuleCallbackContext(bot, chat, admin, data)

		err := warnsModule.rmWarnButton(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("rmWarnButton() error = %v, want edit request error", err)
		}
	})

	t.Run("remove warn answer failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.membersByUser = f.client.membersByUser
		client.responses["getChatAdministrators"] = f.client.adminsByChat[fmt.Sprint(f.chat.Id)]
		client.errors["answerCallbackQuery"] = requestErr
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
		_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "button"})
		data := encodeCallbackData("rmWarn", map[string]string{"u": "42"})
		ctx := newModuleCallbackContext(bot, chat, admin, data)

		err := warnsModule.rmWarnButton(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("rmWarnButton() error = %v, want answer request error", err)
		}
	})

	t.Run("reset all missing callback", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
		ctx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")

		if err := warnsModule.warnsButtonHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("warnsButtonHandler(no callback) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("answerCallbackQuery"); len(calls) != 0 {
			t.Fatalf("answerCallbackQuery calls = %d, want none without callback", len(calls))
		}
	})

	for _, tt := range []struct {
		name   string
		method string
	}{
		{name: "reset all edit failure", method: "editMessageText"},
		{name: "reset all answer failure", method: "answerCallbackQuery"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.membersByUser = f.client.membersByUser
			client.responses["getChatAdministrators"] = f.client.adminsByChat[fmt.Sprint(f.chat.Id)]
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			_, _, _ = warns.WarnUser(target.Id, chat.Id, models.WarnEntry{Reason: "first"})
			data := encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"})
			ctx := newModuleCallbackContext(bot, chat, admin, data)

			err := warnsModule.warnsButtonHandler(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("warnsButtonHandler() error = %v, want request error", err)
			}
		})
	}
}

func TestResetAllWarnsConfirmationAndCallback(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	_, _, _ = warns.WarnUser(42, chat.Id, models.WarnEntry{Reason: "first"})
	data := encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"})

	confirmCtx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")
	if err := warnsModule.resetAllWarns(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("resetAllWarns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want confirmation reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("resetAllWarns confirmation did not include reply_markup")
	}

	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := warnsModule.warnsButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler() error = %v, want EndGroups", err)
	}
	if got := warns.GetAllChatWarns(chat.Id); got != 0 {
		t.Fatalf("GetAllChatWarns() = %d, want 0 after reset all", got)
	}
}

func TestResetAllWarnsHandlesEmptyCancelAndInvalidCallbacks(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	emptyCtx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")
	if err := warnsModule.resetAllWarns(bot, emptyCtx); err != nil {
		t.Fatalf("resetAllWarns empty error = %v, want nil", err)
	}

	_, _, _ = warns.WarnUser(42, chat.Id, models.WarnEntry{Reason: "first"})
	cancelCtx := newModuleCallbackContext(bot, chat, admin, "rmAllChatWarns.no")
	if err := warnsModule.warnsButtonHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler cancel error = %v, want EndGroups", err)
	}
	if got := warns.GetAllChatWarns(chat.Id); got != 1 {
		t.Fatalf("GetAllChatWarns() = %d, want warning retained after cancel", got)
	}

	invalidCtx := newModuleCallbackContext(bot, chat, admin, "rmAllChatWarns")
	if err := warnsModule.warnsButtonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler invalid error = %v, want EndGroups", err)
	}
	unknownCtx := newModuleCallbackContext(
		bot,
		chat,
		admin,
		encodeCallbackData("rmAllChatWarns", map[string]string{"a": "crafted"}),
	)
	if err := warnsModule.warnsButtonHandler(bot, unknownCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler unknown action error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel and invalid answers", len(calls))
	}
}

func TestLoadWarnsRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadWarns(dispatcher)

	if !DefaultHelpRegistry().AbleMap[warnsModule.moduleName] {
		t.Fatal("warns help registration = false, want enabled")
	}
}

func TestWarnSettingsEscapeUserInputInHTMLReplies(t *testing.T) {
	useRealLocales(t)
	for _, tt := range []struct {
		name    string
		command string
		run     func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "setwarnmode", command: "/setwarnmode <b", run: warnsModule.setWarnMode},
		{name: "setwarnlimit", command: "/setwarnlimit a<b", run: warnsModule.setWarnLimit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			if err := tt.run(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, tt.command)); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.name, err)
			}
			call := f.client.onlyCall(t, "sendMessage")
			if mode := call.Params["parse_mode"]; mode != "HTML" {
				t.Fatalf("parse_mode = %v, want HTML", mode)
			}
			if text, _ := call.Params["text"].(string); !strings.Contains(text, "&lt;b") || strings.Contains(text, "%!") {
				t.Fatalf("reply = %q, want the input HTML-escaped", text)
			}
		})
	}
}
