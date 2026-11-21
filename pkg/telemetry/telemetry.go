// Package telemetry configures OpenTelemetry tracing, metrics and structured
// logging the same way in every ticketbench binary.
//
// Configuration comes from the standard OTel environment variables:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT  collector address, e.g. http://otel-collector:4317
//	OTEL_SDK_DISABLED=true       turn exporting off (spans are still propagated)
//	TB_TRACE_RATIO               sampling ratio for root spans, default 1
//
// Trace context travels between services as a W3C traceparent header in gRPC
// metadata, so a trace started by the CLI continues through the runner and the
// stores down to the database driver spans.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// Options tune Setup.
type Options struct {
	Service string
	Version string
	// Ratio overrides TB_TRACE_RATIO when >= 0.
	Ratio float64
	// Logger output; defaults to stderr.
	LogLevel slog.Level
	LogText  bool
}

// Shutdown flushes and stops the providers.
type Shutdown func(context.Context) error

// Enabled reports whether spans and metrics are exported.
func Enabled() bool {
	if strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true") {
		return false
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// Setup installs global tracer and meter providers, the W3C propagator and a
// slog default logger that adds trace_id and span_id to every record.
func Setup(ctx context.Context, opts Options) (Shutdown, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	setupLogger(opts)

	ratio := opts.Ratio
	if ratio < 0 {
		ratio = 1
		if v, err := strconv.ParseFloat(os.Getenv("TB_TRACE_RATIO"), 64); err == nil {
			ratio = v
		}
	}

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceName(opts.Service),
			semconv.ServiceVersion(opts.Version),
			semconv.ServiceNamespace("ticketbench"),
		),
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	if !Enabled() {
		// Keep a real SDK provider so span contexts (and trace ids) are still
		// generated and propagated, but export nothing.
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		)
		otel.SetTracerProvider(tp)
		return tp.Shutdown, nil
	}

	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithBatcher(traceExp, sdktrace.WithMaxQueueSize(20000), sdktrace.WithBatchTimeout(2*time.Second)),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, errors.Join(err, tp.Shutdown(ctx))
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(10*time.Second))),
	)
	otel.SetMeterProvider(mp)

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}

// Tracer returns the named tracer from the global provider.
func Tracer(name string) trace.Tracer { return otel.Tracer(name) }

// Meter returns the named meter from the global provider.
func Meter(name string) metric.Meter { return otel.Meter(name) }

// TraceID returns the hex trace id of the span in ctx, or "".
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// Quiet returns a context whose span is marked as not sampled. Child spans
// (for example one span per bulk insert batch in the database driver) are
// then not recorded, while the trace id stays the same.
func Quiet(ctx context.Context) context.Context {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ctx
	}
	return trace.ContextWithSpanContext(ctx, sc.WithTraceFlags(sc.TraceFlags().WithSampled(false)))
}

// GRPCServerOptions instruments a gRPC server: incoming traceparent metadata
// becomes the parent of the server span.
func GRPCServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.StatsHandler(otelgrpc.NewServerHandler())}
}

// GRPCDialOptions instruments a gRPC client: the active span is injected as a
// traceparent header into outgoing metadata.
func GRPCDialOptions() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithStatsHandler(otelgrpc.NewClientHandler())}
}

func setupLogger(opts Options) {
	ho := &slog.HandlerOptions{Level: opts.LogLevel}
	var h slog.Handler
	if opts.LogText {
		h = slog.NewTextHandler(os.Stderr, ho)
	} else {
		h = slog.NewJSONHandler(os.Stderr, ho)
	}
	h = h.WithAttrs([]slog.Attr{slog.String("service", opts.Service)})
	slog.SetDefault(slog.New(traceHandler{h}))
}

// traceHandler adds trace_id and span_id of the record's context.
type traceHandler struct{ slog.Handler }

func (t traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return t.Handler.Handle(ctx, r)
}

func (t traceHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return traceHandler{t.Handler.WithAttrs(as)}
}

func (t traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{t.Handler.WithGroup(name)}
}
