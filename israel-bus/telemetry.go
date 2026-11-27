package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// otelEnabled is true when any OTLP endpoint is configured; without one the
// service runs exactly as before (no exporters, no background goroutines).
func otelEnabled() bool {
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "OTEL_EXPORTER_OTLP_") && strings.Contains(kv, "ENDPOINT=") {
			return true
		}
	}
	return false
}

// setupTelemetry wires traces, metrics and logs to OTLP/HTTP from the standard
// OTEL_* env vars (endpoints, service name, resource attributes, sampler) and
// returns a shutdown that flushes all three. See the otel-best-practices skill.
func setupTelemetry(ctx context.Context) (func(context.Context) error, error) {
	if !otelEnabled() {
		return func(context.Context) error { return nil }, nil
	}
	res, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK(), resource.WithProcess())
	if err != nil {
		return nil, err
	}
	// Exporters fail open: a down sink drops data, it never blocks a request.
	te, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(te), sdktrace.WithResource(res))
	me, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)), sdkmetric.WithResource(res))
	le, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(le)), sdklog.WithResource(res))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	global.SetLoggerProvider(lp)

	// log.Printf keeps writing to stderr and is also exported as an OTLP log record.
	log.SetOutput(teeLog{os.Stderr, slog.New(otelslog.NewHandler("israel-bus", otelslog.WithLoggerProvider(lp)))})

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}

type teeLog struct {
	w interface{ Write([]byte) (int, error) }
	l *slog.Logger
}

func (t teeLog) Write(p []byte) (int, error) {
	t.l.Info(strings.TrimRight(string(p), "\n"))
	return t.w.Write(p)
}
