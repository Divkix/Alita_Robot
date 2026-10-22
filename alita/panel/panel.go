// Package panel serves the Telegram Mini App admin dashboard. The handler owns
// authentication, routing, JSON encoding, security headers and rate limiting;
// the HTTP server only mounts it.
package panel

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

//go:embed web
var webFiles embed.FS

const (
	initDataHeader = "X-Telegram-Init-Data"

	// maxRequestBodyBytes caps every mutating panel request. The panel has no
	// bulk uploads yet; 64 KiB is far above any current JSON body.
	maxRequestBodyBytes = 64 * 1024

	sessionCandidateLimit = 50
)

// Deps is everything the panel needs from the bot and its configuration. Every
// field is injected so handler tests run without Telegram or Postgres.
type Deps struct {
	// SessionMaxAge is the maximum accepted initData auth_date age.
	SessionMaxAge time.Duration
	// LookupChats returns active chats whose tracked users include userID,
	// most recently active first. Production wires chats.ListChatsWithUser.
	LookupChats func(ctx context.Context, userID int64, limit int) ([]models.Chat, error)
	// Now is the clock used for initData validation, memo expiry and rate
	// limiting. Nil means time.Now.
	Now func() time.Time
}

// New builds the panel handler. Mount it at /panel and /panel/.
func New(bot *gotgbot.Bot, deps Deps) http.Handler {
	if deps.SessionMaxAge <= 0 {
		deps.SessionMaxAge = DefaultSessionMaxAge
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		// The embedded directory is fixed at build time; a failure here is a
		// programming error, not an operator condition.
		panic("panel: embedded web assets are missing: " + err.Error())
	}

	p := &Panel{
		bot:     bot,
		deps:    deps,
		web:     web,
		mux:     http.NewServeMux(),
		limiter: newRateLimiter(deps.Now),
		access:  make(map[accessKey]accessEntry),
	}
	p.routes()
	return p.handler()
}

// Panel is the panel handler state. Later tickets add routes in routes and
// handlers next to the session and chat handlers.
type Panel struct {
	bot     *gotgbot.Bot
	deps    Deps
	web     fs.FS
	mux     *http.ServeMux
	limiter *rateLimiter

	accessMu sync.Mutex
	access   map[accessKey]accessEntry
}

// routes is the single place panel endpoints are registered so later tickets
// append one line each.
func (p *Panel) routes() {
	p.mux.HandleFunc("POST /panel/api/v1/session", p.handleSession)
	p.mux.HandleFunc("GET /panel/api/v1/chats/{id}", p.handleChat)

	p.mux.Handle("GET /panel/", http.StripPrefix("/panel/", http.FileServer(http.FS(p.web))))
}

// handler applies the panel-wide response policy before routing: security
// headers on every response, and a JSON content-type plus body cap on
// mutating methods.
func (p *Panel) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if mutatingMethod(r.Method) {
			if !isJSONContentType(r.Header.Get("Content-Type")) {
				writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
				return
			}
			if r.ContentLength > maxRequestBodyBytes {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}

		p.mux.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy allows Telegram Web to embed the panel in an iframe.
// frame-ancestors is the only directive that can express that allowlist, which
// is why the panel never sends X-Frame-Options.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' https://telegram.org; " +
	"style-src 'self'; img-src 'self' data:; connect-src 'self'; " +
	"frame-ancestors https://web.telegram.org https://*.web.telegram.org; " +
	"base-uri 'none'; form-action 'none'"

func mutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isJSONContentType(value string) bool {
	if value == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

// authenticate verifies the initData header and writes a 401 with a stable code
// when it is missing or invalid.
func (p *Panel) authenticate(w http.ResponseWriter, r *http.Request) (Session, bool) {
	session, err := VerifyInitData(
		r.Header.Get(initDataHeader),
		p.bot.Token,
		p.deps.Now(),
		p.deps.SessionMaxAge,
	)
	if err != nil {
		code := CodeMissingInitData
		var authErr *AuthError
		if errors.As(err, &authErr) {
			code = authErr.Code
		}
		writeJSONError(w, http.StatusUnauthorized, code, err.Error())
		return Session{}, false
	}
	return session, true
}

// allow applies the per-(user, chat) rate limit, writing 429 with Retry-After
// when the window is exhausted.
func (p *Panel) allow(w http.ResponseWriter, session Session, chatID int64) bool {
	retryAfter := p.limiter.allow(session.User.ID, chatID)
	if retryAfter <= 0 {
		return true
	}
	seconds := int64((retryAfter + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
	return false
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.WithError(err).Debug("[Panel] Failed to encode response")
	}
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

// WebAppURL is the origin of the inline web_app button that opens a chat.
func WebAppURL(publicURL string, chatID int64) string {
	return strings.TrimRight(publicURL, "/") + "?chat=" + strconv.FormatInt(chatID, 10)
}

// StartDeepLink is the PM deep link that opens the panel from a group button
// when Mini App short names are not configured.
func StartDeepLink(botUsername string, chatID int64) string {
	return "https://t.me/" + botUsername + "?start=panel_" + strconv.FormatInt(chatID, 10)
}

// EntryURL is the URL a group /settings or /help button should use: the direct
// Mini App link when a short name is configured, else the PM deep link.
func EntryURL(botUsername, shortName string, chatID int64) string {
	if shortName != "" {
		return "https://t.me/" + botUsername + "/" + shortName + "?startapp=chat_" + strconv.FormatInt(chatID, 10)
	}
	return StartDeepLink(botUsername, chatID)
}

// ParseStartParamChat extracts the chat ID from a start_param of the form
// chat_<id>. It returns false for anything else.
func ParseStartParamChat(startParam string) (int64, bool) {
	if !strings.HasPrefix(startParam, "chat_") {
		return 0, false
	}
	chatID, err := strconv.ParseInt(strings.TrimPrefix(startParam, "chat_"), 10, 64)
	if err != nil || chatID == 0 {
		return 0, false
	}
	return chatID, true
}
