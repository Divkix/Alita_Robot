package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serveServerRoute(t *testing.T, s *Server, target string) int {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestPanelRoutesAreNotFoundWhenNotRegistered(t *testing.T) {
	s := New(0, time.Now())

	for _, target := range []string{"/panel", "/panel/", "/panel/api/v1/session", "/panel/app.js"} {
		if code := serveServerRoute(t, s, target); code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404 while the panel is disabled", target, code)
		}
	}
}

func TestRegisterPanelMountsPanelAndAPI(t *testing.T) {
	s := New(0, time.Now())
	s.RegisterPanel(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, target := range []string{"/panel", "/panel/", "/panel/api/v1/session", "/panel/app.js"} {
		if code := serveServerRoute(t, s, target); code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want the panel handler", target, code)
		}
	}
}

func TestRegisterPanelIgnoresNilHandler(t *testing.T) {
	s := New(0, time.Now())
	s.RegisterPanel(nil)

	if code := serveServerRoute(t, s, "/panel"); code != http.StatusNotFound {
		t.Fatalf("/panel status = %d, want 404 for a nil handler", code)
	}
}
