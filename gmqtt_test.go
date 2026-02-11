package gmqtt

import (
	"testing"
	"time"
)

// TestNewDefaultOptions 测试默认配置
func TestNewDefaultOptions(t *testing.T) {
	opts := NewDefaultOptions()

	if opts.CleanSession != true {
		t.Error("CleanSession should be true by default")
	}

	if opts.KeepAlive != 60*time.Second {
		t.Error("KeepAlive should be 60 seconds by default")
	}

	if opts.ConnectTimeout != 30*time.Second {
		t.Error("ConnectTimeout should be 30 seconds by default")
	}

	if opts.AutoReconnect != true {
		t.Error("AutoReconnect should be true by default")
	}

	if opts.MaxReconnectInterval != 10*time.Minute {
		t.Error("MaxReconnectInterval should be 10 minutes by default")
	}

	if opts.MessageChannelDepth != 100 {
		t.Error("MessageChannelDepth should be 100 by default")
	}

	if opts.WriteTimeout != 30*time.Second {
		t.Error("WriteTimeout should be 30 seconds by default")
	}

	if opts.ResumeSubs != true {
		t.Error("ResumeSubs should be true by default")
	}

	if opts.OrderMatters != true {
		t.Error("OrderMatters should be true by default")
	}
}

// TestOptionsValidate 测试配置验证
func TestOptionsValidate(t *testing.T) {
	// 测试空 Brokers
	opts := &ClientOptions{
		ClientID: "test-client",
	}
	if err := opts.Validate(); err != ErrNoBrokers {
		t.Errorf("Expected ErrNoBrokers, got %v", err)
	}

	// 测试空 ClientID
	opts = &ClientOptions{
		Brokers: []string{"tcp://localhost:1883"},
	}
	if err := opts.Validate(); err != ErrNoClientID {
		t.Errorf("Expected ErrNoClientID, got %v", err)
	}

	// 测试有效配置
	opts = &ClientOptions{
		Brokers:  []string{"tcp://localhost:1883"},
		ClientID: "test-client",
	}
	if err := opts.Validate(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestQoSValues 测试 QoS 值
func TestQoSValues(t *testing.T) {
	if QoS0 != 0 {
		t.Error("QoS0 should be 0")
	}
	if QoS1 != 1 {
		t.Error("QoS1 should be 1")
	}
	if QoS2 != 2 {
		t.Error("QoS2 should be 2")
	}
}

// TestSubscription 测试订阅结构
func TestSubscription(t *testing.T) {
	handler := func(topic string, payload []byte) error {
		return nil
	}

	sub := &Subscription{
		Topic:   "test/topic",
		QoS:     QoS1,
		Handler: handler,
	}

	if sub.Topic != "test/topic" {
		t.Error("Topic mismatch")
	}

	if sub.QoS != QoS1 {
		t.Error("QoS mismatch")
	}

	if sub.Handler == nil {
		t.Error("Handler should not be nil")
	}
}

// TestPublisher 测试发布者
func TestPublisher(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{"tcp://localhost:1883"}
	opts.ClientID = "test-publisher"

	// 注意：这个测试需要实际的 MQTT Broker
	// 在没有 Broker 的情况下，只测试结构创建
	client := &Client{
		options: opts,
	}

	publisher := NewPublisher(client, "test/topic", QoS1)

	if publisher.GetTopic() != "test/topic" {
		t.Error("Topic mismatch")
	}

	if publisher.GetQoS() != QoS1 {
		t.Error("QoS mismatch")
	}

	// 测试设置方法
	publisher.SetTopic("new/topic")
	if publisher.GetTopic() != "new/topic" {
		t.Error("SetTopic failed")
	}

	publisher.SetQoS(QoS2)
	if publisher.GetQoS() != QoS2 {
		t.Error("SetQoS failed")
	}
}

// TestSubscriber 测试订阅者
func TestSubscriber(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{"tcp://localhost:1883"}
	opts.ClientID = "test-subscriber"

	client := &Client{
		options: opts,
	}

	topics := []string{"test/topic1", "test/topic2"}
	subscriber := NewSubscriber(client, topics, QoS1)

	if len(subscriber.GetTopics()) != 2 {
		t.Error("Topics count mismatch")
	}

	if subscriber.GetQoS() != QoS1 {
		t.Error("QoS mismatch")
	}

	// 测试添加主题
	subscriber.AddTopic("test/topic3")
	if len(subscriber.GetTopics()) != 3 {
		t.Error("AddTopic failed")
	}

	// 测试移除主题
	subscriber.RemoveTopic("test/topic2")
	if len(subscriber.GetTopics()) != 2 {
		t.Error("RemoveTopic failed")
	}

	// 测试设置 QoS
	subscriber.SetQoS(QoS2)
	if subscriber.GetQoS() != QoS2 {
		t.Error("SetQoS failed")
	}
}

// TestSubscriberGroup 测试订阅者组
func TestSubscriberGroup(t *testing.T) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{"tcp://localhost:1883"}
	opts.ClientID = "test-group"

	client := &Client{
		options: opts,
	}

	group := NewSubscriberGroup()

	if group.Count() != 0 {
		t.Error("Initial count should be 0")
	}

	// 添加订阅者
	sub1 := NewSubscriber(client, []string{"topic1"}, QoS1)
	group.Add(sub1)

	if group.Count() != 1 {
		t.Error("Count should be 1 after adding one subscriber")
	}

	sub2 := NewSubscriber(client, []string{"topic2"}, QoS1)
	group.Add(sub2)

	if group.Count() != 2 {
		t.Error("Count should be 2 after adding two subscribers")
	}
}

// TestErrorTypes 测试错误类型
func TestErrorTypes(t *testing.T) {
	errors := []error{
		ErrNoBrokers,
		ErrNoClientID,
		ErrNotConnected,
		ErrAlreadyConnected,
		ErrNoHandler,
		ErrClientNotFound,
		ErrPublishTimeout,
		ErrSubscribeTimeout,
		ErrUnsubscribeTimeout,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// TestGlobalClientManagement 测试全局客户端管理
func TestGlobalClientManagement(t *testing.T) {
	// 清理之前的客户端
	CloseAll(0)

	// 测试获取不存在的客户端
	_, err := GetClient("non-existent")
	if err != ErrClientNotFound {
		t.Errorf("Expected ErrClientNotFound, got %v", err)
	}

	// 注意：实际的注册测试需要真实的 MQTT Broker
	// 这里只测试错误情况

	// 测试无效配置
	opts := &ClientOptions{
		ClientID: "test",
		// 缺少 Brokers
	}
	_, err = RegisterClient("test", opts)
	if err != ErrNoBrokers {
		t.Errorf("Expected ErrNoBrokers, got %v", err)
	}
}

// BenchmarkPublisher 性能测试：发布者创建
func BenchmarkPublisher(b *testing.B) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{"tcp://localhost:1883"}
	opts.ClientID = "bench-client"

	client := &Client{
		options: opts,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewPublisher(client, "test/topic", QoS1)
	}
}

// BenchmarkSubscriber 性能测试：订阅者创建
func BenchmarkSubscriber(b *testing.B) {
	opts := NewDefaultOptions()
	opts.Brokers = []string{"tcp://localhost:1883"}
	opts.ClientID = "bench-client"

	client := &Client{
		options: opts,
	}

	topics := []string{"test/topic"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscriber(client, topics, QoS1)
	}
}

// BenchmarkOptionsValidate 性能测试：配置验证
func BenchmarkOptionsValidate(b *testing.B) {
	opts := &ClientOptions{
		Brokers:  []string{"tcp://localhost:1883"},
		ClientID: "test-client",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = opts.Validate()
	}
}
