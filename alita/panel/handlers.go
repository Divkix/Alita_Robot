package panel

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

type chatSummary struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	LastActivity *time.Time `json:"last_activity,omitempty"`
	// Verified is false for every candidate: the session endpoint never fans
	// out Telegram calls, so opening a chat is what proves the role.
	Verified bool `json:"verified"`
}

type sessionResponse struct {
	User         User          `json:"user"`
	Chats        []chatSummary `json:"chats"`
	PinnedChatID *int64        `json:"pinned_chat_id,omitempty"`
}

type chatResponse struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Role     string `json:"role"`
	BotAdmin bool   `json:"bot_admin"`
}

// handleSession answers the signed-in user and the chats where the bot has
// seen them. The lookup is injected and deliberately does not verify each chat
// with Telegram.
func (p *Panel) handleSession(w http.ResponseWriter, r *http.Request) {
	session, ok := p.authenticate(w, r)
	if !ok {
		return
	}
	if !p.allow(w, session) {
		return
	}

	chats := make([]chatSummary, 0)
	if p.deps.LookupChats != nil {
		rows, err := p.deps.LookupChats(r.Context(), session.User.ID, sessionCandidateLimit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "lookup_failed", "could not list chats")
			return
		}
		for _, row := range rows {
			summary := chatSummary{ID: row.ChatId, Title: row.ChatName}
			if !row.LastActivity.IsZero() {
				lastActivity := row.LastActivity
				summary.LastActivity = &lastActivity
			}
			chats = append(chats, summary)
		}
	}

	var pinnedChatID *int64
	if chatID, ok := ParseStartParamChat(session.StartParam); ok {
		pinned := chatID
		pinnedChatID = &pinned
		present := false
		for _, summary := range chats {
			if summary.ID == chatID {
				present = true
				break
			}
		}
		if !present {
			chats = append([]chatSummary{{ID: chatID}}, chats...)
		}
	}

	writeJSON(w, http.StatusOK, sessionResponse{
		User:         session.User,
		Chats:        chats,
		PinnedChatID: pinnedChatID,
	})
}

// handleChat verifies the caller's membership and role in one chat. A member
// who is not an admin still gets a 200 with role readonly; a non-member or a
// chat that is not a group is refused, and a Telegram failure is surfaced.
func (p *Panel) handleChat(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{
		ID:       access.ChatID,
		Title:    access.Title,
		Role:     access.Role,
		BotAdmin: access.BotAdmin,
	})
}

// chatRequest is the preamble of every /chats/{id} endpoint: it parses the id,
// authenticates the caller, applies the per-user rate limit and verifies
// access. When it returns false it has already written the error response.
func (p *Panel) chatRequest(w http.ResponseWriter, r *http.Request) (Session, chatAccess, bool) {
	chatID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || chatID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_chat_id", "chat id must be an integer")
		return Session{}, chatAccess{}, false
	}

	session, ok := p.authenticate(w, r)
	if !ok {
		return Session{}, chatAccess{}, false
	}
	if !p.allow(w, session) {
		return Session{}, chatAccess{}, false
	}

	access, err := p.accessChat(r.Context(), session.User.ID, chatID)
	if err != nil {
		switch {
		case r.Context().Err() != nil:
			writeJSONError(w, http.StatusGatewayTimeout, "timeout", "Telegram did not answer before the panel request deadline")
		case errors.Is(err, errNotGroup):
			writeJSONError(w, http.StatusNotFound, "chat_not_found", "the panel only serves group chats")
		case errors.Is(err, errNotMember):
			writeJSONError(w, http.StatusForbidden, "not_member", "you are not a member of this chat")
		default:
			writeJSONError(w, http.StatusBadGateway, "telegram_error", "Telegram could not return this chat: "+err.Error())
		}
		return Session{}, chatAccess{}, false
	}
	return session, access, true
}
