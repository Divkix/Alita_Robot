//go:build testtools

package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m))
}

var panelChatSequence atomic.Int64

func uniquePanelChatID() int64 {
	return -1_000_000_000_000 - panelChatSequence.Add(1)
}

func seedPanelChat(t *testing.T, chatID int64, name string, activity time.Time) {
	t.Helper()
	chat := models.Chat{ChatId: chatID, ChatName: name, LastActivity: activity}
	if err := db.DB.Create(&chat).Error; err != nil {
		t.Fatalf("seed chat %d: %v", chatID, err)
	}
	t.Cleanup(func() {
		db.DB.Delete(&models.Chat{}, "chat_id = ?", chatID)
	})
}

// sqliteLookup mirrors the production chat list without the Postgres-only
// jsonb containment, which is why the lookup is injected.
func sqliteLookup(ctx context.Context, _ int64, limit int) ([]models.Chat, error) {
	var chats []models.Chat
	err := db.DB.WithContext(ctx).Model(&models.Chat{}).
		Where("is_inactive = ?", false).
		Order("last_activity DESC, chat_id DESC").
		Limit(limit).
		Find(&chats).Error
	return chats, err
}

type panelFakeClient struct {
	mu       sync.Mutex
	calls    []string
	admins   map[int64]json.RawMessage
	chatErr  error
	botAdmin bool
}

func newPanelFakeClient() *panelFakeClient {
	return &panelFakeClient{admins: make(map[int64]json.RawMessage), botAdmin: true}
}

func (c *panelFakeClient) record(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, method)
}

func (c *panelFakeClient) countCalls(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, call := range c.calls {
		if call == method {
			count++
		}
	}
	return count
}

func (c *panelFakeClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.record(method)

	chatID, _ := strconv.ParseInt(fmt.Sprint(params["chat_id"]), 10, 64)
	switch method {
	case "getChat":
		if c.chatErr != nil {
			return nil, c.chatErr
		}
		return json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Panel Chat %d"}`, chatID, chatID)), nil
	case "getChatAdministrators":
		if response, ok := c.admins[chatID]; ok {
			return response, nil
		}
		return json.RawMessage(`[]`), nil
	case "getChatMember":
		userID, _ := strconv.ParseInt(fmt.Sprint(params["user_id"]), 10, 64)
		status := "member"
		if userID == 999 && c.botAdmin {
			status = "administrator"
		}
		return json.RawMessage(fmt.Sprintf(`{"status":%q,"user":{"id":%d,"is_bot":%t,"first_name":"Fake"}}`, status, userID, userID == 999)), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *panelFakeClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *panelFakeClient) FileURL(token, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func adminListJSON(ids ...int64) json.RawMessage {
	entries := make([]string, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":%t,"first_name":"Admin %d"}}`, id, id == 999, id))
	}
	return json.RawMessage("[" + strings.Join(entries, ",") + "]")
}

type panelFixture struct {
	handler http.Handler
	client  *panelFakeClient
}

func newPanelFixture(t *testing.T) panelFixture {
	t.Helper()
	return newPanelFixtureWithClock(t, fixtureBotToken, fixtureNow)
}

func newPanelFixtureWithClock(t *testing.T, botToken string, now time.Time) panelFixture {
	t.Helper()
	cache.SetupTestMemoryMarshaler(t)

	client := newPanelFakeClient()
	bot := &gotgbot.Bot{
		Token:     botToken,
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"},
	}
	handler := New(bot, Deps{
		SessionMaxAge: 24 * time.Hour,
		LookupChats:   sqliteLookup,
		Now:           func() time.Time { return now },
	})
	return panelFixture{handler: handler, client: client}
}

func panelRequest(t *testing.T, handler http.Handler, method, target, initData, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if initData != "" {
		request.Header.Set(initDataHeader, initData)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func requireAPIError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", recorder.Code, wantStatus, recorder.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", recorder.Body.String(), err)
	}
	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", body.Error.Code, wantCode)
	}
}

func TestSessionReturnsUserAndCandidateChats(t *testing.T) {
	fixture := newPanelFixture(t)
	older := uniquePanelChatID()
	newer := uniquePanelChatID()
	seedPanelChat(t, older, "Older Chat", fixtureNow.Add(-2*time.Hour))
	seedPanelChat(t, newer, "Newer Chat", fixtureNow.Add(-time.Hour))

	recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", readFixture(t, "initdata_main.txt"), "", "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if cookies := recorder.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("session response set cookies: %v", cookies)
	}

	var body sessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if body.User.ID != 279058397 {
		t.Fatalf("user id = %d, want 279058397", body.User.ID)
	}
	if body.PinnedChatID != nil {
		t.Fatalf("pinned chat = %v, want none", *body.PinnedChatID)
	}
	if len(body.Chats) != 2 {
		t.Fatalf("chats = %d, want 2", len(body.Chats))
	}
	if body.Chats[0].ID != newer || body.Chats[1].ID != older {
		t.Fatalf("chat order = %v, want most recently active first", body.Chats)
	}
	for _, chat := range body.Chats {
		if chat.Verified {
			t.Fatalf("session candidate %d was marked verified without opening it", chat.ID)
		}
	}
	if calls := len(fixture.client.calls); calls != 0 {
		t.Fatalf("session made %d Telegram calls, want none: %v", calls, fixture.client.calls)
	}
}

func TestSessionPinsStartParamChat(t *testing.T) {
	fixture := newPanelFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", readFixture(t, "initdata_special_name.txt"), "", "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	var body sessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if body.PinnedChatID == nil || *body.PinnedChatID != -1001234567890 {
		t.Fatalf("pinned chat = %v, want -1001234567890", body.PinnedChatID)
	}
	if len(body.Chats) == 0 || body.Chats[0].ID != -1001234567890 {
		t.Fatalf("chats = %v, want the pinned chat first", body.Chats)
	}
	if body.Chats[0].Verified {
		t.Fatal("pinned chat was marked verified without opening it")
	}
}

func TestSessionSecondRequestWithSameHeaderSucceeds(t *testing.T) {
	fixture := newPanelFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	for i := range 2 {
		recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", raw, "", "application/json")
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i+1, recorder.Code)
		}
		if cookies := recorder.Header().Values("Set-Cookie"); len(cookies) != 0 {
			t.Fatalf("request %d set cookies: %v", i+1, cookies)
		}
	}
}

func TestMissingInitDataHeader(t *testing.T) {
	fixture := newPanelFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", "", "", "application/json")
	requireAPIError(t, recorder, http.StatusUnauthorized, CodeMissingInitData)
}

func TestAuthFailuresReturn401WithStableCodes(t *testing.T) {
	raw := readFixture(t, "initdata_main.txt")

	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	values.Set("user", strings.Replace(values.Get("user"), `"username":"vdkfrost"`, `"username":"attacker"`, 1))
	tampered := values.Encode()

	tests := []struct {
		name    string
		token   string
		now     time.Time
		raw     string
		wantErr string
	}{
		{
			name:    "tampered field",
			token:   fixtureBotToken,
			now:     fixtureNow,
			raw:     tampered,
			wantErr: CodeBadHash,
		},
		{
			name:    "stale auth date",
			token:   fixtureBotToken,
			now:     fixtureNow.Add(25 * time.Hour),
			raw:     raw,
			wantErr: CodeStaleAuthDate,
		},
		{
			name:    "future auth date",
			token:   fixtureBotToken,
			now:     fixtureNow.Add(-time.Minute),
			raw:     raw,
			wantErr: CodeFutureAuthDate,
		},
		{
			name:    "missing hash",
			token:   fixtureBotToken,
			now:     fixtureNow,
			raw:     encodedWithout(t, raw, "hash"),
			wantErr: CodeMissingHash,
		},
		{
			name:    "wrong bot token",
			token:   "123456789:AAHnotTheSigningToken",
			now:     fixtureNow,
			raw:     raw,
			wantErr: CodeBadHash,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPanelFixtureWithClock(t, test.token, test.now)
			recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", test.raw, "", "application/json")
			requireAPIError(t, recorder, http.StatusUnauthorized, test.wantErr)
		})
	}
}

func TestChatReturnsAdminRoleAndBotAdmin(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	fixture.client.admins[chatID] = adminListJSON(279058397, 999)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, "/panel/api/v1/chats/"+strconv.FormatInt(chatID, 10), readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	var body chatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode chat: %v", err)
	}
	if body.ID != chatID {
		t.Fatalf("chat id = %d, want %d", body.ID, chatID)
	}
	if body.Title != fmt.Sprintf("Panel Chat %d", chatID) {
		t.Fatalf("title = %q", body.Title)
	}
	if body.Role != RoleAdmin {
		t.Fatalf("role = %q, want admin", body.Role)
	}
	if !body.BotAdmin {
		t.Fatal("bot_admin = false, want true")
	}
}

func TestChatNonAdminIsReadonly(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	fixture.client.admins[chatID] = adminListJSON(999)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, "/panel/api/v1/chats/"+strconv.FormatInt(chatID, 10), readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 read-only (body %s)", recorder.Code, recorder.Body.String())
	}

	var body chatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode chat: %v", err)
	}
	if body.Role != RoleReadonly {
		t.Fatalf("role = %q, want readonly", body.Role)
	}
	if !body.BotAdmin {
		t.Fatal("bot_admin = false, want true")
	}
}

func TestChatTelegramErrorIsSurfaced(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	fixture.client.chatErr = errors.New("Bad Request: chat not found")

	recorder := panelRequest(t, fixture.handler, http.MethodGet, "/panel/api/v1/chats/"+strconv.FormatInt(chatID, 10), readFixture(t, "initdata_main.txt"), "", "")
	requireAPIError(t, recorder, http.StatusBadGateway, "telegram_error")
	if !strings.Contains(recorder.Body.String(), "chat not found") {
		t.Fatalf("body = %s, want the Telegram error message", recorder.Body.String())
	}
}

func TestInvalidChatID(t *testing.T) {
	fixture := newPanelFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, "/panel/api/v1/chats/not-a-number", readFixture(t, "initdata_main.txt"), "", "")
	requireAPIError(t, recorder, http.StatusBadRequest, "invalid_chat_id")
}

func TestChatAccessMemoAvoidsRepeatedTelegramLookups(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	fixture.client.admins[chatID] = adminListJSON(279058397, 999)
	target := "/panel/api/v1/chats/" + strconv.FormatInt(chatID, 10)

	first := panelRequest(t, fixture.handler, http.MethodGet, target, readFixture(t, "initdata_main.txt"), "", "")
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}
	callsAfterFirst := fixture.client.countCalls("getChat")

	second := panelRequest(t, fixture.handler, http.MethodGet, target, readFixture(t, "initdata_main.txt"), "", "")
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", second.Code)
	}
	if calls := fixture.client.countCalls("getChat"); calls != callsAfterFirst {
		t.Fatalf("getChat calls = %d after second request, want %d within the 60 s memo", calls, callsAfterFirst)
	}
}

func TestRateLimitReturns429WithRetryAfter(t *testing.T) {
	fixture := newPanelFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	for i := range 60 {
		recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", raw, "", "application/json")
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 (body %s)", i+1, recorder.Code, recorder.Body.String())
		}
	}

	recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", raw, "", "application/json")
	requireAPIError(t, recorder, http.StatusTooManyRequests, "rate_limited")
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
}

func TestSecurityHeadersOnPanelResponses(t *testing.T) {
	fixture := newPanelFixture(t)

	for _, target := range []string{"/panel/", "/panel/app.js", "/panel/api/v1/session"} {
		recorder := panelRequest(t, fixture.handler, http.MethodGet, target, "", "", "")
		csp := recorder.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "frame-ancestors https://web.telegram.org https://*.web.telegram.org") {
			t.Fatalf("%s CSP = %q, want the Telegram Web frame-ancestors allowlist", target, csp)
		}
		if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "script-src 'self' https://telegram.org") {
			t.Fatalf("%s CSP = %q, want default-src and script-src as documented", target, csp)
		}
		if got := recorder.Header().Get("X-Frame-Options"); got != "" {
			t.Fatalf("%s X-Frame-Options = %q, want it absent", target, got)
		}
		if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("%s X-Content-Type-Options = %q, want nosniff", target, got)
		}
		if got := recorder.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Fatalf("%s Referrer-Policy = %q, want no-referrer", target, got)
		}
	}
}

func TestStaticAssetsUseCorrectContentTypes(t *testing.T) {
	fixture := newPanelFixture(t)

	tests := []struct {
		target      string
		contentType string
	}{
		{target: "/panel/", contentType: "text/html"},
		{target: "/panel/app.css", contentType: "text/css"},
		{target: "/panel/app.js", contentType: "text/javascript"},
		{target: "/panel/screens/chats.js", contentType: "text/javascript"},
	}
	for _, test := range tests {
		recorder := panelRequest(t, fixture.handler, http.MethodGet, test.target, "", "", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", test.target, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, test.contentType) {
			t.Fatalf("%s Content-Type = %q, want %s", test.target, got, test.contentType)
		}
	}
}

func TestMutatingRequestsRequireJSONContentType(t *testing.T) {
	fixture := newPanelFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", readFixture(t, "initdata_main.txt"), "", "")
	requireAPIError(t, recorder, http.StatusUnsupportedMediaType, "unsupported_media_type")
}

func TestRequestBodyIsCapped(t *testing.T) {
	fixture := newPanelFixture(t)
	body := strings.Repeat(" ", maxRequestBodyBytes+1024)

	recorder := panelRequest(t, fixture.handler, http.MethodPost, "/panel/api/v1/session", readFixture(t, "initdata_main.txt"), body, "application/json")
	requireAPIError(t, recorder, http.StatusRequestEntityTooLarge, "request_too_large")
}
