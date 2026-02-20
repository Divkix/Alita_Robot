package tracing

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/config"
)

var (
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
	propagator     propagation.TextMapPropagator
)

// InitTracing initializes the OpenTelemetry tracing provider.
// It configures exporters based on environment variables:
// - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP gRPC endpoint (e.g., localhost:4317)
// - OTEL_EXPORTER_CONSOLE: Enable console exporter (true/false, default: false)
// - OTEL_SERVICE_NAME: Service name (default: alita_robot)
// - OTEL_TRACES_SAMPLE_RATE: Trace sample rate (default: 1.0)
func InitTracing() error {
	ctx := context.Background()

	// Get configuration from environment
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "alita_robot"
	}

	sampleRate := 1.0
	if sampleRateEnv := os.Getenv("OTEL_TRACES_SAMPLE_RATE"); sampleRateEnv != "" {
		if _, err := fmt.Sscanf(sampleRateEnv, "%f", &sampleRate); err != nil {
			log.Warnf("[Tracing] Failed to parse OTEL_TRACES_SAMPLE_RATE: %v, using default 1.0", err)
			sampleRate = 1.0
		}
		// Clamp sampleRate to [0.0, 1.0] to ensure a safe sampling ratio
		if sampleRate < 0.0 {
			log.Warnf("[Tracing] OTEL_TRACES_SAMPLE_RATE %.4f is less than 0.0, clamping to 0.0", sampleRate)
			sampleRate = 0.0
		} else if sampleRate > 1.0 {
			log.Warnf("[Tracing] OTEL_TRACES_SAMPLE_RATE %.4f is greater than 1.0, clamping to 1.0", sampleRate)
			sampleRate = 1.0
		}
	}

	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			attribute.String("bot.version", config.AppConfig.BotVersion),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Determine which exporter to use
	var exporter sdktrace.SpanExporter
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	useConsole := os.Getenv("OTEL_EXPORTER_CONSOLE") == "true"
	otlpInsecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"

	if otlpEndpoint != "" {
		// Use OTLP exporter
		log.Infof("[Tracing] Using OTLP exporter with endpoint: %s", otlpEndpoint)
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(otlpEndpoint),
		}
		if otlpInsecure {
			log.Warn("[Tracing] Using insecure OTLP gRPC connection (no TLS)")
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	} else if useConsole {
		// Use console exporter for debugging
		log.Info("[Tracing] Using console exporter")
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
			stdouttrace.WithWriter(os.Stderr),
		)
		if err != nil {
			return fmt.Errorf("failed to create console exporter: %w", err)
		}
	} else {
		// No exporter configured, tracing disabled but we still set up the propagator
		log.Info("[Tracing] No OTLP endpoint or console exporter configured, tracing is disabled")
		propagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
		otel.SetTextMapPropagator(propagator)
		return nil
	}

	// Create tracer provider with sampling
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate))),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	tracerProvider = tp

	// Set up propagator for trace context propagation
	propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	// Create tracer instance
	tracer = otel.Tracer(serviceName)

	log.Infof("[Tracing] Initialized with service name: %s, sample rate: %.2f", serviceName, sampleRate)

	return nil
}

// Shutdown gracefully shuts down the tracer provider.
func Shutdown(ctx context.Context) error {
	if tracerProvider == nil {
		return nil
	}

	log.Info("[Tracing] Shutting down tracer provider...")
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}

	log.Info("[Tracing] Tracer provider shut down successfully")
	return nil
}

// GetTracer returns the global tracer instance.
// If tracing is not initialized, it returns a no-op tracer.
func GetTracer() trace.Tracer {
	if tracer != nil {
		return tracer
	}
	return otel.Tracer("alita_robot")
}

// GetPropagator returns the global text map propagator for trace context propagation.
func GetPropagator() propagation.TextMapPropagator {
	if propagator == nil {
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}
	return propagator
}

// WorkingModeAttribute returns a span attribute for the current working mode.
// This reads config.AppConfig.WorkingMode at call time, so it reflects the
// actual runtime value (webhook/polling) rather than the default set at init.
func WorkingModeAttribute() attribute.KeyValue {
	return attribute.String("bot.working_mode", config.AppConfig.WorkingMode)
}

// StartSpan starts a new span with the given name and options.
// It uses the global tracer if available.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name, opts...)
}
