package gmqtt

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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
	subOrder      []string
	subLock       sync.RWMutex

	// 默认消息处理器
	defaultHandler        MessageHandler
	defaultContextHandler MessageHandlerContext

	// 中间件链
	publishHandler PublishHandler
	handlerChain   HandlerFunc
}

// NewClient 创建新的MQTT客户端
func NewClient(opts *ClientOptions) (*Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	c := &Client{
		options:       opts,
		subscriptions: make(map[string]*Subscription),
		subOrder:      make([]string, 0),
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
	c.publishHandler = chainPublishMiddlewares(c.publishFinal, opts.PublishMiddlewares...)
	c.handlerChain = chainHandlerMiddlewares(c.handleFinal, opts.HandlerMiddlewares...)
	return c, nil
}

func adaptMessageHandler(handler MessageHandler) MessageHandlerContext {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, topic string, payload []byte) error {
		return handler(topic, payload)
	}
}

func chainPublishMiddlewares(final PublishHandler, middlewares ...PublishMiddleware) PublishHandler {
	handler := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			handler = middlewares[i](handler)
		}
	}
	return handler
}

func chainHandlerMiddlewares(final HandlerFunc, middlewares ...HandlerMiddleware) HandlerFunc {
	handler := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			handler = middlewares[i](handler)
		}
	}
	return handler
}

func waitTokenContext(ctx context.Context, token mqtt.Token) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) addSubscription(topic string, qos QoS, handler MessageHandler, contextHandler ...MessageHandlerContext) {
	var handlerContext MessageHandlerContext
	if len(contextHandler) > 0 {
		handlerContext = contextHandler[0]
	}
	if handlerContext == nil {
		handlerContext = adaptMessageHandler(handler)
	}

	if _, exists := c.subscriptions[topic]; !exists {
		c.subOrder = append(c.subOrder, topic)
	}
	c.subscriptions[topic] = &Subscription{
		Topic:          topic,
		QoS:            qos,
		Handler:        handler,
		ContextHandler: handlerContext,
	}
}

func (c *Client) removeSubscription(topic string) {
	delete(c.subscriptions, topic)
	for i, filter := range c.subOrder {
		if filter == topic {
			c.subOrder = append(c.subOrder[:i], c.subOrder[i+1:]...)
			return
		}
	}
}

func buildSharedSubscriptionTopic(group, filter string) (string, error) {
	if group == "" || filter == "" || strings.ContainsAny(group, "/+#") {
		return "", ErrInvalidSharedSubscription
	}
	return "$share/" + group + "/" + filter, nil
}

func logicalSubscriptionFilter(filter string) string {
	if !strings.HasPrefix(filter, "$share/") {
		return filter
	}

	rest := strings.TrimPrefix(filter, "$share/")
	separator := strings.IndexByte(rest, '/')
	if separator < 0 || separator == len(rest)-1 {
		return filter
	}
	return rest[separator+1:]
}

func topicFilterMatches(filter, topic string) bool {
	filterLevels := strings.Split(filter, "/")
	topicLevels := strings.Split(topic, "/")

	for i, filterLevel := range filterLevels {
		if filterLevel == "#" {
			return i == len(filterLevels)-1
		}
		if i >= len(topicLevels) {
			return false
		}
		if filterLevel == "+" {
			continue
		}
		if filterLevel != topicLevels[i] {
			return false
		}
	}

	return len(filterLevels) == len(topicLevels)
}

func (c *Client) findHandler(topic string) MessageHandlerContext {
	c.subLock.RLock()
	defer c.subLock.RUnlock()

	if sub, exists := c.subscriptions[topic]; exists {
		if sub.ContextHandler != nil {
			return sub.ContextHandler
		}
		if sub.Handler != nil {
			return adaptMessageHandler(sub.Handler)
		}
	}

	for _, filter := range c.subOrder {
		if filter == topic {
			continue
		}

		sub := c.subscriptions[filter]
		if sub == nil {
			continue
		}

		if topicFilterMatches(logicalSubscriptionFilter(filter), topic) {
			if sub.ContextHandler != nil {
				return sub.ContextHandler
			}
			if sub.Handler != nil {
				return adaptMessageHandler(sub.Handler)
			}
		}
	}

	return nil
}

// Connect 连接到MQTT Broker
func (c *Client) Connect() error {
	return c.ConnectContext(context.Background())
}

// ConnectContext 连接到MQTT Broker
func (c *Client) ConnectContext(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client.IsConnected() {
		return ErrAlreadyConnected
	}

	token := c.client.Connect()
	if err := waitTokenContext(ctx, token); err != nil {
		return fmt.Errorf("gmqtt: connect failed: %w", err)
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
	return c.PublishContext(context.Background(), topic, qos, retained, payload)
}

// PublishContext 发布消息
func (c *Client) PublishContext(ctx context.Context, topic string, qos QoS, retained bool, payload interface{}) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	_, err := c.publishHandler(ctx, &PublishRequest{
		Topic:    topic,
		QoS:      qos,
		Retained: retained,
		Payload:  payload,
	})
	return err
}

func (c *Client) publishFinal(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	if req == nil {
		return nil, errors.New("gmqtt: publish request is nil")
	}

	token := c.client.Publish(req.Topic, byte(req.QoS), req.Retained, req.Payload)
	if err := waitTokenContext(ctx, token); err != nil {
		return nil, fmt.Errorf("gmqtt: publish failed: %w", err)
	}

	return &PublishResponse{}, nil
}

// PublishWithTimeout 发布消息（带超时）
func (c *Client) PublishWithTimeout(topic string, qos QoS, retained bool, payload interface{}, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := c.PublishContext(ctx, topic, qos, retained, payload); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrPublishTimeout
		}
		return err
	}
	return nil
}

// Subscribe 订阅主题
func (c *Client) Subscribe(topic string, qos QoS, handler MessageHandler) error {
	return c.SubscribeContext(context.Background(), topic, qos, adaptMessageHandler(handler))
}

// SubscribeContext 订阅主题
func (c *Client) SubscribeContext(ctx context.Context, topic string, qos QoS, handler MessageHandlerContext) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	if handler == nil {
		return ErrNoHandler
	}

	// 保存订阅信息
	c.subLock.Lock()
	c.addSubscription(topic, qos, nil, handler)
	c.subLock.Unlock()

	// 执行订阅
	token := c.client.Subscribe(topic, byte(qos), func(client mqtt.Client, msg mqtt.Message) {
		c.handleMessage(msg)
	})

	if err := waitTokenContext(ctx, token); err != nil {
		c.subLock.Lock()
		c.removeSubscription(topic)
		c.subLock.Unlock()
		return fmt.Errorf("gmqtt: subscribe failed: %w", err)
	}

	log.Printf("gmqtt: client [%s] subscribed to topic [%s] with QoS %d", c.options.ClientID, topic, qos)
	return nil
}

// SubscribeMultiple 批量订阅
func (c *Client) SubscribeMultiple(subscriptions map[string]QoS, handler MessageHandler) error {
	return c.SubscribeMultipleContext(context.Background(), subscriptions, adaptMessageHandler(handler))
}

// SubscribeMultipleContext 批量订阅
func (c *Client) SubscribeMultipleContext(ctx context.Context, subscriptions map[string]QoS, handler MessageHandlerContext) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	if handler == nil {
		return ErrNoHandler
	}

	// 转换为paho格式
	filters := make(map[string]byte)
	c.subLock.Lock()
	for topic, qos := range subscriptions {
		filters[topic] = byte(qos)
		c.addSubscription(topic, qos, nil, handler)
	}
	c.subLock.Unlock()

	// 执行批量订阅
	token := c.client.SubscribeMultiple(filters, func(client mqtt.Client, msg mqtt.Message) {
		c.handleMessage(msg)
	})

	if err := waitTokenContext(ctx, token); err != nil {
		// 清理订阅信息
		c.subLock.Lock()
		for topic := range subscriptions {
			c.removeSubscription(topic)
		}
		c.subLock.Unlock()
		return fmt.Errorf("gmqtt: subscribe multiple failed: %w", err)
	}

	log.Printf("gmqtt: client [%s] subscribed to %d topics", c.options.ClientID, len(subscriptions))
	return nil
}

// SubscribeShared 订阅共享主题
func (c *Client) SubscribeShared(group, topic string, qos QoS, handler MessageHandler) error {
	return c.SubscribeSharedContext(context.Background(), group, topic, qos, adaptMessageHandler(handler))
}

// SubscribeSharedContext 订阅共享主题
func (c *Client) SubscribeSharedContext(ctx context.Context, group, topic string, qos QoS, handler MessageHandlerContext) error {
	sharedTopic, err := buildSharedSubscriptionTopic(group, topic)
	if err != nil {
		return err
	}
	return c.SubscribeContext(ctx, sharedTopic, qos, handler)
}

// SubscribeSharedMultiple 批量订阅共享主题
func (c *Client) SubscribeSharedMultiple(group string, subscriptions map[string]QoS, handler MessageHandler) error {
	return c.SubscribeSharedMultipleContext(context.Background(), group, subscriptions, adaptMessageHandler(handler))
}

// SubscribeSharedMultipleContext 批量订阅共享主题
func (c *Client) SubscribeSharedMultipleContext(ctx context.Context, group string, subscriptions map[string]QoS, handler MessageHandlerContext) error {
	sharedSubscriptions := make(map[string]QoS)
	for topic, qos := range subscriptions {
		sharedTopic, err := buildSharedSubscriptionTopic(group, topic)
		if err != nil {
			return err
		}
		sharedSubscriptions[sharedTopic] = qos
	}
	return c.SubscribeMultipleContext(ctx, sharedSubscriptions, handler)
}

// UnsubscribeShared 取消共享订阅
func (c *Client) UnsubscribeShared(group string, topics ...string) error {
	return c.UnsubscribeSharedContext(context.Background(), group, topics...)
}

// UnsubscribeSharedContext 取消共享订阅
func (c *Client) UnsubscribeSharedContext(ctx context.Context, group string, topics ...string) error {
	sharedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		sharedTopic, err := buildSharedSubscriptionTopic(group, topic)
		if err != nil {
			return err
		}
		sharedTopics = append(sharedTopics, sharedTopic)
	}
	return c.UnsubscribeContext(ctx, sharedTopics...)
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(topics ...string) error {
	return c.UnsubscribeContext(context.Background(), topics...)
}

// UnsubscribeContext 取消订阅
func (c *Client) UnsubscribeContext(ctx context.Context, topics ...string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	token := c.client.Unsubscribe(topics...)
	if err := waitTokenContext(ctx, token); err != nil {
		return fmt.Errorf("gmqtt: unsubscribe failed: %w", err)
	}

	// 清理订阅信息
	c.subLock.Lock()
	for _, topic := range topics {
		c.removeSubscription(topic)
	}
	c.subLock.Unlock()

	log.Printf("gmqtt: client [%s] unsubscribed from topics: %v", c.options.ClientID, topics)
	return nil
}

// SetDefaultHandler 设置默认消息处理器
func (c *Client) SetDefaultHandler(handler MessageHandler) {
	c.SetDefaultHandlerContext(adaptMessageHandler(handler))
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultHandler = handler
}

// SetDefaultHandlerContext 设置默认消息处理器
func (c *Client) SetDefaultHandlerContext(handler MessageHandlerContext) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultContextHandler = handler
	c.defaultHandler = nil
}

func (c *Client) handleFinal(ctx context.Context, req *HandlerRequest) (*HandlerResponse, error) {
	if req == nil || req.Message == nil {
		return nil, errors.New("gmqtt: handler request is nil")
	}

	handler := c.findHandler(req.Message.Topic)
	if handler == nil {
		c.mu.RLock()
		handler = c.defaultContextHandler
		if handler == nil && c.defaultHandler != nil {
			handler = adaptMessageHandler(c.defaultHandler)
		}
		c.mu.RUnlock()
	}
	if handler == nil {
		return &HandlerResponse{}, nil
	}

	if err := handler(ctx, req.Message.Topic, req.Message.Payload); err != nil {
		return nil, err
	}
	return &HandlerResponse{}, nil
}

// handleMessage 处理接收到的消息
func (c *Client) handleMessage(msg mqtt.Message) {
	message := FromMQTTMessage(msg)
	_, err := c.handlerChain(context.Background(), &HandlerRequest{
		Message: message,
	})
	if err != nil {
		log.Printf("gmqtt: message handler error for topic [%s]: %v", message.Topic, err)
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
