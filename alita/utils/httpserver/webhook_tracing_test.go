//go:build testtools

package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

func TestWebhookTracesOnlyAuthenticatedRequests(t *testing.T) {
	recorder := tracing.RecordSpansForTest(t)
	s := New(0, time.Now())
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: 1})
	dispatcher.AddHandler(handlers.NewMessage(message.All, func(*gotgbot.Bot, *ext.Context) error {
		return ext.EndGroups
	}))
	if err := s.RegisterWebhook(newHTTPServerTestBot(&httpServerBotClient{}), dispatcher, "test-secret", "https://example.test"); err != nil {
		t.Fatal(err)
	}
	send := func(secret string) int {
		body := `{"update_id":1,"message":{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup"},"text":"hello"}}`
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, req)
		return rr.Code
	}

	if status := send("wrong"); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated update: got %d, want 401", status)
	}
	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("unauthenticated request recorded %d spans, want 0", len(spans))
	}

	if status := send("test-secret"); status != http.StatusOK {
		t.Fatalf("authenticated update: got %d, want 200", status)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want webhook.request and dispatcher.processUpdate", len(spans))
	}
	request, process := spans[0], spans[1]
	if request.Name() != "webhook.request" {
		request, process = process, request
	}
	if process.Name() != "dispatcher.processUpdate" || process.Parent().SpanID() != request.SpanContext().SpanID() {
		t.Fatalf("%s is not a child of %s", process.Name(), request.Name())
	}
}
