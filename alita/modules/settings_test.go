package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

const settingsTestAdminID = 4242

func adminListJSON(userIDs ...int64) json.RawMessage {
	members := []string{`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_change_info":true,"can_restrict_members":true,"can_delete_messages":true}`}
	for _, id := range userIDs {
		members = append(members, fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},"can_change_info":true}`, id))
	}
	return json.RawMessage("[" + strings.Join(members, ",") + "]")
}

// settingsFixture builds two groups: the test admin administers A but not B.
func settingsFixture(t *testing.T) (client *moduleBotClient, bot *gotgbot.Bot, groupA, groupB int64) {
	t.Helper()
	client = newModuleBotClient()
	bot = newModuleTestBot(client)
	groupA, groupB = -uniqueModuleChatID(), -uniqueModuleChatID()
	for _, id := range []int64{groupA, groupB} {
		if err := chats.EnsureChatInDb(id, fmt.Sprintf("Group %d", id)); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}
	}
	client.adminsByChat = map[string]json.RawMessage{
		fmt.Sprint(groupA): adminListJSON(settingsTestAdminID),
		fmt.Sprint(groupB): adminListJSON(),
	}
	original, originalCooldown := settingsCandidateChats, settingsPickerCooldown
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		return []models.Chat{{ChatId: groupA, ChatName: "Group A"}, {ChatId: groupB, ChatName: "Group B"}}, nil
	}
	// Tests share the admin's user ID; only the pause test turns the pause on.
	settingsPickerCooldown = 0
	t.Cleanup(func() {
		settingsCandidateChats, settingsPickerCooldown = original, originalCooldown
		if err := connections.DisconnectId(settingsTestAdminID); err != nil {
			t.Errorf("DisconnectId(%d) error = %v", settingsTestAdminID, err)
		}
	})
	return client, bot, groupA, groupB
}

func privateChatOf(user gotgbot.User) gotgbot.Chat {
	return gotgbot.Chat{Id: user.Id, Type: "private", FirstName: user.FirstName}
}

func markupOf(t *testing.T, call moduleBotCall) gotgbot.InlineKeyboardMarkup {
	t.Helper()
	switch markup := call.Params["reply_markup"].(type) {
	case gotgbot.InlineKeyboardMarkup:
		return markup
	case nil:
		return gotgbot.InlineKeyboardMarkup{}
	default:
		t.Fatalf("reply_markup type = %T, want InlineKeyboardMarkup", markup)
		return gotgbot.InlineKeyboardMarkup{}
	}
}

func pickedChats(t *testing.T, markup gotgbot.InlineKeyboardMarkup) []string {
	t.Helper()
	var picked []string
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			decoded, ok := decodeCallbackData(button.CallbackData, "settings")
			if !ok {
				continue
			}
			if action, _ := decoded.Field("a"); action == "p" {
				chatID, _ := decoded.Field("c")
				picked = append(picked, chatID)
			}
		}
	}
	return picked
}

func assertNothingSentTo(t *testing.T, client *moduleBotClient, chatIDs ...int64) {
	t.Helper()
	for _, call := range client.callsFor("sendMessage") {
		for _, id := range chatIDs {
			if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(id) {
				t.Fatalf("sendMessage reached group %d: %v", id, call.Params["text"])
			}
		}
	}
}

func TestSettingsPMListsOnlyAdminGroups(t *testing.T) {
	client, bot, groupA, groupB := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}

	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/settings")
	if err := ConnectionsModule.settings(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settings error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if got := pickedChats(t, markupOf(t, calls[0])); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("picker chats = %v, want only %d", got, groupA)
	}
	assertNothingSentTo(t, client, groupA, groupB)
}

// adminLookups counts the Telegram calls IsUserAdmin makes on a cache miss.
func adminLookups(client *moduleBotClient) int {
	return len(client.callsFor("getChat")) + len(client.callsFor("getChatMember")) + len(client.callsFor("getChatAdministrators"))
}

func resetCalls(client *moduleBotClient) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.calls = nil
}

func setPickerCandidates(t *testing.T, ids ...int64) {
	t.Helper()
	original := settingsCandidateChats
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		out := make([]models.Chat, 0, len(ids))
		for _, id := range ids {
			out = append(out, models.Chat{ChatId: id, ChatName: fmt.Sprint(id)})
		}
		return out, nil
	}
	t.Cleanup(func() { settingsCandidateChats = original })
}

func runSettingsInPM(t *testing.T, bot *gotgbot.Bot, client *moduleBotClient, user gotgbot.User) []string {
	t.Helper()
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/settings")
	if err := ConnectionsModule.settings(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settings error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) == 0 {
		t.Fatal("no sendMessage call")
	}
	return pickedChats(t, markupOf(t, calls[len(calls)-1]))
}

func TestSettingsPickerWarmCacheNeedsNoTelegram(t *testing.T) {
	client, bot, groupA, groupB := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	cache.LoadAdminCache(bot, groupA)
	cache.LoadAdminCache(bot, groupB)
	resetCalls(client)

	if got := runSettingsInPM(t, bot, client, user); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("picker chats = %v, want only %d", got, groupA)
	}
	if n := adminLookups(client); n != 0 {
		t.Fatalf("admin lookups = %d, want 0 with warm caches", n)
	}
}

func TestSettingsPickerStopsLiveChecksAtDeadline(t *testing.T) {
	client, bot, groupA, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	cold := []int64{-uniqueModuleChatID(), -uniqueModuleChatID(), -uniqueModuleChatID()}
	for _, id := range cold {
		client.adminsByChat[fmt.Sprint(id)] = adminListJSON(settingsTestAdminID)
	}
	cache.LoadAdminCache(bot, groupA)
	// The warm group is listed after the cold ones yet still shows.
	setPickerCandidates(t, cold[0], cold[1], cold[2], groupA)
	resetCalls(client)

	// Each admin-list fetch "takes" 2 s of a 3 s budget: two cold groups fit.
	start := time.Now()
	originalNow, originalBudget := settingsPickerNow, settingsPickerLiveBudget
	settingsPickerNow = func() time.Time {
		return start.Add(time.Duration(len(client.callsFor("getChatAdministrators"))) * 2 * time.Second)
	}
	settingsPickerLiveBudget = 3 * time.Second
	t.Cleanup(func() { settingsPickerNow, settingsPickerLiveBudget = originalNow, originalBudget })

	got := runSettingsInPM(t, bot, client, user)
	want := []string{fmt.Sprint(groupA), fmt.Sprint(cold[0]), fmt.Sprint(cold[1])}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("picker chats = %v, want %v", got, want)
	}
	if n := len(client.callsFor("getChatAdministrators")); n != 2 {
		t.Fatalf("getChatAdministrators calls = %d, want 2 before the deadline", n)
	}
	for _, call := range client.calls {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(cold[2]) {
			t.Fatalf("checked %d after the deadline: %s", cold[2], call.Method)
		}
	}
}

// hungTelegramClient stalls one method for one chat like a hung Telegram: the
// call returns when released, when its context ends, or after stall.
type hungTelegramClient struct {
	*moduleBotClient
	method  string
	chatID  string
	release chan struct{}
	stall   time.Duration
}

func (c *hungTelegramClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == c.method && fmt.Sprint(params["chat_id"]) == c.chatID {
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.stall):
		}
	}
	return c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
}

// A hung Telegram call inside one group's check must not hold the picker past
// its budget, and the group it cut must be checked again on the next press.
func TestSettingsPickerBudgetCutsAHungGroupCheck(t *testing.T) {
	for _, method := range []string{"getChat", "getChatAdministrators"} {
		t.Run(method, func(t *testing.T) {
			client, bot, groupA, _ := settingsFixture(t)
			user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
			hung := -uniqueModuleChatID()
			client.adminsByChat[fmt.Sprint(hung)] = adminListJSON(settingsTestAdminID)
			cache.LoadAdminCache(bot, groupA)
			setPickerCandidates(t, hung, groupA)

			hungClient := &hungTelegramClient{
				moduleBotClient: client,
				method:          method,
				chatID:          fmt.Sprint(hung),
				release:         make(chan struct{}),
				stall:           2 * time.Second,
			}
			bot.BotClient = hungClient
			release := sync.OnceFunc(func() { close(hungClient.release) })
			t.Cleanup(release)
			originalBudget := settingsPickerLiveBudget
			settingsPickerLiveBudget = 100 * time.Millisecond
			t.Cleanup(func() { settingsPickerLiveBudget = originalBudget })

			start := time.Now()
			got := runSettingsInPM(t, bot, client, user)
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("picker answered after %v, want it near the 100ms budget", elapsed)
			}
			if strings.Join(got, ",") != fmt.Sprint(groupA) {
				t.Fatalf("picker chats = %v, want only the warm %d", got, groupA)
			}

			release()
			if method == "getChatAdministrators" {
				// The cut wait left the shared load running; it caches the list.
				deadline := time.Now().Add(3 * time.Second)
				for found, _ := cache.GetAdminCacheList(hung); !found; found, _ = cache.GetAdminCacheList(hung) {
					if time.Now().After(deadline) {
						t.Fatal("the cut admin-list load never cached its result")
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			settingsPickerLiveBudget = originalBudget
			if got := runSettingsInPM(t, bot, client, user); !slices.Contains(got, fmt.Sprint(hung)) {
				t.Fatalf("next picker chats = %v, want %d checked again", got, hung)
			}
		})
	}
}

// The connected group stays in the list when the budget cuts its check:
// tapping it re-checks the user's rights anyway.
func TestSettingsPickerKeepsConnectedGroupWhenItsCheckIsCut(t *testing.T) {
	client, bot, _, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	connected := -uniqueModuleChatID()
	client.adminsByChat[fmt.Sprint(connected)] = adminListJSON(settingsTestAdminID)
	if err := chats.EnsureChatInDb(connected, "Connected"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	if err := connections.ConnectId(user.Id, connected); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	setPickerCandidates(t)

	hungClient := &hungTelegramClient{
		moduleBotClient: client,
		method:          "getChat",
		chatID:          fmt.Sprint(connected),
		release:         make(chan struct{}),
		stall:           2 * time.Second,
	}
	bot.BotClient = hungClient
	t.Cleanup(sync.OnceFunc(func() { close(hungClient.release) }))
	originalBudget := settingsPickerLiveBudget
	settingsPickerLiveBudget = 100 * time.Millisecond
	t.Cleanup(func() { settingsPickerLiveBudget = originalBudget })

	start := time.Now()
	got := runSettingsInPM(t, bot, client, user)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("picker answered after %v, want it near the 100ms budget", elapsed)
	}
	if len(got) != 1 || got[0] != fmt.Sprint(connected) {
		t.Fatalf("picker chats = %v, want the connected %d", got, connected)
	}
}

func TestSettingsPickRechecksAdminAfterPicker(t *testing.T) {
	client, bot, groupA, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	cache.LoadAdminCache(bot, groupA)
	if got := runSettingsInPM(t, bot, client, user); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("picker chats = %v, want only %d", got, groupA)
	}

	// Demoted after the picker was shown: Telegram no longer lists the user.
	client.adminsByChat[fmt.Sprint(groupA)] = adminListJSON()
	cache.InvalidateAdminCache(groupA)

	data := encodeCallbackData("settings", map[string]string{"a": "p", "c": fmt.Sprint(groupA)})
	ctx := newModuleCallbackContext(bot, privateChatOf(user), user, data)
	if err := ConnectionsModule.settingsCallback(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settingsCallback error = %v, want EndGroups", err)
	}
	if conn := connections.Connection(user.Id); conn.Connected && conn.ChatId == groupA {
		t.Fatalf("demoted user got connected to %d", groupA)
	}
	if edits := client.callsFor("editMessageText"); len(edits) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0", len(edits))
	}
}

func TestSettingsPMNoCandidatesShowsHint(t *testing.T) {
	client, bot, _, _ := settingsFixture(t)
	user := gotgbot.User{Id: 4243, FirstName: "Nobody"}

	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/settings")
	if err := ConnectionsModule.settings(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settings error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if got := pickedChats(t, markupOf(t, calls[0])); len(got) != 0 {
		t.Fatalf("picker chats = %v, want none for a non-admin", got)
	}
}

func TestSettingsPickConnectsAdminAndShowsSummary(t *testing.T) {
	client, bot, groupA, groupB := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}

	data := encodeCallbackData("settings", map[string]string{"a": "p", "c": fmt.Sprint(groupA)})
	if len(data) > 64 {
		t.Fatalf("callback data is %d bytes, want <= 64", len(data))
	}
	ctx := newModuleCallbackContext(bot, privateChatOf(user), user, data)
	if err := ConnectionsModule.settingsCallback(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settingsCallback error = %v, want EndGroups", err)
	}

	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId != groupA {
		t.Fatalf("connection = %+v, want connected to %d", conn, groupA)
	}
	edits := client.callsFor("editMessageText")
	if len(edits) != 1 || !strings.Contains(fmt.Sprint(edits[0].Params["text"]), "<b>Captcha</b>") {
		t.Fatalf("editMessageText calls = %+v, want one summary", edits)
	}
	assertNothingSentTo(t, client, groupA, groupB)
}

func TestSettingsPickRejectsNonAdmin(t *testing.T) {
	client, bot, _, groupB := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}

	data := encodeCallbackData("settings", map[string]string{"a": "p", "c": fmt.Sprint(groupB)})
	ctx := newModuleCallbackContext(bot, privateChatOf(user), user, data)
	if err := ConnectionsModule.settingsCallback(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settingsCallback error = %v, want EndGroups", err)
	}

	if conn := connections.Connection(user.Id); conn.Connected && conn.ChatId == groupB {
		t.Fatalf("non-admin got connected to %d", groupB)
	}
	if edits := client.callsFor("editMessageText"); len(edits) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0", len(edits))
	}
	if answers := client.callsFor("answerCallbackQuery"); len(answers) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1 refusal", len(answers))
	}
}

func TestSettingsRefreshAndListButtons(t *testing.T) {
	client, bot, groupA, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	if err := connections.ConnectId(user.Id, groupA); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	client.responses["getChat"] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Group A"}`, groupA))

	refresh := newModuleCallbackContext(bot, privateChatOf(user), user,
		encodeCallbackData("settings", map[string]string{"a": "r", "c": fmt.Sprint(groupA)}))
	if err := ConnectionsModule.settingsCallback(bot, refresh); err != ext.EndGroups {
		t.Fatalf("refresh error = %v, want EndGroups", err)
	}
	list := newModuleCallbackContext(bot, privateChatOf(user), user,
		encodeCallbackData("settings", map[string]string{"a": "l"}))
	if err := ConnectionsModule.settingsCallback(bot, list); err != ext.EndGroups {
		t.Fatalf("list error = %v, want EndGroups", err)
	}

	edits := client.callsFor("editMessageText")
	if len(edits) != 2 {
		t.Fatalf("editMessageText calls = %d, want 2", len(edits))
	}
	if !strings.Contains(fmt.Sprint(edits[0].Params["text"]), "<b>Captcha</b>") {
		t.Fatalf("refresh did not render the summary: %v", edits[0].Params["text"])
	}
	if got := pickedChats(t, markupOf(t, edits[1])); len(got) == 0 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("list picker chats = %v, want connected %d first", got, groupA)
	}
}

// A forged settings button on a bot message in a group must not overwrite that
// message with the presser's picker or summary, nor connect them.
func TestSettingsCallbackOutsidePMIsRefused(t *testing.T) {
	client, bot, groupA, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	group := gotgbot.Chat{Id: groupA, Type: "supergroup", Title: "Group A"}

	for _, fields := range []map[string]string{
		{"a": "p", "c": fmt.Sprint(groupA)},
		{"a": "r", "c": fmt.Sprint(groupA)},
		{"a": "l"},
	} {
		ctx := newModuleCallbackContext(bot, group, user, encodeCallbackData("settings", fields))
		if err := ConnectionsModule.settingsCallback(bot, ctx); err != ext.EndGroups {
			t.Fatalf("settingsCallback(%v) error = %v, want EndGroups", fields, err)
		}
	}

	if edits := client.callsFor("editMessageText"); len(edits) != 0 {
		t.Fatalf("editMessageText calls = %+v, want none", edits)
	}
	if conn := connections.Connection(user.Id); conn.Connected {
		t.Fatalf("group button connected the presser: %+v", conn)
	}
	if answers := client.callsFor("answerCallbackQuery"); len(answers) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want 3 refusals", len(answers))
	}
}

func TestSettingsInGroupRepliesWithDeepLink(t *testing.T) {
	client, bot, groupA, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	group := gotgbot.Chat{Id: groupA, Type: "supergroup", Title: "Group A"}

	ctx := newModuleMessageContext(bot, group, user, "/settings")
	if err := ConnectionsModule.settings(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settings error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	markup := markupOf(t, calls[0])
	want := fmt.Sprintf("start=settings_%d", groupA)
	if len(markup.InlineKeyboard) == 0 || !strings.Contains(markup.InlineKeyboard[0][0].Url, want) {
		t.Fatalf("group reply markup = %+v, want URL with %s", markup, want)
	}
}

func TestSettingsDeepLinkConnects(t *testing.T) {
	client, bot, groupA, _ := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}

	arg := fmt.Sprintf("settings_%d", groupA)
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/start "+arg)
	if err := HandleDeepLink(bot, ctx, &user, arg); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink error = %v, want EndGroups", err)
	}

	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId != groupA {
		t.Fatalf("connection = %+v, want connected to %d", conn, groupA)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 || !strings.Contains(fmt.Sprint(calls[0].Params["text"]), "<b>Captcha</b>") {
		t.Fatalf("sendMessage calls = %+v, want one summary", calls)
	}
	assertNothingSentTo(t, client, groupA)
}
