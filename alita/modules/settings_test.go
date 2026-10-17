package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/models"
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
	original := settingsCandidateChats
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		return []models.Chat{{ChatId: groupA, ChatName: "Group A"}, {ChatId: groupB, ChatName: "Group B"}}, nil
	}
	t.Cleanup(func() {
		settingsCandidateChats = original
		connections.DisconnectId(settingsTestAdminID)
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
