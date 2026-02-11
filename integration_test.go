// +build integration

package gmqtt

/*
集成测试

注意：这些测试需要运行 MQTT Broker
可以使用 Docker 快速启动一个测试 Broker：

docker run -d --name mosquitto -p 1883:1883 eclipse-mosquitto:latest

运行测试：
go test -v -tags=integration

*/

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

const (
	testBroker   = "tcp://localhost:1883"
	testUsername = ""
	testPassword = ""
)

// TestIntegrationBasicPubSub 测试基础发布订阅
func TestIntegrationBasicPubSub(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-basic-pubsub"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	// 订阅
	received := make(chan string, 1)
	err = client.Subscribe("test/basic", QoS1, func(topic string, payload []byte) error {
		received <- string(payload)
		return nil
	})
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 发布
	testMessage := "Hello MQTT"
	err = client.Publish("test/basic", QoS1, false, testMessage)
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}

	// 验证
	select {
	case msg := <-received:
		if msg != testMessage {
			t.Errorf("消息不匹配: 期望 %s, 收到 %s", testMessage, msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("超时未收到消息")
	}
}

// TestIntegrationQoS 测试不同 QoS 级别
func TestIntegrationQoS(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-qos"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	qosLevels := []QoS{QoS0, QoS1, QoS2}

	for _, qos := range qosLevels {
		t.Run(fmt.Sprintf("QoS%d", qos), func(t *testing.T) {
			topic := fmt.Sprintf("test/qos/%d", qos)
			received := make(chan bool, 1)

			err := client.Subscribe(topic, qos, func(topic string, payload []byte) error {
				received <- true
				return nil
			})
			if err != nil {
				t.Fatalf("订阅失败: %v", err)
			}

			time.Sleep(500 * time.Millisecond)

			err = client.Publish(topic, qos, false, fmt.Sprintf("QoS %d message", qos))
			if err != nil {
				t.Fatalf("发布失败: %v", err)
			}

			select {
			case <-received:
				// 成功
			case <-time.After(5 * time.Second):
				t.Error("超时未收到消息")
			}
		})
	}
}

// TestIntegrationPublisher 测试 Publisher
func TestIntegrationPublisher(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-publisher"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	// 订阅
	received := make(chan string, 10)
	err = client.Subscribe("test/publisher", QoS1, func(topic string, payload []byte) error {
		received <- string(payload)
		return nil
	})
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 创建 Publisher
	publisher := NewPublisher(client, "test/publisher", QoS1)

	// 测试字符串发布
	err = publisher.PublishString("string message")
	if err != nil {
		t.Fatalf("发布字符串失败: %v", err)
	}

	// 测试字节发布
	err = publisher.PublishBytes([]byte("bytes message"))
	if err != nil {
		t.Fatalf("发布字节失败: %v", err)
	}

	// 测试 JSON 发布
	type TestData struct {
		Value string `json:"value"`
	}
	err = publisher.PublishJSON(TestData{Value: "json message"})
	if err != nil {
		t.Fatalf("发布 JSON 失败: %v", err)
	}

	// 验证收到 3 条消息
	count := 0
	timeout := time.After(5 * time.Second)
	for count < 3 {
		select {
		case <-received:
			count++
		case <-timeout:
			t.Errorf("只收到 %d 条消息，期望 3 条", count)
			return
		}
	}
}

// TestIntegrationSubscriber 测试 Subscriber
func TestIntegrationSubscriber(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-subscriber"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	// 测试字符串处理器
	received := make(chan string, 1)
	subscriber := NewSubscriber(client, []string{"test/subscriber/string"}, QoS1)
	subscriber.SetStringHandler(func(topic string, message string) error {
		received <- message
		return nil
	})

	err = subscriber.Subscribe()
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	client.Publish("test/subscriber/string", QoS1, false, "test message")

	select {
	case msg := <-received:
		if msg != "test message" {
			t.Errorf("消息不匹配: 期望 'test message', 收到 '%s'", msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("超时未收到消息")
	}
}

// TestIntegrationJSONHandler 测试 JSON 处理器
func TestIntegrationJSONHandler(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-json-handler"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	type SensorData struct {
		DeviceID    string  `json:"device_id"`
		Temperature float64 `json:"temperature"`
	}

	received := make(chan SensorData, 1)
	subscriber := NewSubscriber(client, []string{"test/json"}, QoS1)

	var data SensorData
	subscriber.SetJSONHandler(func(topic string, d interface{}) error {
		sensorData := d.(*SensorData)
		received <- *sensorData
		return nil
	}, &data)

	err = subscriber.Subscribe()
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 发布 JSON 数据
	testData := SensorData{
		DeviceID:    "sensor-001",
		Temperature: 25.5,
	}

	jsonData, _ := json.Marshal(testData)
	client.Publish("test/json", QoS1, false, jsonData)

	select {
	case receivedData := <-received:
		if receivedData.DeviceID != testData.DeviceID {
			t.Errorf("DeviceID 不匹配")
		}
		if receivedData.Temperature != testData.Temperature {
			t.Errorf("Temperature 不匹配")
		}
	case <-time.After(5 * time.Second):
		t.Error("超时未收到消息")
	}
}

// TestIntegrationMultipleSubscribe 测试批量订阅
func TestIntegrationMultipleSubscribe(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-multiple-sub"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	received := make(map[string]bool)
	var mu sync.Mutex

	subscriptions := map[string]QoS{
		"test/multi/1": QoS1,
		"test/multi/2": QoS1,
		"test/multi/3": QoS1,
	}

	err = client.SubscribeMultiple(subscriptions, func(topic string, payload []byte) error {
		mu.Lock()
		received[topic] = true
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("批量订阅失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 发布到所有主题
	for topic := range subscriptions {
		client.Publish(topic, QoS1, false, "test")
	}

	time.Sleep(2 * time.Second)

	// 验证所有主题都收到消息
	mu.Lock()
	defer mu.Unlock()

	if len(received) != len(subscriptions) {
		t.Errorf("收到的主题数量不匹配: 期望 %d, 收到 %d", len(subscriptions), len(received))
	}

	for topic := range subscriptions {
		if !received[topic] {
			t.Errorf("未收到主题 %s 的消息", topic)
		}
	}
}

// TestIntegrationWildcard 测试通配符订阅
func TestIntegrationWildcard(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-wildcard"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client.Disconnect(250)

	received := make(chan string, 10)

	// 测试单级通配符 +
	err = client.Subscribe("test/wildcard/+/data", QoS1, func(topic string, payload []byte) error {
		received <- topic
		return nil
	})
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 发布到匹配的主题
	topics := []string{
		"test/wildcard/001/data",
		"test/wildcard/002/data",
		"test/wildcard/003/data",
	}

	for _, topic := range topics {
		client.Publish(topic, QoS1, false, "test")
	}

	// 验证收到所有消息
	count := 0
	timeout := time.After(5 * time.Second)
	for count < len(topics) {
		select {
		case <-received:
			count++
		case <-timeout:
			t.Errorf("只收到 %d 条消息，期望 %d 条", count, len(topics))
			return
		}
	}
}

// TestIntegrationRetainedMessage 测试保留消息
func TestIntegrationRetainedMessage(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{testBroker}
	opts.ClientID = "test-retained-pub"

	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 发布保留消息
	testMessage := "retained message"
	err = client.Publish("test/retained", QoS1, true, testMessage)
	if err != nil {
		t.Fatalf("发布保留消息失败: %v", err)
	}

	client.Disconnect(250)
	time.Sleep(1 * time.Second)

	// 创建新客户端订阅
	opts2 := NewDefaultOptions()
	opts2.Brokers = []string{testBroker}
	opts2.ClientID = "test-retained-sub"

	client2, err := NewClient(opts2)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	if err := client2.Connect(); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer client2.Disconnect(250)

	received := make(chan string, 1)
	err = client2.Subscribe("test/retained", QoS1, func(topic string, payload []byte) error {
		received <- string(payload)
		return nil
	})
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	// 应该立即收到保留消息
	select {
	case msg := <-received:
		if msg != testMessage {
			t.Errorf("保留消息不匹配: 期望 '%s', 收到 '%s'", testMessage, msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("超时未收到保留消息")
	}

	// 清除保留消息
	client2.Publish("test/retained", QoS1, true, "")
}
