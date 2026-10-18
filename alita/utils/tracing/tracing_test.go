package tracing

import (
	"context"
	"testing"

	"github.com/divkix/Alita_Robot/alita/config"
)

func resetTracingState(t *testing.T) {
	t.Helper()
	origTracerProvider := tracerProvider
	origTracer := tracer
	origPropagator := propagator
	origEnabled := enabled
	t.Cleanup(func() {
		tracerProvider = origTracerProvider
		tracer = origTracer
		propagator = origPropagator
		enabled = origEnabled
	})
	tracerProvider = nil
	tracer = nil
	propagator = nil
	enabled = false
}

func ensureAppConfig(t *testing.T) {
	t.Helper()
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{BotVersion: "2.18.2"}
	}
}

func TestGetPropagator_BeforeInit_CarriesTraceContextAndBaggage(t *testing.T) {
	resetTracingState(t)

	p := GetPropagator()
	if p == nil {
		t.Fatal("GetPropagator() = nil before InitTracing")
	}
	fields := make(map[string]bool)
	for _, f := range p.Fields() {
		fields[f] = true
	}
	for _, want := range []string{"traceparent", "baggage"} {
		if !fields[want] {
			t.Fatalf("GetPropagator().Fields() = %v, want it to carry %q", p.Fields(), want)
		}
	}
}

func TestStartSpan_WhenDisabled_ReturnsSameContext(t *testing.T) {
	resetTracingState(t)

	inputCtx := context.Background()
	ctx, span := StartSpan(inputCtx, "test-span")

	if ctx != inputCtx {
		t.Error("expected StartSpan to return the same context when tracing is disabled")
	}

	if span == nil {
		t.Fatal("expected span to be non-nil")
	}

	if span.SpanContext().IsValid() {
		t.Error("expected no-op span to have an invalid SpanContext")
	}
}

func TestShutdown_WhenProviderNil_ReturnsNil(t *testing.T) {
	resetTracingState(t)

	err := Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected Shutdown to return nil when provider is nil, got %v", err)
	}
}

func TestWorkingModeAttribute_ValueReflectsConfig(t *testing.T) {
	ensureAppConfig(t)

	origApp := config.AppConfig
	defer func() {
		config.AppConfig = origApp
	}()
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}
	origMode := config.AppConfig.WorkingMode
	defer func() {
		if config.AppConfig != nil {
			config.AppConfig.WorkingMode = origMode
		}
	}()

	config.AppConfig.WorkingMode = "webhook"
	attr := WorkingModeAttribute()

	if string(attr.Key) != "bot.working_mode" {
		t.Errorf("expected key 'bot.working_mode', got %q", attr.Key)
	}
	if attr.Value.AsString() != "webhook" {
		t.Errorf("expected value 'webhook', got %q", attr.Value.AsString())
	}
}

func TestInitTracing_NoEndpoint_NoConsoleExporter(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_CONSOLE", "")
	t.Setenv("OTEL_TRACES_SAMPLE_RATE", "")

	ensureAppConfig(t)
	resetTracingState(t)

	err := InitTracing()
	if err != nil {
		t.Fatalf("expected InitTracing to return nil, got %v", err)
	}
	if enabled {
		t.Error("expected enabled to be false when no exporter is configured")
	}
	if propagator == nil {
		t.Error("expected propagator to be set even when tracing is disabled")
	}
	if tracerProvider != nil {
		t.Error("expected tracerProvider to be nil when no exporter is configured")
	}
}

func TestInitTracingSampleRateControlsRootSampling(t *testing.T) {
	tests := []struct {
		rate        string
		wantSampled bool
	}{
		{rate: "", wantSampled: true},        // default 1.0
		{rate: "invalid", wantSampled: true}, // parse failure falls back to 1.0
		{rate: "2.0", wantSampled: true},     // clamped to 1.0
		{rate: "0", wantSampled: false},
		{rate: "-0.5", wantSampled: false}, // clamped to 0.0
	}
	for _, tc := range tests {
		t.Run("rate="+tc.rate, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
			t.Setenv("OTEL_EXPORTER_CONSOLE", "true")
			t.Setenv("OTEL_TRACES_SAMPLE_RATE", tc.rate)

			ensureAppConfig(t)
			resetTracingState(t)

			if err := InitTracing(); err != nil {
				t.Fatalf("InitTracing() error = %v", err)
			}
			t.Cleanup(func() { _ = Shutdown(context.Background()) })

			// Spans are left unended so the console exporter prints nothing.
			_, span := StartSpan(context.Background(), "sample-rate-test")
			if got := span.SpanContext().IsSampled(); got != tc.wantSampled {
				t.Fatalf("root span sampled = %v, want %v", got, tc.wantSampled)
			}
		})
	}
}

func TestInitTracingConsoleExporterEnablesTracingAndShutdown(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_CONSOLE", "true")
	t.Setenv("OTEL_TRACES_SAMPLE_RATE", "2.0")
	t.Setenv("OTEL_SERVICE_NAME", "alita-test")

	ensureAppConfig(t)
	resetTracingState(t)

	if err := InitTracing(); err != nil {
		t.Fatalf("InitTracing() error = %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true with console exporter")
	}
	if tracerProvider == nil {
		t.Fatal("tracerProvider = nil, want initialized provider")
	}
	if tracer == nil {
		t.Fatal("tracer = nil, want initialized tracer")
	}
	if propagator == nil {
		t.Fatal("propagator = nil, want initialized propagator")
	}

	ctx, span := StartSpan(context.Background(), "console-exporter-test")
	if ctx == nil {
		t.Fatal("StartSpan() returned nil context")
	}
	if !span.SpanContext().IsValid() {
		t.Fatal("StartSpan() returned invalid span context while tracing enabled")
	}
	span.End()

	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
