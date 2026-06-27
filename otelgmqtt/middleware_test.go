package otelgmqtt

import (
	"context"
	"errors"
	"testing"

	"github.com/nicexiaonie/gmqtt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func newSpanRecorder() (*tracetest.SpanRecorder, *sdktrace.TracerProvider) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	return recorder, provider
}

func TestPublishMiddlewareEnvelopeModeOffDoesNotMutatePayload(t *testing.T) {
	recorder, provider := newSpanRecorder()
	middleware := PublishMiddleware(WithTracerProvider(provider))
	payload := []byte("plain")
	var gotPayload interface{}

	_, err := middleware(func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
		gotPayload = req.Payload
		return &gmqtt.PublishResponse{}, nil
	})(context.Background(), &gmqtt.PublishRequest{Topic: "test/topic", QoS: gmqtt.QoS1, Payload: payload})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(gotPayload.([]byte)) != "plain" {
		t.Fatalf("unexpected payload: %#v", gotPayload)
	}
	if len(recorder.Ended()) != 1 {
		t.Fatalf("expected one span, got %d", len(recorder.Ended()))
	}
}

func TestPublishMiddlewareEnvelopeModeJSONInjectsTraceparent(t *testing.T) {
	_, provider := newSpanRecorder()
	middleware := PublishMiddleware(
		WithTracerProvider(provider),
		WithPropagator(propagation.TraceContext{}),
		WithEnvelopeMode(EnvelopeModeJSON),
	)
	var gotPayload []byte

	_, err := middleware(func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
		gotPayload = req.Payload.([]byte)
		return &gmqtt.PublishResponse{}, nil
	})(context.Background(), &gmqtt.PublishRequest{Topic: "test/topic", QoS: gmqtt.QoS1, Payload: []byte(`{"value":1}`)})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	decoded, err := decodeEnvelope(gotPayload)
	if err != nil {
		t.Fatalf("expected envelope: %v", err)
	}
	if decoded.Traceparent == "" {
		t.Fatal("expected injected traceparent")
	}
}

func TestHandlerMiddlewareExtractsAndUnwrapsEnvelope(t *testing.T) {
	recorder, provider := newSpanRecorder()
	propagator := propagation.TraceContext{}
	tracer := provider.Tracer("test")
	parentCtx, parent := tracer.Start(context.Background(), "parent", trace.WithSpanKind(trace.SpanKindProducer))
	carrier := propagation.MapCarrier{}
	propagator.Inject(parentCtx, carrier)
	parent.End()

	enveloped, _, err := encodeEnvelope([]byte(`{"value":1}`), map[string]string(carrier))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	middleware := HandlerMiddleware(
		WithTracerProvider(provider),
		WithPropagator(propagator),
		WithEnvelopeMode(EnvelopeModeJSON),
		WithUnwrapPayload(true),
	)
	var gotPayload []byte
	var gotCtx context.Context

	_, err = middleware(func(ctx context.Context, req *gmqtt.HandlerRequest) (*gmqtt.HandlerResponse, error) {
		gotCtx = ctx
		gotPayload = req.Message.Payload
		return &gmqtt.HandlerResponse{}, nil
	})(context.Background(), &gmqtt.HandlerRequest{Message: &gmqtt.Message{Topic: "test/topic", Payload: enveloped}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(gotPayload) != `{"value":1}` {
		t.Fatalf("unexpected unwrapped payload: %s", gotPayload)
	}
	if !trace.SpanContextFromContext(gotCtx).IsValid() {
		t.Fatal("expected span-bearing context")
	}

	ended := recorder.Ended()
	if len(ended) != 2 {
		t.Fatalf("expected parent and consumer spans, got %d", len(ended))
	}
	consumer := ended[1]
	if consumer.Parent().TraceID() != ended[0].SpanContext().TraceID() {
		t.Fatalf("expected consumer trace to continue parent trace")
	}
}

func TestHandlerMiddlewareWithoutEnvelopeStillCreatesSpan(t *testing.T) {
	recorder, provider := newSpanRecorder()
	middleware := HandlerMiddleware(WithTracerProvider(provider))

	_, err := middleware(func(ctx context.Context, req *gmqtt.HandlerRequest) (*gmqtt.HandlerResponse, error) {
		if !trace.SpanContextFromContext(ctx).IsValid() {
			t.Fatal("expected span-bearing context")
		}
		if string(req.Message.Payload) != "plain" {
			t.Fatalf("unexpected payload: %s", req.Message.Payload)
		}
		return &gmqtt.HandlerResponse{}, nil
	})(context.Background(), &gmqtt.HandlerRequest{Message: &gmqtt.Message{Topic: "test/topic", Payload: []byte("plain")}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(recorder.Ended()) != 1 {
		t.Fatalf("expected one span, got %d", len(recorder.Ended()))
	}
}

func TestMiddlewareRecordsAndReturnsNextError(t *testing.T) {
	recorder, provider := newSpanRecorder()
	middleware := PublishMiddleware(WithTracerProvider(provider))
	expectedErr := errors.New("publish failed")

	_, err := middleware(func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
		return nil, expectedErr
	})(context.Background(), &gmqtt.PublishRequest{Topic: "test/topic", QoS: gmqtt.QoS1})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected one span, got %d", len(ended))
	}
	if ended[0].Status().Code != codes.Error {
		t.Fatalf("expected error status, got %v", ended[0].Status())
	}
}

func TestFilterFalseSkipsTracingAndEnvelope(t *testing.T) {
	recorder, provider := newSpanRecorder()
	middleware := PublishMiddleware(
		WithTracerProvider(provider),
		WithEnvelopeMode(EnvelopeModeJSON),
		WithFilter(func(string) bool { return false }),
	)
	var gotPayload interface{}

	_, err := middleware(func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
		gotPayload = req.Payload
		return &gmqtt.PublishResponse{}, nil
	})(context.Background(), &gmqtt.PublishRequest{Topic: "test/topic", Payload: []byte("plain")})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(gotPayload.([]byte)) != "plain" {
		t.Fatalf("unexpected payload: %#v", gotPayload)
	}
	if len(recorder.Ended()) != 0 {
		t.Fatalf("expected no spans, got %d", len(recorder.Ended()))
	}
}

func TestPublishToHandlerTraceContinuity(t *testing.T) {
	recorder, provider := newSpanRecorder()
	propagator := propagation.TraceContext{}
	otel.SetTextMapPropagator(propagator)
	publish := PublishMiddleware(WithTracerProvider(provider), WithPropagator(propagator), WithEnvelopeMode(EnvelopeModeJSON))
	handler := HandlerMiddleware(WithTracerProvider(provider), WithPropagator(propagator), WithEnvelopeMode(EnvelopeModeJSON))
	var wirePayload []byte

	_, err := publish(func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
		wirePayload = req.Payload.([]byte)
		return &gmqtt.PublishResponse{}, nil
	})(context.Background(), &gmqtt.PublishRequest{Topic: "test/topic", Payload: []byte(`{"value":1}`)})
	if err != nil {
		t.Fatalf("publish middleware failed: %v", err)
	}

	_, err = handler(func(ctx context.Context, req *gmqtt.HandlerRequest) (*gmqtt.HandlerResponse, error) {
		return &gmqtt.HandlerResponse{}, nil
	})(context.Background(), &gmqtt.HandlerRequest{Message: &gmqtt.Message{Topic: "test/topic", Payload: wirePayload}})
	if err != nil {
		t.Fatalf("handler middleware failed: %v", err)
	}

	ended := recorder.Ended()
	if len(ended) != 2 {
		t.Fatalf("expected publish and process spans, got %d", len(ended))
	}
	producer := ended[0]
	consumer := ended[1]
	if consumer.SpanContext().TraceID() != producer.SpanContext().TraceID() {
		t.Fatalf("expected same trace id")
	}
	if consumer.Parent().SpanID() != producer.SpanContext().SpanID() {
		t.Fatalf("expected producer span as consumer parent")
	}
}

func TestDefaultPropagatorInjectsWithoutGlobalOrOption(t *testing.T) {
	// Simulate a user who never configured a global propagator: the global is
	// an empty no-op composite. The built-in TraceContext+Baggage default must
	// still inject traceparent, otherwise cross-process linkage silently breaks.
	prev := otel.GetTextMapPropagator()
	defer otel.SetTextMapPropagator(prev)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	_, provider := newSpanRecorder()
	cfg := newConfig(WithTracerProvider(provider))

	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "parent")
	defer span.End()

	carrier := propagation.MapCarrier{}
	cfg.propagator.Inject(ctx, carrier)
	if carrier["traceparent"] == "" {
		t.Fatal("expected default propagator to inject traceparent without global or WithPropagator")
	}
}

// markerPropagator injects a distinctive header so a test can prove which
// propagator was actually used.
type markerPropagator struct{}

func (markerPropagator) Inject(_ context.Context, carrier propagation.TextMapCarrier) {
	carrier.Set("x-marker", "set")
}
func (markerPropagator) Extract(ctx context.Context, _ propagation.TextMapCarrier) context.Context {
	return ctx
}
func (markerPropagator) Fields() []string { return []string{"x-marker"} }

func TestUserSetGlobalPropagatorIsHonored(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	defer otel.SetTextMapPropagator(prev)
	otel.SetTextMapPropagator(markerPropagator{})

	cfg := newConfig()

	carrier := propagation.MapCarrier{}
	cfg.propagator.Inject(context.Background(), carrier)
	if carrier["x-marker"] != "set" {
		t.Fatal("expected user-set global propagator to be adopted when WithPropagator is absent")
	}
}

func TestWithPropagatorOverridesGlobal(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	defer otel.SetTextMapPropagator(prev)
	otel.SetTextMapPropagator(markerPropagator{})

	cfg := newConfig(WithPropagator(propagation.TraceContext{}))

	carrier := propagation.MapCarrier{}
	cfg.propagator.Inject(context.Background(), carrier)
	if _, ok := carrier["x-marker"]; ok {
		t.Fatal("expected explicit WithPropagator to take precedence over global")
	}
}

func TestHandlerMiddlewareSpanLinksInsteadOfParent(t *testing.T) {
	recorder, provider := newSpanRecorder()
	propagator := propagation.TraceContext{}
	tracer := provider.Tracer("test")
	parentCtx, parent := tracer.Start(context.Background(), "parent", trace.WithSpanKind(trace.SpanKindProducer))
	producerSC := parent.SpanContext()
	carrier := propagation.MapCarrier{}
	propagator.Inject(parentCtx, carrier)
	parent.End()

	enveloped, _, err := encodeEnvelope([]byte(`{"value":1}`), map[string]string(carrier))
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	middleware := HandlerMiddleware(
		WithTracerProvider(provider),
		WithPropagator(propagator),
		WithEnvelopeMode(EnvelopeModeJSON),
		WithSpanLinks(true),
	)

	_, err = middleware(func(ctx context.Context, req *gmqtt.HandlerRequest) (*gmqtt.HandlerResponse, error) {
		return &gmqtt.HandlerResponse{}, nil
	})(context.Background(), &gmqtt.HandlerRequest{Message: &gmqtt.Message{Topic: "test/topic", Payload: enveloped}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ended := recorder.Ended()
	if len(ended) != 2 {
		t.Fatalf("expected parent and consumer spans, got %d", len(ended))
	}
	consumer := ended[1]
	// In span-link mode the consumer must be a new root, not a child.
	if consumer.Parent().IsValid() {
		t.Fatalf("expected consumer to be a new root, got parent %v", consumer.Parent())
	}
	if consumer.SpanContext().TraceID() == producerSC.TraceID() {
		t.Fatalf("expected consumer in a new trace, got shared trace id")
	}
	links := consumer.Links()
	if len(links) != 1 {
		t.Fatalf("expected one span link, got %d", len(links))
	}
	if links[0].SpanContext.SpanID() != producerSC.SpanID() {
		t.Fatalf("expected link to producer span, got %v", links[0].SpanContext.SpanID())
	}
}
