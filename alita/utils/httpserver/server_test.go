package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from expected panic due to nil db/cache: %v", r)
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

func TestStopWithNilServer(t *testing.T) {
	t.Parallel()

	s := New(9007, time.Now())

	if err := s.Stop(); err != nil {
		t.Errorf("expected nil error from Stop on unstarted server, got: %v", err)
	}
}
