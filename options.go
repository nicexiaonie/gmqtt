package gmqtt

import (
	"crypto/tls"
	"time"
)

// ClientOptions MQTT客户端配置选项
type ClientOptions struct {
	// Broker 地址列表，支持多个broker进行故障转移
	// 格式: tcp://host:port 或 ssl://host:port 或 ws://host:port
	Brokers []string

	// ClientID 客户端唯一标识
	ClientID string

	// Username 用户名（可选）
	Username string

	// Password 密码（可选）
	Password string

	// CleanSession 是否清除会话
	// true: 连接断开后清除会话，不保留订阅和未确认的消息
	// false: 保持会话，重连后恢复订阅和未确认的消息
	CleanSession bool

	// KeepAlive 心跳间隔（秒）
	// 默认: 60秒
	KeepAlive time.Duration

	// ConnectTimeout 连接超时时间
	// 默认: 30秒
	ConnectTimeout time.Duration

	// AutoReconnect 是否自动重连
	// 默认: true
	AutoReconnect bool

	// MaxReconnectInterval 最大重连间隔
	// 默认: 10分钟
	MaxReconnectInterval time.Duration

	// ConnectRetry 是否在首次连接失败时重试
	// 默认: true
	ConnectRetry bool

	// ConnectRetryInterval 首次连接重试间隔
	// 默认: 1秒
	ConnectRetryInterval time.Duration

	// TLSConfig TLS配置（用于SSL连接）
	TLSConfig *tls.Config

	// WillEnabled 是否启用遗嘱消息
	WillEnabled bool

	// WillTopic 遗嘱消息主题
	WillTopic string

	// WillPayload 遗嘱消息内容
	WillPayload []byte

	// WillQoS 遗嘱消息QoS
	WillQoS QoS

	// WillRetained 遗嘱消息是否保留
	WillRetained bool

	// MessageChannelDepth 消息通道深度
	// 默认: 100
	MessageChannelDepth uint

	// OnConnect 连接成功回调
	OnConnect OnConnectHandler

	// OnConnectionLost 连接丢失回调
	OnConnectionLost ConnectionLostHandler

	// WriteTimeout 写超时
	// 默认: 30秒
	WriteTimeout time.Duration

	// ResumeSubs 重连时是否自动恢复订阅
	// 默认: true
	ResumeSubs bool

	// OrderMatters 是否保证消息顺序
	// 默认: true
	OrderMatters bool
}

// NewDefaultOptions 创建默认配置
func NewDefaultOptions() *ClientOptions {
	return &ClientOptions{
		CleanSession:         true,
		KeepAlive:            60 * time.Second,
		ConnectTimeout:       30 * time.Second,
		AutoReconnect:        true,
		MaxReconnectInterval: 10 * time.Minute,
		ConnectRetry:         true,
		ConnectRetryInterval: 1 * time.Second,
		MessageChannelDepth:  100,
		WriteTimeout:         30 * time.Second,
		ResumeSubs:           true,
		OrderMatters:         true,
	}
}

// Validate 验证配置
func (o *ClientOptions) Validate() error {
	if len(o.Brokers) == 0 {
		return ErrNoBrokers
	}
	if o.ClientID == "" {
		return ErrNoClientID
	}
	return nil
}
