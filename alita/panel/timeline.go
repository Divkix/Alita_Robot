package panel

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
)

const (
	timelinePageDefault = 50
	timelinePageMax     = 200
)

// timelineActions is the set an ?action= filter accepts.
var timelineActions = []string{
	models.ModerationActionBan,
	models.ModerationActionKick,
	models.ModerationActionMute,
	models.ModerationActionUnban,
}

// timelineEvent is one row of the dashboard timeline. It carries no message
// text and no message id, and reason stays empty for automated actors.
type timelineEvent struct {
	ID           int64  `json:"id"`
	Action       string `json:"action"`
	ActorID      int64  `json:"actor_id"`
	TargetUserID int64  `json:"target_user_id"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}

type timelineResponse struct {
	Events     []timelineEvent `json:"events"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// handleTimeline lists a chat's moderation events newest first. The cursor is
// the (created_at, id) keyset of the last returned row, so a row inserted
// between two pages neither repeats nor hides another; paging stops at the
// retention boundary the sweep deletes past.
func (p *Panel) handleTimeline(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}

	values := r.URL.Query()
	limit, ok := parseTimelineLimit(w, values.Get("limit"))
	if !ok {
		return
	}
	action := strings.TrimSpace(values.Get("action"))
	if action != "" && !slices.Contains(timelineActions, action) {
		writeJSONError(w, http.StatusBadRequest, "invalid_action", "action must be ban, kick, mute or unban")
		return
	}
	var actorID int64
	if raw := strings.TrimSpace(values.Get("actor")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_actor", "actor must be a user id")
			return
		}
		actorID = parsed
	}

	filter := moderation.TimelineFilter{
		ChatID:  access.ChatID,
		Since:   p.deps.Now().UTC().Add(-moderation.EventRetention),
		Action:  action,
		ActorID: actorID,
		// One extra row proves whether another page exists.
		Limit: limit + 1,
	}
	if raw := values.Get("cursor"); raw != "" {
		before, beforeID, valid := parseTimelineCursor(raw)
		if !valid {
			writeJSONError(w, http.StatusBadRequest, "invalid_cursor", "cursor is not a timeline cursor")
			return
		}
		filter.Before, filter.BeforeID = before, beforeID
	}

	rows, err := moderation.ListTimeline(filter)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to read moderation timeline")
		writeJSONError(w, http.StatusInternalServerError, "timeline_failed", "could not read the moderation timeline")
		return
	}

	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = timelineCursor(rows[len(rows)-1])
	}
	events := make([]timelineEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, eventView(row))
	}
	writeJSON(w, http.StatusOK, timelineResponse{Events: events, NextCursor: next})
}

// eventView maps one stored event to the API shape. A non-positive actor is
// the automated sentinel, so its reason is never exposed.
func eventView(ev models.ModerationEvent) timelineEvent {
	view := timelineEvent{
		ID:           ev.ID,
		Action:       ev.Action,
		ActorID:      ev.ActorId,
		TargetUserID: ev.TargetUserId,
		CreatedAt:    ev.CreatedAt.UTC().Format(time.RFC3339),
	}
	if ev.ActorId > 0 {
		view.Reason = ev.Reason
	}
	return view
}

func parseTimelineLimit(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		return timelinePageDefault, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
		return 0, false
	}
	return min(parsed, timelinePageMax), true
}

// timelineCursor encodes a row's (created_at, id) pair. UnixNano keeps the
// ordering key exact on both databases.
func timelineCursor(ev models.ModerationEvent) string {
	return fmt.Sprintf("%d:%d", ev.CreatedAt.UnixNano(), ev.ID)
}

func parseTimelineCursor(raw string) (time.Time, int64, bool) {
	nanos, id, found := strings.Cut(raw, ":")
	if !found {
		return time.Time{}, 0, false
	}
	value, err := strconv.ParseInt(nanos, 10, 64)
	if err != nil {
		return time.Time{}, 0, false
	}
	eventID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || eventID <= 0 {
		return time.Time{}, 0, false
	}
	return time.Unix(0, value).UTC(), eventID, true
}
