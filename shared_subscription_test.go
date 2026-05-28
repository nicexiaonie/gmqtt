package gmqtt

import (
	"errors"
	"reflect"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type mockToken struct {
	err error
}

func (t *mockToken) Wait() bool {
	return true
}

func (t *mockToken) WaitTimeout(time.Duration) bool {
	return true
}

func (t *mockToken) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func (t *mockToken) Error() error {
	return t.err
}

type mockMQTTClient struct {
	connected          bool
	subscribedTopic    string
	subscribedQoS      byte
	subscribedMultiple map[string]byte
	unsubscribedTopics []string
	subscribeErr       error
	unsubscribeErr     error
}

func (c *mockMQTTClient) IsConnected() bool {
	return c.connected
}

func (c *mockMQTTClient) IsConnectionOpen() bool {
	return c.connected
}

func (c *mockMQTTClient) Connect() mqtt.Token {
	return &mockToken{}
}

func (c *mockMQTTClient) Disconnect(uint) {}

func (c *mockMQTTClient) Publish(string, byte, bool, interface{}) mqtt.Token {
	return &mockToken{}
}

func (c *mockMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	c.subscribedTopic = topic
	c.subscribedQoS = qos
	return &mockToken{err: c.subscribeErr}
}

func (c *mockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	c.subscribedMultiple = filters
	return &mockToken{err: c.subscribeErr}
}

func (c *mockMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	c.unsubscribedTopics = topics
	return &mockToken{err: c.unsubscribeErr}
}

func (c *mockMQTTClient) AddRoute(string, mqtt.MessageHandler) {}

func (c *mockMQTTClient) OptionsReader() mqtt.ClientOptionsReader {
	return mqtt.ClientOptionsReader{}
}

func newTestClient(mqttClient mqtt.Client) *Client {
	return &Client{
		options:       &ClientOptions{ClientID: "test"},
		client:        mqttClient,
		subscriptions: make(map[string]*Subscription),
		subOrder:      make([]string, 0),
	}
}

func TestBuildSharedSubscriptionTopic(t *testing.T) {
	topic, err := buildSharedSubscriptionTopic("workers", "sensors/+/data")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if topic != "$share/workers/sensors/+/data" {
		t.Fatalf("unexpected shared topic: %s", topic)
	}
}

func TestBuildSharedSubscriptionTopicValidation(t *testing.T) {
	tests := []struct {
		name   string
		group  string
		filter string
	}{
		{name: "empty group", group: "", filter: "test/topic"},
		{name: "empty filter", group: "workers", filter: ""},
		{name: "group contains slash", group: "bad/group", filter: "test/topic"},
		{name: "group contains plus", group: "bad+group", filter: "test/topic"},
		{name: "group contains hash", group: "bad#group", filter: "test/topic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildSharedSubscriptionTopic(tt.group, tt.filter)
			if !errors.Is(err, ErrInvalidSharedSubscription) {
				t.Fatalf("expected ErrInvalidSharedSubscription, got %v", err)
			}
		})
	}
}

func TestTopicFilterMatches(t *testing.T) {
	tests := []struct {
		filter string
		topic  string
		match  bool
	}{
		{filter: "devices/a/status", topic: "devices/a/status", match: true},
		{filter: "devices/a/status", topic: "devices/b/status", match: false},
		{filter: "devices/+/status", topic: "devices/a/status", match: true},
		{filter: "devices/+/status", topic: "devices/a/config", match: false},
		{filter: "devices/#", topic: "devices/a/status", match: true},
		{filter: "devices/#", topic: "devices", match: true},
		{filter: "devices/+/status", topic: "devices/a/b/status", match: false},
	}

	for _, tt := range tests {
		t.Run(tt.filter+" matches "+tt.topic, func(t *testing.T) {
			if got := topicFilterMatches(tt.filter, tt.topic); got != tt.match {
				t.Fatalf("expected %v, got %v", tt.match, got)
			}
		})
	}
}

func TestLogicalSubscriptionFilter(t *testing.T) {
	filter := logicalSubscriptionFilter("$share/workers/sensors/+/data")
	if filter != "sensors/+/data" {
		t.Fatalf("unexpected logical filter: %s", filter)
	}

	filter = logicalSubscriptionFilter("sensors/+/data")
	if filter != "sensors/+/data" {
		t.Fatalf("unexpected normal filter: %s", filter)
	}
}

func TestFindHandlerPrefersExactMatch(t *testing.T) {
	client := newTestClient(&mockMQTTClient{connected: true})
	exactCalled := false
	wildcardCalled := false

	client.subLock.Lock()
	client.addSubscription("devices/+/status", QoS1, func(string, []byte) error {
		wildcardCalled = true
		return nil
	})
	client.addSubscription("devices/a/status", QoS1, func(string, []byte) error {
		exactCalled = true
		return nil
	})
	client.subLock.Unlock()

	handler := client.findHandler("devices/a/status")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if err := handler("devices/a/status", nil); err != nil {
		t.Fatal(err)
	}
	if !exactCalled || wildcardCalled {
		t.Fatalf("expected exact handler only, exact=%v wildcard=%v", exactCalled, wildcardCalled)
	}
}

func TestFindHandlerUsesRegistrationOrder(t *testing.T) {
	client := newTestClient(&mockMQTTClient{connected: true})
	called := ""

	client.subLock.Lock()
	client.addSubscription("devices/#", QoS1, func(string, []byte) error {
		called = "first"
		return nil
	})
	client.addSubscription("devices/+/status", QoS1, func(string, []byte) error {
		called = "second"
		return nil
	})
	client.subLock.Unlock()

	handler := client.findHandler("devices/a/status")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if err := handler("devices/a/status", nil); err != nil {
		t.Fatal(err)
	}
	if called != "first" {
		t.Fatalf("expected first registered handler, got %s", called)
	}
}

func TestFindHandlerMatchesSharedWildcard(t *testing.T) {
	client := newTestClient(&mockMQTTClient{connected: true})
	called := false

	client.subLock.Lock()
	client.addSubscription("$share/workers/sensors/+/data", QoS1, func(string, []byte) error {
		called = true
		return nil
	})
	client.subLock.Unlock()

	handler := client.findHandler("sensors/a/data")
	if handler == nil {
		t.Fatal("expected handler")
	}
	if err := handler("sensors/a/data", nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected shared wildcard handler")
	}
}

func TestSubscribeSharedStoresBrokerFilter(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)

	err := client.SubscribeShared("workers", "sensors/+/data", QoS1, func(string, []byte) error { return nil })
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mqttClient.subscribedTopic != "$share/workers/sensors/+/data" {
		t.Fatalf("unexpected subscribed topic: %s", mqttClient.subscribedTopic)
	}
	if mqttClient.subscribedQoS != byte(QoS1) {
		t.Fatalf("unexpected qos: %d", mqttClient.subscribedQoS)
	}
	if _, ok := client.subscriptions["$share/workers/sensors/+/data"]; !ok {
		t.Fatal("expected local subscription")
	}
}

func TestSubscribeSharedMultipleStoresBrokerFilters(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)

	err := client.SubscribeSharedMultiple("workers", map[string]QoS{
		"sensors/+/data": QoS1,
		"alerts/#":       QoS0,
	}, func(string, []byte) error { return nil })
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := map[string]byte{
		"$share/workers/sensors/+/data": byte(QoS1),
		"$share/workers/alerts/#":       byte(QoS0),
	}
	if !reflect.DeepEqual(mqttClient.subscribedMultiple, expected) {
		t.Fatalf("unexpected subscriptions: %#v", mqttClient.subscribedMultiple)
	}
	if len(client.subscriptions) != 2 {
		t.Fatalf("expected 2 local subscriptions, got %d", len(client.subscriptions))
	}
}

func TestSubscribeSharedMultipleCleansUpOnFailure(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true, subscribeErr: errors.New("failed")}
	client := newTestClient(mqttClient)

	err := client.SubscribeSharedMultiple("workers", map[string]QoS{
		"sensors/+/data": QoS1,
		"alerts/#":       QoS0,
	}, func(string, []byte) error { return nil })
	if err == nil {
		t.Fatal("expected error")
	}
	if len(client.subscriptions) != 0 {
		t.Fatalf("expected cleanup, got %d subscriptions", len(client.subscriptions))
	}
	if len(client.subOrder) != 0 {
		t.Fatalf("expected order cleanup, got %d entries", len(client.subOrder))
	}
}

func TestUnsubscribeSharedUsesBrokerFilters(t *testing.T) {
	mqttClient := &mockMQTTClient{connected: true}
	client := newTestClient(mqttClient)
	client.subLock.Lock()
	client.addSubscription("$share/workers/sensors/+/data", QoS1, func(string, []byte) error { return nil })
	client.addSubscription("$share/workers/alerts/#", QoS0, func(string, []byte) error { return nil })
	client.subLock.Unlock()

	err := client.UnsubscribeShared("workers", "sensors/+/data", "alerts/#")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := []string{"$share/workers/sensors/+/data", "$share/workers/alerts/#"}
	if !reflect.DeepEqual(mqttClient.unsubscribedTopics, expected) {
		t.Fatalf("unexpected unsubscribed topics: %#v", mqttClient.unsubscribedTopics)
	}
	if len(client.subscriptions) != 0 {
		t.Fatalf("expected local subscriptions removed, got %d", len(client.subscriptions))
	}
}
