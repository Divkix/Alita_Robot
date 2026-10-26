//go:build testtools

package modules

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamfilter"
	"github.com/divkix/Alita_Robot/alita/utils/spamtext"
)

const spamFilterLogChannel = int64(-1009876543299)

const spamFilterSpamText = "Earn 500 USDT a day from home, no experience needed! Write to @easy_money_bot now"

// spamFilterReset clears the process copy of the samples, which outlives a
// test, and puts the dry-run clock back.
func spamFilterReset(t *testing.T) {
	t.Helper()
	spamCorpusForgetAll()
	previous := spamFilterNow
	t.Cleanup(func() {
		spamFilterNow = previous
		spamCorpusForgetAll()
	})
}

func spamFilterSamples(t *testing.T, chatID int64, label string) []models.SpamSample {
	t.Helper()
	samples, err := spamfilter.List(context.Background(), chatID)
	if err != nil {
		t.Fatalf("spamfilter.List(%d) error = %v", chatID, err)
	}
	var out []models.SpamSample
	for _, s := range samples {
		if s.Label == label {
			out = append(out, s)
		}
	}
	return out
}

func spamFilterAddSample(t *testing.T, chatID, senderID int64, text string) *models.SpamSample {
	t.Helper()
	sample, err := spamfilter.Add(spamfilter.NewSample{
		ChatID: chatID, Label: models.SpamLabelSpam, Text: text, SenderID: senderID, AddedBy: 1, OwnerID: 1,
	})
	if err != nil {
		t.Fatalf("spamfilter.Add() error = %v", err)
	}
	spamCorpusForget(chatID)
	return sample
}

func spamFilterSetMode(t *testing.T, chatID int64, mode string) {
	t.Helper()
	if err := spamfilter.SetMode(chatID, mode); err != nil {
		t.Fatalf("spamfilter.SetMode(%q) error = %v", mode, err)
	}
}

func spamFilterWithLogChannel(t *testing.T, chat gotgbot.Chat) {
	t.Helper()
	if err := logchannels.Set(chat.Id, chat.Title, spamFilterLogChannel); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
}

// spamFilterRun sends one member message through the AI-spam watcher, which
// runs the filter first, with a provider that answers "not spam" and counts
// its requests.
func spamFilterRun(t *testing.T, client *moduleBotClient, chat gotgbot.Chat, messageID int64, text string) (requests int64) {
	t.Helper()
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)
	var calls atomic.Int64
	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.1, "none"))
	})

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)
	aispamProcessUpdate(t, dispatcher, newModuleTestBot(client), chat, gotgbot.User{Id: 42, FirstName: "Spammer"}, messageID, text)
	DrainAISpamChecks()
	return calls.Load()
}

var htmlTag = regexp.MustCompile(`<[^>]+>`)

// spamFilterLogEntry returns the one entry sent to the log channel as
// Telegram shows it, without markup, and the data of its button.
func spamFilterLogEntry(t *testing.T, client *moduleBotClient) (string, string) {
	t.Helper()
	var entries []moduleBotCall
	for _, call := range client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(spamFilterLogChannel) {
			entries = append(entries, call)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("log channel entries = %d, want 1", len(entries))
	}
	text := html.UnescapeString(htmlTag.ReplaceAllString(fmt.Sprint(entries[0].Params["text"]), ""))
	markup, ok := entries[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("log entry reply_markup = %#v, want one button", entries[0].Params["reply_markup"])
	}
	return text, markup.InlineKeyboard[0][0].CallbackData
}

// spamCommandHandled reports a CommandContext handler's success: it returns
// nil, which WrapCommand turns into EndGroups, or EndGroups after a refusal.
func spamCommandHandled(err error) bool {
	return err == nil || err == ext.EndGroups
}

func spamFilterLogEntries(client *moduleBotClient) int {
	return len(spamDBMessagesTo(client, spamFilterLogChannel))
}

//
// /spam
//

func TestSpamCommandDeletesBansRecordsAndStoresSample(t *testing.T) {
	spamFilterReset(t)
	f := newAdminChatFixture(t)
	spammer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Spammer"}

	ctx := newBanReplyContext(f.bot, f.chat, f.admin, spammer, "/spam")
	ctx.EffectiveMessage.ReplyToMessage.Text = spamFilterSpamText
	if err := anonPipelineHandler(spamDesc, spamFilterModule.spamCommand)(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("/spam error = %v, want EndGroups", err)
	}

	assertParams(t, f.client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": f.chat.Id, "message_id": 303})
	assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, spammer.Id))
	if reply := f.client.onlyCall(t, "sendMessage"); reply.Params["reply_markup"] == nil {
		t.Fatal("/spam reply has no unban button")
	}

	events := chatModerationEvents(t, f.chat.Id)
	if len(events) != 1 || events[0].Action != models.ModerationActionBan || events[0].TargetUserId != spammer.Id {
		t.Fatalf("moderation events = %+v, want one ban of %d", events, spammer.Id)
	}

	samples := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam)
	if len(samples) != 1 {
		t.Fatalf("spam samples = %d, want 1", len(samples))
	}
	// List leaves the admin columns out; Get reads the whole row.
	got, err := spamfilter.Get(samples[0].ID)
	if err != nil {
		t.Fatalf("spamfilter.Get(%d) error = %v", samples[0].ID, err)
	}
	if got.Text != spamFilterSpamText || got.SenderID != spammer.Id || got.AddedBy != f.admin.Id {
		t.Fatalf("sample = %+v, want the replied text from %d added by %d", got, spammer.Id, f.admin.Id)
	}
	ids := strings.Split(got.AdminIDs, ",")
	if !slices.Contains(ids, fmt.Sprint(f.admin.Id)) || slices.Contains(ids, fmt.Sprint(f.bot.Id)) {
		t.Fatalf("sample admin_ids = %q, want the human admin %d and not the bot", got.AdminIDs, f.admin.Id)
	}
}

func TestSpamCommandStillBansWhenTheTextIsTooShortToStore(t *testing.T) {
	spamFilterReset(t)
	f := newAdminChatFixture(t)
	spammer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Spammer"}

	ctx := newBanReplyContext(f.bot, f.chat, f.admin, spammer, "/spam")
	ctx.EffectiveMessage.ReplyToMessage.Text = "hi"
	if err := anonPipelineHandler(spamDesc, spamFilterModule.spamCommand)(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("/spam error = %v, want EndGroups", err)
	}
	f.client.onlyCall(t, "banChatMember")
	f.client.onlyCall(t, "sendMessage")
	if samples := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam); len(samples) != 0 {
		t.Fatalf("spam samples = %d, want none for a two-letter text", len(samples))
	}
}

func TestSpamCommandRefusals(t *testing.T) {
	spamFilterReset(t)
	f := newAdminChatFixture(t)
	spammer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Spammer"}
	run := anonPipelineHandler(spamDesc, spamFilterModule.spamCommand)

	// Without a reply there is nothing to delete or learn.
	if err := run(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/spam")); err != ext.EndGroups {
		t.Fatalf("/spam without reply error = %v, want EndGroups", err)
	}
	f.client.onlyCall(t, "sendMessage")

	// A member cannot use it.
	ctx := newBanReplyContext(f.bot, f.chat, f.member, spammer, "/spam")
	ctx.EffectiveMessage.ReplyToMessage.Text = spamFilterSpamText
	if err := run(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("/spam by member error = %v, want EndGroups", err)
	}

	// Nor can it target an admin.
	ctx = newBanReplyContext(f.bot, f.chat, f.admin, f.admin, "/spam")
	ctx.EffectiveMessage.ReplyToMessage.Text = spamFilterSpamText
	if err := run(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("/spam on admin error = %v, want EndGroups", err)
	}

	for _, method := range []string{"banChatMember", "deleteMessage"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Fatalf("%s calls = %d, want none", method, len(calls))
		}
	}
	if samples := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam); len(samples) != 0 {
		t.Fatalf("spam samples = %d, want none", len(samples))
	}
}

//
// Matching before the AI check
//

func TestSpamFilterOffLeavesTheAICheckAlone(t *testing.T) {
	spamFilterReset(t)
	client := newModuleBotClient()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Off"}
	spamFilterWithLogChannel(t, chat)
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}
	spamFilterAddSample(t, chat.Id, 7, spamFilterSpamText)

	if requests := spamFilterRun(t, client, chat, 901, spamFilterSpamText); requests != 1 {
		t.Fatalf("provider requests = %d, want 1", requests)
	}
	if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0", len(deletes))
	}
	if n := spamFilterLogEntries(client); n != 0 {
		t.Fatalf("log channel entries = %d, want 0", n)
	}
}

func TestSpamFilterDryRunLogsTheMatchAndKeepsTheMessage(t *testing.T) {
	for _, mode := range []string{models.SpamFilterDryRun, models.SpamFilterOn} {
		t.Run(mode, func(t *testing.T) {
			spamFilterReset(t)
			client := newModuleBotClient()
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Dry Run"}
			spamFilterWithLogChannel(t, chat)
			if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
				t.Fatalf("SetAISpamEnabled() error = %v", err)
			}
			sample := spamFilterAddSample(t, chat.Id, 7, spamFilterSpamText)
			spamFilterSetMode(t, chat.Id, mode)

			// A near copy: another handle, emoji, look-alike letters.
			variant := "Еarn 500 USDT a day from home 💰 no experience needed!! Write to @easy_money_bot2 now"
			if requests := spamFilterRun(t, client, chat, 902, variant); requests != 1 {
				t.Fatalf("provider requests = %d, want 1: a dry run must not skip the AI check", requests)
			}
			if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
				t.Fatalf("deleteMessage calls = %d, want 0 in the dry run", len(deletes))
			}
			entry, data := spamFilterLogEntry(t, client)
			for _, want := range []string{"#" + spamFilterTag, fmt.Sprintf("#%d (this chat)", sample.ID), "dry run, message kept"} {
				if !strings.Contains(entry, want) {
					t.Fatalf("log entry %q does not contain %q", entry, want)
				}
			}
			if !strings.HasPrefix(data, spamHamNamespace+"|") {
				t.Fatalf("button data = %q, want the %s namespace", data, spamHamNamespace)
			}
		})
	}
}

func TestSpamFilterOnDeletesTheMatchAndSkipsTheAICheck(t *testing.T) {
	spamFilterReset(t)
	client := newModuleBotClient()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter On"}
	spamFilterWithLogChannel(t, chat)
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}
	spamFilterAddSample(t, chat.Id, 7, spamFilterSpamText)
	spamFilterSetMode(t, chat.Id, models.SpamFilterOn)
	spamFilterNow = func() time.Time { return time.Now().Add(spamfilter.DryRunPeriod + time.Hour) }

	if requests := spamFilterRun(t, client, chat, 903, spamFilterSpamText+"!!"); requests != 0 {
		t.Fatalf("provider requests = %d, want 0 for a message the filter deleted", requests)
	}
	assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": chat.Id, "message_id": 903})
	if entry, _ := spamFilterLogEntry(t, client); !strings.Contains(entry, "message deleted") {
		t.Fatalf("log entry %q does not say the message was deleted", entry)
	}
}

// The filter works without the AI check: a chat that never turned /aispam on,
// on a bot with no provider key, still has its matches deleted.
func TestSpamFilterWorksWithoutTheAICheck(t *testing.T) {
	spamFilterReset(t)
	client := newModuleBotClient()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "No AI"}
	spamFilterAddSample(t, chat.Id, 7, spamFilterSpamText)
	spamFilterSetMode(t, chat.Id, models.SpamFilterOn)
	spamFilterNow = func() time.Time { return time.Now().Add(spamfilter.DryRunPeriod + time.Hour) }

	resetAISpamRuntime()
	withAISpamTestConfig(t, "", false, 0)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)
	aispamProcessUpdate(t, dispatcher, newModuleTestBot(client), chat, gotgbot.User{Id: 42, FirstName: "Spammer"}, 904, spamFilterSpamText)

	assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": chat.Id, "message_id": 904})
}

func TestSpamFilterIgnoresUnlikeMessagesAndHam(t *testing.T) {
	spamFilterReset(t)
	client := newModuleBotClient()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ham"}
	spamFilterWithLogChannel(t, chat)
	spamFilterAddSample(t, chat.Id, 7, spamFilterSpamText)
	spamFilterSetMode(t, chat.Id, models.SpamFilterOn)
	spamFilterNow = func() time.Time { return time.Now().Add(spamfilter.DryRunPeriod + time.Hour) }

	spamFilterRun(t, client, chat, 905, "does anyone know when the next meetup is happening this month?")
	if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0 for an unrelated message", len(deletes))
	}

	// The chat said a close variant is fine: a message nearer that ham than
	// the spam is kept, though it is near enough the spam to match it.
	hamText := spamFilterSpamText + " says our sponsor"
	if s := spamtext.Similarity(spamtext.Vectorize(spamtext.Normalize(hamText)), spamtext.Vectorize(spamtext.Normalize(spamFilterSpamText))); s < models.DefaultSpamFilterThreshold {
		t.Fatalf("variant similarity = %.2f, want at least %.2f for the test to mean anything", s, models.DefaultSpamFilterThreshold)
	}
	if err := spamfilter.MarkHam(chat.Id, 0, hamText, 1); err != nil {
		t.Fatalf("MarkHam() error = %v", err)
	}
	spamCorpusForget(chat.Id)
	spamFilterRun(t, client, chat, 906, hamText)
	if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0 for a text the chat marked not spam", len(deletes))
	}
}

func TestSpamFilterSkipsAdminsApprovedUsersAndOtherChatsSamples(t *testing.T) {
	spamFilterReset(t)
	spamFilterNow = func() time.Time { return time.Now().Add(spamfilter.DryRunPeriod + time.Hour) }
	newChat := func(withSample bool) gotgbot.Chat {
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Skips"}
		if withSample {
			spamFilterAddSample(t, chat.Id, 7, spamFilterSpamText)
		}
		spamFilterSetMode(t, chat.Id, models.SpamFilterOn)
		return chat
	}

	// 42 is an admin in aispamAdminClient's list.
	admins := aispamAdminClient()
	spamFilterRun(t, admins, newChat(true), 907, spamFilterSpamText)

	approved := newModuleBotClient()
	approvedChat := newChat(true)
	if err := approvals.AddApprovedUser(approvedChat.Id, 42, 1, "test"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	spamFilterRun(t, approved, approvedChat, 908, spamFilterSpamText)

	// One chat's sample does not match in another.
	elsewhere := newModuleBotClient()
	newChat(true)
	spamFilterRun(t, elsewhere, newChat(false), 909, spamFilterSpamText)

	for name, client := range map[string]*moduleBotClient{"admin": admins, "approved": approved, "other chat": elsewhere} {
		if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
			t.Fatalf("%s: deleteMessage calls = %d, want 0", name, len(deletes))
		}
	}
}

//
// The "not spam" button
//

func spamHamPress(bot *gotgbot.Bot, presser gotgbot.User, entry, data string) *ext.Context {
	logChat := gotgbot.Chat{Id: spamFilterLogChannel, Type: "channel", Title: "Log"}
	query := &gotgbot.CallbackQuery{
		Id:   "spamham-1",
		From: presser,
		Message: gotgbot.Message{
			MessageId: 77,
			Date:      1,
			Chat:      logChat,
			Text:      entry,
		},
		Data:         data,
		ChatInstance: "test-chat-instance",
	}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 3, CallbackQuery: query}, nil)
}

func TestSpamHamButtonChecksThePresserAgainstTheGroup(t *testing.T) {
	spamFilterReset(t)
	f := newAdminChatFixture(t)
	spamFilterWithLogChannel(t, f.chat)
	sample := spamFilterAddSample(t, f.chat.Id, 7, spamFilterSpamText)
	spamFilterSetMode(t, f.chat.Id, models.SpamFilterDryRun)

	spamFilterRun(t, f.client, f.chat, 910, spamFilterSpamText)
	entry, data := spamFilterLogEntry(t, f.client)

	// A forged button naming another group does nothing.
	forged := encodeCallbackData(spamHamNamespace, map[string]string{"c": fmt.Sprint(uniqueModuleChatID()), "s": fmt.Sprint(sample.ID)})
	if err := spamFilterModule.spamHamButton(f.bot, spamHamPress(f.bot, f.admin, entry, forged)); err != ext.EndGroups {
		t.Fatalf("forged press error = %v, want EndGroups", err)
	}
	// A member of the group cannot press it.
	if err := spamFilterModule.spamHamButton(f.bot, spamHamPress(f.bot, f.member, entry, data)); err != ext.EndGroups {
		t.Fatalf("member press error = %v, want EndGroups", err)
	}
	if spam := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam); len(spam) != 1 {
		t.Fatalf("spam samples = %d after refused presses, want 1", len(spam))
	}
	if calls := f.client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d after refused presses, want 0", len(calls))
	}

	if err := spamFilterModule.spamHamButton(f.bot, spamHamPress(f.bot, f.admin, entry, data)); err != ext.EndGroups {
		t.Fatalf("admin press error = %v, want EndGroups", err)
	}
	if spam := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam); len(spam) != 0 {
		t.Fatalf("spam samples = %d after not spam, want 0", len(spam))
	}
	ham := spamFilterSamples(t, f.chat.Id, models.SpamLabelHam)
	if len(ham) != 1 || ham[0].Text != spamFilterSpamText {
		t.Fatalf("ham samples = %+v, want the logged message", ham)
	}
	f.client.onlyCall(t, "editMessageText")
}

//
// Unban
//

func TestUnbanTurnsTheSendersSamplesIntoHam(t *testing.T) {
	for _, via := range []string{"command", "button"} {
		t.Run(via, func(t *testing.T) {
			spamFilterReset(t)
			f := newAdminChatFixture(t)
			spammer := uniquePositiveUserID()
			spamFilterAddSample(t, f.chat.Id, spammer, spamFilterSpamText)
			spamFilterAddSample(t, f.chat.Id, 7, "Another unrelated advert for cheap followers at @followers_shop")

			var err error
			if via == "command" {
				err = bansModule.unban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, fmt.Sprintf("/unban %d", spammer)))
			} else {
				data := encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(spammer)})
				err = bansModule.unrestrictButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data))
			}
			if err != ext.EndGroups {
				t.Fatalf("unban error = %v, want EndGroups", err)
			}
			f.client.onlyCall(t, "unbanChatMember")

			spam := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam)
			if len(spam) != 1 || spam[0].SenderID != 7 {
				t.Fatalf("spam samples = %+v, want only the other sender's", spam)
			}
			ham := spamFilterSamples(t, f.chat.Id, models.SpamLabelHam)
			if len(ham) != 1 || ham[0].SenderID != spammer {
				t.Fatalf("ham samples = %+v, want the unbanned sender's", ham)
			}
		})
	}
}

//
// /spamfilter, /spamsamples, /forgetspam
//

func TestSpamFilterCommandSetsModeAndThreshold(t *testing.T) {
	useRealLocales(t)
	spamFilterReset(t)
	f := newAdminChatFixture(t)
	run := anonCommandContextHandler(spamFilterDesc, spamFilterModule.spamFilter)
	send := func(text string) {
		t.Helper()
		if err := run(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text)); !spamCommandHandled(err) {
			t.Fatalf("%s error = %v, want it handled", text, err)
		}
	}
	settings := func() *models.SpamFilterSettings {
		t.Helper()
		s, err := spamfilter.GetSettings(context.Background(), f.chat.Id)
		if err != nil {
			t.Fatalf("GetSettings() error = %v", err)
		}
		return s
	}

	send("/spamfilter dryrun")
	if got := settings(); got.Mode != models.SpamFilterDryRun || got.EnabledAt == nil {
		t.Fatalf("settings after dryrun = %+v, want dryrun with a start", got)
	}
	send("/spamfilter threshold 90%")
	if got := settings().Threshold; got != 0.9 {
		t.Fatalf("threshold = %v, want 0.9", got)
	}
	send("/spamfilter threshold 0.3")
	send("/spamfilter sometimes")
	if got := settings(); got.Threshold != 0.9 || got.Mode != models.SpamFilterDryRun {
		t.Fatalf("settings after invalid input = %+v, want them unchanged", got)
	}
	send("/spamfilter off")
	if got := settings(); got.Mode != models.SpamFilterOff || got.EnabledAt != nil {
		t.Fatalf("settings after off = %+v, want off", got)
	}
	if calls := f.client.callsFor("sendMessage"); len(calls) != 5 {
		t.Fatalf("sendMessage calls = %d, want one reply per command", len(calls))
	}

	// A member cannot change it.
	if err := run(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "/spamfilter on")); !spamCommandHandled(err) {
		t.Fatalf("member /spamfilter error = %v, want it handled", err)
	}
	if got := settings().Mode; got != models.SpamFilterOff {
		t.Fatalf("mode after member /spamfilter on = %q, want off", got)
	}
}

func TestSpamSamplesListsAndForgetSpamDeletes(t *testing.T) {
	useRealLocales(t)
	spamFilterReset(t)
	f := newAdminChatFixture(t)
	other := uniqueModuleChatID()
	mine := spamFilterAddSample(t, f.chat.Id, 7, "Join <b>VIP</b> signals & get 300% profit at @vip_signals_channel")
	theirs := spamFilterAddSample(t, other, 7, spamFilterSpamText)

	list := anonCommandContextHandler(spamSamplesDesc, spamFilterModule.spamSamples)
	if err := list(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/spamsamples")); !spamCommandHandled(err) {
		t.Fatalf("/spamsamples error = %v, want it handled", err)
	}
	listing := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
	if !strings.Contains(listing, fmt.Sprintf("#%d", mine.ID)) || !strings.Contains(listing, "&lt;b&gt;VIP&lt;/b&gt;") {
		t.Fatalf("/spamsamples = %q, want sample #%d with its text escaped", listing, mine.ID)
	}

	forget := anonCommandContextHandler(forgetSpamDesc, spamFilterModule.forgetSpam)
	for _, text := range []string{fmt.Sprintf("/forgetspam %d", theirs.ID), "/forgetspam x", fmt.Sprintf("/forgetspam #%d", mine.ID)} {
		if err := forget(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text)); !spamCommandHandled(err) {
			t.Fatalf("%s error = %v, want it handled", text, err)
		}
	}
	if spam := spamFilterSamples(t, f.chat.Id, models.SpamLabelSpam); len(spam) != 0 {
		t.Fatalf("chat samples = %d after /forgetspam, want 0", len(spam))
	}
	if spam := spamFilterSamples(t, other, models.SpamLabelSpam); len(spam) != 1 {
		t.Fatalf("other chat's samples = %d, want its sample kept", len(spam))
	}
}
