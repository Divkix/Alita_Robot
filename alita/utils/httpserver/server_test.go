package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type httpServerBotClient struct {
	mu      sync.Mutex
	methods []string
}

func (c *httpServerBotClient) RequestWithContext(_ context.Context, _ string, method string, _ map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.mu.Lock()
	c.methods = append(c.methods, method)
	c.mu.Unlock()

	switch method {
	case "setWebhook", "deleteWebhook":
		return json.RawMessage(`true`), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *httpServerBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *httpServerBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func (c *httpServerBotClient) called(method string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, got := range c.methods {
		if got == method {
			return true
		}
	}
	return false
}

func newHTTPServerTestBot(client *httpServerBotClient) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User: gotgbot.User{
			Id:       999,
			IsBot:    true,
			Username: "AlitaTestBot",
		},
	}
}

var (
	httpServerDBOnce sync.Once
	httpServerDBErr  error
)

func setupHTTPServerDB(t *testing.T) {
	t.Helper()

	httpServerDBOnce.Do(func() {
		if db.DB == nil {
			db.DB, httpServerDBErr = gorm.Open(
				sqlite.Open("file:httpserver_test?mode=memory&cache=shared"),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
			)
			if httpServerDBErr != nil {
				return
			}
		}
		httpServerDBErr = db.DB.AutoMigrate(&db.Chat{}, &db.User{})
	})
	if httpServerDBErr != nil {
		t.Fatalf("setup HTTP server DB: %v", httpServerDBErr)
	}
}

func TestNewServer(t *testing.T) {
	t.Parallel()

	startTime := time.Now().Add(-time.Hour)
	s := New(8080, startTime)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.port != 8080 {
		t.Errorf("expected port 8080, got %d", s.port)
	}
	if s.mux == nil {
		t.Error("expected non-nil mux")
	}
	if !s.startTime.Equal(startTime) {
		t.Errorf("expected startTime %v, got %v", startTime, s.startTime)
	}
}

func TestValidateWebhookValidSecret(t *testing.T) {
	t.Parallel()

	s := New(9000, time.Now())
	s.secret = "mysecret"

	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", nil)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "mysecret")

	if !s.validateWebhook(req) {
		t.Error("expected validateWebhook to return true for matching secret")
	}
}

func TestValidateWebhookWrongSecret(t *testing.T) {
	t.Parallel()

	s := New(9001, time.Now())
	s.secret = "mysecret"

	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", nil)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrongsecret")

	if s.validateWebhook(req) {
		t.Error("expected validateWebhook to return false for mismatched secret")
	}
}

func TestValidateWebhookEmptyServerSecret(t *testing.T) {
	t.Parallel()

	s := New(9002, time.Now())
	s.secret = ""

	req := httptest.NewRequest(http.MethodPost, "/webhook/", nil)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "anything")

	if s.validateWebhook(req) {
		t.Error("expected validateWebhook to return false when server secret is empty")
	}
}

func TestValidateWebhookMissingHeader(t *testing.T) {
	t.Parallel()

	s := New(9003, time.Now())
	s.secret = "mysecret"

	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", nil)
	if s.validateWebhook(req) {
		t.Error("expected validateWebhook to return false when header is missing")
	}
}

func TestValidateWebhookEmptyHeader(t *testing.T) {
	t.Parallel()

	s := New(9004, time.Now())
	s.secret = "mysecret"

	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", nil)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "")

	if s.validateWebhook(req) {
		t.Error("expected validateWebhook to return false when header is empty")
	}
}

func TestWebhookHandlerMethodNotAllowed(t *testing.T) {
	t.Parallel()

	s := New(9005, time.Now())
	s.secret = "mysecret"

	req := httptest.NewRequest(http.MethodGet, "/webhook/mysecret", nil)
	rr := httptest.NewRecorder()

	s.webhookHandler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}
}

func TestWebhookHandlerUnauthorized(t *testing.T) {
	t.Parallel()

	s := New(9006, time.Now())
	s.secret = "mysecret"

	body := strings.NewReader("{}")
	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", body)
	rr := httptest.NewRecorder()

	s.webhookHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestWebhookHandlerRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	s := New(9006, time.Now())
	s.secret = "mysecret"

	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", strings.NewReader("{"))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "mysecret")
	rr := httptest.NewRecorder()

	s.webhookHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestWebhookHandlerRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	s := New(9006, time.Now())
	s.secret = "mysecret"
	body := strings.NewReader(strings.Repeat("x", maxRequestBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", body)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "mysecret")
	rr := httptest.NewRecorder()

	s.webhookHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestWebhookHandlerAcceptsAuthorizedTelegramUpdate(t *testing.T) {
	client := &httpServerBotClient{}
	s := New(9006, time.Now())
	s.secret = "mysecret"
	s.bot = newHTTPServerTestBot(client)
	s.dispatcher = ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	body := `{"update_id":1,"message":{"message_id":2,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Webhook Chat"},"from":{"id":42,"is_bot":false,"first_name":"Member"},"text":"hello"}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", strings.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "mysecret")
	rr := httptest.NewRecorder()

	s.webhookHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "OK" {
		t.Errorf("expected OK response body, got %q", rr.Body.String())
	}
}

func TestPprofHandler(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rr := httptest.NewRecorder()

	pprofHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "profile") && !strings.Contains(body, "Profiles") && !strings.Contains(body, "Types of profiles") {
		t.Errorf("expected pprof content in response body, got: %s", body)
	}
}

func TestRegisterHealth(t *testing.T) {
	s := New(8080, time.Now())
	s.RegisterHealth()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	// This test intentionally calls a handler that may panic with nil db/cache.
	// We catch the panic and report it via Logf because the route still responds.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("recovered panic in /health handler: %v", r)
		}
	}()

	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 200 or 503, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %s", ct)
	}
}

func TestRegisterMetrics(t *testing.T) {
	s := New(8080, time.Now())
	s.RegisterMetrics()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", ct)
	}
}

func TestRegisterDBMetrics(t *testing.T) {
	setupHTTPServerDB(t)

	s := New(8080, time.Now())
	s.RegisterDBMetrics()

	req := httptest.NewRequest(http.MethodGet, "/db_metrics", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %s", ct)
	}
	if !strings.Contains(rr.Body.String(), "open_connections") {
		t.Errorf("expected database metrics JSON, got %s", rr.Body.String())
	}
}

func TestRegisterPPROF(t *testing.T) {
	s := New(8080, time.Now())
	s.RegisterPPROF()

	paths := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/threadcreate",
		"/debug/pprof/block",
		"/debug/pprof/mutex",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 for %s, got %d", path, rr.Code)
		}
	}

	if !s.pprofEnabled {
		t.Error("expected pprofEnabled to be true after registration")
	}
}

func TestRegisterWebhookConfiguresTelegramWebhook(t *testing.T) {
	client := &httpServerBotClient{}
	bot := newHTTPServerTestBot(client)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	s := New(8080, time.Now())

	err := s.RegisterWebhook(bot, dispatcher, "secret-token", "https://example.test")
	if err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}
	if !s.webhookEnabled {
		t.Fatal("webhookEnabled = false, want true")
	}
	if s.secret != "secret-token" {
		t.Fatalf("secret = %q, want secret-token", s.secret)
	}
	if !client.called("setWebhook") {
		t.Fatal("setWebhook was not called")
	}

	req := httptest.NewRequest(http.MethodGet, "/webhook/secret-token", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("registered webhook route status = %d, want 405 for GET", rr.Code)
	}
}

func TestStartAndStopEphemeralServer(t *testing.T) {
	client := &httpServerBotClient{}
	s := New(0, time.Now())
	s.bot = newHTTPServerTestBot(client)
	s.webhookEnabled = true

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if s.server == nil {
		t.Fatal("server was not initialized by Start")
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !client.called("deleteWebhook") {
		t.Fatal("deleteWebhook was not called on Stop for webhook-enabled server")
	}
}

func TestStopWithNilServer(t *testing.T) {
	t.Parallel()

	s := New(9007, time.Now())

	if err := s.Stop(); err != nil {
		t.Errorf("expected nil error from Stop on unstarted server, got: %v", err)
	}
}
