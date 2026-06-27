package gmqtt

import (
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// QoS 服务质量等级
type QoS byte

const (
	// QoS0 最多一次，消息可能丢失
	QoS0 QoS = 0
	// QoS1 至少一次，消息可能重复
	QoS1 QoS = 1
	// QoS2 只有一次，消息不会丢失也不会重复
	QoS2 QoS = 2
)

// MessageHandler 消息处理函数
type MessageHandler func(topic string, payload []byte) error

// MessageHandlerContext 带上下文的消息处理函数
type MessageHandlerContext func(ctx context.Context, topic string, payload []byte) error

// PublishRequest 发布请求
type PublishRequest struct {
	Topic    string
	QoS      QoS
	Retained bool
	Payload  interface{}
	Metadata map[string]string
}

// PublishResponse 发布响应
type PublishResponse struct{}

// PublishHandler 发布处理函数
type PublishHandler func(ctx context.Context, req *PublishRequest) (*PublishResponse, error)

// PublishMiddleware 发布中间件
type PublishMiddleware func(next PublishHandler) PublishHandler

// HandlerRequest 消息处理请求
type HandlerRequest struct {
	Message  *Message
	Metadata map[string]string
}

// HandlerResponse 消息处理响应
type HandlerResponse struct{}

// HandlerFunc 消息处理中间件处理函数
type HandlerFunc func(ctx context.Context, req *HandlerRequest) (*HandlerResponse, error)

// HandlerMiddleware 消息处理中间件
type HandlerMiddleware func(next HandlerFunc) HandlerFunc

// ConnectionLostHandler 连接丢失处理函数
type ConnectionLostHandler func(err error)

// OnConnectHandler 连接成功处理函数
type OnConnectHandler func()

// Message MQTT消息结构
type Message struct {
	Topic     string
	Payload   []byte
	QoS       QoS
	Retained  bool
	MessageID uint16
	Duplicate bool
}

// FromMQTTMessage 从 paho.mqtt.Message 转换
func FromMQTTMessage(msg mqtt.Message) *Message {
	return &Message{
		Topic:     msg.Topic(),
		Payload:   msg.Payload(),
		QoS:       QoS(msg.Qos()),
		Retained:  msg.Retained(),
		MessageID: msg.MessageID(),
		Duplicate: msg.Duplicate(),
	}
}

// Subscription 订阅信息
type Subscription struct {
	Topic          string
	QoS            QoS
	Handler        MessageHandler
	ContextHandler MessageHandlerContext
}
