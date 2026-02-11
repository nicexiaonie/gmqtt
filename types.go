package gmqtt

import (
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
	Topic   string
	QoS     QoS
	Handler MessageHandler
}
