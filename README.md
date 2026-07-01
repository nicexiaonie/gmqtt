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
- ✅ **共享订阅** - 支持 MQTT shared subscription 消费组
- ✅ **Context API** - 支持 `context.Context` 取消、超时和调用链上下文传递
- ✅ **Middleware 扩展** - 支持发布和消息处理链路的中间件扩展
- ✅ **OpenTelemetry Trace** - 提供独立 `otelgmqtt` Go module，支持 producer/consumer span、跨 MQTT 的 trace context 传播，以及父子/span link 两种关联模型

## 安装

核心模块：

```bash
go get github.com/nicexiaonie/gmqtt
```

OpenTelemetry trace 模块按需单独安装：

```bash
go get github.com/nicexiaonie/gmqtt/otelgmqtt
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

### Context API

所有核心操作都提供 context 版本，适合在 HTTP 请求、任务调度和微服务调用链中传递取消信号、deadline 和 trace context。

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// 带 context 连接
if err := client.ConnectContext(ctx); err != nil {
    log.Fatal(err)
}

// 带 context 发布
if err := client.PublishContext(ctx, "topic", gmqtt.QoS1, false, "message"); err != nil {
    log.Fatal(err)
}

// 带 context 订阅，handler 也能拿到 ctx
err := client.SubscribeContext(ctx, "topic", gmqtt.QoS1, func(ctx context.Context, topic string, payload []byte) error {
    log.Printf("收到消息: %s", string(payload))
    return nil
})
if err != nil {
    log.Fatal(err)
}

// 带 context 取消订阅
if err := client.UnsubscribeContext(ctx, "topic"); err != nil {
    log.Fatal(err)
}
```

旧 API 仍然保留，例如 `Publish`、`Subscribe`、`Connect` 会继续工作；`PublishWithTimeout` 仍然在超时时返回 `ErrPublishTimeout`。

### Middleware

gmqtt 支持两类中间件：

- `PublishMiddleware`：包裹发布流程，适合做 trace、日志、指标、topic 校验、payload 包装等。
- `HandlerMiddleware`：包裹消息处理流程，适合做 trace、日志、指标、panic recover、payload 解包等。

```go
opts.UsePublishMiddleware(func(next gmqtt.PublishHandler) gmqtt.PublishHandler {
    return func(ctx context.Context, req *gmqtt.PublishRequest) (*gmqtt.PublishResponse, error) {
        start := time.Now()
        res, err := next(ctx, req)
        log.Printf("publish topic=%s cost=%s err=%v", req.Topic, time.Since(start), err)
        return res, err
    }
})

opts.UseHandlerMiddleware(func(next gmqtt.HandlerFunc) gmqtt.HandlerFunc {
    return func(ctx context.Context, req *gmqtt.HandlerRequest) (*gmqtt.HandlerResponse, error) {
        start := time.Now()
        res, err := next(ctx, req)
        log.Printf("handle topic=%s cost=%s err=%v", req.Message.Topic, time.Since(start), err)
        return res, err
    }
})
```

中间件按注册顺序从外到内执行：`A before -> B before -> final -> B after -> A after`。

### OpenTelemetry Trace

OpenTelemetry 支持位于独立 Go module `github.com/nicexiaonie/gmqtt/otelgmqtt`，核心 `gmqtt` module 不依赖任何 OTel API。它通过 `PublishMiddleware` 和 `HandlerMiddleware` 接入，提供两个层级的能力：

1. **Span 采集**（默认）：为每次发布创建 producer span、每次消息处理创建 consumer span，并附带 messaging 语义属性。**不修改 MQTT payload。**
2. **跨 MQTT 传播**（opt-in）：通过 JSON envelope 将 W3C trace context 随消息一起传输，使消费端 span 能与生产端 span 关联成同一条链路。

#### 仅采集 span（不改 payload）

```go
import (
    "github.com/nicexiaonie/gmqtt"
    "github.com/nicexiaonie/gmqtt/otelgmqtt"
)

opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://localhost:1883"}
opts.ClientID = "trace-client"

opts.UsePublishMiddleware(otelgmqtt.PublishMiddleware(
    otelgmqtt.WithClientID(opts.ClientID),
))
opts.UseHandlerMiddleware(otelgmqtt.HandlerMiddleware(
    otelgmqtt.WithClientID(opts.ClientID),
))
```

此模式下生产端与消费端 span 落在**各自独立的 trace** 中，不会跨进程关联。

#### 跨 MQTT 传播 trace context

要让消费端 span 接续生产端，需在收发两端都开启 JSON envelope：

```go
opts.UsePublishMiddleware(otelgmqtt.PublishMiddleware(
    otelgmqtt.WithEnvelopeMode(otelgmqtt.EnvelopeModeJSON),
))
opts.UseHandlerMiddleware(otelgmqtt.HandlerMiddleware(
    otelgmqtt.WithEnvelopeMode(otelgmqtt.EnvelopeModeJSON),
    otelgmqtt.WithUnwrapPayload(true), // 默认即为 true
))
```

开启后，发布端把业务 payload 包进如下 envelope，并注入 W3C `traceparent`：

```json
{
  "_gmqtt_envelope": "v1",
  "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
  "tracestate": "",
  "baggage": "",
  "content_type": "application/json",
  "payload_encoding": "string",
  "payload": "{\"value\":1}"
}
```

字段说明：

| 字段 | 含义 |
| --- | --- |
| `_gmqtt_envelope` | envelope 版本标记，当前为 `v1`，用于识别和兼容演进 |
| `traceparent` / `tracestate` / `baggage` | W3C trace context，由 propagator 注入 |
| `payload_encoding` | 原始 payload 的编码方式：合法 UTF-8（含 JSON）使用 `string`，二进制使用 `base64` |
| `content_type` | 信息性字段，供人工和非 Go 消费端识别原始类型；**解码不依赖它**，仅按 `payload_encoding` 还原 |
| `payload` | 编码后的原始 payload。`string` 编码保证字节级一致，无 base64 膨胀 |

消费端开启 `WithUnwrapPayload(true)`（默认）时，中间件会在调用业务 handler **之前**把 payload 还原为原始字节，业务代码对 envelope 完全无感知。

> **互操作约定**：MQTT 3.1.1 没有标准 message header，跨 MQTT 传播 trace context 只能改变 payload 格式，因此 envelope 是 opt-in 的。一旦开启，所有消费端都隐式接受同一契约——使用本库且开启 envelope 的消费端由中间件自动剥壳；其它语言的服务、调试工具（如 MQTTX）或未开启 envelope 的消费端，会直接看到 envelope JSON，需自行按上表解析。若链路中存在异构消费端，请权衡后再开启。

#### Span 关联模型：父子 vs span link

默认情况下，消费端 span 以生产端 span 为**父节点**（同一 TraceID），这是兼容性最好、对 1:1 即时消费最直观的形式。

但在**共享订阅 fan-out**和**保留消息（retained）**场景下，单个生产端 span 会成为大量、跨时间到达的消费端 span 的父节点——尤其 retained 消息携带的 `traceparent` 会让后续每个新订阅者都挂到一个早已结束的生产端 span 上。此时改用 span link 更合适：消费端 span 成为新 trace 的根，通过 link 引用生产端 span context。

```go
opts.UseHandlerMiddleware(otelgmqtt.HandlerMiddleware(
    otelgmqtt.WithEnvelopeMode(otelgmqtt.EnvelopeModeJSON),
    otelgmqtt.WithSpanLinks(true), // fan-out / retained 场景推荐
))
```

> 部分 APM 后端对 span link 的可视化弱于父子瀑布图。若部署以 1:1 即时消费为主，保持默认的父子模型即可。

#### Propagator 优先级

中间件按以下优先级选择 propagator：

1. 显式传入的 `WithPropagator(...)`
2. 用户通过 `otel.SetTextMapPropagator(...)` 配置的**全局** propagator
3. 内置默认：`TraceContext` + `Baggage` 组合

即未做任何配置时，开箱即可注入 W3C `traceparent`；同时也尊重进程级的全局 propagator，与其它 instrumentation 保持一致。

#### 配置选项

| Option | 说明 |
| --- | --- |
| `WithTracerProvider(tp)` | 指定 TracerProvider，默认取全局 |
| `WithPropagator(p)` | 指定 propagator，优先级最高 |
| `WithTracerName(name)` | 设置 instrumentation tracer 名称 |
| `WithEnvelopeMode(mode)` | `EnvelopeModeOff`（默认）/ `EnvelopeModeJSON` |
| `WithUnwrapPayload(bool)` | 消费端是否还原原始 payload，默认 `true` |
| `WithSpanLinks(bool)` | 消费端用 span link 而非父子关联，默认 `false` |
| `WithFilter(func(topic) bool)` | 返回 `false` 的 topic 跳过 tracing 与传播 |
| `WithSpanNameFormatter(fn)` | 自定义 span 名称 |
| `WithClientID(id)` | 将 MQTT client id 作为 span 属性 |

#### Span 属性

publish 与 process span 均遵循 OpenTelemetry messaging 语义约定，包含 `messaging.system=mqtt`、`messaging.destination.name`、`messaging.mqtt.qos`、`messaging.client.id` 等属性。process span 额外携带 `messaging.mqtt.message_id`、`messaging.mqtt.duplicate` 和 `messaging.message.payload_size_bytes`；publish span 在可计算 payload 大小时也会带上 `messaging.message.payload_size_bytes`。开启 envelope 时还会附加 `gmqtt.envelope.*` 与 `gmqtt.trace.*` 状态属性，便于排查传播是否生效。

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

### 共享订阅

shared subscription 是 MQTT broker 侧的消费组能力。同一个 group 内的多个客户端订阅相同 filter 时，broker 会把每条消息投递给其中一个客户端，而不是每个客户端都收到一份。

```go
// 单个 shared subscription
err := client.SubscribeShared("workers", "sensor/+/temperature", gmqtt.QoS1, func(topic string, payload []byte) error {
    log.Printf("收到温度数据: topic=%s payload=%s", topic, string(payload))
    return nil
})
if err != nil {
    log.Fatal(err)
}

// 批量 shared subscription
subscriptions := map[string]gmqtt.QoS{
    "device/+/status": gmqtt.QoS1,
    "system/#":        gmqtt.QoS0,
}
err = client.SubscribeSharedMultiple("workers", subscriptions, handler)
if err != nil {
    log.Fatal(err)
}

// 取消 shared subscription
err = client.UnsubscribeShared("workers", "sensor/+/temperature")
```

group name 不能为空，且不能包含 `/`、`+`、`#`；topic filter 不能为空。

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
ConnectContext(ctx context.Context) error
Disconnect(quiesce uint)
IsConnected() bool

// 发布消息
Publish(topic string, qos QoS, retained bool, payload interface{}) error
PublishContext(ctx context.Context, topic string, qos QoS, retained bool, payload interface{}) error
PublishWithTimeout(topic string, qos QoS, retained bool, payload interface{}, timeout time.Duration) error

// 订阅管理
Subscribe(topic string, qos QoS, handler MessageHandler) error
SubscribeContext(ctx context.Context, topic string, qos QoS, handler MessageHandlerContext) error
SubscribeMultiple(subscriptions map[string]QoS, handler MessageHandler) error
SubscribeMultipleContext(ctx context.Context, subscriptions map[string]QoS, handler MessageHandlerContext) error
SubscribeShared(group, topic string, qos QoS, handler MessageHandler) error
SubscribeSharedContext(ctx context.Context, group, topic string, qos QoS, handler MessageHandlerContext) error
SubscribeSharedMultiple(group string, subscriptions map[string]QoS, handler MessageHandler) error
SubscribeSharedMultipleContext(ctx context.Context, group string, subscriptions map[string]QoS, handler MessageHandlerContext) error
Unsubscribe(topics ...string) error
UnsubscribeContext(ctx context.Context, topics ...string) error
UnsubscribeShared(group string, topics ...string) error
UnsubscribeSharedContext(ctx context.Context, group string, topics ...string) error

// 其他
SetDefaultHandler(handler MessageHandler)
SetDefaultHandlerContext(handler MessageHandlerContext)
GetSubscriptions() map[string]*Subscription
```

### Publisher

```go
Publish(payload interface{}) error
PublishContext(ctx context.Context, payload interface{}) error
PublishRetained(payload interface{}) error
PublishRetainedContext(ctx context.Context, payload interface{}) error
PublishWithTimeout(payload interface{}, timeout time.Duration) error
PublishJSON(data interface{}) error
PublishJSONContext(ctx context.Context, data interface{}) error
PublishJSONRetained(data interface{}) error
PublishJSONRetainedContext(ctx context.Context, data interface{}) error
PublishString(message string) error
PublishStringContext(ctx context.Context, message string) error
PublishBytes(data []byte) error
PublishBytesContext(ctx context.Context, data []byte) error
SetTopic(topic string)
SetQoS(qos QoS)
```

### Subscriber

```go
Subscribe() error
SubscribeContext(ctx context.Context) error
Unsubscribe() error
UnsubscribeContext(ctx context.Context) error
SetHandler(handler MessageHandler)
SetHandlerContext(handler MessageHandlerContext)
SetJSONHandler(handler func(topic string, data interface{}) error, dataType interface{})
SetJSONHandlerContext(handler func(ctx context.Context, topic string, data interface{}) error, dataType interface{})
SetStringHandler(handler func(topic string, message string) error)
SetStringHandlerContext(handler func(ctx context.Context, topic string, message string) error)
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
    ErrSubscribeTimeout          // 订阅超时
    ErrUnsubscribeTimeout        // 取消订阅超时
    ErrInvalidSharedSubscription // shared subscription 参数无效
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

完整的可运行示例请参考 [`test/main.go`](test/main.go)，覆盖连接、发布、Publisher/Subscriber 模式、订阅者组、批量订阅与共享订阅等用法。

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

核心模块依赖：

- [github.com/eclipse/paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) v1.4.3

`otelgmqtt` 模块按需单独安装，并依赖 [go.opentelemetry.io/otel](https://go.opentelemetry.io/otel) v1.24.0。

## 发布标签

根模块 `github.com/nicexiaonie/gmqtt` 使用普通 tag，例如 `v1.0.0`。

独立 trace 模块 `github.com/nicexiaonie/gmqtt/otelgmqtt` 使用子目录前缀 tag，例如 `otelgmqtt/v1.0.0`。

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
- 支持 `context.Context` API 和 middleware 扩展
- 提供 `otelgmqtt` OpenTelemetry trace 中间件
