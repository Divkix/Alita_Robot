package tracing

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/bridges/otellogrus"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

type recordingLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (*recordingLogExporter) Shutdown(context.Context) error   { return nil }
func (*recordingLogExporter) ForceFlush(context.Context) error { return nil }

func TestLogHookExportsScrubbedEntriesLoggedWithoutContext(t *testing.T) {
	exporter := &recordingLogExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	// Same order as production: logredact's hook is installed first.
	logredact.Install(logger)
	logger.AddHook(logHook{otellogrus.NewHook("test", otellogrus.WithLoggerProvider(provider))})

	logger.Error("dial postgres://alita:hunter2secret@db:5432/alita failed")

	if len(exporter.records) != 1 {
		t.Fatalf("exported records = %d, want 1", len(exporter.records))
	}
	body := exporter.records[0].Body().AsString()
	if body == "" || strings.Contains(body, "hunter2secret") {
		t.Fatalf("exported body = %q, want it scrubbed by logredact", body)
	}
}

func TestSignalEnabledHonoursNone(t *testing.T) {
	t.Setenv("OTEL_LOGS_EXPORTER", " None ")
	if signalEnabled("OTEL_LOGS_EXPORTER") {
		t.Fatal(`OTEL_LOGS_EXPORTER="None" left logs enabled`)
	}
	t.Setenv("OTEL_LOGS_EXPORTER", "")
	if !signalEnabled("OTEL_LOGS_EXPORTER") {
		t.Fatal("unset OTEL_LOGS_EXPORTER disabled logs")
	}
}

func TestRegisterHeaderSecretsRedactsHeaderValues(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=ingest-key-123456, x-team = team-secret-9")
	registerHeaderSecrets()

	for _, secret := range []string{"ingest-key-123456", "team-secret-9"} {
		if got := logredact.Scrub("key " + secret); strings.Contains(got, secret) {
			t.Errorf("Scrub() = %q, still contains %q", got, secret)
		}
	}
}
