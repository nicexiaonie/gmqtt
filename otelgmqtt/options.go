package otelgmqtt

import (
	"github.com/nicexiaonie/gmqtt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const defaultTracerName = "github.com/nicexiaonie/gmqtt/otelgmqtt"

// EnvelopeMode controls whether MQTT payloads are wrapped for trace propagation.
type EnvelopeMode int

const (
	// EnvelopeModeOff creates spans without changing MQTT payloads.
	EnvelopeModeOff EnvelopeMode = iota
	// EnvelopeModeJSON wraps payloads in a JSON envelope that carries trace context.
	EnvelopeModeJSON
)

// SpanKind identifies which gmqtt operation a span represents.
type SpanKind string

const (
	// SpanKindPublish identifies publish spans.
	SpanKindPublish SpanKind = "publish"
	// SpanKindProcess identifies message processing spans.
	SpanKindProcess SpanKind = "process"
)

// SpanNameFormatter formats span names for gmqtt operations.
type SpanNameFormatter func(kind SpanKind, topic string) string

// FilterFunc decides whether tracing middleware should run for a topic.
type FilterFunc func(topic string) bool

// Option configures OpenTelemetry middleware.
type Option func(*config)

type config struct {
	tracerProvider trace.TracerProvider
	propagator     propagation.TextMapPropagator
	tracerName     string
	envelopeMode   EnvelopeMode
	unwrapPayload  bool
	filter         FilterFunc
	spanName       SpanNameFormatter
	clientID       string
	spanLinks      bool
}

func newConfig(opts ...Option) config {
	cfg := config{
		tracerProvider: otel.GetTracerProvider(),
		// propagator stays nil here so we can tell whether WithPropagator was
		// set explicitly; it is resolved below.
		tracerName:    defaultTracerName,
		envelopeMode:  EnvelopeModeOff,
		unwrapPayload: true,
		filter: func(string) bool {
			return true
		},
		spanName: defaultSpanName,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.tracerProvider == nil {
		cfg.tracerProvider = otel.GetTracerProvider()
	}
	if cfg.propagator == nil {
		cfg.propagator = resolvePropagator()
	}
	if cfg.tracerName == "" {
		cfg.tracerName = defaultTracerName
	}
	if cfg.filter == nil {
		cfg.filter = func(string) bool { return true }
	}
	if cfg.spanName == nil {
		cfg.spanName = defaultSpanName
	}
	return cfg
}

// resolvePropagator picks the propagator when WithPropagator was not used.
// It honors a globally configured propagator (the OTel convention) but falls
// back to a built-in TraceContext+Baggage default when no global was set.
//
// otel.GetTextMapPropagator never returns nil: with no global configured it
// returns a non-empty no-op composite, which is why a nil check cannot be used.
// A real, user-set propagator advertises the header names it handles via
// Fields(); the no-op stub advertises none, so an empty Fields() means "no
// global was set" and we use our own default instead.
func resolvePropagator() propagation.TextMapPropagator {
	if global := otel.GetTextMapPropagator(); global != nil && len(global.Fields()) > 0 {
		return global
	}
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func defaultSpanName(kind SpanKind, topic string) string {
	switch kind {
	case SpanKindPublish:
		return "mqtt.publish " + topic
	case SpanKindProcess:
		return "mqtt.process " + topic
	default:
		return "mqtt." + string(kind) + " " + topic
	}
}

// WithTracerProvider sets the tracer provider used by middleware.
func WithTracerProvider(provider trace.TracerProvider) Option {
	return func(cfg *config) {
		cfg.tracerProvider = provider
	}
}

// WithPropagator sets the text map propagator used for envelope propagation.
func WithPropagator(propagator propagation.TextMapPropagator) Option {
	return func(cfg *config) {
		cfg.propagator = propagator
	}
}

// WithTracerName sets the instrumentation tracer name.
func WithTracerName(name string) Option {
	return func(cfg *config) {
		cfg.tracerName = name
	}
}

// WithEnvelopeMode controls whether payload envelopes are used.
func WithEnvelopeMode(mode EnvelopeMode) Option {
	return func(cfg *config) {
		cfg.envelopeMode = mode
	}
}

// WithUnwrapPayload controls whether handler middleware replaces envelope payloads with original payloads.
func WithUnwrapPayload(enabled bool) Option {
	return func(cfg *config) {
		cfg.unwrapPayload = enabled
	}
}

// WithFilter skips tracing and propagation for topics that return false.
func WithFilter(filter FilterFunc) Option {
	return func(cfg *config) {
		cfg.filter = filter
	}
}

// WithSpanNameFormatter sets the span name formatter.
func WithSpanNameFormatter(formatter SpanNameFormatter) Option {
	return func(cfg *config) {
		cfg.spanName = formatter
	}
}

// WithClientID adds the MQTT client id as a span attribute.
func WithClientID(clientID string) Option {
	return func(cfg *config) {
		cfg.clientID = clientID
	}
}

// WithSpanLinks links the consumer span to the producer via a span link instead
// of a parent-child relationship. The consumer span becomes the root of a new
// trace that references the producer's span context.
//
// The default (parent-child) is the most widely supported form across APM
// backends and is the most intuitive for 1:1 immediate consumption. Enable span
// links for fan-out (shared subscriptions) or retained messages, where a single
// producer span would otherwise become the parent of many consumer spans spread
// across time — including consumers that connect long after the producer span
// has ended.
func WithSpanLinks(enabled bool) Option {
	return func(cfg *config) {
		cfg.spanLinks = enabled
	}
}

// PublishMiddleware creates OpenTelemetry tracing middleware for gmqtt publish operations.
func PublishMiddleware(opts ...Option) gmqtt.PublishMiddleware {
	cfg := newConfig(opts...)
	return publishMiddleware(cfg)
}

// HandlerMiddleware creates OpenTelemetry tracing middleware for gmqtt message handlers.
func HandlerMiddleware(opts ...Option) gmqtt.HandlerMiddleware {
	cfg := newConfig(opts...)
	return handlerMiddleware(cfg)
}
