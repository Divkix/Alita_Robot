//go:build testtools

package panel

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

func analyticsPath(chatID int64, endpoint, query string) string {
	path := "/panel/api/v1/chats/" + strconv.FormatInt(chatID, 10) + "/" + endpoint
	if query != "" {
		path += "?" + query
	}
	return path
}

// newAnalyticsFixture is the overview fixture plus cleanup of the analytics
// tables.
func newAnalyticsFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	fixture, chatID := newOverviewFixture(t)
	t.Cleanup(func() {
		for _, model := range []any{&models.ChatHourlyStat{}, &models.ChatMemberDailyStat{}, &models.ChatMemberCount{}} {
			if err := db.DB.Where("chat_id = ?", chatID).Delete(model).Error; err != nil {
				t.Errorf("delete analytics rows of chat %d: %v", chatID, err)
			}
		}
	})
	return fixture, chatID
}

func seedRows(t *testing.T, rows any) {
	t.Helper()
	if err := db.DB.Create(rows).Error; err != nil {
		t.Fatalf("seed %T: %v", rows, err)
	}
}

func getAnalytics[T any](t *testing.T, fixture panelFixture, path string) T {
	t.Helper()
	recorder := panelRequest(t, fixture.handler, http.MethodGet, path, readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200 (body %s)", path, recorder.Code, recorder.Body.String())
	}
	var body T
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return body
}

func TestHeatmapSumsMessagesByWeekdayAndUTCHour(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	today := overviewDayStart()
	yesterday := today.AddDate(0, 0, -1)
	tenDaysAgo := today.AddDate(0, 0, -10)
	seedRows(t, &[]models.ChatHourlyStat{
		{ChatID: chatID, Day: stats.Day(today), Hour: 9, Messages: 4},
		{ChatID: chatID, Day: stats.Day(yesterday), Hour: 9, Messages: 3},
		{ChatID: chatID, Day: stats.Day(today), Hour: 21, Messages: 5},
		{ChatID: chatID, Day: stats.Day(tenDaysAgo), Hour: 3, Messages: 100},
	})

	week := getAnalytics[heatmapResponse](t, fixture, analyticsPath(chatID, "heatmap", ""))
	if week.Timezone != "UTC" || week.Days != 7 {
		t.Fatalf("timezone/days = %q/%d, want UTC/7", week.Timezone, week.Days)
	}
	if len(week.Grid) != 7 || len(week.Grid[0]) != 24 {
		t.Fatalf("grid is %dx%d, want 7x24", len(week.Grid), len(week.Grid[0]))
	}
	if want := []int{1, 1, 1, 1, 1, 1, 1}; !reflect.DeepEqual(week.Weekdays, want) {
		t.Fatalf("7-day weekdays = %v, want %v", week.Weekdays, want)
	}
	todayRow, yesterdayRow := weekdayRow(today.Weekday()), weekdayRow(yesterday.Weekday())
	if got := week.Grid[todayRow][9]; got != 4 {
		t.Fatalf("today 09:00 = %d, want 4", got)
	}
	if got := week.Grid[yesterdayRow][9]; got != 3 {
		t.Fatalf("yesterday 09:00 = %d, want 3", got)
	}
	if got := week.Grid[todayRow][21]; got != 5 {
		t.Fatalf("today 21:00 = %d, want 5", got)
	}
	if week.Total != 12 {
		t.Fatalf("total = %d, want 12 (the row 10 days back is outside 7 days)", week.Total)
	}
	if week.Busiest == nil || *week.Busiest != (busiestHour{Hour: 9, Messages: 7}) {
		t.Fatalf("busiest = %+v, want 09:00 with 7 (summed over weekdays)", week.Busiest)
	}

	month := getAnalytics[heatmapResponse](t, fixture, analyticsPath(chatID, "heatmap", "days=30"))
	if month.Total != 112 || month.Busiest == nil || month.Busiest.Hour != 3 {
		t.Fatalf("30-day total = %d, busiest = %+v, want 112 and 03:00", month.Total, month.Busiest)
	}
	// 30 days are four whole weeks plus two days: those two weekdays (today's
	// and yesterday's) occur five times.
	for row, n := range month.Weekdays {
		want := 4
		if row == todayRow || row == yesterdayRow {
			want = 5
		}
		if n != want {
			t.Fatalf("30-day weekdays = %v, want 5 for rows %d and %d and 4 elsewhere", month.Weekdays, todayRow, yesterdayRow)
		}
	}
}

func TestHeatmapAndGrowthAreReadableByMembers(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	// The caller is a plain member: getChatMember reports "member".
	fixture.client.admins[chatID] = adminListJSON(999)

	heat := getAnalytics[heatmapResponse](t, fixture, analyticsPath(chatID, "heatmap", ""))
	if heat.Total != 0 || heat.Busiest != nil || len(heat.Grid) != 7 {
		t.Fatalf("empty heatmap = total %d, busiest %+v, %d rows; want 0, none, 7", heat.Total, heat.Busiest, len(heat.Grid))
	}
	growth := getAnalytics[growthResponse](t, fixture, analyticsPath(chatID, "growth", ""))
	if growth.Points == nil || len(growth.Points) != 0 || growth.Change != nil {
		t.Fatalf("empty growth = %+v, want an empty points array and no change", growth)
	}
}

func TestGrowthReturnsMeasuredDaysAndTheirChange(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	today := overviewDayStart()
	count := func(n int64) *int64 { return &n }
	seedRows(t, &[]models.ChatMemberCount{
		{ChatID: chatID, Day: stats.Day(today.AddDate(0, 0, -40)), Count: count(50)},
		{ChatID: chatID, Day: stats.Day(today.AddDate(0, 0, -5)), Count: count(100)},
		// A claim whose call failed for good: no count, not a point.
		{ChatID: chatID, Day: stats.Day(today.AddDate(0, 0, -2))},
		{ChatID: chatID, Day: stats.Day(today), Count: count(112)},
	})

	week := getAnalytics[growthResponse](t, fixture, analyticsPath(chatID, "growth", ""))
	want := []growthPoint{
		{Day: stats.Day(today.AddDate(0, 0, -5)), Count: 100},
		{Day: stats.Day(today), Count: 112},
	}
	if !reflect.DeepEqual(week.Points, want) {
		t.Fatalf("points = %+v, want %+v", week.Points, want)
	}
	if week.Change == nil || *week.Change != 12 {
		t.Fatalf("change = %v, want +12", week.Change)
	}

	quarter := getAnalytics[growthResponse](t, fixture, analyticsPath(chatID, "growth", "days=90"))
	if len(quarter.Points) != 3 || quarter.Change == nil || *quarter.Change != 62 {
		t.Fatalf("90-day points = %+v, change = %v, want 3 points and +62", quarter.Points, quarter.Change)
	}
}

func TestGrowthWithOnePointHasNoChange(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	n := int64(40)
	seedRows(t, &models.ChatMemberCount{ChatID: chatID, Day: stats.Day(overviewDayStart()), Count: &n})

	growth := getAnalytics[growthResponse](t, fixture, analyticsPath(chatID, "growth", ""))
	if len(growth.Points) != 1 || growth.Change != nil {
		t.Fatalf("growth = %+v, want one point and no change", growth)
	}
}

func TestTopMembersIsAdminOnly(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	fixture.client.admins[chatID] = adminListJSON(999)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, analyticsPath(chatID, "top-members", ""), readFixture(t, "initdata_main.txt"), "", "")
	requireAPIError(t, recorder, http.StatusForbidden, "not_admin")
}

func TestTopMembersRanksTheWindowAndNamesUsers(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	today := overviewDayStart()
	ada := seedNamedUser(t, "Ada", "ada")
	bob := seedNamedUser(t, "Bob", "")
	unknown := int64(6_999_999_902)
	seedRows(t, &[]models.ChatMemberDailyStat{
		{ChatID: chatID, Day: stats.Day(today), UserID: ada, Messages: 3},
		{ChatID: chatID, Day: stats.Day(today.AddDate(0, 0, -1)), UserID: ada, Messages: 4},
		{ChatID: chatID, Day: stats.Day(today), UserID: bob, Messages: 9},
		{ChatID: chatID, Day: stats.Day(today), UserID: unknown, Messages: 1},
		// Outside 7 days: only the 30-day window counts it.
		{ChatID: chatID, Day: stats.Day(today.AddDate(0, 0, -20)), UserID: ada, Messages: 10},
	})

	week := getAnalytics[topMembersResponse](t, fixture, analyticsPath(chatID, "top-members", ""))
	wantWeek := []topMember{{UserID: bob, Messages: 9}, {UserID: ada, Messages: 7}, {UserID: unknown, Messages: 1}}
	if !reflect.DeepEqual(week.Members, wantWeek) {
		t.Fatalf("7-day members = %+v, want %+v", week.Members, wantWeek)
	}
	wantNames := map[string]userName{key(ada): {Name: "Ada", Username: "ada"}, key(bob): {Name: "Bob"}}
	if !reflect.DeepEqual(week.Names, wantNames) {
		t.Fatalf("names = %+v, want %+v (a user the bot never stored is left out)", week.Names, wantNames)
	}

	month := getAnalytics[topMembersResponse](t, fixture, analyticsPath(chatID, "top-members", "days=30&limit=1"))
	if want := []topMember{{UserID: ada, Messages: 17}}; !reflect.DeepEqual(month.Members, want) {
		t.Fatalf("30-day top member = %+v, want %+v", month.Members, want)
	}
}

func TestAnalyticsRejectInvalidQueries(t *testing.T) {
	fixture, chatID := newAnalyticsFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	for _, endpoint := range []string{"heatmap", "growth", "top-members"} {
		for _, query := range []string{"days=13", "days=0", "days=week"} {
			recorder := panelRequest(t, fixture.handler, http.MethodGet, analyticsPath(chatID, endpoint, query), raw, "", "")
			requireAPIError(t, recorder, http.StatusBadRequest, "invalid_days")
		}
	}
	for _, query := range []string{"limit=0", "limit=ten"} {
		recorder := panelRequest(t, fixture.handler, http.MethodGet, analyticsPath(chatID, "top-members", query), raw, "", "")
		requireAPIError(t, recorder, http.StatusBadRequest, "invalid_limit")
	}
}
