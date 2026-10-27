package tracing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/config"
)

var (
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
	propagator     propagation.TextMapPropagator
	enabled        bool

	// providersMu guards Shutdown's clearing of the providers against a
	// concurrent Flush from a log.Fatal on another goroutine.
	providersMu      sync.Mutex
	registerLogHooks sync.Once
)

// exitFlushTimeout bounds the flush before the process exits: enough for one
// export round trip, short enough not to hold up a restart.
const exitFlushTimeout = 5 * time.Second

const defaultServiceName = "alita_robot"

// init starts telemetry as soon as config has loaded and before alita/db's
// init (which imports this package) connects to Postgres, so the connection
// and migration logs are exported, a fatal failure there included.
func init() {
	if !isBotProcess() {
		return
	}
	if err := InitTracing(); err != nil {
		log.Warnf("[Tracing] Failed to initialize: %v - continuing without telemetry", err)
	}
}

// isBotProcess reports whether this process runs the bot. Config's init
// leaves BotToken empty in --version and --health mode and without a
// configuration; test binaries start telemetry themselves.
func isBotProcess() bool {
	if strings.HasSuffix(strings.TrimSuffix(os.Args[0], ".exe"), ".test") {
		return false
	}
	return config.AppConfig != nil && config.AppConfig.BotToken != ""
}

func InitTracing() error {
	ctx := context.Background()

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

	res, err := newResource(ctx, config.AppConfig.BotVersion)
	if err != nil {
		// resource.New still returns what it detected, e.g. after a malformed
		// OTEL_RESOURCE_ATTRIBUTES entry; a typo there shouldn't turn tracing off.
		log.Warnf("[Tracing] Resource is incomplete: %v", err)
	}
	// Take the name from the resource, so the tracer, the log hook and every
	// signal agree on it whichever variable set it.
	serviceNameValue, _ := res.Set().Value(semconv.ServiceNameKey)
	serviceName := serviceNameValue.AsString()

	var exporter sdktrace.SpanExporter
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	useConsole := os.Getenv("OTEL_EXPORTER_CONSOLE") == "true"
	otlpInsecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"
	var useHTTP bool

	if otlpEndpoint != "" {
		if useHTTP, err = otlpUsesHTTP(otlpEndpoint); err != nil {
			return err
		}
		var target string
		if target, otlpInsecure, err = otlpTarget(otlpEndpoint, otlpInsecure); err != nil {
			return err
		}
		if useHTTP {
			log.Infof("[Tracing] Using OTLP/HTTP exporter with endpoint: %s", target)
			var traceURL string
			if traceURL, err = otlpSignalURL(otlpEndpoint, "traces"); err != nil {
				return err
			}
			// Headers come from the standard variables.
			exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(traceURL))
		} else {
			otlpEndpoint = target
			log.Infof("[Tracing] Using OTLP exporter with endpoint: %s", otlpEndpoint)
			opts := []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint(otlpEndpoint),
			}
			if otlpInsecure {
				log.Warn("[Tracing] Using insecure OTLP gRPC connection (no TLS)")
				opts = append(opts, otlptracegrpc.WithInsecure())
			}
			exporter, err = otlptracegrpc.New(ctx, opts...)
		}
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

	if otlpEndpoint != "" {
		otel.SetErrorHandler(otel.ErrorHandlerFunc(handleSDKError))
		registerHeaderSecrets()
		if err := initOTLPSignals(ctx, res, otlpEndpoint, otlpInsecure, useHTTP, serviceName); err != nil {
			return errors.Join(err, exporter.Shutdown(ctx))
		}
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
	registerLogHooks.Do(func() {
		log.RegisterExitHandler(Flush)
		log.AddHook(traceFieldsHook{})
	})

	log.Infof("[Tracing] Initialized with service name: %s, sample rate: %.2f", serviceName, sampleRate)

	return nil
}

func Shutdown(ctx context.Context) error {
	providersMu.Lock()
	tp := tracerProvider
	tracerProvider = nil
	providersMu.Unlock()
	if tp == nil {
		return shutdownOTLPSignals(ctx)
	}

	log.Info("[Tracing] Shutting down tracer provider...")
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := tp.Shutdown(shutdownCtx); err != nil {
		return errors.Join(fmt.Errorf("failed to shutdown tracer provider: %w", err), shutdownOTLPSignals(shutdownCtx))
	}

	log.Info("[Tracing] Tracer provider shut down successfully")
	return shutdownOTLPSignals(shutdownCtx)
}

// Flush exports the spans, metrics and logs still buffered, giving up after
// exitFlushTimeout. logrus runs it before every log.Fatal* exit; exit with
// log.StandardLogger().Exit rather than os.Exit so a dying process keeps its last telemetry.
// It does nothing once Shutdown has run.
func Flush() {
	ctx, cancel := context.WithTimeout(context.Background(), exitFlushTimeout)
	defer cancel()
	var flushes []func(context.Context) error
	providersMu.Lock()
	if tracerProvider != nil {
		flushes = append(flushes, tracerProvider.ForceFlush)
	}
	if meterProvider != nil {
		flushes = append(flushes, meterProvider.ForceFlush)
	}
	if loggerProvider != nil {
		flushes = append(flushes, loggerProvider.ForceFlush)
	}
	providersMu.Unlock()
	// In parallel, so a signal whose export hangs does not use up the time
	// of the others (the fatal log entry is the one most wanted).
	var wg sync.WaitGroup
	for _, flush := range flushes {
		wg.Go(func() { _ = flush(ctx) })
	}
	wg.Wait()
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

// newResource describes this process to every signal. OTEL_SERVICE_NAME and
// OTEL_RESOURCE_ATTRIBUTES are applied last, so they can set
// deployment.environment.name or override any default or detected value.
// Process attributes are left out: the command line can carry secrets. Host and
// container detectors are left out too: each deploy gets a fresh container ID
// and hostname, which would start a new set of metric series per deploy.
// OTEL_RESOURCE_ATTRIBUTES can set a stable host.name where one is wanted.
func newResource(ctx context.Context, version string) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(defaultServiceName), semconv.ServiceVersion(version)),
		resource.WithFromEnv(),
	)
}

func WorkingModeAttribute() attribute.KeyValue {
	return attribute.String("bot.working_mode", config.AppConfig.WorkingMode)
}

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !enabled || tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, name, opts...)
}

// StartDBSpan starts a database span under a span ctx already records, as
// StartClientSpan does. Attributes are built only then, so the common path
// stays allocation-free.
func StartDBSpan(ctx context.Context, name string, model any) (context.Context, trace.Span) {
	if !trace.SpanFromContext(ctx).IsRecording() {
		return ctx, noopSpan
	}

	attrs := make([]attribute.KeyValue, 0, 2)
	if model != nil {
		attrs = append(attrs, attribute.String("db.model", fmt.Sprintf("%T", model)))
	}
	attrs = append(attrs, WorkingModeAttribute())
	return StartSpan(ctx, name, trace.WithAttributes(attrs...))
}
