//go:build testtools

package recurring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/recurringmessages"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

var recurringChatSeq atomic.Int64

// newRecurringTestChat returns a group no other test in this package uses,
// stored with a title, and removes its rows when the test ends.
func newRecurringTestChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + recurringChatSeq.Add(1))
	if err := chats.EnsureChatInDb(chatID, "Test Group"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.RecurringMessage{}).Error; err != nil {
			t.Errorf("cleanup recurring messages of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

type botCall struct {
	Method string
	Params map[string]any
}

// recurringBotClient is a hand-written gotgbot.BotClient: it records every
// call, numbers the messages it "posts", and can fail a set number of sends.
type recurringBotClient struct {
	mu        sync.Mutex
	calls     []botCall
	nextID    int64
	failSends int
	sendErr   error
}

func (c *recurringBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make(map[string]any, len(params))
	for key, value := range params {
		copied[key] = value
	}
	c.calls = append(c.calls, botCall{Method: method, Params: copied})

	if strings.HasPrefix(method, "send") {
		if c.failSends > 0 {
			c.failSends--
			if c.sendErr != nil {
				return nil, c.sendErr
			}
			return nil, errors.New("telegram is unreachable")
		}
		c.nextID++
		return json.RawMessage(fmt.Sprintf(`{"message_id":%d,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test"}}`, 100+c.nextID)), nil
	}
	return json.RawMessage(`true`), nil
}

func (c *recurringBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *recurringBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func (c *recurringBotClient) callsOf(prefix string) []botCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []botCall
	for _, call := range c.calls {
		if strings.HasPrefix(call.Method, prefix) {
			out = append(out, call)
		}
	}
	return out
}

// newTestScheduler returns a scheduler with a limiter of its own, so a test's
// pause or 429 never slows the other tests' sends.
func newTestScheduler(client *recurringBotClient) *Scheduler {
	s := NewScheduler(&gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true, Username: "RecurringTestBot"},
	})
	s.limiter = ratelimit.NewSendLimiter(ratelimit.DefaultSendRate, ratelimit.DefaultSendBurst)
	return s
}

func addMessage(t *testing.T, msg *models.RecurringMessage) *models.RecurringMessage {
	t.Helper()
	if msg.IntervalSecs == 0 {
		msg.IntervalSecs = int64(time.Hour / time.Second)
	}
	if msg.MsgType == 0 {
		msg.MsgType = 1
	}
	if err := recurringmessages.Add(msg); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	return msg
}

func loadRow(t *testing.T, id int64) models.RecurringMessage {
	t.Helper()
	var row models.RecurringMessage
	if err := db.DB.Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatalf("load recurring message %d: %v", id, err)
	}
	return row
}

func isDue(t *testing.T, id int64, now time.Time) bool {
	t.Helper()
	rows, err := recurringmessages.ListDue(now, 1000)
	if err != nil {
		t.Fatalf("ListDue() error = %v", err)
	}
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func dueCopy(t *testing.T, id int64, now time.Time) *models.RecurringMessage {
	t.Helper()
	rows, err := recurringmessages.ListDue(now, 1000)
	if err != nil {
		t.Fatalf("ListDue() error = %v", err)
	}
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	t.Fatalf("message %d is not due at %v", id, now)
	return nil
}

func telegramError(code int, description string) error {
	return &gotgbot.TelegramError{Method: "sendMessage", Code: code, Description: description}
}

func floodError(retryAfter int64) error {
	return &gotgbot.TelegramError{
		Method:         "sendMessage",
		Code:           http.StatusTooManyRequests,
		Description:    "Too Many Requests: retry after",
		ResponseParams: &gotgbot.ResponseParameters{RetryAfter: retryAfter},
	}
}

func TestNextRun(t *testing.T) {
	from := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"not due yet", from.Add(-time.Minute), from},
		{"due now", from, from.Add(time.Hour)},
		{"a minute late", from.Add(time.Minute), from.Add(time.Hour)},
		{"three runs missed", from.Add(3*time.Hour + 30*time.Minute), from.Add(4 * time.Hour)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextRun(from, time.Hour, tc.now); !got.Equal(tc.want) {
				t.Fatalf("NextRun() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Two replicas that read the same due row post one message; the occurrence
// moves to the next one on the schedule.
func TestSchedulerPostsOneMessageAcrossTwoReplicas(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "Read the rules", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{}
	first, second := newTestScheduler(client), newTestScheduler(client)
	rowA, rowB := dueCopy(t, msg.ID, now), dueCopy(t, msg.ID, now)
	first.fire(context.Background(), rowA, now)
	second.fire(context.Background(), rowB, now)

	sends := client.callsOf("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	if got := sends[0].Params["text"]; got != "Read the rules" {
		t.Fatalf("text = %v, want the stored text", got)
	}
	row := loadRow(t, msg.ID)
	if want := now.Add(-time.Minute).Add(time.Hour); !row.NextRunAt.Equal(want) || !row.Enabled {
		t.Fatalf("next_run_at = %v enabled = %v, want %v and enabled", row.NextRunAt, row.Enabled, want)
	}
}

// runDue posts every due message once and leaves the rest alone.
func TestSchedulerRunDuePostsOnlyDueMessages(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	due := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "due", NextRunAt: now.Add(-time.Minute)})
	later := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "later", NextRunAt: now.Add(time.Minute)})

	client := &recurringBotClient{}
	newTestScheduler(client).runDue(context.Background(), now)

	var texts []string
	for _, call := range client.callsOf("sendMessage") {
		if call.Params["chat_id"] == fmt.Sprint(chatID) || call.Params["chat_id"] == chatID {
			texts = append(texts, fmt.Sprint(call.Params["text"]))
		}
	}
	if len(texts) != 1 || texts[0] != "due" {
		t.Fatalf("posted %q, want only the due message", texts)
	}
	if isDue(t, due.ID, now) || !isDue(t, later.ID, now.Add(time.Minute)) {
		t.Fatal("runDue moved the wrong message")
	}
}

// After downtime one message goes out and the next run is the first one after
// now on the schedule, not a backlog of missed ones.
func TestSchedulerPostsOnceAfterMissedRuns(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-3*time.Hour - 30*time.Minute)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hello", NextRunAt: start})

	client := &recurringBotClient{}
	s := newTestScheduler(client)
	s.runDue(context.Background(), now)
	s.runDue(context.Background(), now)

	if got := len(client.callsOf("sendMessage")); got != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", got)
	}
	if row := loadRow(t, msg.ID); !row.NextRunAt.Equal(start.Add(4 * time.Hour)) {
		t.Fatalf("next_run_at = %v, want %v", row.NextRunAt, start.Add(4*time.Hour))
	}
}

// The content is rendered like a note: chat fillings, buttons, the topic and
// the directives.
func TestSchedulerRendersTheContentIntoTheTopic(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{
		ChatID:    chatID,
		ThreadID:  42,
		Text:      "Welcome to {chatname}! {nonotif}",
		Buttons:   models.ButtonArray{{Name: "Site", Url: "https://example.com"}},
		NextRunAt: now.Add(-time.Minute),
	})

	client := &recurringBotClient{}
	newTestScheduler(client).fire(context.Background(), dueCopy(t, msg.ID, now), now)

	sends := client.callsOf("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	params := sends[0].Params
	if got := strings.TrimSpace(fmt.Sprint(params["text"])); got != "Welcome to Test Group!" {
		t.Fatalf("text = %q, want the chat name filled and the directive removed", got)
	}
	if got := fmt.Sprint(params["message_thread_id"]); got != "42" {
		t.Fatalf("message_thread_id = %v, want 42", params["message_thread_id"])
	}
	if got := fmt.Sprint(params["disable_notification"]); got != "true" {
		t.Fatalf("disable_notification = %v, want true from {nonotif}", params["disable_notification"])
	}
	if markup := fmt.Sprint(params["reply_markup"]); !strings.Contains(markup, "https://example.com") {
		t.Fatalf("reply_markup = %s, want the stored button", markup)
	}
}

// Media content goes out through the media sender with its file.
func TestSchedulerPostsMedia(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{
		ChatID:    chatID,
		Text:      "caption",
		FileID:    "photo-file-id",
		MsgType:   db.PHOTO,
		NextRunAt: now.Add(-time.Minute),
	})

	client := &recurringBotClient{}
	newTestScheduler(client).fire(context.Background(), dueCopy(t, msg.ID, now), now)

	photos := client.callsOf("sendPhoto")
	if len(photos) != 1 {
		t.Fatalf("sendPhoto calls = %d, want 1", len(photos))
	}
	if got := fmt.Sprint(photos[0].Params["photo"]); !strings.Contains(got, "photo-file-id") {
		t.Fatalf("photo = %v, want the stored file", photos[0].Params["photo"])
	}
}

// With delete-previous the earlier occurrence is deleted once the new one is
// out, and the new one is remembered for the next run.
func TestSchedulerDeletesThePreviousOccurrence(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", DeletePrevious: true, NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{}
	s := newTestScheduler(client)
	s.fire(context.Background(), dueCopy(t, msg.ID, now), now)
	if got := len(client.callsOf("deleteMessage")); got != 0 {
		t.Fatalf("deleteMessage calls after the first run = %d, want 0", got)
	}
	firstID := loadRow(t, msg.ID).LastMessageID
	if firstID == 0 {
		t.Fatal("the posted message was not remembered")
	}

	later := now.Add(time.Hour)
	s.fire(context.Background(), dueCopy(t, msg.ID, later), later)
	deletes := client.callsOf("deleteMessage")
	if len(deletes) != 1 || fmt.Sprint(deletes[0].Params["message_id"]) != fmt.Sprint(firstID) {
		t.Fatalf("deleteMessage calls = %+v, want one for message %d", deletes, firstID)
	}
	if got := loadRow(t, msg.ID).LastMessageID; got == firstID || got == 0 {
		t.Fatalf("last_message_id = %d, want the second post", got)
	}
}

// Without delete-previous nothing is deleted.
func TestSchedulerKeepsThePreviousOccurrenceByDefault(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{}
	s := newTestScheduler(client)
	s.fire(context.Background(), dueCopy(t, msg.ID, now), now)
	later := now.Add(time.Hour)
	s.fire(context.Background(), dueCopy(t, msg.ID, later), later)

	if got := len(client.callsOf("sendMessage")); got != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", got)
	}
	if got := len(client.callsOf("deleteMessage")); got != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0", got)
	}
}

// A message whose end time has passed stops without posting.
func TestSchedulerStopsAnEndedMessageWithoutPosting(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	ended := now.Add(-2 * time.Minute)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute), EndsAt: &ended})

	client := &recurringBotClient{}
	newTestScheduler(client).runDue(context.Background(), now)

	if got := len(client.callsOf("send")); got != 0 {
		t.Fatalf("send calls = %d, want none after the end", got)
	}
	if row := loadRow(t, msg.ID); row.Enabled {
		t.Fatal("ended message is still enabled")
	}
}

// An end time that passed while occurrences were missed stops the message
// without posting, even though the missed run was before the end.
func TestSchedulerDoesNotPostAfterTheEndWhenRunsWereMissed(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	ended := now.Add(-4 * time.Hour)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-5 * time.Hour), EndsAt: &ended})

	client := &recurringBotClient{}
	newTestScheduler(client).runDue(context.Background(), now)

	if got := len(client.callsOf("send")); got != 0 {
		t.Fatalf("send calls = %d, want none after the end", got)
	}
	if row := loadRow(t, msg.ID); row.Enabled {
		t.Fatal("ended message is still enabled")
	}
}

// The last occurrence before the end time is posted, then the message stops.
func TestSchedulerPostsTheLastOccurrenceThenStops(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	ends := now.Add(30 * time.Minute)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute), EndsAt: &ends})

	client := &recurringBotClient{}
	newTestScheduler(client).runDue(context.Background(), now)

	if got := len(client.callsOf("sendMessage")); got != 1 {
		t.Fatalf("sendMessage calls = %d, want the last occurrence", got)
	}
	if row := loadRow(t, msg.ID); row.Enabled {
		t.Fatal("message is still enabled after its last occurrence")
	}
}

// A send the chat can never receive stops the message.
func TestSchedulerStopsAnUndeliverableMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"bot kicked", telegramError(http.StatusForbidden, "Forbidden: bot was kicked from the supergroup chat")},
		{"topic gone", telegramError(http.StatusBadRequest, "Bad Request: message thread not found")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache.SetupTestMemoryMarshaler(t)
			chatID := newRecurringTestChat(t)
			now := time.Now().UTC().Truncate(time.Second)
			msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

			client := &recurringBotClient{failSends: 1, sendErr: tc.err}
			newTestScheduler(client).fire(context.Background(), dueCopy(t, msg.ID, now), now)

			if row := loadRow(t, msg.ID); row.Enabled {
				t.Fatal("undeliverable message is still enabled")
			}
			if isDue(t, msg.ID, now.Add(48*time.Hour)) {
				t.Fatal("undeliverable message came due again")
			}
		})
	}
}

// Any other failure skips this occurrence: the message stays on and is not
// retried until its next run.
func TestSchedulerSkipsAnOccurrenceOnATransientFailure(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{failSends: 1}
	newTestScheduler(client).fire(context.Background(), dueCopy(t, msg.ID, now), now)

	row := loadRow(t, msg.ID)
	if !row.Enabled {
		t.Fatal("a transient failure stopped the message")
	}
	if isDue(t, msg.ID, now.Add(time.Minute)) {
		t.Fatal("the failed occurrence is retried before the next run")
	}
	if !isDue(t, msg.ID, now.Add(time.Hour)) {
		t.Fatal("the next run is not due on schedule")
	}
}

// A bot that may not write in the chat skips the occurrence: an admin can give
// the right back, and the message resumes on its next run.
func TestSchedulerSkipsAnOccurrenceWhenTheBotMayNotWrite(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{
		failSends: 1,
		sendErr:   telegramError(http.StatusBadRequest, "Bad Request: not enough rights to send text messages to the chat"),
	}
	newTestScheduler(client).fire(context.Background(), dueCopy(t, msg.ID, now), now)

	if row := loadRow(t, msg.ID); !row.Enabled {
		t.Fatal("a missing send right stopped the message")
	}
	if !isDue(t, msg.ID, now.Add(time.Hour)) {
		t.Fatal("the next run is not due on schedule")
	}
}

// A flood wait the limiter will not wait out gives the claim back.
func TestSchedulerRestoresTheClaimOnALongFloodWait(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{failSends: 1, sendErr: floodError(3600)}
	newTestScheduler(client).fire(context.Background(), dueCopy(t, msg.ID, now), now)

	if !isDue(t, msg.ID, now) {
		t.Fatal("message is not due again after the flood wait")
	}
	if !loadRow(t, msg.ID).Enabled {
		t.Fatal("flood wait stopped the message")
	}
}

// Shutdown during a limiter wait gives the claim back.
func TestSchedulerRestoresTheClaimWhenShutdownEndsTheWait(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{}
	s := newTestScheduler(client)
	s.limiter.Pause(30 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	s.fire(ctx, dueCopy(t, msg.ID, now), now)

	if got := len(client.callsOf("send")); got != 0 {
		t.Fatalf("send calls = %d, want none", got)
	}
	if !isDue(t, msg.ID, now) {
		t.Fatal("message is not due again after shutdown")
	}
}

// During a long flood wait a tick claims nothing.
func TestSchedulerClaimsNothingDuringALongFloodWait(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringTestChat(t)
	now := time.Now().UTC().Truncate(time.Second)
	msg := addMessage(t, &models.RecurringMessage{ChatID: chatID, Text: "hi", NextRunAt: now.Add(-time.Minute)})

	client := &recurringBotClient{}
	s := newTestScheduler(client)
	s.limiter.Pause(time.Hour)
	s.runDue(context.Background(), now)

	if got := len(client.callsOf("send")); got != 0 {
		t.Fatalf("send calls = %d, want none", got)
	}
	if !isDue(t, msg.ID, now) {
		t.Fatal("message was claimed during the flood wait")
	}
}

func TestSchedulerStopIsIdempotentAndSafeWithoutStart(t *testing.T) {
	s := newTestScheduler(&recurringBotClient{})
	s.Stop()
	s.Stop()

	started := newTestScheduler(&recurringBotClient{})
	started.Start()
	started.Start()
	started.Stop()
	started.Stop()
}
