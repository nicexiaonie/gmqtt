package otelgmqtt

import (
	"context"
	"fmt"

	"github.com/nicexiaonie/gmqtt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func publishMiddleware(cfg config) gmqtt.PublishMiddleware {
	tracer := cfg.tracerProvider.Tracer(cfg.tracerName)
	return func(next gmqtt.PublishHandler) gmqtt.PublishHandler {
		return func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
			if req == nil || !cfg.filter(req.Topic) {
				return next(ctx, req)
			}

			ctx, span := tracer.Start(ctx, cfg.spanName(SpanKindPublish, req.Topic), trace.WithSpanKind(trace.SpanKindProducer))
			defer span.End()

			attrs := publishAttributes(cfg, req)
			envelopeEnabled := cfg.envelopeMode == EnvelopeModeJSON
			injected := false
			var payloadSizeValue int
			var payloadSizeOK bool

			if envelopeEnabled {
				carrier := propagation.MapCarrier{}
				cfg.propagator.Inject(ctx, carrier)
				// req.Metadata is process-local: it is exposed to other in-process
				// middleware but is NOT sent on the wire. Cross-process trace
				// propagation rides on the JSON envelope payload below, not here.
				if req.Metadata == nil {
					req.Metadata = make(map[string]string)
				}
				for k, v := range carrier {
					req.Metadata[k] = v
				}

				payload, size, err := encodeEnvelope(req.Payload, carrier)
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return nil, err
				}
				req.Payload = payload
				payloadSizeValue = size
				payloadSizeOK = true
				injected = carrier["traceparent"] != ""
			} else if size, ok := payloadSize(req.Payload); ok {
				payloadSizeValue = size
				payloadSizeOK = true
			}

			attrs = append(attrs,
				attribute.Bool("gmqtt.envelope.enabled", envelopeEnabled),
				attribute.Bool("gmqtt.trace.injected", injected),
			)
			if payloadSizeOK {
				attrs = append(attrs, attribute.Int("messaging.message.payload_size_bytes", payloadSizeValue))
			}
			span.SetAttributes(attrs...)

			res, err := next(ctx, req)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return res, err
		}
	}
}

func handlerMiddleware(cfg config) gmqtt.HandlerMiddleware {
	tracer := cfg.tracerProvider.Tracer(cfg.tracerName)
	return func(next gmqtt.HandlerFunc) gmqtt.HandlerFunc {
		return func(ctx context.Context, req *gmqtt.HandlerRequest) (*gmqtt.HandlerResponse, error) {
			if req == nil || req.Message == nil || !cfg.filter(req.Message.Topic) {
				return next(ctx, req)
			}

			spanCtx := ctx
			envelopeDetected := false
			envelopeUnwrapped := false
			traceExtracted := false
			var extractedCtx context.Context

			decoded, decodeErr := (*decodedEnvelope)(nil), error(nil)
			if cfg.envelopeMode == EnvelopeModeJSON {
				decoded, decodeErr = decodeEnvelope(req.Message.Payload)
			}
			if decodeErr == nil && decoded != nil {
				envelopeDetected = true
				carrier := propagation.MapCarrier{}
				if decoded.Traceparent != "" {
					carrier["traceparent"] = decoded.Traceparent
				}
				if decoded.Tracestate != "" {
					carrier["tracestate"] = decoded.Tracestate
				}
				if decoded.Baggage != "" {
					carrier["baggage"] = decoded.Baggage
				}
				if len(carrier) > 0 {
					extractedCtx = cfg.propagator.Extract(ctx, carrier)
					traceExtracted = trace.SpanContextFromContext(extractedCtx).IsValid()
					// In parent-child mode the extracted context becomes the
					// parent. In span-link mode the consumer span stays a new
					// root and only references the producer via a link.
					if traceExtracted && !cfg.spanLinks {
						spanCtx = extractedCtx
					}
					if req.Metadata == nil {
						req.Metadata = make(map[string]string)
					}
					for k, v := range carrier {
						req.Metadata[k] = v
					}
				}
				if cfg.envelopeMode == EnvelopeModeJSON && cfg.unwrapPayload {
					req.Message.Payload = decoded.Payload
					envelopeUnwrapped = true
				}
			}

			startOpts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindConsumer)}
			if cfg.spanLinks && traceExtracted {
				startOpts = append(startOpts, trace.WithLinks(trace.Link{
					SpanContext: trace.SpanContextFromContext(extractedCtx),
				}))
			}
			spanCtx, span := tracer.Start(spanCtx, cfg.spanName(SpanKindProcess, req.Message.Topic), startOpts...)
			defer span.End()

			attrs := handlerAttributes(cfg, req)
			attrs = append(attrs,
				attribute.Bool("gmqtt.envelope.detected", envelopeDetected),
				attribute.Bool("gmqtt.envelope.unwrapped", envelopeUnwrapped),
				attribute.Bool("gmqtt.trace.extracted", traceExtracted),
			)
			span.SetAttributes(attrs...)

			if decodeErr != nil && decodeErr != errNotEnvelope {
				span.AddEvent("gmqtt.envelope_decode_failed", trace.WithAttributes(attribute.String("error", decodeErr.Error())))
			}

			res, err := next(spanCtx, req)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return res, err
		}
	}
}

func publishAttributes(cfg config, req *gmqtt.PublishRequest) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "mqtt"),
		attribute.String("messaging.operation", "publish"),
		attribute.String("messaging.destination.name", req.Topic),
		attribute.Int("messaging.mqtt.qos", int(req.QoS)),
		attribute.Bool("messaging.mqtt.retained", req.Retained),
	}
	if cfg.clientID != "" {
		attrs = append(attrs, attribute.String("messaging.client.id", cfg.clientID))
	}
	return attrs
}

func handlerAttributes(cfg config, req *gmqtt.HandlerRequest) []attribute.KeyValue {
	msg := req.Message
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "mqtt"),
		attribute.String("messaging.operation", "process"),
		attribute.String("messaging.destination.name", msg.Topic),
		attribute.Int("messaging.mqtt.qos", int(msg.QoS)),
		attribute.Bool("messaging.mqtt.retained", msg.Retained),
		attribute.Int("messaging.mqtt.message_id", int(msg.MessageID)),
		attribute.Bool("messaging.mqtt.duplicate", msg.Duplicate),
		attribute.Int("messaging.message.payload_size_bytes", len(msg.Payload)),
	}
	if cfg.clientID != "" {
		attrs = append(attrs, attribute.String("messaging.client.id", cfg.clientID))
	}
	return attrs
}

func payloadSize(payload interface{}) (int, bool) {
	switch v := payload.(type) {
	case nil:
		return 0, true
	case []byte:
		return len(v), true
	case string:
		return len(v), true
	case fmt.Stringer:
		return len(v.String()), true
	default:
		data, err := payloadToBytes(v)
		if err != nil {
			return 0, false
		}
		return len(data), true
	}
}
