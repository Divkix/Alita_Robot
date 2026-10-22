//go:build testtools

package panel

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
)

func newTimelineFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	seedPanelChat(t, chatID, "Timeline Chat", fixtureNow)
	fixture.client.admins[chatID] = adminListJSON(panelTestUserID, 999)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ModerationEvent{})
	})
	return fixture, chatID
}

// recordTimelineEvent appends one event through the same repository the
// moderation commands use. A set time is stored in UTC, the zone Record uses
// when it stamps one, so SQLite's text comparison orders correctly.
func recordTimelineEvent(t *testing.T, event models.ModerationEvent) {
	t.Helper()
	if !event.CreatedAt.IsZero() {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	if err := moderation.Record(event); err != nil {
		t.Fatalf("Record(%+v) error = %v", event, err)
	}
}

// storedEventIDs lists the chat's events newest first, the API's order.
func storedEventIDs(t *testing.T, chatID int64, since time.Time) []int64 {
	t.Helper()
	events, err := moderation.ListForChat(chatID, since, 0)
	if err != nil {
		t.Fatalf("ListForChat() error = %v", err)
	}
	ids := make([]int64, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}

func timelinePage(t *testing.T, fixture panelFixture, chatID int64, query string) timelineResponse {
	t.Helper()
	target := "/panel/api/v1/chats/" + strconv.FormatInt(chatID, 10) + "/timeline"
	if query != "" {
		target += "?" + query
	}
	recorder := panelRequest(t, fixture.handler, http.MethodGet, target, readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200 (body %s)", target, recorder.Code, recorder.Body.String())
	}
	var body timelineResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode timeline response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func eventIDs(events []timelineEvent) []int64 {
	ids := make([]int64, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}

func TestTimelinePaginationStableUnderInserts(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	base := fixtureNow.UTC().Add(-24 * time.Hour)
	for i := range 5 {
		recordTimelineEvent(t, models.ModerationEvent{
			ChatId:       chatID,
			ActorId:      100,
			TargetUserId: int64(200 + i),
			Action:       models.ModerationActionBan,
			CreatedAt:    base.Add(time.Duration(i) * time.Minute),
		})
	}
	original := storedEventIDs(t, chatID, base.Add(-time.Hour))
	if len(original) != 5 {
		t.Fatalf("seeded %d events, want 5", len(original))
	}

	first := timelinePage(t, fixture, chatID, "limit=2")
	if got, want := eventIDs(first.Events), original[:2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 1 = %v, want %v", got, want)
	}
	if first.NextCursor == "" {
		t.Fatal("page 1 has no next_cursor")
	}

	// A new event between the pages must not shift them: it is newer than the
	// cursor, so it belongs before page 1, never inside or over page 2.
	recordTimelineEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      100,
		TargetUserId: 900,
		Action:       models.ModerationActionKick,
		CreatedAt:    base.Add(10 * time.Minute),
	})

	second := timelinePage(t, fixture, chatID, "limit=2&cursor="+first.NextCursor)
	if got, want := eventIDs(second.Events), original[2:4]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 2 = %v, want %v", got, want)
	}
	if second.NextCursor == "" {
		t.Fatal("page 2 has no next_cursor")
	}
	third := timelinePage(t, fixture, chatID, "limit=2&cursor="+second.NextCursor)
	if got, want := eventIDs(third.Events), original[4:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 3 = %v, want %v", got, want)
	}
	if third.NextCursor != "" {
		t.Fatalf("page 3 next_cursor = %q, want empty", third.NextCursor)
	}

	seen := make(map[int64]bool)
	for _, page := range [][]timelineEvent{first.Events, second.Events, third.Events} {
		for _, event := range page {
			if seen[event.ID] {
				t.Fatalf("event %d appeared on two pages", event.ID)
			}
			seen[event.ID] = true
		}
	}
	if len(seen) != len(original) {
		t.Fatalf("pages carried %d distinct original events, want %d", len(seen), len(original))
	}

	fresh := timelinePage(t, fixture, chatID, "limit=2")
	if fresh.Events[0].TargetUserID != 900 {
		t.Fatalf("fresh page 1 newest target = %d, want the event inserted between pages", fresh.Events[0].TargetUserID)
	}
}

func TestTimelineCursorBreaksSameTimestampTies(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	at := fixtureNow.UTC().Add(-time.Hour)
	for i := range 2 {
		recordTimelineEvent(t, models.ModerationEvent{
			ChatId:       chatID,
			ActorId:      100,
			TargetUserId: int64(300 + i),
			Action:       models.ModerationActionMute,
			CreatedAt:    at,
		})
	}
	original := storedEventIDs(t, chatID, at.Add(-time.Minute))

	first := timelinePage(t, fixture, chatID, "limit=1")
	second := timelinePage(t, fixture, chatID, "limit=1&cursor="+first.NextCursor)
	if got, want := eventIDs(first.Events), original[:1]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 1 = %v, want %v", got, want)
	}
	if got, want := eventIDs(second.Events), original[1:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("page 2 = %v, want %v", got, want)
	}
}

func TestTimelineStopsAtTheRetentionBoundary(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	recordTimelineEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      100,
		TargetUserId: 401,
		Action:       models.ModerationActionBan,
		CreatedAt:    fixtureNow.UTC().Add(-moderation.EventRetention - time.Hour),
	})
	recordTimelineEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      100,
		TargetUserId: 402,
		Action:       models.ModerationActionBan,
		CreatedAt:    fixtureNow.UTC().Add(-moderation.EventRetention + time.Hour),
	})

	page := timelinePage(t, fixture, chatID, "")
	if len(page.Events) != 1 || page.Events[0].TargetUserID != 402 {
		t.Fatalf("events = %+v, want only the event inside 180 days", page.Events)
	}
}

func TestTimelineFiltersByActionAndActor(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	base := fixtureNow.UTC().Add(-time.Hour)
	events := []models.ModerationEvent{
		{ChatId: chatID, ActorId: 501, TargetUserId: 601, Action: models.ModerationActionBan, CreatedAt: base},
		{ChatId: chatID, ActorId: 501, TargetUserId: 602, Action: models.ModerationActionMute, CreatedAt: base.Add(time.Minute)},
		{ChatId: chatID, ActorId: 502, TargetUserId: 603, Action: models.ModerationActionBan, CreatedAt: base.Add(2 * time.Minute)},
		{ChatId: chatID, ActorId: 502, TargetUserId: 604, Action: models.ModerationActionUnban, CreatedAt: base.Add(3 * time.Minute)},
	}
	for _, event := range events {
		recordTimelineEvent(t, event)
	}

	byAction := timelinePage(t, fixture, chatID, "action=ban")
	if len(byAction.Events) != 2 {
		t.Fatalf("action=ban returned %d events, want 2", len(byAction.Events))
	}
	for _, event := range byAction.Events {
		if event.Action != models.ModerationActionBan {
			t.Fatalf("action=ban returned action %q", event.Action)
		}
	}

	byActor := timelinePage(t, fixture, chatID, "actor=501")
	if len(byActor.Events) != 2 {
		t.Fatalf("actor=501 returned %d events, want 2", len(byActor.Events))
	}
	for _, event := range byActor.Events {
		if event.ActorID != 501 {
			t.Fatalf("actor=501 returned actor %d", event.ActorID)
		}
	}

	both := timelinePage(t, fixture, chatID, "action=ban&actor=502")
	if len(both.Events) != 1 || both.Events[0].TargetUserID != 603 {
		t.Fatalf("action=ban&actor=502 = %+v, want only target 603", both.Events)
	}

	none := timelinePage(t, fixture, chatID, "action=ban&actor=501&limit=1")
	if len(none.Events) != 1 || none.Events[0].TargetUserID != 601 {
		t.Fatalf("filtered page = %+v, want only target 601", none.Events)
	}
	if none.NextCursor != "" {
		t.Fatalf("filtered single page next_cursor = %q, want empty", none.NextCursor)
	}
}

func TestTimelineHidesAutomatedReasonsAndMessageFields(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	base := fixtureNow.UTC().Add(-time.Hour)
	recordTimelineEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      -1,
		TargetUserId: 701,
		Action:       models.ModerationActionKick,
		Reason:       "automatic sweep",
		CreatedAt:    base,
	})
	recordTimelineEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      502,
		TargetUserId: 702,
		Action:       models.ModerationActionBan,
		Reason:       "spam",
		CreatedAt:    base.Add(time.Minute),
	})

	recorder := panelRequest(t, fixture.handler, http.MethodGet, "/panel/api/v1/chats/"+strconv.FormatInt(chatID, 10)+"/timeline", readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var raw struct {
		Events []map[string]json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw timeline %q: %v", recorder.Body.String(), err)
	}
	if len(raw.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(raw.Events))
	}
	for _, event := range raw.Events {
		for _, field := range []string{"text", "message_id", "message_text"} {
			if _, ok := event[field]; ok {
				t.Fatalf("timeline event exposes %q", field)
			}
		}
	}

	page := timelinePage(t, fixture, chatID, "")
	if page.Events[0].Reason != "spam" {
		t.Fatalf("human reason = %q, want spam", page.Events[0].Reason)
	}
	if page.Events[1].Reason != "" {
		t.Fatalf("automated reason = %q, want empty", page.Events[1].Reason)
	}
	if page.Events[1].ActorID != -1 {
		t.Fatalf("automated actor = %d, want -1", page.Events[1].ActorID)
	}
	if page.Events[0].CreatedAt != base.Add(time.Minute).UTC().Format(time.RFC3339) {
		t.Fatalf("created_at = %q, want RFC3339 UTC", page.Events[0].CreatedAt)
	}
}

func TestTimelineRejectsInvalidQuery(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	for _, query := range []string{"cursor=nonsense", "action=promote", "actor=abc", "limit=0"} {
		recorder := panelRequest(t, fixture.handler, http.MethodGet, "/panel/api/v1/chats/"+strconv.FormatInt(chatID, 10)+"/timeline?"+query, readFixture(t, "initdata_main.txt"), "", "")
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d, want 400 (body %s)", query, recorder.Code, recorder.Body.String())
		}
	}
}
