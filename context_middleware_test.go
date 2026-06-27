package gmqtt

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestWaitTokenContextCanceled(t *testing.T) {
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitTokenContext(ctx, &mockToken{done: done})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestWaitTokenContextDeadlineExceeded(t *testing.T) {
	done := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	err := waitTokenContext(ctx, &mockToken{done: done})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestPublishContextMiddlewareOrderAndMutation(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)
	calls := make([]string, 0)

	client.publishHandler = chainPublishMiddlewares(client.publishFinal,
		func(next PublishHandler) PublishHandler {
			return func(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
				calls = append(calls, "a-before")
				req.Topic = "mutated/topic"
				res, err := next(ctx, req)
				calls = append(calls, "a-after")
				return res, err
			}
		},
		func(next PublishHandler) PublishHandler {
			return func(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
				calls = append(calls, "b-before")
				req.Payload = "mutated-payload"
				res, err := next(ctx, req)
				calls = append(calls, "b-after")
				return res, err
			}
		},
	)

	if err := client.PublishContext(context.Background(), "test/topic", QoS1, true, "payload"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedCalls := []string{"a-before", "b-before", "b-after", "a-after"}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
	if mqttClient.publishedTopic != "mutated/topic" {
		t.Fatalf("unexpected topic: %s", mqttClient.publishedTopic)
	}
	if mqttClient.publishedPayload != "mutated-payload" {
		t.Fatalf("unexpected payload: %#v", mqttClient.publishedPayload)
	}
	if mqttClient.publishedQoS != byte(QoS1) || !mqttClient.publishedRetained {
		t.Fatalf("unexpected publish options")
	}
}

func TestPublishContextMiddlewareShortCircuit(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)
	expectedErr := errors.New("blocked")

	client.publishHandler = chainPublishMiddlewares(client.publishFinal,
		func(next PublishHandler) PublishHandler {
			return func(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
				return nil, expectedErr
			}
		},
	)

	err := client.PublishContext(context.Background(), "test/topic", QoS1, false, "payload")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected blocked error, got %v", err)
	}
	if mqttClient.publishedTopic != "" {
		t.Fatalf("expected final publish not to run")
	}
}

func TestPublishWithTimeoutKeepsLegacyError(t *testing.T) {
	done := make(chan struct{})
	mqttClient := &mockMQTTClient{connected: true, publishToken: &mockToken{done: done}}
	client := newTestClient(mqttClient)

	err := client.PublishWithTimeout("test/topic", QoS1, false, "payload", time.Nanosecond)
	if !errors.Is(err, ErrPublishTimeout) {
		t.Fatalf("expected ErrPublishTimeout, got %v", err)
	}
}

func TestSubscribeContextCanceledCleansLocalSubscription(t *testing.T) {
	done := make(chan struct{})
	mqttClient := &mockMQTTClient{connected: true, subscribeToken: &mockToken{done: done}}
	client := newTestClient(mqttClient)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.SubscribeContext(ctx, "test/topic", QoS1, func(context.Context, string, []byte) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(client.subscriptions) != 0 {
		t.Fatalf("expected local subscription cleanup, got %d", len(client.subscriptions))
	}
}

func TestUnsubscribeContextCanceledPreservesLocalSubscription(t *testing.T) {
	done := make(chan struct{})
	mqttClient := &mockMQTTClient{connected: true, unsubscribeToken: &mockToken{done: done}}
	client := newTestClient(mqttClient)
	client.subLock.Lock()
	client.addSubscription("test/topic", QoS1, func(string, []byte) error { return nil })
	client.subLock.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.UnsubscribeContext(ctx, "test/topic")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(client.subscriptions) != 1 {
		t.Fatalf("expected local subscription preserved, got %d", len(client.subscriptions))
	}
}

func TestHandlerMiddlewareOrderAndMutation(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)
	calls := make([]string, 0)
	var gotPayload string

	client.handlerChain = chainHandlerMiddlewares(client.handleFinal,
		func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, req *HandlerRequest) (*HandlerResponse, error) {
				calls = append(calls, "a-before")
				res, err := next(ctx, req)
				calls = append(calls, "a-after")
				return res, err
			}
		},
		func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, req *HandlerRequest) (*HandlerResponse, error) {
				calls = append(calls, "b-before")
				req.Message.Payload = []byte("mutated")
				res, err := next(ctx, req)
				calls = append(calls, "b-after")
				return res, err
			}
		},
	)
	client.subLock.Lock()
	client.addSubscription("test/topic", QoS1, func(topic string, payload []byte) error {
		gotPayload = string(payload)
		return nil
	})
	client.subLock.Unlock()

	_, err := client.handlerChain(context.Background(), &HandlerRequest{
		Message: &Message{Topic: "test/topic", Payload: []byte("original")},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedCalls := []string{"a-before", "b-before", "b-after", "a-after"}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
	if gotPayload != "mutated" {
		t.Fatalf("unexpected payload: %s", gotPayload)
	}
}

func TestHandlerMiddlewareShortCircuit(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)
	expectedErr := errors.New("blocked")
	called := false

	client.handlerChain = chainHandlerMiddlewares(client.handleFinal,
		func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, req *HandlerRequest) (*HandlerResponse, error) {
				return nil, expectedErr
			}
		},
	)
	client.subLock.Lock()
	client.addSubscription("test/topic", QoS1, func(topic string, payload []byte) error {
		called = true
		return nil
	})
	client.subLock.Unlock()

	_, err := client.handlerChain(context.Background(), &HandlerRequest{Message: &Message{Topic: "test/topic"}})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected blocked error, got %v", err)
	}
	if called {
		t.Fatalf("expected middleware short circuit to skip user handler")
	}
}
