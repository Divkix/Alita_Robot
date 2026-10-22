package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// newAutoMuteFixture is a report card fixture whose chat auto-mutes at
// threshold (0 leaves the default, off).
func newAutoMuteFixture(t *testing.T, threshold int) reportCardFixture {
	t.Helper()
	f := newReportCardFixture(t, true)
	if threshold != 0 {
		if err := reports.SetChatReportMuteThreshold(f.chat.Id, threshold); err != nil {
			t.Fatalf("SetChatReportMuteThreshold() error = %v", err)
		}
	}
	f.client.membersByChat = map[string]json.RawMessage{}
	return f
}

// reportMessage makes reporter report the target's message messageID.
func (f reportCardFixture) reportMessage(t *testing.T, reporter gotgbot.User, messageID int64) {
	t.Helper()
	ctx := newReportReplyContext(f.bot, f.chat, reporter, f.target, "/report spam")
	ctx.EffectiveMessage.ReplyToMessage.MessageId = messageID
	if err := reportsModule.report(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}
}

func reporterUser(id int64) gotgbot.User {
	return gotgbot.User{Id: id, FirstName: "Reporter " + strconv.FormatInt(id, 10)}
}

// targetRestrictions returns the restrictChatMember calls on the target.
func targetRestrictions(f reportCardFixture) []moduleBotCall {
	var out []moduleBotCall
	for _, call := range callsToChat(f.client, "restrictChatMember", f.chat.Id) {
		if fmt.Sprint(call.Params["user_id"]) == fmt.Sprint(f.target.Id) {
			out = append(out, call)
		}
	}
	return out
}

func callPermissions(t *testing.T, call moduleBotCall) gotgbot.ChatPermissions {
	t.Helper()
	perms, ok := call.Params["permissions"].(gotgbot.ChatPermissions)
	if !ok {
		t.Fatalf("permissions = %T, want gotgbot.ChatPermissions", call.Params["permissions"])
	}
	return perms
}

func autoMuteLogEntries(f reportCardFixture) []string {
	var out []string
	for _, call := range callsToChat(f.client, "sendMessage", reportCardChannelID) {
		if text := fmt.Sprint(call.Params["text"]); strings.Contains(text, "#REPORT_MUTE") {
			out = append(out, text)
		}
	}
	return out
}

func storedAutoMute(t *testing.T, f reportCardFixture) *models.ReportAutoMute {
	t.Helper()
	m, err := reports.GetAutoMute(f.chat.Id, f.target.Id)
	if err != nil {
		t.Fatalf("GetAutoMute() error = %v", err)
	}
	return m
}

func TestReportAutoMuteAfterDistinctReporters(t *testing.T) {
	f := newAutoMuteFixture(t, 3)

	f.reportMessage(t, reporterUser(51), 505)
	f.reportMessage(t, reporterUser(52), 506)
	if got := targetRestrictions(f); len(got) != 0 {
		t.Fatalf("restrictions after 2 of 3 reporters = %v, want none", got)
	}
	if got := autoMuteLogEntries(f); len(got) != 0 {
		t.Fatalf("log entries after 2 of 3 reporters = %v, want none", got)
	}

	before := time.Now()
	f.reportMessage(t, reporterUser(53), 505)
	mutes := targetRestrictions(f)
	if len(mutes) != 1 {
		t.Fatalf("restrictions after 3 reporters = %d, want 1", len(mutes))
	}
	if callPermissions(t, mutes[0]).CanSendMessages {
		t.Fatal("auto-mute left sending enabled")
	}
	untilUnix, err := strconv.ParseInt(fmt.Sprint(mutes[0].Params["until_date"]), 10, 64)
	if err != nil {
		t.Fatalf("until_date = %v, want a unix time", mutes[0].Params["until_date"])
	}
	until := time.Unix(untilUnix, 0)
	if until.Before(before.Add(24*time.Hour-time.Minute)) || until.After(time.Now().Add(24*time.Hour)) {
		t.Fatalf("until_date = %v, want about 24h from %v", until, before)
	}
	if m := storedAutoMute(t, f); m == nil || m.MutedUntil.Unix() != untilUnix {
		t.Fatalf("stored auto-mute = %+v, want one until %d", m, untilUnix)
	}
	logs := autoMuteLogEntries(f)
	if len(logs) != 1 || !strings.Contains(logs[0], "Reports: 3") || !strings.Contains(logs[0], strconv.FormatInt(f.target.Id, 10)) {
		t.Fatalf("auto-mute log entries = %q, want one naming the target with 3 reports", logs)
	}
	if got := chatModerationEvents(t, f.chat.Id); len(got) != 0 {
		t.Fatalf("moderation events = %+v, want none for an automated mute", got)
	}

	// Further reports do not mute again or extend the mute.
	f.reportMessage(t, reporterUser(54), 506)
	if got := targetRestrictions(f); len(got) != 1 {
		t.Fatalf("restrictions after a 4th reporter = %d, want still 1", len(got))
	}
}

func TestReportAutoMuteCountsEachReporterOnce(t *testing.T) {
	f := newAutoMuteFixture(t, 2)
	reporter := reporterUser(51)
	f.reportMessage(t, reporter, 505)
	f.reportMessage(t, reporter, 506)
	f.reportMessage(t, reporter, 505)
	if got := targetRestrictions(f); len(got) != 0 {
		t.Fatalf("restrictions after one reporter = %v, want none", got)
	}
	if m := storedAutoMute(t, f); m != nil {
		t.Fatalf("stored auto-mute = %+v, want none", m)
	}
}

func TestReportAutoMuteOffByDefault(t *testing.T) {
	f := newAutoMuteFixture(t, 0)
	for id := int64(51); id <= 55; id++ {
		f.reportMessage(t, reporterUser(id), 505)
	}
	if got := targetRestrictions(f); len(got) != 0 {
		t.Fatalf("restrictions with auto-mute off = %v, want none", got)
	}
	if got := autoMuteLogEntries(f); len(got) != 0 {
		t.Fatalf("log entries with auto-mute off = %v, want none", got)
	}
}

func TestReportAutoMuteSkipsProtectedTargets(t *testing.T) {
	t.Run("approved", func(t *testing.T) {
		f := newAutoMuteFixture(t, 2)
		if err := approvals.AddApprovedUser(f.chat.Id, f.target.Id, reportCardAdminID, ""); err != nil {
			t.Fatalf("AddApprovedUser() error = %v", err)
		}
		f.reportMessage(t, reporterUser(51), 505)
		f.reportMessage(t, reporterUser(52), 505)
		if got := targetRestrictions(f); len(got) != 0 {
			t.Fatalf("restrictions on an approved user = %v, want none", got)
		}
	})
	t.Run("admin", func(t *testing.T) {
		f := newAutoMuteFixture(t, 2)
		// Not in the cached admin list, so the reports are stored; the live
		// status check before muting still finds the admin.
		f.client.membersByChat[fmt.Sprintf("%d|%d", f.chat.Id, f.target.Id)] = json.RawMessage(
			`{"status":"administrator","user":{"id":43,"is_bot":false,"first_name":"Target"},"can_restrict_members":true}`)
		f.reportMessage(t, reporterUser(51), 505)
		f.reportMessage(t, reporterUser(52), 505)
		if got := targetRestrictions(f); len(got) != 0 {
			t.Fatalf("restrictions on an admin = %v, want none", got)
		}
		if m := storedAutoMute(t, f); m != nil {
			t.Fatalf("stored auto-mute = %+v, want none for an admin", m)
		}
	})
	t.Run("admin-muted member", func(t *testing.T) {
		f := newAutoMuteFixture(t, 2)
		f.client.membersByChat[fmt.Sprintf("%d|%d", f.chat.Id, f.target.Id)] = json.RawMessage(
			`{"status":"restricted","user":{"id":43,"is_bot":false,"first_name":"Target"},"is_member":true,"can_send_messages":false,"until_date":0}`)
		f.reportMessage(t, reporterUser(51), 505)
		f.reportMessage(t, reporterUser(52), 505)
		if got := targetRestrictions(f); len(got) != 0 {
			t.Fatalf("restrictions replacing an admin's mute = %v, want none", got)
		}
	})
}

// autoMutedCard reports message 505 by two members in a chat that
// auto-mutes at 2 and returns the card's actions and the mute's until_date.
func autoMutedCard(t *testing.T) (reportCardFixture, map[string]string, int64) {
	t.Helper()
	f := newAutoMuteFixture(t, 2)
	f.reportMessage(t, reporterUser(51), 505)
	cards := callsToChat(f.client, "sendMessage", reportCardChannelID)
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	actions := cardActions(t, cards[0].Params["reply_markup"])
	f.reportMessage(t, reporterUser(52), 505)
	m := storedAutoMute(t, f)
	if m == nil {
		t.Fatal("target was not auto-muted")
	}
	return f, actions, m.MutedUntil.Unix()
}

func TestReportDismissLiftsOnlyTheAutoMute(t *testing.T) {
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Admin"}
	memberKey := func(f reportCardFixture) string { return fmt.Sprintf("%d|%d", f.chat.Id, f.target.Id) }

	t.Run("auto-mute lifted", func(t *testing.T) {
		f, actions, until := autoMutedCard(t)
		f.client.membersByChat[memberKey(f)] = json.RawMessage(fmt.Sprintf(
			`{"status":"restricted","user":{"id":43,"is_bot":false,"first_name":"Target"},"is_member":true,"can_send_messages":false,"until_date":%d}`, until))
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["dismiss"]); err != ext.EndGroups {
			t.Fatalf("dismiss press error = %v, want EndGroups", err)
		}
		calls := targetRestrictions(f)
		if len(calls) != 2 {
			t.Fatalf("restrictions = %d, want the auto-mute and its lift", len(calls))
		}
		if !callPermissions(t, calls[1]).CanSendMessages {
			t.Fatal("dismiss did not restore sending")
		}
		if m := storedAutoMute(t, f); m != nil {
			t.Fatalf("auto-mute record = %+v after dismiss, want none", m)
		}
	})

	t.Run("admin's own mute kept", func(t *testing.T) {
		f, actions, _ := autoMutedCard(t)
		// An admin muted the user for good after the auto-mute.
		f.client.membersByChat[memberKey(f)] = json.RawMessage(
			`{"status":"restricted","user":{"id":43,"is_bot":false,"first_name":"Target"},"is_member":true,"can_send_messages":false,"until_date":0}`)
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["dismiss"]); err != ext.EndGroups {
			t.Fatalf("dismiss press error = %v, want EndGroups", err)
		}
		if calls := targetRestrictions(f); len(calls) != 1 {
			t.Fatalf("restrictions = %d, want only the auto-mute", len(calls))
		}
		if m := storedAutoMute(t, f); m != nil {
			t.Fatalf("auto-mute record = %+v after dismiss, want none", m)
		}
	})

	t.Run("no auto-mute", func(t *testing.T) {
		f := newAutoMuteFixture(t, 0)
		f.reportMessage(t, reporterUser(51), 505)
		actions := cardActions(t, callsToChat(f.client, "sendMessage", reportCardChannelID)[0].Params["reply_markup"])
		f.client.membersByChat[memberKey(f)] = json.RawMessage(
			`{"status":"restricted","user":{"id":43,"is_bot":false,"first_name":"Target"},"is_member":true,"can_send_messages":false,"until_date":0}`)
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["dismiss"]); err != ext.EndGroups {
			t.Fatalf("dismiss press error = %v, want EndGroups", err)
		}
		if calls := targetRestrictions(f); len(calls) != 0 {
			t.Fatalf("restrictions = %v, want none without an auto-mute", calls)
		}
	})

	t.Run("card mute keeps its mute", func(t *testing.T) {
		f, actions, _ := autoMutedCard(t)
		seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
			Status: "administrator", User: admin, CanRestrictMembers: true,
		})
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["mute"]); err != ext.EndGroups {
			t.Fatalf("mute press error = %v, want EndGroups", err)
		}
		calls := targetRestrictions(f)
		if len(calls) != 2 || callPermissions(t, calls[1]).CanSendMessages {
			t.Fatalf("restrictions = %v, want the auto-mute and the card's mute", calls)
		}
		if _, ok := calls[1].Params["until_date"]; ok {
			t.Fatalf("card mute until_date = %v, want a mute without end as before", calls[1].Params["until_date"])
		}
		if m := storedAutoMute(t, f); m != nil {
			t.Fatalf("auto-mute record = %+v after the card's mute, want none", m)
		}
	})
}

func TestReportsMuteSetting(t *testing.T) {
	f := newAdminChatFixture(t)
	run := func(from gotgbot.User, text string) {
		t.Helper()
		ctx := newModuleMessageContext(f.bot, f.chat, from, text)
		if err := reportsModule.reports(f.bot, ctx); err != ext.EndGroups {
			t.Fatalf("%s error = %v, want EndGroups", text, err)
		}
	}
	threshold := func() int { return reports.GetChatReportSettings(f.chat.Id).MuteThreshold }
	// An admin without the right to restrict, listed before the first
	// check caches the admin list.
	limited := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Limited"}
	limitedJSON := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Limited"},"can_manage_chat":true,"can_delete_messages":true}`, limited.Id)
	f.addAdmins(limitedJSON)
	f.client.membersByUser[fmt.Sprint(limited.Id)] = json.RawMessage(limitedJSON)

	if got := threshold(); got != 0 {
		t.Fatalf("default threshold = %d, want 0 (off)", got)
	}
	run(f.member, "/reports mute 3")
	if got := threshold(); got != 0 {
		t.Fatalf("threshold after a member's /reports mute 3 = %d, want 0", got)
	}
	run(limited, "/reports mute")
	status, _ := i18n.MustNewTranslator("en").GetString("reports_automute_status_off")
	if last := sentTexts(f.client); len(last) == 0 || !strings.Contains(last[len(last)-1], status) {
		t.Fatalf("/reports mute by an admin who cannot restrict = %q, want the status", last)
	}
	run(limited, "/reports mute 3")
	if got := threshold(); got != 0 {
		t.Fatalf("threshold after /reports mute 3 by an admin who cannot restrict = %d, want 0", got)
	}
	run(f.admin, "/reports mute 3")
	if got := threshold(); got != 3 {
		t.Fatalf("threshold = %d, want 3", got)
	}
	for _, text := range []string{"/reports mute 1", "/reports mute 21", "/reports mute many", "/reports mute 3 4", "/reports mute -2"} {
		run(f.admin, text)
		if got := threshold(); got != 3 {
			t.Fatalf("threshold after %q = %d, want 3 kept", text, got)
		}
	}

	tr := i18n.MustNewTranslator("en")
	usage, _ := tr.GetString("reports_automute_usage", i18n.TranslationParams{
		"min": models.ReportMuteThresholdMin, "max": models.ReportMuteThresholdMax,
	})
	if last := sentTexts(f.client); len(last) == 0 || last[len(last)-1] != usage {
		t.Fatalf("reply to an invalid threshold = %q, want the usage", last)
	}

	run(f.admin, "/reports")
	status, _ = tr.GetString("reports_automute_status_on", i18n.TranslationParams{"count": 3})
	if last := sentTexts(f.client); !strings.Contains(last[len(last)-1], status) {
		t.Fatalf("/reports reply = %q, want the auto-mute status", last[len(last)-1])
	}

	run(f.admin, "/reports mute off")
	if got := threshold(); got != 0 {
		t.Fatalf("threshold after off = %d, want 0", got)
	}
	if !reports.GetChatReportSettings(f.chat.Id).Status {
		t.Fatal("/reports mute off turned reports off")
	}
}

func TestReportsMuteSettingInSummary(t *testing.T) {
	chatID := uniqueModuleChatID()
	row := func() settingsRow {
		t.Helper()
		section := reportsSettingsSection(t.Context(), chatID)
		for _, r := range section.Rows {
			if r.Command == "/reports mute" {
				return r
			}
		}
		t.Fatalf("reports section rows = %+v, want a /reports mute row", section.Rows)
		return settingsRow{}
	}
	if r := row(); r.Value != "off" || r.Changed {
		t.Fatalf("default row = %+v, want off and unchanged", r)
	}
	if err := reports.SetChatReportMuteThreshold(chatID, 4); err != nil {
		t.Fatalf("SetChatReportMuteThreshold() error = %v", err)
	}
	if r := row(); r.Value != "4" || !r.Changed {
		t.Fatalf("row = %+v, want 4 and changed", r)
	}
}
