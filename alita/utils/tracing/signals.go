package tracing

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/bridges/otellogrus"
	prombridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

var (
	loggerProvider *sdklog.LoggerProvider
	meterProvider  *sdkmetric.MeterProvider
)

// signalEnabled follows the OpenTelemetry convention: OTEL_LOGS_EXPORTER or
// OTEL_METRICS_EXPORTER set to "none" turns that signal off; anything else
// keeps it on whenever an OTLP endpoint is configured.
func signalEnabled(envName string) bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv(envName)), "none")
}

// otlpUsesHTTP reads OTEL_EXPORTER_OTLP_PROTOCOL. gRPC stays the default so
// existing host:4317 setups keep working; http/protobuf is for backends that
// take only OTLP/HTTP, such as Grafana Cloud. The HTTP exporters read the
// endpoint (a base URL, "/v1/<signal>" appended), headers and TLS from the
// standard variables, so the endpoint must be a URL.
func otlpUsesHTTP(endpoint string) (bool, error) {
	switch protocol := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))); protocol {
	case "", "grpc":
		return false, nil
	case "http/protobuf":
		if !strings.Contains(endpoint, "://") {
			return false, errors.New("OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf needs OTEL_EXPORTER_OTLP_ENDPOINT as an http(s):// URL")
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported OTEL_EXPORTER_OTLP_PROTOCOL %q: want grpc or http/protobuf", protocol)
	}
}

// otlpTarget turns OTEL_EXPORTER_OTLP_ENDPOINT into the gRPC host:port and
// whether to skip TLS. It accepts the bare "host:4317" form and the spec's
// "http(s)://host:4317": a scheme decides TLS, so the insecure flag only
// applies to the bare form. A path in the URL is dropped, as gRPC ignores it.
func otlpTarget(endpoint string, insecure bool) (string, bool, error) {
	if !strings.Contains(endpoint, "://") {
		return endpoint, insecure, nil
	}
	const invalid = "invalid OTEL_EXPORTER_OTLP_ENDPOINT: want host:port or http(s)://host:port"
	u, err := url.Parse(endpoint)
	if err != nil {
		// The parse error quotes the raw URL, credentials included.
		return "", false, errors.New(invalid)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false, fmt.Errorf("%s, got %q", invalid, u.Redacted())
	}
	return u.Host, u.Scheme == "http", nil
}

// otlpHeaderEnvs are the variables the OTLP exporters read headers from.
var otlpHeaderEnvs = []string{
	"OTEL_EXPORTER_OTLP_HEADERS",
	"OTEL_EXPORTER_OTLP_TRACES_HEADERS",
	"OTEL_EXPORTER_OTLP_LOGS_HEADERS",
	"OTEL_EXPORTER_OTLP_METRICS_HEADERS",
}

// registerHeaderSecrets keeps OTLP header values (an ingestion key, usually)
// out of the logs, as written and URL-decoded. Headers are "k1=v1,k2=v2".
func registerHeaderSecrets() {
	for _, name := range otlpHeaderEnvs {
		for pair := range strings.SplitSeq(os.Getenv(name), ",") {
			_, value, ok := strings.Cut(pair, "=")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			logredact.RegisterSecret(value)
			if decoded, err := url.QueryUnescape(value); err == nil && decoded != value {
				logredact.RegisterSecret(decoded)
			}
		}
	}
}

// initOTLPSignals exports logs and the Prometheus registry over the same OTLP
// endpoint as traces, so a single collector receives all three. On error
// nothing is left running.
func initOTLPSignals(ctx context.Context, res *resource.Resource, endpoint string, insecure, useHTTP bool, serviceName string) error {
	var err error
	if signalEnabled("OTEL_LOGS_EXPORTER") {
		if loggerProvider, err = newLoggerProvider(ctx, res, endpoint, insecure, useHTTP); err != nil {
			return err
		}
	}
	if signalEnabled("OTEL_METRICS_EXPORTER") {
		if meterProvider, err = newMeterProvider(ctx, res, endpoint, insecure, useHTTP); err != nil {
			return errors.Join(err, shutdownOTLPSignals(ctx))
		}
		log.Info("[Tracing] Exporting Prometheus metrics over OTLP")
	}
	if loggerProvider != nil {
		// Added after logredact's hook, so entries arrive already scrubbed.
		log.AddHook(logHook{otellogrus.NewHook(serviceName, otellogrus.WithLoggerProvider(warnTextProvider{loggerProvider}))})
		log.Info("[Tracing] Exporting logs over OTLP")
	}
	return nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource, endpoint string, insecure, useHTTP bool) (*sdklog.LoggerProvider, error) {
	var exporter sdklog.Exporter
	var err error
	if useHTTP {
		exporter, err = otlploghttp.New(ctx)
	} else {
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		exporter, err = otlploggrpc.New(ctx, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, endpoint string, insecure, useHTTP bool) (*sdkmetric.MeterProvider, error) {
	var exporter sdkmetric.Exporter
	var err error
	if useHTTP {
		exporter, err = otlpmetrichttp.New(ctx)
	} else {
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		exporter, err = otlpmetricgrpc.New(ctx, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}
	// The interval comes from OTEL_METRIC_EXPORT_INTERVAL (default 60 s).
	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithProducer(prombridge.NewMetricProducer()))
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	), nil
}

func shutdownOTLPSignals(ctx context.Context) error {
	providersMu.Lock()
	mp, lp := meterProvider, loggerProvider
	meterProvider, loggerProvider = nil, nil
	providersMu.Unlock()

	var errs []error
	if mp != nil {
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
	}
	if lp != nil {
		if err := lp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown logger provider: %w", err))
		}
	}
	return errors.Join(errs...)
}

// sdkErrorField marks the SDK's own errors, which stay out of the OTLP log
// export: a failing export would otherwise queue a log record per failure.
const sdkErrorField = "otel_sdk_error"

// handleSDKError logs an OpenTelemetry SDK error (a failed export, usually)
// to stdout only. The SDK's default handler writes to the standard log
// package, which main sends through logrus and so back into the export.
func handleSDKError(err error) {
	log.WithField(sdkErrorField, true).Warnf("[Tracing] OpenTelemetry SDK error: %v", err)
}

// logHook gives otellogrus a non-nil context: most log calls are made without
// WithContext, and the SDK reads the span from the context it is given.
// Hooks get logrus's per-call copy of the entry, so setting it here is local.
type logHook struct {
	*otellogrus.Hook
}

func (h logHook) Fire(entry *log.Entry) error {
	if _, internal := entry.Data[sdkErrorField]; internal {
		return nil
	}
	if entry.Context == nil {
		entry.Context = context.Background()
	}
	return h.Hook.Fire(entry)
}

// warnTextProvider exports logrus's "warning" severity text as "warn", matching
// the short names of the other levels ("info", "error").
type warnTextProvider struct {
	otellog.LoggerProvider
}

func (p warnTextProvider) Logger(name string, opts ...otellog.LoggerOption) otellog.Logger {
	return warnTextLogger{p.LoggerProvider.Logger(name, opts...)}
}

type warnTextLogger struct {
	otellog.Logger
}

func (l warnTextLogger) Emit(ctx context.Context, record otellog.Record) {
	if record.SeverityText() == log.WarnLevel.String() {
		record.SetSeverityText("warn")
	}
	l.Logger.Emit(ctx, record)
}
