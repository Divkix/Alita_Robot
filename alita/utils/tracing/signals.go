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
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
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
func initOTLPSignals(ctx context.Context, res *resource.Resource, endpoint string, insecure bool, serviceName string) error {
	var err error
	if signalEnabled("OTEL_LOGS_EXPORTER") {
		if loggerProvider, err = newLoggerProvider(ctx, res, endpoint, insecure); err != nil {
			return err
		}
	}
	if signalEnabled("OTEL_METRICS_EXPORTER") {
		if meterProvider, err = newMeterProvider(ctx, res, endpoint, insecure); err != nil {
			return errors.Join(err, shutdownOTLPSignals(ctx))
		}
		log.Info("[Tracing] Exporting Prometheus metrics over OTLP")
	}
	if loggerProvider != nil {
		// Added after logredact's hook, so entries arrive already scrubbed.
		log.AddHook(logHook{otellogrus.NewHook(serviceName, otellogrus.WithLoggerProvider(loggerProvider))})
		log.Info("[Tracing] Exporting logs over OTLP")
	}
	return nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource, endpoint string, insecure bool) (*sdklog.LoggerProvider, error) {
	opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	exporter, err := otlploggrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, endpoint string, insecure bool) (*sdkmetric.MeterProvider, error) {
	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
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
	var errs []error
	if meterProvider != nil {
		if err := meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
		meterProvider = nil
	}
	if loggerProvider != nil {
		if err := loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown logger provider: %w", err))
		}
		loggerProvider = nil
	}
	return errors.Join(errs...)
}

// logHook gives otellogrus a non-nil context: most log calls are made without
// WithContext, and the SDK reads the span from the context it is given.
// Hooks get logrus's per-call copy of the entry, so setting it here is local.
type logHook struct {
	*otellogrus.Hook
}

func (h logHook) Fire(entry *log.Entry) error {
	if entry.Context == nil {
		entry.Context = context.Background()
	}
	return h.Hook.Fire(entry)
}
