package gmqtt

import (
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	// 全局客户端管理
	clients     = make(map[string]*Client)
	clientsLock sync.RWMutex
)

// Client MQTT客户端
type Client struct {
	options *ClientOptions
	client  mqtt.Client
	mu      sync.RWMutex

	// 订阅管理
	subscriptions map[string]*Subscription
	subLock       sync.RWMutex

	// 默认消息处理器
	defaultHandler MessageHandler
}

// NewClient 创建新的MQTT客户端
func NewClient(opts *ClientOptions) (*Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	c := &Client{
		options:       opts,
		subscriptions: make(map[string]*Subscription),
	}

	// 创建paho客户端选项
	mqttOpts := mqtt.NewClientOptions()

	// 设置Broker地址
	for _, broker := range opts.Brokers {
		mqttOpts.AddBroker(broker)
	}

	// 基础配置
	mqttOpts.SetClientID(opts.ClientID)
	mqttOpts.SetCleanSession(opts.CleanSession)
	mqttOpts.SetKeepAlive(opts.KeepAlive)
	mqttOpts.SetConnectTimeout(opts.ConnectTimeout)
	mqttOpts.SetAutoReconnect(opts.AutoReconnect)
	mqttOpts.SetMaxReconnectInterval(opts.MaxReconnectInterval)
	mqttOpts.SetConnectRetry(opts.ConnectRetry)
	mqttOpts.SetConnectRetryInterval(opts.ConnectRetryInterval)
	mqttOpts.SetWriteTimeout(opts.WriteTimeout)
	mqttOpts.SetMessageChannelDepth(opts.MessageChannelDepth)
	mqttOpts.SetResumeSubs(opts.ResumeSubs)
	mqttOpts.SetOrderMatters(opts.OrderMatters)

	// 认证
	if opts.Username != "" {
		mqttOpts.SetUsername(opts.Username)
	}
	if opts.Password != "" {
		mqttOpts.SetPassword(opts.Password)
	}

	// TLS配置
	if opts.TLSConfig != nil {
		mqttOpts.SetTLSConfig(opts.TLSConfig)
	}

	// 遗嘱消息
	if opts.WillEnabled {
		mqttOpts.SetWill(opts.WillTopic, string(opts.WillPayload), byte(opts.WillQoS), opts.WillRetained)
	}

	// 连接回调
	if opts.OnConnect != nil {
		mqttOpts.SetOnConnectHandler(func(client mqtt.Client) {
			log.Printf("gmqtt: client [%s] connected", opts.ClientID)
			opts.OnConnect()
		})
	}

	// 连接丢失回调
	if opts.OnConnectionLost != nil {
		mqttOpts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
			log.Printf("gmqtt: client [%s] connection lost: %v", opts.ClientID, err)
			opts.OnConnectionLost(err)
		})
	}

	// 默认消息处理器
	mqttOpts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		c.handleMessage(msg)
	})

	c.client = mqtt.NewClient(mqttOpts)
	return c, nil
}

// Connect 连接到MQTT Broker
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client.IsConnected() {
		return ErrAlreadyConnected
	}

	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("gmqtt: connect failed: %w", token.Error())
	}

	log.Printf("gmqtt: client [%s] connected successfully", c.options.ClientID)
	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect(quiesce uint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client.IsConnected() {
		c.client.Disconnect(quiesce)
		log.Printf("gmqtt: client [%s] disconnected", c.options.ClientID)
	}
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.IsConnected()
}

// Publish 发布消息
func (c *Client) Publish(topic string, qos QoS, retained bool, payload interface{}) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	token := c.client.Publish(topic, byte(qos), retained, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("gmqtt: publish failed: %w", token.Error())
	}

	return nil
}

// PublishWithTimeout 发布消息（带超时）
func (c *Client) PublishWithTimeout(topic string, qos QoS, retained bool, payload interface{}, timeout time.Duration) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	token := c.client.Publish(topic, byte(qos), retained, payload)
	if !token.WaitTimeout(timeout) {
		return ErrPublishTimeout
	}
	if token.Error() != nil {
		return fmt.Errorf("gmqtt: publish failed: %w", token.Error())
	}

	return nil
}

// Subscribe 订阅主题
func (c *Client) Subscribe(topic string, qos QoS, handler MessageHandler) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	if handler == nil {
		return ErrNoHandler
	}

	// 保存订阅信息
	c.subLock.Lock()
	c.subscriptions[topic] = &Subscription{
		Topic:   topic,
		QoS:     qos,
		Handler: handler,
	}
	c.subLock.Unlock()

	// 执行订阅
	token := c.client.Subscribe(topic, byte(qos), func(client mqtt.Client, msg mqtt.Message) {
		c.handleMessage(msg)
	})

	if token.Wait() && token.Error() != nil {
		c.subLock.Lock()
		delete(c.subscriptions, topic)
		c.subLock.Unlock()
		return fmt.Errorf("gmqtt: subscribe failed: %w", token.Error())
	}

	log.Printf("gmqtt: client [%s] subscribed to topic [%s] with QoS %d", c.options.ClientID, topic, qos)
	return nil
}

// SubscribeMultiple 批量订阅
func (c *Client) SubscribeMultiple(subscriptions map[string]QoS, handler MessageHandler) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	if handler == nil {
		return ErrNoHandler
	}

	// 转换为paho格式
	filters := make(map[string]byte)
	for topic, qos := range subscriptions {
		filters[topic] = byte(qos)

		// 保存订阅信息
		c.subLock.Lock()
		c.subscriptions[topic] = &Subscription{
			Topic:   topic,
			QoS:     qos,
			Handler: handler,
		}
		c.subLock.Unlock()
	}

	// 执行批量订阅
	token := c.client.SubscribeMultiple(filters, func(client mqtt.Client, msg mqtt.Message) {
		c.handleMessage(msg)
	})

	if token.Wait() && token.Error() != nil {
		// 清理订阅信息
		c.subLock.Lock()
		for topic := range subscriptions {
			delete(c.subscriptions, topic)
		}
		c.subLock.Unlock()
		return fmt.Errorf("gmqtt: subscribe multiple failed: %w", token.Error())
	}

	log.Printf("gmqtt: client [%s] subscribed to %d topics", c.options.ClientID, len(subscriptions))
	return nil
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(topics ...string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	token := c.client.Unsubscribe(topics...)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("gmqtt: unsubscribe failed: %w", token.Error())
	}

	// 清理订阅信息
	c.subLock.Lock()
	for _, topic := range topics {
		delete(c.subscriptions, topic)
	}
	c.subLock.Unlock()

	log.Printf("gmqtt: client [%s] unsubscribed from topics: %v", c.options.ClientID, topics)
	return nil
}

// SetDefaultHandler 设置默认消息处理器
func (c *Client) SetDefaultHandler(handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultHandler = handler
}

// handleMessage 处理接收到的消息
func (c *Client) handleMessage(msg mqtt.Message) {
	topic := msg.Topic()

	// 查找对应的处理器
	c.subLock.RLock()
	sub, exists := c.subscriptions[topic]
	c.subLock.RUnlock()

	var handler MessageHandler
	if exists && sub.Handler != nil {
		handler = sub.Handler
	} else {
		c.mu.RLock()
		handler = c.defaultHandler
		c.mu.RUnlock()
	}

	if handler != nil {
		if err := handler(topic, msg.Payload()); err != nil {
			log.Printf("gmqtt: message handler error for topic [%s]: %v", topic, err)
		}
	}
}

// GetSubscriptions 获取所有订阅
func (c *Client) GetSubscriptions() map[string]*Subscription {
	c.subLock.RLock()
	defer c.subLock.RUnlock()

	subs := make(map[string]*Subscription)
	for k, v := range c.subscriptions {
		subs[k] = v
	}
	return subs
}

// RegisterClient 注册全局客户端
func RegisterClient(name string, opts *ClientOptions) (*Client, error) {
	client, err := NewClient(opts)
	if err != nil {
		return nil, err
	}

	clientsLock.Lock()
	clients[name] = client
	clientsLock.Unlock()

	log.Printf("gmqtt: client [%s] registered", name)
	return client, nil
}

// GetClient 获取全局客户端
func GetClient(name string) (*Client, error) {
	clientsLock.RLock()
	defer clientsLock.RUnlock()

	client, exists := clients[name]
	if !exists {
		return nil, ErrClientNotFound
	}
	return client, nil
}

// CloseAll 关闭所有客户端
func CloseAll(quiesce uint) {
	clientsLock.Lock()
	defer clientsLock.Unlock()

	for name, client := range clients {
		client.Disconnect(quiesce)
		log.Printf("gmqtt: client [%s] closed", name)
	}
	clients = make(map[string]*Client)
}
