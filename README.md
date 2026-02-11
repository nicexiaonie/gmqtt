# gmqtt - MQTT消息队列封装

基于 `github.com/eclipse/paho.mqtt.golang` 的高性能、稳定、易用的 MQTT 客户端封装库。

## 特性

- ✅ **全面功能** - 支持 MQTT 3.1.1 协议的所有特性
- ✅ **高性能** - 基于 paho.mqtt.golang，经过生产环境验证
- ✅ **稳定可靠** - 支持自动重连、持久化会话、QoS 0/1/2
- ✅ **现代化设计** - 简洁的 API，支持链式调用
- ✅ **便捷使用** - 提供 Publisher/Subscriber 模式，支持 JSON 序列化
- ✅ **安全连接** - 支持 TLS/SSL 加密连接
- ✅ **灵活配置** - 丰富的配置选项，满足各种场景需求
- ✅ **全局管理** - 支持多客户端注册和管理
- ✅ **遗嘱消息** - 支持 Last Will and Testament
- ✅ **通配符订阅** - 支持 `+` 和 `#` 通配符

## 安装

```bash
go get github.com/nicexiaonie/gmqtt
```

## 快速开始

### 基础示例

```go
package main

import (
    "log"
    "time"
    "github.com/nicexiaonie/gmqtt"
)

func main() {
    // 创建客户端配置
    opts := gmqtt.NewDefaultOptions()
    opts.Brokers = []string{"tcp://localhost:1883"}
    opts.ClientID = "my-client"
    opts.Username = "admin"
    opts.Password = "password"

    // 创建客户端
    client, err := gmqtt.NewClient(opts)
    if err != nil {
        log.Fatal(err)
    }

    // 连接到 Broker
    if err := client.Connect(); err != nil {
        log.Fatal(err)
    }
    defer client.Disconnect(250)

    // 订阅主题
    err = client.Subscribe("test/topic", gmqtt.QoS1, func(topic string, payload []byte) error {
        log.Printf("收到消息: %s", string(payload))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }

    // 发布消息
    err = client.Publish("test/topic", gmqtt.QoS1, false, "Hello MQTT!")
    if err != nil {
        log.Fatal(err)
    }

    time.Sleep(2 * time.Second)
}
```

## 核心概念

### QoS 服务质量等级

```go
gmqtt.QoS0  // 最多一次，消息可能丢失
gmqtt.QoS1  // 至少一次，消息可能重复
gmqtt.QoS2  // 只有一次，消息不会丢失也不会重复
```

### 客户端配置

```go
opts := gmqtt.NewDefaultOptions()

// Broker 配置
opts.Brokers = []string{"tcp://localhost:1883"}  // 支持多个 broker
opts.ClientID = "unique-client-id"

// 认证配置
opts.Username = "admin"
opts.Password = "password"

// 会话配置
opts.CleanSession = true                         // 是否清除会话
opts.KeepAlive = 60 * time.Second               // 心跳间隔

// 重连配置
opts.AutoReconnect = true                        // 自动重连
opts.MaxReconnectInterval = 10 * time.Minute    // 最大重连间隔

// 超时配置
opts.ConnectTimeout = 30 * time.Second          // 连接超时
opts.WriteTimeout = 30 * time.Second            // 写超时

// 回调配置
opts.OnConnect = func() {
    log.Println("已连接")
}
opts.OnConnectionLost = func(err error) {
    log.Printf("连接丢失: %v", err)
}
```

## 使用方式

### 1. 直接使用客户端

```go
client, _ := gmqtt.NewClient(opts)
client.Connect()

// 发布消息
client.Publish("topic", gmqtt.QoS1, false, "message")

// 订阅消息
client.Subscribe("topic", gmqtt.QoS1, func(topic string, payload []byte) error {
    // 处理消息
    return nil
})
```

### 2. 使用 Publisher 模式

```go
publisher := gmqtt.NewPublisher(client, "sensor/data", gmqtt.QoS1)

// 发布字符串
publisher.PublishString("hello")

// 发布字节
publisher.PublishBytes([]byte("data"))

// 发布 JSON
type Data struct {
    Value float64 `json:"value"`
}
publisher.PublishJSON(Data{Value: 25.5})

// 发布保留消息
publisher.PublishRetained("retained message")

// 带超时发布
publisher.PublishWithTimeout("message", 5*time.Second)
```

### 3. 使用 Subscriber 模式

```go
subscriber := gmqtt.NewSubscriber(client, []string{"sensor/+"}, gmqtt.QoS1)

// 字符串处理器
subscriber.SetStringHandler(func(topic string, message string) error {
    log.Printf("%s: %s", topic, message)
    return nil
})

// JSON 处理器
type SensorData struct {
    Temperature float64 `json:"temperature"`
}
var data SensorData
subscriber.SetJSONHandler(func(topic string, d interface{}) error {
    sensorData := d.(*SensorData)
    log.Printf("温度: %.2f", sensorData.Temperature)
    return nil
}, &data)

// 开始订阅
subscriber.Subscribe()
```

### 4. 使用订阅者组

```go
group := gmqtt.NewSubscriberGroup()

// 添加多个订阅者
sub1 := gmqtt.NewSubscriber(client, []string{"topic1"}, gmqtt.QoS1)
sub1.SetStringHandler(handler1)
group.Add(sub1)

sub2 := gmqtt.NewSubscriber(client, []string{"topic2"}, gmqtt.QoS1)
sub2.SetStringHandler(handler2)
group.Add(sub2)

// 批量订阅
group.SubscribeAll()

// 批量取消订阅
group.UnsubscribeAll()
```

### 5. 全局客户端管理

```go
// 注册全局客户端
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://localhost:1883"}
opts.ClientID = "global-client"

client, _ := gmqtt.RegisterClient("default", opts)
client.Connect()

// 在任何地方获取客户端
client, _ := gmqtt.GetClient("default")

// 关闭所有客户端
gmqtt.CloseAll(250)
```

## 高级特性

### 遗嘱消息 (Last Will)

当客户端异常断开时，Broker 会自动发送遗嘱消息。

```go
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://localhost:1883"}
opts.ClientID = "device-001"

// 配置遗嘱消息
opts.WillEnabled = true
opts.WillTopic = "device/status"
opts.WillPayload = []byte(`{"status":"offline"}`)
opts.WillQoS = gmqtt.QoS1
opts.WillRetained = true

client, _ := gmqtt.NewClient(opts)
client.Connect()

// 发布在线状态
client.Publish("device/status", gmqtt.QoS1, true, `{"status":"online"}`)

// 当客户端异常断开时，Broker 会自动发送遗嘱消息
```

### 持久化会话

保持会话状态，重连后恢复订阅和未确认的消息。

```go
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://localhost:1883"}
opts.ClientID = "persistent-client"
opts.CleanSession = false  // 保持会话

client, _ := gmqtt.NewClient(opts)
client.Connect()

// 订阅主题
client.Subscribe("topic", gmqtt.QoS1, handler)

// 断开连接
client.Disconnect(250)

// 重新连接后，订阅会自动恢复，离线期间的消息也会收到
client.Connect()
```

### TLS/SSL 加密连接

```go
import "crypto/tls"

tlsConfig := &tls.Config{
    InsecureSkipVerify: false,
}

// 加载客户端证书
cert, _ := tls.LoadX509KeyPair("client-cert.pem", "client-key.pem")
tlsConfig.Certificates = []tls.Certificate{cert}

// 加载 CA 证书
caCert, _ := ioutil.ReadFile("ca-cert.pem")
caCertPool := x509.NewCertPool()
caCertPool.AppendCertsFromPEM(caCert)
tlsConfig.RootCAs = caCertPool

opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"ssl://localhost:8883"}
opts.ClientID = "tls-client"
opts.TLSConfig = tlsConfig

client, _ := gmqtt.NewClient(opts)
client.Connect()
```

### 通配符订阅

```go
// 单级通配符 +
client.Subscribe("sensor/+/temperature", gmqtt.QoS1, handler)
// 匹配: sensor/001/temperature, sensor/002/temperature

// 多级通配符 #
client.Subscribe("device/#", gmqtt.QoS1, handler)
// 匹配: device/status, device/sensor/temperature, device/a/b/c

// 批量订阅
subscriptions := map[string]gmqtt.QoS{
    "device/+/status":      gmqtt.QoS1,
    "sensor/+/temperature": gmqtt.QoS1,
    "system/#":             gmqtt.QoS2,
}
client.SubscribeMultiple(subscriptions, handler)
```

### 自动重连

```go
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://localhost:1883"}
opts.ClientID = "auto-reconnect-client"
opts.AutoReconnect = true
opts.MaxReconnectInterval = 10 * time.Second

opts.OnConnect = func() {
    log.Println("已连接")
}

opts.OnConnectionLost = func(err error) {
    log.Printf("连接丢失: %v (将自动重连)", err)
}

client, _ := gmqtt.NewClient(opts)
client.Connect()

// 当连接丢失时，客户端会自动尝试重连
```

## API 参考

### Client

```go
// 连接管理
Connect() error
Disconnect(quiesce uint)
IsConnected() bool

// 发布消息
Publish(topic string, qos QoS, retained bool, payload interface{}) error
PublishWithTimeout(topic string, qos QoS, retained bool, payload interface{}, timeout time.Duration) error

// 订阅管理
Subscribe(topic string, qos QoS, handler MessageHandler) error
SubscribeMultiple(subscriptions map[string]QoS, handler MessageHandler) error
Unsubscribe(topics ...string) error

// 其他
SetDefaultHandler(handler MessageHandler)
GetSubscriptions() map[string]*Subscription
```

### Publisher

```go
Publish(payload interface{}) error
PublishRetained(payload interface{}) error
PublishWithTimeout(payload interface{}, timeout time.Duration) error
PublishJSON(data interface{}) error
PublishJSONRetained(data interface{}) error
PublishString(message string) error
PublishBytes(data []byte) error
SetTopic(topic string)
SetQoS(qos QoS)
```

### Subscriber

```go
Subscribe() error
Unsubscribe() error
SetHandler(handler MessageHandler)
SetJSONHandler(handler func(topic string, data interface{}) error, dataType interface{})
SetStringHandler(handler func(topic string, message string) error)
AddTopic(topic string)
RemoveTopic(topic string)
SetQoS(qos QoS)
```

### SubscriberGroup

```go
Add(subscriber *Subscriber)
SubscribeAll() error
UnsubscribeAll() error
Count() int
```

## 错误处理

```go
var (
    ErrNoBrokers           // 未配置 Broker 地址
    ErrNoClientID          // 未配置 ClientID
    ErrNotConnected        // 客户端未连接
    ErrAlreadyConnected    // 客户端已连接
    ErrNoHandler           // 未设置消息处理函数
    ErrClientNotFound      // 客户端不存在
    ErrPublishTimeout      // 发布超时
    ErrSubscribeTimeout    // 订阅超时
    ErrUnsubscribeTimeout  // 取消订阅超时
)
```

## 最佳实践

### 1. 选择合适的 QoS

- **QoS 0**: 适用于可以容忍消息丢失的场景，如传感器数据上报
- **QoS 1**: 适用于需要确保消息送达的场景，如日志上报
- **QoS 2**: 适用于不能容忍消息重复的场景，如支付通知

### 2. 使用持久化会话

对于重要的订阅，建议使用持久化会话（CleanSession=false），确保离线期间的消息不会丢失。

### 3. 设置合理的超时时间

```go
opts.ConnectTimeout = 30 * time.Second
opts.WriteTimeout = 30 * time.Second
```

### 4. 使用遗嘱消息

对于需要监控设备在线状态的场景，建议配置遗嘱消息。

### 5. 错误处理

```go
err := client.Publish(topic, qos, false, payload)
if err != nil {
    if err == gmqtt.ErrNotConnected {
        // 处理未连接错误
    } else {
        // 处理其他错误
    }
}
```

### 6. 资源清理

```go
defer client.Disconnect(250)  // 250ms 等待时间
```

## 性能优化

### 1. 批量操作

使用 `SubscribeMultiple` 批量订阅，减少网络往返。

```go
subscriptions := map[string]gmqtt.QoS{
    "topic1": gmqtt.QoS1,
    "topic2": gmqtt.QoS1,
    "topic3": gmqtt.QoS1,
}
client.SubscribeMultiple(subscriptions, handler)
```

### 2. 调整消息通道深度

```go
opts.MessageChannelDepth = 1000  // 默认 100
```

### 3. 使用 QoS 0

在可以容忍消息丢失的场景下，使用 QoS 0 可以获得最佳性能。

### 4. 连接池

对于高并发场景，可以创建多个客户端实例。

```go
for i := 0; i < 10; i++ {
    opts := gmqtt.NewDefaultOptions()
    opts.ClientID = fmt.Sprintf("client-%d", i)
    client, _ := gmqtt.RegisterClient(opts.ClientID, opts)
    client.Connect()
}
```

## 示例代码

完整示例代码请参考：
- [基础示例](examples/basic/main.go)
- [高级特性示例](examples/advanced/main.go)

## 常见问题

### Q: 如何处理大量消息？

A: 可以调整 `MessageChannelDepth` 参数，或者使用多个客户端实例。

### Q: 如何确保消息不丢失？

A: 使用 QoS 1 或 QoS 2，并启用持久化会话（CleanSession=false）。

### Q: 如何监控客户端状态？

A: 使用 `OnConnect` 和 `OnConnectionLost` 回调函数。

### Q: 支持 WebSocket 吗？

A: 支持，Broker 地址使用 `ws://` 或 `wss://` 协议。

```go
opts.Brokers = []string{"ws://localhost:8080/mqtt"}
```

## 依赖

- [github.com/eclipse/paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) v1.4.3

## 许可证

本项目采用与主项目相同的许可证。

## 贡献

欢迎提交 Issue 和 Pull Request。

## 更新日志

### v1.0.0 (2026-02-10)

- 初始版本发布
- 支持 MQTT 3.1.1 协议
- 支持 QoS 0/1/2
- 支持 TLS/SSL
- 支持遗嘱消息
- 支持持久化会话
- 支持自动重连
- 提供 Publisher/Subscriber 模式
- 支持 JSON 序列化
- 支持全局客户端管理
