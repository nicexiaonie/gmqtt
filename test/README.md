# GMQTT 全面测试框架

这是一个全面的 MQTT 客户端测试框架，覆盖了 gmqtt 包的所有 API 方法，并支持多维度的持久化测试。

## 功能特性

### 1. 全面的 API 测试覆盖

测试框架覆盖了 gmqtt 包的所有公开 API：

#### Client API
- `NewClient` - 客户端创建
- `Connect` / `Disconnect` - 连接管理
- `IsConnected` - 状态检查
- `Publish` / `PublishWithTimeout` - 消息发布
- `Subscribe` / `SubscribeMultiple` - 订阅管理
- `Unsubscribe` - 取消订阅
- `SetDefaultHandler` - 默认处理器
- `GetSubscriptions` - 订阅查询
- `RegisterClient` / `GetClient` / `CloseAll` - 全局客户端管理

#### Publisher API
- `NewPublisher` - 创建发布者
- `Publish` / `PublishRetained` / `PublishWithTimeout` - 发布方法
- `PublishJSON` / `PublishJSONRetained` - JSON 发布
- `PublishString` / `PublishBytes` - 字符串和字节发布
- `SetTopic` / `GetTopic` - 主题管理
- `SetQoS` / `GetQoS` - QoS 管理

#### Subscriber API
- `NewSubscriber` - 创建订阅者
- `SetHandler` / `SetJSONHandler` / `SetStringHandler` - 处理器设置
- `Subscribe` / `Unsubscribe` - 订阅控制
- `AddTopic` / `RemoveTopic` / `GetTopics` - 主题管理
- `SetQoS` / `GetQoS` - QoS 管理

#### SubscriberGroup API
- `NewSubscriberGroup` - 创建订阅组
- `Add` - 添加订阅者
- `SubscribeAll` / `UnsubscribeAll` - 批量操作
- `Count` - 计数

### 2. 多维度测试支持

#### 测试类别
1. **基础连接测试** - 连接、断开、状态检查、回调函数
2. **QoS 级别测试** - QoS 0/1/2 的消息传递
3. **持久化会话测试** - 离线消息接收
4. **保留消息测试** - Retained 消息机制
5. **遗嘱消息测试** - Last Will 机制
6. **Publisher API 测试** - 所有发布方法
7. **Subscriber API 测试** - 所有订阅方法
8. **JSON 消息测试** - 序列化和反序列化
9. **全局客户端管理** - 客户端注册和查询
10. **并发压力测试** - 多客户端并发发布

### 3. 持久化组合测试

框架支持以下持久化配置的组合测试：

| 测试场景 | CleanSession | QoS | 说明 |
|---------|--------------|-----|------|
| 2.1 | true | 0 | 不保留会话，无消息确认 |
| 2.2 | false | 0 | 保留会话，无消息确认 |
| 2.3 | true | 1 | 不保留会话，至少一次 |
| 2.4 | false | 1 | 保留会话，至少一次 |
| 2.5 | true | 2 | 不保留会话，只有一次 |
| 2.6 | false | 2 | 保留会话，只有一次 |
| 3.1 | false | 1 | 离线消息接收测试 |

每个测试都会验证：
- 消息发送成功
- 消息接收完整
- 消息丢失统计
- 消息内容正确

### 4. 测试结果追踪

框架提供详细的测试统计：

#### 测试维度统计
- 总测试数
- 通过数量和百分比
- 失败数量和百分比
- 每个测试的耗时

#### 消息维度统计
- 消息发送总数
- 消息接收总数
- 消息丢失总数
- 消息重复总数（预留）

#### 输出格式
- 控制台实时输出
- 日志文件 (`gmqtt_test.log`)
- JSON 报告 (`gmqtt_test_report.json`)

## 环境准备

### 1. 启动 EMQX Broker

```bash
# 拉取并启动 EMQX 容器
docker run -d --name emqx -p 18083:18083 -p 1883:1883 emqx/emqx:latest

# 访问 Dashboard
# URL: http://localhost:18083
# 默认用户名/密码: admin / public
```

### 2. 创建测试用户

```bash
# 进入容器
docker exec -it emqx /bin/bash

# 创建测试用户
emqx ctl users add test1 123456
emqx ctl users add test2 123456

# 退出容器
exit
```

### 3. 配置测试参数

编辑 `main.go` 中的配置常量：

```go
const (
    brokerAddr = "tcp://127.0.0.1:1883"  // Broker 地址
    username   = "test1"                  // 用户名
    password   = "123456"                 // 密码
)
```

## 运行测试

### 编译

```bash
cd gmqtt/test
go build
```

### 执行

```bash
./test
```

### 查看结果

测试完成后，会生成以下文件：

1. **控制台输出** - 实时显示测试进度和结果
2. **gmqtt_test.log** - 详细的测试日志
3. **gmqtt_test_report.json** - 可机读的 JSON 格式报告

## 测试报告示例

```
================================================================================
GMQTT 全面测试报告
================================================================================
测试开始时间: 2024-01-15 10:30:00
测试结束时间: 2024-01-15 10:35:00
总耗时: 5m0s

总测试数: 19
通过: 18 (94.7%)
失败: 1 (5.3%)

消息统计:
  发送: 230
  接收: 228
  丢失: 2
  重复: 0

详细结果:
--------------------------------------------------------------------------------
  1. [✓ PASS] 1.1 基础连接与断开 (耗时: 150ms)
  2. [✓ PASS] 1.2 连接回调函数 (耗时: 200ms)
  3. [✓ PASS] 2.1 QoS0 + CleanSession=true (耗时: 1.5s)
     详情: QoS=0, CleanSession=true, 发送=10, 接收=10
  ...
================================================================================
```

## 测试架构

### 核心组件

#### TestSuite
测试套件，负责协调和管理所有测试

```go
type TestSuite struct {
    stats   *TestStats        // 统计信息
    tracker *MessageTracker   // 消息追踪
}
```

#### TestStats
测试统计，记录测试结果和消息统计

```go
type TestStats struct {
    Total           int           // 总测试数
    Passed          int           // 通过数
    Failed          int           // 失败数
    Results         []*TestResult // 详细结果
    MessagesSent    int64         // 发送消息数
    MessagesRecv    int64         // 接收消息数
    MessagesLost    int64         // 丢失消息数
}
```

#### TestResult
单个测试结果

```go
type TestResult struct {
    Name      string        // 测试名称
    Passed    bool          // 是否通过
    Error     error         // 错误信息
    Duration  time.Duration // 耗时
    Details   string        // 详细信息
    Timestamp time.Time     // 时间戳
}
```

#### MessageTracker
消息追踪器，用于追踪消息的发送和接收（预留扩展）

### 测试方法

#### Run()
运行基础测试，返回 error

```go
suite.Run("测试名称", func() error {
    // 测试逻辑
    return nil
})
```

#### RunWithDetails()
运行带详情的测试，返回 (details string, error)

```go
suite.RunWithDetails("测试名称", func() (string, error) {
    // 测试逻辑
    return "详细信息", nil
})
```

## 扩展测试

### 添加新测试

1. 在 TestSuite 上定义新的测试方法：

```go
func (ts *TestSuite) TestNewFeature() error {
    // 测试逻辑
    return nil
}
```

2. 在 main() 函数中调用：

```go
suite.Run("新功能测试", suite.TestNewFeature)
```

### 添加持久化测试场景

修改 TestQoS 方法，添加新的 QoS 和 CleanSession 组合：

```go
suite.RunWithDetails("2.7 QoS1 + 自定义配置", func() (string, error) {
    return suite.TestQoS(gmqtt.QoS1, false)
})
```

### 添加消息验证

使用 MessageTracker 追踪消息：

```go
msgID := "unique-message-id"
ts.tracker.MarkSent(msgID, payload)

// 在接收端
ts.tracker.MarkReceived(msgID)

// 获取统计
sent, recv, lost, avgLatency := ts.tracker.GetStats()
```

## 故障排查

### 常见问题

#### 1. 连接失败
- 检查 EMQX 是否正在运行：`docker ps`
- 检查端口是否开放：`netstat -an | grep 1883`
- 检查用户名密码是否正确

#### 2. 测试超时
- 增加测试超时时间
- 检查网络延迟
- 检查 Broker 性能

#### 3. 消息丢失
- QoS0 可能会丢失消息，这是正常的
- 检查 Broker 配置
- 检查网络稳定性

#### 4. 并发测试失败
- 减少并发客户端数量
- 增加消息发送间隔
- 检查系统资源

## 最佳实践

1. **隔离测试** - 每个测试使用独立的 ClientID 和 Topic
2. **清理资源** - 使用 defer 确保客户端正确断开
3. **合理超时** - 根据网络情况设置合适的超时时间
4. **消息验证** - 验证消息内容而不仅仅是数量
5. **错误处理** - 记录详细的错误信息便于调试

## 性能基准

在标准测试环境下的预期性能：

- 单客户端连接时间：< 200ms
- QoS0 消息延迟：< 10ms
- QoS1 消息延迟：< 50ms
- QoS2 消息延迟：< 100ms
- 并发10客户端吞吐量：> 1000 msg/s

## 许可证

本测试框架遵循 gmqtt 包的许可证。
