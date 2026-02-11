package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nicexiaonie/gmqtt"
)

/*
EMQX Docker 启动命令:
docker run -d --name emqx -p 18083:18083 -p 1883:1883 emqx/emqx:latest

Dashboard: http://localhost:18083
默认用户名/密码: admin / public

创建测试用户:
docker exec -it emqx emqx ctl users add test1 123456
docker exec -it emqx emqx ctl users add test2 123456
*/

// 测试配置
const (
	brokerAddr = "tcp://127.0.0.1:1883"
	username   = "test1"
	password   = "123456"
)

// 测试结果
type TestResult struct {
	Name      string
	Passed    bool
	Error     error
	Duration  time.Duration
	Details   string
	Timestamp time.Time
}

// 测试统计
type TestStats struct {
	mu           sync.Mutex
	Total        int
	Passed       int
	Failed       int
	Results      []*TestResult
	MessagesSent int64
	MessagesRecv int64
	MessagesLost int64
	MessagesDup  int64
	StartTime    time.Time
	EndTime      time.Time
}

func (ts *TestStats) AddResult(result *TestResult) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.Total++
	if result.Passed {
		ts.Passed++
	} else {
		ts.Failed++
	}
	ts.Results = append(ts.Results, result)
}

func (ts *TestStats) PrintReport() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("GMQTT 全面测试报告")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("测试开始时间: %s\n", ts.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("测试结束时间: %s\n", ts.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("总耗时: %v\n\n", ts.EndTime.Sub(ts.StartTime))

	fmt.Printf("总测试数: %d\n", ts.Total)
	fmt.Printf("通过: %d (%.1f%%)\n", ts.Passed, float64(ts.Passed)/float64(ts.Total)*100)
	fmt.Printf("失败: %d (%.1f%%)\n\n", ts.Failed, float64(ts.Failed)/float64(ts.Total)*100)

	fmt.Printf("消息统计:\n")
	fmt.Printf("  发送: %d\n", atomic.LoadInt64(&ts.MessagesSent))
	fmt.Printf("  接收: %d\n", atomic.LoadInt64(&ts.MessagesRecv))
	fmt.Printf("  丢失: %d\n", atomic.LoadInt64(&ts.MessagesLost))
	fmt.Printf("  重复: %d\n\n", atomic.LoadInt64(&ts.MessagesDup))

	fmt.Println("详细结果:")
	fmt.Println(strings.Repeat("-", 80))

	for i, result := range ts.Results {
		status := "✓ PASS"
		if !result.Passed {
			status = "✗ FAIL"
		}
		fmt.Printf("%3d. [%s] %s (耗时: %v)\n", i+1, status, result.Name, result.Duration)
		if result.Details != "" {
			fmt.Printf("     详情: %s\n", result.Details)
		}
		if result.Error != nil {
			fmt.Printf("     错误: %v\n", result.Error)
		}
	}

	fmt.Println(strings.Repeat("=", 80))
}

// 消息追踪器
type MessageTracker struct {
	mu       sync.Mutex
	sent     map[string]time.Time
	received map[string]time.Time
	payloads map[string]string
}

func NewMessageTracker() *MessageTracker {
	return &MessageTracker{
		sent:     make(map[string]time.Time),
		received: make(map[string]time.Time),
		payloads: make(map[string]string),
	}
}

func (mt *MessageTracker) MarkSent(msgID, payload string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.sent[msgID] = time.Now()
	mt.payloads[msgID] = payload
}

func (mt *MessageTracker) MarkReceived(msgID string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.received[msgID] = time.Now()
}

func (mt *MessageTracker) GetStats() (sent, received, lost, latencyAvg time.Duration) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	sent = time.Duration(len(mt.sent))
	received = time.Duration(len(mt.received))
	lost = sent - received

	var totalLatency time.Duration
	for msgID, sendTime := range mt.sent {
		if recvTime, ok := mt.received[msgID]; ok {
			totalLatency += recvTime.Sub(sendTime)
		}
	}
	if received > 0 {
		latencyAvg = totalLatency / received
	}

	return
}

// 测试框架
type TestSuite struct {
	stats   *TestStats
	tracker *MessageTracker
}

func NewTestSuite() *TestSuite {
	return &TestSuite{
		stats: &TestStats{
			Results:   make([]*TestResult, 0),
			StartTime: time.Now(),
		},
		tracker: NewMessageTracker(),
	}
}

func (ts *TestSuite) Run(name string, fn func() error) {
	start := time.Now()
	result := &TestResult{
		Name:      name,
		Timestamp: start,
	}

	log.Printf("▶ 运行测试: %s", name)
	err := fn()
	result.Duration = time.Since(start)

	if err != nil {
		result.Passed = false
		result.Error = err
		log.Printf("  ✗ 失败: %v", err)
	} else {
		result.Passed = true
		log.Printf("  ✓ 通过 (耗时: %v)", result.Duration)
	}

	ts.stats.AddResult(result)
}

func (ts *TestSuite) RunWithDetails(name string, fn func() (string, error)) {
	start := time.Now()
	result := &TestResult{
		Name:      name,
		Timestamp: start,
	}

	log.Printf("▶ 运行测试: %s", name)
	details, err := fn()
	result.Duration = time.Since(start)
	result.Details = details

	if err != nil {
		result.Passed = false
		result.Error = err
		log.Printf("  ✗ 失败: %v", err)
	} else {
		result.Passed = true
		log.Printf("  ✓ 通过 (耗时: %v)", result.Duration)
		if details != "" {
			log.Printf("  详情: %s", details)
		}
	}

	ts.stats.AddResult(result)
}

// ========== 基础连接测试 ==========

func (ts *TestSuite) TestBasicConnection() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-basic-conn"
	opts.Username = username
	opts.Password = password

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return fmt.Errorf("创建客户端失败: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer client.Disconnect(250)

	if !client.IsConnected() {
		return fmt.Errorf("连接状态错误")
	}

	return nil
}

func (ts *TestSuite) TestConnectionCallbacks() error {
	var connected bool
	var mu sync.Mutex

	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-callbacks"
	opts.Username = username
	opts.Password = password
	opts.OnConnect = func() {
		mu.Lock()
		connected = true
		mu.Unlock()
	}
	opts.OnConnectionLost = func(err error) {
		log.Printf("连接丢失: %v", err)
	}

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	isConnected := connected
	mu.Unlock()

	if !isConnected {
		return fmt.Errorf("OnConnect 回调未触发")
	}

	client.Disconnect(250)
	return nil
}

// ========== QoS 测试 ==========

func (ts *TestSuite) TestQoS(qos gmqtt.QoS, cleanSession bool) (string, error) {
	clientID := fmt.Sprintf("test-qos%d-clean%v", qos, cleanSession)
	topic := fmt.Sprintf("test/qos%d/%s", qos, clientID)

	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = clientID
	opts.Username = username
	opts.Password = password
	opts.CleanSession = cleanSession

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return "", err
	}

	if err := client.Connect(); err != nil {
		return "", err
	}
	defer client.Disconnect(250)

	var received int64
	var receivedMsgs []string
	var mu sync.Mutex
	done := make(chan struct{})

	handler := func(t string, payload []byte) error {
		count := atomic.AddInt64(&received, 1)
		mu.Lock()
		receivedMsgs = append(receivedMsgs, string(payload))
		mu.Unlock()

		if count == 10 {
			close(done)
		}
		return nil
	}

	if err := client.Subscribe(topic, qos, handler); err != nil {
		return "", fmt.Errorf("订阅失败: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发送10条消息
	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("QoS%d-CleanSession%v-Msg%d", qos, cleanSession, i)
		if err := client.Publish(topic, qos, false, msg); err != nil {
			return "", fmt.Errorf("发布失败: %w", err)
		}
		atomic.AddInt64(&ts.stats.MessagesSent, 1)
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, received)
		details := fmt.Sprintf("QoS=%d, CleanSession=%v, 发送=10, 接收=%d", qos, cleanSession, received)
		return details, nil
	case <-time.After(5 * time.Second):
		atomic.AddInt64(&ts.stats.MessagesRecv, received)
		atomic.AddInt64(&ts.stats.MessagesLost, 10-received)
		return "", fmt.Errorf("超时: 只接收到 %d/10 条消息", received)
	}
}

// ========== 持久化会话测试 ==========

func (ts *TestSuite) TestPersistentSession() (string, error) {
	clientID := "test-persistent-session"
	topic := "test/persistent/messages"

	// 第一阶段: 订阅后断开（CleanSession=false）
	opts1 := gmqtt.NewDefaultOptions()
	opts1.Brokers = []string{brokerAddr}
	opts1.ClientID = clientID
	opts1.Username = username
	opts1.Password = password
	opts1.CleanSession = false

	client1, err := gmqtt.NewClient(opts1)
	if err != nil {
		return "", err
	}

	if err := client1.Connect(); err != nil {
		return "", err
	}

	var received1 int64
	handler := func(t string, payload []byte) error {
		atomic.AddInt64(&received1, 1)
		return nil
	}

	if err := client1.Subscribe(topic, gmqtt.QoS1, handler); err != nil {
		client1.Disconnect(250)
		return "", err
	}

	time.Sleep(100 * time.Millisecond)
	client1.Disconnect(250)

	// 第二阶段: 离线时发送消息
	opts2 := gmqtt.NewDefaultOptions()
	opts2.Brokers = []string{brokerAddr}
	opts2.ClientID = "test-publisher"
	opts2.Username = username
	opts2.Password = password

	client2, err := gmqtt.NewClient(opts2)
	if err != nil {
		return "", err
	}

	if err := client2.Connect(); err != nil {
		return "", err
	}

	// 发送10条消息（客户端离线）
	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("Offline-Msg-%d", i)
		if err := client2.Publish(topic, gmqtt.QoS1, false, msg); err != nil {
			client2.Disconnect(250)
			return "", err
		}
		atomic.AddInt64(&ts.stats.MessagesSent, 1)
		time.Sleep(10 * time.Millisecond)
	}
	client2.Disconnect(250)

	// 第三阶段: 重新连接并接收离线消息
	time.Sleep(200 * time.Millisecond)

	opts3 := gmqtt.NewDefaultOptions()
	opts3.Brokers = []string{brokerAddr}
	opts3.ClientID = clientID
	opts3.Username = username
	opts3.Password = password
	opts3.CleanSession = false

	client3, err := gmqtt.NewClient(opts3)
	if err != nil {
		return "", err
	}

	var received2 int64
	done := make(chan struct{})
	handler2 := func(t string, payload []byte) error {
		count := atomic.AddInt64(&received2, 1)
		if count == 10 {
			close(done)
		}
		return nil
	}

	if err := client3.Connect(); err != nil {
		return "", err
	}
	defer client3.Disconnect(250)

	if err := client3.Subscribe(topic, gmqtt.QoS1, handler2); err != nil {
		return "", err
	}

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, received2)
		details := fmt.Sprintf("离线消息测试: 发送=10, 重连后接收=%d", received2)
		return details, nil
	case <-time.After(5 * time.Second):
		atomic.AddInt64(&ts.stats.MessagesRecv, received2)
		atomic.AddInt64(&ts.stats.MessagesLost, 10-received2)
		return "", fmt.Errorf("超时: 只接收到 %d/10 条离线消息", received2)
	}
}

// ========== 保留消息测试 ==========

func (ts *TestSuite) TestRetainedMessages() (string, error) {
	topic := "test/retained/message"
	retainedMsg := "This is a retained message"

	// 发送保留消息
	opts1 := gmqtt.NewDefaultOptions()
	opts1.Brokers = []string{brokerAddr}
	opts1.ClientID = "test-retained-pub"
	opts1.Username = username
	opts1.Password = password

	client1, err := gmqtt.NewClient(opts1)
	if err != nil {
		return "", err
	}

	if err := client1.Connect(); err != nil {
		return "", err
	}

	if err := client1.Publish(topic, gmqtt.QoS1, true, retainedMsg); err != nil {
		client1.Disconnect(250)
		return "", err
	}
	atomic.AddInt64(&ts.stats.MessagesSent, 1)
	client1.Disconnect(250)

	// 等待消息被保存
	time.Sleep(200 * time.Millisecond)

	// 新客户端订阅并接收保留消息
	opts2 := gmqtt.NewDefaultOptions()
	opts2.Brokers = []string{brokerAddr}
	opts2.ClientID = "test-retained-sub"
	opts2.Username = username
	opts2.Password = password

	client2, err := gmqtt.NewClient(opts2)
	if err != nil {
		return "", err
	}

	if err := client2.Connect(); err != nil {
		return "", err
	}
	defer client2.Disconnect(250)

	var receivedMsg string
	done := make(chan struct{})

	handler := func(t string, payload []byte) error {
		receivedMsg = string(payload)
		close(done)
		return nil
	}

	if err := client2.Subscribe(topic, gmqtt.QoS1, handler); err != nil {
		return "", err
	}

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, 1)
		if receivedMsg == retainedMsg {
			return "保留消息正确接收", nil
		}
		return "", fmt.Errorf("保留消息内容不匹配: 期望=%s, 实际=%s", retainedMsg, receivedMsg)
	case <-time.After(3 * time.Second):
		return "", fmt.Errorf("未接收到保留消息")
	}
}

// ========== Publisher 测试 ==========

func (ts *TestSuite) TestPublisher() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-publisher-api"
	opts.Username = username
	opts.Password = password

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect(250)

	publisher := gmqtt.NewPublisher(client, "test/publisher", gmqtt.QoS1)

	// 测试不同的发布方法
	if err := publisher.PublishString("test string"); err != nil {
		return fmt.Errorf("PublishString 失败: %w", err)
	}

	if err := publisher.PublishBytes([]byte("test bytes")); err != nil {
		return fmt.Errorf("PublishBytes 失败: %w", err)
	}

	testData := map[string]interface{}{"key": "value", "count": 42}
	if err := publisher.PublishJSON(testData); err != nil {
		return fmt.Errorf("PublishJSON 失败: %w", err)
	}

	// 测试 SetTopic/GetTopic
	publisher.SetTopic("test/new-topic")
	if publisher.GetTopic() != "test/new-topic" {
		return fmt.Errorf("SetTopic/GetTopic 失败")
	}

	// 测试 SetQoS/GetQoS
	publisher.SetQoS(gmqtt.QoS2)
	if publisher.GetQoS() != gmqtt.QoS2 {
		return fmt.Errorf("SetQoS/GetQoS 失败")
	}

	atomic.AddInt64(&ts.stats.MessagesSent, 3)
	return nil
}

// ========== Subscriber 测试 ==========

func (ts *TestSuite) TestSubscriber() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-subscriber-api"
	opts.Username = username
	opts.Password = password

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect(250)

	// 测试 Subscriber
	subscriber := gmqtt.NewSubscriber(client, []string{"test/sub1", "test/sub2"}, gmqtt.QoS1)

	var received int64
	subscriber.SetStringHandler(func(topic string, message string) error {
		atomic.AddInt64(&received, 1)
		return nil
	})

	if err := subscriber.Subscribe(); err != nil {
		return fmt.Errorf("Subscribe 失败: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发送测试消息
	if err := client.Publish("test/sub1", gmqtt.QoS1, false, "msg1"); err != nil {
		return err
	}
	if err := client.Publish("test/sub2", gmqtt.QoS1, false, "msg2"); err != nil {
		return err
	}
	atomic.AddInt64(&ts.stats.MessagesSent, 2)

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt64(&received) != 2 {
		return fmt.Errorf("接收消息数量错误: 期望=2, 实际=%d", received)
	}
	atomic.AddInt64(&ts.stats.MessagesRecv, 2)

	// 测试 AddTopic
	subscriber.AddTopic("test/sub3")
	if len(subscriber.GetTopics()) != 3 {
		return fmt.Errorf("AddTopic 失败")
	}

	// 测试 RemoveTopic
	subscriber.RemoveTopic("test/sub2")
	if len(subscriber.GetTopics()) != 2 {
		return fmt.Errorf("RemoveTopic 失败")
	}

	// 测试 Unsubscribe
	if err := subscriber.Unsubscribe(); err != nil {
		return fmt.Errorf("Unsubscribe 失败: %w", err)
	}

	return nil
}

// ========== SubscriberGroup 测试 ==========

func (ts *TestSuite) TestSubscriberGroup() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-sub-group"
	opts.Username = username
	opts.Password = password

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect(250)

	group := gmqtt.NewSubscriberGroup()

	var received int64
	handler := func(topic string, payload []byte) error {
		atomic.AddInt64(&received, 1)
		return nil
	}

	sub1 := gmqtt.NewSubscriber(client, []string{"test/group1"}, gmqtt.QoS1)
	sub1.SetHandler(handler)

	sub2 := gmqtt.NewSubscriber(client, []string{"test/group2"}, gmqtt.QoS1)
	sub2.SetHandler(handler)

	group.Add(sub1)
	group.Add(sub2)

	if group.Count() != 2 {
		return fmt.Errorf("Count 错误: 期望=2, 实际=%d", group.Count())
	}

	if err := group.SubscribeAll(); err != nil {
		return fmt.Errorf("SubscribeAll 失败: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发送测试消息
	if err := client.Publish("test/group1", gmqtt.QoS1, false, "msg1"); err != nil {
		return err
	}
	if err := client.Publish("test/group2", gmqtt.QoS1, false, "msg2"); err != nil {
		return err
	}
	atomic.AddInt64(&ts.stats.MessagesSent, 2)

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt64(&received) != 2 {
		return fmt.Errorf("接收消息数量错误: 期望=2, 实际=%d", received)
	}
	atomic.AddInt64(&ts.stats.MessagesRecv, 2)

	if err := group.UnsubscribeAll(); err != nil {
		return fmt.Errorf("UnsubscribeAll 失败: %w", err)
	}

	return nil
}

// ========== 全局客户端管理测试 ==========

func (ts *TestSuite) TestGlobalClientManagement() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-global-client"
	opts.Username = username
	opts.Password = password

	// RegisterClient
	client, err := gmqtt.RegisterClient("test-client", opts)
	if err != nil {
		return fmt.Errorf("RegisterClient 失败: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	// GetClient
	retrievedClient, err := gmqtt.GetClient("test-client")
	if err != nil {
		return fmt.Errorf("GetClient 失败: %w", err)
	}

	if retrievedClient != client {
		return fmt.Errorf("GetClient 返回的客户端不一致")
	}

	// CloseAll
	gmqtt.CloseAll(250)

	return nil
}

// ========== 多客户端并发测试 ==========

func (ts *TestSuite) TestConcurrentClients() (string, error) {
	const numClients = 10
	const msgsPerClient = 10
	topic := "test/concurrent"

	var wg sync.WaitGroup
	var errors []error
	var errMu sync.Mutex

	// 创建订阅客户端
	subOpts := gmqtt.NewDefaultOptions()
	subOpts.Brokers = []string{brokerAddr}
	subOpts.ClientID = "test-concurrent-sub"
	subOpts.Username = username
	subOpts.Password = password

	subClient, err := gmqtt.NewClient(subOpts)
	if err != nil {
		return "", err
	}

	if err := subClient.Connect(); err != nil {
		return "", err
	}
	defer subClient.Disconnect(250)

	var received int64
	expected := int64(numClients * msgsPerClient)
	done := make(chan struct{})

	handler := func(t string, payload []byte) error {
		count := atomic.AddInt64(&received, 1)
		if count == expected {
			close(done)
		}
		return nil
	}

	if err := subClient.Subscribe(topic, gmqtt.QoS1, handler); err != nil {
		return "", err
	}

	time.Sleep(100 * time.Millisecond)

	// 创建多个发布客户端
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()

			opts := gmqtt.NewDefaultOptions()
			opts.Brokers = []string{brokerAddr}
			opts.ClientID = fmt.Sprintf("test-concurrent-pub-%d", clientNum)
			opts.Username = username
			opts.Password = password

			client, err := gmqtt.NewClient(opts)
			if err != nil {
				errMu.Lock()
				errors = append(errors, err)
				errMu.Unlock()
				return
			}

			if err := client.Connect(); err != nil {
				errMu.Lock()
				errors = append(errors, err)
				errMu.Unlock()
				return
			}
			defer client.Disconnect(250)

			for j := 0; j < msgsPerClient; j++ {
				msg := fmt.Sprintf("Client-%d-Msg-%d", clientNum, j)
				if err := client.Publish(topic, gmqtt.QoS1, false, msg); err != nil {
					errMu.Lock()
					errors = append(errors, err)
					errMu.Unlock()
					return
				}
				atomic.AddInt64(&ts.stats.MessagesSent, 1)
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	if len(errors) > 0 {
		return "", fmt.Errorf("发生 %d 个错误: %v", len(errors), errors[0])
	}

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, received)
		details := fmt.Sprintf("并发测试: %d个客户端, 每个发送%d条, 总接收=%d", numClients, msgsPerClient, received)
		return details, nil
	case <-time.After(10 * time.Second):
		atomic.AddInt64(&ts.stats.MessagesRecv, received)
		atomic.AddInt64(&ts.stats.MessagesLost, expected-received)
		return "", fmt.Errorf("超时: 只接收到 %d/%d 条消息", received, expected)
	}
}

// ========== 遗嘱消息测试 ==========

func (ts *TestSuite) TestWillMessage() (string, error) {
	willTopic := "test/will"
	willMessage := "Client disconnected unexpectedly"

	// 创建监听遗嘱消息的客户端
	opts1 := gmqtt.NewDefaultOptions()
	opts1.Brokers = []string{brokerAddr}
	opts1.ClientID = "test-will-listener"
	opts1.Username = username
	opts1.Password = password

	listener, err := gmqtt.NewClient(opts1)
	if err != nil {
		return "", err
	}

	if err := listener.Connect(); err != nil {
		return "", err
	}
	defer listener.Disconnect(250)

	var receivedMsg string
	done := make(chan struct{})

	handler := func(t string, payload []byte) error {
		receivedMsg = string(payload)
		close(done)
		return nil
	}

	if err := listener.Subscribe(willTopic, gmqtt.QoS1, handler); err != nil {
		return "", err
	}

	time.Sleep(100 * time.Millisecond)

	// 创建带遗嘱消息的客户端
	opts2 := gmqtt.NewDefaultOptions()
	opts2.Brokers = []string{brokerAddr}
	opts2.ClientID = "test-will-client"
	opts2.Username = username
	opts2.Password = password
	opts2.WillEnabled = true
	opts2.WillTopic = willTopic
	opts2.WillPayload = []byte(willMessage)
	opts2.WillQoS = gmqtt.QoS1
	opts2.WillRetained = false

	willClient, err := gmqtt.NewClient(opts2)
	if err != nil {
		return "", err
	}

	if err := willClient.Connect(); err != nil {
		return "", err
	}

	// 模拟异常断开（使用0毫秒超时）
	time.Sleep(100 * time.Millisecond)
	willClient.Disconnect(0)

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, 1)
		if receivedMsg == willMessage {
			return "遗嘱消息正确接收", nil
		}
		return "", fmt.Errorf("遗嘱消息内容不匹配: 期望=%s, 实际=%s", willMessage, receivedMsg)
	case <-time.After(3 * time.Second):
		return "", fmt.Errorf("未接收到遗嘱消息")
	}
}

// ========== JSON 消息测试 ==========

func (ts *TestSuite) TestJSONMessages() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-json"
	opts.Username = username
	opts.Password = password

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect(250)

	type TestData struct {
		ID      int     `json:"id"`
		Message string  `json:"message"`
		Value   float64 `json:"value"`
	}

	sendData := TestData{
		ID:      123,
		Message: "test json",
		Value:   45.67,
	}

	done := make(chan struct{})
	var recvData TestData

	subscriber := gmqtt.NewSubscriber(client, []string{"test/json"}, gmqtt.QoS1)
	subscriber.SetJSONHandler(func(topic string, data interface{}) error {
		recvData = *(data.(*TestData))
		close(done)
		return nil
	}, &TestData{})

	if err := subscriber.Subscribe(); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	publisher := gmqtt.NewPublisher(client, "test/json", gmqtt.QoS1)
	if err := publisher.PublishJSON(sendData); err != nil {
		return err
	}
	atomic.AddInt64(&ts.stats.MessagesSent, 1)

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, 1)
		if recvData.ID != sendData.ID || recvData.Message != sendData.Message || recvData.Value != sendData.Value {
			return fmt.Errorf("JSON 数据不匹配")
		}
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("超时: 未接收到 JSON 消息")
	}
}

// ========== 批量订阅测试 ==========

func (ts *TestSuite) TestMultipleSubscribe() error {
	opts := gmqtt.NewDefaultOptions()
	opts.Brokers = []string{brokerAddr}
	opts.ClientID = "test-multi-sub"
	opts.Username = username
	opts.Password = password

	client, err := gmqtt.NewClient(opts)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect(250)

	topics := map[string]gmqtt.QoS{
		"test/multi/1": gmqtt.QoS0,
		"test/multi/2": gmqtt.QoS1,
		"test/multi/3": gmqtt.QoS2,
	}

	var received int64
	done := make(chan struct{})

	handler := func(topic string, payload []byte) error {
		count := atomic.AddInt64(&received, 1)
		if count == 3 {
			close(done)
		}
		return nil
	}

	if err := client.SubscribeMultiple(topics, handler); err != nil {
		return fmt.Errorf("SubscribeMultiple 失败: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 发送到各个主题
	for topic := range topics {
		if err := client.Publish(topic, gmqtt.QoS1, false, "test"); err != nil {
			return err
		}
		atomic.AddInt64(&ts.stats.MessagesSent, 1)
	}

	select {
	case <-done:
		atomic.AddInt64(&ts.stats.MessagesRecv, 3)
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("超时: 只接收到 %d/3 条消息", received)
	}
}

// ========== 主测试入口 ==========

func main() {
	// 设置日志输出到文件和控制台
	logFile, err := os.OpenFile("gmqtt_test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("无法创建日志文件:", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("GMQTT 全面功能测试")
	fmt.Println("========================================")
	fmt.Printf("Broker: %s\n", brokerAddr)
	fmt.Printf("用户名: %s\n", username)
	fmt.Println("========================================\n")

	suite := NewTestSuite()

	// ========== 1. 基础连接测试 ==========
	fmt.Println("\n【1. 基础连接测试】")
	suite.Run("1.1 基础连接与断开", suite.TestBasicConnection)
	suite.Run("1.2 连接回调函数", suite.TestConnectionCallbacks)

	// ========== 2. QoS 测试 ==========
	fmt.Println("\n【2. QoS 级别测试】")
	suite.RunWithDetails("2.1 QoS0 + CleanSession=true", func() (string, error) {
		return suite.TestQoS(gmqtt.QoS0, true)
	})
	suite.RunWithDetails("2.2 QoS0 + CleanSession=false", func() (string, error) {
		return suite.TestQoS(gmqtt.QoS0, false)
	})
	suite.RunWithDetails("2.3 QoS1 + CleanSession=true", func() (string, error) {
		return suite.TestQoS(gmqtt.QoS1, true)
	})
	suite.RunWithDetails("2.4 QoS1 + CleanSession=false", func() (string, error) {
		return suite.TestQoS(gmqtt.QoS1, false)
	})
	suite.RunWithDetails("2.5 QoS2 + CleanSession=true", func() (string, error) {
		return suite.TestQoS(gmqtt.QoS2, true)
	})
	suite.RunWithDetails("2.6 QoS2 + CleanSession=false", func() (string, error) {
		return suite.TestQoS(gmqtt.QoS2, false)
	})

	// ========== 3. 持久化会话测试 ==========
	fmt.Println("\n【3. 持久化会话测试】")
	suite.RunWithDetails("3.1 离线消息接收测试", suite.TestPersistentSession)

	// ========== 4. 保留消息测试 ==========
	fmt.Println("\n【4. 保留消息测试】")
	suite.RunWithDetails("4.1 保留消息发送与接收", suite.TestRetainedMessages)

	// ========== 5. 遗嘱消息测试 ==========
	fmt.Println("\n【5. 遗嘱消息测试】")
	suite.RunWithDetails("5.1 遗嘱消息触发", suite.TestWillMessage)

	// ========== 6. Publisher API 测试 ==========
	fmt.Println("\n【6. Publisher API 测试】")
	suite.Run("6.1 Publisher 所有方法", suite.TestPublisher)

	// ========== 7. Subscriber API 测试 ==========
	fmt.Println("\n【7. Subscriber API 测试】")
	suite.Run("7.1 Subscriber 所有方法", suite.TestSubscriber)
	suite.Run("7.2 SubscriberGroup 管理", suite.TestSubscriberGroup)
	suite.Run("7.3 批量订阅", suite.TestMultipleSubscribe)

	// ========== 8. JSON 消息测试 ==========
	fmt.Println("\n【8. JSON 消息测试】")
	suite.Run("8.1 JSON 序列化与反序列化", suite.TestJSONMessages)

	// ========== 9. 全局客户端管理 ==========
	fmt.Println("\n【9. 全局客户端管理】")
	suite.Run("9.1 RegisterClient/GetClient/CloseAll", suite.TestGlobalClientManagement)

	// ========== 10. 并发测试 ==========
	fmt.Println("\n【10. 并发压力测试】")
	suite.RunWithDetails("10.1 多客户端并发发布", suite.TestConcurrentClients)

	// 生成测试报告
	suite.stats.EndTime = time.Now()
	suite.stats.PrintReport()

	// 保存JSON报告
	reportData, _ := json.MarshalIndent(suite.stats, "", "  ")
	os.WriteFile("gmqtt_test_report.json", reportData, 0644)
	fmt.Println("\n详细报告已保存到: gmqtt_test_report.json")
	fmt.Println("日志文件: gmqtt_test.log")
}
