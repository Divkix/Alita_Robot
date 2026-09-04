package tracing

import (
	"context"
	"sync/atomic"
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

func TestGetPropagator_BeforeInit_ReturnsNonNil(t *testing.T) {
	p := GetPropagator()
	if p == nil {
		t.Fatal("expected GetPropagator() to return non-nil propagator")
	}
}

func TestWorkingModeAttribute_ReturnsCorrectKeyAndValue(t *testing.T) {
	attr := WorkingModeAttribute()

	if string(attr.Key) != "bot.working_mode" {
		t.Errorf("expected key 'bot.working_mode', got %q", attr.Key)
	}

	_, ok := attr.Value.AsInterface().(string)
	if !ok {
		t.Errorf("expected attribute value to be a string, got %T", attr.Value.AsInterface())
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

func TestSetOnProcessUpdateCallback_StoresCallback(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	var called atomic.Int32
	SetOnProcessUpdateCallback(func() {
		called.Add(1)
	})
	defer SetOnProcessUpdateCallback(nil)

	runOnProcessUpdateCallback()
	if called.Load() != 1 {
		t.Errorf("expected callback to be called once, got %d", called.Load())
	}
}

func TestRunOnProcessUpdateCallback_CallsStoredCallback(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	var counter atomic.Int32
	SetOnProcessUpdateCallback(func() {
		counter.Add(1)
	})
	defer SetOnProcessUpdateCallback(nil)

	runOnProcessUpdateCallback()
	if counter.Load() != 1 {
		t.Errorf("expected counter=1 after first call, got %d", counter.Load())
	}

	runOnProcessUpdateCallback()
	if counter.Load() != 2 {
		t.Errorf("expected counter=2 after second call, got %d", counter.Load())
	}
}

func TestRunOnProcessUpdateCallback_NoCallback_DoesNotPanic(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	SetOnProcessUpdateCallback(nil)

	runOnProcessUpdateCallback()
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

func TestInitTracing_InvalidSampleRate(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_CONSOLE", "")
	t.Setenv("OTEL_TRACES_SAMPLE_RATE", "invalid")

	ensureAppConfig(t)
	resetTracingState(t)

	err := InitTracing()
	if err != nil {
		t.Fatalf("expected InitTracing to return nil when sample rate is invalid, got %v", err)
	}
	if enabled {
		t.Error("expected enabled to be false when no exporter is configured")
	}
}

func TestInitTracing_NegativeSampleRate(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_CONSOLE", "")
	t.Setenv("OTEL_TRACES_SAMPLE_RATE", "-0.5")

	ensureAppConfig(t)
	resetTracingState(t)

	err := InitTracing()
	if err != nil {
		t.Fatalf("expected InitTracing to return nil, got %v", err)
	}
	if enabled {
		t.Error("expected enabled to be false when no exporter is configured")
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

func TestShutdown_AfterFailedInit_ReturnsNil(t *testing.T) {
	ensureAppConfig(t)
	resetTracingState(t)

	err := Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected Shutdown to return nil after failed init, got %v", err)
	}
}
