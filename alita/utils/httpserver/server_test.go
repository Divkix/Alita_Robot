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

	startTime := time.Now().Add(-time.Hour) // Use a distinct past time to verify it’s stored
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
	// No header set

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

	// POST with no secret header → reads body OK, then validateWebhook returns false → 401
	body := strings.NewReader("{}")
	req := httptest.NewRequest(http.MethodPost, "/webhook/mysecret", body)
	rr := httptest.NewRecorder()

	s.webhookHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestStopWithNilServer(t *testing.T) {
	t.Parallel()

	s := New(9007, time.Now())
	// s.server is nil — never called Start()

	if err := s.Stop(); err != nil {
		t.Errorf("expected nil error from Stop on unstarted server, got: %v", err)
	}
}
