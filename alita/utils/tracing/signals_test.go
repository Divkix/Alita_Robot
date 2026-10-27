package tracing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/bridges/otellogrus"
	"go.opentelemetry.io/otel"
	otellog "go.opentelemetry.io/otel/log"
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

func TestLogHookExportsWarnSeverityText(t *testing.T) {
	exporter := &recordingLogExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.AddHook(logHook{otellogrus.NewHook("test", otellogrus.WithLoggerProvider(warnTextProvider{provider}))})

	logger.Warn("slow query")
	logger.Error("query failed")

	want := []string{"warn", "error"}
	if len(exporter.records) != len(want) {
		t.Fatalf("exported records = %d, want %d", len(exporter.records), len(want))
	}
	for i, w := range want {
		if got := exporter.records[i].SeverityText(); got != w {
			t.Errorf("record %d severity text = %q, want %q", i, got, w)
		}
	}
	if got := exporter.records[0].Severity(); got != otellog.SeverityWarn {
		t.Errorf("warn record severity = %v, want %v", got, otellog.SeverityWarn)
	}
}

func TestLogHookSkipsSDKErrors(t *testing.T) {
	exporter := &recordingLogExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.AddHook(logHook{otellogrus.NewHook("test", otellogrus.WithLoggerProvider(provider))})

	// A failed export must not queue another record to export.
	logger.WithField(sdkErrorField, true).Warn("exporter failed")
	logger.Info("bot started")

	if len(exporter.records) != 1 || exporter.records[0].Body().AsString() != "bot started" {
		t.Fatalf("exported records = %d, want only the bot's own entry", len(exporter.records))
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

func TestOTLPTargetAcceptsBareAndURLEndpoints(t *testing.T) {
	cases := []struct {
		endpoint     string
		insecure     bool
		wantTarget   string
		wantInsecure bool
	}{
		{"otel-collector:4317", true, "otel-collector:4317", true},
		{"otel-collector:4317", false, "otel-collector:4317", false},
		{"http://otel-collector:4317", false, "otel-collector:4317", true},
		{"https://in-otel.hyperdx.io:4317/", true, "in-otel.hyperdx.io:4317", false},
	}
	for _, c := range cases {
		target, insecure, err := otlpTarget(c.endpoint, c.insecure)
		if err != nil {
			t.Fatalf("otlpTarget(%q) error: %v", c.endpoint, err)
		}
		if target != c.wantTarget || insecure != c.wantInsecure {
			t.Errorf("otlpTarget(%q, %v) = %q, %v; want %q, %v", c.endpoint, c.insecure, target, insecure, c.wantTarget, c.wantInsecure)
		}
	}
}

func TestOTLPTargetRejectsUnsupportedURLs(t *testing.T) {
	for _, endpoint := range []string{"grpc://user:hunter2@otel-collector:4317", "http://", "http://user:hunter2@%zz"} {
		_, _, err := otlpTarget(endpoint, false)
		if err == nil {
			t.Fatalf("otlpTarget(%q) accepted an unsupported endpoint", endpoint)
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("otlpTarget(%q) error leaks the password: %v", endpoint, err)
		}
	}
}

func TestInitTracingHTTPProtocolExportsAllSignalsUnderBasePath(t *testing.T) {
	var mu sync.Mutex
	got := map[string]string{} // path -> authorization header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Grafana Cloud's endpoint is a base URL with a path, like this one.
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL+"/otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=Basic%20dGVzdDp0b2tlbg==")
	t.Setenv("OTEL_EXPORTER_CONSOLE", "")
	t.Setenv("OTEL_LOGS_EXPORTER", "")
	t.Setenv("OTEL_METRICS_EXPORTER", "")
	ensureAppConfig(t)
	resetTracingState(t)
	origHooks := logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	origHandler := otel.GetErrorHandler()
	t.Cleanup(func() {
		logrus.StandardLogger().ReplaceHooks(origHooks)
		otel.SetErrorHandler(origHandler)
	})

	if err := InitTracing(); err != nil {
		t.Fatalf("InitTracing() error = %v", err)
	}
	_, span := StartSpan(context.Background(), "http-export-test")
	span.End()
	logrus.Warn("http export test")
	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/otlp/v1/traces", "/otlp/v1/logs", "/otlp/v1/metrics"} {
		auth, ok := got[path]
		if !ok {
			t.Errorf("nothing exported to %s; got %v", path, got)
		} else if auth != "Basic dGVzdDp0b2tlbg==" {
			t.Errorf("%s Authorization = %q, want the configured header", path, auth)
		}
	}
}

func TestOTLPUsesHTTPRefusesBadSettings(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	if _, err := otlpUsesHTTP("otel-collector:4318"); err == nil {
		t.Error("http/protobuf accepted a host:port endpoint, which the HTTP exporters cannot use")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/json")
	if _, err := otlpUsesHTTP("https://collector:4318"); err == nil {
		t.Error("unsupported protocol http/json accepted")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")
	if useHTTP, err := otlpUsesHTTP("otel-collector:4317"); err != nil || useHTTP {
		t.Errorf("unset protocol = %v, %v; want gRPC as before", useHTTP, err)
	}
}
