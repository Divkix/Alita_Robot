//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/i18n"
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
	t.Cleanup(func() {
		if err := connections.DisconnectId(settingsTestMemberID); err != nil {
			t.Errorf("DisconnectId(%d) error = %v", settingsTestMemberID, err)
		}
	})
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

// extraGroup adds a group with the given admin list and makes getChat answer
// with each group's own id.
func extraGroup(t *testing.T, client *moduleBotClient, admins json.RawMessage, knownGroups ...int64) (group int64) {
	t.Helper()
	group = -uniqueModuleChatID()
	if err := chats.EnsureChatInDb(group, fmt.Sprintf("Group %d", group)); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	client.adminsByChat[fmt.Sprint(group)] = admins
	if client.chatsByID == nil {
		client.chatsByID = map[string]json.RawMessage{}
	}
	for _, id := range append(knownGroups, group) {
		client.chatsByID[fmt.Sprint(id)] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Group %d"}`, id, id))
	}
	return group
}

// botlessAdminList lists userID as the only admin, without the bot.
func botlessAdminList(userID int64) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`[{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"}}]`, userID))
}

// lastButtons returns the inline keyboard of the latest sendMessage call.
func lastButtons(t *testing.T, client *moduleBotClient) []gotgbot.InlineKeyboardButton {
	t.Helper()
	calls := client.callsFor("sendMessage")
	if len(calls) == 0 {
		t.Fatal("no sendMessage call")
	}
	var buttons []gotgbot.InlineKeyboardButton
	for _, row := range markupOf(t, calls[len(calls)-1]).InlineKeyboard {
		buttons = append(buttons, row...)
	}
	if len(buttons) == 0 {
		t.Fatal("latest message has no buttons")
	}
	return buttons
}

func pressInPM(t *testing.T, bot *gotgbot.Bot, user gotgbot.User, data string, handler func(*gotgbot.Bot, *ext.Context) error) {
	t.Helper()
	ctx := newModuleCallbackContext(bot, privateChatOf(user), user, data)
	if err := handler(bot, ctx); err != nil && err != ext.EndGroups {
		t.Fatalf("callback %q error = %v", data, err)
	}
}

func TestAntiraidButtonInPMActsOnShownGroup(t *testing.T) {
	withMiniredis(t)
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	groupC := extraGroup(t, client, adminListJSON(settingsTestAdminID), groupA)

	runInPM(t, bot, admin, "/antiraid", antiRaidModule.antiraid)
	enable := lastButtons(t, client)[0].CallbackData
	if err := connections.ConnectId(admin.Id, groupC); err != nil {
		t.Fatalf("ConnectId(C) error = %v", err)
	}
	pressInPM(t, bot, admin, enable, antiRaidModule.callbackHandler)

	if antiRaidModule.isRaidActive(groupC) {
		t.Fatal("old status button enabled raid mode in the group connected later")
	}
	if !antiRaidModule.isRaidActive(groupA) {
		t.Fatal("status button did not enable raid mode in the group it showed")
	}
	assertNothingSentTo(t, client, groupA, groupC)
}

func TestAntiraidButtonInPMRejectsForgedGroup(t *testing.T) {
	withMiniredis(t)
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	groupB := extraGroup(t, client, adminListJSON(), groupA)
	groupNoBot := extraGroup(t, client, botlessAdminList(admin.Id))

	for name, target := range map[string]int64{"user not admin": groupB, "bot not admin": groupNoBot} {
		pressInPM(t, bot, admin, encodeCallbackData("antiraid",
			map[string]string{"a": "on", "c": fmt.Sprint(target)}), antiRaidModule.callbackHandler)
		if antiRaidModule.isRaidActive(target) {
			t.Fatalf("%s: forged button enabled raid mode in group %d", name, target)
		}
	}
	if antiRaidModule.isRaidActive(groupA) {
		t.Fatal("forged button enabled raid mode in the connected group")
	}
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

// lastEdit returns the latest editMessageText call.
func lastEdit(t *testing.T, client *moduleBotClient) moduleBotCall {
	t.Helper()
	edits := client.callsFor("editMessageText")
	if len(edits) == 0 {
		t.Fatal("no editMessageText call")
	}
	return edits[len(edits)-1]
}

func summaryTitle(group int64) string {
	return fmt.Sprintf("<b>Group %d</b>", group)
}

func TestSettingsRefreshInPMShowsShownGroup(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	groupC := extraGroup(t, client, adminListJSON(settingsTestAdminID), groupA)

	pressInPM(t, bot, admin, encodeCallbackData("settings",
		map[string]string{"a": "p", "c": fmt.Sprint(groupA)}), ConnectionsModule.settingsCallback)
	refresh := markupOf(t, lastEdit(t, client)).InlineKeyboard[0][0].CallbackData
	if len(refresh) > 64 {
		t.Fatalf("refresh callback data is %d bytes, want <= 64", len(refresh))
	}
	decoded, ok := decodeCallbackData(refresh, "settings")
	if !ok {
		t.Fatalf("refresh callback data %q does not decode", refresh)
	}
	if c, _ := decoded.Field("c"); c != fmt.Sprint(groupA) {
		t.Fatalf("refresh button carries c=%q, want %d", c, groupA)
	}
	if err := connections.ConnectId(admin.Id, groupC); err != nil {
		t.Fatalf("ConnectId(C) error = %v", err)
	}
	editsBefore := len(client.callsFor("editMessageText"))
	pressInPM(t, bot, admin, refresh, ConnectionsModule.settingsCallback)
	if got := len(client.callsFor("editMessageText")); got != editsBefore+1 {
		t.Fatalf("refresh press made %d edits, want 1", got-editsBefore)
	}

	text := fmt.Sprint(lastEdit(t, client).Params["text"])
	if strings.Contains(text, summaryTitle(groupC)) {
		t.Fatal("old summary's refresh showed the group connected later")
	}
	if !strings.Contains(text, summaryTitle(groupA)) {
		t.Fatalf("refresh did not show the group it was sent for: %s", text)
	}
	if conn := connections.Connection(admin.Id); conn.ChatId != groupC {
		t.Fatalf("refresh changed the connection to %d, want %d kept", conn.ChatId, groupC)
	}
	assertNothingSentTo(t, client, groupA, groupC)
}

func TestSettingsRefreshInPMRejectsForgedGroup(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	groupB := extraGroup(t, client, adminListJSON(), groupA)

	pressInPM(t, bot, admin, encodeCallbackData("settings",
		map[string]string{"a": "r", "c": fmt.Sprint(groupB)}), ConnectionsModule.settingsCallback)

	if edits := client.callsFor("editMessageText"); len(edits) != 0 {
		t.Fatalf("forged refresh rendered a summary: %v", edits[0].Params["text"])
	}
	if answers := client.callsFor("answerCallbackQuery"); len(answers) != 1 || answers[0].Params["show_alert"] != true {
		t.Fatalf("answerCallbackQuery calls = %+v, want one refusal alert", answers)
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
	t.Cleanup(func() {
		if err := connections.DisconnectId(owner.Id); err != nil {
			t.Errorf("DisconnectId(%d) error = %v", owner.Id, err)
		}
	})

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

// langButton returns the callback data of the latest keyboard's button for code.
func langButton(t *testing.T, client *moduleBotClient, code string) string {
	t.Helper()
	for _, btn := range lastButtons(t, client) {
		if decoded, ok := decodeCallbackData(btn.CallbackData, "change_language"); ok {
			if l, _ := decoded.Field("l"); l == code {
				return btn.CallbackData
			}
		}
	}
	t.Fatalf("no %s button in the language keyboard", code)
	return ""
}

func TestLangGroupFromPMChangesGroupLanguage(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	// The keyboard skips languages without a name, and tests load no locales.
	restoreI18n, err := i18n.OverrideManagerForTest("language_name: Lang\nlanguage_flag: F\n")
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	original := config.AppConfig.ValidLangCodes
	config.AppConfig.ValidLangCodes = []string{"en", "ru"}
	t.Cleanup(func() { config.AppConfig.ValidLangCodes = original })
	groupC := extraGroup(t, client, adminListJSON(settingsTestAdminID), groupA)

	runInPM(t, bot, admin, "/lang group", languagesModule.changeLanguage)
	data := langButton(t, client, "ru")
	if len(data) > 64 {
		t.Fatalf("callback data is %d bytes, want <= 64", len(data))
	}
	if err := connections.ConnectId(admin.Id, groupC); err != nil {
		t.Fatalf("ConnectId(C) error = %v", err)
	}
	pressInPM(t, bot, admin, data, languagesModule.langBtnHandler)

	if got := groupLanguageOf(t, groupC); got == "ru" {
		t.Fatal("old /lang group button changed the group connected later")
	}
	if got := groupLanguageOf(t, groupA); got != "ru" {
		t.Fatalf("group language = %q, want ru", got)
	}
	assertNothingSentTo(t, client, groupA, groupC)
}

func TestLangGroupKeyboardKeepsHelpTranslateLink(t *testing.T) {
	client, bot, _, admin, _ := connectedPMFixture(t)
	restoreI18n, err := i18n.OverrideManagerForTest("language_name: Lang\nlanguage_flag: F\nlanguage_help_translate: Help\n")
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	original := config.AppConfig.ValidLangCodes
	config.AppConfig.ValidLangCodes = []string{"en", "ru"}
	t.Cleanup(func() { config.AppConfig.ValidLangCodes = original })

	urlButtons := func() []gotgbot.InlineKeyboardButton {
		var out []gotgbot.InlineKeyboardButton
		for _, btn := range lastButtons(t, client) {
			if btn.Url != "" {
				out = append(out, btn)
			}
		}
		return out
	}

	runInPM(t, bot, admin, "/lang", languagesModule.changeLanguage)
	want := urlButtons()
	if len(want) == 0 {
		t.Fatal("plain /lang keyboard has no link button")
	}

	runInPM(t, bot, admin, "/lang group", languagesModule.changeLanguage)
	got := urlButtons()
	if len(got) != len(want) {
		t.Fatalf("/lang group link buttons = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Url != want[i].Url || got[i].Text != want[i].Text {
			t.Fatalf("/lang group link button %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	for _, btn := range lastButtons(t, client) {
		if len(btn.CallbackData) > 64 {
			t.Fatalf("callback data %q is %d bytes, want <= 64", btn.CallbackData, len(btn.CallbackData))
		}
	}
}

func TestLangGroupCallbackRejectsForgedGroup(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	groupB := extraGroup(t, client, adminListJSON(), groupA)

	pressInPM(t, bot, admin, encodeCallbackData("change_language",
		map[string]string{"l": "fr", "g": "1", "c": fmt.Sprint(groupB)}), languagesModule.langBtnHandler)
	if got := groupLanguageOf(t, groupB); got == "fr" {
		t.Fatal("forged button changed a group where the user is not an admin")
	}
	if got := groupLanguageOf(t, groupA); got == "fr" {
		t.Fatal("forged button changed the connected group's language")
	}
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

// writesTo returns the non-read Bot API calls that targeted chatID.
func writesTo(client *moduleBotClient, chatID int64) []moduleBotCall {
	client.mu.Lock()
	defer client.mu.Unlock()
	var writes []moduleBotCall
	for _, call := range client.calls {
		if !strings.HasPrefix(call.Method, "get") && fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			writes = append(writes, call)
		}
	}
	return writes
}

// The PM command handlers point ctx.EffectiveChat at the connected group, and
// gotgbot hands that ctx to every later handler group. A group reaction whose
// keyword matches the command text must not fire on the group for a command
// sent in PM, yet must keep firing for the same command sent in the group.
func TestConnectedCommandFromPMStopsLaterHandlerGroups(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	if err := reactions.AddReaction(groupA, "captcha", "👍"); err != nil {
		t.Fatalf("AddReaction() error = %v", err)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	LoadCaptcha(dispatcher)
	LoadReactions(dispatcher)
	send := func(chat gotgbot.Chat, text string) {
		t.Helper()
		update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{MessageId: 101, Date: 1, Chat: chat, From: &admin, Text: text}}
		if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
		}
	}

	for _, text := range []string{"/captcha on", "/captchapending 424243", "/captchaclear 424243"} {
		send(privateChatOf(admin), text)
		if writes := writesTo(client, groupA); len(writes) != 0 {
			t.Fatalf("%s from PM reached the group through a later handler group: %+v", text, writes)
		}
	}
	if !captchaSettingsOf(t, groupA).Enabled {
		t.Fatal("/captcha on from PM did not enable captcha in the connected group")
	}

	send(gotgbot.Chat{Id: groupA, Type: "supergroup", Title: "Group A"}, "/captcha on")
	if len(client.callsFor("setMessageReaction")) == 0 {
		t.Fatal("/captcha on sent in the group no longer reaches the group's reaction watcher")
	}
}

func TestLangGroupCallbackRejectsNonAdmin(t *testing.T) {
	_, bot, groupA, _, member := connectedPMFixture(t)

	data := encodeCallbackData("change_language", map[string]string{"l": "fr", "g": "1", "c": fmt.Sprint(groupA)})
	ctx := newModuleCallbackContext(bot, privateChatOf(member), member, data)
	_ = languagesModule.langBtnHandler(bot, ctx)
	if got := groupLanguageOf(t, groupA); got == "fr" {
		t.Fatal("non-admin changed the group's language")
	}
}
