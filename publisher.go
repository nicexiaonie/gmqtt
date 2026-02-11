package gmqtt

import (
	"encoding/json"
	"fmt"
	"time"
)

// Publisher MQTT发布者
type Publisher struct {
	client *Client
	topic  string
	qos    QoS
}

// NewPublisher 创建发布者
func NewPublisher(client *Client, topic string, qos QoS) *Publisher {
	return &Publisher{
		client: client,
		topic:  topic,
		qos:    qos,
	}
}

// Publish 发布消息
func (p *Publisher) Publish(payload interface{}) error {
	return p.client.Publish(p.topic, p.qos, false, payload)
}

// PublishRetained 发布保留消息
func (p *Publisher) PublishRetained(payload interface{}) error {
	return p.client.Publish(p.topic, p.qos, true, payload)
}

// PublishWithTimeout 发布消息（带超时）
func (p *Publisher) PublishWithTimeout(payload interface{}, timeout time.Duration) error {
	return p.client.PublishWithTimeout(p.topic, p.qos, false, payload, timeout)
}

// PublishJSON 发布JSON消息
func (p *Publisher) PublishJSON(data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("gmqtt: marshal json failed: %w", err)
	}
	return p.Publish(payload)
}

// PublishJSONRetained 发布JSON保留消息
func (p *Publisher) PublishJSONRetained(data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("gmqtt: marshal json failed: %w", err)
	}
	return p.PublishRetained(payload)
}

// PublishString 发布字符串消息
func (p *Publisher) PublishString(message string) error {
	return p.Publish(message)
}

// PublishBytes 发布字节消息
func (p *Publisher) PublishBytes(data []byte) error {
	return p.Publish(data)
}

// SetTopic 设置主题
func (p *Publisher) SetTopic(topic string) {
	p.topic = topic
}

// SetQoS 设置QoS
func (p *Publisher) SetQoS(qos QoS) {
	p.qos = qos
}

// GetTopic 获取主题
func (p *Publisher) GetTopic() string {
	return p.topic
}

// GetQoS 获取QoS
func (p *Publisher) GetQoS() QoS {
	return p.qos
}
