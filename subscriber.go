package gmqtt

import (
	"encoding/json"
	"fmt"
	"log"
)

// Subscriber MQTT订阅者
type Subscriber struct {
	client  *Client
	topics  []string
	qos     QoS
	handler MessageHandler
}

// NewSubscriber 创建订阅者
func NewSubscriber(client *Client, topics []string, qos QoS) *Subscriber {
	return &Subscriber{
		client: client,
		topics: topics,
		qos:    qos,
	}
}

// SetHandler 设置消息处理器
func (s *Subscriber) SetHandler(handler MessageHandler) {
	s.handler = handler
}

// SetJSONHandler 设置JSON消息处理器
func (s *Subscriber) SetJSONHandler(handler func(topic string, data interface{}) error, dataType interface{}) {
	s.handler = func(topic string, payload []byte) error {
		if err := json.Unmarshal(payload, dataType); err != nil {
			return fmt.Errorf("gmqtt: unmarshal json failed: %w", err)
		}
		return handler(topic, dataType)
	}
}

// SetStringHandler 设置字符串消息处理器
func (s *Subscriber) SetStringHandler(handler func(topic string, message string) error) {
	s.handler = func(topic string, payload []byte) error {
		return handler(topic, string(payload))
	}
}

// Subscribe 开始订阅
func (s *Subscriber) Subscribe() error {
	if s.handler == nil {
		return ErrNoHandler
	}

	if len(s.topics) == 1 {
		return s.client.Subscribe(s.topics[0], s.qos, s.handler)
	}

	// 批量订阅
	subscriptions := make(map[string]QoS)
	for _, topic := range s.topics {
		subscriptions[topic] = s.qos
	}
	return s.client.SubscribeMultiple(subscriptions, s.handler)
}

// Unsubscribe 取消订阅
func (s *Subscriber) Unsubscribe() error {
	return s.client.Unsubscribe(s.topics...)
}

// AddTopic 添加订阅主题
func (s *Subscriber) AddTopic(topic string) {
	s.topics = append(s.topics, topic)
}

// RemoveTopic 移除订阅主题
func (s *Subscriber) RemoveTopic(topic string) {
	for i, t := range s.topics {
		if t == topic {
			s.topics = append(s.topics[:i], s.topics[i+1:]...)
			break
		}
	}
}

// GetTopics 获取订阅主题列表
func (s *Subscriber) GetTopics() []string {
	return s.topics
}

// SetQoS 设置QoS
func (s *Subscriber) SetQoS(qos QoS) {
	s.qos = qos
}

// GetQoS 获取QoS
func (s *Subscriber) GetQoS() QoS {
	return s.qos
}

// SubscriberGroup 订阅者组（用于管理多个订阅者）
type SubscriberGroup struct {
	subscribers []*Subscriber
}

// NewSubscriberGroup 创建订阅者组
func NewSubscriberGroup() *SubscriberGroup {
	return &SubscriberGroup{
		subscribers: make([]*Subscriber, 0),
	}
}

// Add 添加订阅者
func (sg *SubscriberGroup) Add(subscriber *Subscriber) {
	sg.subscribers = append(sg.subscribers, subscriber)
}

// SubscribeAll 订阅所有
func (sg *SubscriberGroup) SubscribeAll() error {
	for _, sub := range sg.subscribers {
		if err := sub.Subscribe(); err != nil {
			log.Printf("gmqtt: subscribe failed for topics %v: %v", sub.topics, err)
			return err
		}
	}
	return nil
}

// UnsubscribeAll 取消所有订阅
func (sg *SubscriberGroup) UnsubscribeAll() error {
	var lastErr error
	for _, sub := range sg.subscribers {
		if err := sub.Unsubscribe(); err != nil {
			log.Printf("gmqtt: unsubscribe failed for topics %v: %v", sub.topics, err)
			lastErr = err
		}
	}
	return lastErr
}

// Count 获取订阅者数量
func (sg *SubscriberGroup) Count() int {
	return len(sg.subscribers)
}
