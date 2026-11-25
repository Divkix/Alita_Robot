package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

func newPurgeReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	text string,
	messageID int64,
	replyID int64,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.MessageId = messageID
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: replyID,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 42, FirstName: "Member"},
		Text:      "message to purge",
	}
	return ctx
}

func TestPurgeMsgsConcurrentHandlesDeleteBoundariesAndErrors(t *testing.T) {
	tests := []struct {
		name       string
		pFrom      bool
		msgID      int64
		deleteTo   int64
		deleteErr  error
		wantOK     bool
		wantSends  int
		wantDelete int
	}{
		{
			name:       "empty range skips deletion when purging from marker",
			pFrom:      true,
			msgID:      10,
			deleteTo:   9,
			wantOK:     true,
			wantDelete: 0,
		},
		{
			name:       "old starting message sends explanation and continues",
			msgID:      10,
			deleteTo:   10,
			deleteErr:  fmt.Errorf("Bad Request: message can't be deleted"),
			wantOK:     true,
			wantSends:  1,
			wantDelete: 1,
		},
		{
			name:       "missing starting message is ignored",
			msgID:      10,
			deleteTo:   10,
			deleteErr:  fmt.Errorf("Bad Request: message to delete not found"),
			wantOK:     true,
			wantDelete: 1,
		},
		{
			name:       "unexpected starting delete error fails purge",
			msgID:      10,
			deleteTo:   10,
			deleteErr:  fmt.Errorf("Internal Server Error"),
			wantOK:     false,
			wantDelete: 1,
		},
		{
			name:       "large ranges use worker deletion path",
			pFrom:      true,
			msgID:      1,
			deleteTo:   12,
			wantOK:     true,
			wantDelete: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
			if tt.deleteErr != nil {
				client.errors["deleteMessage"] = tt.deleteErr
			}

			got := purgesModule.purgeMsgsConcurrent(bot, &chat, tt.pFrom, tt.msgID, tt.deleteTo)
			if got != tt.wantOK {
				t.Fatalf("purgeMsgsConcurrent() = %v, want %v", got, tt.wantOK)
			}
			if calls := client.callsFor("deleteMessage"); len(calls) != tt.wantDelete {
				t.Fatalf("deleteMessage calls = %d, want %d", len(calls), tt.wantDelete)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != tt.wantSends {
				t.Fatalf("sendMessage calls = %d, want %d", len(calls), tt.wantSends)
			}
		})
	}
}

func TestPurgeRequiresReplyAndDeletesRange(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/purge")
	if err := purgesModule.purge(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("purge no reply error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want reply-required message", len(calls))
	}

	replyCtx := newPurgeReplyContext(bot, chat, admin, "/purge cleanup", 105, 101)
	if err := purgesModule.purge(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("purge reply error = %v, want EndGroups", err)
	}
	assertDeleted(t, client, chat.Id, 101, 102, 103, 104, 105)
	if calls := client.callsFor("sendMessage"); len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want purge notification", len(calls))
	}
}

// The reason is user text inside a legacy-Markdown message; markup
// characters in it must not break or restyle the notification.
func TestPurgeReasonIsEscapedForMarkdown(t *testing.T) {
	useRealLocales(t)
	for _, cmd := range []string{"purge", "purgeto"} {
		t.Run(cmd, func(t *testing.T) {
			f := newAdminChatFixture(t)
			delMsgs.Delete(f.chat.Id)
			t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })
			handler, replyID := purgesModule.purge, int64(501)
			if cmd == "purgeto" {
				// /purgefrom marked 501; /purgeto replies to the range's last message.
				delMsgs.Store(f.chat.Id, delMsgEntry{msgID: 501})
				handler, replyID = purgesModule.purgeTo, 503
			}

			ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, "/"+cmd+" spam_bot *x* [y] `z`", 505, replyID)
			if err := handler(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s() error = %v, want EndGroups", cmd, err)
			}
			call := f.client.onlyCall(t, "sendMessage")
			assertParams(t, call, map[string]any{"parse_mode": "Markdown"})
			if text := fmt.Sprint(call.Params["text"]); !strings.Contains(text, "spam\\_bot \\*x\\* \\[y] \\`z\\`") {
				t.Fatalf("%s notification = %q, want the reason Markdown-escaped", cmd, text)
			}
		})
	}
}

func TestPurgeRejectsOverLimitRange(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	ctx := newPurgeReplyContext(bot, chat, admin, "/purge", 2005, 1)
	if err := purgesModule.purge(bot, ctx); err != ext.EndGroups {
		t.Fatalf("purge over limit error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for over-limit purge", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want limit message", len(calls))
	}
}

func TestDelCommandDeletesReplyAndCommandMessage(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/del")
	if err := purgesModule.delCmd(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("del no reply error = %v, want EndGroups", err)
	}

	replyCtx := newPurgeReplyContext(bot, chat, admin, "/del", 120, 119)
	if err := purgesModule.delCmd(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("del reply error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 2 {
		t.Fatalf("deleteMessage calls = %d, want reply and command deletions", len(calls))
	}
	assertDeleted(t, client, chat.Id, 119, 120)
}

func TestPurgeFromToDeletesMarkedRange(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	delMsgs.Delete(chat.Id)

	missingFromCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 210, 208)
	if err := purgesModule.purgeTo(bot, missingFromCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo without marker error = %v, want EndGroups", err)
	}

	fromCtx := newPurgeReplyContext(bot, chat, admin, "/purgefrom", 220, 215)
	if err := purgesModule.purgeFrom(bot, fromCtx); err != ext.EndGroups {
		t.Fatalf("purgeFrom error = %v, want EndGroups", err)
	}
	if stored, ok := delMsgs.Load(chat.Id); !ok {
		t.Fatalf("stored purge marker missing, want 215")
	} else {
		var sid int64
		switch v := stored.(type) {
		case delMsgEntry:
			sid = v.msgID
		case int64:
			sid = v
		default:
			t.Fatalf("stored purge marker type = %T, want int64 or delMsgEntry", stored)
		}
		if sid != 215 {
			t.Fatalf("stored purge marker = %v, want 215", sid)
		}
	}

	toCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto reason", 230, 218)
	if err := purgesModule.purgeTo(bot, toCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo error = %v, want EndGroups", err)
	}
	if _, ok := delMsgs.Load(chat.Id); ok {
		t.Fatal("purge marker was not cleared after purgeTo")
	}
	assertDeleted(t, client, chat.Id, 215, 216, 217, 218, 220, 230)
}

// Both marked-range commands retain their replies through anonymous proof replay.
func TestAnonymousAdminPurgeFromToDeletesMarkedRangeAfterProof(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	delMsgs.Delete(chat.Id)
	t.Cleanup(func() { delMsgs.Delete(chat.Id) })
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadPurges(dispatcher)
	LoadBotUpdates(dispatcher)

	fromCtx := newPurgeReplyContext(bot, chat, admin, "/purgefrom", 420, 415)
	dispatchAnonymousAdminCommand(t, dispatcher, f, fromCtx)
	toCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 430, 417)
	dispatchAnonymousAdminCommand(t, dispatcher, f, toCtx)
	assertDeleted(t, client, chat.Id, 415, 416, 417)
	if _, ok := delMsgs.Load(chat.Id); ok {
		t.Fatal("purge marker was not cleared after the confirmed /purgeto")
	}
}

// A second /purgefrom on another message is refused with its own message and
// keeps the first mark.
func TestPurgeFromRefusesWhileAnotherMessageIsMarked(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	delMsgs.Store(f.chat.Id, delMsgEntry{msgID: 215})
	t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })

	ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, "/purgefrom", 220, 216)
	if err := purgesModule.purgeFrom(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("purgeFrom() error = %v, want EndGroups", err)
	}
	text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
	if want := trS(i18n.MustNewTranslator("en"), "purges_mark_already_active"); want == "" || text != want {
		t.Fatalf("purgeFrom reply = %q, want purges_mark_already_active %q", text, want)
	}
	if stored, ok := delMsgs.Load(f.chat.Id); !ok || stored.(delMsgEntry).msgID != 215 {
		t.Fatalf("purge marker = %v, %v, want message 215 kept", stored, ok)
	}
}

func TestPurgeFromToValidationBranches(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	delMsgs.Delete(chat.Id)

	noFromReplyCtx := newModuleMessageContext(bot, chat, admin, "/purgefrom")
	if err := purgesModule.purgeFrom(bot, noFromReplyCtx); err != ext.EndGroups {
		t.Fatalf("purgeFrom no reply error = %v, want EndGroups", err)
	}

	noToReplyCtx := newModuleMessageContext(bot, chat, admin, "/purgeto")
	if err := purgesModule.purgeTo(bot, noToReplyCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo no reply error = %v, want EndGroups", err)
	}

	delMsgs.Store(chat.Id, int64(300))
	sameMessageCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 310, 300)
	if err := purgesModule.purgeTo(bot, sameMessageCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo same message error = %v, want EndGroups", err)
	}

	delMsgs.Store(chat.Id, int64(1))
	overLimitCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 2100, 1002)
	if err := purgesModule.purgeTo(bot, overLimitCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo over limit error = %v, want EndGroups", err)
	}
	if _, ok := delMsgs.Load(chat.Id); !ok {
		t.Fatal("purge marker was deleted for rejected over-limit purgeto")
	}
	delMsgs.Delete(chat.Id)

	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want one validation reply per branch", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no deletion for validation branches", len(calls))
	}
}

// A deletion worker whose delete panics must not take the process down: the
// panic is logged and counted as a failed deletion, the other messages are
// still deleted, and the purge is announced.
func TestPurgeSurvivesPanicInDeletionWorker(t *testing.T) {
	f := newAdminChatFixture(t)
	panicked := map[string]bool{"110": true, "120": true}
	f.bot.BotClient = &scriptedTelegramClient{
		moduleBotClient: f.client,
		chatID:          fmt.Sprint(f.chat.Id),
		script: func(method string, params map[string]any) (json.RawMessage, bool) {
			if method == "deleteMessage" && panicked[fmt.Sprint(params["message_id"])] {
				panic("delete transport exploded")
			}
			return nil, false
		},
	}

	logger := log.StandardLogger()
	previous := logger.ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { logger.ReplaceHooks(previous) })
	hook := logtest.NewLocal(logger)

	// Replying to 101 from 131 purges 101..130, past the 10-message worker threshold.
	ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, "/purge", 131, 101)
	if err := purgesModule.purge(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("purge() error = %v, want EndGroups", err)
	}

	var survivors []int64
	for id := int64(101); id <= 131; id++ {
		if !panicked[fmt.Sprint(id)] {
			survivors = append(survivors, id)
		}
	}
	assertDeleted(t, f.client, f.chat.Id, survivors...)
	if calls := f.client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want the purge notification", len(calls))
	}

	var panicLogs int
	failedCount := -1
	for _, entry := range hook.AllEntries() {
		if entry.Level <= log.ErrorLevel && strings.Contains(entry.Message, "delete transport exploded") {
			panicLogs++
		}
		if entry.Message == "Some messages could not be deleted during purge" {
			failedCount, _ = entry.Data["error_count"].(int)
		}
	}
	if panicLogs != len(panicked) {
		t.Fatalf("panic log entries = %d, want %d", panicLogs, len(panicked))
	}
	if failedCount != len(panicked) {
		t.Fatalf("failed deletions reported = %d, want %d", failedCount, len(panicked))
	}
}

// /spurge count=<n> bounds the purge to n messages starting at the reply and
// sends no confirmation, unlike /purge which announces the result.
func TestSpurgeCountBoundsPurgeWithoutConfirmation(t *testing.T) {
	f := newAdminChatFixture(t)
	delMsgs.Delete(f.chat.Id)
	t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	loadModuleByName(t, "Purges", dispatcher)

	ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, "/spurge count=5", 131, 101)
	if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatalf("/spurge dispatch error = %v", err)
	}

	// count=5 deletes the replied message and the four after it, plus the
	// command message itself: 101-105 and 131.
	assertDeleted(t, f.client, f.chat.Id, 101, 102, 103, 104, 105, 131)
	if calls := f.client.callsFor("deleteMessage"); len(calls) != 6 {
		t.Fatalf("deleteMessage calls = %d, want 6", len(calls))
	}
	if calls := f.client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want no confirmation for /spurge", len(calls))
	}
}

// A bare number after /purge is the legacy reason text, not a count: the
// whole range is deleted and the number shows up in the confirmation.
func TestPurgeNumericArgumentIsReasonNotCount(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	delMsgs.Delete(f.chat.Id)
	t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	loadModuleByName(t, "Purges", dispatcher)

	ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, "/purge 10", 131, 101)
	if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatalf("/purge dispatch error = %v", err)
	}

	// No count bound: the full 101..130 range plus the command message.
	assertDeleted(t, f.client, f.chat.Id, 102, 110, 120, 130, 131)
	if calls := f.client.callsFor("deleteMessage"); len(calls) != 31 {
		t.Fatalf("deleteMessage calls = %d, want 31 (full range)", len(calls))
	}
	call := f.client.onlyCall(t, "sendMessage")
	if text := fmt.Sprint(call.Params["text"]); !strings.Contains(text, "10") {
		t.Fatalf("confirmation = %q, want the numeric reason preserved", text)
	}
}

// A malformed count= must refuse the purge before anything is deleted: falling
// back to an unbounded legacy range would widen a destructive operation.
func TestPurgeMalformedCountDeletesNothing(t *testing.T) {
	useRealLocales(t)
	for _, text := range []string{"/purge count=abc", "/purge count=0", "/spurge count=-2"} {
		t.Run(text, func(t *testing.T) {
			f := newAdminChatFixture(t)
			delMsgs.Delete(f.chat.Id)
			t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })
			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			loadModuleByName(t, "Purges", dispatcher)

			ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, text, 131, 101)
			if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
				t.Fatalf("%s dispatch error = %v", text, err)
			}
			if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
				t.Fatalf("deleteMessage calls = %d, want none for a malformed count", len(calls))
			}
			call := f.client.onlyCall(t, "sendMessage")
			want := trS(i18n.MustNewTranslator("en"), "purges_invalid_count")
			if got := fmt.Sprint(call.Params["text"]); got != want {
				t.Fatalf("reply = %q, want %q", got, want)
			}
		})
	}
}

// A count= at the int64 ceiling must not overflow the range math: compared
// against the inclusive length first, it simply covers the whole range.
func TestPurgeMaxInt64CountPurgesFullRange(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	delMsgs.Delete(f.chat.Id)
	t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	loadModuleByName(t, "Purges", dispatcher)

	ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, "/purge count=9223372036854775807", 131, 101)
	if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatalf("/purge dispatch error = %v", err)
	}

	// The count exceeds the 101..130 range, so the whole range plus the
	// command message is deleted — same as an unbounded purge.
	assertDeleted(t, f.client, f.chat.Id, 101, 115, 130, 131)
	if calls := f.client.callsFor("deleteMessage"); len(calls) != 31 {
		t.Fatalf("deleteMessage calls = %d, want 31 (full range)", len(calls))
	}
}

// In a forum supergroup message IDs interleave across topics, so matching
// command and reply topics prove nothing about the messages between them:
// the new forms — a count=<n> bound or the silent /spurge — may run there
// only as count=1, which scopes the purge to the replied message and the
// command itself, and only when both endpoints share one named topic. Wider
// new forms are refused without deleting anything, while a legacy /purge
// without count= keeps its unscoped behavior.
func TestPurgeForumScope(t *testing.T) {
	useRealLocales(t)

	forumCtx := func(f adminChatFixture, text string, messageID, replyID, thread int64) *ext.Context {
		ctx := newPurgeReplyContext(f.bot, f.chat, f.admin, text, messageID, replyID)
		ctx.EffectiveMessage.MessageThreadId = thread
		ctx.EffectiveMessage.IsTopicMessage = true
		ctx.EffectiveMessage.ReplyToMessage.MessageThreadId = thread
		ctx.EffectiveMessage.ReplyToMessage.IsTopicMessage = true
		return ctx
	}
	newForum := func(t *testing.T) (adminChatFixture, *ext.Dispatcher) {
		f := newAdminChatFixture(t)
		f.chat.IsForum = true
		delMsgs.Delete(f.chat.Id)
		t.Cleanup(func() { delMsgs.Delete(f.chat.Id) })
		dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
		loadModuleByName(t, "Purges", dispatcher)
		return f, dispatcher
	}

	t.Run("count=1 in one named topic deletes only the endpoints", func(t *testing.T) {
		f, dispatcher := newForum(t)
		for _, text := range []string{"/purge count=1", "/spurge count=1"} {
			ctx := forumCtx(f, text, 150, 140, 7)
			if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
				t.Fatalf("%s dispatch error = %v", text, err)
			}
		}
		// Each run deletes the reply (140) and its own command; nothing between.
		assertDeleted(t, f.client, f.chat.Id, 140, 150)
		if calls := f.client.callsFor("deleteMessage"); len(calls) != 4 {
			t.Fatalf("deleteMessage calls = %d, want 4 (two endpoints per command)", len(calls))
		}
		for _, call := range f.client.callsFor("deleteMessage") {
			id := fmt.Sprint(call.Params["message_id"])
			if id != "140" && id != "150" {
				t.Fatalf("deleted message %s, want only the reply and command endpoints", id)
			}
		}
		// /purge announces its single-message purge; /spurge stays silent.
		if calls := f.client.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("sendMessage calls = %d, want only /purge's confirmation", len(calls))
		}
	})

	t.Run("count=1 refuses a reply from another topic", func(t *testing.T) {
		f, dispatcher := newForum(t)
		ctx := forumCtx(f, "/purge count=1", 150, 140, 8)
		ctx.EffectiveMessage.ReplyToMessage.MessageThreadId = 7
		if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatalf("/purge dispatch error = %v", err)
		}
		if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
			t.Fatalf("deleteMessage calls = %d, want none for a cross-topic purge", len(calls))
		}
		if calls := f.client.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("sendMessage calls = %d, want the refusal", len(calls))
		}
	})

	t.Run("wider new forms refuse even when endpoints share a topic", func(t *testing.T) {
		for _, text := range []string{"/purge count=5", "/spurge", "/spurge count=3"} {
			t.Run(text, func(t *testing.T) {
				f, dispatcher := newForum(t)
				ctx := forumCtx(f, text, 200, 100, 7)
				if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
					t.Fatalf("%s dispatch error = %v", text, err)
				}
				if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
					t.Fatalf("deleteMessage calls = %d, want none for %s", len(calls), text)
				}
				if calls := f.client.callsFor("sendMessage"); len(calls) != 1 {
					t.Fatalf("sendMessage calls = %d, want the refusal", len(calls))
				}
			})
		}
	})

	t.Run("legacy /purge without count= keeps its unscoped forum behavior", func(t *testing.T) {
		for _, text := range []string{"/purge", "/purge 10"} {
			t.Run(text, func(t *testing.T) {
				f, dispatcher := newForum(t)
				ctx := forumCtx(f, text, 200, 100, 7)
				if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
					t.Fatalf("%s dispatch error = %v", text, err)
				}
				// No count bound: the full 100..199 range plus the command.
				assertDeleted(t, f.client, f.chat.Id, 100, 150, 199, 200)
				if calls := f.client.callsFor("deleteMessage"); len(calls) != 101 {
					t.Fatalf("deleteMessage calls = %d, want 101 (full range)", len(calls))
				}
				if calls := f.client.callsFor("sendMessage"); len(calls) != 1 {
					t.Fatalf("sendMessage calls = %d, want the confirmation", len(calls))
				}
			})
		}
		// A bare number is the legacy reason, not a count: it shows up in the
		// confirmation.
		f, dispatcher := newForum(t)
		ctx := forumCtx(f, "/purge 10", 200, 100, 7)
		if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatalf("/purge dispatch error = %v", err)
		}
		call := f.client.onlyCall(t, "sendMessage")
		if text := fmt.Sprint(call.Params["text"]); !strings.Contains(text, "10") {
			t.Fatalf("confirmation = %q, want the numeric reason preserved", text)
		}
	})

	t.Run("topic-less endpoints cannot prove a shared topic", func(t *testing.T) {
		f, dispatcher := newForum(t)
		ctx := forumCtx(f, "/purge count=1", 200, 100, 0)
		if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatalf("/purge dispatch error = %v", err)
		}
		if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
			t.Fatalf("deleteMessage calls = %d, want none without a named topic", len(calls))
		}
		if calls := f.client.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("sendMessage calls = %d, want the refusal", len(calls))
		}
	})
}
