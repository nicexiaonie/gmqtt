package otelgmqtt

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

const envelopeVersion = "v1"

const (
	payloadEncodingString = "string"
	payloadEncodingBase64 = "base64"
)

var (
	errNotEnvelope      = errors.New("otelgmqtt: payload is not a gmqtt envelope")
	errInvalidEnvelope  = errors.New("otelgmqtt: invalid gmqtt envelope")
	errUnsupportedValue = errors.New("otelgmqtt: unsupported payload value")
)

type envelope struct {
	Version     string `json:"_gmqtt_envelope"`
	Traceparent string `json:"traceparent,omitempty"`
	Tracestate  string `json:"tracestate,omitempty"`
	Baggage     string `json:"baggage,omitempty"`
	// ContentType is informational, for humans and non-Go consumers inspecting
	// the wire. Decoding does not depend on it; payload bytes are recovered from
	// PayloadEncoding alone.
	ContentType     string      `json:"content_type,omitempty"`
	PayloadEncoding string      `json:"payload_encoding"`
	Payload         interface{} `json:"payload"`
}

type decodedEnvelope struct {
	Traceparent string
	Tracestate  string
	Baggage     string
	Payload     []byte
}

type rawEnvelope struct {
	Version         string          `json:"_gmqtt_envelope"`
	Traceparent     string          `json:"traceparent,omitempty"`
	Tracestate      string          `json:"tracestate,omitempty"`
	Baggage         string          `json:"baggage,omitempty"`
	ContentType     string          `json:"content_type,omitempty"`
	PayloadEncoding string          `json:"payload_encoding"`
	Payload         json.RawMessage `json:"payload"`
}

func encodeEnvelope(payload interface{}, carrier map[string]string) ([]byte, int, error) {
	payloadBytes, err := payloadToBytes(payload)
	if err != nil {
		return nil, 0, err
	}

	env := envelope{
		Version:     envelopeVersion,
		Traceparent: carrier["traceparent"],
		Tracestate:  carrier["tracestate"],
		Baggage:     carrier["baggage"],
	}

	if utf8.Valid(payloadBytes) {
		if json.Valid(payloadBytes) {
			env.ContentType = "application/json"
		} else {
			env.ContentType = "text/plain; charset=utf-8"
		}
		env.PayloadEncoding = payloadEncodingString
		env.Payload = string(payloadBytes)
	} else {
		env.ContentType = "application/octet-stream"
		env.PayloadEncoding = payloadEncodingBase64
		env.Payload = base64.StdEncoding.EncodeToString(payloadBytes)
	}

	data, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	return data, len(payloadBytes), nil
}

func decodeEnvelope(payload []byte) (*decodedEnvelope, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, errNotEnvelope
	}

	versionRaw, ok := raw["_gmqtt_envelope"]
	if !ok {
		return nil, errNotEnvelope
	}
	var version string
	if err := json.Unmarshal(versionRaw, &version); err != nil || version != envelopeVersion {
		return nil, errNotEnvelope
	}

	var env rawEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidEnvelope, err)
	}
	if len(env.Payload) == 0 || env.PayloadEncoding == "" {
		return nil, errInvalidEnvelope
	}

	decoded := &decodedEnvelope{
		Traceparent: env.Traceparent,
		Tracestate:  env.Tracestate,
		Baggage:     env.Baggage,
	}

	switch env.PayloadEncoding {
	case payloadEncodingString:
		var s string
		if err := json.Unmarshal(env.Payload, &s); err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidEnvelope, err)
		}
		decoded.Payload = []byte(s)
	case payloadEncodingBase64:
		var s string
		if err := json.Unmarshal(env.Payload, &s); err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidEnvelope, err)
		}
		data, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errInvalidEnvelope, err)
		}
		decoded.Payload = data
	default:
		return nil, fmt.Errorf("%w: unsupported payload_encoding %q", errInvalidEnvelope, env.PayloadEncoding)
	}

	return decoded, nil
}

func payloadToBytes(payload interface{}) ([]byte, error) {
	switch v := payload.(type) {
	case nil:
		return []byte("null"), nil
	case []byte:
		return append([]byte(nil), v...), nil
	case string:
		return []byte(v), nil
	case json.RawMessage:
		return append([]byte(nil), v...), nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errUnsupportedValue, err)
		}
		return data, nil
	}
}
