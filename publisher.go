package gmqtt

import (
	"context"
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
	return p.PublishContext(context.Background(), payload)
}

// PublishContext 发布消息
func (p *Publisher) PublishContext(ctx context.Context, payload interface{}) error {
	return p.client.PublishContext(ctx, p.topic, p.qos, false, payload)
}

// PublishRetained 发布保留消息
func (p *Publisher) PublishRetained(payload interface{}) error {
	return p.PublishRetainedContext(context.Background(), payload)
}

// PublishRetainedContext 发布保留消息
func (p *Publisher) PublishRetainedContext(ctx context.Context, payload interface{}) error {
	return p.client.PublishContext(ctx, p.topic, p.qos, true, payload)
}

// PublishWithTimeout 发布消息（带超时）
func (p *Publisher) PublishWithTimeout(payload interface{}, timeout time.Duration) error {
	return p.client.PublishWithTimeout(p.topic, p.qos, false, payload, timeout)
}

// PublishJSON 发布JSON消息
func (p *Publisher) PublishJSON(data interface{}) error {
	return p.PublishJSONContext(context.Background(), data)
}

// PublishJSONContext 发布JSON消息
func (p *Publisher) PublishJSONContext(ctx context.Context, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("gmqtt: marshal json failed: %w", err)
	}
	return p.PublishContext(ctx, payload)
}

// PublishJSONRetained 发布JSON保留消息
func (p *Publisher) PublishJSONRetained(data interface{}) error {
	return p.PublishJSONRetainedContext(context.Background(), data)
}

// PublishJSONRetainedContext 发布JSON保留消息
func (p *Publisher) PublishJSONRetainedContext(ctx context.Context, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("gmqtt: marshal json failed: %w", err)
	}
	return p.PublishRetainedContext(ctx, payload)
}

// PublishString 发布字符串消息
func (p *Publisher) PublishString(message string) error {
	return p.PublishStringContext(context.Background(), message)
}

// PublishStringContext 发布字符串消息
func (p *Publisher) PublishStringContext(ctx context.Context, message string) error {
	return p.PublishContext(ctx, message)
}

// PublishBytes 发布字节消息
func (p *Publisher) PublishBytes(data []byte) error {
	return p.PublishBytesContext(context.Background(), data)
}

// PublishBytesContext 发布字节消息
func (p *Publisher) PublishBytesContext(ctx context.Context, data []byte) error {
	return p.PublishContext(ctx, data)
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
