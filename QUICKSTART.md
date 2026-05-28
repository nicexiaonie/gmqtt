# gmqtt 快速开始指南

## 5 分钟上手

### 1. 安装

```bash
go get github.com/nicexiaonie/gmqtt
```

### 2. 最简示例

```go
package main

import (
    "log"
    "time"
    "github.com/nicexiaonie/gmqtt"
)

func main() {
    // 创建配置
    opts := gmqtt.NewDefaultOptions()
    opts.Brokers = []string{"tcp://localhost:1883"}
    opts.ClientID = "quickstart"

    // 创建客户端
    client, _ := gmqtt.NewClient(opts)
    client.Connect()
    defer client.Disconnect(250)

    // 订阅
    client.Subscribe("test/topic", gmqtt.QoS1, func(topic string, payload []byte) error {
        log.Printf("收到: %s", string(payload))
        return nil
    })

    // 发布
    client.Publish("test/topic", gmqtt.QoS1, false, "Hello!")

    time.Sleep(2 * time.Second)
}
```

### 3. 运行测试 Broker

```bash
# 使用 Docker 启动 Mosquitto
docker run -d --name mosquitto -p 1883:1883 eclipse-mosquitto:latest

# 或使用 EMQX
docker run -d --name emqx -p 1883:1883 emqx/emqx:latest
```

### 4. 运行示例

```bash
cd gmqtt/examples/basic
go run main.go
```

## 常用场景

### 场景 1: IoT 设备数据上报

```go
// 设备端
publisher := gmqtt.NewPublisher(client, "device/001/data", gmqtt.QoS1)

type SensorData struct {
    Temperature float64 `json:"temperature"`
    Humidity    float64 `json:"humidity"`
    Timestamp   int64   `json:"timestamp"`
}

data := SensorData{
    Temperature: 25.5,
    Humidity:    60.0,
    Timestamp:   time.Now().Unix(),
}

publisher.PublishJSON(data)
```

```go
// 服务端
subscriber := gmqtt.NewSubscriber(client, []string{"device/+/data"}, gmqtt.QoS1)

var data SensorData
subscriber.SetJSONHandler(func(topic string, d interface{}) error {
    sensorData := d.(*SensorData)
    log.Printf("设备数据: 温度=%.2f, 湿度=%.2f",
        sensorData.Temperature, sensorData.Humidity)
    return nil
}, &data)

subscriber.Subscribe()
```

### 场景 2: 设备在线监控

```go
// 配置遗嘱消息
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://localhost:1883"}
opts.ClientID = "device-001"
opts.WillEnabled = true
opts.WillTopic = "device/001/status"
opts.WillPayload = []byte(`{"status":"offline"}`)
opts.WillQoS = gmqtt.QoS1
opts.WillRetained = true

client, _ := gmqtt.NewClient(opts)
client.Connect()

// 发布在线状态
client.Publish("device/001/status", gmqtt.QoS1, true, `{"status":"online"}`)

// 监控端订阅状态主题
client.Subscribe("device/+/status", gmqtt.QoS1, func(topic string, payload []byte) error {
    log.Printf("设备状态变化: %s - %s", topic, string(payload))
    return nil
})
```

### 场景 3: 实时告警推送

```go
// 告警发送
publisher := gmqtt.NewPublisher(client, "system/alert", gmqtt.QoS2)

type Alert struct {
    Level   string `json:"level"`   // info, warning, error, critical
    Message string `json:"message"`
    Time    int64  `json:"time"`
}

alert := Alert{
    Level:   "critical",
    Message: "CPU 使用率超过 90%",
    Time:    time.Now().Unix(),
}

publisher.PublishJSON(alert)
```

```go
// 告警接收
subscriber := gmqtt.NewSubscriber(client, []string{"system/alert"}, gmqtt.QoS2)

var alert Alert
subscriber.SetJSONHandler(func(topic string, d interface{}) error {
    a := d.(*Alert)

    // 根据级别处理
    switch a.Level {
    case "critical":
        // 发送短信、电话告警
        sendSMS(a.Message)
    case "error":
        // 发送邮件
        sendEmail(a.Message)
    case "warning":
        // 记录日志
        log.Println(a.Message)
    }

    return nil
}, &alert)

subscriber.Subscribe()
```

### 场景 4: 微服务间通信

```go
// 服务 A 发布事件
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://mqtt-broker:1883"}
opts.ClientID = "service-a"

client, _ := gmqtt.RegisterClient("default", opts)
client.Connect()

publisher := gmqtt.NewPublisher(client, "events/order/created", gmqtt.QoS1)

type OrderCreatedEvent struct {
    OrderID   string  `json:"order_id"`
    UserID    string  `json:"user_id"`
    Amount    float64 `json:"amount"`
    Timestamp int64   `json:"timestamp"`
}

event := OrderCreatedEvent{
    OrderID:   "ORD-12345",
    UserID:    "USER-001",
    Amount:    99.99,
    Timestamp: time.Now().Unix(),
}

publisher.PublishJSON(event)
```

```go
// 服务 B 订阅事件
opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"tcp://mqtt-broker:1883"}
opts.ClientID = "service-b"

client, _ := gmqtt.RegisterClient("default", opts)
client.Connect()

subscriber := gmqtt.NewSubscriber(client, []string{"events/order/#"}, gmqtt.QoS1)

var event OrderCreatedEvent
subscriber.SetJSONHandler(func(topic string, d interface{}) error {
    e := d.(*OrderCreatedEvent)

    // 处理订单创建事件
    log.Printf("收到订单: %s, 金额: %.2f", e.OrderID, e.Amount)

    // 执行业务逻辑
    processOrder(e)

    return nil
}, &event)

subscriber.Subscribe()
```

```go
// 多个服务实例共享消费同一类事件
err := client.SubscribeShared("order-workers", "events/order/#", gmqtt.QoS1, func(topic string, payload []byte) error {
    log.Printf("处理订单事件: topic=%s payload=%s", topic, string(payload))
    return nil
})
if err != nil {
    log.Fatal(err)
}
```

## 配置示例

### 生产环境配置

```go
opts := gmqtt.NewDefaultOptions()

// Broker 配置（支持多个，自动故障转移）
opts.Brokers = []string{
    "tcp://mqtt1.example.com:1883",
    "tcp://mqtt2.example.com:1883",
}

// 客户端标识（必须唯一）
opts.ClientID = fmt.Sprintf("app-%s-%d", hostname, pid)

// 认证
opts.Username = "your-username"
opts.Password = "your-password"

// 会话配置
opts.CleanSession = false  // 持久化会话
opts.KeepAlive = 60 * time.Second

// 重连配置
opts.AutoReconnect = true
opts.MaxReconnectInterval = 10 * time.Minute
opts.ConnectRetry = true
opts.ConnectRetryInterval = 1 * time.Second

// 超时配置
opts.ConnectTimeout = 30 * time.Second
opts.WriteTimeout = 30 * time.Second

// 性能配置
opts.MessageChannelDepth = 1000  // 增加消息缓冲

// 回调配置
opts.OnConnect = func() {
    log.Println("✓ MQTT 已连接")
    // 重新订阅主题
}

opts.OnConnectionLost = func(err error) {
    log.Printf("✗ MQTT 连接丢失: %v", err)
    // 告警通知
}

client, _ := gmqtt.NewClient(opts)
```

### TLS/SSL 配置

```go
import "crypto/tls"

tlsConfig := &tls.Config{
    InsecureSkipVerify: false,
}

// 加载客户端证书
cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
tlsConfig.Certificates = []tls.Certificate{cert}

// 加载 CA 证书
caCert, _ := ioutil.ReadFile("ca.crt")
caCertPool := x509.NewCertPool()
caCertPool.AppendCertsFromPEM(caCert)
tlsConfig.RootCAs = caCertPool

opts := gmqtt.NewDefaultOptions()
opts.Brokers = []string{"ssl://mqtt.example.com:8883"}
opts.ClientID = "secure-client"
opts.TLSConfig = tlsConfig

client, _ := gmqtt.NewClient(opts)
```

## 调试技巧

### 1. 启用详细日志

```go
import "log"

opts.OnConnect = func() {
    log.Println("[MQTT] Connected")
}

opts.OnConnectionLost = func(err error) {
    log.Printf("[MQTT] Connection lost: %v", err)
}

// 在消息处理器中记录
client.Subscribe("test/+", gmqtt.QoS1, func(topic string, payload []byte) error {
    log.Printf("[MQTT] Received: topic=%s, payload=%s", topic, string(payload))
    return nil
})
```

### 2. 检查连接状态

```go
if !client.IsConnected() {
    log.Println("客户端未连接")
    // 重连逻辑
}
```

### 3. 错误处理

```go
err := client.Publish(topic, qos, false, payload)
if err != nil {
    switch err {
    case gmqtt.ErrNotConnected:
        log.Println("未连接，尝试重连")
        client.Connect()
    case gmqtt.ErrPublishTimeout:
        log.Println("发布超时，重试")
        // 重试逻辑
    default:
        log.Printf("发布失败: %v", err)
    }
}
```

### 4. 使用 MQTT 客户端工具测试

```bash
# 使用 mosquitto_sub 订阅
mosquitto_sub -h localhost -t "test/#" -v

# 使用 mosquitto_pub 发布
mosquitto_pub -h localhost -t "test/topic" -m "Hello"

# 使用 MQTTX (GUI 工具)
# https://mqttx.app/
```

## 性能优化

### 1. 批量订阅

```go
// ❌ 不推荐：逐个订阅
client.Subscribe("topic1", gmqtt.QoS1, handler)
client.Subscribe("topic2", gmqtt.QoS1, handler)
client.Subscribe("topic3", gmqtt.QoS1, handler)

// ✅ 推荐：批量订阅
subscriptions := map[string]gmqtt.QoS{
    "topic1": gmqtt.QoS1,
    "topic2": gmqtt.QoS1,
    "topic3": gmqtt.QoS1,
}
client.SubscribeMultiple(subscriptions, handler)
```

### 2. 选择合适的 QoS

```go
// 传感器数据（可容忍丢失）
publisher.SetQoS(gmqtt.QoS0)

// 日志上报（需要送达）
publisher.SetQoS(gmqtt.QoS1)

// 支付通知（不能重复）
publisher.SetQoS(gmqtt.QoS2)
```

### 3. 调整消息缓冲

```go
opts.MessageChannelDepth = 1000  // 默认 100
```

### 4. 使用连接池

```go
// 创建多个客户端实例
for i := 0; i < 10; i++ {
    opts := gmqtt.NewDefaultOptions()
    opts.ClientID = fmt.Sprintf("client-%d", i)
    client, _ := gmqtt.RegisterClient(opts.ClientID, opts)
    client.Connect()
}
```

## 常见问题

### Q: 如何确保消息不丢失？
A: 使用 QoS 1 或 QoS 2，并启用持久化会话（CleanSession=false）

### Q: 如何处理大量消息？
A: 调整 MessageChannelDepth，或使用多个客户端实例

### Q: 如何监控客户端状态？
A: 使用 OnConnect 和 OnConnectionLost 回调

### Q: 支持 WebSocket 吗？
A: 支持，使用 `ws://` 或 `wss://` 协议

### Q: 如何实现请求/响应模式？
A: 使用两个主题，一个用于请求，一个用于响应

## 下一步

- 阅读 [完整文档](README.md)
- 查看 [设计文档](DESIGN.md)
- 运行 [示例代码](examples/)
- 阅读 [测试用例](gmqtt_test.go)

## 获取帮助

- 查看文档: `README.md`
- 运行测试: `go test -v`
- 查看示例: `examples/`
- 提交 Issue: 项目仓库

---

**祝你使用愉快！** 🚀
