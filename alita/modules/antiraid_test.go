package modules

import (
	"context"
	"encoding/json"
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
		encodeCallbackData("antiraid", map[string]string{"a": "on"}),
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
		encodeCallbackData("antiraid", map[string]string{"a": "off"}),
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
			data: encodeCallbackData("antiraid", map[string]string{"a": "later"}),
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

// TestAntiRaidTrackJoinTriggersAtThreshold characterizes the join-counting logic in
// trackJoin: the count must stay below T for the first T-1 joins and reach T on
// the T-th join, with no overlap between distinct chat IDs.
func TestAntiRaidTrackJoinTriggersAtThreshold(t *testing.T) {
	withMiniredis(t)

	chatID := uniqueModuleChatID()
	threshold := 3

	// Clean up join tracking state for the chat after the test.
	t.Cleanup(func() {
		clearJoinTracking(chatID)
	})

	// First T-1 joins must not meet the threshold.
	for i := 0; i < threshold-1; i++ {
		userID := int64(1000 + i)
		count, err := trackJoin(chatID, userID)
		if err != nil {
			t.Fatalf("trackJoin(%d, %d) error = %v", chatID, userID, err)
		}
		if count >= threshold {
			t.Fatalf("join %d: count = %d, want < %d (threshold not yet reached)", i+1, count, threshold)
		}
	}

	// T-th join must meet or exceed the threshold.
	count, err := trackJoin(chatID, int64(1000+threshold-1))
	if err != nil {
		t.Fatalf("trackJoin (T-th) error = %v", err)
	}
	if count < threshold {
		t.Fatalf("T-th join: count = %d, want >= %d (threshold should be reached)", count, threshold)
	}

	// A separate chat ID must have an independent counter.
	otherChatID := uniqueModuleChatID()
	t.Cleanup(func() {
		clearJoinTracking(otherChatID)
	})
	otherCount, err := trackJoin(otherChatID, 9999)
	if err != nil {
		t.Fatalf("trackJoin(other chat) error = %v", err)
	}
	if otherCount != 1 {
		t.Fatalf("other chat join count = %d, want 1 (counters are independent)", otherCount)
	}
}

// TestAntiRaidOnJoinAppliesConfiguredAction characterizes two key onJoin branches:
//  1. No action when raid is inactive and AutoAntiRaidThreshold is 0.
//  2. Ban + raid activation when the threshold is reached (via miniredis).
func TestAntiRaidOnJoinAppliesConfiguredAction(t *testing.T) {
	t.Run("no action when raid inactive and threshold disabled", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
		raider := gotgbot.User{Id: 5001, FirstName: "Raider"}

		// Ensure threshold is 0 (default) so no auto-raid logic runs.
		if err := antiraid.SetAutoAntiRaidThreshold(chat.Id, 0); err != nil {
			t.Fatalf("SetAutoAntiRaidThreshold setup error = %v", err)
		}

		msg := &gotgbot.Message{
			MessageId:      401,
			Date:           1,
			Chat:           chat,
			From:           &raider,
			NewChatMembers: []gotgbot.User{raider},
		}
		ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 401, Message: msg}, nil)
		if err := antiRaidModule.onJoin(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("onJoin(no threshold) error = %v, want ContinueGroups", err)
		}
		if calls := client.callsFor("banChatMember"); len(calls) != 0 {
			t.Fatalf("banChatMember calls = %d, want no action when threshold disabled", len(calls))
		}
	})

	t.Run("auto-triggers raid and bans joiner at threshold", func(t *testing.T) {
		withMiniredis(t)

		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Raid Chat"}
		raider := gotgbot.User{Id: 5002, FirstName: "Raider"}

		// Set threshold to 1: the very first tracked join must trigger auto-raid.
		if err := antiraid.SetAutoAntiRaidThreshold(chat.Id, 1); err != nil {
			t.Fatalf("SetAutoAntiRaidThreshold(1) error = %v", err)
		}
		t.Cleanup(func() {
			antiRaidModule.disableRaid(chat.Id)
			clearJoinTracking(chat.Id)
		})

		msg := &gotgbot.Message{
			MessageId:      402,
			Date:           1,
			Chat:           chat,
			From:           &raider,
			NewChatMembers: []gotgbot.User{raider},
		}
		ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 402, Message: msg}, nil)
		if err := antiRaidModule.onJoin(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("onJoin(auto-trigger) error = %v, want ContinueGroups", err)
		}

		// The raid must now be active.
		if !antiRaidModule.isRaidActive(chat.Id) {
			t.Fatal("isRaidActive = false after threshold reached, want true")
		}
		// The triggering joiner must have been banned.
		if calls := client.callsFor("banChatMember"); len(calls) != 1 {
			t.Fatalf("banChatMember calls = %d, want 1 (triggering joiner)", len(calls))
		}
	})
}

// TestAntiRaidCheckExpiredRaidsReleasesAfterWindow characterizes the observable
// contract of checkExpiredRaids.
//
// CHARACTERIZATION FINDING (STOP condition documented here, not fixed):
// The inner release branch of checkExpiredRaids (antiraid.go:173–178,
// `if st.Active && time.Now().Unix() > st.ExpiresAt { … }`) is unreachable dead
// code: getRaidState (antiraid.go:126–128) already coerces Active=false for any
// expired state before returning, so the `st.Active` guard is always false when
// checkExpiredRaids evaluates it.  The persist/clear sub-block (setRaidState +
// clearJoinTracking) is therefore never executed by checkExpiredRaids; expiry is
// fully handled by the read-side coercion in getRaidState.
//
// This test characterizes what checkExpiredRaids ACTUALLY does:
//  - It iterates via Redis SCAN without error.
//  - For a key whose raid is already past its ExpiresAt, the chat ends inactive
//    after the call — but that is due to getRaidState's read coercion, NOT due
//    to the checkExpiredRaids persist branch (which is never entered).
//  - For a key whose raid is genuinely still active (ExpiresAt in the future),
//    checkExpiredRaids leaves it active (the non-expired path is also a no-op).
//
// Uses miniredis for the Redis SCAN; the in-memory gocache marshal handles
// state reads/writes.
func TestAntiRaidCheckExpiredRaidsReleasesAfterWindow(t *testing.T) {
	withMiniredis(t)

	rdb := cache.GetRedisClient()

	// --- Scenario A: expired raid ---
	expiredChatID := uniqueModuleChatID()
	t.Cleanup(func() {
		antiRaidModule.disableRaid(expiredChatID)
		clearJoinTracking(expiredChatID)
		if rdb != nil {
			_ = rdb.Del(cache.Context, stateKey(expiredChatID)).Err()
		}
	})

	expiredState := &raidState{
		Active:    true,
		StartedAt: time.Now().Add(-2 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
	}
	// Persist to the gocache marshal so getRaidState can read it.
	if err := setRaidState(expiredChatID, expiredState); err != nil {
		t.Fatalf("setRaidState(expired) setup error = %v", err)
	}
	// Seed the state key into Redis so the SCAN discovers it.
	raw, err := json.Marshal(expiredState)
	if err != nil {
		t.Fatalf("json.Marshal(expiredState) error = %v", err)
	}
	if err := rdb.Set(cache.Context, stateKey(expiredChatID), raw, 24*time.Hour).Err(); err != nil {
		t.Fatalf("Redis SET expiredChatID error = %v", err)
	}

	// --- Scenario B: still-active raid ---
	activeChatID := uniqueModuleChatID()
	t.Cleanup(func() {
		antiRaidModule.disableRaid(activeChatID)
		clearJoinTracking(activeChatID)
		if rdb != nil {
			_ = rdb.Del(cache.Context, stateKey(activeChatID)).Err()
		}
	})

	activeState := &raidState{
		Active:    true,
		StartedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(), // expires 1 hour from now
	}
	if err := setRaidState(activeChatID, activeState); err != nil {
		t.Fatalf("setRaidState(active) setup error = %v", err)
	}
	rawActive, err := json.Marshal(activeState)
	if err != nil {
		t.Fatalf("json.Marshal(activeState) error = %v", err)
	}
	if err := rdb.Set(cache.Context, stateKey(activeChatID), rawActive, 24*time.Hour).Err(); err != nil {
		t.Fatalf("Redis SET activeChatID error = %v", err)
	}

	// Run the expiry check — must not return an error or panic.
	antiRaidModule.checkExpiredRaids(context.Background())

	// --- Assertions for expired scenario ---
	//
	// The expired chat must be inactive.  NOTE: this passes because getRaidState's
	// read-side coercion (antiraid.go:126–128) returns Active=false for any state
	// whose ExpiresAt is in the past.  The checkExpiredRaids persist branch
	// (antiraid.go:173–178) is never entered for this key because st.Active is
	// already false when the guard is evaluated — that is the dead-code finding.
	expiredSt := getRaidState(expiredChatID)
	if expiredSt.Active {
		t.Fatalf("getRaidState(expired).Active = true after checkExpiredRaids, want false")
	}
	if antiRaidModule.isRaidActive(expiredChatID) {
		t.Fatal("isRaidActive(expired) = true after checkExpiredRaids, want false")
	}

	// --- Assertions for still-active scenario ---
	//
	// checkExpiredRaids must leave a non-expired raid untouched (it evaluates the
	// same dead guard but the guard body is irrelevant here — the raid stays active
	// because getRaidState returns Active=true for a future ExpiresAt).
	activeSt := getRaidState(activeChatID)
	if !activeSt.Active {
		t.Fatalf("getRaidState(active).Active = false after checkExpiredRaids, want true (non-expired raid must not be released)")
	}
	if !antiRaidModule.isRaidActive(activeChatID) {
		t.Fatal("isRaidActive(active) = false after checkExpiredRaids, want true (non-expired raid must stay active)")
	}
}
