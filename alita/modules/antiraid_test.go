package modules

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantSec int
		wantOk  bool
	}{
		{"minutes", "30m", 30 * 60, true},
		{"hours", "2h", 2 * 60 * 60, true},
		{"days", "1d", 24 * 60 * 60, true},
		{"weeks", "1w", 7 * 24 * 60 * 60, true},
		{"raw seconds", "3600", 3600, true},
		{"bare single digit", "5", 5, true},
		{"bare 30 seconds", "30", 30, true},
		{"s suffix", "30s", 30, true},
		{"5m suffix", "5m", 300, true},
		{"2h suffix", "2h", 7200, true},
		{"1d suffix", "1d", 86400, true},
		{"1w suffix", "1w", 604800, true},
		{"empty", "", 0, false},
		{"garbage", "abc", 0, false},
		{"unknown unit", "30x", 0, false},
		{"negative bare", "-5", 0, false},
		{"negative minutes", "-5m", 0, false},
		{"uppercase", "1H", 3600, true},
		{"whitespace", "  5m  ", 5 * 60, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseDuration(tc.input)
			if got != tc.wantSec || ok != tc.wantOk {
				t.Errorf("parseDuration(%q) = (%d, %v), want (%d, %v)", tc.input, got, ok, tc.wantSec, tc.wantOk)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    int
		expected string
	}{
		{60, "1m"},
		{3600, "1h"},
		{86400, "1d"},
		{604800, "1w"},
		{30, "30s"},
		{7200, "2h"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			got := formatDuration(tc.input)
			if got != tc.expected {
				t.Errorf("formatDuration(%d) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestAntiRaidKeysAndNoCacheFallbacks(t *testing.T) {
	withNilCacheMarshal(t)

	chatID := int64(-1001234567890)
	if got := stateKey(chatID); got != "alita:antiraid:state:-1001234567890" {
		t.Fatalf("stateKey() = %q", got)
	}
	if got := joinsKey(chatID); got != "alita:antiraid:joins:-1001234567890" {
		t.Fatalf("joinsKey() = %q", got)
	}

	count, err := trackJoin(chatID, 42)
	if err == nil {
		t.Fatal("trackJoin() error = nil, want cache not initialized")
	}
	if count != 0 {
		t.Fatalf("trackJoin() count = %d, want 0", count)
	}

	clearJoinTracking(chatID)

	state := getRaidState(chatID)
	if state == nil {
		t.Fatal("getRaidState() = nil, want inactive state")
	}
	if state.Active {
		t.Fatalf("getRaidState() Active = true, want false")
	}

	if err := setRaidState(chatID, &raidState{Active: true}); err == nil {
		t.Fatal("setRaidState() error = nil, want cache not initialized")
	}
}

func TestStopAntiRaidExpiryPollerCancelsExistingContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	antiRaidCtx = ctx
	antiRaidCancel = cancel
	t.Cleanup(func() {
		antiRaidCancel = nil
		antiRaidCtx = nil
	})

	StopAntiRaidExpiryPoller()
	if antiRaidCancel != nil {
		t.Fatal("antiRaidCancel was not cleared")
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("anti-raid context was not cancelled")
	}
}

func TestStartAntiRaidExpiryPollerSkipsWhenRedisUnavailable(t *testing.T) {
	antiRaidCancel = nil
	antiRaidCtx = nil
	restoreRedis := cache.DisableRedisForTest()
	t.Cleanup(func() {
		restoreRedis()
		StopAntiRaidExpiryPoller()
		antiRaidCtx = nil
	})

	StartAntiRaidExpiryPoller()
	if antiRaidCancel != nil {
		t.Fatal("StartAntiRaidExpiryPoller created cancel func without Redis")
	}
}

func TestAntiRaidPollerReturnsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		antiRaidModule.expiryPoller(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expiryPoller did not return after context cancellation")
	}
}

func TestAntiRaidCheckExpiredRaidsNoRedisIsNoop(t *testing.T) {
	antiRaidModule.checkExpiredRaids(context.Background())
}

func TestAntiRaidStateMachine(t *testing.T) {
	if cache.GetMarshal() == nil {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano()

	// Initial state
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid to be inactive initially")
	}

	// Enable
	antiRaidModule.enableRaid(chatID, 3600)
	if !antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid to be active after enable")
	}

	// Disable
	if !antiRaidModule.disableRaid(chatID) {
		t.Fatal("expected disableRaid to return true for active raid")
	}
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid to be inactive after disable")
	}

	// Disable when already disabled
	if antiRaidModule.disableRaid(chatID) {
		t.Fatal("expected disableRaid to return false for already-inactive raid")
	}
}

func TestAntiRaidAutoExpiry(t *testing.T) {
	if cache.GetMarshal() == nil {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano() + 1

	antiRaidModule.enableRaid(chatID, 1) // 1 second
	if !antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid active immediately")
	}

	time.Sleep(2 * time.Second)
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid expired after 1s duration")
	}
}

func TestAntiRaidExtend(t *testing.T) {
	if cache.GetMarshal() == nil {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano() + 2

	antiRaidModule.enableRaid(chatID, 3600)
	st := getRaidState(chatID)
	originalExpiry := st.ExpiresAt

	time.Sleep(100 * time.Millisecond)
	st.ExpiresAt = time.Now().Unix() + 7200
	if err := setRaidState(chatID, st); err != nil {
		t.Fatalf("setRaidState failed: %v", err)
	}

	st2 := getRaidState(chatID)
	if st2.ExpiresAt <= originalExpiry {
		t.Fatalf("expected extended expiry > original, got %d vs %d", st2.ExpiresAt, originalExpiry)
	}

	antiRaidModule.disableRaid(chatID)
}

func TestAntiRaidExpiredStoredStateIsMarkedInactive(t *testing.T) {
	if cache.GetMarshal() == nil {
		t.Skip("requires cache marshal")
	}

	chatID := time.Now().UnixNano() + 3
	if err := setRaidState(chatID, &raidState{
		Active:    true,
		StartedAt: time.Now().Add(-2 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	}); err != nil {
		t.Fatalf("setRaidState() error = %v", err)
	}

	st := getRaidState(chatID)
	if st.Active {
		t.Fatalf("getRaidState() Active = true for expired state: %+v", st)
	}
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("isRaidActive() = true for expired stored state")
	}
	if antiRaidModule.disableRaid(chatID) {
		t.Fatal("disableRaid() = true for already expired inactive state")
	}
}

func TestAntiRaidCommandShowsStatusAndTogglesState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		antiRaidModule.disableRaid(chat.Id)
	})

	statusCtx := newModuleMessageContext(bot, chat, user, "/antiraid")
	if err := antiRaidModule.antiraid(bot, statusCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(status) error = %v, want EndGroups", err)
	}

	onCtx := newModuleMessageContext(bot, chat, user, "/antiraid on")
	if err := antiRaidModule.antiraid(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(on) error = %v, want EndGroups", err)
	}
	if !antiRaidModule.isRaidActive(chat.Id) {
		t.Fatal("raid was not activated by /antiraid on")
	}

	durationCtx := newModuleMessageContext(bot, chat, user, "/antiraid 45m")
	if err := antiRaidModule.antiraid(bot, durationCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(duration) error = %v, want EndGroups", err)
	}
	if st := getRaidState(chat.Id); st.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("duration update produced expired state: %+v", st)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/antiraid off")
	if err := antiRaidModule.antiraid(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(off) error = %v, want EndGroups", err)
	}
	if antiRaidModule.isRaidActive(chat.Id) {
		t.Fatal("raid stayed active after /antiraid off")
	}
}

func TestAntiRaidCommandHandlesInvalidAndNoopBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		antiRaidModule.disableRaid(chat.Id)
	})

	offInactiveCtx := newModuleMessageContext(bot, chat, user, "/antiraid off")
	if err := antiRaidModule.antiraid(bot, offInactiveCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(off inactive) error = %v, want EndGroups", err)
	}

	invalidCtx := newModuleMessageContext(bot, chat, user, "/antiraid nope")
	if err := antiRaidModule.antiraid(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(invalid duration) error = %v, want EndGroups", err)
	}

	onCtx := newModuleMessageContext(bot, chat, user, "/antiraid on")
	if err := antiRaidModule.antiraid(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(on) error = %v, want EndGroups", err)
	}
	onAgainCtx := newModuleMessageContext(bot, chat, user, "/antiraid on")
	if err := antiRaidModule.antiraid(bot, onAgainCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(on already active) error = %v, want EndGroups", err)
	}

	activeStatusCtx := newModuleMessageContext(bot, chat, user, "/antiraid")
	if err := antiRaidModule.antiraid(bot, activeStatusCtx); err != ext.EndGroups {
		t.Fatalf("antiraid(active status) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) < 5 {
		t.Fatalf("sendMessage calls = %d, want command replies", len(calls))
	}
}

func TestAntiRaidTimeCommandsPersistSettings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	raidTimeCtx := newModuleMessageContext(bot, chat, user, "/raidtime 2h")
	if err := antiRaidModule.raidTime(bot, raidTimeCtx); err != ext.EndGroups {
		t.Fatalf("raidTime() error = %v, want EndGroups", err)
	}
	if got := antiraid.GetAntiRaidSettings(chat.Id).RaidTime; got != 2*60*60 {
		t.Fatalf("RaidTime = %d, want 7200", got)
	}

	actionTimeCtx := newModuleMessageContext(bot, chat, user, "/raidactiontime 30m")
	if err := antiRaidModule.raidActionTime(bot, actionTimeCtx); err != ext.EndGroups {
		t.Fatalf("raidActionTime() error = %v, want EndGroups", err)
	}
	if got := antiraid.GetAntiRaidSettings(chat.Id).RaidActionTime; got != 30*60 {
		t.Fatalf("RaidActionTime = %d, want 1800", got)
	}
}

func TestAntiRaidTimeCommandsValidateInputAndNoChange(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, text := range []string{
		"/raidtime",
		"/raidtime nope",
		"/raidtime 0",
		"/raidactiontime",
		"/raidactiontime nope",
		"/raidactiontime 0",
	} {
		ctx := newModuleMessageContext(bot, chat, user, text)
		if err := antiRaidModule.raidTimeSetter(bot, ctx, strings.HasPrefix(text, "/raidtime")); err != ext.EndGroups {
			t.Fatalf("raidTimeSetter(%q) error = %v, want EndGroups", text, err)
		}
	}

	if err := antiraid.SetRaidTime(chat.Id, 60); err != nil {
		t.Fatalf("SetRaidTime setup error = %v", err)
	}
	noChangeRaidCtx := newModuleMessageContext(bot, chat, user, "/raidtime 1m")
	if err := antiRaidModule.raidTime(bot, noChangeRaidCtx); err != ext.EndGroups {
		t.Fatalf("raidTime(no change) error = %v, want EndGroups", err)
	}

	if err := antiraid.SetRaidActionTime(chat.Id, 120); err != nil {
		t.Fatalf("SetRaidActionTime setup error = %v", err)
	}
	noChangeActionCtx := newModuleMessageContext(bot, chat, user, "/raidactiontime 2m")
	if err := antiRaidModule.raidActionTime(bot, noChangeActionCtx); err != ext.EndGroups {
		t.Fatalf("raidActionTime(no change) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 8 {
		t.Fatalf("sendMessage calls = %d, want validation and no-change replies", len(calls))
	}
}

func TestAutoAntiRaidCommandPersistsThreshold(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setCtx := newModuleMessageContext(bot, chat, user, "/autoantiraid 4")
	if err := antiRaidModule.autoAntiRaid(bot, setCtx); err != ext.EndGroups {
		t.Fatalf("autoAntiRaid(set) error = %v, want EndGroups", err)
	}
	if got := antiraid.GetAntiRaidSettings(chat.Id).AutoAntiRaidThreshold; got != 4 {
		t.Fatalf("AutoAntiRaidThreshold = %d, want 4", got)
	}

	statusCtx := newModuleMessageContext(bot, chat, user, "/autoantiraid")
	if err := antiRaidModule.autoAntiRaid(bot, statusCtx); err != ext.EndGroups {
		t.Fatalf("autoAntiRaid(status) error = %v, want EndGroups", err)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/autoantiraid off")
	if err := antiRaidModule.autoAntiRaid(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("autoAntiRaid(off) error = %v, want EndGroups", err)
	}
	if got := antiraid.GetAntiRaidSettings(chat.Id).AutoAntiRaidThreshold; got != 0 {
		t.Fatalf("AutoAntiRaidThreshold = %d, want 0 after off", got)
	}
}

func TestAutoAntiRaidCommandHandlesInvalidAndNoopBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, text := range []string{
		"/autoantiraid off",
		"/autoantiraid nope",
		"/autoantiraid -1",
	} {
		ctx := newModuleMessageContext(bot, chat, user, text)
		if err := antiRaidModule.autoAntiRaid(bot, ctx); err != ext.EndGroups {
			t.Fatalf("autoAntiRaid(%q) error = %v, want EndGroups", text, err)
		}
	}

	if err := antiraid.SetAutoAntiRaidThreshold(chat.Id, 3); err != nil {
		t.Fatalf("SetAutoAntiRaidThreshold setup error = %v", err)
	}
	noChangeCtx := newModuleMessageContext(bot, chat, user, "/autoantiraid 3")
	if err := antiRaidModule.autoAntiRaid(bot, noChangeCtx); err != ext.EndGroups {
		t.Fatalf("autoAntiRaid(no change) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want validation and no-change replies", len(calls))
	}
}

func TestAntiRaidCallbackTogglesState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		antiRaidModule.disableRaid(chat.Id)
	})

	onCtx := newModuleCallbackContext(
		bot,
		chat,
		user,
		encodeCallbackData("antiraid", map[string]string{"a": "on"}, "antiraid.on"),
	)
	if err := antiRaidModule.callbackHandler(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("callbackHandler(on) error = %v, want EndGroups", err)
	}
	if !antiRaidModule.isRaidActive(chat.Id) {
		t.Fatal("raid was not activated by callback")
	}

	offCtx := newModuleCallbackContext(
		bot,
		chat,
		user,
		encodeCallbackData("antiraid", map[string]string{"a": "off"}, "antiraid.off"),
	)
	if err := antiRaidModule.callbackHandler(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("callbackHandler(off) error = %v, want EndGroups", err)
	}
	if antiRaidModule.isRaidActive(chat.Id) {
		t.Fatal("raid stayed active after callback off")
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want 2", len(calls))
	}
}

func TestAntiRaidCallbackRejectsInvalidAndUnknownActions(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		data string
		want error
	}{
		{
			name: "malformed",
			data: "antiraid.bad.extra",
			want: ext.ContinueGroups,
		},
		{
			name: "unknown action",
			data: encodeCallbackData("antiraid", map[string]string{"a": "later"}, "antiraid.later"),
			want: ext.EndGroups,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleCallbackContext(bot, chat, user, tt.data)
			if err := antiRaidModule.callbackHandler(bot, ctx); err != tt.want {
				t.Fatalf("callbackHandler(%q) error = %v, want %v", tt.data, err, tt.want)
			}
		})
	}
}

func TestAntiRaidOnJoinBansDuringActiveRaid(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	user := gotgbot.User{Id: 4249, FirstName: "Raider"}
	antiRaidModule.enableRaid(chat.Id, 3600)
	t.Cleanup(func() {
		antiRaidModule.disableRaid(chat.Id)
	})

	msg := &gotgbot.Message{
		MessageId:      202,
		Date:           1,
		Chat:           chat,
		From:           &user,
		NewChatMembers: []gotgbot.User{user},
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 202, Message: msg}, nil)
	if err := antiRaidModule.onJoin(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("onJoin() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
}

func TestAntiRaidOnJoinSkipsIneligibleUpdates(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	member := gotgbot.User{Id: 4250, FirstName: "Member"}

	noChatCtx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 301}, nil)
	if err := antiRaidModule.onJoin(bot, noChatCtx); err != ext.ContinueGroups {
		t.Fatalf("onJoin(no chat) error = %v, want ContinueGroups", err)
	}

	privateChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", Title: "Private"}
	privateCtx := newModuleMessageContext(bot, privateChat, member, "joined")
	privateCtx.EffectiveMessage.NewChatMembers = []gotgbot.User{member}
	if err := antiRaidModule.onJoin(bot, privateCtx); err != ext.ContinueGroups {
		t.Fatalf("onJoin(private) error = %v, want ContinueGroups", err)
	}

	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
	if err := approvals.AddApprovedUser(chat.Id, member.Id, 777000, "trusted"); err != nil {
		t.Fatalf("AddApprovedUser setup error = %v", err)
	}
	msg := &gotgbot.Message{
		MessageId: 302,
		Date:      1,
		Chat:      chat,
		From:      &member,
		NewChatMembers: []gotgbot.User{
			{Id: bot.Id, FirstName: "Alita"},
			member,
			{Id: 777000, FirstName: "Telegram"},
		},
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 302, Message: msg}, nil)
	if err := antiRaidModule.onJoin(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("onJoin(skip members) error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want no bans for skipped members", len(calls))
	}
}
