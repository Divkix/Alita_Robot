//go:build testtools

package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

// overviewDayStart is UTC midnight of fixtureNow, the day the fixture clock
// reports as today.
func overviewDayStart() time.Time {
	now := fixtureNow.UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func overviewPath(chatID int64, query string) string {
	path := "/panel/api/v1/chats/" + strconv.FormatInt(chatID, 10) + "/overview"
	if query != "" {
		path += "?" + query
	}
	return path
}

func newOverviewFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	return newOverviewFixtureWithClock(t, func() time.Time { return fixtureNow })
}

func newOverviewFixtureWithClock(t *testing.T, clock func() time.Time) (panelFixture, int64) {
	t.Helper()
	cache.SetupTestMemoryMarshaler(t)

	client := newPanelFakeClient()
	bot := &gotgbot.Bot{
		Token:     fixtureBotToken,
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"},
	}
	handler := New(bot, Deps{
		SessionMaxAge: 24 * time.Hour,
		LookupChats:   sqliteLookup,
		Now:           clock,
	})

	chatID := uniquePanelChatID()
	seedPanelChat(t, chatID, fmt.Sprintf("Overview Chat %d", chatID), fixtureNow)
	client.admins[chatID] = adminListJSON(panelTestUserID, 999)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatDailyStat{}).Error; err != nil {
			t.Errorf("delete daily stats of chat %d: %v", chatID, err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ModerationEvent{}).Error; err != nil {
			t.Errorf("delete moderation events of chat %d: %v", chatID, err)
		}
	})
	return panelFixture{handler: handler, client: client}, chatID
}

func seedDailyStats(t *testing.T, chatID int64, day string, messages, joins, leaves int64) {
	t.Helper()
	row := models.ChatDailyStat{ChatID: chatID, Day: day, Messages: messages, Joins: joins, Leaves: leaves}
	if err := db.DB.Create(&row).Error; err != nil {
		t.Fatalf("seed daily stats for %s: %v", day, err)
	}
}

func seedModerationEvent(t *testing.T, chatID, actorID, targetID int64, action, reason string, at time.Time) models.ModerationEvent {
	t.Helper()
	row := models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      actorID,
		TargetUserId: targetID,
		Action:       action,
		Reason:       reason,
		CreatedAt:    at.UTC(),
	}
	if err := db.DB.Create(&row).Error; err != nil {
		t.Fatalf("seed moderation event: %v", err)
	}
	return row
}

func decodeOverview(t *testing.T, recorder *httptest.ResponseRecorder) overviewResponse {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body overviewResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	return body
}

func TestOverviewReturnsZeroFilledSeriesAndRecentEvents(t *testing.T) {
	fixture, chatID := newOverviewFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	start := overviewDayStart()
	yesterday := start.AddDate(0, 0, -1)
	tenDaysAgo := start.AddDate(0, 0, -10)

	// Two days inside the default 7-day window, one only inside 30 days.
	seedDailyStats(t, chatID, stats.Day(yesterday), 10, 2, 1)
	seedDailyStats(t, chatID, stats.Day(start), 5, 1, 0)
	seedDailyStats(t, chatID, stats.Day(tenDaysAgo), 999, 9, 9)

	// One event outside the window, then six inside it. Insertion order is
	// timestamp order, so ids grow with time.
	seeded := []models.ModerationEvent{
		seedModerationEvent(t, chatID, 700, 701, models.ModerationActionBan, "old", start.AddDate(0, 0, -10).Add(9*time.Hour)),
	}
	inWindow := []struct {
		action string
		at     time.Time
		reason string
	}{
		{models.ModerationActionBan, start.AddDate(0, 0, -3).Add(9 * time.Hour), ""},
		{models.ModerationActionKick, start.AddDate(0, 0, -2).Add(9 * time.Hour), ""},
		{models.ModerationActionMute, start.AddDate(0, 0, -1).Add(9 * time.Hour), ""},
		{models.ModerationActionUnban, start.Add(8 * time.Hour), ""},
		{models.ModerationActionBan, start.Add(9 * time.Hour), ""},
		{models.ModerationActionKick, start.Add(10 * time.Hour), "spam"},
	}
	for i, event := range inWindow {
		seeded = append(seeded, seedModerationEvent(t, chatID, 800+int64(i), 900+int64(i), event.action, event.reason, event.at))
	}

	body := decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), raw, "", ""))

	if body.MemberCount != fixture.client.memberCount {
		t.Fatalf("member_count = %d, want %d", body.MemberCount, fixture.client.memberCount)
	}
	if len(body.Days) != 7 {
		t.Fatalf("days = %d, want 7", len(body.Days))
	}
	if got, want := body.Days[0].Day, stats.Day(start.AddDate(0, 0, -6)); got != want {
		t.Fatalf("first day = %q, want %q", got, want)
	}
	if got, want := body.Days[6].Day, stats.Day(start); got != want {
		t.Fatalf("last day = %q, want %q", got, want)
	}
	if point := body.Days[0]; point.Messages != 0 || point.Joins != 0 || point.Leaves != 0 {
		t.Fatalf("unseeded day = %+v, want zeroes", point)
	}
	if point := body.Days[5]; point.Day != stats.Day(yesterday) || point.Messages != 10 || point.Joins != 2 || point.Leaves != 1 {
		t.Fatalf("yesterday = %+v, want 10/2/1", point)
	}
	if point := body.Days[6]; point.Day != stats.Day(start) || point.Messages != 5 || point.Joins != 1 || point.Leaves != 0 {
		t.Fatalf("today = %+v, want 5/1/0", point)
	}
	if body.Totals.Messages != 15 || body.Totals.Joins != 3 || body.Totals.Leaves != 1 {
		t.Fatalf("totals = %+v, want 15/3/1", body.Totals)
	}
	if body.ModerationCount != 6 {
		t.Fatalf("moderation_count = %d, want 6", body.ModerationCount)
	}

	// The five newest events, newest first, and never message text.
	if len(body.Recent) != 5 {
		t.Fatalf("recent = %d events, want 5", len(body.Recent))
	}
	for i, got := range body.Recent {
		want := seeded[len(seeded)-1-i]
		if got.ID != want.ID {
			t.Fatalf("recent[%d].id = %d, want %d (newest first)", i, got.ID, want.ID)
		}
		if got.Action != want.Action || got.ActorID != want.ActorId || got.TargetUserID != want.TargetUserId {
			t.Fatalf("recent[%d] = %+v, want action %q actor %d target %d", i, got, want.Action, want.ActorId, want.TargetUserId)
		}
		if got.CreatedAt != want.CreatedAt.UTC().Format(time.RFC3339) {
			t.Fatalf("recent[%d].created_at = %q, want %q", i, got.CreatedAt, want.CreatedAt.UTC().Format(time.RFC3339))
		}
	}
	if body.Recent[0].Reason != "spam" {
		t.Fatalf("recent[0].reason = %q, want the stored reason", body.Recent[0].Reason)
	}

	// The wider window picks up the older row and event and reshapes the
	// series, proving days selects the range instead of only relabelling it.
	wider := decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, "days=30"), raw, "", ""))
	if len(wider.Days) != 30 {
		t.Fatalf("30-day series = %d points, want 30", len(wider.Days))
	}
	if wider.Totals.Messages != 1014 || wider.Totals.Joins != 12 || wider.Totals.Leaves != 10 {
		t.Fatalf("30-day totals = %+v, want 1014/12/10", wider.Totals)
	}
	if wider.ModerationCount != 7 {
		t.Fatalf("30-day moderation_count = %d, want 7", wider.ModerationCount)
	}
}

func TestOverviewRejectsUnsupportedWindows(t *testing.T) {
	fixture, chatID := newOverviewFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	for _, query := range []string{"days=13", "days=0", "days=week"} {
		recorder := panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, query), raw, "", "")
		requireAPIError(t, recorder, http.StatusBadRequest, "invalid_days")
	}
}

func TestOverviewMemberCountIsCachedForAMinute(t *testing.T) {
	now := fixtureNow
	fixture, chatID := newOverviewFixtureWithClock(t, func() time.Time { return now })
	raw := readFixture(t, "initdata_main.txt")

	first := decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), raw, "", ""))
	if first.MemberCount != fixture.client.memberCount {
		t.Fatalf("member_count = %d, want %d", first.MemberCount, fixture.client.memberCount)
	}

	now = fixtureNow.Add(30 * time.Second)
	second := decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), raw, "", ""))
	if second.MemberCount != first.MemberCount {
		t.Fatalf("cached member_count = %d, want %d", second.MemberCount, first.MemberCount)
	}
	if calls := fixture.client.countCalls("getChatMemberCount"); calls != 1 {
		t.Fatalf("getChatMemberCount calls = %d, want 1 within the minute", calls)
	}

	now = fixtureNow.Add(61 * time.Second)
	decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), raw, "", ""))
	if calls := fixture.client.countCalls("getChatMemberCount"); calls != 2 {
		t.Fatalf("getChatMemberCount calls = %d, want 2 after the cache expires", calls)
	}
}

func TestOverviewEmptyChatReturnsZeroFilledSeries(t *testing.T) {
	fixture, chatID := newOverviewFixture(t)

	body := decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), readFixture(t, "initdata_main.txt"), "", ""))

	if body.MemberCount != fixture.client.memberCount {
		t.Fatalf("member_count = %d, want %d", body.MemberCount, fixture.client.memberCount)
	}
	if len(body.Days) != 7 {
		t.Fatalf("days = %d, want 7 zero-filled points", len(body.Days))
	}
	for _, point := range body.Days {
		if point.Messages != 0 || point.Joins != 0 || point.Leaves != 0 {
			t.Fatalf("point %+v is not zero", point)
		}
	}
	if body.Totals != (overviewTotals{}) {
		t.Fatalf("totals = %+v, want zeroes", body.Totals)
	}
	if body.ModerationCount != 0 {
		t.Fatalf("moderation_count = %d, want 0", body.ModerationCount)
	}
	if body.Recent == nil {
		t.Fatal("recent is null, want an empty JSON array")
	}
	if len(body.Recent) != 0 {
		t.Fatalf("recent = %d events, want none", len(body.Recent))
	}
}

func TestOverviewIsReadableWithoutAdminRole(t *testing.T) {
	fixture, chatID := newOverviewFixture(t)
	// The caller is a plain member: getChatMember reports "member".
	fixture.client.admins[chatID] = adminListJSON(999)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), readFixture(t, "initdata_main.txt"), "", "")
	body := decodeOverview(t, recorder)
	if body.MemberCount != fixture.client.memberCount {
		t.Fatalf("member_count = %d, want %d for a non-admin", body.MemberCount, fixture.client.memberCount)
	}
}

func TestOverviewSurfacesTelegramError(t *testing.T) {
	fixture, chatID := newOverviewFixture(t)
	fixture.client.memberCountErr = errors.New("telegram unavailable")

	recorder := panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), readFixture(t, "initdata_main.txt"), "", "")
	requireAPIError(t, recorder, http.StatusBadGateway, "telegram_error")
}
