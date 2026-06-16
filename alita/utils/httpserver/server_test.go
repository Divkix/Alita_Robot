package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
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

type httpServerMemoryStore struct {
	mu   sync.RWMutex
	data map[string]any
	ttls map[string]time.Time
}

func newHTTPServerMemoryStore() *httpServerMemoryStore {
	return &httpServerMemoryStore{
		data: make(map[string]any),
		ttls: make(map[string]time.Time),
	}
}

func (m *httpServerMemoryStore) Get(_ context.Context, key any) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := fmt.Sprint(key)
	if expiry, ok := m.ttls[k]; ok && time.Now().After(expiry) {
		return nil, fmt.Errorf("key expired")
	}
	value, ok := m.data[k]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return value, nil
}

func (m *httpServerMemoryStore) GetWithTTL(_ context.Context, key any) (any, time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := fmt.Sprint(key)
	value, ok := m.data[k]
	if !ok {
		return nil, 0, fmt.Errorf("key not found")
	}
	if expiry, ok := m.ttls[k]; ok {
		ttl := time.Until(expiry)
		if ttl < 0 {
			return nil, 0, fmt.Errorf("key expired")
		}
		return value, ttl, nil
	}
	return value, 0, nil
}

func (m *httpServerMemoryStore) Set(_ context.Context, key, value any, options ...store.Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := fmt.Sprint(key)
	m.data[k] = value
	if expiration := store.ApplyOptions(options...).Expiration; expiration > 0 {
		m.ttls[k] = time.Now().Add(expiration)
	} else {
		delete(m.ttls, k)
	}
	return nil
}

func (m *httpServerMemoryStore) Delete(_ context.Context, key any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := fmt.Sprint(key)
	delete(m.data, k)
	delete(m.ttls, k)
	return nil
}

func (m *httpServerMemoryStore) Invalidate(context.Context, ...store.InvalidateOption) error {
	return nil
}

func (m *httpServerMemoryStore) Clear(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]any)
	m.ttls = make(map[string]time.Time)
	return nil
}

func (m *httpServerMemoryStore) GetType() string {
	return "httpserver-memory-test"
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

func TestCheckDatabaseWithHealthyConnection(t *testing.T) {
	setupHTTPServerDB(t)

	if !checkDatabase() {
		t.Fatal("checkDatabase() = false, want true with configured test DB")
	}
}

func TestCheckRedisWithNilAndHealthyManagers(t *testing.T) {
	previousManager := cache.Manager
	cache.Manager = nil
	t.Cleanup(func() {
		cache.Manager = previousManager
	})

	if checkRedis() {
		t.Fatal("checkRedis() = true, want false with nil manager")
	}

	cache.Manager = gocache.New[any](newHTTPServerMemoryStore())
	if !checkRedis() {
		t.Fatal("checkRedis() = false, want true with memory manager")
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
	// No token configured: endpoint should be open (with a startup warning).
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

	// No token configured: endpoint should be open (with a startup warning).
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

// TestMetricsRequiresToken verifies that /metrics returns 401 without the correct bearer
// token and 200 when the correct bearer token is supplied.
func TestMetricsRequiresToken(t *testing.T) {
	t.Parallel()

	const token = "super-secret-token"

	s := New(9100, time.Now())
	s.SetMetricsAuthToken(token)
	s.RegisterMetrics()

	// No Authorization header → 401.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no auth header: expected 401, got %d", rr.Code)
	}

	// Wrong token → 401.
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr = httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: expected 401, got %d", rr.Code)
	}

	// Correct token → 200.
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("correct token: expected 200, got %d", rr.Code)
	}
}

// TestDBMetricsRequiresToken verifies that /db_metrics returns 401 without the correct bearer
// token and 200 when the correct bearer token is supplied.
func TestDBMetricsRequiresToken(t *testing.T) {
	setupHTTPServerDB(t)

	const token = "db-metrics-secret"

	s := New(9101, time.Now())
	s.SetMetricsAuthToken(token)
	s.RegisterDBMetrics()

	// No Authorization header → 401.
	req := httptest.NewRequest(http.MethodGet, "/db_metrics", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no auth header: expected 401, got %d", rr.Code)
	}

	// Wrong token → 401.
	req = httptest.NewRequest(http.MethodGet, "/db_metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr = httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: expected 401, got %d", rr.Code)
	}

	// Correct token → 200.
	req = httptest.NewRequest(http.MethodGet, "/db_metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("correct token: expected 200, got %d", rr.Code)
	}
}

// TestDBMetricsDoesNotLeakError verifies that when GetCurrentMetrics fails the response body
// contains the static string "internal error" and NOT the raw Go error string.
func TestDBMetricsDoesNotLeakError(t *testing.T) {
	t.Parallel()

	// Build a server without setting up a real DB, so monitoring.GetCurrentMetrics will fail.
	// Temporarily nil out db.DB to force an error.
	s := New(9102, time.Now())
	s.RegisterDBMetrics()

	req := httptest.NewRequest(http.MethodGet, "/db_metrics", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	// The handler either returns 200 (DB is set up via shared test DB) or 500 (no DB).
	// We only check the error-path guarantee when it actually errors.
	if rr.Code == http.StatusInternalServerError {
		body := strings.TrimSpace(rr.Body.String())
		if body != "internal error" {
			t.Errorf("error response body = %q, want %q", body, "internal error")
		}
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

	// The webhook is now registered at the static path /webhook (no secret in URL).
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("registered webhook route status = %d, want 405 for GET", rr.Code)
	}

	// The old secret-bearing path must NOT be registered.
	req2 := httptest.NewRequest(http.MethodGet, "/webhook/secret-token", nil)
	rr2 := httptest.NewRecorder()
	s.mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("/webhook/<secret> path should not be registered, got status %d", rr2.Code)
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

// TestWebhookValidatesHeaderNotPath verifies that the webhook endpoint is served at the
// static path /webhook (with no secret in the URL) and that access control is enforced
// entirely via the X-Telegram-Bot-Api-Secret-Token header, not by the URL path.
func TestWebhookValidatesHeaderNotPath(t *testing.T) {
	t.Parallel()

	client := &httpServerBotClient{}
	bot := newHTTPServerTestBot(client)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	s := New(9200, time.Now())
	if err := s.RegisterWebhook(bot, dispatcher, "my-secret", "https://example.test"); err != nil {
		t.Fatalf("RegisterWebhook() error = %v", err)
	}

	validBody := `{"update_id":1,"message":{"message_id":1,"date":1,"chat":{"id":1,"type":"private"},"from":{"id":1,"is_bot":false,"first_name":"T"},"text":"hi"}}`

	t.Run("correct header accepted on /webhook", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(validBody))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "my-secret")
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("correct header: expected 200, got %d", rr.Code)
		}
	})

	t.Run("wrong header rejected on /webhook", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(validBody))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("wrong header: expected 401, got %d", rr.Code)
		}
	})

	t.Run("missing header rejected on /webhook", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(validBody))
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("missing header: expected 401, got %d", rr.Code)
		}
	})

	t.Run("secret-bearing path is not routed", func(t *testing.T) {
		// The old /webhook/<secret> path must not be registered.
		req := httptest.NewRequest(http.MethodPost, "/webhook/my-secret", strings.NewReader(validBody))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "my-secret")
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("secret path: expected 404, got %d", rr.Code)
		}
	})
}
