package gmqtt

import "errors"

var (
	// ErrNoBrokers 未配置Broker地址
	ErrNoBrokers = errors.New("gmqtt: no brokers configured")

	// ErrNoClientID 未配置ClientID
	ErrNoClientID = errors.New("gmqtt: no client id configured")

	// ErrNotConnected 客户端未连接
	ErrNotConnected = errors.New("gmqtt: client not connected")

	// ErrAlreadyConnected 客户端已连接
	ErrAlreadyConnected = errors.New("gmqtt: client already connected")

	// ErrNoHandler 未设置消息处理函数
	ErrNoHandler = errors.New("gmqtt: no message handler set")

	// ErrClientNotFound 客户端不存在
	ErrClientNotFound = errors.New("gmqtt: client not found")

	// ErrPublishTimeout 发布超时
	ErrPublishTimeout = errors.New("gmqtt: publish timeout")

	// ErrSubscribeTimeout 订阅超时
	ErrSubscribeTimeout = errors.New("gmqtt: subscribe timeout")

	// ErrUnsubscribeTimeout 取消订阅超时
	ErrUnsubscribeTimeout = errors.New("gmqtt: unsubscribe timeout")
)
