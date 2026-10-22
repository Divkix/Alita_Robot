package panel

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

const (
	warnPageDefault = 50
	warnPageMax     = 200
)

// warnEntryView is one warning as the panel lists it. Expired uses the same
// predicate /warns counts with, Automatic marks a warn the bot issued itself,
// and an anonymous admin's account is hidden like a backup export hides it.
type warnEntryView struct {
	ID        uint       `json:"id"`
	Reason    string     `json:"reason"`
	WarnedBy  int64      `json:"warned_by"`
	Anonymous bool       `json:"anonymous"`
	Automatic bool       `json:"automatic"`
	WarnedAt  *time.Time `json:"warned_at,omitempty"`
	Expired   bool       `json:"expired"`
}

// warnUserView is one warned user of the chat with the live entry count.
type warnUserView struct {
	UserID  int64           `json:"user_id"`
	Active  int             `json:"active"`
	Entries []warnEntryView `json:"entries"`
}

// warningsResponse is the GET shape; ReadOnly disables the remove buttons and
// NextCursor is the last user id of this page when another page follows.
type warningsResponse struct {
	Users      []warnUserView `json:"users"`
	NextCursor int64          `json:"next_cursor,omitempty"`
	ReadOnly   bool           `json:"read_only"`
}

type warningRemoveRequest struct {
	UserID int64 `json:"user_id"`
}

type warningRemoveResponse struct {
	Removed bool `json:"removed"`
	Active  int  `json:"active"`
}

// handleWarnings lists one page of warned users with their entries, expired
// ones labelled. Reads never prune expired entries, so the label is all that
// separates them from live warns. Who warned whom is admin-only data.
func (p *Panel) handleWarnings(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	if !requireAdmin(w, access) {
		return
	}

	values := r.URL.Query()
	limit, ok := parsePageLimit(w, values.Get("limit"), warnPageDefault, warnPageMax)
	if !ok {
		return
	}
	var afterUserID int64
	if raw := values.Get("cursor"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_cursor", "cursor must be a non-negative user id")
			return
		}
		afterUserID = parsed
	}

	// One extra user proves whether another page exists.
	rows, err := warns.ListChatWarns(access.ChatID, afterUserID, limit+1)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to read warnings")
		writeJSONError(w, http.StatusInternalServerError, "warnings_failed", "could not read warnings")
		return
	}
	next := int64(0)
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].UserID
	}
	expiry := warns.GetWarnSettingContext(r.Context(), access.ChatID).WarnExpiry

	users := make([]warnUserView, 0, len(rows))
	for _, row := range rows {
		view := warnUserView{UserID: row.UserID, Entries: make([]warnEntryView, 0, len(row.Entries))}
		for _, entry := range row.Entries {
			expired := warns.EntryExpired(entry, expiry)
			if !expired {
				view.Active++
			}
			entryView := warnEntryView{
				ID:        entry.ID,
				Reason:    entry.Reason,
				WarnedBy:  entry.WarnedBy,
				Anonymous: entry.Anonymous,
				Automatic: entry.WarnedBy == models.WarnedByBot,
				WarnedAt:  entry.WarnedAt,
				Expired:   expired,
			}
			if entry.Anonymous {
				entryView.WarnedBy = 0
			}
			view.Entries = append(view.Entries, entryView)
		}
		users = append(users, view)
	}

	writeJSON(w, http.StatusOK, warningsResponse{
		Users:      users,
		NextCursor: next,
		ReadOnly:   !access.BotAdmin,
	})
}

// handleRemoveWarning removes one warning for a user through warns.RemoveWarn,
// the function /rmwarn and /unwarn call, so the row locks, the entry
// reconciliation and the cache invalidation are identical to the command path.
// Both commands require a plain chat admin, so the panel asks for permNone.
func (p *Panel) handleRemoveWarning(w http.ResponseWriter, r *http.Request) {
	session, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	if !p.requirePermission(w, r, access, session.User.ID, permNone) {
		return
	}

	var request warningRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body", "body must be a JSON object with user_id")
		return
	}
	if request.UserID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_user_id", "user_id must be a positive integer")
		return
	}

	removed, err := warns.RemoveWarn(request.UserID, access.ChatID)
	if err != nil {
		log.WithError(err).Error("[Panel] Failed to remove warning")
		writeJSONError(w, http.StatusInternalServerError, "remove_failed", "could not remove this warning")
		return
	}
	active, _ := warns.GetWarnsContext(r.Context(), request.UserID, access.ChatID)
	writeJSON(w, http.StatusOK, warningRemoveResponse{Removed: removed, Active: active})
}
