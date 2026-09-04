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
	enabled        bool
)

func InitTracing() error {
	ctx := context.Background()

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
		if sampleRate < 0.0 {
			log.Warnf("[Tracing] OTEL_TRACES_SAMPLE_RATE %.4f is less than 0.0, clamping to 0.0", sampleRate)
			sampleRate = 0.0
		} else if sampleRate > 1.0 {
			log.Warnf("[Tracing] OTEL_TRACES_SAMPLE_RATE %.4f is greater than 1.0, clamping to 1.0", sampleRate)
			sampleRate = 1.0
		}
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			attribute.String("bot.version", config.AppConfig.BotVersion),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	useConsole := os.Getenv("OTEL_EXPORTER_CONSOLE") == "true"
	otlpInsecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"

	if otlpEndpoint != "" {
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
		log.Info("[Tracing] Using console exporter")
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
			stdouttrace.WithWriter(os.Stderr),
		)
		if err != nil {
			return fmt.Errorf("failed to create console exporter: %w", err)
		}
	} else {
		log.Info("[Tracing] No OTLP endpoint or console exporter configured, tracing is disabled")
		propagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
		otel.SetTextMapPropagator(propagator)
		enabled = false
		return nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate))),
	)

	otel.SetTracerProvider(tp)
	tracerProvider = tp

	propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	tracer = otel.Tracer(serviceName)
	enabled = true

	log.Infof("[Tracing] Initialized with service name: %s, sample rate: %.2f", serviceName, sampleRate)

	return nil
}

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

func GetPropagator() propagation.TextMapPropagator {
	if propagator == nil {
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}
	return propagator
}

func WorkingModeAttribute() attribute.KeyValue {
	return attribute.String("bot.working_mode", config.AppConfig.WorkingMode)
}

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !enabled {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, name, opts...)
}
