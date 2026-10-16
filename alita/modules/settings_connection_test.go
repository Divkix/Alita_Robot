package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

const settingsTestMemberID = 4250

// connectedPMFixture connects the test admin and a plain member to group A,
// where only the admin is an administrator.
func connectedPMFixture(t *testing.T) (client *moduleBotClient, bot *gotgbot.Bot, groupA int64, admin, member gotgbot.User) {
	t.Helper()
	client, bot, groupA, _ = settingsFixture(t)
	client.responses["getChat"] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Group A"}`, groupA))
	client.responses["getChatMember"] = json.RawMessage(fmt.Sprintf(`{"status":"member","user":{"id":%d,"is_bot":false,"first_name":"Member"}}`, settingsTestMemberID))
	admin = gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	member = gotgbot.User{Id: settingsTestMemberID, FirstName: "Member"}
	for _, u := range []gotgbot.User{admin, member} {
		if err := connections.ConnectId(u.Id, groupA); err != nil {
			t.Fatalf("ConnectId(%d) error = %v", u.Id, err)
		}
	}
	t.Cleanup(func() { connections.DisconnectId(settingsTestMemberID) })
	return client, bot, groupA, admin, member
}

func runInPM(t *testing.T, bot *gotgbot.Bot, user gotgbot.User, text string, handler func(*gotgbot.Bot, *ext.Context) error) {
	t.Helper()
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, text)
	if err := handler(bot, ctx); err != nil && err != ext.EndGroups {
		t.Fatalf("%s from PM error = %v", text, err)
	}
}

func captchaSettingsOf(t *testing.T, chatID int64) *models.CaptchaSettings {
	t.Helper()
	settings, err := captcha.GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() error = %v", err)
	}
	return settings
}

func TestCaptchaFromPMViaConnection(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)

	runInPM(t, bot, admin, "/captcha on", captchaModule.captchaCommand)
	runInPM(t, bot, admin, "/captchamode text", captchaModule.captchaModeCommand)
	runInPM(t, bot, admin, "/captchatime 5", captchaModule.captchaTimeCommand)
	runInPM(t, bot, admin, "/captchaaction ban", captchaModule.captchaActionCommand)
	runInPM(t, bot, admin, "/captchamaxattempts 4", captchaModule.captchaMaxAttemptsCommand)

	got := captchaSettingsOf(t, groupA)
	if !got.Enabled || got.CaptchaMode != "text" || got.Timeout != 5 || got.FailureAction != "ban" || got.MaxAttempts != 4 {
		t.Fatalf("group captcha settings = %+v, want enabled/text/5/ban/4", got)
	}
	assertNothingSentTo(t, client, groupA)
}

func TestCaptchaPendingCommandsFromPMViaConnection(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	targetID := int64(424243)
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(targetID, groupA, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := captcha.StoreMessageForCaptcha(targetID, groupA, attempt.ID, db.TEXT, "held in group", "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha() error = %v", err)
	}

	runInPM(t, bot, admin, "/captchapending 424243", captchaModule.viewPendingMessages)
	shown := false
	for _, call := range client.callsFor("sendMessage") {
		if strings.Contains(fmt.Sprint(call.Params["text"]), "held in group") {
			shown = true
		}
	}
	if !shown {
		t.Fatal("/captchapending from PM did not show the group's stored message")
	}

	runInPM(t, bot, admin, "/captchaclear 424243", captchaModule.clearPendingMessages)
	if messages, err := captcha.GetStoredMessagesForUser(targetID, groupA); err != nil || len(messages) != 0 {
		t.Fatalf("stored messages after clear = %d, %v; want 0, nil", len(messages), err)
	}
	assertNothingSentTo(t, client, groupA)
}

func TestCaptchaFromPMRejectsNonAdmin(t *testing.T) {
	client, bot, groupA, _, member := connectedPMFixture(t)

	runInPM(t, bot, member, "/captcha on", captchaModule.captchaCommand)
	runInPM(t, bot, member, "/captchamode text", captchaModule.captchaModeCommand)

	if got := captchaSettingsOf(t, groupA); got.Enabled || got.CaptchaMode == "text" {
		t.Fatalf("non-admin changed group captcha settings: %+v", got)
	}
	assertNothingSentTo(t, client, groupA)
}

func TestAntiraidFromPMViaConnection(t *testing.T) {
	withMiniredis(t)
	client, bot, groupA, admin, _ := connectedPMFixture(t)

	runInPM(t, bot, admin, "/raidtime 2h", antiRaidModule.raidTime)
	runInPM(t, bot, admin, "/raidactiontime 30m", antiRaidModule.raidActionTime)
	runInPM(t, bot, admin, "/autoantiraid 10", antiRaidModule.autoAntiRaid)
	runInPM(t, bot, admin, "/antiraid on", antiRaidModule.antiraid)

	got := antiraid.GetAntiRaidSettings(groupA)
	if got.RaidTime != 7200 || got.RaidActionTime != 1800 || got.AutoAntiRaidThreshold != 10 {
		t.Fatalf("group antiraid settings = %+v, want 7200/1800/10", got)
	}
	if !antiRaidModule.isRaidActive(groupA) {
		t.Fatal("/antiraid on from PM did not activate the group's raid mode")
	}
	assertNothingSentTo(t, client, groupA)
}

func TestAntiraidButtonInPMUsesConnection(t *testing.T) {
	withMiniredis(t)
	client, bot, groupA, admin, _ := connectedPMFixture(t)

	ctx := newModuleCallbackContext(bot, privateChatOf(admin), admin,
		encodeCallbackData("antiraid", map[string]string{"a": "on"}))
	if err := antiRaidModule.callbackHandler(bot, ctx); err != nil && err != ext.EndGroups {
		t.Fatalf("antiraid callback error = %v", err)
	}
	if !antiRaidModule.isRaidActive(groupA) {
		t.Fatal("antiraid button in PM did not activate the connected group's raid mode")
	}
	assertNothingSentTo(t, client, groupA)
}

func TestAntiraidFromPMRejectsNonAdmin(t *testing.T) {
	withMiniredis(t)
	_, bot, groupA, _, member := connectedPMFixture(t)

	runInPM(t, bot, member, "/raidtime 2h", antiRaidModule.raidTime)
	ctx := newModuleCallbackContext(bot, privateChatOf(member), member,
		encodeCallbackData("antiraid", map[string]string{"a": "on"}))
	_ = antiRaidModule.callbackHandler(bot, ctx)

	if got := antiraid.GetAntiRaidSettings(groupA); got.RaidTime == 7200 {
		t.Fatalf("non-admin changed group raid time: %+v", got)
	}
	if antiRaidModule.isRaidActive(groupA) {
		t.Fatal("non-admin activated the group's raid mode")
	}
}

func runCommandInPM(t *testing.T, bot *gotgbot.Bot, user gotgbot.User, text string, handler func(*helpers.CommandContext) error) {
	t.Helper()
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, text)
	c, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext(%s) error = %v", text, err)
	}
	if err := handler(c); err != nil && err != ext.EndGroups {
		t.Fatalf("%s from PM error = %v", text, err)
	}
}

func TestAISpamFromPMViaConnection(t *testing.T) {
	client, bot, groupA, admin, member := connectedPMFixture(t)
	if err := aispam.SetAISpamEnabled(groupA, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	runCommandInPM(t, bot, member, "/aispam off", aispamModule.aispam)
	if !aispam.IsAISpamEnabled(groupA) {
		t.Fatal("non-admin disabled the group's AI spam filter")
	}

	runCommandInPM(t, bot, admin, "/aispam off", aispamModule.aispam)
	if aispam.IsAISpamEnabled(groupA) {
		t.Fatal("/aispam off from PM did not change the connected group")
	}
	assertNothingSentTo(t, client, groupA)
}

func TestAnonAdminFromPMViaConnection(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	owner := gotgbot.User{Id: 4260, FirstName: "Owner"}
	client.membersByUser = map[string]json.RawMessage{
		"4260": json.RawMessage(`{"status":"creator","user":{"id":4260,"is_bot":false,"first_name":"Owner"}}`),
	}
	if err := connections.ConnectId(owner.Id, groupA); err != nil {
		t.Fatalf("ConnectId(owner) error = %v", err)
	}
	t.Cleanup(func() { connections.DisconnectId(owner.Id) })

	runCommandInPM(t, bot, admin, "/anonadmin on", adminModule.anonAdmin)
	if admin2 := readAnonAdmin(t, groupA); admin2 {
		t.Fatal("a non-owner admin enabled anonymous admin mode")
	}

	runCommandInPM(t, bot, owner, "/anonadmin on", adminModule.anonAdmin)
	if !readAnonAdmin(t, groupA) {
		t.Fatal("/anonadmin on from PM did not change the connected group")
	}
	assertNothingSentTo(t, client, groupA)
}

func readAnonAdmin(t *testing.T, chatID int64) bool {
	t.Helper()
	settings, ok := readChatSetting[models.AdminSettings](t.Context(), chatID)
	return ok && settings.AnonAdmin
}

func TestReactionsFromPMViaConnection(t *testing.T) {
	client, bot, groupA, admin, member := connectedPMFixture(t)
	client.membersByUser = map[string]json.RawMessage{
		fmt.Sprint(admin.Id): json.RawMessage(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},"can_change_info":true}`, admin.Id)),
	}

	runInPM(t, bot, member, "/addreaction hello 👍", reactionsModule.addReaction)
	if len(reactions.GetReactions(groupA)) != 0 {
		t.Fatal("non-admin added a reaction to the group")
	}

	runInPM(t, bot, admin, "/addreaction hello 👍", reactionsModule.addReaction)
	runInPM(t, bot, admin, "/addreaction bye 👋", reactionsModule.addReaction)
	if got := reactions.GetReactions(groupA); got["hello"] != "👍" {
		t.Fatalf("group reactions after add = %v, want hello→👍", got)
	}
	runInPM(t, bot, admin, "/reactions", reactionsModule.listReactions)
	runInPM(t, bot, admin, "/removereaction hello", reactionsModule.removeReaction)
	if _, ok := reactions.GetReactions(groupA)["hello"]; ok {
		t.Fatal("/removereaction from PM did not change the connected group")
	}
	runInPM(t, bot, admin, "/resetreactions", reactionsModule.resetReactions)
	if got := reactions.GetReactions(groupA); len(got) != 0 {
		t.Fatalf("group reactions after reset = %v, want none", got)
	}
	assertNothingSentTo(t, client, groupA)
}

func TestAllowConnectFromPMViaConnection(t *testing.T) {
	client, bot, groupA, admin, member := connectedPMFixture(t)

	runInPM(t, bot, member, "/allowconnect off", ConnectionsModule.allowConnect)
	if !connections.GetChatConnectionSetting(groupA).AllowConnect {
		t.Fatal("non-admin turned allowconnect off")
	}

	runInPM(t, bot, admin, "/allowconnect off", ConnectionsModule.allowConnect)
	if connections.GetChatConnectionSetting(groupA).AllowConnect {
		t.Fatal("/allowconnect off from PM did not change the connected group")
	}
	if conn := connections.Connection(admin.Id); !conn.Connected || conn.ChatId != groupA {
		t.Fatalf("admin connection after allowconnect off = %+v, want still connected", conn)
	}
	assertNothingSentTo(t, client, groupA)
}

func groupLanguageOf(t *testing.T, chatID int64) string {
	t.Helper()
	chat, ok := readChatSetting[models.Chat](t.Context(), chatID)
	if !ok {
		t.Fatalf("chat %d not found", chatID)
	}
	return chat.Language
}

func TestLangGroupFromPMChangesGroupLanguage(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)

	runInPM(t, bot, admin, "/lang group", languagesModule.changeLanguage)
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("/lang group did not send a language keyboard")
	}

	data := encodeCallbackData("change_language", map[string]string{"l": "ru", "g": "1"})
	if len(data) > 64 {
		t.Fatalf("callback data is %d bytes, want <= 64", len(data))
	}
	ctx := newModuleCallbackContext(bot, privateChatOf(admin), admin, data)
	if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("langBtnHandler error = %v, want EndGroups", err)
	}
	if got := groupLanguageOf(t, groupA); got != "ru" {
		t.Fatalf("group language = %q, want ru", got)
	}
	assertNothingSentTo(t, client, groupA)
}

func TestLangInPMWithoutGroupStillChangesUser(t *testing.T) {
	_, bot, groupA, admin, _ := connectedPMFixture(t)

	data := encodeCallbackData("change_language", map[string]string{"l": "es"})
	ctx := newModuleCallbackContext(bot, privateChatOf(admin), admin, data)
	if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("langBtnHandler error = %v, want EndGroups", err)
	}
	if got := lang.GetLanguage(ctx); got != "es" {
		t.Fatalf("user language = %q, want es", got)
	}
	if got := groupLanguageOf(t, groupA); got == "es" {
		t.Fatal("plain /lang button in PM changed the connected group's language")
	}
}

func TestLangGroupCallbackRejectsNonAdmin(t *testing.T) {
	_, bot, groupA, _, member := connectedPMFixture(t)

	data := encodeCallbackData("change_language", map[string]string{"l": "fr", "g": "1"})
	ctx := newModuleCallbackContext(bot, privateChatOf(member), member, data)
	_ = languagesModule.langBtnHandler(bot, ctx)
	if got := groupLanguageOf(t, groupA); got == "fr" {
		t.Fatal("non-admin changed the group's language")
	}
}
