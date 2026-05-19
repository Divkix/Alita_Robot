package tracing

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/divkix/Alita_Robot/alita/config"
)

// resetTracingState saves the current tracing globals, resets them to nil/false,
// and registers a t.Cleanup to restore the original values.
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

// ensureAppConfig initializes config.AppConfig if it is nil.
func ensureAppConfig(t *testing.T) {
	t.Helper()
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{BotVersion: "2.17.24"}
	}
}

func TestIsEnabled_BeforeInit_ReturnsFalse(t *testing.T) {
	if IsEnabled() {
		t.Error("expected IsEnabled() to return false before InitTracing is called")
	}
}

func TestGetTracer_BeforeInit_ReturnsNonNil(t *testing.T) {
	tr := GetTracer()
	if tr == nil {
		t.Fatal("expected GetTracer() to return non-nil tracer")
	}

	// Verify it behaves as a no-op tracer by starting a span
	ctx, span := tr.Start(context.Background(), "test-span")
	if ctx == nil {
		t.Error("no-op tracer.Start returned nil context")
	}
	if span == nil {
		t.Error("no-op tracer.Start returned nil span")
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

	// When config.AppConfig.WorkingMode is unset (empty string in test env),
	// the value should still be a valid string attribute.
	_, ok := attr.Value.AsInterface().(string)
	if !ok {
		t.Errorf("expected attribute value to be a string, got %T", attr.Value.AsInterface())
	}
}

func TestStartSpan_WhenDisabled_ReturnsSameContext(t *testing.T) {
	resetTracingState(t)

	inputCtx := context.Background()
	ctx, span := StartSpan(inputCtx, "test-span")

	// When disabled, StartSpan must return the exact same context, not a new one.
	if ctx != inputCtx {
		t.Error("expected StartSpan to return the same context when tracing is disabled")
	}

	// Span should be a no-op (from trace.SpanFromContext on background context).
	if span == nil {
		t.Fatal("expected span to be non-nil")
	}

	// A no-op span has empty SpanContext.
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
	defer SetOnProcessUpdateCallback(nil) // cleanup

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
	defer SetOnProcessUpdateCallback(nil) // cleanup

	// First call
	runOnProcessUpdateCallback()
	if counter.Load() != 1 {
		t.Errorf("expected counter=1 after first call, got %d", counter.Load())
	}

	// Second call
	runOnProcessUpdateCallback()
	if counter.Load() != 2 {
		t.Errorf("expected counter=2 after second call, got %d", counter.Load())
	}
}

func TestRunOnProcessUpdateCallback_NoCallback_DoesNotPanic(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	SetOnProcessUpdateCallback(nil)

	// Should not panic when no callback is registered
	runOnProcessUpdateCallback()
}

func TestWorkingModeAttribute_ValueReflectsConfig(t *testing.T) {
	ensureAppConfig(t)

	// Save original pointer to restore after test
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
	// Clear all tracing environment variables
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

func TestShutdown_AfterFailedInit_ReturnsNil(t *testing.T) {
	// Simulate the state after a failed InitTracing: tracerProvider is nil.
	ensureAppConfig(t)
	resetTracingState(t)

	// Shutdown should return nil when tracerProvider is nil.
	err := Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected Shutdown to return nil after failed init, got %v", err)
	}
}
