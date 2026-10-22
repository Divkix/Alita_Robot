package panel

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

const (
	// analyticsTimezone is the zone the heatmap's weekdays and hours are in.
	// The counters are stored per UTC hour and no per-chat timezone exists
	// yet, so the screens say UTC instead of guessing the viewer's zone.
	analyticsTimezone = "UTC"

	topMembersDefault = 10
	topMembersMax     = 50
)

// heatmapResponse is the GET /heatmap shape. Grid is 7 rows (Monday first) of
// 24 hours each, every cell the window's messages in that weekday and hour.
// Weekdays counts how often each row's weekday occurs in the window (a 30-day
// window has two weekdays five times), so a client can compare rows by their
// average per day instead of by totals.
type heatmapResponse struct {
	Timezone string       `json:"timezone"`
	Days     int          `json:"days"`
	From     string       `json:"from"`
	To       string       `json:"to"`
	Grid     [][]int64    `json:"grid"`
	Weekdays []int        `json:"weekdays"`
	Total    int64        `json:"total"`
	Busiest  *busiestHour `json:"busiest,omitempty"`
}

// busiestHour is the hour of day with the most messages in the window, summed
// over its weekdays.
type busiestHour struct {
	Hour     int   `json:"hour"`
	Messages int64 `json:"messages"`
}

// growthPoint is one measured day of the chat's Telegram member count.
type growthPoint struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// growthResponse is the GET /growth shape. Points has only the days a count
// was taken; Change is the last point minus the first, absent with fewer than
// two points.
type growthResponse struct {
	Days   int           `json:"days"`
	From   string        `json:"from"`
	To     string        `json:"to"`
	Points []growthPoint `json:"points"`
	Change *int64        `json:"change,omitempty"`
}

// topMember is one ranked user.
type topMember struct {
	UserID   int64 `json:"user_id"`
	Messages int64 `json:"messages"`
}

type topMembersResponse struct {
	Days    int                 `json:"days"`
	From    string              `json:"from"`
	To      string              `json:"to"`
	Members []topMember         `json:"members"`
	Names   map[string]userName `json:"names,omitempty"`
}

// analyticsWindow parses ?days= like the overview and returns the window's
// first and last UTC day keys. It writes the 400 itself.
func (p *Panel) analyticsWindow(w http.ResponseWriter, r *http.Request) (days int, from, to string, ok bool) {
	days, ok = parseOverviewDays(r.URL.Query().Get("days"))
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_days", "days must be 7, 30 or 90")
		return 0, "", "", false
	}
	now := p.deps.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1-days)
	return days, stats.Day(start), stats.Day(now), true
}

// weekdayRow maps a weekday to its grid row, Monday first.
func weekdayRow(day time.Weekday) int {
	return (int(day) + 6) % 7
}

// handleHeatmap answers the hourly heatmap: the window's messages by weekday
// and UTC hour. It shows counts only, so every member with access may read it.
func (p *Panel) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	days, from, to, ok := p.analyticsWindow(w, r)
	if !ok {
		return
	}

	rows, err := chatstats.GetHourlyRange(access.ChatID, from, to)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to read chat hourly stats")
		writeJSONError(w, http.StatusInternalServerError, "heatmap_failed", "could not read this chat's activity")
		return
	}

	response := heatmapResponse{
		Timezone: analyticsTimezone,
		Days:     days,
		From:     from,
		To:       to,
		Grid:     make([][]int64, 7),
		Weekdays: make([]int, 7),
	}
	for i := range response.Grid {
		response.Grid[i] = make([]int64, 24)
	}
	if start, err := time.Parse(time.DateOnly, from); err == nil {
		for i := range days {
			response.Weekdays[weekdayRow(start.AddDate(0, 0, i).Weekday())]++
		}
	}
	hours := make([]int64, 24)
	for _, row := range rows {
		date, err := time.Parse(time.DateOnly, row.Day)
		if err != nil || row.Hour < 0 || row.Hour > 23 {
			continue
		}
		response.Grid[weekdayRow(date.Weekday())][row.Hour] += row.Messages
		hours[row.Hour] += row.Messages
		response.Total += row.Messages
	}
	for hour, messages := range hours {
		if messages > 0 && (response.Busiest == nil || messages > response.Busiest.Messages) {
			response.Busiest = &busiestHour{Hour: hour, Messages: messages}
		}
	}
	writeJSON(w, http.StatusOK, response)
}

// handleGrowth answers the member growth chart: the counts the daily snapshot
// took in the window. Members already see the live count on the overview, so
// every member with access may read it.
func (p *Panel) handleGrowth(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	days, from, to, ok := p.analyticsWindow(w, r)
	if !ok {
		return
	}

	rows, err := chatstats.GetMemberCounts(access.ChatID, from, to)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to read chat member counts")
		writeJSONError(w, http.StatusInternalServerError, "growth_failed", "could not read this chat's member history")
		return
	}

	response := growthResponse{Days: days, From: from, To: to, Points: make([]growthPoint, 0, len(rows))}
	for _, row := range rows {
		if row.Count == nil {
			continue
		}
		response.Points = append(response.Points, growthPoint{Day: row.Day, Count: *row.Count})
	}
	if n := len(response.Points); n >= 2 {
		change := response.Points[n-1].Count - response.Points[0].Count
		response.Change = &change
	}
	writeJSON(w, http.StatusOK, response)
}

// handleTopMembers ranks the chat's most active users in the window. It names
// users, so it is admin-only like the timeline.
func (p *Panel) handleTopMembers(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	if !requireAdmin(w, access) {
		return
	}
	days, from, to, ok := p.analyticsWindow(w, r)
	if !ok {
		return
	}
	limit, ok := parsePageLimit(w, r.URL.Query().Get("limit"), topMembersDefault, topMembersMax)
	if !ok {
		return
	}

	rows, err := chatstats.TopMembers(access.ChatID, from, to, limit)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to read top chat members")
		writeJSONError(w, http.StatusInternalServerError, "top_members_failed", "could not read this chat's members")
		return
	}

	response := topMembersResponse{Days: days, From: from, To: to, Members: make([]topMember, 0, len(rows))}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		response.Members = append(response.Members, topMember{UserID: row.UserID, Messages: row.Messages})
		ids = append(ids, row.UserID)
	}
	response.Names = userNames(r.Context(), ids)
	writeJSON(w, http.StatusOK, response)
}
