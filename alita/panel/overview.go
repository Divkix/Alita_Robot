package panel

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

// overviewWindows are the windows the overview accepts, and the buttons the
// screen offers. Anything else is a client bug, not a smaller window.
var overviewWindows = []int{7, 30, 90}

const (
	overviewDefaultDays = 7
	overviewRecentLimit = 5

	// memberCountMemoTTL is how long a chat's Telegram member count is reused.
	// Re-opening the overview twice in a minute must not call Telegram twice.
	memberCountMemoTTL = 60 * time.Second
)

// overviewDay is one UTC day of the activity series. The handler zero-fills
// every day of the window, so the chart never has to guess about gaps.
type overviewDay struct {
	Day      string `json:"day"`
	Messages int64  `json:"messages"`
	Joins    int64  `json:"joins"`
	Leaves   int64  `json:"leaves"`
}

// overviewTotals sums the returned series, so it always matches what is drawn.
type overviewTotals struct {
	Messages int64 `json:"messages"`
	Joins    int64 `json:"joins"`
	Leaves   int64 `json:"leaves"`
}

// overviewEvent is one moderation action reduced to what the dashboard shows:
// who did what to whom and when. No message text is stored, so none can leak.
// The shape is shared with the moderation timeline screen.
type overviewEvent struct {
	ID           int64  `json:"id"`
	Action       string `json:"action"`
	ActorID      int64  `json:"actor_id"`
	TargetUserID int64  `json:"target_user_id"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}

// overviewResponse is the GET shape. Days always has one entry per day of the
// window, oldest first.
type overviewResponse struct {
	MemberCount     int64           `json:"member_count"`
	Days            []overviewDay   `json:"days"`
	Totals          overviewTotals  `json:"totals"`
	ModerationCount int64           `json:"moderation_count"`
	Recent          []overviewEvent `json:"recent"`
	// Names is sent to chat admins only, as on the admin-only timeline.
	Names map[string]userName `json:"names,omitempty"`
}

// parseOverviewDays accepts only the offered windows, so a typo cannot
// silently change what the numbers mean.
func parseOverviewDays(raw string) (int, bool) {
	if raw == "" {
		return overviewDefaultDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	for _, window := range overviewWindows {
		if days == window {
			return days, true
		}
	}
	return 0, false
}

// handleOverview answers the dashboard's first screen: the Telegram member
// count, one point per UTC day for the selected window, the window's totals,
// how many moderation actions it recorded, and the five most recent actions.
// Every chat member with access may read it; nothing here is writable.
func (p *Panel) handleOverview(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	days, ok := parseOverviewDays(r.URL.Query().Get("days"))
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_days", "days must be 7, 30 or 90")
		return
	}

	now := p.deps.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, 1-days)

	rows, err := chatstats.GetRange(access.ChatID, stats.Day(start), stats.Day(now))
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to read chat daily stats")
		writeJSONError(w, http.StatusInternalServerError, "overview_failed", "could not read this chat's activity")
		return
	}
	byDay := make(map[string]models.ChatDailyStat, len(rows))
	for _, row := range rows {
		byDay[row.Day] = row
	}

	series := make([]overviewDay, days)
	var totals overviewTotals
	for i := range series {
		day := stats.Day(start.AddDate(0, 0, i))
		point := overviewDay{Day: day}
		if row, found := byDay[day]; found {
			point.Messages = row.Messages
			point.Joins = row.Joins
			point.Leaves = row.Leaves
		}
		totals.Messages += point.Messages
		totals.Joins += point.Joins
		totals.Leaves += point.Leaves
		series[i] = point
	}

	moderationCount, err := moderation.CountSince(access.ChatID, start)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to count moderation events")
		writeJSONError(w, http.StatusInternalServerError, "overview_failed", "could not read this chat's moderation history")
		return
	}

	recentRows, err := moderation.ListForChat(access.ChatID, time.Time{}, overviewRecentLimit)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to list recent moderation events")
		writeJSONError(w, http.StatusInternalServerError, "overview_failed", "could not read this chat's moderation history")
		return
	}
	recent := make([]overviewEvent, 0, len(recentRows))
	ids := make([]int64, 0, 2*len(recentRows))
	for _, ev := range recentRows {
		ids = append(ids, ev.ActorId, ev.TargetUserId)
		recent = append(recent, overviewEvent{
			ID:           ev.ID,
			Action:       ev.Action,
			ActorID:      ev.ActorId,
			TargetUserID: ev.TargetUserId,
			Reason:       ev.Reason,
			CreatedAt:    ev.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	memberCount, err := p.memberCount(r.Context(), access.ChatID)
	if err != nil {
		log.WithError(err).Warn("[Panel] Failed to read chat member count")
		writeJSONError(w, http.StatusBadGateway, "telegram_error", "Telegram could not return this chat's member count")
		return
	}

	response := overviewResponse{
		MemberCount:     memberCount,
		Days:            series,
		Totals:          totals,
		ModerationCount: moderationCount,
		Recent:          recent,
	}
	if access.Role == RoleAdmin {
		response.Names = userNames(r.Context(), ids)
	}
	writeJSON(w, http.StatusOK, response)
}

// memberCountEntry is one memoised getChatMemberCount answer.
type memberCountEntry struct {
	count   int64
	expires time.Time
}

// memberCount asks Telegram for the chat size and trusts the answer for 60
// seconds, so opening the overview screen twice in a minute makes one API
// call. Only answers are cached; an error is retried on the next request.
func (p *Panel) memberCount(ctx context.Context, chatID int64) (int64, error) {
	now := p.deps.Now()

	p.membersMu.Lock()
	entry, ok := p.members[chatID]
	p.membersMu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.count, nil
	}

	count, err := p.bot.GetChatMemberCountWithContext(ctx, chatID, nil)
	if err != nil {
		return 0, fmt.Errorf("get member count: %w", err)
	}

	p.membersMu.Lock()
	if len(p.members) > 4096 {
		for key, cached := range p.members {
			if !now.Before(cached.expires) {
				delete(p.members, key)
			}
		}
	}
	p.members[chatID] = memberCountEntry{count: count, expires: now.Add(memberCountMemoTTL)}
	p.membersMu.Unlock()

	return count, nil
}
