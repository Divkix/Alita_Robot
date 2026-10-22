//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/probation"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// probationFixture is an admin fixture whose chat has probation on, with
// the given changes to the defaults.
func probationFixture(t *testing.T, configure func(chatID int64) error) adminChatFixture {
	t.Helper()
	f := newAdminChatFixture(t)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", f.chat.Id).Delete(&models.ProbationSettings{}).Error; err != nil {
			t.Errorf("cleanup probation settings: %v", err)
		}
		if err := db.DB.Where("chat_id = ?", f.chat.Id).Delete(&models.ProbationMember{}).Error; err != nil {
			t.Errorf("cleanup probation members: %v", err)
		}
	})
	if err := probation.SetEnabled(f.chat.Id, true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	if configure != nil {
		if err := configure(f.chat.Id); err != nil {
			t.Fatalf("configure probation: %v", err)
		}
	}
	return f
}

func onProbation(t *testing.T, chatID, userID int64) bool {
	t.Helper()
	member, err := probation.GetMember(context.Background(), chatID, userID)
	if err != nil {
		t.Fatalf("GetMember(%d, %d) error = %v", chatID, userID, err)
	}
	return member.Found
}

func startProbationAt(t *testing.T, chatID, userID int64, joined time.Time) {
	t.Helper()
	if err := probation.Start(chatID, userID, joined); err != nil {
		t.Fatalf("Start(%d, %d) error = %v", chatID, userID, err)
	}
}

// joinChat runs the ChatMemberUpdated join of user through the greetings
// handler, as Telegram delivers it.
func joinChat(t *testing.T, f adminChatFixture, user gotgbot.User) {
	t.Helper()
	ctx := newChatMemberContext(f.bot, f.chat, user, gotgbot.ChatMemberLeft{User: user}, gotgbot.ChatMemberMember{User: user})
	if err := greetingsModule.newMember(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("newMember(%d) error = %v, want EndGroups", user.Id, err)
	}
}

// postInChat runs msg from its sender through the probation watcher and
// fails unless the watcher lets later groups run.
func postInChat(t *testing.T, f adminChatFixture, msg *gotgbot.Message) {
	t.Helper()
	msg.Chat = f.chat
	msg.Date = 1
	ctx := ext.NewContext(f.bot, &gotgbot.Update{UpdateId: msg.MessageId, Message: msg}, nil)
	if err := probationModule.checkProbation(f.bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("checkProbation(%d) = %v, want ContinueGroups", msg.MessageId, err)
	}
}

func textFrom(user gotgbot.User, id int64, text string) *gotgbot.Message {
	return &gotgbot.Message{MessageId: id, From: &user, Text: text}
}

func linkFrom(user gotgbot.User, id int64) *gotgbot.Message {
	msg := textFrom(user, id, "see https://example.com")
	msg.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 4, Length: 19}}
	return msg
}

func deletedIDs(client *moduleBotClient, chatID int64) map[int64]bool {
	deleted := map[int64]bool{}
	for _, call := range client.callsFor("deleteMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			var id int64
			_, _ = fmt.Sscan(fmt.Sprint(call.Params["message_id"]), &id)
			deleted[id] = true
		}
	}
	return deleted
}

func TestProbationStartsOnJoinOnlyWhenOnAndForTheClaimWinner(t *testing.T) {
	off := newAdminChatFixture(t)
	newcomer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
	clearRecentJoinProcessing(off.chat.Id, newcomer.Id)
	joinChat(t, off, newcomer)
	if onProbation(t, off.chat.Id, newcomer.Id) {
		t.Fatal("a join in a chat with probation off put the member on probation")
	}

	f := probationFixture(t, nil)
	clearRecentJoinProcessing(f.chat.Id, newcomer.Id)
	joinChat(t, f, newcomer)
	if !onProbation(t, f.chat.Id, newcomer.Id) {
		t.Fatal("a join in a chat with probation on did not start probation")
	}

	approved := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Approved"}
	if err := approvals.AddApprovedUser(f.chat.Id, approved.Id, f.admin.Id, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	t.Cleanup(func() { _ = approvals.RemoveApprovedUser(f.chat.Id, approved.Id) })
	clearRecentJoinProcessing(f.chat.Id, approved.Id)
	joinChat(t, f, approved)

	bot := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper", IsBot: true}
	clearRecentJoinProcessing(f.chat.Id, bot.Id)
	joinChat(t, f, bot)

	// Another guard (spamdb, antiraid) took this join first.
	taken := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Taken"}
	clearRecentJoinProcessing(f.chat.Id, taken.Id)
	if !claimRecentJoinProcessing(f.chat.Id, taken.Id) {
		t.Fatal("claimRecentJoinProcessing() = false on a fresh join")
	}
	joinChat(t, f, taken)

	for _, user := range []gotgbot.User{approved, bot, taken} {
		if onProbation(t, f.chat.Id, user.Id) {
			t.Errorf("%s was put on probation", user.FirstName)
		}
	}
}

func TestProbationDeletesWhatTheChatBlocks(t *testing.T) {
	f := probationFixture(t, nil)
	startProbationAt(t, f.chat.Id, f.member.Id, time.Now())

	forward := textFrom(f.member, 2, "look at this")
	forward.ForwardOrigin = gotgbot.MessageOriginUser{Date: 1, SenderUser: gotgbot.User{Id: 5, FirstName: "Elsewhere"}}
	photo := &gotgbot.Message{MessageId: 3, From: &f.member, Photo: []gotgbot.PhotoSize{{FileId: "p", FileUniqueId: "pu", Width: 1, Height: 1}}}
	for _, msg := range []*gotgbot.Message{linkFrom(f.member, 1), forward, photo, textFrom(f.member, 4, "hi")} {
		postInChat(t, f, msg)
	}
	deleted := deletedIDs(f.client, f.chat.Id)
	if !deleted[1] || !deleted[2] || deleted[3] || deleted[4] {
		t.Fatalf("deleted = %v, want the link (1) and the forward (2) only", deleted)
	}

	if err := probation.SetBlocked(f.chat.Id, false, false, true); err != nil {
		t.Fatalf("SetBlocked() error = %v", err)
	}
	photo.MessageId = 5
	postInChat(t, f, photo)
	postInChat(t, f, linkFrom(f.member, 6))
	deleted = deletedIDs(f.client, f.chat.Id)
	if !deleted[5] || deleted[6] {
		t.Fatalf("deleted = %v, want the photo (5) and not the link (6) once only media is blocked", deleted)
	}
	if got := f.client.callsFor("sendMessage"); len(got) != 0 {
		t.Fatalf("sendMessage calls = %d, want none while the notice is off", len(got))
	}
}

func TestProbationSkipsAdminsApprovedUsersAndOtherMembers(t *testing.T) {
	f := probationFixture(t, nil)
	approved := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Approved"}
	if err := approvals.AddApprovedUser(f.chat.Id, approved.Id, f.admin.Id, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	t.Cleanup(func() { _ = approvals.RemoveApprovedUser(f.chat.Id, approved.Id) })
	startProbationAt(t, f.chat.Id, f.admin.Id, time.Now())
	startProbationAt(t, f.chat.Id, approved.Id, time.Now())

	postInChat(t, f, linkFrom(f.admin, 1))
	postInChat(t, f, linkFrom(approved, 2))
	// The member joined before probation was on, so has no row.
	postInChat(t, f, linkFrom(f.member, 3))
	if deleted := deletedIDs(f.client, f.chat.Id); len(deleted) != 0 {
		t.Fatalf("deleted = %v, want nothing", deleted)
	}
}

func TestProbationEndsAfterEnoughDifferentMessages(t *testing.T) {
	withMiniredis(t)
	f := probationFixture(t, func(chatID int64) error {
		if err := probation.SetMessageCount(chatID, 3); err != nil {
			return err
		}
		return probation.SetMinLength(chatID, 10)
	})
	startProbationAt(t, f.chat.Id, f.member.Id, time.Now())

	for i, text := range []string{
		"hello there everyone",
		"Hello   there EVERYONE", // the same message again
		"short",                  // under the minimum length
		"how is everyone doing",
	} {
		postInChat(t, f, textFrom(f.member, int64(i+1), text))
	}
	if !onProbation(t, f.chat.Id, f.member.Id) {
		t.Fatal("probation ended after two different messages, want three")
	}

	postInChat(t, f, textFrom(f.member, 5, "a third different message"))
	if onProbation(t, f.chat.Id, f.member.Id) {
		t.Fatal("probation did not end after three different messages")
	}
	if n, err := cache.GetRedisClient().Exists(context.Background(), probationRedisKey(f.chat.Id, f.member.Id)).Result(); err != nil || n != 0 {
		t.Fatalf("message count key exists = %d, %v; want removed on graduation", n, err)
	}
	postInChat(t, f, linkFrom(f.member, 6))
	if deleted := deletedIDs(f.client, f.chat.Id); len(deleted) != 0 {
		t.Fatalf("deleted = %v after graduation, want nothing", deleted)
	}
}

// Without Redis nothing is counted: probation ends only when its time is up.
func TestProbationWithoutRedisEndsOnTime(t *testing.T) {
	if cache.GetRedisClient() != nil {
		t.Fatal("test expects no Redis client")
	}
	f := probationFixture(t, func(chatID int64) error { return probation.SetMessageCount(chatID, 1) })
	startProbationAt(t, f.chat.Id, f.member.Id, time.Now())

	postInChat(t, f, textFrom(f.member, 1, "a long enough message to count"))
	if !onProbation(t, f.chat.Id, f.member.Id) {
		t.Fatal("probation ended on a message count without Redis")
	}

	startProbationAt(t, f.chat.Id, f.member.Id, time.Now().Add(-25*time.Hour))
	postInChat(t, f, linkFrom(f.member, 2))
	if deleted := deletedIDs(f.client, f.chat.Id); deleted[2] {
		t.Fatal("a link was deleted after probation's time was up")
	}
	if onProbation(t, f.chat.Id, f.member.Id) {
		t.Fatal("the member row outlived probation's time")
	}
}

func TestProbationEndsWhenTheMemberLeaves(t *testing.T) {
	withMiniredis(t)
	f := probationFixture(t, nil)
	startProbationAt(t, f.chat.Id, f.member.Id, time.Now())
	postInChat(t, f, textFrom(f.member, 1, "a message long enough to count"))

	ctx := newChatMemberContext(f.bot, f.chat, f.member, gotgbot.ChatMemberMember{User: f.member}, gotgbot.ChatMemberLeft{User: f.member})
	if err := greetingsModule.leftMember(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("leftMember error = %v, want EndGroups", err)
	}
	if onProbation(t, f.chat.Id, f.member.Id) {
		t.Fatal("the member is still on probation after leaving")
	}
	if n, err := cache.GetRedisClient().Exists(context.Background(), probationRedisKey(f.chat.Id, f.member.Id)).Result(); err != nil || n != 0 {
		t.Fatalf("message count key exists = %d, %v; want removed on leave", n, err)
	}
}

func TestProbationNoticeIsSentOnceAndRemoved(t *testing.T) {
	useRealLocales(t)
	previous := probationNoticeLifetime
	probationNoticeLifetime = 10 * time.Millisecond
	t.Cleanup(func() { probationNoticeLifetime = previous })

	f := probationFixture(t, func(chatID int64) error { return probation.SetNotice(chatID, true) })
	startProbationAt(t, f.chat.Id, f.member.Id, time.Now())
	postInChat(t, f, linkFrom(f.member, 1))
	postInChat(t, f, linkFrom(f.member, 2))

	sent := f.client.onlyCall(t, "sendMessage")
	if text := fmt.Sprint(sent.Params["text"]); !strings.Contains(text, fmt.Sprint(f.member.Id)) {
		t.Fatalf("notice %q does not mention the member", text)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !deletedIDs(f.client, f.chat.Id)[9001] {
		if time.Now().After(deadline) {
			t.Fatal("the notice (9001) was not deleted")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if deleted := deletedIDs(f.client, f.chat.Id); !deleted[1] || !deleted[2] {
		t.Fatalf("deleted = %v, want both links", deleted)
	}
}

func probationCommand(t *testing.T, f adminChatFixture, user gotgbot.User, id int64, text string) string {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	helpers.WrapCommand(dispatcher, probationDesc, probationModule.probation)
	before := len(spamDBMessagesTo(f.client, f.chat.Id))
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, user, id, text)
	replies := spamDBMessagesTo(f.client, f.chat.Id)
	if len(replies) != before+1 {
		t.Fatalf("%s: replies = %d, want one", text, len(replies)-before)
	}
	return replies[len(replies)-1]
}

func TestProbationCommandChangesSettings(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", f.chat.Id).Delete(&models.ProbationSettings{}).Error
	})
	settings := func() *models.ProbationSettings {
		s, err := probation.GetSettings(context.Background(), f.chat.Id)
		if err != nil {
			t.Fatalf("GetSettings() error = %v", err)
		}
		return s
	}

	probationCommand(t, f, f.admin, 1, "/probation on")
	probationCommand(t, f, f.admin, 2, "/probation time 2h")
	probationCommand(t, f, f.admin, 3, "/probation messages 3")
	probationCommand(t, f, f.admin, 4, "/probation minlen 10")
	probationCommand(t, f, f.admin, 5, "/probation block media, links")
	probationCommand(t, f, f.admin, 6, "/probation notice on")
	got := settings()
	if !got.Enabled || got.Duration() != 2*time.Hour || got.MessageCount != 3 || got.MinLength != 10 ||
		!got.BlockLinks || got.BlockForwards || !got.BlockMedia || !got.Notice {
		t.Fatalf("settings = %+v", got)
	}

	probationCommand(t, f, f.admin, 7, "/probation")
	if after := settings(); after.Enabled != got.Enabled || after.DurationSeconds != got.DurationSeconds ||
		after.MessageCount != got.MessageCount || after.MinLength != got.MinLength || after.BlockLinks != got.BlockLinks ||
		after.BlockForwards != got.BlockForwards || after.BlockMedia != got.BlockMedia || after.Notice != got.Notice {
		t.Fatalf("a bare /probation changed the settings: %+v", after)
	}

	probationCommand(t, f, f.admin, 8, "/probation off")
	if settings().Enabled {
		t.Fatal("/probation off left probation on")
	}
}

func TestProbationCommandRejectsBadInput(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", f.chat.Id).Delete(&models.ProbationSettings{}).Error
	})
	tr := i18n.MustNewTranslator("en")
	for i, tc := range []struct{ text, key string }{
		{"/probation time 30m", "probation_invalid_time"},
		{"/probation time 31d", "probation_invalid_time"},
		{"/probation messages 0", "probation_invalid_messages"},
		{"/probation minlen 501", "probation_invalid_minlen"},
		{"/probation block stickers", "probation_invalid_block"},
		{"/probation block", "probation_invalid_block"},
		{"/probation maybe", "probation_usage"},
		{"/probation notice", "probation_usage"},
	} {
		reply := probationCommand(t, f, f.admin, int64(i+1), tc.text)
		want, _ := tr.GetString(tc.key)
		if want == "" || !strings.Contains(reply, want) {
			t.Errorf("%s: reply %q, want the %s text", tc.text, reply, tc.key)
		}
	}
	if n := countRows(t, &models.ProbationSettings{}, f.chat.Id); n != 0 {
		t.Fatalf("rejected input wrote %d settings rows", n)
	}

	// A member without admin rights is refused by the pipeline.
	probationCommand(t, f, f.member, 20, "/probation on")
	if s, err := probation.GetSettings(context.Background(), f.chat.Id); err != nil || s.Enabled {
		t.Fatalf("a member turned probation on: %+v, %v", s, err)
	}
}

func countRows(t *testing.T, model any, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(model).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestSweepProbationMembersDropsOnlyOldRows(t *testing.T) {
	f := probationFixture(t, nil)
	old := gotgbot.User{Id: uniquePositiveUserID()}
	startProbationAt(t, f.chat.Id, old.Id, time.Now().Add(-probationRetention-time.Hour))
	startProbationAt(t, f.chat.Id, f.member.Id, time.Now())

	sweepProbationMembers(time.Now())
	var users []int64
	if err := db.DB.Model(&models.ProbationMember{}).Where("chat_id = ?", f.chat.Id).Pluck("user_id", &users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0] != f.member.Id {
		t.Fatalf("members after sweep = %v, want [%d]", users, f.member.Id)
	}
}
