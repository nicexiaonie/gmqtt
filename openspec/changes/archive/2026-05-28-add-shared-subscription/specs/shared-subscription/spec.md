## ADDED Requirements

### Requirement: Shared subscription API
库 SHALL 提供 Client 级 API，用于通过 MQTT shared subscription group 订阅 topic filter。

#### Scenario: 订阅 shared topic filter
- **WHEN** 调用方使用 group `workers`、filter `sensors/+/data`、QoS 1 和 handler 发起 shared subscription
- **THEN** Client 使用 QoS 1 向 broker 订阅 `$share/workers/sensors/+/data`

#### Scenario: 接收 shared subscription 消息
- **WHEN** broker 基于 shared subscription `$share/workers/sensors/+/data` 投递一条发布到 `sensors/a/data` 的消息
- **THEN** 为该 shared subscription 注册的 handler 被调用，并收到 topic `sensors/a/data` 和消息 payload

### Requirement: Batch shared subscription API
库 SHALL 提供 Client 级 API，用于在同一个 MQTT shared subscription group 下批量订阅多个 topic filter。

#### Scenario: 批量订阅多个 shared filters
- **WHEN** 调用方使用 group `workers` 批量订阅 filter `sensors/+/data`，QoS 1，以及 filter `alerts/#`，QoS 0，并使用同一个 handler
- **THEN** Client 分别向 broker 订阅 `$share/workers/sensors/+/data` 和 `$share/workers/alerts/#`，并使用各自请求的 QoS

#### Scenario: 批量 shared subscription 失败清理
- **WHEN** 批量 shared subscription 尝试失败
- **THEN** Client 删除本次失败尝试创建的本地订阅项

### Requirement: Shared unsubscribe API
库 SHALL 提供 Client 级 API，用于通过 group 和原始 topic filter 取消 shared subscription。

#### Scenario: 取消单个 shared topic filter
- **WHEN** 调用方取消 group `workers` 和 filter `sensors/+/data` 的 shared subscription
- **THEN** Client 向 broker 取消订阅 `$share/workers/sensors/+/data`，并删除对应本地订阅项

#### Scenario: 批量取消多个 shared topic filters
- **WHEN** 调用方取消 group `workers` 下的 filters `sensors/+/data` 和 `alerts/#`
- **THEN** Client 向 broker 取消订阅 `$share/workers/sensors/+/data` 和 `$share/workers/alerts/#`

### Requirement: Shared subscription validation
库 SHALL 在调用 broker 前拒绝结构非法的 shared subscription 输入，并返回可与 broker 订阅失败区分的 validation error。

#### Scenario: 拒绝空 group name
- **WHEN** 调用方使用空 group name 创建 shared subscription
- **THEN** 操作以 validation error 失败，且不会调用 broker

#### Scenario: 拒绝非法 group name
- **WHEN** 调用方使用包含 `/`、`+` 或 `#` 的 group name 创建 shared subscription
- **THEN** 操作以 validation error 失败，且不会调用 broker

#### Scenario: 拒绝空 topic filter
- **WHEN** 调用方使用空 topic filter 创建 shared subscription
- **THEN** 操作以 validation error 失败，且不会调用 broker

### Requirement: Wildcard handler dispatch
库 SHALL 将收到的消息路由到匹配 MQTT topic filter 的 handler，包括普通 wildcard filter 和 shared subscription wildcard filter。

#### Scenario: 匹配单层 wildcard filter
- **WHEN** handler 注册在 filter `devices/+/status`，且消息到达 topic `devices/a/status`
- **THEN** 该 handler 被调用，并收到 topic `devices/a/status`

#### Scenario: 匹配多层 wildcard filter
- **WHEN** handler 注册在 filter `devices/#`，且消息到达 topic `devices/a/status`
- **THEN** 该 handler 被调用，并收到 topic `devices/a/status`

#### Scenario: 精确 topic handler 优先
- **WHEN** handler 分别注册在精确 topic `devices/a/status` 和 wildcard filter `devices/+/status`
- **THEN** 到达 topic `devices/a/status` 的消息被路由到精确 topic handler

#### Scenario: 多个 wildcard 命中时使用注册顺序
- **WHEN** handler 按顺序注册在 wildcard filters `devices/#` 和 `devices/+/status`
- **THEN** 到达 topic `devices/a/status` 的消息被路由到第一个匹配的 wildcard handler

#### Scenario: 无 filter 匹配时使用 default handler
- **WHEN** 没有已注册的精确 topic 或 wildcard filter 匹配收到的消息 topic
- **THEN** 如果配置了 default handler，则调用 default handler
