package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceIDAndQuiet(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	shutdown, err := Setup(context.Background(), Options{Service: "test", Ratio: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(context.Background())

	ctx, span := Tracer("test").Start(context.Background(), "root")
	defer span.End()
	id := TraceID(ctx)
	if len(id) != 32 {
		t.Fatalf("trace id %q", id)
	}
	if !span.SpanContext().IsSampled() {
		t.Fatal("root span should be sampled with ratio 1")
	}
	q := Quiet(ctx)
	if TraceID(q) != id {
		t.Fatal("Quiet changed the trace id")
	}
	_, child := Tracer("test").Start(q, "child")
	defer child.End()
	if child.SpanContext().IsSampled() {
		t.Fatal("child of a quiet context must not be sampled")
	}
	if child.SpanContext().TraceID().String() != id {
		t.Fatal("child lost the trace id")
	}
}

func TestLoggerAddsTraceID(t *testing.T) {
	var buf bytes.Buffer
	h := traceHandler{slog.NewJSONHandler(&buf, nil)}
	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("0102030405060708")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid}))
	slog.New(h).InfoContext(ctx, "hello")
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["trace_id"] != tid.String() || rec["span_id"] != sid.String() {
		t.Fatalf("record %v lacks trace fields", rec)
	}
}
