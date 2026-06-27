package otelgmqtt

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestEnvelopeRoundTripJSONPayload(t *testing.T) {
	payload := []byte(` {"device":"a","value":42}
`)
	enveloped, _, err := encodeEnvelope(payload, map[string]string{"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"})
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(enveloped, &raw); err != nil {
		t.Fatalf("expected JSON envelope: %v", err)
	}
	var version string
	if err := json.Unmarshal(raw["_gmqtt_envelope"], &version); err != nil || version != envelopeVersion {
		t.Fatalf("unexpected envelope version: %q err=%v", version, err)
	}
	var encoding string
	if err := json.Unmarshal(raw["payload_encoding"], &encoding); err != nil || encoding != payloadEncodingString {
		t.Fatalf("unexpected encoding: %q err=%v", encoding, err)
	}

	decoded, err := decodeEnvelope(enveloped)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Fatalf("unexpected payload: %s", decoded.Payload)
	}
	if decoded.Traceparent == "" {
		t.Fatal("expected traceparent")
	}
}

func TestEnvelopeRoundTripJSONValues(t *testing.T) {
	payloads := [][]byte{
		[]byte(`[1,2,3]`),
		[]byte(`"json string"`),
		[]byte(`42`),
		[]byte(`null`),
	}
	for _, payload := range payloads {
		enveloped, _, err := encodeEnvelope(payload, nil)
		if err != nil {
			t.Fatalf("encode failed for %s: %v", payload, err)
		}
		decoded, err := decodeEnvelope(enveloped)
		if err != nil {
			t.Fatalf("decode failed for %s: %v", payload, err)
		}
		if !bytes.Equal(decoded.Payload, payload) {
			t.Fatalf("unexpected payload for %s: %s", payload, decoded.Payload)
		}
	}
}

func TestEnvelopeRoundTripStringPayload(t *testing.T) {
	payload := []byte("hello mqtt")
	enveloped, _, err := encodeEnvelope(payload, nil)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := decodeEnvelope(enveloped)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Fatalf("unexpected payload: %q", decoded.Payload)
	}
}

func TestEnvelopeRoundTripBinaryPayload(t *testing.T) {
	payload := []byte{0xff, 0x00, 0x01, 0x02}
	enveloped, _, err := encodeEnvelope(payload, nil)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := decodeEnvelope(enveloped)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Fatalf("unexpected payload: %#v", decoded.Payload)
	}
}

func TestDecodePlainJSONIsNotEnvelope(t *testing.T) {
	_, err := decodeEnvelope([]byte(`{"traceparent":"not an envelope","payload":"data"}`))
	if !errors.Is(err, errNotEnvelope) {
		t.Fatalf("expected errNotEnvelope, got %v", err)
	}
}

func TestDecodeMalformedEnvelope(t *testing.T) {
	_, err := decodeEnvelope([]byte(`{"_gmqtt_envelope":"v1","payload_encoding":"base64","payload":"%%%"}`))
	if !errors.Is(err, errInvalidEnvelope) {
		t.Fatalf("expected errInvalidEnvelope, got %v", err)
	}
}
